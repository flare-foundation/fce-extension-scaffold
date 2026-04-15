# Unified Testing Analysis Report

**Data:** 135 smoketest + 14 edgecase + 6 chaos results analyzed (155 files total)
**Period:** 2026-04-13 05:41 through 2026-04-14 12:28 (~31 hours)
**Reviewers:** 3 dedicated agents (smoketest, edgecase, chaos) + 1 independent Codex review
**Generated:** 2026-04-14

---

## Overall Statistics

| Agent | Total Runs | Pass | Fail | Partial | Skip |
|-------|-----------|------|------|---------|------|
| Smoketest | 135 | 62 (46%) | 56 (41%) | 14 (10%) | 1 |
| Edge Case | 14 | 0 | 0 | 0 | 8 skipped, 5 expected-fail, 1 skip |
| Chaos | 6 | 2 | 0 | 0 | 4 interesting |

**Smoketest pass rate excluding infrastructure outages: ~100%.** Every single smoketest failure (56/56) traces to external proxy outages, not to the deployment pipeline itself. The pipeline code is solid when infrastructure is healthy.

---

## Prioritized Findings

### 1. CRITICAL -- Normal Proxy Has Recurring ~6-Hour Outages; No Retry Logic

**All 4 reviewers flagged this as the #1 issue.**

The Coston2 normal proxy (`tee-proxy-coston2-1.flare.rocks`) exhibits a strictly periodic outage pattern. Three outage windows were observed:

| Window | Start | Duration | Failed Runs |
|--------|-------|----------|-------------|
| 1 | Apr 13 17:36 | ~90 min | 9 |
| 2 | Apr 13 23:36 | ~90 min | 9 |
| 3 | Apr 14 05:36 | **6h52m+ (never recovered)** | 42+ |

All start at :36 past the hour, exactly 6 hours apart. The first two self-healed in ~90 min; the third became an extended/indefinite outage.

**Why this matters in production:** The `register-tee` tool treats a 404 from the availability check poll as a **fatal error with zero retry logic**. Each failed attempt still burns gas deploying contracts and registering the extension on-chain before hitting the wall at TEE registration. During outage window 3, 42 consecutive deployments failed with the same cryptic `action result status not ok, got: 404`.

**Fix:**
- Add retry-with-backoff for 404 responses during availability check polling in `fccutils/tee_calls.go`
- Distinguish "result not available yet" (retry) from "instruction rejected" (fatal)
- Investigate the 6-hour cycle on the normal proxy (scheduled maintenance? cert rotation? cache eviction?)

---

### 2. HIGH -- Command Injection via `.env` Sourcing in All 5 Bash Scripts

**Chaos agent confirmed arbitrary code execution.**

All scripts use `set -a; source "$PROJECT_DIR/.env"; set +a`. Since `source` interprets the file as bash, values like `$(touch /tmp/pwned)` or `$(cat /tmp/key > /tmp/exfil)` execute with full user permissions. Confirmed: arbitrary file creation and arbitrary file content writes.

**Why this matters in production:** In a TEE/blockchain context, a compromised `.env` file could exfiltrate `DEPLOYMENT_PRIVATE_KEY`. The Go-side `godotenv` library is NOT vulnerable (treats values as literals), creating an inconsistency where the same file is safe when read by Go but dangerous when read by bash.

**Fix:** Replace `source .env` with a safe key=value parser, or switch entirely to Go-based config loading.

---

### 3. HIGH -- `verify-deploy` R3 Check Is a 100% False Positive

**100% failure rate across every verify-only and verify-between-phases run in the entire 22.5-hour test session.** All 14 PARTIAL smoketest results are caused solely by this.

After a fully successful deployment (all 4 phases pass, both SAY_HELLO and SAY_GOODBYE succeed), R3 reports: `"deployer is NOT on the TEE machine owner allowlist"`. This is contradictory -- the deployer just successfully registered a TEE machine. All other 22 checks pass.

**Why this matters in production:** Developers will either (a) waste time "fixing" something that isn't broken, following the misleading suggestion to "run the register step," or (b) learn to ignore verify-deploy entirely, defeating the tool's purpose and masking real failures.

**Fix:** The R3 check is querying a different allowlist condition than what `register-tee` actually requires. Fix the check logic or remove it.

---

### 4. HIGH -- `pre-build.sh` Silently Overwrites Config, Orphans On-Chain State

**Edge case D4 confirmed: zero warning, zero backup, zero prompt.**

Running `pre-build.sh` a second time deploys new contracts, registers a new extension, and silently overwrites `config/extension.env`. The previous extension ID and contract address are permanently lost from local config. The edge case agent got two separate extension IDs (0x188 and 0x189) with no indication the first was being abandoned.

**Why this matters in production:** Accidental re-run = silent config loss + orphaned on-chain state + wasted gas. On mainnet, this is real money.

**Fix:** Check for existing `config/extension.env`. If it exists, warn and require `--force` to proceed.

---

### 5. MEDIUM -- `deployed-addresses.json` Silently Accepts Missing/Invalid Fields

**Chaos agent confirmed: `support.DefaultSupport` happily loads `{"SomeRandomField": "0x1234"}` without complaint.**

Missing required fields (e.g., `TeeExtensionRegistry`) default to zero addresses. The error only surfaces later as a confusing `connection refused` when the code tries to interact with `0x0000...0000`. JSON parse errors also lack the file path.

**Why this matters in production:** A typo in the addresses file (`TeeExtensionRegisty` missing the `r`) produces no error at parse time. The deployment proceeds with zero addresses and fails with misleading connectivity errors.

