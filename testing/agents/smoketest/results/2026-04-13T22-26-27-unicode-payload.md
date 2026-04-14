# [Smoketest] Unicode Payload
**Date:** 2026-04-13 22:26:27
**Scenario:** unicode-payload
**Duration:** 2m 23s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~78s | Docker build + containers started |
| post-build | PASS | ~22s | TEE registered |
| test | PASS | ~25s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Unicode not actually tested (known tooling gap). 21st consecutive pass since proxy recovery.
