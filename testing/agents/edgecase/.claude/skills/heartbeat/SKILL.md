---
description: Run an edge case test cycle. Acquires lock, picks next scenario, runs it, logs results. Dispatched by the sequencer.
disable-model-invocation: true
---

# Heartbeat — Edge Case Cycle

You are performing an edge case test cycle dispatched by the sequencer.

## Step 1: Acquire the lock

```bash
echo "edgecase|$(date +%s)" > /tmp/flare-extension-testing.lock
```

## Step 2: Pick the next scenario

Read `scenario-tracker.md`. Find the first row with `[ ]` (untested). If all are tested, reset all checkboxes back to `[ ]` and start from the first one (loop for flakiness).

## Step 3: Run the scenario

Use `/run-scenario` with the scenario ID (e.g., "D1-wrong-registry"). Read the scenario instructions from `../../shared/scenarios/edgecase-scenarios.md`.

## Step 4: Update tracker and log results

1. Update the matching row in `scenario-tracker.md` with date, result, error clarity, and notes
2. Write detailed result to `results/YYYY-MM-DDTHH-MM-SS-scenario-id.md`
3. Update `../../summary/latest-status.md`
4. If the result was unexpected (behavior doesn't match docs), append to `../../summary/findings.md`

## Step 5: Tear down and release

```bash
REPO_ROOT="$(cd ../../.. && pwd)"
cd "$REPO_ROOT" && docker compose down 2>/dev/null || true
rm -f /tmp/flare-extension-testing.lock
```
