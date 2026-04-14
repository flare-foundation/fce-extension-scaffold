# [Smoketest] Verify Between Phases
**Date:** 2026-04-13 22:16:29
**Scenario:** verify-between-phases
**Duration:** 3m 20s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 15s | Contract deployed |
| verify (deploy+register) | PARTIAL | ~46s | 4 passed, 1 failed (R3 — known) |
| start-services | PASS | 70s | Docker build + containers started |
| verify (services) | PASS | <1s | 4 passed, 1 skipped (S9) |
| post-build | PASS | 18s | TEE registered |
| test | PASS | 24s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
verify-deploy R3 FAILED (known).

## Observations
- 20th consecutive pass since proxy recovery. Pipeline stable.
