# [Smoketest] Long Name Payload
**Date:** 2026-04-14 03:06:26
**Scenario:** long-name-payload
**Duration:** 2m 11s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~72s | Docker build + containers started |
| post-build | PASS | ~20s | TEE registered |
| test | PASS | ~21s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Long name not actually tested (known tooling gap). 13th consecutive pass since second outage recovery.
