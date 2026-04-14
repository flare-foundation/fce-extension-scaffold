# [Smoketest] Full Setup Standard
**Date:** 2026-04-14 00:46:29
**Scenario:** full-setup-standard
**Duration:** 2m 11s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~70s | Docker build + containers started |
| post-build | FAIL | ~43s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy during TEE registration availability check.

## Observations
- Eighth consecutive failure in second outage window (23:36–00:48, ~72 minutes).
