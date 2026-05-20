#!/usr/bin/env bash
#
# use-chain.sh — Activate a chain-specific .env file.
#
# Reads .env.<chain> from the project root and copies it to .env. All
# deployment scripts (pre-build, post-build, test, full-setup) auto-load .env.
#
# Usage:
#   ./scripts/use-chain.sh <chain>       Copy .env.<chain> → .env (e.g. coston).
#   ./scripts/use-chain.sh --list        List available .env.<chain> files.
#   ./scripts/use-chain.sh -h | --help   Show this help.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SCRIPT_NAME="$(basename "$0")"

log() { echo "[use-chain] $*"; }
die() { echo "[use-chain] ERROR: $*" >&2; exit 1; }

usage() {
    cat <<EOF
use-chain.sh — Activate a chain-specific .env file.

Usage:
  $SCRIPT_NAME <chain>       Copy .env.<chain> → .env (e.g. coston, coston2).
  $SCRIPT_NAME --list        List available .env.<chain> files.
  $SCRIPT_NAME -h | --help   Show this help.

The active .env is auto-loaded by every deployment script in scripts/.
EOF
}

# Discover .env.<chain> files in the project root, skipping helper files.
list_chains() {
    local any=0 f name
    for f in "$PROJECT_DIR"/.env.*; do
        [[ -f "$f" ]] || continue
        name="${f##*/.env.}"
        case "$name" in example|local|template|sample) continue ;; esac
        echo "  $name"
        any=1
    done
    [[ $any -eq 1 ]] || echo "  (none — create .env.<chain> from .env.example)"
}

# Extract a value from $DST for the given KEY:
#   - strip the "KEY=" prefix
#   - strip trailing " # comment"
#   - trim trailing whitespace
#   - strip surrounding double quotes
get_val() {
    grep -E "^$1=" "$DST" 2>/dev/null | head -1 \
        | sed -E "s/^[^=]*=//; s/[[:space:]]+#.*$//; s/[[:space:]]+$//" \
        | tr -d '"'
}

# --- Argument parsing ---

[[ $# -ge 1 ]] || { usage; exit 1; }
case "$1" in
    -h|--help) usage; exit 0 ;;
    --list)    log "available chains:"; list_chains; exit 0 ;;
    -*)        die "unknown flag: $1 (try --help)" ;;
esac
[[ $# -eq 1 ]] || die "expected one chain name (got $#); try --help"

CHAIN="$1"
SRC="$PROJECT_DIR/.env.$CHAIN"
DST="$PROJECT_DIR/.env"

if [[ ! -f "$SRC" ]]; then
    log "no .env.$CHAIN in $PROJECT_DIR"
    log "available chains:"
    list_chains
    exit 1
fi

# --- Swap ---

cp "$SRC" "$DST" || die "failed to copy $SRC → $DST"
log "active chain: $CHAIN"

# --- Summary ---

key_val=$(get_val DEPLOYMENT_PRIVATE_KEY)
log "  CHAIN_URL              = $(v=$(get_val CHAIN_URL);        echo "${v:-<unset>}")"
log "  EXT_PROXY_URL          = $(v=$(get_val EXT_PROXY_URL);    echo "${v:-<unset>}")"
log "  NORMAL_PROXY_URL       = $(v=$(get_val NORMAL_PROXY_URL); echo "${v:-<unset>}")"
log "  INITIAL_OWNER          = $(v=$(get_val INITIAL_OWNER);    echo "${v:-<unset>}")"
log "  DEPLOYMENT_PRIVATE_KEY = $([[ -n "$key_val" ]] && echo "<set>" || echo "<unset>")"
