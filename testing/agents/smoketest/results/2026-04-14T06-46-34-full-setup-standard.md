# [Smoketest] Full Setup Standard
**Date:** 2026-04-14 06:46:34
**Scenario:** full-setup-standard
**Duration:** 1m 29s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~30s | Docker build (cached) + containers started |
| post-build | FAIL | ~41s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Eighth consecutive failure in third outage window (05:36–06:48, ~72 minutes). Recovery predicted ~07:06 (~18 minutes).
