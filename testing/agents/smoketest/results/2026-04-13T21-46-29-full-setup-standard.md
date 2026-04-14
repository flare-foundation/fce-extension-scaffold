# [Smoketest] Full Setup Standard
**Date:** 2026-04-13 21:46:29
**Scenario:** full-setup-standard
**Duration:** 2m 9s
**Result:** PASS

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
- 17th consecutive pass since proxy recovery. Pipeline stable across 8+ full rotations.
