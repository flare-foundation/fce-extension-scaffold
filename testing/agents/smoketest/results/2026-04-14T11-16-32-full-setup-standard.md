# [Smoketest] Full Setup Standard
**Date:** 2026-04-14 11:16:32
**Scenario:** full-setup-standard
**Duration:** 1m 37s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~37s | Docker build (cached) |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Thirty-fifth consecutive failure in third outage (05:36–11:18, 5h42m). Extended outage — now approaching 6h, the interval between the first two outage starts. The third outage has merged what would have been separate outage cycles into one continuous downtime.
