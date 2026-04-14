# [Smoketest] Long Name Payload
**Date:** 2026-04-13 21:06:26
**Scenario:** long-name-payload
**Duration:** 2m 8s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~70s | Docker build + containers started |
| post-build | PASS | ~20s | TEE registered |
| test | PASS | ~20s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Long name not actually tested (known tooling gap). 13th consecutive pass since proxy recovery.
