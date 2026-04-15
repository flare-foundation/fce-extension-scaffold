---
description: Run a test cycle. Acquires lock, picks a scenario, runs it, logs results. Dispatched by the sequencer.
disable-model-invocation: true
---

# Heartbeat — Smoketest Cycle

You are performing a test cycle dispatched by the sequencer.

## Step 1: Acquire the lock

```bash
echo "smoketest|$(date +%s)" > /tmp/flare-extension-testing.lock
```

## Step 2: Pick the next scenario

Read `../../shared/scenarios/smoketest-scenarios.md`. Pick the next scenario in sequence. Track which scenario was last run by reading the most recent result file in `results/`.

## Step 3: Run the scenario

Use `/run-scenario` with the scenario name. Always wrap deployment scripts in `timeout 600`.

## Step 4: Log results

Write the result to `results/YYYY-MM-DDTHH-MM-SS-scenario-name.md` using the format from CLAUDE.md.

Update `../../summary/latest-status.md` — edit the smoketest row with the latest result.

If anything unexpected happened (unclear error, surprising behavior, timing anomaly), append a finding to `../../summary/findings.md`.

## Step 5: Tear down and release

```bash
REPO_ROOT="$(cd ../../.. && pwd)"
cd "$REPO_ROOT" && docker compose down 2>/dev/null || true
# Only release the lock if we still own it
LOCK_OWNER=$(cut -d'|' -f1 /tmp/flare-extension-testing.lock 2>/dev/null || echo "")
[ "$LOCK_OWNER" = "smoketest" ] && rm -f /tmp/flare-extension-testing.lock
```

Always do this, even if the scenario failed.
