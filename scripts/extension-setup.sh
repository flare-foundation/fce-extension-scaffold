#!/usr/bin/env bash
# extension-setup.sh — Extension-specific setup that runs BEFORE Docker Compose.
#
# This hook runs between pre-build (contract deployment) and docker-up (starting
# the TEE). Use it for any setup whose output the extension needs at startup —
# for example:
#   - Deploying auxiliary contracts (ERC20 tokens, oracles, vaults)
#   - Writing config files the extension reads at init (pairs, feeds, pools)
#   - Minting test tokens, setting allowances, seeding initial state
#
# The following variables are available (sourced from .env + config/extension.env):
#
#   INSTRUCTION_SENDER   — your deployed InstructionSender contract address
#   EXTENSION_ID         — your extension's ID on the TeeExtensionRegistry
#   CHAIN_URL            — chain RPC endpoint
#   ADDRESSES_FILE       — path to deployed-addresses.json
#   DEPLOYMENT_PRIVATE_KEY — funded deployer key
#
# Example: deploy a helper contract and write its address to a config file
#
#   cd "$PROJECT_DIR/tools"
#   HELPER_ADDR=$(go run ./cmd/deploy-helper -a "$ADDRESSES_FILE" -c "$CHAIN_URL")
#   echo "HELPER_CONTRACT=$HELPER_ADDR" > "$PROJECT_DIR/config/helper.env"
#
# Why this matters:
#   The extension container reads config at startup. Anything it needs must be
#   written to disk BEFORE `docker compose up`. If you deploy contracts after
#   the container starts, you'll need to restart it — this hook prevents that.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

GREEN='\033[0;32m'; NC='\033[0m'
log() { echo -e "${GREEN}[extension-setup]${NC} $*"; }

# --- Load environment ---
if [[ -f "$PROJECT_DIR/.env" ]]; then
    set -a; source "$PROJECT_DIR/.env"; set +a
fi
if [[ -f "$PROJECT_DIR/config/extension.env" ]]; then
    source "$PROJECT_DIR/config/extension.env"
fi

log "EXTENSION_ID:       ${EXTENSION_ID:-<not set>}"
log "INSTRUCTION_SENDER: ${INSTRUCTION_SENDER:-<not set>}"
log "CHAIN_URL:          ${CHAIN_URL:-<not set>}"

# --- Add your extension-specific setup below ---
# The Hello World scaffold has no extra setup. Replace this with your own logic.

log "No extension-specific setup needed (Hello World scaffold)."
