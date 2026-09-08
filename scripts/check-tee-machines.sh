#!/usr/bin/env bash
#
# check-tee-machines.sh — list registered TEE machines, flag the dead ones.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/chain-env.sh"   # chain table + resolve_chain

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[check-tee-machines]${NC} $*"; }
warn() { echo -e "${YELLOW}[check-tee-machines] WARN:${NC} $*" >&2; }
die()  { echo -e "${RED}[check-tee-machines] ERROR:${NC} $*" >&2; exit 1; }

usage() {
    cat <<EOF
check-tee-machines.sh — list registered TEE machines, flag the dead ones.

Usage:
  $(basename "$0")                  the chain from .env
  $(basename "$0") --chain coston   a specific chain
  $(basename "$0") --ext 0          only this extension id

A relaunched Confidential Space mints a new key but leaves the old machine
active on-chain, and getRandomTeeIds routes to it anyway — those instructions
404 forever. This asks each machine's own proxy which key it holds now and
compares. Reports only: pause commands are printed, never run.
EOF
}

TIMEOUT=12
CHAIN_ARG=""; EXT_ARG=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --chain) CHAIN_ARG="${2:-}"; shift 2 ;;
        --ext)   EXT_ARG="${2:-}"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) die "unknown argument: $1 (try --help)" ;;
    esac
done

for t in curl jq cast go; do command -v "$t" >/dev/null || die "$t is required"; done

[[ -f "$PROJECT_DIR/.env" ]] && { set -a; source "$PROJECT_DIR/.env"; set +a; }
# After sourcing: `set -a; source .env` would otherwise clobber a bare CHAIN.
resolve_chain_default "$CHAIN_ARG"
[[ "$CHAIN" == "local" ]] && die "local devnet has no registered TEE machines"

ADDRESSES="$CHAIN_CONFIG_DIR/deployed-addresses.json"
[[ -f "$ADDRESSES" ]] || die "no $ADDRESSES — run ./scripts/use-chain.sh $CHAIN first"
REG=$(jq -r '.[] | select(.name=="FlareTeeManager") | .address' "$ADDRESSES" | head -1)
[[ -n "$REG" && "$REG" != "null" ]] || die "no FlareTeeManager address in $ADDRESSES"

log "chain:    $CHAIN (id $(chain_id_for "$CHAIN"))"
log "registry: $REG"

# Live teeId for a proxy: keccak256(pubkey.x ‖ pubkey.y)[12:]. Fails if unreachable.
live_tee_id() {
    local info x y
    info=$(curl -fsS -m "$TIMEOUT" "$1/info" 2>/dev/null) || return 1
    x=$(printf '%s' "$info" | jq -r '.teeInfo.publicKey.x // empty' | sed 's/^0x//')
    y=$(printf '%s' "$info" | jq -r '.teeInfo.publicKey.y // empty' | sed 's/^0x//')
    [[ -n "$x" && -n "$y" ]] || return 1
    printf '0x%s\n' "$(cast keccak "0x$x$y" | sed 's/^0x//' | tail -c 41 | tr -d '\r\n' | tr '[:upper:]' '[:lower:]')"
}

lc() { tr '[:upper:]' '[:lower:]' <<< "$1"; }

# bytes32 form, middle-elided: 0x000...10336. Zero has nothing to elide.
ext_display() {
    local p
    [[ "$1" == "0" ]] && { echo "0x0"; return; }
    p=$(printf '%064x' "$1")
    echo "0x${p:0:3}...${p: -5}"
}

# cast reads CHAIN from the repo's .env as its own --chain flag, so pin it here.
CAST_CHAIN=(--chain "$(chain_id_for "$CHAIN")")

# A proxy whose signing policy lags the chain's reward epoch answers 404 "no round".
show_policy() {
    local fsm epoch url pol st
    fsm=$(jq -r '.[]|select(.name=="FlareSystemsManager")|.address' "$ADDRESSES" 2>/dev/null | head -1)
    [[ -n "$fsm" && "$fsm" != "null" ]] || return 0
    epoch=$(cast call "$fsm" "getCurrentRewardEpochId()(uint256)" \
              --rpc-url "$CHAIN_URL" "${CAST_CHAIN[@]}" 2>/dev/null | awk '{print $1}') || return 0
    [[ -n "$epoch" ]] || return 0
    echo -e "\n${CYAN}=== signing policy ===${NC}"
    printf '  %-56s %s\n' "chain reward epoch" "$epoch"
    for url in "${URLS[@]}"; do
        pol=$(curl -fsS -m "$TIMEOUT" "$url/info" 2>/dev/null | jq -r '.teeInfo.lastSigningPolicyId // empty') || pol=""
        if [[ -z "$pol" ]]; then st="${YELLOW}unreachable${NC}"
        elif (( pol == epoch || pol == epoch + 1 )); then st="${GREEN}in sync${NC}"
        else st="${RED}OUT OF SYNC${NC}"; fi
        printf '  %-56s %-6s %b\n' "$url" "${pol:-–}" "$st"
    done
}

