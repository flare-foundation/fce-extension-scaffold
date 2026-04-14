# [Smoketest] Verify Between Phases
**Date:** 2026-04-13 17:46:34
**Scenario:** verify-between-phases
**Duration:** 2m 49s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | 15s | Contract 0x7E22dF0d2940cE65cE147D642b4bDd579735DAf1, Extension ID 0x19d |
| verify (deploy+register) | PARTIAL | ~46s | 4 passed, 1 failed (R3 — known) |
| start-services | PASS | 71s | Docker build + containers started, proxy ready |
| verify (services) | PASS | <1s | 4 passed, 1 skipped (S9) |
| post-build | FAIL | 43s | TEE registration failed — 404 from normal proxy |
| test | SKIPPED | - | Not reached due to post-build failure |

## Errors
Same error as previous rapid-cycle failure — TEE registration availability check returns 404 from normal proxy:
```
FATAL fccutils/encoding.go:27  Error: action result status not ok, got: 404
[post-build] ERROR: Register TEE failed
```

## Observations
- **Normal proxy outage confirmed:** This is the second consecutive failure with the same 404 error from `tee-proxy-coston2-1.flare.rocks`. The normal proxy appears to be experiencing a sustained outage or degraded state, not a transient blip.
- Pre-registration and TEE attestation steps succeeded — only the availability check result polling fails.
- All other phases (pre-build, start-services, verify checks) continue to work fine.
- The extension proxy (ngrok) is healthy — only the external normal proxy is affected.
- This blocks all scenarios that require TEE registration (i.e., all of them except verify-only with pre-existing registration).
