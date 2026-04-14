# [Smoketest] Long Name Payload
**Date:** 2026-04-14 01:36:29
**Scenario:** long-name-payload
**Duration:** 2m 10s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~72s | Docker build + containers started |
| post-build | PASS | ~20s | TEE registered |
| test | PASS | ~20s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Long name not actually tested (known tooling gap). 4th consecutive pass since second outage recovery.
