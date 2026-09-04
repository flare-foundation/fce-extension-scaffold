#!/usr/bin/env bash
#
# use-chain.sh — Scaffold and activate everything one chain needs.
#
# One command readies a chain end to end; no per-script <chain> arg needed
# afterwards, because every script falls back to CHAIN= in .env.
#
#   ./scripts/use-chain.sh <chain>       local | coston | coston2 | songbird | flare
#   ./scripts/use-chain.sh --list        List chains with a .env.<chain> file.
#   ./scripts/use-chain.sh -h | --help   Show this help.
#
# What it does for <chain>, creating anything missing (never overwriting):
#   .env.<chain>                                       root env file (secrets
#                                                      carried from active .env)
#   config/proxy/extension_proxy.<chain>.docker.toml   proxy config (Docker)
#   docker-compose.<chain>.yaml                        compose override
# then copies .env.<chain> to .env and prints what still needs a human.
#
# Deployment artifacts (extension.env, register-tee.state) live per chain under
# config/<chain>/, so switching never disturbs them.
#
# The one thing it cannot invent is config/<chain>/deployed-addresses.json —
# the per-network Flare deployment dump. It must already exist for the chain.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SCRIPT_NAME="$(basename "$0")"
source "$SCRIPT_DIR/chain-env.sh"   # chain table: CHAINS, valid_chain, chain_id_for

log()  { echo "[use-chain] $*"; }
warn() { echo "[use-chain] WARN: $*" >&2; }
die()  { echo "[use-chain] ERROR: $*" >&2; exit 1; }

usage() {
    cat <<EOF
use-chain.sh — Scaffold and activate everything one chain needs.

Usage:
  $SCRIPT_NAME <chain>          ${CHAINS// / | }
  $SCRIPT_NAME --list           List chains with a .env.<chain> file.
  $SCRIPT_NAME -v | --versions  Show the tee-node / tee-proxy versions in use vs latest upstream.
  $SCRIPT_NAME -h | --help      Show this help.

Creates any missing per-chain files (.env.<chain>, proxy toml, compose
override), then activates the chain by copying .env.<chain> to .env. Existing
files are never overwritten — delete one to regenerate it.
EOF
}

# Value of KEY $2 in file $1 (comment/quote-stripped); empty if unset.
# The trailing `|| true` matters: a missing key would otherwise trip pipefail.
get_val() {
    grep -E "^$2=" "$1" 2>/dev/null | head -1 \
        | sed -E "s/^[^=]*=//; s/[[:space:]]+#.*$//; s/[[:space:]]+$//" \
        | tr -d '"' || true
}

list_chains() {
    local any=0 f name active
    active=$(get_val "$PROJECT_DIR/.env" CHAIN)
    for f in "$PROJECT_DIR"/.env.*; do
        [[ -f "$f" ]] || continue
        name="${f##*/.env.}"
        case "$name" in example|template|sample|backup*) continue ;; esac
        [[ "$name" == "$active" ]] && echo "  $name (active)" || echo "  $name"
        any=1
    done
    [[ $any -eq 1 ]] || echo "  (none yet — run: $SCRIPT_NAME <chain>)"
}

# Address of contract $2 in deployed-addresses.json $1.
addr_of() { jq -r --arg n "$2" '.[] | select(.name == $n) | .address' "$1"; }

# Module $2's version in go.mod $1; empty if absent.
gomod_ver() { grep -E "^\s*$2 v" "$1" 2>/dev/null | head -1 | awk '{print $2}' || true; }

# Default value of build ARG $2 in Dockerfile $1.
arg_ver() { grep -E "^ARG $2=" "$1" 2>/dev/null | head -1 | sed -E 's/.*=//' || true; }

# Latest tag on the upstream repo $1; empty when offline or git is unavailable.
latest_tag() {
    timeout 10 git ls-remote --tags --refs "https://github.com/flare-foundation/$1.git" 2>/dev/null \
        | sed 's|.*refs/tags/||' | sort -V | tail -1 || true
}

