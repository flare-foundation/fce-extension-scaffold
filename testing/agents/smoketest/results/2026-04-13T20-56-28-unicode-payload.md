# [Smoketest] Unicode Payload
**Date:** 2026-04-13 20:56:28
**Scenario:** unicode-payload
**Duration:** 2m 16s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~74s | Docker build + containers started |
| post-build | PASS | ~22s | TEE registered |
| test | PASS | ~22s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Unicode not actually tested (known tooling gap). 12th consecutive pass since proxy recovery.
