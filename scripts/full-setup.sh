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
#   --tunnel         also start the shared cloudflared tunnel (project "tunnel")
#   --test           run scripts/test.sh after setup
#
# Prerequisites:
#   - Local devnet: Hardhat, indexer, Redis, normal TEE + proxy running.
#   - Coston/Coston2: a reachable chain indexer DB. For Coston, bring up
#     e2e/docker/coston (coston-indexer-db + coston-indexer) before this script.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/chain-env.sh"   # chain table + resolve_chain

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[full-setup]${NC} $*"; }
warn() { echo -e "${YELLOW}[full-setup]${NC} $*"; }
die()  { echo -e "${RED}[full-setup] ERROR:${NC} $*" >&2; exit 1; }

RUN_TESTS=false
USE_LOCAL=false
USE_TUNNEL=false
CHAIN=""
CHAIN_FLAG=""
usage() {
    cat <<USAGE
full-setup.sh — Run the whole lifecycle: pre-build, start, post-build, test.

Usage: ./scripts/full-setup.sh [flags]

Flags:
  --chain <name>   local | coston | coston2 | songbird | flare (default: local)
  --test           run the e2e test after setup
  --local          run TEE + proxy as background Go processes instead of Docker
  --tunnel         start the shared Cloudflare tunnel even in live mode
  -h, --help       this message
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help) usage; exit 0 ;;
        --test) RUN_TESTS=true; shift ;;
        --local) USE_LOCAL=true; shift ;;
        --tunnel) USE_TUNNEL=true; shift ;;
        --chain) [[ $# -ge 2 ]] || die "--chain requires a value (${CHAINS// /|})"
                 CHAIN_FLAG="$2"; shift 2 ;;
        --chain=*) CHAIN_FLAG="${1#--chain=}"; shift ;;
        *) die "Unknown argument: $1" ;;
    esac
done

# Validate before anything with a side effect — the .env copy below is one.
CHAIN="$CHAIN_FLAG"
[[ -z "$CHAIN" ]] || valid_chain "$CHAIN"     || die "Unknown --chain value: $CHAIN (valid: ${CHAINS// /, })"

# --- Auto-activate chain-specific env when --chain was passed explicitly ---
# If --chain <name> was given and a .env.<name> exists, activate it (copy → .env)
# BEFORE loading, so --chain actually switches chains. Put per-chain values in
# .env.<name>; note this overwrites the active .env.
if [[ -n "$CHAIN" && -f "$PROJECT_DIR/.env.$CHAIN" ]]; then
    cp "$PROJECT_DIR/.env.$CHAIN" "$PROJECT_DIR/.env"
    log "Activated .env.$CHAIN → .env"
elif [[ -n "$CHAIN" && "$CHAIN" != "local" && ! -f "$PROJECT_DIR/.env.$CHAIN" ]]; then
    warn "No .env.$CHAIN found — using the existing .env as-is, which may target a different chain."
    warn "Run ./scripts/use-chain.sh $CHAIN to generate it (env, proxy toml, compose override)."
fi

# --- Resolve chain (flag > env > legacy LOCAL_MODE) ---
# Load .env first so user-set defaults are visible
if [[ -f "$PROJECT_DIR/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "$PROJECT_DIR/.env"
    set +a
fi

# Flag beats .env: sourcing .env above would otherwise clobber --chain.
CHAIN="${CHAIN_FLAG:-${CHAIN:-}}"
if [[ -z "$CHAIN" ]]; then
    if [[ "${LOCAL_MODE:-true}" == "true" ]]; then
        CHAIN="local"
    else
        # Legacy default: LOCAL_MODE=false meant Coston2
        CHAIN="coston2"
    fi
fi

# --- Apply chain defaults (don't clobber explicit env settings) ---
valid_chain "$CHAIN" || die "Unknown --chain value: $CHAIN (valid: ${CHAINS// /, })"
if [[ "$CHAIN" == "local" ]]; then
    export LOCAL_MODE="true"
else
    export LOCAL_MODE="false"
    export ADDRESSES_FILE="${ADDRESSES_FILE:-$PROJECT_DIR/config/$CHAIN/deployed-addresses.json}"
    export CHAIN_URL="${CHAIN_URL:-$(chain_rpc_for "$CHAIN")}"
    export NORMAL_PROXY_URL="${NORMAL_PROXY_URL:-https://tee-proxy-$CHAIN-1.flare.rocks}"
    export CHAIN_ID="${CHAIN_ID:-$(chain_id_for "$CHAIN")}"
fi
resolve_chain "$CHAIN"
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

# --- Phase 1.1: Verify contract source on the block explorer ---
# Non-fatal by design (see verify-contract.sh) — explorer indexing lag or a
# transient API error shouldn't block the rest of the pipeline.
if [[ "$CHAIN" != "local" ]]; then
    echo -e "\n${CYAN}╔══════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  Phase 1.1: Verify contract          ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
    "$SCRIPT_DIR/verify-contract.sh" "$CHAIN"
fi

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
[[ "$USE_TUNNEL" == "true" ]] && START_ARGS+=(--tunnel)
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
