# [Smoketest] Verify Between Phases
**Date:** 2026-04-14 07:16:35
**Scenario:** verify-between-phases
**Duration:** 1m 33s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~33s | Docker build (cached) |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Eleventh consecutive failure in third outage window (05:36–07:18, ~102 minutes). Third outage is longer than the first two (~90min each). Duration varies between outage cycles.
