# [Smoketest] Verify Between Phases
**Date:** 2026-04-14 08:46:34
**Scenario:** verify-between-phases
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
- Twentieth consecutive failure in third outage window (05:36–08:48, ~192 minutes / 3h12m). Extended outage continues.
