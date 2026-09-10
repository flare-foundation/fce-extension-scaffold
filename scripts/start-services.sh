#!/usr/bin/env bash
#
# Start extension TEE node and proxy.
#
# By default, starts services via Docker Compose, picking the compose overlay
# from --chain (or env CHAIN, or legacy LOCAL_MODE):
#   --chain local    → docker-compose.yaml only (local devnet)
#   --chain coston   → + docker-compose.coston.yaml
#   --chain coston2  → + docker-compose.coston2.yaml
#
# Pass --local to start services as background Go processes instead of Docker.
#
# On coston/coston2 this first syncs the Cloudflare tunnel
# (docker-compose.cloudflared.yaml): if it's already running (started by this
# or another extension), its current URL is written into .env as
# EXT_PROXY_URL — no flag needed. --tunnel additionally starts it when it's
# not already running.
#
# Usage:
#   ./scripts/start-services.sh                       # local devnet, docker compose
#   ./scripts/start-services.sh --chain coston        # Coston, docker compose
#   ./scripts/start-services.sh --local               # local devnet, Go processes
#   ./scripts/start-services.sh --chain coston2 --tunnel      # also start the tunnel if missing
#
# By default the node and proxy are built from PINNED module/image versions
# (tee-node + tee-proxy), fully self-contained — no sibling repos required.
# To build them from on-disk sibling checkouts instead (while developing
# tee-node / tee-proxy), set USE_LOCAL_SIBLINGS=1:
#   USE_LOCAL_SIBLINGS=1 ./scripts/start-services.sh --chain coston2
#
# Prerequisites:
#   - Infrastructure running (Hardhat, indexer, Redis, normal TEE + proxy)
#   - config/extension.env exists (created by pre-build.sh), OR EXTENSION_ID is set
#   - Redis will be started on :6382 automatically (separate from infrastructure Redis)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/chain-env.sh"   # chain table + resolve_chain

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[start-services]${NC} $*"; }
die()  { echo -e "${RED}[start-services] ERROR:${NC} $*" >&2; exit 1; }

# --- Parse flags ---
USE_LOCAL=false
USE_TUNNEL=false
CHAIN_FLAG=""
usage() {
    cat <<USAGE
start-services.sh — Start the extension TEE node and proxy.

Usage: ./scripts/start-services.sh [flags]

Flags:
  --chain <name>   local | coston | coston2 | songbird | flare (default: from .env)
  --local          run TEE + proxy as background Go processes instead of Docker
  --tunnel         start the shared Cloudflare tunnel even in live mode
  -h, --help       this message
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help) usage; exit 0 ;;
        --local) USE_LOCAL=true; shift ;;
        --tunnel) USE_TUNNEL=true; shift ;;
        --chain) [[ $# -ge 2 ]] || die "--chain requires a value (${CHAINS// /|})"
                 CHAIN_FLAG="$2"; shift 2 ;;
        --chain=*) CHAIN_FLAG="${1#--chain=}"; shift ;;
        *) die "Unknown argument: $1" ;;
    esac
done

# --- Load .env from project root (if present) ---
if [[ -f "$PROJECT_DIR/.env" ]]; then
    set -a
    source "$PROJECT_DIR/.env"
    set +a
fi

# Images target linux/amd64 (GCP); an arm64 host would pull arm64 base images.
export DOCKER_DEFAULT_PLATFORM="${DOCKER_DEFAULT_PLATFORM:-linux/amd64}"

# --- Load extension config ---
CONFIG_FILE="$(extension_env_path)"
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$CONFIG_FILE"
fi

EXTENSION_ID="${EXTENSION_ID:-}"
PROXY_PRIVATE_KEY="${PROXY_PRIVATE_KEY:-983760a4ebf75b2ac3a93531168a0f225d01e5dc6e3568adbd46233ba1fb4fa4}"
LOCAL_MODE="${LOCAL_MODE:-true}"

# --- Resolve CHAIN (flag > env > legacy LOCAL_MODE) ---
# Flag beats .env: sourcing .env above would otherwise clobber --chain.
CHAIN="${CHAIN_FLAG:-${CHAIN:-}}"
if [[ -z "$CHAIN" ]]; then
    if [[ "$LOCAL_MODE" == "true" ]]; then
        CHAIN="local"
    else
        CHAIN="coston2"  # legacy
    fi
