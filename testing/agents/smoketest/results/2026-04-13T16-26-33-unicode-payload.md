# [Smoketest] Unicode Payload
**Date:** 2026-04-13 16:26:33
**Scenario:** unicode-payload
**Duration:** 2m 9s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~17s | Contract 0x3c193A2B1415A35A8867CcF36a365Cc374348CE1, Extension ID 0x195 |
| start-services | PASS | ~73s | Docker build + containers started, proxy ready in 2s |
| post-build | PASS | ~17s | TEE 0xDeAD2dF7D996bC7EBE0c7C8e1EdAC7126079de83 registered |
| test | PASS | ~21s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- **Unicode not actually tested:** The `run-test` tool hardcodes the name `"World"` for SAY_HELLO and `"World"`, `"heading out"` for SAY_GOODBYE. There is no CLI flag to pass custom payloads. This is a known tooling gap (previously logged as a finding). Until `--name`/`--reason` flags are added to `run-test`, unicode payload testing cannot be performed without modifying source code (which the smoketest agent is not allowed to do).
- No encoding-related warnings appeared in any phase output.
- The response data shows clean ASCII: `{Greeting:Hello, World! Welcome to Flare Confidential Compute. GreetingNumber:1}` — no mojibake or encoding issues with the default payload.
- This scenario functionally equivalent to full-setup-standard until `run-test` gains custom payload support.
