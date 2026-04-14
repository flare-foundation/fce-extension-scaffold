# [Smoketest] Verify Between Phases
**Date:** 2026-04-14 04:16:33
**Scenario:** verify-between-phases
**Duration:** 3m 9s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 17s | Contract deployed |
| verify (deploy+register) | PARTIAL | ~46s | 4 passed, 1 failed (R3 — known) |
| start-services | PASS | 70s | Docker build + containers started |
| verify (services) | PASS | <1s | 4 passed, 1 skipped |
| post-build | PASS | 26s | TEE registered |
| test | PASS | 21s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
verify-deploy R3 FAILED (known).

## Observations
- 20th consecutive pass since second outage recovery. Pipeline stable.
