# [Smoketest] Full Setup Standard
**Date:** 2026-04-13 20:16:29
**Scenario:** full-setup-standard
**Duration:** 2m 25s
**Result:** PASS

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~80s | Docker build + containers started |
| post-build | PASS | ~22s | TEE registered |
| test | PASS | ~25s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- 8th consecutive pass since proxy recovery. Pipeline stable.
- Slightly longer total time (145s vs typical ~130s) — within normal variance.
