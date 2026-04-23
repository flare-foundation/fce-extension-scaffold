# Verify Deploy

Runs deployment verification checks and helps diagnose deployment failures.

## When to Use

The user wants to check their deployment setup, diagnose a failed deployment, or verify everything is correctly configured. They may say things like:
- "verify my deployment"
- "check my setup"
- "why is deploy failing"
- "is my config correct"
- "pre-flight check"
- "/verify-deploy"

## Steps to Execute

### Step 1: Determine deployment state

Read these files to understand where the user is in the deployment lifecycle:

1. `.env` — check CHAIN_URL, DEPLOYMENT_PRIVATE_KEY (set or not), LOCAL_MODE, ADDRESSES_FILE
2. `config/extension.env` — if it exists, deployment has been run before
3. `config/deploy.log` — if it exists, contains stderr from the last deployment attempt

### Step 2: Run the verification tool

```bash
cd tools && go run ./cmd/verify-deploy \
  -a <ADDRESSES_FILE> \
  -c <CHAIN_URL> \
  --config ../config/extension.env
```

Use the values from `.env` for ADDRESSES_FILE and CHAIN_URL. If `.env` doesn't set them, use the defaults:
- ADDRESSES_FILE: auto-detected (same logic as pre-build.sh)
- CHAIN_URL: http://127.0.0.1:8545

**Scope flags — use these to avoid running everything when you only need one thing:**

- `--step <name>` — limit to a single step group. Values: `deploy`, `register`, `services`, `tee-version`, `tee-machine`, `test`, `all` (default).
- `--checks <IDs>` — further filter to specific check IDs within the selected step (comma-separated, e.g. `D5,R2`). All other checks are hidden and don't count toward failures.

Examples:
```bash
# Only the deploy step
--step deploy

# Only the deployer-key/balance check within deploy
--step deploy --checks D5

# Only the instruction-sender mismatch check (no step needed — runs all steps, shows only R2)
--checks R2

# Check whether the proxy is reachable for TEE version registration
--step tee-version --checks V6
```

When diagnosing a specific error code (e.g. the user says "R3 is failing"), use `--checks R3` so the output is focused.
When doing a pre-flight before a long operation, use `--step deploy` to avoid waiting for service/network checks that aren't relevant yet.

### Step 3: Interpret results

For each FAIL or WARN, explain what it means and how to fix it:

**D1/D2 — Registry address has no code:**
The addresses in deployed-addresses.json don't point to contracts on this chain.
Likely causes:
- Wrong CHAIN_URL (pointing to a different network than the addresses file)
- Wrong ADDRESSES_FILE (using coston2 addresses on local devnet or vice versa)
Fix: Check that CHAIN_URL and ADDRESSES_FILE in .env match the same network.

**D3 — Zero address in config:**
A required contract address is 0x000...000 in the addresses file.
Fix: Check deployed-addresses.json — a required entry is missing or unpopulated.

**D4 — Stale extension.env:**
The INSTRUCTION_SENDER from a previous deploy no longer has code on-chain.
Fix: Delete config/extension.env and re-run `scripts/pre-build.sh`.

**D5 — Deployer key/balance issues:**
Either DEPLOYMENT_PRIVATE_KEY is not set (using Hardhat dev key) or the account has no funds.
Fix: Set DEPLOYMENT_PRIVATE_KEY in .env to a funded account on the target network.

**D6 — Unexpected chain ID:**
Connected to a chain that may not match your intent.
Fix: Verify CHAIN_URL points to the intended network (Coston2: chain ID 114, local: 31337).

**D7 — Malformed extension.env values:**
The INSTRUCTION_SENDER or EXTENSION_ID in extension.env is not a valid hex value. This usually means a previous deploy command printed unexpected output that got captured by the script.
Fix: Delete config/extension.env and re-run `scripts/pre-build.sh`.

**R1 — Extensions counter is zero:**
The registry has no extensions registered. If this is a fresh deployment, that's expected — the first call to pre-build.sh will register one. If you expected extensions to exist, check CHAIN_URL and ADDRESSES_FILE.

