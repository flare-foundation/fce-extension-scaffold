# [Smoketest] Unicode Payload
**Date:** 2026-04-14 10:26:36
**Scenario:** unicode-payload
**Duration:** 1m 32s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~32s | Docker build (cached) |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Thirtieth consecutive failure in third outage (05:36–10:28, 4h52m). Approaching 5 hours — this is a major extended outage.
