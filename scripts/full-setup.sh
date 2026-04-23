#!/usr/bin/env bash
# full-setup.sh — Run the complete extension lifecycle: pre-build → start → post-build → test.
#
# Usage:
#   ./scripts/full-setup.sh                  # setup only (steps 1-3), docker compose
#   ./scripts/full-setup.sh --test           # setup + run e2e test (steps 1-4)
#   ./scripts/full-setup.sh --local          # start services as Go processes instead of docker
#   ./scripts/full-setup.sh --local --test   # both
#
# Prerequisites:
#   - Infrastructure running (Hardhat, indexer, Redis, normal TEE + proxy)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[full-setup]${NC} $*"; }
die()  { echo -e "${RED}[full-setup] ERROR:${NC} $*" >&2; exit 1; }

RUN_TESTS=false
USE_LOCAL=false
for arg in "$@"; do
    case "$arg" in
        --test) RUN_TESTS=true ;;
        --local) USE_LOCAL=true ;;
        *) die "Unknown argument: $arg" ;;
    esac
done

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
if [[ "$USE_LOCAL" == "true" ]]; then
    "$SCRIPT_DIR/start-services.sh" --local || die "Failed to start services"
else
    "$SCRIPT_DIR/start-services.sh" || die "Failed to start services"
fi

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
if [[ "$USE_LOCAL" == "true" ]]; then
    echo -e "${CYAN}Stop services:${NC}  ./scripts/stop-services.sh --local"
else
    echo -e "${CYAN}Stop services:${NC}  ./scripts/stop-services.sh"
fi
