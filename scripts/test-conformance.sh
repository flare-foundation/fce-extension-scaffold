#!/usr/bin/env bash
#
# test-conformance.sh — verify a language implementation against the container
# contract (docs/extension-contract.md).
#
# Starts ONLY the extension process — no tee-node, no proxy, no chain, no
# Docker — then replays the golden fixtures in testdata/conformance/ over HTTP
# and diffs the responses. Runs in seconds, which is what makes writing a new
# language port tractable: you get a correctness signal without a full deploy.
#
# The fixtures are ORDER-DEPENDENT and share one process: counters accumulate,
# and fixture 16 asserts the final state. index.json fixes the order.
#
# Usage:
#   ./scripts/test-conformance.sh              # the LANGUAGE from .env
#   ./scripts/test-conformance.sh python       # a specific language
#   ./scripts/test-conformance.sh --all        # every discovered language
#   ./scripts/test-conformance.sh --skip-setup # reuse installed deps
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURE_DIR="$PROJECT_DIR/testdata/conformance"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; YELLOW='\033[0;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[conformance]${NC} $*"; }
warn() { echo -e "${YELLOW}[conformance]${NC} $*"; }
die()  { echo -e "${RED}[conformance] ERROR:${NC} $*" >&2; exit 1; }

command -v jq >/dev/null   || die "jq is required (brew install jq / apt install jq)"
command -v curl >/dev/null || die "curl is required"

# shellcheck source=lib/language.sh
source "$SCRIPT_DIR/lib/language.sh"

SKIP_SETUP=false
RUN_ALL=false
TARGET=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --all)        RUN_ALL=true; shift ;;
        --skip-setup) SKIP_SETUP=true; shift ;;
        -h|--help)    sed -n '2,25p' "$0"; exit 0 ;;
        -*)           die "unknown flag: $1" ;;
        *)            TARGET="$1"; shift ;;
    esac
done

if [[ -f "$PROJECT_DIR/.env" ]]; then
    set -a; source "$PROJECT_DIR/.env"; set +a
fi

# --- pick a free TCP port -----------------------------------------------------
free_port() {
    python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()' 2>/dev/null \
      || echo $(( 20000 + RANDOM % 20000 ))
}

# --- run the fixture suite against one language -------------------------------
run_language() {
    local lang="$1"
    load_language "$PROJECT_DIR" "$lang" || return 1

    echo ""
    echo -e "${CYAN}=== $LANGUAGE_NAME ($lang) ===${NC}"

    if [[ -z "$LANGUAGE_RUN_CMD" ]]; then
        warn "$lang/language.env defines no LANGUAGE_RUN_CMD — skipping"
        return 0
    fi

    if [[ "$SKIP_SETUP" == "false" && -n "$LANGUAGE_SETUP_CMD" ]]; then
        log "setup: $LANGUAGE_SETUP_CMD"
        ( cd "$EXTENSION_DIR" && eval "$LANGUAGE_SETUP_CMD" ) \
            || { echo -e "${RED}setup failed${NC}" >&2; return 1; }
    fi
    if [[ -n "$LANGUAGE_BUILD_CMD" ]]; then
        log "build: $LANGUAGE_BUILD_CMD"
        ( cd "$EXTENSION_DIR" && eval "$LANGUAGE_BUILD_CMD" >/dev/null ) \
            || { echo -e "${RED}build failed${NC}" >&2; return 1; }
    fi

    local port; port="$(free_port)"
    local logfile; logfile="$(mktemp)"

    log "starting extension on :$port"
    ( cd "$EXTENSION_DIR" && EXTENSION_PORT="$port" SIGN_PORT=9090 \
        eval "exec $LANGUAGE_RUN_CMD" ) >"$logfile" 2>&1 &
    local server_pid=$!

    # Kill the whole process group: `go run` spawns a child binary that would
    # otherwise survive and hold the port.
    cleanup() {
        kill -TERM -"$server_pid" 2>/dev/null || kill -TERM "$server_pid" 2>/dev/null
        wait "$server_pid" 2>/dev/null
    }

    local ready=false
    for _ in $(seq 1 100); do
        if curl -sf -o /dev/null "http://127.0.0.1:$port/state" 2>/dev/null; then
            ready=true; break
        fi
        if ! kill -0 "$server_pid" 2>/dev/null; then
            echo -e "${RED}extension exited during startup:${NC}" >&2
            sed 's/^/    /' "$logfile" >&2
            rm -f "$logfile"; return 1
        fi
        sleep 0.3
    done
    if [[ "$ready" != "true" ]]; then
        echo -e "${RED}extension did not become ready on :$port${NC}" >&2
        sed 's/^/    /' "$logfile" >&2
        cleanup; rm -f "$logfile"; return 1
    fi

    local passed=0 failed=0
    while read -r name; do
        [[ -n "$name" ]] || continue
        if run_fixture "$FIXTURE_DIR/$name.json" "$port"; then
            passed=$((passed + 1))
        else
            failed=$((failed + 1))
        fi
    done < <(jq -r '.fixtures[]' "$FIXTURE_DIR/index.json")

    cleanup
    rm -f "$logfile"

    if [[ "$failed" -gt 0 ]]; then
        echo -e "${RED}  $passed passed, $failed FAILED${NC}"
        return 1
    fi
    echo -e "${GREEN}  $passed passed${NC}"
    return 0
}

