# [Smoketest] Verify Between Phases
**Date:** 2026-04-13 20:46:31
**Scenario:** verify-between-phases
**Duration:** 3m 36s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 17s | Extension ID 0x1b3 (435) |
| verify (deploy+register) | PARTIAL | ~46s | 4 passed, 1 failed (R3 — known) |
| start-services | PASS | 70s | Docker build + containers started |
| verify (services) | PASS | <1s | 4 passed, 1 skipped (S9) |
| post-build | PASS | 22s | TEE registered |
| test | PASS | 21s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
verify-deploy R3 FAILED (known — deployer allowlist check).

## Observations
- 11th consecutive pass since proxy recovery. Pipeline stable.
