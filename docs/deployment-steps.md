# 🚀 TEE Extension Deployment — Step by Step

Linear recipe to deploy a TEE extension to Flare Coston or Coston2. Run the steps top to bottom.

## Prerequisites

- 🐳 Docker Desktop (Linux containers)
- 🐹 Go 1.25.1+
- 🔨 Foundry (`forge`, `cast`)
- `jq`
- Bash (Git Bash on Windows works)
- VPN access to Flare's indexer DB (`35.241.249.150:3306`)

## 1. Get the extension repo

The default build is **self-contained**: `go.mod` pins `tee-node` and
`proxy/Dockerfile` pins `tee-proxy`, both fetched from the network at build
time. You only need this repo — no sibling `tee-node/` or `tee-proxy/`
checkouts.

```text
<workspace>/
└── <your-extension>/     # this repo — builds standalone
```

> **Developing `tee-node`/`tee-proxy` locally?** Place them as siblings and use
> the opt-in toggle, which builds the node + proxy from your on-disk checkouts
> instead of the pinned versions:
>
> ```text
> <workspace>/tee/
> ├── tee-node/         # github.com/flare-foundation/tee-node
> ├── tee-proxy/        # github.com/flare-foundation/tee-proxy
> └── extension-examples/
>     └── <your-extension>/
> ```
>
> ```bash
> USE_LOCAL_SIBLINGS=1 ./scripts/start-services.sh --chain coston2
> ```

## 2. Generate a funded deployer key

```bash
cast wallet new
cast wallet address --private-key 0x<private-key>
```

The derived address becomes your `INITIAL_OWNER`. Fund it from the target chain's faucet.

| Chain   | Faucet                                 |
| ------- | -------------------------------------- |
| Coston  | `https://faucet.flare.network/coston`  |
| Coston2 | `https://faucet.flare.network/coston2` |

## 3. Create `.env.<chain>`

`use-chain.sh` creates everything the chain needs — `.env.<chain>`, the proxy
toml and the compose override — cloning the active `.env` and rewriting only the
chain-specific values. It never overwrites an existing file.

```bash
bash ./scripts/use-chain.sh <chain>     # local | coston | coston2 | songbird | flare
```

It cannot invent `config/<chain>/deployed-addresses.json` — copy that in first.

It writes `CHAIN`, `CHAIN_ID`, `CHAIN_URL`, `ADDRESSES_FILE`, `NORMAL_PROXY_URL`,
`LOCAL_MODE=false` and `SIMULATED_TEE=false` itself. Two values are yours, in
`.env.<chain>`:

```bash
DEPLOYMENT_PRIVATE_KEY=<private key, no 0x prefix>
INITIAL_OWNER=0x<derived address from Step 2>
```

`EXT_PROXY_URL` is left empty deliberately — you get it in Step 6.

Re-run `use-chain.sh <chain>` after editing to copy `.env.<chain>` → `.env`,
which every script auto-loads. Edit `.env.<chain>`, never `.env`.

## 4. Register the extension on-chain

```bash
bash ./scripts/pre-build.sh
```

Compiles Solidity, deploys `InstructionSender`, registers the extension on-chain. Writes `EXTENSION_ID` and `INSTRUCTION_SENDER` to `config/<chain>/extension.env`.

Read the new values — `EXTENSION_ID` is part of the hand-off in Step 6:

```bash
source .env && cat "config/$CHAIN/extension.env"
```

## 5. Build the Docker image

The image built is selected by `LANGUAGE` in `.env` — `go/Dockerfile`, `python/Dockerfile` or `typescript/Dockerfile`. The steps below are identical for all of them.

### Attestation mode

`MODE` selects the attestation backend:

| Value | Meaning | Used for |
|---|---|---|
| `1` | Simulated attestation (test code hash) | local devnet — **the scaffold's default** |
| `0` | Production attestation | a real Confidential Space VM |

**Every language's Dockerfile deliberately ships `MODE=1`**, so a bare `docker run` and the compose stack both work against the local devnet without extra configuration. `docker-compose.yaml` reinforces this with `MODE=${MODE:-1}`.

