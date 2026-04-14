# [Smoketest] Full Setup Standard
**Date:** 2026-04-14 03:46:28
**Scenario:** full-setup-standard
**Duration:** 2m 11s
**Result:** PASS

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
- 17th consecutive pass since second outage recovery. Pipeline stable.
