# [Smoketest] Verify Between Phases
**Date:** 2026-04-13 19:16:29
**Scenario:** verify-between-phases
**Duration:** 3m 34s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 19s | Contract 0xd91e095b52208Ef1617D341424cAA0969C1c3628, Extension ID 0x1a9 (425) |
| verify (deploy+register) | PARTIAL | ~46s | 4 passed, 1 failed (R3 — known) |
| start-services | PASS | 73s | Docker build + containers started |
| verify (services) | PASS | <1s | 4 passed, 1 skipped (S9) |
| post-build | PASS | 19s | TEE 0xC37Cf27Bc2A7b8845Ca5fD654479b1d4De97772F registered |
| test | PASS | 21s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
verify-deploy R3 FAILED (known issue — deployer allowlist check).

## Observations
- Normal proxy continues working post-recovery. TEE registration succeeded cleanly.
- All deployment phases passed. Only the R3 verify-deploy check failed (known, systematic issue).
- Extension ID 425 (0x1a9). Pipeline fully recovered from the ~90min outage.
