# [Smoketest] Full Setup Standard
**Date:** 2026-04-13 18:46:35
**Scenario:** full-setup-standard
**Duration:** 2m 16s
**Result:** FAIL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| pre-build | PASS | ~18s | Contract deployed |
| start-services | PASS | ~76s | Docker build + containers started |
| post-build | FAIL | ~42s | TEE registration 404 from normal proxy |
| test | SKIPPED | - | Not reached |

## Errors
Same persistent 404 from normal proxy during TEE registration availability check.

## Observations
- **Eighth consecutive failure.** Normal proxy outage now ~72 minutes (17:36–18:48).
- Outage has persisted through nearly two full rotations of all 9 scenarios.
