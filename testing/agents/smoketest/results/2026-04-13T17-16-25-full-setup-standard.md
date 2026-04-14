# [Smoketest] Full Setup Standard
**Date:** 2026-04-13 17:16:25
**Scenario:** full-setup-standard
**Duration:** 2m 9s
**Result:** PASS

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract 0xBb12dE71509d12b5a0182A0769ad8B32e201505C, Extension ID 0x19a |
| start-services | PASS | ~76s | Docker build + containers started, proxy ready in 2s |
| post-build | PASS | ~17s | TEE 0xA0d31Ca95f4d974CC144Ae5E4939e6716aA67C47 registered |
| test | PASS | ~17s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Clean run, no surprises. Total wall time 129s — consistent with previous full-setup-standard runs.
- Extension ID reached 410 (0x19a). Fourth rotation through the scenario list, pipeline continues stable.
- Docker build consistently takes ~57s (go mod download + go build), which dominates the start-services phase.
