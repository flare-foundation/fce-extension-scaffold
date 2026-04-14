# [Smoketest] Unicode Payload
**Date:** 2026-04-14 07:26:29
**Scenario:** unicode-payload
**Duration:** 1m 39s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~38s | Docker build (cached) |
| post-build | FAIL | ~43s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Twelfth consecutive failure in third outage window (05:36–07:28, ~112 minutes). Third outage significantly longer than first two (~90min). The outage duration is not strictly fixed.
