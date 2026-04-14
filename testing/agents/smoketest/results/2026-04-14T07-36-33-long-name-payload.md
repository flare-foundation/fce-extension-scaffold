# [Smoketest] Long Name Payload
**Date:** 2026-04-14 07:36:33
**Scenario:** long-name-payload
**Duration:** 1m 31s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~31s | Docker build (cached) |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Thirteenth consecutive failure in third outage window (05:36–07:38, ~122 minutes). Third outage now 35% longer than the first two (~90min). This may indicate a more severe issue or a different failure mode than the first two outages.
