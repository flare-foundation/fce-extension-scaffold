# [Smoketest] Long Name Payload
**Date:** 2026-04-13 16:36:23
**Scenario:** long-name-payload
**Duration:** 2m 15s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract 0xAEF391753CbF0689920a7A94337b690820D68Ee4, Extension ID 0x196 |
| start-services | PASS | ~74s | Docker build + containers started, proxy ready in 2s |
| post-build | PASS | ~20s | TEE 0x94deD92183876f6e6e715f5Ca95E64150AEB67ac registered |
| test | PASS | ~20s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- **Long name not actually tested:** The `run-test` tool hardcodes the name `"World"` with no CLI flag for custom payloads. A 1000-character name test requires either modifying source code (not allowed for smoketest agent) or adding `--name`/`--reason` flags to `run-test`. This is a known tooling gap (previously logged as a finding).
- Standard test with default "World" payload passed cleanly.
- This scenario is functionally equivalent to full-setup-standard until `run-test` gains custom payload support.
- Extension ID incremented to 406 (0x196), consistent with sequential on-chain registration.
