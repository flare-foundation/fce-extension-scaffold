#!/usr/bin/env bash
# post-build.sh — Register TEE version and TEE machine on-chain.
#
# Run this AFTER Docker Compose brings up the extension TEE + proxy + Redis.
#
# Inputs (env vars):
#   EXT_PROXY_URL       — extension proxy URL (auto-detected: :6674 for Docker, :6664 for local)
#   NORMAL_PROXY_URL    — normal/FTDC proxy URL (default: http://localhost:6662)
#   CHAIN_URL           — chain RPC URL (default: http://127.0.0.1:8545)
#   ADDRESSES_FILE      — path to deployed-addresses.json (auto-detected if unset)
#   TEE_VERSION         — version string (default: v0.1.0)
#   LOCAL_MODE          — skip attestation (default: true)
#   WAIT_TIMEOUT        — service wait timeout in seconds (default: 120)
#   EXTENSION_OWNER_KEY — private key override for AddTeeVersion (optional)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[post-build]${NC} $*"; }
step() { echo -e "\n${CYAN}=== Step $1: $2 ===${NC}"; }
die()  { echo -e "${RED}[post-build] ERROR:${NC} $*" >&2; exit 1; }

# --- Load .env from project root (if present) ---
if [[ -f "$PROJECT_DIR/.env" ]]; then
    set -a
    source "$PROJECT_DIR/.env"
    set +a
fi

# Auto-detect proxy port: use Docker port (6674) if ext-proxy container is running,
# otherwise fall back to local Go process port (6664).
if [[ -z "${EXT_PROXY_URL:-}" ]]; then
    if docker compose -f "$PROJECT_DIR/docker-compose.yaml" ps ext-proxy --status running 2>/dev/null | grep -q ext-proxy; then
        EXT_PROXY_URL="http://localhost:6674"
    else
        EXT_PROXY_URL="http://localhost:6664"
    fi
fi
NORMAL_PROXY_URL="${NORMAL_PROXY_URL:-http://localhost:6662}"
CHAIN_URL="${CHAIN_URL:-http://127.0.0.1:8545}"
ADDRESSES_FILE="${ADDRESSES_FILE:-}"
# Resolve relative paths against PROJECT_DIR (not caller's cwd)
if [[ -n "$ADDRESSES_FILE" && "$ADDRESSES_FILE" != /* ]]; then
    ADDRESSES_FILE="$PROJECT_DIR/$ADDRESSES_FILE"
fi
TEE_VERSION="${TEE_VERSION:-v0.1.0}"
LOCAL_MODE="${LOCAL_MODE:-true}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-120}"

# --- Auto-detect addresses file ---
if [[ -z "$ADDRESSES_FILE" ]]; then
    if [[ "$LOCAL_MODE" != "true" ]]; then
        # Non-local mode: use coston2 deployed addresses
        candidate="$PROJECT_DIR/config/coston2/deployed-addresses.json"
        if [[ -f "$candidate" ]]; then
            ADDRESSES_FILE="$(cd "$(dirname "$candidate")" && pwd)/$(basename "$candidate")"
        fi
    fi

    # Fall back to sim_dump candidates (local devnet)
    if [[ -z "$ADDRESSES_FILE" ]]; then
        for candidate in \
            "$PROJECT_DIR/../../e2e/docker/sim_dump/deployed-addresses.json" \
            "$PROJECT_DIR/../docker/sim_dump/deployed-addresses.json" \
            "$PROJECT_DIR/../../docker/sim_dump/deployed-addresses.json" \
            "$PROJECT_DIR/../../../docker/sim_dump/deployed-addresses.json"; do
            if [[ -f "$candidate" ]]; then
                ADDRESSES_FILE="$(cd "$(dirname "$candidate")" && pwd)/$(basename "$candidate")"
                break
            fi
        done
    fi

    [[ -n "$ADDRESSES_FILE" ]] || die "Cannot find deployed-addresses.json. Set ADDRESSES_FILE."
fi

[[ -f "$ADDRESSES_FILE" ]] || die "Addresses file not found: $ADDRESSES_FILE"

# Resolve to absolute path so it works after cd into tools/
ADDRESSES_FILE="$(cd "$(dirname "$ADDRESSES_FILE")" && pwd)/$(basename "$ADDRESSES_FILE")"

log "Extension proxy: $EXT_PROXY_URL"
log "Normal proxy:    $NORMAL_PROXY_URL"
log "Chain URL:       $CHAIN_URL"
log "Addresses file:  $ADDRESSES_FILE"
log "TEE version:     $TEE_VERSION"
log "Local mode:      $LOCAL_MODE"

# --- wait_for_url: poll until a URL responds with HTTP 200 ---
wait_for_url() {
    local url="$1"
    local label="${2:-$url}"
    local timeout="${3:-$WAIT_TIMEOUT}"
    local interval=2
    local elapsed=0

    log "Waiting for $label ($url) ..."
    while ! curl -sf -o /dev/null "$url" 2>/dev/null; do
        elapsed=$((elapsed + interval))
        if [[ $elapsed -ge $timeout ]]; then
            die "Timed out after ${timeout}s waiting for $label ($url)"
        fi
        sleep "$interval"
    done
    log "$label is ready"
}

# --- Wait for services ---
wait_for_url "$EXT_PROXY_URL/info" "Extension proxy"
wait_for_url "$NORMAL_PROXY_URL/info" "Normal proxy"

# --- Step 1: Allow TEE version on extension ---
step 1 "Allow TEE version"
cd "$PROJECT_DIR/tools"

go run ./cmd/allow-tee-version \
    -a "$ADDRESSES_FILE" \
    -c "$CHAIN_URL" \
    -p "$EXT_PROXY_URL" \
    -version "$TEE_VERSION" \
    || die "Allow TEE version failed"

# Export SIMULATED_TEE for register-tee (controls attestation mode)
export SIMULATED_TEE="${SIMULATED_TEE:-true}"
log "Simulated TEE: $SIMULATED_TEE"

# --- Step 2: Register TEE on-chain ---
step 2 "Register TEE machine"
go run ./cmd/register-tee \
    -a "$ADDRESSES_FILE" \
    -c "$CHAIN_URL" \
    -p "$EXT_PROXY_URL" \
    -h "${EXT_PROXY_HOST_URL:-$EXT_PROXY_URL}" \
    -ep "$NORMAL_PROXY_URL" \
    -state "$PROJECT_DIR/config/register-tee.state" \
    || die "Register TEE failed"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN} Post-build complete${NC}"
echo -e "${GREEN}========================================${NC}"
