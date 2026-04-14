# [Smoketest] Long Name Payload
**Date:** 2026-04-13 19:36:25
**Scenario:** long-name-payload
**Duration:** 2m 22s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~78s | Docker build + containers started |
| post-build | PASS | ~22s | TEE registered |
| test | PASS | ~24s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Long name not actually tested — `run-test` hardcodes "World" with no custom payload flag (known tooling gap).
- Pipeline stable. 4th consecutive pass since proxy recovery.
