#!/usr/bin/env bash
#
# Start extension TEE node and proxy.
#
# By default, starts services via Docker Compose:
#   - LOCAL_MODE=true  (default) → uses docker-compose.yaml only (local devnet)
#   - LOCAL_MODE=false            → also attaches docker-compose.coston2.yaml
#
# Pass --local to start services as background Go processes instead of Docker.
#
# Usage:
#   ./scripts/start-services.sh              # docker compose (default)
#   ./scripts/start-services.sh --local      # background Go processes
#
# Prerequisites:
#   - Infrastructure running (Hardhat, indexer, Redis, normal TEE + proxy)
#   - config/extension.env exists (created by pre-build.sh), OR EXTENSION_ID is set
#   - Redis will be started on :6382 automatically (separate from infrastructure Redis)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[start-services]${NC} $*"; }
die()  { echo -e "${RED}[start-services] ERROR:${NC} $*" >&2; exit 1; }

# --- Parse flags ---
USE_LOCAL=false
for arg in "$@"; do
    case "$arg" in
        --local) USE_LOCAL=true ;;
        *) die "Unknown argument: $arg" ;;
    esac
done

# --- Load .env from project root (if present) ---
if [[ -f "$PROJECT_DIR/.env" ]]; then
    set -a
    source "$PROJECT_DIR/.env"
    set +a
fi

# --- Load extension config ---
CONFIG_FILE="$PROJECT_DIR/config/extension.env"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$CONFIG_FILE"
fi

EXTENSION_ID="${EXTENSION_ID:-}"
PROXY_PRIVATE_KEY="${PROXY_PRIVATE_KEY:-0x983760a4ebf75b2ac3a93531168a0f225d01e5dc6e3568adbd46233ba1fb4fa4}"
LOCAL_MODE="${LOCAL_MODE:-true}"

[[ -n "$EXTENSION_ID" ]] || die "EXTENSION_ID not set. Run pre-build.sh first or set it manually."

log "Extension ID: $EXTENSION_ID"
log "Local mode:   $LOCAL_MODE"

