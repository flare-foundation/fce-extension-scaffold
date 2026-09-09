# chain-env.sh — sourced helper and the single chain table for every script here.
# resolve_chain <chain> selects the per-chain config dir (config/<chain>/) and,
# for non-local chains, forces CHAIN_URL/ADDRESSES_FILE.
#
# tee-proxy is NOT pinned per chain here: this repo pins it once in
# proxy/Dockerfile and scripts/check-versions.sh enforces that pin.

CHAINS="local coston coston2 songbird flare"

valid_chain() { [[ " $CHAINS " == *" ${1:-} "* ]]; }

# Empty for local — its proxy config carries whatever the devnet was built with.
chain_id_for() {
    case "${1:-}" in
        local) echo 31337 ;;
        coston) echo 16 ;; coston2) echo 114 ;; songbird) echo 19 ;; flare) echo 14 ;;
        *) return 1 ;;
    esac
}

chain_rpc_for() { echo "https://$1-api.flare.network/ext/C/rpc"; }

resolve_chain() {
    CHAIN_ARG="${1:-${CHAIN:-}}"
    if [[ -z "$CHAIN_ARG" ]]; then
        echo "usage: $(basename "$0") <${CHAINS// /|}>" >&2
        exit 1
    fi
    valid_chain "$CHAIN_ARG" || {
        echo "unknown chain '$CHAIN_ARG' (expected ${CHAINS// /, })" >&2
        exit 1
    }
    # local: CHAIN_URL/.env defaults and sim_dump auto-detect stay in charge.
    if [[ "$CHAIN_ARG" != "local" ]]; then
        CHAIN_URL="$(chain_rpc_for "$CHAIN_ARG")"
        ADDRESSES_FILE="$PROJECT_DIR/config/$CHAIN_ARG/deployed-addresses.json"
    fi
    CHAIN="$CHAIN_ARG"
    CHAIN_CONFIG_DIR="$PROJECT_DIR/config/$CHAIN_ARG"
    mkdir -p "$CHAIN_CONFIG_DIR"
    export EXTENSION_CONFIG_DIR="$CHAIN_CONFIG_DIR"
}

# Path to this chain's extension.env. Deliberately no fallback to a flat
# config/extension.env: that file belongs to whichever chain wrote it last, so
# falling back hands one chain's extension id to another.
extension_env_path() { echo "$PROJECT_DIR/config/${1:-$CHAIN}/extension.env"; }

# Resolve CHAIN the way every script here does it — explicit CHAIN wins, then
# legacy LOCAL_MODE — and select the per-chain config dir. Idempotent.
resolve_chain_default() {
    local want="${1:-${CHAIN:-}}"
    if [[ -z "$want" ]]; then
        [[ "${LOCAL_MODE:-true}" == "true" ]] && want="local" || want="coston2"
    fi
    resolve_chain "$want"
}
