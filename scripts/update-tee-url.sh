#!/usr/bin/env bash
# update-tee-url.sh — repoint the on-chain TEE machine record at a new proxy URL.
#
# Run this whenever the public tunnel URL changes (ngrok / cloudflared restart):
# Flare's TEE infrastructure delivers instructions to the URL registered in the
# MachineManager, so a dead tunnel means every on-chain op (deposit, withdraw
# request, session-key bind) silently times out at pollAction even though the
# chain leg succeeded. post-build.sh CANNOT fix this — its pre-registration
# step (the only writer of the URL) is skipped once the machine exists, and a
# full re-registration trips ChallengeExpired on a long-running TEE. This calls
# MachineManager.updateTeeMachineSettings directly instead — no attestation,
# one transaction, ledger untouched.
#
# Usage: ./scripts/update-tee-url.sh [new-url]
#        (default: EXT_PROXY_URL from .env — update that first, then run this)
#
# Requires: foundry (cast), python, curl; DEPLOYMENT_PRIVATE_KEY + CHAIN_URL
# in .env; the local ext-proxy running (its /info supplies the TEE pubkey).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

set -a; source "$PROJECT_DIR/.env"; set +a

URL="${1:-${EXT_PROXY_URL:?EXT_PROXY_URL not set}}"
RPC="${CHAIN_URL:?CHAIN_URL not set}"
KEY="0x${DEPLOYMENT_PRIVATE_KEY#0x}"
ADDRESSES_FILE="${ADDRESSES_FILE:-./config/coston2/deployed-addresses.json}"
[[ "$ADDRESSES_FILE" != /* ]] && ADDRESSES_FILE="$PROJECT_DIR/$ADDRESSES_FILE"
LOCAL_INFO="${LOCAL_INFO_URL:-http://localhost:6674/info}"

FTM=$(python -c "
import json, sys
entries = json.load(open(sys.argv[1]))
print(next(e['address'] for e in entries if e['name'] == 'FlareTeeManager'))
" "$ADDRESSES_FILE")

# teeId = the EVM address of the TEE's secp256k1 pubkey (keccak(X||Y)[12:]).
XY=$(curl -sf -m 10 "$LOCAL_INFO" | python -c "
import json,sys
k = json.load(sys.stdin)['teeInfo']['publicKey']
print(k['x'][2:] + k['y'][2:])
")
TEE_ID=0x$(cast keccak "0x$XY" | tail -c 41)

CURRENT=$(cast call "$FTM" "getTeeMachine(address)((address,address,string))" "$TEE_ID" --rpc-url "$RPC")
PROXY_ID=$(echo "$CURRENT" | sed -E 's/^\([^,]+, ([^,]+),.*$/\1/')
echo "machine:  $TEE_ID"
echo "current:  $CURRENT"
echo "new url:  $URL"

cast send "$FTM" "updateTeeMachineSettings(address,address,string)" \
    "$TEE_ID" "$PROXY_ID" "$URL" \
    --private-key "$KEY" --rpc-url "$RPC" > /dev/null

cast call "$FTM" "getTeeMachine(address)((address,address,string))" "$TEE_ID" --rpc-url "$RPC" \
    | grep -qF "$URL" && echo "OK — on-chain machine URL updated." \
    || { echo "ERROR: on-chain URL does not match after update" >&2; exit 1; }
