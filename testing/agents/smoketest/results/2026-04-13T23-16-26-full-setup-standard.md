# [Smoketest] Full Setup Standard
**Date:** 2026-04-13 23:16:26
**Scenario:** full-setup-standard
**Duration:** 2m 18s
**Result:** PASS

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~76s | Docker build + containers started |
| post-build | PASS | ~22s | TEE registered |
| test | PASS | ~22s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- 26th consecutive pass since proxy recovery. Pipeline stable across 10+ full rotations.
