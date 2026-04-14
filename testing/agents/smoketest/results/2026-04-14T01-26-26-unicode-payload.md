# [Smoketest] Unicode Payload
**Date:** 2026-04-14 01:26:26
**Scenario:** unicode-payload
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
- Unicode not actually tested (known tooling gap). 3rd consecutive pass since second outage recovery.
