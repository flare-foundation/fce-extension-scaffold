# [Smoketest] Verify Between Phases
**Date:** 2026-04-13 23:46:27
**Scenario:** verify-between-phases
**Duration:** 2m 13s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 19s | Contract deployed |
| start-services | PASS | 71s | Docker build + containers started |
| post-build | FAIL | 43s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy during TEE registration availability check.

## Observations
- Second consecutive failure in second outage window (started 23:36). Outage ~10 minutes so far.
- Verify steps between phases were skipped since the scenario failed at post-build before reaching test.
