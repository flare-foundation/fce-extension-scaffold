# [Smoketest] Full Setup Standard
**Date:** 2026-04-14 08:16:37
**Scenario:** full-setup-standard
**Duration:** 1m 35s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~35s | Docker build (cached) |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Seventeenth consecutive failure in third outage window (05:36–08:18, ~162 minutes). Third outage now nearly 2x the first two.
