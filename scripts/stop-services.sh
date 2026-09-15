#!/usr/bin/env bash
#
# Stop extension services.
#
# By default, stops Docker Compose services, picking the compose overlay from
# --chain (or env CHAIN, or legacy LOCAL_MODE):
#   --chain local    → docker-compose.yaml only
#   --chain coston   → + docker-compose.coston.yaml
#   --chain coston2  → + docker-compose.coston2.yaml
#
# Pass --local to stop background Go processes instead.
#
# Pass --tunnel to also stop the shared Cloudflare tunnel (compose project
# "tunnel"), after everything behind it. Without it the tunnel is left running:
# other extensions reuse the same container, and stopping it rotates their URL.
#
# Usage:
#   ./scripts/stop-services.sh                       # local devnet, docker compose
#   ./scripts/stop-services.sh --chain coston        # Coston, docker compose
#   ./scripts/stop-services.sh --local               # background Go processes
#   ./scripts/stop-services.sh --chain coston2 --tunnel   # also stop the tunnel
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/chain-env.sh"   # chain table + resolve_chain

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
log()  { echo -e "${GREEN}[stop-services]${NC} $*"; }
die()  { echo -e "${RED}[stop-services] ERROR:${NC} $*" >&2; exit 1; }

# --- Parse flags ---
USE_LOCAL=false
USE_TUNNEL=false
CHAIN_FLAG=""
usage() {
    cat <<USAGE
stop-services.sh — Stop the extension services.

Usage: ./scripts/stop-services.sh [flags]

Flags:
  --chain <name>   local | coston | coston2 | songbird | flare (default: from .env)
  --local          stop background Go processes instead of Docker
  --tunnel         also stop the shared Cloudflare tunnel
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

if [[ "$USE_LOCAL" == "true" ]]; then
    # --- Stop background Go processes ---
    E2E="$SCRIPT_DIR/e2e.sh"
    PID_DIR="$PROJECT_DIR/out/pids"

    log "Stopping background Go processes..."
    "$E2E" stop-all "$PID_DIR"
else
    # --- Stop Docker Compose services ---
    COMPOSE_FILES=("-f" "$PROJECT_DIR/docker-compose.yaml")

    # Mirror start-services.sh: include the siblings overlay when active so
    # compose resolves the same project/services that were started.
    case "${USE_LOCAL_SIBLINGS:-}" in
        1|true|yes|on) COMPOSE_FILES+=("-f" "$PROJECT_DIR/docker-compose.siblings.yaml") ;;
    esac

    if [[ "$CHAIN" != "local" && -f "$PROJECT_DIR/docker-compose.$CHAIN.yaml" ]]; then
        COMPOSE_FILES+=("-f" "$PROJECT_DIR/docker-compose.$CHAIN.yaml")
    fi

    # docker-compose.yaml interpolates SOURCE_DATE_EPOCH as a build arg. It's
    # irrelevant on `down`, but compose still warns when it's unset — silence it.
    export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-0}"

    log "Stopping Docker Compose services (chain: $CHAIN)..."
    docker compose "${COMPOSE_FILES[@]}" down
fi

# Stopped last: simulated runs own the tunnel. Other extensions reuse this
# container, so tearing it down rotates their URL too.
CF_COMPOSE="$PROJECT_DIR/docker-compose.cloudflared.yaml"
if [[ -f "$CF_COMPOSE" ]]; then
    # Never-empty: bash 3.2 + set -u treats "${empty[@]}" as unbound.
    CF_COMPOSE_CMD=(docker compose -f "$CF_COMPOSE")
    [[ "$USE_LOCAL" == "true" ]] && CF_COMPOSE_CMD=(docker compose -p tunnel-local -f "$CF_COMPOSE")
    # -a: also catch a crashed/exited tunnel container, not just a running one —
    # otherwise a dead container is left behind and the next start-services.sh
    # run restarts it in place, minting a fresh quick-tunnel URL that nothing
    # here caused to be re-synced.
    if [[ "$USE_TUNNEL" == "true" || "${SIMULATED_TEE:-true}" == "true" ]]; then
        if "${CF_COMPOSE_CMD[@]}" ps -aq cloudflared 2>/dev/null | grep -q .; then
            log "Stopping the shared Cloudflare tunnel (last)..."
            "${CF_COMPOSE_CMD[@]}" down || log "WARNING: failed to stop cloudflared"
        else
            log "No tunnel running — nothing to stop."
        fi
    elif "${CF_COMPOSE_CMD[@]}" ps -aq cloudflared 2>/dev/null | grep -q .; then
        log "Leaving the shared Cloudflare tunnel running (pass --tunnel to stop it)."
    fi
fi

log "Done."