**FTDC rejects simulated attestation**, so a production deploy must run with `MODE=0`. You have two options, and the second is preferred:

1. Edit `ENV MODE=1` → `ENV MODE=0` in your `<LANGUAGE>/Dockerfile` before building the release image.
2. **Leave the image as-is and override at workload launch.** The `tee.launch_policy.allow_env_override` label lists `MODE`, so the Confidential Space VM accepts an override — and without that label it would reject one. This keeps a single image usable for both local dev and production, and keeps the code hash independent of which environment it is destined for.

Whichever you choose, verify what actually ended up in the image before registering its hash on-chain — see the check at the end of this section.

Then build:

```powershell
$env:SOURCE_DATE_EPOCH = (git log -1 --format=%ct)
docker compose -f docker-compose.yaml build --no-cache extension-tee
docker tag <your-extension>-extension-tee:latest <your-extension>:v0.1.0
docker save <your-extension>:v0.1.0 -o <your-extension>-v0.1.0.tar
```

Compose resolves the Dockerfile from `EXTENSION_DOCKERFILE`, which `start-services.sh` derives from `LANGUAGE`. Building compose directly (as above) uses the `go/Dockerfile` default, so for another language export it first:

```bash
export EXTENSION_DOCKERFILE=python/Dockerfile TEE_NODE_REF=$(bash -c 'source scripts/lib/versions.sh; load_versions "$PWD"; echo $TEE_NODE_REF')
```

Non-Go images also need the shared tee-node base image, which `start-services.sh` builds automatically:

```bash
./scripts/build-node-base.sh
```

Setting `SOURCE_DATE_EPOCH` makes the build reproducible (same source → same `codeHash`). Note that only the Go image is reproducible **across machines**; Python and TypeScript are same-machine only — see [REPRODUCIBILITY.md](../REPRODUCIBILITY.md) before promising an auditor they can reproduce your hash.

Check which mode is baked into the image:

```powershell
docker inspect <your-extension>:v0.1.0 --format '{{range .Config.Env}}{{println .}}{{end}}' | Select-String MODE
```

If you took option 1 above, expect `MODE=0`. If you took option 2, expect the scaffold default `MODE=1` and supply `MODE=0` at workload launch instead — confirm the launch policy label permits it:

```powershell
docker inspect <your-extension>:v0.1.0 --format '{{index .Config.Labels "tee.launch_policy.allow_env_override"}}'
# MODE must appear in this list, or the VM rejects the override at attestation time
```

## 6. Deploy the image on a Confidential Space VM

Hand off (or deploy yourself) to a GCP Confidential Space VM with:

- The image (tar or registry URL+tag)
- Workload-launch env: `INITIAL_OWNER`, `CHAIN_URL`, `EXTENSION_ID` (from Step 4), `PROXY_URL` (proxy URL reachable from the TEE)
- Public HTTPS routed to port `6664` of the proxy container

You receive back the **public proxy URL**. Add it to `.env.<chain>` and re-activate:

```bash
# in .env.<chain>
EXT_PROXY_URL=<public proxy URL>
```

```bash
bash ./scripts/use-chain.sh <chain>
```

## 7. Verify the proxy `/info`

```powershell
curl -s $env:EXT_PROXY_URL/info | jq '.machineData'
```

Required values:

