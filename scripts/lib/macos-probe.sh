#!/usr/bin/env bash
#
# macos-probe.sh — run the read-only entry points the way macOS would.
#
# Two halves of macOS break scripts written on Linux: bash 3.2, and a BSD
# userland. Run this under `bash:3.2` and it covers both — the shims below reject
# the GNU-only spellings at exec time, so a flag buried in a code path no grep
# reaches still fails here.
set -uo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1

fail=0
err() { echo "FAIL  $*"; fail=1; }

case "$(bash --version | head -1)" in
    *"version 3.2"*) ;;
    *) echo "warn  not bash 3.2 — the bash half of this probe proves nothing" ;;
esac

# --- BSD userland shims ------------------------------------------------------
# One dispatcher, linked under each name; it rejects the GNU-only form of its
# own argv and otherwise hands off to the real tool.
SHIM="$(mktemp -d)/bin"; mkdir -p "$SHIM"
cat > "$SHIM/_dispatch" <<'SHIMEOF'
#!/usr/bin/env bash
tool="$(basename "$0")"
die() { echo "$tool: $1 — not available on macOS (BSD userland)" >&2; exit 64; }
case "$tool" in
    timeout|nproc|tac|sha256sum|md5sum|realpath) die "the command itself" ;;
esac
for a in "$@"; do
    case "$tool:$a" in
        sed:-i)        die "bare -i (BSD reads the next arg as the suffix; use -i.bak)" ;;
        sed:-r)        die "-r (BSD spells it -E)" ;;
        grep:-P|grep:-*P*) [[ "$a" == -*P* && "$a" != --* ]] && die "-P (BSD grep has no PCRE)" ;;
        stat:-c)       die "-c (BSD stat uses -f)" ;;
        date:-d|date:--date*) die "-d (BSD date uses -v)" ;;
        base64:-w)     die "-w (BSD base64 has no -w)" ;;
        find:-printf)  die "-printf (BSD find has none)" ;;
        xargs:-r)      die "-r (BSD xargs has none)" ;;
        readlink:-f)   die "-f (not on stock macOS)" ;;
    esac
done
real=""
IFS=: read -ra dirs <<< "$_REAL_PATH"
for d in "${dirs[@]}"; do [[ -x "$d/$tool" ]] && { real="$d/$tool"; break; }; done
[[ -n "$real" ]] || { echo "$tool: not found" >&2; exit 127; }
exec "$real" "$@"
SHIMEOF
chmod +x "$SHIM/_dispatch"
for t in sed grep stat date base64 find xargs readlink timeout nproc tac sha256sum md5sum realpath; do
    cp "$SHIM/_dispatch" "$SHIM/$t"
done
export _REAL_PATH="$PATH"
export PATH="$SHIM:$PATH"

# --- 1. everything parses ----------------------------------------------------
while IFS= read -r f; do
    bash -n "$f" || err "$f does not parse"
done < <(find . -name '*.sh' -not -path './node_modules/*' 2>/dev/null || /usr/bin/find . -name '*.sh')

# --- 2. read-only entry points actually run ----------------------------------
# set -u prints "unbound variable"; a GNU-only flag trips a shim above. A no-op
# docker lets stop-services.sh reach its compose-array code with no daemon.
printf '#!/bin/sh
exit 0
' > "$SHIM/docker"; chmod +x "$SHIM/docker"

run_probe() {   # run_probe <label> <script> [args...]
    local label="$1"; shift
    local out; out="$(bash "$@" 2>&1)"
    case "$out" in
        *"unbound variable"*)       err "$label: unbound variable (empty array under bash 3.2)" ;;
        *"bad substitution"*)       err "$label: bad substitution (bash 4 syntax)" ;;
        *"not available on macOS"*) err "$label: $(echo "$out" | grep -m1 'not available on macOS')" ;;
    esac
}

for a in --list --status --versions --help; do
    run_probe "use-chain.sh $a" scripts/use-chain.sh "$a"
done
for sc in start-services stop-services full-setup test-unit test-conformance; do
    run_probe "$sc.sh --help" "scripts/$sc.sh" --help
done
# The only entry point that reaches a compose array without a real daemon.
run_probe "stop-services.sh --chain local" scripts/stop-services.sh --chain local

rm -rf "$(dirname "$SHIM")"
(( fail )) && { echo "macos probe FAILED"; exit 1; }
echo "macos probe ok — $(bash --version | head -1)"
