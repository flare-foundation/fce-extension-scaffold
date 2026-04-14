# [Smoketest] Unicode Payload
**Date:** 2026-04-14 05:56:30
**Scenario:** unicode-payload
**Duration:** 2m 19s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~76s | Docker build + containers started |
| post-build | FAIL | ~45s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy during TEE registration availability check.

## Observations
- Third consecutive failure in third outage window (05:36–05:58, ~22 minutes). Recovery predicted ~07:06.