fi
valid_chain "$CHAIN" || die "Unknown --chain value: $CHAIN (valid: ${CHAINS// /, })"
resolve_chain "$CHAIN"

[[ -n "$EXTENSION_ID" ]] || die "EXTENSION_ID not set. Run pre-build.sh first or set it manually."

# --- Resolve LANGUAGE by directory convention ---------------------------------
# Discovery is driven by <LANGUAGE>/language.env; there is deliberately no
# hardcoded list here (see docs/extension-contract.md §8).
# shellcheck source=lib/language.sh
source "$SCRIPT_DIR/lib/language.sh"
load_language "$PROJECT_DIR" || die "could not resolve LANGUAGE"

# --- Derive dependency versions from the Go module pin ------------------------
# Language images that build tee-node from source consume TEE_NODE_REF; see
# scripts/lib/versions.sh for why a pseudo-version needs a SHA, not a tag.
# shellcheck source=lib/versions.sh
source "$SCRIPT_DIR/lib/versions.sh"
load_versions "$PROJECT_DIR" || die "could not derive dependency versions"

# --- Guard: --local runs Go binaries directly, so it is Go-only ---------------
if [[ "$USE_LOCAL" == "true" && "$LANGUAGE" != "go" ]]; then
    die "--local mode builds and runs Go binaries in-process and only supports LANGUAGE=go (got '$LANGUAGE').\n  Use Docker Compose mode instead: ./scripts/start-services.sh --chain $CHAIN"
fi

# --- Resolve local-siblings toggle ---
case "${USE_LOCAL_SIBLINGS:-}" in
    1|true|yes|on) USE_LOCAL_SIBLINGS=true ;;
    *)             USE_LOCAL_SIBLINGS=false ;;
esac

# The sibling build compiles tee-node into the Go binary; other languages build
# tee-node as a separate binary from a pinned ref and have no sibling path.
if [[ "$USE_LOCAL_SIBLINGS" == "true" && "$LANGUAGE" != "go" ]]; then
    die "USE_LOCAL_SIBLINGS is only supported for LANGUAGE=go (got '$LANGUAGE').\n  Other languages build tee-node from the pinned ref ($TEE_NODE_REF).\n  To test a local tee-node there, push it and update the pin in go/go.mod."
fi

log "Chain:          $CHAIN"
log "Language:       $LANGUAGE ($EXTENSION_DOCKERFILE)"
log "Extension ID:   $EXTENSION_ID"
log "Local mode:     $LOCAL_MODE"

SIMULATED_TEE="${SIMULATED_TEE:-true}"
if [[ "$SIMULATED_TEE" == "true" ]]; then
    SOURCE_ENV_FILE="$PROJECT_DIR/.env.local.$CHAIN"
else
    SOURCE_ENV_FILE="$PROJECT_DIR/.env.$CHAIN"
fi
# MODE follows SIMULATED_TEE so the tooling and the container cannot disagree.
[[ "$SIMULATED_TEE" == "true" ]] && MODE="${MODE:-1}" || MODE="${MODE:-0}"
export MODE
log "Local siblings: $USE_LOCAL_SIBLINGS"
log "tee-node ref:   $TEE_NODE_REF"

# --- Manage go.work for host builds (git-ignored, fully managed here) ---
# Toggle on: host `go` builds (e.g. --local mode) resolve the on-disk sibling
# tee-node / tee-proxy. Toggle off: remove any stale go.work so host builds fall
# back to the pinned module versions in go.mod / tools/go.mod.
GOWORK_FILE="$PROJECT_DIR/go.work"
if [[ "$USE_LOCAL_SIBLINGS" == "true" ]]; then
    SIBLINGS_ROOT="$(cd "$PROJECT_DIR/../.." && pwd)"
    for sib in tee-node tee-proxy; do
        [[ -d "$SIBLINGS_ROOT/$sib" ]] || die "USE_LOCAL_SIBLINGS set but $SIBLINGS_ROOT/$sib is missing.\n  Clone it into $SIBLINGS_ROOT/, or unset USE_LOCAL_SIBLINGS to use pinned modules."
    done
    log "Writing $GOWORK_FILE (host builds use sibling tee-node / tee-proxy)"
    GO_VERSION=$(grep '^go ' "$PROJECT_DIR/go/go.mod" | awk '{print $2}')
    cat > "$GOWORK_FILE" <<EOF
