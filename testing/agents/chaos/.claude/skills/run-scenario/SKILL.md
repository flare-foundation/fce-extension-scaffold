---
description: Execute a specific chaos scenario. May involve code modifications in the worktree.
---

# Run Chaos Scenario

The scenario name or description is passed as $ARGUMENTS. Read `../../shared/scenarios/chaos-scenarios.md` for detailed instructions if it matches a named scenario.

## Worktree Paths

```bash
WORKTREE="$(pwd)/worktree"
ORDERBOOK="$WORKTREE/extension-examples/orderbook"
```

## For Runtime-Only Scenarios

Run scripts from the worktree (even without modifications, this keeps paths consistent):
```bash
cd "$ORDERBOOK" && timeout 600 ./scripts/full-setup.sh --test 2>&1
```

## For Code Modification Scenarios

1. Apply the modification in the worktree:
   ```bash
   # Example: change an OPType constant
   sed -i 's/GREETING/greeting/' "$ORDERBOOK/internal/config/config.go"
   ```
2. If Go code was changed, verify it compiles:
   ```bash
   cd "$ORDERBOOK/tools" && go build ./... 2>&1
   ```
3. If Solidity was changed, regenerate bindings:
   ```bash
   cd "$ORDERBOOK" && ./scripts/generate-bindings.sh 2>&1
   ```
4. Run the deployment/test scripts from the worktree
5. Capture `cd "$WORKTREE" && git diff` before any reset

## Result Capture

Always capture:
- The exact modifications made (git diff)
- Full stdout/stderr output (truncate to last 300 lines)
- Exit codes
- Timing per phase
- Whether the error messages clearly explain what went wrong

## Invention Mode

If you're inventing a new scenario:
1. Describe what you're trying to break and why
2. List the exact steps you'll take
3. Execute and capture results
4. Log ideas for follow-up scenarios in the "Ideas for Next Time" section
