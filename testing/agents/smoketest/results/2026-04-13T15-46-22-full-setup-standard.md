# [Smoketest] Full Setup Standard
**Date:** 2026-04-13 15:46:22
**Scenario:** full-setup-standard
**Duration:** 2m 9s
**Result:** PASS

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~28s | Contract deployed at 0x4266feC98c57cFE95A48B17CfCB0c03716D0B23b, Extension ID 0x190 |
| start-services | PASS | ~58s | Docker build + containers started, proxy ready in 2s |
| post-build | PASS | ~22s | TEE version allowed, TEE machine 0x61968b8C61c244b199456015ae24D27d98381B66 registered |
| test | PASS | ~19s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Clean run, no surprises. Total wall time 129s.
- The recurring `.env` file warning ("Error loading .env file: open .env: no such file or directory") continues in all Go tool invocations — cosmetic only.
- Docker build cached most layers; only the COPY and go mod download/build steps ran (~53s of the build time).
- Extension ID incremented to 400 (0x190), consistent with sequential on-chain registration.
