# [Chaos] Invalid Private Key Formats
**Date:** 2026-04-13 14:11:00
**Scenario:** invented: invalid-private-key-formats
**Duration:** 1m 30s
**Result:** PASS
**Code Modified:** NO

## Modifications Made
none (tested with environment variable variations, not code changes)

## What Was Tried
Set `DEPLOYMENT_PRIVATE_KEY` to 7 different invalid values and ran `deploy-contract --preflight-only` against a valid addresses file (coston2) to test the key parsing/validation path.

| # | Input | Description |
|---|-------|-------------|
| 1 | `""` (empty) | Falls back to hardcoded Hardhat key |
| 2 | `"ab"` | Valid hex, too short |
| 3 | `"ZZZ..."` (64 Z chars) | Invalid hex characters |
| 4 | `"0xNOTHEX"` | 0x prefix with non-hex |
| 5 | `"000...0"` (64 zeros) | Valid hex, cryptographically invalid |
| 6 | `"aaa..."` (128 a chars) | Valid hex, too long |
| 7 | `"ac09 36a8..."` (spaces) | Spaces between hex groups |

## What Happened

| # | Error Message | Clear? |
|---|--------------|--------|
| 1 | `WARNING: DEPLOYMENT_PRIVATE_KEY not set — using hardcoded Hardhat dev key` | YES — clear fallback behavior with warning |
| 2 | `invalid length, need 256 bits` | YES — explains exactly what's needed |
| 3 | `invalid hex character 'Z' in private key` | YES — names the bad character |
| 4 | `invalid hex character 'N' in private key` | YES — 0x prefix stripped correctly, then catches bad char |
| 5 | `invalid private key, zero or negative` | YES — go-ethereum rejects the zero key |
| 6 | `invalid length, need 256 bits` | YES — same clear length message |
| 7 | `invalid hex character ' ' in private key` | YES — catches the space |

All errors:
- Include the specific character or condition that failed
- Produce stack traces showing the call path (DefaultPrivateKey → DefaultSupport)
- Result in exit code 1
- Are caught by `pre-build.sh`'s preflight and wrapped with "Pre-flight check failed"

## Analysis

**Private key validation is solid.** The `crypto.HexToECDSA` function from go-ethereum handles all tested edge cases with clear, specific error messages. The 0x prefix stripping works correctly. The empty-key fallback to the hardcoded Hardhat key is clearly warned about.

No bugs found. The error handling for private key input is thorough and well-messaged.

**Minor observation:** The `Warning: Error loading .env file: open .env: no such file or directory` message appears on every run when there's no .env file. This is cosmetic noise — `godotenv.Load()` is called unconditionally and warns on missing .env even though .env is optional. Not a bug, but slightly noisy.

## Ideas for Next Time
- Now that no-chain scenarios are well-covered, focus on scenarios needing live infrastructure
- Test scenario #11 (hash-mismatch) — modify OPType constants, rebuild, test with chain
- Test scenario #13 (wrong-ports) — change EXTENSION_PORT in docker-compose
- Invent: test what happens when .env file contains shell metacharacters or newlines in values
