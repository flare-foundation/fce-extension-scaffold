# [Smoketest] Full Setup Standard
**Date:** 2026-04-14 05:16:32
**Scenario:** full-setup-standard
**Duration:** 2m 10s
**Result:** PASS

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~72s | Docker build + containers started |
| post-build | PASS | ~20s | TEE registered |
| test | PASS | ~20s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- 26th consecutive pass since second outage recovery. No third outage at predicted ~05:36 window — the 6h pattern may not be strictly periodic, or the outages may have been a one-time event.
