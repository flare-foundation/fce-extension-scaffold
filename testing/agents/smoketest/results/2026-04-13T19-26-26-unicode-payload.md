# [Smoketest] Unicode Payload
**Date:** 2026-04-13 19:26:26
**Scenario:** unicode-payload
**Duration:** 2m 20s
**Result:** PASS (observational only)

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~76s | Docker build + containers started |
| post-build | PASS | ~22s | TEE registered |
| test | PASS | ~24s | SAY_HELLO and SAY_GOODBYE both passed |

## Errors
None.

## Observations
- Unicode not actually tested — `run-test` hardcodes "World" with no custom payload flag (known tooling gap).
- Pipeline stable post-recovery. Normal proxy working consistently.