# up-to-date / behind / unknown, comparing $1 against upstream tag $2.
state() {
    [[ -n "$1" && -n "$2" ]] || { echo "(offline)"; return; }
    [[ "$1" == "$2" ]] && { echo "up to date"; return; }
    echo "BEHIND"
}

# Bumping either version changes the attested codeHash, so the TEE must be re-registered.
# python/typescript need no edit: their TEE_NODE_REF is derived from go/go.mod.
upgrade_hint() {
    local n="${1:-<tag>}" p="${2:-<tag>}"
    echo
    echo "  bump both together (shared TeeInfoResponse), then rebuild and re-register:"
    echo "    > go -C go    get github.com/flare-foundation/tee-node@$n && go -C go mod tidy"
    echo "    > go -C tools get github.com/flare-foundation/tee-node@$n github.com/flare-foundation/tee-proxy@$p && go -C tools mod tidy"
    echo "    set ARG TEE_PROXY_VERSION=$p  in proxy/Dockerfile"
    echo "    (python/typescript need no edit — their TEE_NODE_REF comes from go/go.mod)"
    echo "    > ./scripts/start-services.sh && ./scripts/post-build.sh"
}

# What we build with, and how far behind upstream that is.
show_versions() {
    local node proxy node_latest proxy_latest tools_node tools_proxy
    local fmt='  %-10s using %-42s latest %-8s %s\n'

    # go/go.mod is the single source of truth for tee-node; proxy/Dockerfile for tee-proxy.
    node=$(gomod_ver "$PROJECT_DIR/go/go.mod" github.com/flare-foundation/tee-node)
    proxy=$(arg_ver "$PROJECT_DIR/proxy/Dockerfile" TEE_PROXY_VERSION)

    node_latest=$(latest_tag tee-node)
    proxy_latest=$(latest_tag tee-proxy)

    printf "$fmt" tee-node  "${node:-<unset>}"     "${node_latest:-?}"  "$(state "$node" "$node_latest")"
    printf "$fmt" tee-proxy "${proxy:-<unpinned>}" "${proxy_latest:-?}" "$(state "$proxy" "$proxy_latest")"

    if [[ "$(state "$node" "$node_latest")" == "BEHIND" || "$(state "$proxy" "$proxy_latest")" == "BEHIND" ]]; then
        upgrade_hint "$node_latest" "$proxy_latest"
    fi

    # check-versions.sh enforces these too; surface the drift here rather than at build time.
    tools_node=$(gomod_ver "$PROJECT_DIR/tools/go.mod" github.com/flare-foundation/tee-node)
    tools_proxy=$(gomod_ver "$PROJECT_DIR/tools/go.mod" github.com/flare-foundation/tee-proxy)
    [[ -z "$tools_node" || "$tools_node" == "$node" ]] \
        || warn "go/go.mod pins tee-node $node but tools/go.mod pins $tools_node — the images differ"
    [[ -z "$tools_proxy" || "$tools_proxy" == "$proxy" ]] \
        || warn "proxy/Dockerfile pins tee-proxy $proxy but tools/go.mod pins $tools_proxy"
}

# --- Argument parsing ---
[[ $# -ge 1 ]] || { usage; exit 1; }
case "$1" in
    -h|--help) usage; exit 0 ;;
    --list)    log "available chains:"; list_chains; exit 0 ;;
    -v|--versions) log "versions:"; show_versions; exit 0 ;;
    -*)        die "unknown flag: $1 (try --help)" ;;