# ============================================================
# Docker Compose mode (default)
# ============================================================
if [[ "$USE_LOCAL" == "false" ]]; then
    log "Starting services with Docker Compose..."

    # --- Build tee-proxy image locally if no remote registry is configured ---
    if [[ -z "${REGISTRY:-}" ]]; then
        if ! docker image inspect local/tee-proxy >/dev/null 2>&1; then
            TEE_ROOT="$(cd "$PROJECT_DIR/../.." && pwd)"
            TEE_PROXY_DIR="$TEE_ROOT/tee-proxy"
            TEE_NODE_DIR="$TEE_ROOT/tee-node"
            if [[ ! -d "$TEE_PROXY_DIR" ]]; then
                die "Image local/tee-proxy not found and tee-proxy repo not present at $TEE_PROXY_DIR.\n  Either set REGISTRY in .env to pull from a remote registry, or clone the tee-proxy repo into $TEE_ROOT/."
            fi
            if [[ ! -d "$TEE_NODE_DIR" ]]; then
                die "Image local/tee-proxy not found and tee-node repo not present at $TEE_NODE_DIR.\n  The tee-proxy Dockerfile requires tee-node as a build dependency. Clone tee-node into $TEE_ROOT/."
            fi
            log "Building local/tee-proxy image from $TEE_PROXY_DIR..."
            docker build -f "$TEE_PROXY_DIR/Dockerfile" -t local/tee-proxy "$TEE_ROOT" || die "Failed to build tee-proxy image"
            log "local/tee-proxy image built successfully"
        else
            log "local/tee-proxy image already exists (use 'docker rmi local/tee-proxy' to force rebuild)"
        fi
    fi

    COMPOSE_FILES=("-f" "$PROJECT_DIR/docker-compose.yaml")

    if [[ "$LOCAL_MODE" != "true" ]]; then
        log "Coston2 mode detected — attaching docker-compose.coston2.yaml"
        COMPOSE_FILES+=("-f" "$PROJECT_DIR/docker-compose.coston2.yaml")
    fi

    docker compose "${COMPOSE_FILES[@]}" up -d --build || die "docker compose up failed"

    # Wait for proxy to be ready
    E2E="$SCRIPT_DIR/e2e.sh"
    EXT_PROXY_URL="${EXT_PROXY_URL:-http://localhost:6674}"
    log "Waiting for extension proxy at $EXT_PROXY_URL/info ..."
    "$E2E" wait-for-url "$EXT_PROXY_URL/info" 120

    # Validate EXTENSION_ID is recognized by proxy
    log "Validating EXTENSION_ID against proxy..."
    PROXY_INFO=$(curl -sf "$EXT_PROXY_URL/info" 2>/dev/null || true)
    if [[ -n "$PROXY_INFO" ]]; then
        if ! echo "$PROXY_INFO" | grep -q "$EXTENSION_ID" 2>/dev/null; then
            echo -e "${RED}WARNING: EXTENSION_ID $EXTENSION_ID not found in proxy /info response${NC}" >&2
            echo -e "${RED}The proxy may be filtering for a different extension. Check config.${NC}" >&2
        fi
    fi

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN} Services started (Docker Compose)${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "${CYAN}Mode${NC}"
    if [[ "$LOCAL_MODE" == "true" ]]; then
        echo "  Local devnet"
    else
        echo "  Coston2 testnet"
    fi
    echo ""
    echo -e "${CYAN}Services${NC}"
    echo "  redis, ext-proxy, extension-tee"
    echo "  Proxy URL: $EXT_PROXY_URL"
    echo ""
    echo -e "${CYAN}Commands${NC}"
    echo "  Logs:    docker compose ${COMPOSE_FILES[*]} logs -f"
    echo "  Stop:    ./scripts/stop-services.sh"
    exit 0
fi

# ============================================================
# Local Go process mode (--local)
# ============================================================
log "Starting services as local Go processes (--local)..."

E2E="$SCRIPT_DIR/e2e.sh"
PID_DIR="$PROJECT_DIR/out/pids"
LOG_DIR="$PROJECT_DIR/out/logs"

# --- Build Go binaries (once) so we run the actual binary, not `go run` ---
BIN_DIR="$PROJECT_DIR/out/bin"
mkdir -p "$BIN_DIR"
log "Building Go binaries..."
cd "$PROJECT_DIR/tools"
go build -o "$BIN_DIR/start-tee" ./cmd/start-tee
go build -o "$BIN_DIR/start-proxy" ./cmd/start-proxy

# --- Start extension TEE ---
log "Starting extension TEE node..."
EXTENSION_ID="$EXTENSION_ID" "$E2E" start ext-tee "$PID_DIR/ext-tee.pid" "$LOG_DIR/ext-tee.log" \
    "$BIN_DIR/start-tee" -extensionID "$EXTENSION_ID"

log "Waiting for extension TEE to initialize..."
sleep 5

# --- Start extension Redis on port 6382 via Docker Compose ---
log "Starting Redis via Docker Compose..."
docker compose -f "$PROJECT_DIR/docker-compose.yaml" up -d redis
log "Waiting for Redis on :6382..."
retries=0
while ! docker compose -f "$PROJECT_DIR/docker-compose.yaml" exec -T redis redis-cli ping > /dev/null 2>&1; do
    retries=$((retries + 1))
    if [ $retries -ge 15 ]; then
        die "Redis container failed to become healthy"
    fi
    sleep 1
done
log "Redis on :6382 ready"

# --- Start extension proxy ---
log "Starting extension proxy..."
PROXY_PRIVATE_KEY="$PROXY_PRIVATE_KEY" "$E2E" start ext-proxy "$PID_DIR/ext-proxy.pid" "$LOG_DIR/ext-proxy.log" \
    "$BIN_DIR/start-proxy"

cd "$PROJECT_DIR"

# --- Wait for proxy to be ready ---
if [[ "$EXT_PROXY_URL" != *"localhost"* && "$EXT_PROXY_URL" != *"127.0.0.1"* ]]; then
    log "NOTE: EXT_PROXY_URL=$EXT_PROXY_URL (not localhost) — health check targets this URL"
fi
log "Waiting for extension proxy..."
"$E2E" wait-for-url "http://localhost:6664/info" 60

# --- Summary ---
EXT_TEE_PID=$(cat "$PID_DIR/ext-tee.pid" 2>/dev/null || echo "?")
EXT_PROXY_PID=$(cat "$PID_DIR/ext-proxy.pid" 2>/dev/null || echo "?")

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN} Services started (local Go processes)${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${CYAN}Processes${NC}"
echo "  Extension Redis  Docker container (port 6382)"
echo "  Extension TEE    PID $EXT_TEE_PID"
echo "  Extension Proxy  PID $EXT_PROXY_PID"
echo "  Proxy URL        http://localhost:6664"
echo ""
echo -e "${CYAN}Logs${NC}"
echo "  Redis log        docker compose logs redis"
echo "  TEE log          $LOG_DIR/ext-tee.log"
echo "  Proxy log        $LOG_DIR/ext-proxy.log"
echo ""
echo -e "${CYAN}Commands${NC}"
echo "  Status:  $SCRIPT_DIR/e2e.sh status $PID_DIR"
echo "  Stop:    $SCRIPT_DIR/stop-services.sh --local"