# --- run a single fixture -----------------------------------------------------
run_fixture() {
    local file="$1" port="$2"
    local name; name="$(jq -r '.name' "$file")"
    local method; method="$(jq -r '.request.method' "$file")"
    local path;   path="$(jq -r '.request.path' "$file")"

    # Body: `body` is a JSON value to serialize, `raw_body` is sent verbatim
    # (used for deliberately malformed input).
    # Seeded and always ends with the URL: bash 3.2 + set -u rejects an empty array.
    local -a req_args=(-X "$method" -H 'Content-Type: application/json')
    if jq -e 'has("request") and (.request | has("raw_body"))' "$file" >/dev/null; then
        req_args+=(--data-binary "$(jq -r '.request.raw_body' "$file")")
    elif jq -e '.request | has("body")' "$file" >/dev/null; then
        req_args+=(--data-binary "$(jq -c '.request.body' "$file")")
    fi
    req_args+=("http://127.0.0.1:$port$path")

    local resp_file; resp_file="$(mktemp)"
    local code
    code="$(curl -s -o "$resp_file" -w '%{http_code}' \
        "${req_args[@]}" 2>/dev/null)"

    local expected_status; expected_status="$(jq -r '.expect.status' "$file")"
    local fail=""

    if [[ "$code" != "$expected_status" ]]; then
        fail="HTTP status: expected $expected_status, got $code"
    fi

    # Exact whole-body JSON match.
    if [[ -z "$fail" ]] && jq -e '.expect | has("json")' "$file" >/dev/null; then
        local expected actual
        expected="$(jq -S -c '.expect.json' "$file")"
        actual="$(jq -S -c '.' "$resp_file" 2>/dev/null)" || actual="<not JSON>"
        [[ "$expected" == "$actual" ]] || fail="body mismatch"$'\n'"    expected: $expected"$'\n'"    actual:   $actual"
    fi

    # Subset match: every listed key must be present and equal.
    if [[ -z "$fail" ]] && jq -e '.expect | has("json_subset")' "$file" >/dev/null; then
        while read -r key; do
            local expected actual
            expected="$(jq -S -c --arg k "$key" '.expect.json_subset[$k]' "$file")"
            actual="$(jq -S -c --arg k "$key" '.[$k]' "$resp_file" 2>/dev/null)" || actual="null"
            if [[ "$expected" != "$actual" ]]; then
                fail="field '$key': expected $expected, got $actual"
                break
            fi
        done < <(jq -r '.expect.json_subset | keys[]' "$file")
    fi

    # Plain-text body match.
    if [[ -z "$fail" ]] && jq -e '.expect | has("text")' "$file" >/dev/null; then
        local expected actual
        expected="$(jq -r '.expect.text' "$file")"
        actual="$(cat "$resp_file")"
        [[ "$expected" == "$actual" ]] || fail="text body: expected '$expected', got '$actual'"
    fi

    # Substring match, for bodies whose exact wording is diagnostic rather than
    # contractual (e.g. the 501 message, where Go adds the received/expected ids).
    if [[ -z "$fail" ]] && jq -e '.expect | has("text_contains")' "$file" >/dev/null; then
        local needle actual
        needle="$(jq -r '.expect.text_contains' "$file")"
        actual="$(cat "$resp_file")"
        [[ "$actual" == *"$needle"* ]] || fail="text body: expected to contain '$needle', got '$actual'"
    fi

    # log must start with this prefix (exact message wording is not contractual).
    if [[ -z "$fail" ]] && jq -e '.expect | has("log_prefix")' "$file" >/dev/null; then
        local prefix actual
        prefix="$(jq -r '.expect.log_prefix' "$file")"
        actual="$(jq -r '.log // ""' "$resp_file" 2>/dev/null)"
        [[ "$actual" == "$prefix"* ]] || fail="log: expected prefix '$prefix', got '$actual'"
    fi

    # Fields that must be absent entirely.
    if [[ -z "$fail" ]] && jq -e '.expect | has("absent_fields")' "$file" >/dev/null; then
        while read -r key; do
            if jq -e --arg k "$key" 'has($k)' "$resp_file" >/dev/null 2>&1; then
                fail="field '$key' should be absent but was present"
                break
            fi
        done < <(jq -r '.expect.absent_fields[]' "$file")
    fi

    rm -f "$resp_file"

    if [[ -n "$fail" ]]; then
        echo -e "  ${RED}✗${NC} $name"
        echo -e "    ${RED}$fail${NC}"
        return 1
    fi
    echo -e "  ${GREEN}✓${NC} $name"
    return 0
}

# --- main ---------------------------------------------------------------------
[[ -f "$FIXTURE_DIR/index.json" ]] || die "no fixtures at $FIXTURE_DIR"

FAILED_LANGS=()
if [[ "$RUN_ALL" == "true" ]]; then
    while read -r lang; do
        [[ -n "$lang" ]] || continue
        run_language "$lang" || FAILED_LANGS+=("$lang")
    done < <(list_languages "$PROJECT_DIR")
else
    lang="${TARGET:-${LANGUAGE:-go}}"
    run_language "$lang" || FAILED_LANGS+=("$lang")
fi

echo ""
if [[ ${#FAILED_LANGS[@]} -gt 0 ]]; then
    die "conformance FAILED for: ${FAILED_LANGS[*]}"
fi
log "conformance passed"
