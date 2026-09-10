#!/usr/bin/env bash
#
# build-node-base.sh — build the shared tee-node base image.
#
# Produces local/tee-node-base:${TEE_NODE_REF}, carrying the tee-node `server`
# binary that non-Go language images run alongside their extension process.
# Tagged by ref so bumping the pin in go/go.mod naturally invalidates it.
#
# The Go image does not need this — it links tee-node as a library.
#
# Usage: ./scripts/build-node-base.sh [--force]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
log() { echo -e "${GREEN}[build-node-base]${NC} $*"; }
die() { echo -e "${RED}[build-node-base] ERROR:${NC} $*" >&2; exit 1; }

FORCE=false
[[ "${1:-}" == "--force" ]] && FORCE=true

# shellcheck source=lib/versions.sh
source "$SCRIPT_DIR/lib/versions.sh"
load_versions "$PROJECT_DIR" || die "could not derive dependency versions"

IMAGE="local/tee-node-base:${TEE_NODE_REF}"

if [[ "$FORCE" == "false" ]] && docker image inspect "$IMAGE" >/dev/null 2>&1; then
    log "$IMAGE already exists (use --force to rebuild)"
    exit 0
fi

: "${SOURCE_DATE_EPOCH:=$(git -C "$PROJECT_DIR" log -1 --format=%ct 2>/dev/null || echo 0)}"

log "Building $IMAGE"
log "  tee-node pin: $TEE_NODE_VERSION"
log "  git ref:      $TEE_NODE_REF"
log "  SOURCE_DATE_EPOCH: $SOURCE_DATE_EPOCH"

docker build --platform linux/amd64 \
    -f "$PROJECT_DIR/docker/node-base.Dockerfile" \
    --build-arg "TEE_NODE_REF=$TEE_NODE_REF" \
    --build-arg "SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH" \
    -t "$IMAGE" \
    "$PROJECT_DIR/docker" \
    || die "failed to build $IMAGE"

log "Built $IMAGE"
