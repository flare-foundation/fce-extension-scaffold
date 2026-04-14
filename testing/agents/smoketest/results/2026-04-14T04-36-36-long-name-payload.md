# [Smoketest] Long Name Payload
**Date:** 2026-04-14 04:36:36
**Scenario:** long-name-payload
**Duration:** 2m 9s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~70s | Docker build + containers started |
| post-build | PASS | ~20s | TEE registered |
| test | PASS | ~21s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Long name not actually tested (known tooling gap). 22nd consecutive pass since second outage recovery.
