---
description: Run a chaos test cycle. Resets worktree, acquires lock, picks/invents a scenario, runs it, logs results. Dispatched by the sequencer.
disable-model-invocation: true
---

# Heartbeat — Chaos Cycle

You are performing a chaos test cycle dispatched by the sequencer.

## Step 1: Reset worktree

```bash
cd worktree && git checkout . && cd ..
```

This ensures a clean slate for each cycle.

## Step 2: Acquire the lock

```bash
echo "chaos|$(date +%s)" > /tmp/flare-extension-testing.lock
```

## Step 3: Pick a scenario

Read `../../shared/scenarios/chaos-scenarios.md`. Pick the next scenario you haven't tried recently (check your previous results in `results/`).

You can also **invent a new scenario** — look at:
- Previous results for patterns of failure
- The edge case docs at `worktree/extension-examples/orderbook/notes/`
- Combinations of failures not yet tried

## Step 4: Run the scenario

Use `/run-scenario` with the scenario name or description.

For code modification scenarios:
1. Make changes in `worktree/extension-examples/orderbook/`
2. Run scripts from that directory
3. Before teardown, capture `cd worktree && git diff` for the result log

## Step 5: Log results

Write to `results/YYYY-MM-DDTHH-MM-SS-scenario-name.md` using the Chaos result format from CLAUDE.md.

Update `../../summary/latest-status.md`. If anything interesting happened, append to `../../summary/findings.md`.

## Step 6: Tear down and release

```bash
cd worktree/extension-examples/orderbook && docker compose down 2>/dev/null || true
rm -f /tmp/flare-extension-testing.lock
```
