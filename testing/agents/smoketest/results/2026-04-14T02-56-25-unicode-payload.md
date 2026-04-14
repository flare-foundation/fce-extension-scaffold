# [Smoketest] Unicode Payload
**Date:** 2026-04-14 02:56:25
**Scenario:** unicode-payload
**Duration:** 2m 26s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~80s | Docker build + containers started |
| post-build | PASS | ~24s | TEE registered |
| test | PASS | ~24s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Unicode not actually tested (known tooling gap). 12th consecutive pass since second outage recovery.
