# [Smoketest] Long Name Payload
**Date:** 2026-04-14 10:36:35
**Scenario:** long-name-payload
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
- Thirty-first consecutive failure in third outage (05:36–10:38, 5h2m). Extended outage now exceeds 5 hours.
