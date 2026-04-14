# [Smoketest] Verify Between Phases
**Date:** 2026-04-13 14:28:05
**Scenario:** verify-between-phases
**Duration:** 2m 52s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 22s | Contract 0xEb57...C3a3, Extension ID 0x18a |
| verify-deploy (deploy) | PASS | ~5s | 9/9 checks passed |
| verify-deploy (register) | PARTIAL | ~5s | 4 pass, 1 fail (R3: deployer not on TEE machine owner allowlist — expected at this stage) |
| start-services | PASS | 70s | Docker build + proxy ready |
| verify-deploy (services) | PASS | ~3s | 4 pass, 0 fail, 1 skip (S9 skipped) |
| post-build | PASS | 18s | TEE 0xe139...01D9 registered |
| test | FAIL | 42s | 502 error on SAY_HELLO action result |

## Errors
```
FATAL fccutils/encoding.go:27  Error: action result status not ok, got: 502
  for instruction 0x963ddf801462648839df5b78f5cfbc97f1f9705ad17f2aa9a258e1ee46122d65
  proxy: https://tribunal-pep-turbulent.ngrok-free.dev
```

The SAY_HELLO instruction was sent and accepted (got instruction ID), but the action result poll returned HTTP 502 from the proxy. This suggests the extension-tee container may have crashed or become unresponsive between instruction submission and result retrieval.

## Observations
- R3 (deployer is allowed TEE machine owner) correctly fails after pre-build but before post-build — this is expected sequencing behavior, not a bug.
- S9 (EXT_PROXY_URL reachable) was skipped in services verification — unclear why, may need investigation.
- The 502 failure is intermittent — this exact flow (pre-build -> start -> post-build -> test) succeeded in scenario #2 (step-by-step). Possible flakiness in the TEE proxy or ngrok tunnel.
- Time between start-services and test was longer in this scenario (~90s for verify checks + post-build) vs step-by-step (~34s). The delay may have contributed to proxy instability.