**R2 — Instruction sender mismatch:**
The instruction sender address in extension.env doesn't match what's registered on-chain for this extension ID. The config is stale or a different contract was deployed.
Fix: Delete config/extension.env and re-run `scripts/pre-build.sh`.

**R3 — Deployer not allowed as TEE machine owner:**
The registration partially failed — the extension was created but the deployer wasn't added as a TEE machine owner. This means TEE machine registration will fail later.
Fix: Re-run `scripts/pre-build.sh` — it's now idempotent and will skip completed steps.

**R4 — EVM key type not supported:**
The registration partially failed — the extension was created but the EVM key type wasn't enabled.
Fix: Re-run `scripts/pre-build.sh`.

**R5 — Partially configured extension:**
Composite detection of R3/R4. The extension exists but is missing owner permissions or key type support.
Fix: Re-run `scripts/pre-build.sh`.

**R7 — Duplicate instruction sender:**
The same instruction sender contract is registered for multiple extensions. `setExtensionId()` in the Solidity contract will always resolve to the first (lowest ID) extension.
Fix: If the latest registration was intended, deploy a new instruction sender contract and re-run pre-build.sh.

**S2 — Stale or missing EXTENSION_ID:**
EXTENSION_ID in extension.env is empty or malformed. This means pre-build.sh either wasn't run or failed during registration.
Fix: Delete config/extension.env and re-run `scripts/pre-build.sh`.

**S5 — Extension port already in use:**
The port the extension server binds to (default 7702 in Docker, 8080 standalone) is already occupied. The server will fail to start silently (now caught by error channel).
Fix: Check what's using the port with `lsof -i :<port>` and stop the conflicting process.

**S7/S8 — Port mismatch:**
EXTENSION_PORT or SIGN_PORT env vars don't match expected values for the current mode (Docker vs standalone). Docker defaults: 7702/7701. Standalone defaults: 8080/9090.
Fix: Set the env vars explicitly in `.env` or Docker Compose, or ensure the correct entry point is being used.

