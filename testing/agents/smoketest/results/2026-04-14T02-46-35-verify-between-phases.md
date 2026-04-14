# [Smoketest] Verify Between Phases
**Date:** 2026-04-14 02:46:35
**Scenario:** verify-between-phases
**Duration:** 3m 6s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 16s | Contract deployed |
| verify (deploy+register) | PARTIAL | ~46s | 4 passed, 1 failed (R3 — known) |
| start-services | PASS | 70s | Docker build + containers started |
| verify (services) | PASS | <1s | 4 passed, 1 skipped |
| post-build | PASS | 25s | TEE registered |
| test | PASS | 22s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
verify-deploy R3 FAILED (known).

## Observations
- 11th consecutive pass since second outage recovery. Pipeline stable.
