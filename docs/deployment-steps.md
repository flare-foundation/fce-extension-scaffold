# 🚀 TEE Extension Deployment — Step by Step

Linear recipe to deploy a TEE extension to Flare Coston or Coston2. Run the steps top to bottom.

## Prerequisites

- 🐳 Docker Desktop (Linux containers)
- 🐹 Go 1.25.1+
- 🔨 Foundry (`forge`, `cast`)
- `jq`
- Bash (Git Bash on Windows works)
- VPN access to Flare's indexer DB (`35.241.249.150:3306`)

## 1. Clone sibling repos

The extension's Dockerfiles consume both repos from `../../tee-node/`.

```text
<workspace>/tee/
├── tee-node/         # gitlab.com/flarenetwork/tee/tee-node, tag v0.0.20
├── tee-proxy/        # gitlab.com/flarenetwork/tee/tee-proxy, tag v0.0.17
└── extensions/
    └── <your-extension>/
```

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

Copy `.env.example` to `.env.coston` or `.env.coston2`. Fill in:

```bash
CHAIN=coston2                                                         # or coston
CHAIN_URL=https://coston2-api.flare.network/ext/C/rpc                 # chain RPC
ADDRESSES_FILE=./config/coston2/deployed-addresses.json
NORMAL_PROXY_URL=https://tee-proxy-coston2-1.flare.rocks              # FTDC proxy
EXT_PROXY_URL=                                                        # leave empty — set in Step 6

LOCAL_MODE=false
SIMULATED_TEE=false
DEPLOYMENT_PRIVATE_KEY=<private key, no 0x prefix>
INITIAL_OWNER=0x<derived address from Step 2>
```

Activate it:

```bash
bash ./scripts/use-chain.sh <chain>
```

Copies `.env.<chain>` → `.env`, which all scripts auto-load.

## 4. Register the extension on-chain

```bash
bash ./scripts/pre-build.sh
```

Compiles Solidity, deploys `InstructionSender`, registers the extension on-chain. Writes `EXTENSION_ID` and `INSTRUCTION_SENDER` to `config/extension.env`.

Read the new values — `EXTENSION_ID` is part of the hand-off in Step 6:

```bash
cat config/extension.env
```

## 5. Build the Docker image

Confirm `MODE=0` is the default in your extension's `Dockerfile` (`MODE=0` is the production attestation backend; `MODE=1` produces simulated attestation that FTDC rejects):

```dockerfile
ENV MODE=0 CONFIG_PORT=5501 SIGN_PORT=7701 EXTENSION_PORT=7702
```

Then build:

```powershell
$env:SOURCE_DATE_EPOCH = (git log -1 --format=%ct)
docker compose -f docker-compose.yaml build --no-cache extension-tee
docker tag <your-extension>-extension-tee:latest <your-extension>:v0.1.0
docker save <your-extension>:v0.1.0 -o <your-extension>-v0.1.0.tar
```

Setting `SOURCE_DATE_EPOCH` makes the build reproducible (same source → same `codeHash`).

Verify `MODE=0` is baked into the image:

```powershell
docker inspect <your-extension>:v0.1.0 --format '{{range .Config.Env}}{{println .}}{{end}}' | Select-String MODE
# expected: MODE=0
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
| `extensionId`  | matches your `config/extension.env` `EXTENSION_ID`                |
| `initialOwner` | matches your `INITIAL_OWNER`                                      |

If `extensionId` is wrong, ask the VM operator to restart the container with the correct `EXTENSION_ID` env override (no image rebuild needed — it's a launch-policy override).

## 8. Register the TEE machine

> [!WARNING]
> Before running, ensure `scripts/post-build.sh` invokes `register-tee` with `-command rRap` (not the default `rap`):
>
> ```bash
> go run ./cmd/register-tee \
>     -a "$ADDRESSES_FILE" \
>     -c "$CHAIN_URL" \
>     -p "$EXT_PROXY_URL" \
>     -h "${EXT_PROXY_HOST_URL:-$EXT_PROXY_URL}" \
>     -ep "$NORMAL_PROXY_URL" \
>     -state "$PROJECT_DIR/config/register-tee.state" \
>     -command rRap \
>     || die "Register TEE failed"
> ```
>
> Step `a` (availability check) needs a one-time **challenge** — a random number from the contract that the TEE signs to prove it's alive. By default only `r` issues it, but `r` skips itself once the TEE is registered on-chain. So re-runs (image changes, diamond cuts, retries) revert with `Verification.ChallengeExpired`. Capital `R` issues the challenge directly — decoupled from `r` — so re-runs work.

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
