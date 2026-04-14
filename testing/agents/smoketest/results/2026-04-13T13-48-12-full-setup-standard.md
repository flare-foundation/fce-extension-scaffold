# [Smoketest] Full Setup Standard
**Date:** 2026-04-13 13:48:12
**Scenario:** full-setup-standard
**Duration:** 2m 5s
**Result:** PASS

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~25s | Contract deployed at 0xc61DC3c7A197852E55037aA0a3002aCC7e883505, Extension ID 0x183 |
| start-services | PASS | ~60s | Docker images built, proxy ready at ngrok endpoint in ~2s |
| post-build | PASS | ~22s | TEE version allowed, TEE machine registered (0x520F...E442) |
| test | PASS | ~15s | SAY_HELLO and SAY_GOODBYE both processed successfully |

## Errors
None.

## Observations
- "Warning: Error loading .env file" appears in several Go tool invocations — harmless, the tools fall back to config/extension.env. Recurring across all phases.
- Docker build took ~50s (Go module download + compile). Subsequent runs with cache should be faster.
- Proxy health check responded in 2s — fast.
- TEE registration completed full cycle (attestation request -> availability check -> proof) in ~16s.
- All tests passed on first attempt. No flakiness observed.
