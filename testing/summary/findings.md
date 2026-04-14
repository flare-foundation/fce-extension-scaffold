# Findings

Notable bugs, unclear error messages, and unexpected behavior discovered by the testing agents.

---

## [Chaos] All bash scripts vulnerable to command injection via .env sourcing
**Date:** 2026-04-13
**Severity:** Medium-High
**Scenario:** invented: env-injection-via-dotenv

All 5 deployment scripts (`pre-build.sh`, `start-services.sh`, `post-build.sh`, `test.sh`, `stop-services.sh`) use `set -a; source "$PROJECT_DIR/.env"; set +a` to load configuration. The `source` command executes `.env` as a bash script, meaning shell syntax like `$(touch /tmp/pwned)` or backtick-wrapped commands in env values are executed with the user's full permissions. Confirmed: arbitrary file creation, arbitrary file content writes, and all scripts affected. The Go-level `godotenv` library is NOT vulnerable (treats values as literals). **Fix:** Replace `source .env` with a safe key=value parser, or validate `.env` contents before sourcing.

---

## [Chaos] start-services.sh docker compose error message is generic
**Date:** 2026-04-13
**Severity:** Minor
**Scenario:** wrong-dockerfile-path (#12)

When `docker compose up` fails during `start-services.sh`, the script reports only `docker compose up failed` without capturing or surfacing the underlying Docker build error. The actual error (e.g., `failed to read dockerfile: open Dockerfile.WRONG: no such file or directory`) scrolls by in build output but isn't included in the final die message. Consider capturing docker compose stderr and including it in the error.

---

## [Chaos] Missing field validation in deployed-addresses.json parsing
**Date:** 2026-04-13
**Severity:** Medium
**Scenario:** invented: corrupt-addresses-json

`support.DefaultSupport` silently accepts a JSON file with missing required fields (e.g., `TeeExtensionRegistry`, `TeeMachineRegistry`). Missing fields default to zero-value addresses. The error only surfaces later at chain interaction time with an unrelated "connection refused" or when `validate.AddressNotZero` runs. The JSON loading layer should validate that required fields are present and non-empty before proceeding.

Additionally, JSON parse errors (`unexpected end of JSON input`, `invalid character`) don't include the file path, making debugging harder when multiple addresses files exist.

---

## [Chaos] Pre-flight check fails to compile without generated bindings
**Date:** 2026-04-13
**Severity:** Minor
**Scenario:** invented: corrupt-addresses-json

`pre-build.sh` runs the preflight check (Step 0) before generating Go bindings (Step 1). The preflight uses `go run ./cmd/deploy-contract` which imports the `helloworld` generated bindings package. On a fresh clone, the preflight fails with Go compilation errors (`undefined: helloworld.HelloWorldInstructionSender`) instead of a helpful message. Consider reordering to generate bindings before running the preflight, or adding a check for the generated files.

---

## [Edge Case] .env file overrides explicit environment variables in pre-build.sh
**Date:** 2026-04-13
**Severity:** Minor
**Scenario:** D1-wrong-registry

`pre-build.sh` sources `.env` using `set -a; source .env; set +a` (lines 23-27), which overwrites any `ADDRESSES_FILE` already set in the environment. This means `export ADDRESSES_FILE=/some/path && ./scripts/pre-build.sh` ignores the exported value. Standard convention is for explicit env vars to take precedence over `.env` file defaults. Fix: check if the variable is already set before sourcing, or source first and then apply overrides.

---

## [Edge Case] D1 pre-flight catches wrong registry before deployment (better than documented)
**Date:** 2026-04-13
**Severity:** Informational (positive)
**Scenario:** D1-wrong-registry

The edge case doc expected "deployment succeeds but registration fails." In practice, the pre-flight check (`--preflight-only`) catches a wrong TeeExtensionRegistry address before any gas is spent. Error message quality is GOOD — it names the address, explains why it matters (immutable in constructor), and suggests checking the addresses file/network.

---

## [Smoketest] run-test hardcodes payload names — no custom input possible
**Date:** 2026-04-13
**Severity:** Minor (tooling gap)
**Scenario:** unicode-payload (#5)

`tools/cmd/run-test/main.go` hardcodes the name `"World"` for SAY_HELLO (line 60) and `"World"`, `"heading out"` for SAY_GOODBYE (line 83). There is no CLI flag to customize the payload content. This prevents testing unicode payloads, long strings, empty strings, or other edge cases without modifying source code. Adding a `--name` and `--reason` flag to `run-test` would enable scenarios #5-#7 from the smoketest suite.

---

## [Smoketest] 502 error on test phase — possible proxy/TEE flakiness
**Date:** 2026-04-13
**Severity:** Medium
**Scenario:** verify-between-phases (#4)

During the verify-between-phases scenario, the test phase received a 502 error when polling for the SAY_HELLO action result. The instruction was sent and assigned an ID (`0x963d...2d65`), but the proxy returned HTTP 502 when fetching the result. This same flow succeeded in scenario #2 (step-by-step) with the same phase ordering. The only difference was additional verify-deploy checks between phases, which added ~10s of delay. Possible causes: ngrok tunnel instability, extension-tee container crash after post-build, or a timing-related race condition.

---

## [Edge Case] deploy-contract/register-extension write INFO logs to stdout, making tail -1 capture fragile
**Date:** 2026-04-13
**Severity:** Minor
**Scenario:** D7-output-capture

Both `deploy-contract` and `register-extension` Go commands write their INFO-level structured logs to **stdout** (7 lines), with the clean address/ID as the final line. `pre-build.sh` uses `tail -1` to extract the value, which works today but is fragile — any future log line added after the `fmt.Println(address)` would silently break the capture. The Go logger should write to stderr so stdout is reserved for machine-readable output. Additionally, the INFO logs contain ANSI color escape codes which could interfere with other tooling parsing stdout.

---

## [Chaos] Instruction fee hardcoded as magic number in 4 places; SendSayGoodbye lacks revert decoding
**Date:** 2026-04-13
**Severity:** Medium
**Scenario:** wrong-fee (scenario 14)

The instruction fee (1000000 wei) is hardcoded as a `big.NewInt(1000000)` magic number in 4 separate locations in `tools/pkg/utils/instructions.go`. If the on-chain registry ever changes its required fee, all 4 must be updated manually — easy to miss one.

Additionally, `SendSayGoodbye` lacks the revert-reason decoding that `SendSayHello` has. When `SendSayHello` gets a fee-related revert, it runs `DecodeRevertReason` + `SimulateAndDecodeRevert` to produce a clear message like `"failed to send instruction: ... (revert reason: insufficient fee)"`. `SendSayGoodbye` just returns the raw error with no revert decoding, producing a less helpful message.

**Recommendation:** Extract the fee to a single constant or query it from the registry at runtime. Add revert-reason decoding to `SendSayGoodbye` to match `SendSayHello`.

---

## [Smoketest] verify-deploy R3 check fails despite successful TEE registration
**Date:** 2026-04-13
**Severity:** Medium
**Scenario:** verify-only (#9)

After a fully successful deployment (all 4 phases pass, both SAY_HELLO and SAY_GOODBYE tests succeed), `verify-deploy` reports check R3 as FAILED: "deployer is NOT on the TEE machine owner allowlist." This is contradictory — the post-build phase successfully registered a TEE machine using the deployer key (TEE ID `0x2D094c88181423dAd54cAFD819237c788AB49CEe`), which requires owner permissions. Either the R3 check is looking at a different allowlist condition than what `register-tee` uses, or the allowlist state changed between registration and verification. All other 22 checks passed. This may indicate a verify-deploy bug in the R3 check logic.

---

## [Smoketest] TEE registration 404 from normal proxy — transient infrastructure failure
**Date:** 2026-04-13
**Severity:** Medium
**Scenario:** rapid-cycle (#3, 4th rotation)

TEE registration fails during availability check result polling. The pre-registration and TEE attestation both succeed, but when polling for the availability check result via the normal proxy (`tee-proxy-coston2-1.flare.rocks`), a 404 is returned after ~33s of waiting. Error: `action result status not ok, got: 404`. Three outage windows observed:
1. **17:36–19:06** (~90 minutes, 9 consecutive failures)
2. **23:36–01:06** (~90 minutes, 9 consecutive failures)
3. **05:36–ongoing** (6h12m+ and counting, 38+ consecutive failures)

Outages start every 6 hours on the :36 minute mark. First two lasted ~90 minutes each, but the third has become an extended/indefinite outage (6h+ and counting). The normal proxy (`tee-proxy-coston2-1.flare.rocks`) returns 404 on TEE registration availability check polling. The `register-tee` tool treats a 404 as a fatal error with no retry logic — adding a retry with backoff could improve resilience.

---

## [Edge Case] pre-build.sh silently overwrites extension.env on re-run, orphans on-chain state
**Date:** 2026-04-13
**Severity:** Medium
**Scenario:** D4-double-deploy

Running `pre-build.sh` twice deploys two separate InstructionSender contracts and registers two separate extensions on-chain. The second run silently overwrites `config/extension.env` with the new values — no warning, no backup, no prompt. The previous extension ID and contract address are lost. On a live network (Coston2), this wastes gas and creates orphaned on-chain state. The script should check for an existing `extension.env` and either warn or require `--force` to proceed.

---
