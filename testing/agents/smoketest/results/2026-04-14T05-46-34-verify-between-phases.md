# [Smoketest] Verify Between Phases
**Date:** 2026-04-14 05:46:34
**Scenario:** verify-between-phases
**Duration:** 2m 10s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~70s | Docker build + containers started |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy during TEE registration availability check.

## Observations
- Second consecutive failure in third outage window (05:36–05:48, ~12 minutes). Expected recovery ~07:06.