STALE=(); URLS=()
check_extension() {
    local ext="$1" label="$2" out line id url live n_live=0 n_stale=0 n_unk=0
    # query-tee -ext is an int64, so the call is decimal; the display stays hex.
    echo -e "\n${CYAN}=== $label (extensionId=$(ext_display "$ext")) ===${NC}"
    out=$(cd "$PROJECT_DIR/tools" && go run ./cmd/query-tee \
            -rpc "$CHAIN_URL" -reg "$REG" -ext "$ext" 2>&1) \
        || { warn "query failed for extensionId=$ext"; return 0; }

    while read -r line; do
        [[ "$line" =~ ^[[:space:]]*[0-9]+:[[:space:]]*(0x[0-9a-fA-F]{40})[[:space:]]+url=\"(.*)\"$ ]] || continue
        id="${BASH_REMATCH[1]}"; url="${BASH_REMATCH[2]}"
        if [[ -z "$url" ]]; then
            printf '  %b%-10s%b %s  (no URL registered)\n' "$YELLOW" "UNKNOWN" "$NC" "$id"; n_unk=$((n_unk+1)); continue
        fi
        [[ " ${URLS[*]} " == *" $url "* ]] || URLS+=("$url")
        if ! live=$(live_tee_id "$url"); then
            printf '  %b%-10s%b %s  %s  (proxy unreachable)\n' "$YELLOW" "UNREACHABLE" "$NC" "$id" "$url"
            n_unk=$((n_unk+1)); continue
        fi
        if [[ "$(lc "$id")" == "$live" ]]; then
            printf '  %b%-10s%b %s  %s\n' "$GREEN" "LIVE" "$NC" "$id" "$url"; n_live=$((n_live+1))
        else
            printf '  %b%-10s%b %s  %s\n' "$RED" "STALE" "$NC" "$id" "$url"
            printf '             %bthat URL now serves machine %s%b\n' "$YELLOW" "$live" "$NC"
            STALE+=("$ext|$id|$url"); n_stale=$((n_stale+1))
        fi
    done <<< "$out"

    grep -q '  (none)' <<< "$out" && echo "  (none registered)"
    printf '  %d live, %d stale, %d unverified\n' "$n_live" "$n_stale" "$n_unk"
}

if [[ -n "$EXT_ARG" ]]; then
    check_extension "$((EXT_ARG))" "extension"
else
    check_extension 0 "FTDC infrastructure"
    EXT_ENV="$(extension_env_path)"
    if [[ -f "$EXT_ENV" ]]; then
        EXT_HEX=$(grep -E '^EXTENSION_ID=' "$EXT_ENV" | head -1 | sed -E 's/^[^=]*=//; s/[[:space:]]*#.*$//' | tr -d '"')
        [[ -n "$EXT_HEX" ]] && check_extension "$((EXT_HEX))" "this extension"
    else
        warn "no $EXT_ENV — run pre-build.sh to learn this extension's id"
    fi
fi

show_policy

echo
if (( ${#STALE[@]} == 0 )); then
    log "no stale machines found"
    exit 0
fi

echo -e "${RED}=== ${#STALE[@]} stale machine(s) ===${NC}"
cat <<EOF

Pausing is IRREVERSIBLE: there is no unpause, only toProduction with a fresh
availability proof. Check each address against the LIVE rows above before running
anything — pausing the live machine takes the extension down.

EOF
for entry in "${STALE[@]}"; do
    IFS='|' read -r ext id url <<< "$entry"
    if [[ "$ext" == "0" ]]; then
        echo "  # $id ($url) — extensionId=0 is Flare's FTDC infrastructure."
        echo "  # Not yours to pause: report it to whoever operates $CHAIN."
    else
        echo "  cast send $REG 'pause(address)' $id \\"
        echo "      --rpc-url $CHAIN_URL ${CAST_CHAIN[*]} --private-key \$DEPLOYMENT_PRIVATE_KEY"
    fi
    echo
done
