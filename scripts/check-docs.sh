#!/usr/bin/env bash
# check-docs.sh — validate the shared docs standard (see docs/README.md).
#
# Checks presence, that the platform-wide traps are actually covered, that
# nothing has grown too long to be read by a tester, and that the mechanically
# checkable content claims (hex widths, json tags, command paths) are true. Content checks match on
# keywords rather than headings so rewording does not break them.
#
# Keep this file identical across extensions; the only per-repo difference is
# ONE_SHOT_SETTER below.
#
# Exit 1 on a missing/incomplete required doc; long docs only warn.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOCS="$(cd "$SCRIPT_DIR/.." && pwd)/docs"

RED='\033[0;31m'; YELLOW='\033[0;33m'; GREEN='\033[0;32m'; NC='\033[0m'
fail=0
err()  { echo -e "${RED}FAIL${NC}  $*"; fail=1; }
warn() { echo -e "${YELLOW}WARN${NC}  $*"; }
ok()   { echo -e "${GREEN}ok${NC}    $*"; }

# The one-shot binding differs per extension: shielded-transfer binds the vault's
# teeAuthority, orderbook binds the InstructionSender's teeAddress.
ONE_SHOT_SETTER="setExtensionId"

REQUIRED="getting-started.md deployment-steps.md testing.md testing-against-coston2.md architecture.md cloudflared.md"
SCAFFOLD="extension-guide.md instruction-sender.md manual-setup.md types-server.md"
MAX_LINES=400   # past this a tester stops reading; split or cut

echo "docs: $DOCS"
echo

for f in $REQUIRED; do
    p="$DOCS/$f"
    if [[ ! -f "$p" ]]; then err "$f missing (required)"; continue; fi
    n=$(wc -l <"$p")
    # 20 lines is about the floor for a real doc; below that it is a stub.
    if (( n < 20 )); then err "$f is only $n lines — stub?"; continue; fi
    (( n > MAX_LINES )) && warn "$f is $n lines (>$MAX_LINES) — trim or split"
    ok "$f ($n lines)"
done

echo
for f in $SCAFFOLD; do
    [[ -f "$DOCS/$f" ]] && ok "$f (scaffold)" || warn "$f missing — scaffold doc, expected"
done

# Platform traps every extension hits. Absent = the doc will not save anyone.
echo
D="$DOCS/deployment-steps.md"
if [[ -f "$D" ]]; then
    check() { grep -qiF "$2" "$D" && ok "deployment-steps: $1" || err "deployment-steps: missing $1 (looked for '$2')"; }
    check "stale-machine pausing"        "pause("
    check "active-machine listing"       "getActiveTeeMachines"
    check "one-shot binding warning"     "$ONE_SHOT_SETTER"
    check "launch-policy env override"   "allow_env_override"
    check "digest-pinned deploy"         "digest"
    check "attestation mode"             "SIMULATED_TEE"
fi

# --- content credibility: claims that can be checked mechanically ------------
# Structure passing means nothing if the claims are false. These catch the
# classes that are cheap to verify; signatures and semantics still need a human.
echo
SRC="$(cd "$SCRIPT_DIR/.." && pwd)"

# bytes32 literals are 66 chars incl. 0x; addresses are 42. Only flag literals
# NEAR those widths — hex-encoded payloads are legitimately any length.
while read -r h; do
    n=${#h}
    (( n == 66 || n == 42 )) && continue
    (( (n >= 60 && n <= 70) || (n >= 38 && n <= 44) ))         && err "hex literal ${h:0:14}… is $n chars (bytes32 is 66, address 42)"
done < <(grep -ohE '0x[0-9a-fA-F]+' "$DOCS"/*.md | sort -u)

# Every json tag shown in a doc code block must exist in the source. Scaffold
# docs describe the upstream Hello World example, so they only have to match in
# the scaffold repo itself — identified by its conformance fixtures.
TAG_DOCS=()
for f in "$DOCS"/*.md; do
    if [[ ! -d "$SRC/testdata/conformance" ]]; then
        case " $SCAFFOLD " in *" $(basename "$f") "*) continue ;; esac
    fi
    TAG_DOCS+=("$f")
done
while read -r t; do
    grep -rqF "$t" --include='*.go' --include='*.py' --include='*.ts' "$SRC" \
        || err "docs show $t — no such field in the source"
done < <(grep -ohE 'json:"[a-zA-Z0-9_]+"' "${TAG_DOCS[@]}" | sort -u)

# Scripts and tools named in docs must exist here, not just in a sibling.
while read -r c; do
    [[ -e "$SRC/$c" ]] && continue
    # A gitignored path is absent by design, not stale.
    git -C "$SRC" check-ignore -q "$c" 2>/dev/null && continue
    err "docs reference $c — not in this repo"
done < <(grep -ohE '((testing/)?scripts/[a-z0-9-]+\.sh|(go/)?tools/cmd/[a-z0-9-]+\*?)' "$DOCS"/*.md          | grep -v '[*-]$' | sed 's|^\./||' | sort -u)

(( fail )) || ok "content: hex widths, json tags and command paths check out"

echo
if [[ -f "$DOCS/README.md" ]]; then ok "docs/README.md index present"; else err "docs/README.md index missing"; fi

echo
if (( fail )); then echo -e "${RED}docs standard: FAILED${NC}"; else echo -e "${GREEN}docs standard: OK${NC}"; fi
exit $fail
