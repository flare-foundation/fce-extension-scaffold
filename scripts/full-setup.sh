#!/usr/bin/env bash
# full-setup.sh — Run the complete extension lifecycle: pre-build → start → post-build → test.
#
# Usage:
#   ./scripts/full-setup.sh                          # setup only, docker compose, local devnet
#   ./scripts/full-setup.sh --test                   # setup + run e2e test
#   ./scripts/full-setup.sh --local                  # start services as Go processes instead of docker
#   ./scripts/full-setup.sh --chain coston           # target Coston testnet (chain_id=16)
#   ./scripts/full-setup.sh --chain coston2 --test   # target Coston2 testnet + run tests
#
# Flags:
#   --chain <name>   local | coston | coston2 (default: local)
#   --local          run TEE + proxy as background Go processes instead of Docker
#   --test           run scripts/test.sh after setup
#
# Prerequisites:
#   - Local devnet: Hardhat, indexer, Redis, normal TEE + proxy running.
#   - Coston/Coston2: a reachable chain indexer DB. For Coston, bring up
#     e2e/docker/coston (coston-indexer-db + coston-indexer) before this script.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[full-setup]${NC} $*"; }
warn() { echo -e "${YELLOW}[full-setup]${NC} $*"; }
die()  { echo -e "${RED}[full-setup] ERROR:${NC} $*" >&2; exit 1; }

RUN_TESTS=false
USE_LOCAL=false
CHAIN=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --test) RUN_TESTS=true; shift ;;
        --local) USE_LOCAL=true; shift ;;
        --chain) [[ $# -ge 2 ]] || die "--chain requires a value (local|coston|coston2)"
                 CHAIN="$2"; shift 2 ;;
        --chain=*) CHAIN="${1#--chain=}"; shift ;;
        *) die "Unknown argument: $1" ;;
    esac
done

# --- Resolve chain (flag > env > legacy LOCAL_MODE) ---
# Load .env first so user-set defaults are visible
if [[ -f "$PROJECT_DIR/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "$PROJECT_DIR/.env"
    set +a
fi

if [[ -z "$CHAIN" ]]; then
    if [[ -n "${CHAIN:-}" ]]; then
        :  # honor env CHAIN
    elif [[ "${LOCAL_MODE:-true}" == "true" ]]; then
        CHAIN="local"
    else
        # Legacy default: LOCAL_MODE=false meant Coston2
        CHAIN="coston2"
    fi
fi

# --- Apply chain defaults (don't clobber explicit env settings) ---
case "$CHAIN" in
    local)
        export LOCAL_MODE="true"
        ;;
    coston)
        export LOCAL_MODE="false"
        export ADDRESSES_FILE="${ADDRESSES_FILE:-$PROJECT_DIR/config/coston/deployed-addresses.json}"
        export CHAIN_URL="${CHAIN_URL:-https://coston-api.flare.network/ext/C/rpc}"
        export NORMAL_PROXY_URL="${NORMAL_PROXY_URL:-https://tee-proxy-coston-1.flare.rocks}"
        ;;
    coston2)
        export LOCAL_MODE="false"
        export ADDRESSES_FILE="${ADDRESSES_FILE:-$PROJECT_DIR/config/coston2/deployed-addresses.json}"
        export CHAIN_URL="${CHAIN_URL:-https://coston2-api.flare.network/ext/C/rpc}"
        export NORMAL_PROXY_URL="${NORMAL_PROXY_URL:-https://tee-proxy-coston2-1.flare.rocks}"
        ;;
    *) die "Unknown --chain value: $CHAIN (valid: local, coston, coston2)" ;;
esac
export CHAIN

log "Chain: $CHAIN"

# --- Indexer reachability hint for testnets ---
if [[ "$CHAIN" == "coston" ]]; then
    if ! (echo > /dev/tcp/127.0.0.1/3306) >/dev/null 2>&1; then
        warn "No service on 127.0.0.1:3306 — the ext-proxy needs a Coston indexer DB."
        warn "Start it with: (cd ../../e2e/docker/coston && docker compose up -d coston-indexer-db coston-indexer)"
    fi
fi

# --- Phase 1: Pre-build (deploy contract, register extension) ---
echo -e "\n${CYAN}╔══════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  Phase 1: Pre-build                  ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
"$SCRIPT_DIR/pre-build.sh" || die "Pre-build failed"

# --- Phase 1.5: Extension setup (optional hook for extension-specific config) ---
if [[ -x "$SCRIPT_DIR/extension-setup.sh" ]]; then
    echo -e "\n${CYAN}╔══════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  Phase 1.5: Extension setup          ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
    "$SCRIPT_DIR/extension-setup.sh" || die "Extension setup failed"
fi

# --- Phase 2: Start services (TEE node + proxy) ---
echo -e "\n${CYAN}╔══════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  Phase 2: Start services             ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
START_ARGS=(--chain "$CHAIN")
[[ "$USE_LOCAL" == "true" ]] && START_ARGS+=(--local)
"$SCRIPT_DIR/start-services.sh" "${START_ARGS[@]}" || die "Failed to start services"

# --- Phase 3: Post-build (register TEE version + machine) ---
echo -e "\n${CYAN}╔══════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  Phase 3: Post-build                 ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
"$SCRIPT_DIR/post-build.sh" || die "Post-build failed"

# --- Phase 3.5: Extension post-setup (optional hook for post-registration config) ---
if [[ -x "$SCRIPT_DIR/extension-post-setup.sh" ]]; then
    echo -e "\n${CYAN}╔══════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  Phase 3.5: Extension post-setup     ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
    "$SCRIPT_DIR/extension-post-setup.sh" || die "Extension post-setup failed"
fi

# --- Phase 4: Test (optional) ---
if [[ "$RUN_TESTS" == "true" ]]; then
    echo -e "\n${CYAN}╔══════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  Phase 4: Test                       ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
    "$SCRIPT_DIR/test.sh" || die "Tests failed"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN} Full setup complete${NC}"
if [[ "$RUN_TESTS" == "true" ]]; then
    echo -e "${GREEN} (including tests)${NC}"
fi
echo -e "${GREEN}========================================${NC}"
echo ""
STOP_HINT="./scripts/stop-services.sh --chain $CHAIN"
[[ "$USE_LOCAL" == "true" ]] && STOP_HINT="$STOP_HINT --local"
echo -e "${CYAN}Stop services:${NC}  $STOP_HINT"
