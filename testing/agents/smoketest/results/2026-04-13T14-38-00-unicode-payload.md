# [Smoketest] Unicode Payload
**Date:** 2026-04-13 14:38:00
**Scenario:** unicode-payload
**Duration:** 1m 58s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract 0x5E61...2b6C, Extension ID 0x18b |
| start-services | PASS | ~68s | Docker build + proxy ready |
| post-build | PASS | ~14s | TEE 0x186c...cAe3 registered |
| test | PASS | ~16s | SAY_HELLO + SAY_GOODBYE passed (ASCII name "World") |

## Errors
None.

## Observations
- **Cannot test unicode payloads**: The `run-test` tool hardcodes `"name": "World"` at line 60 of `tools/cmd/run-test/main.go`. There is no CLI flag to pass a custom name string.
- The SAY_GOODBYE instruction also hardcodes `"World"` and `"heading out"` at line 83.
- No encoding-related warnings appeared in any phase output.
- **Limitation**: To actually test unicode payloads, `run-test` would need a `--name` flag or similar. This is a gap in the test tooling — the scenario cannot be fully executed without modifying source code (which the smoketest agent must not do).
- The standard ASCII test passed cleanly, confirming the deployment flow itself is healthy after the 502 failure in scenario #4.
