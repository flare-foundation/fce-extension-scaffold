# [Smoketest] Long Name Payload
**Date:** 2026-04-14 00:06:25
**Scenario:** long-name-payload
**Duration:** 2m 13s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~72s | Docker build + containers started |
| post-build | FAIL | ~43s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy during TEE registration availability check.

## Observations
- Fourth consecutive failure in second outage window (23:36–00:08, ~32 minutes).