| Field          | Expected                                                          |
| -------------- | ----------------------------------------------------------------- |
| `platform`     | starts with `0x4743505f414d445f534556…` (GCP_AMD_SEV)             |
| `codeHash`     | real measured hash (**not** `0x194844cf…` — that's simulated)     |
| `extensionId`  | matches your `config/<chain>/extension.env` `EXTENSION_ID`           |
| `initialOwner` | matches your `INITIAL_OWNER`                                      |

If `extensionId` is wrong, ask the VM operator to restart the container with the correct `EXTENSION_ID` env override (no image rebuild needed — it's a launch-policy override).

## 8. Register the TEE machine

> [!NOTE]
> `scripts/post-build.sh` already passes `-command rRap` (the tool's own default is `rap`). Override with `REGISTER_TEE_COMMAND` if you need to run individual steps.
>
> Step `a` (availability check) needs a one-time **challenge** — a random number from the contract that the TEE signs to prove it's alive. Lowercase `r` only issues one while pre-registering, and it skips itself once the TEE is registered on-chain, so re-runs (image changes, diamond cuts, retries) revert with `Verification.ChallengeExpired`. Capital `R` issues the challenge directly — decoupled from `r` — so re-runs work.

Run:

```bash
bash ./scripts/post-build.sh
```

- `allow-tee-version` whitelists the codeHash for your extension.
- `register-tee -command rRap` pre-registers the TEE, requests fresh attestation, runs the FTDC availability check, promotes to production.

## 9. End-to-end test

```bash
bash ./scripts/test.sh
```

Sends test instructions through the deployed TEE and verifies the round-trip.

---

## Platform traps

Properties of FCC, not of this extension. Each is silent, presents as something
else, and has cost real redeploys.

### The TEE key is in memory only

Confidential Space has no persistent storage, so **every relaunch mints a new
`teeId`**. The previous machine stays *active* on-chain with a key nobody holds, and
`getRandomTeeIds` load-balances across active machines — so instructions are routed
to a dead node roughly half the time and silently never complete (`/action/result`
404s, callers report a poll timeout).

```bash
cd tools && source ../.env && source "../config/$CHAIN/extension.env"
DIAMOND=$(jq -r '.[]|select(.name=="FlareTeeManager").address' ../config/$CHAIN/deployed-addresses.json)
go run ./cmd/query-tee -ext "$((EXTENSION_ID))" -reg "$DIAMOND" -rpc "$CHAIN_URL"   # getActiveTeeMachines
cast send "$DIAMOND" 'pause(address)' <staleTeeId> --rpc-url "$CHAIN_URL" --private-key "$KEY"
```

`-ext` is **decimal**, not the bytes32 hex — `$((EXTENSION_ID))` converts it. `-reg`
defaults to a stale address; always pass the diamond.

The live `teeId` is `keccak256(pubkey.x ‖ pubkey.y)[12:]` from the proxy's `/info`.
There is no `unpause` — only `toProduction` with a fresh availability proof — so
never pause the live one.

### One-shot bindings must be written last

`setExtensionId()` requires the current value to be zero and has no reset. Bound to
a stale value, the contract must be redeployed. Reads keep working, so it hides
until someone sends an instruction. Corollary: never run `full-setup.sh` against a
remote TEE — it chains the post-setup script and binds early.

### The launch policy aborts the workload

Confidential Space rejects any env var outside the image's
`tee.launch_policy.allow_env_override` label and exits `exit_code=4` before the
workload starts. Diff the launcher's `Image Labels` against its `Envs:`. The label
is baked in, so a fix means a new image → new code hash → re-register.

### Deploy by digest, not tag

Attestation pins the code hash registered on-chain, so a rebuild invalidates it.
Mirror between registries instead of rebuilding:

```bash
crane copy <src>@sha256:<digest> <dst>@sha256:<digest>
```

### `SIMULATED_TEE=false` on real hardware

And `CHAIN_ID` must be set — unset leaves `chainID=0` and every signature comes back
empty (`signature must be 65 bytes, got 0`).

---

## When the extension image changes

1. Rebuild and hand off the new image.
2. The VM is re-deployed → `codeHash` changes.
3. `bash ./scripts/post-build.sh` whitelists the new codeHash.
4. `bash ./scripts/test.sh`.

## When the `FlareTeeManager` diamond is re-deployed

All extension registrations on that chain are wiped:

1. `bash ./scripts/pre-build.sh` — mints a fresh `EXTENSION_ID`.
2. Send the new `EXTENSION_ID` to the VM operator. They restart the container with `EXTENSION_ID=<new value>` as a launch-policy env override — no image rebuild needed.
3. Re-curl `/info` and confirm `extensionId` matches.
4. `bash ./scripts/post-build.sh`.
5. `bash ./scripts/test.sh`.