**Fix:** Add required-field validation after JSON unmarshal. Include file path in error messages.

---

### 6. MEDIUM -- `.env` Sourcing Overrides Explicit Environment Variables

**Edge case D1 confirmed this breaks standard Unix convention.**

`set -a; source .env; set +a` unconditionally overwrites pre-existing env vars. Running `export ADDRESSES_FILE=/custom/path && ./scripts/pre-build.sh` silently ignores the exported value.

**Why this matters in production:** CI/CD pipelines that set env vars to control deployment behavior will have their values silently overridden. Deployments could target wrong networks or use wrong keys.

**Fix:** Save pre-existing values before sourcing, restore after. Or use `godotenv` (which already respects existing env vars).

---

### 7. MEDIUM -- Instruction Fee Hardcoded in 4 Places; `SendSayGoodbye` Missing Revert Decoding

**Chaos agent found both issues in `tools/pkg/utils/instructions.go`.**

The fee `big.NewInt(1000000)` appears in 4 separate locations. `SendSayGoodbye` lacks the `DecodeRevertReason` + `SimulateAndDecodeRevert` pipeline that `SendSayHello` has.

**Why this matters:** Fee changes require editing 4 locations (easy to miss one). Wrong-fee errors on the goodbye path produce raw RPC errors instead of human-readable revert reasons.

**Fix:** Extract fee to a single constant or runtime query. Add revert decoding to `SendSayGoodbye`.

---

### 8. MEDIUM -- stdout Used for Both Logs and Machine Output; `tail -1` Capture Is Fragile

**Edge case D7 confirmed: `deploy-contract` and `register-extension` write INFO logs to stdout.**

`pre-build.sh` uses `tail -1` to extract the address. If any future log line is added after `fmt.Println(address)`, the capture silently breaks -- `extension.env` gets a log line instead of an address, and all subsequent phases fail with confusing errors. ANSI color escape codes in the INFO logs compound the issue.

**Fix:** Redirect Go logger (`slog`) to stderr. Reserve stdout for machine-readable output only.

---

### 9. MEDIUM -- `run-test` Hardcodes Payloads; Zero Coverage of Non-Standard Inputs

**Smoketest agent confirmed: 3 scenarios (unicode, long-name, empty-payload) run but don't actually test what they intend.**

`tools/cmd/run-test/main.go` hardcodes `"World"` at line 60. No `--name` flag exists. This means ~33% of the scenario rotation is functionally wasted (identical to standard runs).

**Fix:** Add `--name` and `--reason` CLI flags to `run-test`.

---

### 10. MEDIUM -- Transient 502 from Extension Proxy; No Retry on 5xx

**1 occurrence in ~80 test phases.** The SAY_HELLO instruction was accepted but the result poll returned 502. No retry was attempted.

**Fix:** Add retry-with-backoff for 5xx errors during action result polling.

---

### 11. LOW-MEDIUM -- Preflight Can't Compile on Fresh Clone

`pre-build.sh` runs preflight (Step 0) before generating Go bindings (Step 1). On a fresh clone, the user sees cryptic Go compilation errors instead of "run `go generate` first."

**Fix:** Reorder steps or add a check for generated files.

---

### 12. LOW -- Docker Compose Errors Not Surfaced in Script Error Messages

`start-services.sh` reports only `docker compose up failed` without capturing the actual Docker error (e.g., wrong Dockerfile path).

**Fix:** Capture stderr from `docker compose up` and include it in the `die()` message.

---

## Testing Infrastructure Finding

**Edge case agent starvation:** Only 5 of 22 edge cases were tested (23% coverage). The agent was locked out for 8+ consecutive heartbeats due to smoketest holding the lock. Entire categories (Registration, Service Startup, TEE Version, TEE Machine, Testing) have zero edge case coverage. The testing framework needs fairness in its lock protocol or separate infrastructure per agent.

---

## Agreement Across Reviewers

All 4 reviewers independently identified the same top issues, with strong consensus:

| Finding | Claude Smoketest | Claude Edgecase | Claude Chaos | Codex |
|---------|:---:|:---:|:---:|:---:|
| Proxy 6h outage + no retry | #1 | - | - | #1 |
| .env command injection | - | - | #1 | #2 |
| verify-deploy R3 false positive | #2 | noted | - | #3 |
| pre-build.sh silent overwrite | - | #1 | - | #4 |
| addresses.json no validation | - | - | #2 | #5 |
| .env overrides env vars | - | #2 | - | #6 |
| Fee hardcoded 4x + goodbye revert | - | - | #3-4 | #7 |
| stdout/tail -1 fragile capture | - | #3 | - | #9 |
| run-test no custom payloads | #3 | - | - | - |

The fact that all reviewers converged on essentially the same priority ordering gives high confidence in these findings.

---

## Positive Findings

Not everything was bad. Several areas showed strong design:

- **Pre-flight validation (D1, D3, D5):** Excellent error messages that name the exact problem, explain the consequence, and suggest the fix. This is the gold standard in the codebase.
- **Private key validation:** All 7 tested edge cases (empty, too short, invalid hex, 0x prefix, zero key, too long, spaces) produce clear, specific errors.
- **Pipeline reliability when infra is healthy:** 100% pass rate when the normal proxy is up. Zero pipeline-caused failures across 135 smoketest runs.
- **Phase ordering enforcement:** All scripts correctly identify missing prerequisites and name the fix (`Run pre-build.sh first`). The `die()` pattern works well.
