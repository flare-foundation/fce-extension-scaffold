# [Smoketest] Verify Between Phases
**Date:** 2026-04-14 01:16:31
**Scenario:** verify-between-phases
**Duration:** 3m 5s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 20s | Contract deployed |
| verify (deploy+register) | PARTIAL | ~46s | 4 passed, 1 failed (R3 — known) |
| start-services | PASS | 73s | Docker build + containers started |
| verify (services) | PASS | <1s | 4 passed, 1 skipped |
| post-build | PASS | 19s | TEE registered |
| test | PASS | 20s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
verify-deploy R3 FAILED (known).

## Observations
- Pipeline stable post-recovery. 2nd consecutive pass since second outage recovery.
