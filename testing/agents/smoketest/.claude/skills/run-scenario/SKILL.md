---
description: Execute a specific smoketest scenario. Called by the heartbeat skill with a scenario name.
---

# Run Scenario

Execute the specified test scenario. The scenario name is passed as $ARGUMENTS.

Compute the repo root:
```bash
REPO_ROOT="$(cd ../../.. && pwd)"
```

## Scenarios

### full-setup-standard
```bash
cd "$REPO_ROOT" && timeout 600 ./scripts/full-setup.sh --test
```
Record exit code and any stderr output. All 4 phases should pass.

### step-by-step
Run each phase separately with explicit error capture:
```bash
cd "$REPO_ROOT"
timeout 120 ./scripts/pre-build.sh 2>&1
timeout 120 ./scripts/start-services.sh 2>&1
timeout 300 ./scripts/post-build.sh 2>&1
timeout 120 ./scripts/test.sh 2>&1
```
Record per-phase timing and exit codes.

### rapid-cycle
```bash
cd "$REPO_ROOT"
timeout 600 ./scripts/full-setup.sh --test
./scripts/stop-services.sh
timeout 600 ./scripts/full-setup.sh --test
```
Verify both runs pass.

### verify-between-phases
```bash
cd "$REPO_ROOT"
timeout 120 ./scripts/pre-build.sh
cd tools && timeout 60 go run ./cmd/verify-deploy -a "$ADDRESSES_FILE" -c "$CHAIN_URL" --config ../config/extension.env --step deploy --step register && cd ..
timeout 120 ./scripts/start-services.sh
cd tools && timeout 60 go run ./cmd/verify-deploy -a "$ADDRESSES_FILE" -c "$CHAIN_URL" --config ../config/extension.env --step services && cd ..
timeout 300 ./scripts/post-build.sh
timeout 120 ./scripts/test.sh
```

### unicode-payload
Run full-setup.sh --test. In the result, note whether unicode names are handled. This is an observational test — check if any warnings appear about encoding.

### long-name-payload
Run full-setup.sh --test. After test.sh passes, manually send an instruction with a 1000-character name by running run-test with modified input. Record behavior.

### empty-payload
Run full-setup.sh --test. After test.sh passes, manually send an instruction with an empty name. Record whether it succeeds or fails gracefully.

### double-test
```bash
cd "$REPO_ROOT"
timeout 600 ./scripts/full-setup.sh --test
timeout 120 ./scripts/test.sh
```
Run test.sh twice (full-setup already runs it once). Verify second run also passes.

### verify-only
```bash
cd "$REPO_ROOT"
timeout 600 ./scripts/full-setup.sh --test
cd tools && timeout 60 go run ./cmd/verify-deploy -a "$ADDRESSES_FILE" -c "$CHAIN_URL" --config ../config/extension.env && cd ..
```
Run all verify-deploy checks after a successful setup. All should PASS.

## Result Capture

For every scenario, capture:
1. Start and end timestamps (compute duration)
2. Exit code of each phase
3. Stdout and stderr (truncate to last 200 lines if too long)
4. Any unexpected warnings or output

Write the result using the format defined in CLAUDE.md.
