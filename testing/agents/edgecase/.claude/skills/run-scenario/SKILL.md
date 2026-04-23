---
description: Execute a specific edge case scenario. Called by heartbeat with a scenario ID like "D1-wrong-registry".
---

# Run Edge Case Scenario

The scenario ID is passed as $ARGUMENTS. Read the detailed instructions from `../../shared/scenarios/edgecase-scenarios.md` for the matching scenario.

## General Pattern

Most edge case scenarios follow this pattern:

1. **Read the edge case docs** — check `$REPO_ROOT/notes/EXTENSION-DEPLOYMENT-EDGE-CASES.md` for the scenario ID to understand expected behavior
2. **Set up conditions** — create the specific failure condition (wrong key, missing file, etc.)
3. **Run the relevant script/tool** — with timeout 600
4. **Capture output** — stdout, stderr, exit code
5. **Evaluate** — did the behavior match the documented expectation? How clear was the error?
6. **Clean up** — restore any changed env vars or configs

## Important

- Use `timeout 600` on all long-running commands
- Capture both stdout and stderr: `command 2>&1`
- For scenarios that require wrong env vars, use subshell or export/unset pattern:
  ```bash
  (export DEPLOYMENT_PRIVATE_KEY="wrong_key" && cd "$REPO_ROOT" && timeout 120 ./scripts/pre-build.sh 2>&1)
  ```
- Never permanently modify `.env` — use env var overrides
- Record the EXACT error messages — these are what we're auditing for clarity
