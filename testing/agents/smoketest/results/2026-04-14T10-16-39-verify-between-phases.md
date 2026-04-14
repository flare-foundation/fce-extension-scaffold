# [Smoketest] Verify Between Phases
**Date:** 2026-04-14 10:16:39
**Scenario:** verify-between-phases
**Duration:** 1m 41s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~40s | Docker build (cached) |
| post-build | FAIL | ~43s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Twenty-ninth consecutive failure in third outage (05:36–10:18, 4h42m). Extended outage continues.
