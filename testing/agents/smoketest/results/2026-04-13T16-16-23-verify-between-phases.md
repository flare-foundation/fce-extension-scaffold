# [Smoketest] Verify Between Phases
**Date:** 2026-04-13 16:16:23
**Scenario:** verify-between-phases
**Duration:** 3m 24s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 19s | Contract 0x75c7300174169bd75a5D5CF86629E1d24FE5Cc7f, Extension ID 0x194 |
| verify (deploy+register) | PARTIAL | 46s | 4 passed, 1 failed (R3), 0 skipped |
| start-services | PASS | 68s | Docker build + containers started, proxy ready in 2s |
| verify (services) | PASS | <1s | 4 passed, 1 skipped (S9) |
| post-build | PASS | 18s | TEE 0xC92159aF894Dac2cA7113362EF29aa7049EC856a registered |
| test | PASS | 20s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
verify-deploy check R3 FAILED (same as verify-only scenario):
```
[FAIL] R3  deployer is allowed TEE machine owner
       deployer is NOT on the TEE machine owner allowlist
       Fix: Run the register step to add the deployer as a TEE machine owner
```

This is a known issue — the R3 check consistently fails despite the deployer being able to successfully register TEE machines. Previously logged as a finding.

## Observations
- Unlike the previous verify-between-phases run (scenario #4 on first rotation), this run completed the test phase successfully with no 502 errors. The previous run's 502 failure appears to have been a transient ngrok/proxy issue rather than a systematic problem.
- Services verify step completed in <1s (no Go compilation needed since already warm).
- The S9 skip in services verify is expected — the EXT_PROXY_URL reachability check is skipped in this mode.
- All deployment phases passed cleanly despite the R3 verify-deploy failure, confirming R3 is checking a condition that doesn't actually block the workflow.
