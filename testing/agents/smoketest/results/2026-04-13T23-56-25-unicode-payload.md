# [Smoketest] Unicode Payload
**Date:** 2026-04-13 23:56:25
**Scenario:** unicode-payload
**Duration:** 2m 22s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~78s | Docker build + containers started |
| post-build | FAIL | ~46s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy during TEE registration availability check.

## Observations
- Third consecutive failure in second outage window (23:36–23:58, ~22 minutes so far).
