# [Smoketest] Unicode Payload
**Date:** 2026-04-13 17:56:33
**Scenario:** unicode-payload
**Duration:** 2m 22s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed, Extension ID 0x19e (414) |
| start-services | PASS | ~77s | Docker build + containers started |
| post-build | FAIL | ~45s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same persistent 404 from normal proxy during TEE registration availability check:
```
WARN  action result status not ok: got: 404 for 0x68fd0460..., https://tee-proxy-coston2-1.flare.rocks
FATAL Error: action result status not ok, got: 404
```

## Observations
- **Third consecutive failure** due to normal proxy outage (`tee-proxy-coston2-1.flare.rocks`). Outage duration: ~22 minutes (first observed 17:36, still failing at 17:58).
- Pre-registration and attestation continue to succeed — only the availability check result polling returns 404.
- All other infrastructure (Coston2 RPC, extension proxy/ngrok, Docker) remains healthy.
- No unicode testing was possible due to the failure occurring before the test phase.
