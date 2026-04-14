# [Smoketest] Long Name Payload
**Date:** 2026-04-14 06:06:29
**Scenario:** long-name-payload
**Duration:** 2m 12s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~72s | Docker build + containers started |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Fourth consecutive failure in third outage window (05:36–06:08, ~32 minutes). Recovery predicted ~07:06.
