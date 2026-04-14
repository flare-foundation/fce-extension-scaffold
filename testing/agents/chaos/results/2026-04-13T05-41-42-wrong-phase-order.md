# [Chaos] Wrong Phase Order
**Date:** 2026-04-13 05:41:42
**Scenario:** wrong-phase-order (#3)
**Duration:** 2m 30s
**Result:** PASS
**Code Modified:** NO

## Modifications Made
none (runtime-only scenario)

## What Was Tried
Ran deployment scripts out of the expected order to test whether each script produces a clear, actionable error message when prerequisites are missing.

1. **test.sh with no prior setup** — Ran `test.sh` with no `config/extension.env` and no services running.
2. **post-build.sh with no prior setup** — Ran `post-build.sh` with no `config/extension.env` and no `deployed-addresses.json`.
3. **start-services.sh with no prior setup** — Ran `start-services.sh` with no `config/extension.env`.
4. **start-services.sh with fake EXTENSION_ID but no devnet** — Created a fake `config/extension.env` with an EXTENSION_ID, then ran `start-services.sh`. Docker Compose attempted to start but failed on build context path resolution.

## What Happened

| Test | Exit Code | Error Message |
|------|-----------|---------------|
| test.sh (no setup) | 1 | `INSTRUCTION_SENDER not set. Run pre-build.sh first or set it manually.` |
| post-build.sh (no setup) | 1 | `Cannot find deployed-addresses.json. Set ADDRESSES_FILE.` |
| start-services.sh (no setup) | 1 | `EXTENSION_ID not set. Run pre-build.sh first or set it manually.` |
| start-services.sh (fake config) | 1 | `docker compose up failed` (after Docker error: `lstat .../extension-examples: no such file or directory`) |

All scripts exit with code 1 and use the `die()` pattern for clear error output.

## Analysis

**Error messages are clear and actionable** for the first three cases:
- `test.sh` correctly identifies the missing `INSTRUCTION_SENDER` and tells the user to run `pre-build.sh`.
- `post-build.sh` correctly identifies the missing `deployed-addresses.json` and tells the user to set `ADDRESSES_FILE`.
- `start-services.sh` correctly identifies the missing `EXTENSION_ID` and tells the user to run `pre-build.sh`.

**Minor issue:** When `start-services.sh` proceeds past the EXTENSION_ID check (because a config file exists) but Docker Compose fails due to build context issues, the error message is less helpful — it just says "docker compose up failed" without showing the underlying Docker error to the user. The Docker error itself (`lstat .../extension-examples: no such file or directory`) is visible in the output but is a Docker-level message, not a script-level diagnostic.

**No devnet was available** to test the full wrong-ordering scenario (e.g., running `post-build.sh` after `pre-build.sh` but before `start-services.sh`), which would test timeout behavior when services aren't reachable.

Overall, the guard rails work well. Each script validates its prerequisites before doing destructive work.

## Ideas for Next Time
- Test with devnet running: run `post-build.sh` before `start-services.sh` to see if the timeout/wait message is clear when services aren't up.
- Test `pre-build.sh` with a dead chain URL to see the preflight error messages.
- Try running `stop-services.sh` when nothing is running — does it handle gracefully?
- Combine wrong-phase-order with code modifications (e.g., remove the `die` checks to see what happens downstream).
