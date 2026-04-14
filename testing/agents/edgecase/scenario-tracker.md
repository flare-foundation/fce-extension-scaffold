# Edge Case Scenario Tracker

Track which scenarios have been tested and their results.

## How to Use
- Pick the next `[ ]` (untested) scenario
- After testing, update the checkbox to `[x]` and fill in the result columns
- After all are done, reset checkboxes and loop for flakiness detection

## Contract Deployment (D-series)

| Done | Scenario | Last Run | Result | Error Clarity | Notes |
|------|----------|----------|--------|---------------|-------|
| [x] | D1-wrong-registry | 2026-04-13 | EXPECTED_FAIL | GOOD | Pre-flight catches wrong registry with clear error. .env overrides env vars (finding). |
| [x] | D3-zero-addresses | 2026-04-13 | EXPECTED_FAIL | GOOD | Pre-flight catches zero address with specific error message. Fails fast. |
| [x] | D4-double-deploy | 2026-04-13 | EXPECTED_FAIL | BAD | No warning on re-run. Silently overwrites extension.env, orphans on-chain state. |
| [x] | D5-wrong-key | 2026-04-13 | EXPECTED_FAIL | GOOD | Pre-flight catches unfunded deployer with balance + minimum required in error. |
| [x] | D7-output-capture | 2026-04-13 | EXPECTED_FAIL | MODERATE | Address capture clean via tail -1. But INFO logs go to stdout (fragile). |

## Registration (R-series)

| Done | Scenario | Last Run | Result | Error Clarity | Notes |
|------|----------|----------|--------|---------------|-------|
| [ ] | R3-partial-failure | — | — | — | — |
| [ ] | R5-rerun-after-success | — | — | — | — |
| [ ] | R7-duplicate-sender | — | — | — | — |

## Service Startup (S-series)

| Done | Scenario | Last Run | Result | Error Clarity | Notes |
|------|----------|----------|--------|---------------|-------|
| [ ] | S1-missing-extension-id | — | — | — | — |
| [ ] | S2-stale-extension-id | — | — | — | — |
| [ ] | S3-no-docker-network | — | — | — | — |
| [ ] | S9-wrong-proxy-url | — | — | — | — |

## TEE Version (V-series)

| Done | Scenario | Last Run | Result | Error Clarity | Notes |
|------|----------|----------|--------|---------------|-------|
| [ ] | V1-already-registered | — | — | — | — |
| [ ] | V6-proxy-not-running | — | — | — | — |

## TEE Machine (T-series)

| Done | Scenario | Last Run | Result | Error Clarity | Notes |
|------|----------|----------|--------|---------------|-------|
| [ ] | T4-already-registered | — | — | — | — |

## Testing (E-series)

| Done | Scenario | Last Run | Result | Error Clarity | Notes |
|------|----------|----------|--------|---------------|-------|
| [ ] | E1-no-extension-registered | — | — | — | — |
| [ ] | E4-result-polling | — | — | — | — |

## Verify-Deploy Validation

| Done | Scenario | Last Run | Result | Error Clarity | Notes |
|------|----------|----------|--------|---------------|-------|
| [ ] | verify-before-prebuild | — | — | — | — |
| [ ] | verify-after-prebuild | — | — | — | — |
| [ ] | verify-after-start | — | — | — | — |
| [ ] | verify-full-after-setup | — | — | — | — |
