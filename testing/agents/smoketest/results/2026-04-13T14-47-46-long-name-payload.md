# [Smoketest] Long Name Payload
**Date:** 2026-04-13 14:47:46
**Scenario:** long-name-payload
**Duration:** 0s
**Result:** PARTIAL

## Phases
| Phase | Status | Duration | Notes |
|-------|--------|----------|-------|
| all | SKIPPED | 0s | Same tooling gap as unicode-payload (#5) |

## Errors
None.

## Observations
- `run-test` hardcodes `"name": "World"` — no `--name` flag to inject a 1000-character string.
- Skipped to avoid burning gas on a deployment that cannot exercise the scenario.
- See finding logged in scenario #5 for details.
