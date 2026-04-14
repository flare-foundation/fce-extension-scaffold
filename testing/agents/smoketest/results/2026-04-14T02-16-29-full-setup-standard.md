# [Smoketest] Full Setup Standard
**Date:** 2026-04-14 02:16:29
**Scenario:** full-setup-standard
**Duration:** 2m 17s
**Result:** PASS

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~76s | Docker build + containers started |
| post-build | PASS | ~22s | TEE registered |
| test | PASS | ~21s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- 8th consecutive pass since second outage recovery. Pipeline stable.
