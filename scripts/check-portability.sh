#!/usr/bin/env bash
#
# check-portability.sh — fail on shell that works on Linux but not on macOS.
#
# macOS ships a BSD userland and bash 3.2, so the two failure modes are GNU-only
# utility flags and empty-array expansion under `set -u`. This only greps, so a
# Linux CI job catches macOS breakage without needing a macOS runner.
set -uo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1

RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; NC=$'\033[0m'
fail=0
ok() { echo "${GREEN}ok${NC}    $1"; }
bad() {
    echo "${RED}FAIL${NC}  $1 — $2"
    while IFS= read -r l; do printf '        %s\n' "$l"; done <<< "$3"
    fail=1
}

FILES=()
while IFS= read -r f; do [[ -f "$f" ]] && FILES+=("$f"); done < <(git ls-files -co --exclude-standard '*.sh')
(( ${#FILES[@]} )) || { echo "no shell scripts tracked"; exit 0; }

# check <what> <fix> <ERE matching the bad form>
check() {
    local hits
    hits=$(grep -nE "$3" "${FILES[@]}" 2>/dev/null | grep -vE '^scripts/(check-portability|lib/macos-probe)\.sh:') || true
    [[ -n "$hits" ]] && bad "$1" "$2" "$hits" || ok "$1"
    return 0
}

check 'bare sed -i'          'use sed_i from chain-env.sh; BSD reads -i as the suffix' 'sed +-i +[^.]'
check 'grep -P'              'BSD grep has no PCRE; use -E'                            '(^|[^a-z])grep [^|;]*-[a-zA-Z]*P[ "]'
check 'readlink -f/realpath' 'neither is on stock macOS; use cd + pwd'                 'readlink +-f|[^a-zA-Z_]realpath '
check 'GNU date arithmetic'  'BSD date uses -v, not -d'                                'date +(-d|--date)'
check 'sha256sum/md5sum'     'macOS has shasum -a 256 and md5'                         '(sha256|md5)sum'
check 'base64 -w'            'BSD base64 has no -w'                                    'base64 [^|;]*-w'
check 'stat -c'              'BSD stat uses -f'                                        'stat +-c'
check 'timeout'              'not on stock macOS; bound the tool itself instead'       '(^|[^-a-zA-Z_.])timeout +[0-9]'
check 'nproc/tac'            'neither is on stock macOS'                               '(^|[^-a-zA-Z_])(nproc|tac)([^a-zA-Z_]|$)'
check 'sed -r'               'BSD sed spells it -E'                                    'sed +[^|;]*-r[ "]'
check 'find -printf'         'BSD find has no -printf'                                 'find [^|;]*-printf'
check 'xargs -r'             'BSD xargs has no -r'                                     'xargs [^|;]*-r([ "]|$)'
check 'bash 4 only'          'macOS bash is 3.2'                                       'declare +-A|local +-A|mapfile|readarray|globstar'

# bash 3.2 + set -u: "${a[@]}" on an empty array aborts with "unbound variable".
# Safe if the file guards on ${#a[@]}, so only flag arrays that never do.
arrays=0
for f in "${FILES[@]}"; do
    grep -q 'set -[a-z]*u' "$f" || continue
    while IFS= read -r name; do
        [[ -n "$name" ]] || continue
        grep -q "\${#$name\[@\]}" "$f" && continue
        hit=$(grep -nE "\"\\$\{$name\[@\]\}\"" "$f" | head -1) || true
        [[ -n "$hit" ]] || continue
        bad "$f: \"\${$name[@]}\" can be empty" \
            "seed the array with the command, or guard on \${#$name[@]}" "$hit"
        arrays=1
    done < <(grep -ohE '[A-Za-z_][A-Za-z0-9_]*=\(\)' "$f" | sed 's/=()//' | sort -u)
done
(( arrays )) || ok 'no unguarded empty-array expansions'

# Grep only guesses at bash 3.2. Real 3.2 is one container away - the same
# 3.2.57 macOS ships. Skipped, not failed, when Docker is unavailable.
if docker info >/dev/null 2>&1; then
    tmp=$(mktemp -d)
    for f in "${FILES[@]}"; do
        mkdir -p "$tmp/$(dirname "$f")"
        tr -d $'\r' < "$f" > "$tmp/$f"   # a CRLF checkout fails for the wrong reason
    done
    mnt="$tmp"
    case "${OSTYPE:-}" in msys*|cygwin*) mnt=$(cygpath -w "$tmp"); export MSYS_NO_PATHCONV=1 ;; esac
    # The probe is the real gate: bash 3.2 plus BSD-userland shims. Kept in its
    # own file so CI can run it directly on a bash:3.2 image, without Docker.
    probe='apk add --no-cache git jq curl grep findutils >/dev/null 2>&1
git config --global --add safe.directory /w
exec bash scripts/lib/macos-probe.sh'
    if out=$(docker run --rm -v "$mnt":/w -w /w bash:3.2 bash -c "$probe" 2>&1); then
        ok 'runs under bash 3.2 with BSD shims'
    else
        bad 'bash 3.2' 'macOS ships bash 3.2.57' "$out"
    fi
    rm -rf "$tmp"
else
    echo "skip  bash 3.2 parse (docker not running)"
fi

echo
(( fail )) && { echo "${RED}portability check failed${NC}"; exit 1; }
echo "${GREEN}portable${NC}"
