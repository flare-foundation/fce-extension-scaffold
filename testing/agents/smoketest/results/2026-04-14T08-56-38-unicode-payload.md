# [Smoketest] Unicode Payload
**Date:** 2026-04-14 08:56:38
**Scenario:** unicode-payload
**Duration:** 1m 34s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~34s | Docker build (cached) |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Twenty-first consecutive failure in third outage (05:36–08:58, 3h22m). Extended outage continues.