**S9 — Proxy not reachable:**
EXT_PROXY_URL is set but the proxy's `/info` endpoint doesn't respond. The proxy may not be running, or the URL is wrong (e.g., ngrok URL that's not active).
Fix: Start the proxy first (`docker compose up ext-proxy`), or fix EXT_PROXY_URL in `.env`.

**S10 — PROXY_PRIVATE_KEY not set in non-local mode:**
LOCAL_MODE is false but PROXY_PRIVATE_KEY is empty. The proxy will use its default signing key which won't match the expected identity on Coston2.
Fix: Set PROXY_PRIVATE_KEY in `.env` to the proxy's signing key.

**V2 — Extension owner key not set:**
Neither EXTENSION_OWNER_KEY nor DEPLOYMENT_PRIVATE_KEY is set in non-local mode. AddTeeVersion will use the dev key which isn't the extension owner on Coston2 — the transaction will revert.
Fix: Set EXTENSION_OWNER_KEY or DEPLOYMENT_PRIVATE_KEY in `.env` to the extension owner's key.

**V4 — Extension ID invalid for TEE version:**
EXTENSION_ID in extension.env is missing or malformed. allow-tee-version needs a valid extension ID to register the version.
Fix: Re-run `scripts/pre-build.sh` to generate a valid extension.env.

**V6 — Proxy not reachable for TEE version:**
The proxy's `/info` endpoint is not responding. allow-tee-version queries `/info` to get the code hash and platform before registering the version.
Fix: Start services first with `scripts/start-services.sh`, then run `scripts/post-build.sh`.

**T1 — SIMULATED_TEE on non-local deployment:**
SIMULATED_TEE is true but LOCAL_MODE is false. The machine will be registered with test attestation values (hardcoded code hash + TEST_PLATFORM) which won't work on a real network.
Fix: Set `SIMULATED_TEE=false` in `.env` when deploying to a real GCP TEE.

**T2/T3 — Partial registration failure:**
Registration failed mid-way. The state file at `config/register-tee.state` tracks progress. Re-run `scripts/post-build.sh` and it will resume from the last successful step automatically.

**T4 — Machine already registered:**
The TEE machine is already registered on-chain. Re-running registration will skip pre-registration automatically.

**T5 — Normal proxy not reachable:**
NORMAL_PROXY_URL doesn't respond. The FTDC availability check requires the normal proxy.
Fix: Ensure the normal/FTDC proxy is running and NORMAL_PROXY_URL is correct in `.env`.

**T10 — Host URL not reachable:**
The URL that gets registered on-chain for this TEE machine is not responding. Data providers won't be able to relay instructions to this machine.
Fix: Ensure the host is accessible (check ngrok if using it) and EXT_PROXY_URL or -h flag points to the right URL.

**E1 — INSTRUCTION_SENDER not set or invalid:**
The instruction sender contract address is missing from extension.env. Tests can't send instructions without it.
Fix: Run `scripts/pre-build.sh` to deploy and register the extension.

**E3 — Proxy not reachable for test polling:**
The proxy endpoint is not responding. Tests poll the proxy for action results after sending instructions.
Fix: Ensure services are running with `scripts/start-services.sh`.

**E6 — OPType/OPCommand hash mismatch:**
If tests get a 501 "unsupported op type/command" error, the hash values in the Solidity contract don't match the extension's Go constants. The 501 response now includes the received and expected hash values for comparison.
Fix: Ensure `bytes32("GREETING")` in Solidity matches `teeutils.ToHash("GREETING")` in Go — case-sensitive.

**E7 — Instruction fee mismatch:**
If instruction send transactions revert, the hardcoded fee (1000000 wei) may not match the registry's required fee. Check the revert reason for payment-related errors.

**E9 — Encoding mismatch:**
SAY_HELLO uses JSON encoding, SAY_GOODBYE uses ABI encoding. When adding new commands, use the correct encoding for the contract's OriginalMessage parameter. Check the contract source for `// OriginalMessage encoding:` comments.

### Step 4: Check deployment logs for historical failures

If the user reports a failed deployment, or if verify-deploy shows FAILs:

1. Read `config/deploy.log` — this captures stderr from the Go deploy commands during `pre-build.sh`
2. Look for these patterns:
   - **Revert reasons** — on-chain rejection, likely wrong addresses or permissions (D1/D2)
   - **"insufficient funds"** — wrong key or unfunded account (D5)
   - **Connection errors** — wrong CHAIN_URL or RPC endpoint down (D6)
   - **Panic/stack traces** — bug in tooling (report to developer)
   - **"WARNING: DEPLOYMENT_PRIVATE_KEY not set"** — using dev key on non-local network (D5)
3. Cross-reference the log content with check IDs from the verification output
4. If `config/deploy.log` doesn't exist, the user hasn't run `pre-build.sh` yet — suggest running the verification tool with `--step deploy` as a preflight check

### Step 5: Detect compound misconfigurations

Check for these cross-cutting problems:
- **Coston2 addresses + localhost CHAIN_URL** = network mismatch (everything will fail silently)
- **LOCAL_MODE=false + no DEPLOYMENT_PRIVATE_KEY** = will use Hardhat key which has no Coston2 funds
- **extension.env exists + INSTRUCTION_SENDER has no code** = stale deployment, needs re-run
- **extension.env has EXTENSION_ID but R3/R4 fail** = partial registration, re-run pre-build.sh (now idempotent)

## Edge Cases Reference

Full edge case documentation: `EXTENSION-DEPLOYMENT-EDGE-CASES.md`
Design specs:
- `docs/superpowers/specs/2026-04-09-deployment-hardening-design.md` (Step 1)
- `docs/superpowers/specs/2026-04-09-registration-hardening-design.md` (Step 2)

## Important Notes

- The `.env` file contains secrets — do NOT print DEPLOYMENT_PRIVATE_KEY values. Only report whether it is set or not.
- Always read files before suggesting edits to them.
- The verification tool exits with code 1 if any check FAILs — this is expected, not a bug.
