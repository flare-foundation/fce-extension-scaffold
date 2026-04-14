# [Smoketest] Full Setup Standard
**Date:** 2026-04-14 09:46:33
**Scenario:** full-setup-standard
**Duration:** 1m 35s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~35s | Docker build (cached) |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same 404 from normal proxy.

## Observations
- Twenty-sixth consecutive failure in third outage (05:36–09:48, 4h12m). Extended outage continues with no recovery.
