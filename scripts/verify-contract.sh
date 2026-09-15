#!/usr/bin/env bash
# verify-contract.sh — Verify the deployed InstructionSender's source on the
# chain's block explorer (Blockscout).
#
# Inputs:
#   $1 (optional) — chain name (local | coston | coston2 | songbird | flare)
#   config/<chain>/extension.env — INSTRUCTION_SENDER (from pre-build.sh)
#   config/<chain>/deploy.log    — FlareTeeManager address (constructor args)
#
# No-op on local (no explorer to verify against). Never fails full-setup.sh —
# explorer indexing lag or a transient API error shouldn't block the pipeline.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/chain-env.sh"

GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[verify-contract]${NC} $*"; }
warn() { echo -e "${YELLOW}[verify-contract]${NC} $*"; }

CONTRACT_NAME="HelloWorldInstructionSender"
CONTRACT_PATH="contracts/InstructionSender.sol:${CONTRACT_NAME}"

if [[ -f "$PROJECT_DIR/.env" ]]; then
    set -a
    source "$PROJECT_DIR/.env"
    set +a
fi

resolve_chain_default "${1:-}"

if [[ "$CHAIN" == "local" ]]; then
    log "Chain is local — no explorer to verify against, skipping."
    exit 0
fi

EXPLORER_URL="$(chain_explorer_for "$CHAIN")" || { warn "Unknown chain '$CHAIN', skipping."; exit 0; }
CHAIN_ID="$(chain_id_for "$CHAIN")"

EXTENSION_ENV="$(extension_env_path "$CHAIN")"
DEPLOY_LOG="$CHAIN_CONFIG_DIR/deploy.log"

[[ -f "$EXTENSION_ENV" ]] || { warn "$EXTENSION_ENV not found — run pre-build.sh first. Skipping."; exit 0; }
[[ -f "$DEPLOY_LOG" ]] || { warn "$DEPLOY_LOG not found — run pre-build.sh first. Skipping."; exit 0; }

# shellcheck disable=SC1090
source "$EXTENSION_ENV"
[[ -n "${INSTRUCTION_SENDER:-}" ]] || { warn "INSTRUCTION_SENDER not set in $EXTENSION_ENV. Skipping."; exit 0; }

# Constructor takes (teeExtensionRegistry, teeMachineRegistry) — both the
# FlareTeeManager diamond, per tools/pkg/utils/instructions.go.
FLARE_TEE_MANAGER="$(grep -oE 'FlareTeeManager:\s*0x[0-9a-fA-F]{40}' "$DEPLOY_LOG" | tail -1 | grep -oE '0x[0-9a-fA-F]{40}')"
[[ -n "$FLARE_TEE_MANAGER" ]] || { warn "Could not find FlareTeeManager address in $DEPLOY_LOG. Skipping."; exit 0; }

echo -e "\n${CYAN}=== Verifying $CONTRACT_NAME on $EXPLORER_URL ===${NC}"
log "Address:         $INSTRUCTION_SENDER"
log "Constructor args: ($FLARE_TEE_MANAGER, $FLARE_TEE_MANAGER)"

CONSTRUCTOR_ARGS="$(cast abi-encode "constructor(address,address)" "$FLARE_TEE_MANAGER" "$FLARE_TEE_MANAGER")"

cd "$PROJECT_DIR"
if forge verify-contract "$INSTRUCTION_SENDER" "$CONTRACT_PATH" \
    --chain-id "$CHAIN_ID" \
    --verifier blockscout \
    --verifier-url "${EXPLORER_URL}/api" \
    --constructor-args "$CONSTRUCTOR_ARGS" \
    --watch 2>&1 | tee "$CHAIN_CONFIG_DIR/verify-contract.log"; then
    log "Verified: ${EXPLORER_URL}/address/${INSTRUCTION_SENDER}?tab=contract"
else
    warn "Verification failed or already verified — see $CHAIN_CONFIG_DIR/verify-contract.log"
    warn "Not fatal — continuing."
fi

exit 0