// Generated by scripts/start-services.sh for USE_LOCAL_SIBLINGS (git-ignored).
// Points host \`go\` builds at the on-disk sibling tee-node / tee-proxy checkouts.
// Re-run start-services.sh without USE_LOCAL_SIBLINGS (or delete this file) to
// return to the pinned module versions in go/go.mod / tools/go.mod.
go $GO_VERSION

use (
	./go
	./tools
	../../tee-node
	../../tee-proxy
)
EOF
elif [[ -f "$GOWORK_FILE" ]]; then
    log "Removing stale $GOWORK_FILE — host builds use pinned modules"
    rm -f "$GOWORK_FILE" "$PROJECT_DIR/go.work.sum"
fi

# ============================================================
# Docker Compose mode (default)
# ============================================================
# Synced before the other containers so EXT_PROXY_URL is public by the time
# post-build.sh registers it. Simulated mode owns the tunnel; live mode never touches it.
TUNNEL_ACTIVE=false
sync_tunnel() {
    local cf_compose="$PROJECT_DIR/docker-compose.cloudflared.yaml"
    local -a compose=(docker compose -f "$cf_compose")
    [[ -f "$cf_compose" ]] || die "$cf_compose not found — see docs/cloudflared.md"

    if [[ "$USE_LOCAL" == "true" ]]; then
        # Host Go proxy is on 6664: a different origin would recreate the shared
        # tunnel and rotate everyone's URL, so local mode gets its own project.
        compose=(docker compose -p tunnel-local -f "$cf_compose")
        export TUNNEL_TARGET="http://host.docker.internal:6664"
    elif [[ -n "${TUNNEL_TARGET:-}" ]]; then
        log "NOTE: TUNNEL_TARGET is set — this recreates the shared 'tunnel' container."
    fi

    if "${compose[@]}" ps -q cloudflared 2>/dev/null | grep -q .; then
        log "Cloudflare tunnel already running — reusing it."
    else
        log "Starting the Cloudflare tunnel (first, before the stack)..."
        "${compose[@]}" up -d || die "Failed to start cloudflared"
    fi
    TUNNEL_ACTIVE=true

    # Named tunnel has a fixed hostname already in .env — nothing to discover.
    if [[ -n "${TUNNEL_ARGS:-}" ]]; then
        log "Named tunnel up — keeping EXT_PROXY_URL=${EXT_PROXY_URL:-<unset>}"
        return
    fi

    # A restarted container keeps its old logs, so scan only from this start.
    local cid started
    cid=$("${compose[@]}" ps -q cloudflared 2>/dev/null | head -1 || true)
    started=$(docker inspect -f '{{.State.StartedAt}}' "$cid" 2>/dev/null | cut -c1-19 || true)

    log "Reading the quick-tunnel URL..."
    local url="" i
    for ((i = 0; i < 30; i++)); do
        # `|| true` is load-bearing: grep exits 1 until the URL appears.
        if [[ -n "$started" ]]; then
            url=$("${compose[@]}" logs --since "${started}Z" cloudflared 2>/dev/null \
                  | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' | tail -1 || true)
        else
            url=$("${compose[@]}" logs cloudflared 2>/dev/null \
                  | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' | tail -1 || true)
        fi
        [[ -n "$url" ]] && break
        sleep 1
    done
    [[ -n "$url" ]] || die "cloudflared printed no *.trycloudflare.com URL within 30s.\n  Check: ${compose[*]} logs cloudflared"

    export EXT_PROXY_URL="$url"
    # post-build.sh and test.sh re-source .env, so the file is how the URL reaches
    # them; the per-chain source is updated too so use-chain.sh keeps it.
    write_proxy_url "$PROJECT_DIR/.env" "$url"
    [[ -n "${SOURCE_ENV_FILE:-}" && -f "${SOURCE_ENV_FILE:-}" ]] && write_proxy_url "$SOURCE_ENV_FILE" "$url"
    log "Tunnel URL: $url  (written to .env)"
}

write_proxy_url() {
    local f="$1" url="$2"
    [[ -f "$f" ]] || return 0
    if grep -q '^EXT_PROXY_URL=' "$f"; then
        sed -i.bak "s|^EXT_PROXY_URL=.*|EXT_PROXY_URL=$url|" "$f"
        rm -f "$f.bak"
    else
        printf 'EXT_PROXY_URL=%s\n' "$url" >> "$f"
    fi
}

# Simulated mode owns the tunnel; live mode only starts one when asked with --tunnel.
if [[ "$CHAIN" == "local" && "$USE_TUNNEL" != "true" ]]; then
    log "Local devnet — no tunnel; the proxy is reachable on localhost."
elif [[ "$SIMULATED_TEE" == "true" || "$USE_TUNNEL" == "true" ]]; then
    sync_tunnel
else
    log "Live mode — not starting the tunnel (pass --tunnel to force one); EXT_PROXY_URL must already be reachable."
fi

if [[ "$USE_LOCAL" == "false" ]]; then
    # A plain container cannot attest, so MODE=0 here is a config error.
    [[ "$SIMULATED_TEE" == "true" ]] || die "SIMULATED_TEE=false with the local Docker stack: the container runs MODE=0 and cannot attest.
  For a local stack:   ./scripts/use-chain.sh $CHAIN --simulated
  For a devops TEE:    don't run start-services.sh — point EXT_PROXY_URL at their proxy and run post-build.sh"

    log "Starting services with Docker Compose..."

    # Dockerfile expects SOURCE_DATE_EPOCH for reproducible builds — see REPRODUCIBILITY.md.
    # Without it, `touch -h -d @${SOURCE_DATE_EPOCH}` in the builder stage fails with "invalid date format '@'".
    if [[ -z "${SOURCE_DATE_EPOCH:-}" ]]; then
        if SOURCE_DATE_EPOCH=$(git -C "$PROJECT_DIR" log -1 --format=%ct 2>/dev/null) && [[ -n "$SOURCE_DATE_EPOCH" ]]; then
            export SOURCE_DATE_EPOCH
        else
            export SOURCE_DATE_EPOCH=0
        fi
    fi
    log "SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH"

    # --- Build the shared tee-node base image if this language needs it ---
    # Two-process languages run a prebuilt tee-node `server` binary; the Go
    # image links tee-node as a library and needs nothing here.
    if grep -q 'tee-node-base' "$PROJECT_DIR/$EXTENSION_DOCKERFILE" 2>/dev/null; then
        log "Language '$LANGUAGE' needs the shared tee-node base image"
        "$SCRIPT_DIR/build-node-base.sh" || die "failed to build tee-node base image"
    fi

    # --- Build tee-proxy image locally if no remote registry is configured ---
    if [[ -z "${REGISTRY:-}" ]]; then
        if ! docker image inspect local/tee-proxy >/dev/null 2>&1; then
            if [[ "$USE_LOCAL_SIBLINGS" == "true" ]]; then
                # Dev build: use the on-disk sibling tee-proxy checkout.
                TEE_PROXY_DIR="$SIBLINGS_ROOT/tee-proxy"
                [[ -d "$TEE_PROXY_DIR" ]] || die "USE_LOCAL_SIBLINGS set but tee-proxy repo not present at $TEE_PROXY_DIR.\n  Clone it into $SIBLINGS_ROOT/, or unset USE_LOCAL_SIBLINGS to build the pinned proxy."
                log "Building local/tee-proxy from sibling checkout $TEE_PROXY_DIR (USE_LOCAL_SIBLINGS)..."
                docker build -f "$TEE_PROXY_DIR/Dockerfile" -t local/tee-proxy "$TEE_PROXY_DIR" || die "Failed to build tee-proxy image"
            else
                # Default: pinned, self-contained build from the in-repo Dockerfile
                # (clones tee-proxy at the version in proxy/Dockerfile, matching tools/go.mod).
                PROXY_DOCKERFILE="$PROJECT_DIR/proxy/Dockerfile"
                [[ -f "$PROXY_DOCKERFILE" ]] || die "Image local/tee-proxy not found and proxy Dockerfile missing at $PROXY_DOCKERFILE.\n  Set REGISTRY in .env to pull from a remote registry, restore proxy/Dockerfile, or use USE_LOCAL_SIBLINGS=1 to build from a sibling checkout."
                log "Building local/tee-proxy from $PROXY_DOCKERFILE (pinned, self-cloning)..."
                docker build -f "$PROXY_DOCKERFILE" -t local/tee-proxy "$PROJECT_DIR/proxy" || die "Failed to build tee-proxy image"
            fi
            log "local/tee-proxy image built successfully"
        else
            log "local/tee-proxy image already exists (use 'docker rmi local/tee-proxy' to force rebuild)"
        fi
    fi

    COMPOSE_FILES=("-f" "$PROJECT_DIR/docker-compose.yaml")

    # Toggle: swap the pinned self-contained node build for the sibling-based one.
    if [[ "$USE_LOCAL_SIBLINGS" == "true" ]]; then
        log "USE_LOCAL_SIBLINGS — node built from on-disk tee-node via Dockerfile.siblings"
        COMPOSE_FILES+=("-f" "$PROJECT_DIR/docker-compose.siblings.yaml")
    fi

    if [[ "$CHAIN" != "local" ]]; then
        override="$PROJECT_DIR/docker-compose.$CHAIN.yaml"
        [[ -f "$override" ]] || die "$override not found — run ./scripts/use-chain.sh $CHAIN to generate it"
        log "$CHAIN mode — attaching docker-compose.$CHAIN.yaml"
        COMPOSE_FILES+=("-f" "$override")
    fi

    # Compose bind-mounts this as the proxy's config.toml; if missing, docker
    # makes a directory there and `up` dies with an opaque rootfs error.
    if [[ "$CHAIN" == "local" ]]; then
        PROXY_CFG="extension_proxy.docker.toml"
    else
        PROXY_CFG="extension_proxy.$CHAIN.docker.toml"
    fi
    if [[ -d "$PROJECT_DIR/config/proxy/$PROXY_CFG" ]]; then
        die "config/proxy/$PROXY_CFG is a directory — docker created it on an earlier run when the config was missing.\n  rm -rf config/proxy/$PROXY_CFG && cp config/proxy/$PROXY_CFG.example config/proxy/$PROXY_CFG"
    elif [[ ! -f "$PROJECT_DIR/config/proxy/$PROXY_CFG" ]]; then
        die "config/proxy/$PROXY_CFG not found (it is gitignored — a fresh clone only has the .example).\n  cp config/proxy/$PROXY_CFG.example config/proxy/$PROXY_CFG   # then fill in the [db] credentials"
    fi

    # --force-recreate is load-bearing: ext-proxy pins the node's teeId once at
    # startup (result.SetIdentity is set-once) and rejects results from a new one.
    docker compose "${COMPOSE_FILES[@]}" up -d --build --force-recreate || die "docker compose up failed"

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
    if [[ "$CHAIN" == "local" ]]; then
        echo "  Local devnet"
    else
        echo "  $CHAIN testnet (chain_id=$(chain_id_for "$CHAIN"))"
    fi
    echo ""
    echo -e "${CYAN}Services${NC}"
    echo "  redis, ext-proxy, extension-tee"
    [[ "$TUNNEL_ACTIVE" == "true" ]] && echo "  cloudflared (tunnel)"
    echo "  Proxy URL: $EXT_PROXY_URL"
    echo ""
    echo -e "${CYAN}Commands${NC}"
    echo "  Logs:    docker compose ${COMPOSE_FILES[*]} logs -f"
    echo "  Stop:    ./scripts/stop-services.sh --chain $CHAIN"
    # The tunnel survives stop-services.sh on purpose — killing it rotates the URL.
    [[ "$TUNNEL_ACTIVE" == "true" ]] && \
        echo "  Stop tunnel too: ./scripts/stop-services.sh --chain $CHAIN --tunnel"
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
# start-tee links the Go extension in-process, so it lives in the extension
# module; start-proxy is deployment tooling and lives in tools/.
cd "$EXTENSION_DIR"
go build -o "$BIN_DIR/start-tee" ./cmd/start-tee
cd "$PROJECT_DIR/tools"
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
