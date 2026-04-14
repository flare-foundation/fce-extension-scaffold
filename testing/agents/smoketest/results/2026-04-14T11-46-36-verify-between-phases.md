# [Smoketest] Verify Between Phases
**Date:** 2026-04-14 11:46:36
**Scenario:** verify-between-phases
**Duration:** 1m 36s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~36s | Docker build (cached) |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Thirty-eighth consecutive failure in third outage (05:36–11:48, 6h12m). Extended outage continues.