esac
[[ $# -eq 1 ]] || die "expected one chain name (got $#); try --help"

CHAIN="$1"
valid_chain "$CHAIN" || die "unknown chain '$CHAIN' (expected ${CHAINS// /, })"
IS_LOCAL=false; [[ "$CHAIN" == "local" ]] && IS_LOCAL=true
CHAIN_ID=""

ADDR_JSON="$PROJECT_DIR/config/$CHAIN/deployed-addresses.json"
ENV_FILE="$PROJECT_DIR/.env.$CHAIN"
ACTIVE_ENV="$PROJECT_DIR/.env"
TOML_DOCKER="$PROJECT_DIR/config/proxy/extension_proxy.$CHAIN.docker.toml"
COMPOSE="$PROJECT_DIR/docker-compose.$CHAIN.yaml"

command -v jq >/dev/null || die "jq is required"

# --- 0. The one non-generatable input (local devnet deploys its own) ---
if [[ "$IS_LOCAL" == "false" ]]; then
    CHAIN_ID="$(chain_id_for "$CHAIN")"
    RPC="$(chain_rpc_for "$CHAIN")"
    [[ -f "$ADDR_JSON" ]] || die "config/$CHAIN/deployed-addresses.json is missing.
This is the per-network Flare deployment dump (FlareTeeManager, FlareSystemsManager, ...)
and cannot be generated here. Copy it from the tee deployment / e2e repo for '$CHAIN',
then re-run: $SCRIPT_NAME $CHAIN"

    FSM=$(addr_of "$ADDR_JSON" FlareSystemsManager)
    RELAY=$(addr_of "$ADDR_JSON" Relay)
    VOTER_REG=$(addr_of "$ADDR_JSON" VoterRegistry)
    for v in FSM RELAY VOTER_REG; do
        [[ "${!v}" =~ ^0x[0-9a-fA-F]{40}$ ]] || die "$v not found in $ADDR_JSON"
    done
fi

# A file with no trailing newline would glue an appended key onto its last line.
ensure_nl() { [[ -s "$1" && -n "$(tail -c1 "$1")" ]] && printf '\n' >> "$1"; return 0; }

# Replace KEY $1 line with KEY=$2 in $ENV_FILE (append if absent).
set_kv() {
    if grep -qE "^$1=" "$ENV_FILE"; then
        sed -i "s|^$1=.*|$1=$2|" "$ENV_FILE"
    else
        ensure_nl "$ENV_FILE"
        printf '%s=%s\n' "$1" "$2" >> "$ENV_FILE"
    fi
}

# --- 1. .env.<chain> ---
# Clone the active .env line for line — same keys, same comments — and rewrite
# only the chain-specific values. Falls back to .env.example on a fresh clone.
if [[ -f "$ENV_FILE" ]]; then
    log "keep    .env.$CHAIN (exists)"
else
    donor=""
    if [[ -f "$ACTIVE_ENV" ]]; then
        donor="$ACTIVE_ENV"
    elif [[ -f "$PROJECT_DIR/.env.example" ]]; then
        donor="$PROJECT_DIR/.env.example"
    else
        die "no .env or .env.example to clone .env.$CHAIN from"
    fi
    donor_chain=$(get_val "$donor" CHAIN)
    cp "$donor" "$ENV_FILE"
    set_kv CHAIN "$CHAIN"
    if [[ "$IS_LOCAL" == "true" ]]; then
        set_kv LOCAL_MODE true
        set_kv SIMULATED_TEE true
        set_kv CHAIN_ID 31337
        set_kv CHAIN_URL "http://127.0.0.1:8545"
        # The devnet's own FTDC proxy, not a testnet one carried over from the donor.
        set_kv NORMAL_PROXY_URL "http://localhost:6662"
        # Empty, not a testnet path: pre-build then auto-detects the sim_dump.
        set_kv ADDRESSES_FILE ""
    else
        set_kv LOCAL_MODE false
        set_kv SIMULATED_TEE false
        set_kv CHAIN_ID "$CHAIN_ID"
        set_kv CHAIN_URL "$RPC"
        set_kv ADDRESSES_FILE "./config/$CHAIN/deployed-addresses.json"
        set_kv NORMAL_PROXY_URL "https://tee-proxy-$CHAIN-1.flare.rocks"
    fi
    # Never guessed and never carried over — the donor chain's proxy URL is wrong here.
    set_kv EXT_PROXY_URL ""
    warn "EXT_PROXY_URL left empty in .env.$CHAIN — set it from devops (or your tunnel)"
    log "created .env.$CHAIN (cloned from ${donor##*/}${donor_chain:+ [$donor_chain]}, chain values rewritten)"
fi

# --- 2. Proxy toml (Docker) — from the committed example, addresses from the dump ---
if [[ "$IS_LOCAL" == "true" ]]; then
    :   # local uses the committed extension_proxy.docker.toml
elif [[ -f "$TOML_DOCKER" ]]; then
    log "keep    ${TOML_DOCKER#"$PROJECT_DIR"/} (exists)"
else
    TOML_TEMPLATE="$PROJECT_DIR/config/proxy/extension_proxy.coston2.docker.toml.example"
    [[ -f "$TOML_TEMPLATE" ]] || die "${TOML_TEMPLATE#"$PROJECT_DIR"/} missing — nothing to template the proxy config from"

    # Value of toml key $2 in file $1 (quotes/comments stripped); empty if unset.
    toml_val() {
        grep -E "^$2[[:space:]]*=" "$1" 2>/dev/null | head -1 \
            | sed -E 's/^[^=]*=[[:space:]]*//; s/[[:space:]]+#.*$//' | tr -d '"' || true
    }

    # Carry the [db] block from another chain toml — the indexer DB credentials are
    # usually shared, and a real value beats a placeholder. Reported below so it
    # still gets verified.
    DB_DONOR=""
    for d in coston2 coston songbird flare; do
        [[ "$d" == "$CHAIN" ]] && continue
        f="$PROJECT_DIR/config/proxy/extension_proxy.$d.docker.toml"
        [[ -f "$f" ]] || continue
        h=$(toml_val "$f" host)
        [[ -n "$h" && "$h" != \<* ]] || continue
        DB_DONOR="$d"
        break
    done

    sed -e "1s|.*|# Docker-specific $CHAIN proxy config — generated by use-chain.sh (delete to regenerate).|" \
        -e "s|^chain_id[[:space:]]*=.*|chain_id = $CHAIN_ID|" \
        -e "s|^flare_systems_manager[[:space:]]*=.*|flare_systems_manager = \"$FSM\"|" \
        -e "s|^relay[[:space:]]*=.*|relay = \"$RELAY\"|" \
        -e "s|^voter_registry[[:space:]]*=.*|voter_registry = \"$VOTER_REG\"|" \
        "$TOML_TEMPLATE" > "$TOML_DOCKER"

    if [[ -n "$DB_DONOR" ]]; then
        f="$PROJECT_DIR/config/proxy/extension_proxy.$DB_DONOR.docker.toml"
        sed -i \
            -e "s|^host[[:space:]]*=.*|host = \"$(toml_val "$f" host)\"|" \
            -e "s|^port[[:space:]]*=.*|port = $(toml_val "$f" port)|" \
            -e "s|^database[[:space:]]*=.*|database = \"$(toml_val "$f" database)\"|" \
            -e "s|^username[[:space:]]*=.*|username = \"$(toml_val "$f" username)\"|" \
            -e "s|^password[[:space:]]*=.*|password = \"$(toml_val "$f" password)\"|" \
            "$TOML_DOCKER"
    fi
    log "created ${TOML_DOCKER#"$PROJECT_DIR"/}${DB_DONOR:+ ([db] carried from $DB_DONOR — verify)}"
    warn "  initial_signing_policy_offset=$(toml_val "$TOML_DOCKER" initial_signing_policy_offset) came from the template — it is per-chain, confirm it with devops"
fi

# --- 3. Compose override — from the coston2 one, values swapped ---
if [[ "$IS_LOCAL" == "true" ]]; then
    :   # local runs docker-compose.yaml with no override
elif [[ -f "$COMPOSE" ]]; then
    log "keep    docker-compose.$CHAIN.yaml (exists)"
else
    TEMPLATE="$PROJECT_DIR/docker-compose.coston2.yaml"
    [[ -f "$TEMPLATE" ]] || die "docker-compose.coston2.yaml missing — nothing to template the compose override from"
    sed -e "s/coston2/$CHAIN/g" -e "s/Coston2/${CHAIN^}/g" \
        -e "s/CHAIN_ID:-114/CHAIN_ID:-$CHAIN_ID/" \
        -e "s/chain_id (114)/chain_id ($CHAIN_ID)/" "$TEMPLATE" > "$COMPOSE"
    log "created docker-compose.$CHAIN.yaml"
fi

# --- 4. Activate ---
# .env is a disposable copy of the active .env.<chain>; edits belong in the
# per-chain file. Catch hand-edits to .env before they are silently discarded.
if [[ -f "$ACTIVE_ENV" ]]; then
    # Back up unless this exact content is already saved as some .env.<chain> —
    # keying off CHAIN= would miss a hand-written .env whose chain file is absent.
    saved=false
    for f in "$PROJECT_DIR"/.env.*; do
        [[ -f "$f" && "$f" != *.backup ]] || continue
        diff -q "$ACTIVE_ENV" "$f" >/dev/null 2>&1 && { saved=true; break; }
    done
    if [[ "$saved" == "false" ]]; then
        cp "$ACTIVE_ENV" "$PROJECT_DIR/.env.backup"
        warn ".env had edits not saved in any .env.<chain> — copied to .env.backup (edit .env.<chain> files, not .env)"
    fi
fi
cp "$ENV_FILE" "$ACTIVE_ENV" || die "failed to copy $ENV_FILE to .env"
# Older hand-written env files may predate the CHAIN= convention.
if ! grep -qE '^CHAIN=' "$ACTIVE_ENV"; then
    ensure_nl "$ACTIVE_ENV"
    printf 'CHAIN=%s\n' "$CHAIN" >> "$ACTIVE_ENV"
    warn ".env.$CHAIN has no CHAIN= line — appended CHAIN=$CHAIN to .env (add it to .env.$CHAIN too)"
fi
log "active chain: $CHAIN${CHAIN_ID:+ (chain_id=$CHAIN_ID)}"

# --- 5. Summary + what still needs a human ---
key_val=$(get_val "$ACTIVE_ENV" DEPLOYMENT_PRIVATE_KEY)
log "  CHAIN_URL              = $(v=$(get_val "$ACTIVE_ENV" CHAIN_URL);        echo "${v:-<unset>}")"
log "  EXT_PROXY_URL          = $(v=$(get_val "$ACTIVE_ENV" EXT_PROXY_URL);    echo "${v:-<unset>}")"
log "  NORMAL_PROXY_URL       = $(v=$(get_val "$ACTIVE_ENV" NORMAL_PROXY_URL); echo "${v:-<unset>}")"
log "  INITIAL_OWNER          = $(v=$(get_val "$ACTIVE_ENV" INITIAL_OWNER);    echo "${v:-<unset>}")"
log "  DEPLOYMENT_PRIVATE_KEY = $([[ -n "$key_val" ]] && echo "<set>" || echo "<unset>")"

# Deployment artifacts are already per chain — flag only that this chain has none.
EXT_ENV="$PROJECT_DIR/config/$CHAIN/extension.env"
if [[ -f "$EXT_ENV" ]]; then
    log "  EXTENSION_ID           = $(v=$(get_val "$EXT_ENV" EXTENSION_ID); echo "${v:-<unset>}")"
else
    warn "no config/$CHAIN/extension.env yet — run ./scripts/pre-build.sh $CHAIN before starting services"
fi

todo=0
for f in "$ENV_FILE" "$TOML_DOCKER"; do
    [[ -f "$f" ]] || continue
    if grep -qE '<(your-tunnel|indexer-db)' "$f" 2>/dev/null; then
        [[ $todo -eq 0 ]] && warn "placeholders remain — fill these in before deploying:"
        todo=1
        grep -nE '<(your-tunnel|indexer-db)' "$f" | sed "s|^|[use-chain]   ${f#"$PROJECT_DIR"/}:|"
    fi
done
exit 0
