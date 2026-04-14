# [Smoketest] Long Name Payload
**Date:** 2026-04-13 18:06:29
**Scenario:** long-name-payload
**Duration:** 2m 13s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed, Extension ID 0x19f (415) |
| start-services | PASS | ~74s | Docker build + containers started |
| post-build | FAIL | ~41s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same persistent 404 from normal proxy:
```
WARN  action result status not ok: got: 404 for 0xf8ef305e..., https://tee-proxy-coston2-1.flare.rocks
FATAL Error: action result status not ok, got: 404
```

## Observations
- **Fourth consecutive failure.** Normal proxy outage now ~32 minutes (first observed 17:36, still failing at 18:08).
- Pattern is identical across all 4 failures: pre-registration succeeds, attestation succeeds, availability check is sent successfully, but result polling returns 404 after ~34s.
- No long-name testing possible due to failure before test phase.
