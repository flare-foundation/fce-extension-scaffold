# Getting started — local

Runs the Hello World scaffold end to end on a local devnet: deploy the
`InstructionSender`, register the extension, start the TEE node + proxy, and send
`SAY_HELLO` / `SAY_GOODBYE` through it.

## Prerequisites

| Need | Why |
|---|---|
| Docker + Compose | runs the extension TEE, proxy and redis |
| Go 1.25+ | `tools/` CLIs, and the `go` language implementation |
| Foundry (`forge`, `cast`) | compiles `contracts/`, reads chain state |
| Node 22+ / Python 3.11+ | only for `LANGUAGE=typescript` / `python` |
| A funded key | deploys contracts and registers the extension |
| FCC infrastructure running | Hardhat node + indexer + redis + the "normal" TEE proxy. Not in this repo — see `../../e2e/` |

## Pick a language

The scaffold ships the same extension in Go, Python and TypeScript. Discovery is
by directory: each implementation has a `<lang>/language.env` manifest.

```bash
LANGUAGE=go ./scripts/full-setup.sh --test      # or python, typescript
```

`LANGUAGE` can also live in `.env`. See [languages.md](languages.md).

## One command

```bash
./scripts/use-chain.sh local    # writes .env.local, activates it as .env
./scripts/full-setup.sh --test
```

That chains pre-build (deploy + register) → start-services (node, proxy, redis) →
post-build (allow version, set governance, register TEE) → `test.sh`.

For a testnet, pick simulated or live first — `--simulated` runs the TEE locally
behind a Cloudflare tunnel, without it you point at a devops-hosted TEE:

```bash
./scripts/use-chain.sh coston2 --simulated   # writes .env.local.coston2, activates it
./scripts/full-setup.sh --chain coston2 --test
```

## Verify it works

```bash
docker compose ps                      # redis, ext-proxy, extension-tee
curl -s http://localhost:6674/info     # extension id, code hash, platform
./scripts/test.sh                      # SAY_HELLO + SAY_GOODBYE round-trip
```

A passing run prints `Hello, World! Welcome to Flare Confidential Compute.` and
`Goodbye, World! Reason: heading out`.

## Configuration

`./scripts/use-chain.sh <chain>` is the entry point: it creates `.env.<chain>`,
the proxy toml and the compose override for that chain (never overwriting), then
copies `.env.<chain>` to `.env`. Edit `.env.<chain>`, not `.env` — `.env` is a
disposable copy. Chains: `local`, `coston`, `coston2`, `songbird`, `flare`.

| Command | Creates | Activated as `.env` |
|---|---|---|
| `use-chain.sh local` | `.env.local` | `.env.local` |
| `use-chain.sh coston` | `.env.coston` | `.env.coston` |
| `use-chain.sh coston --simulated` | `.env.coston`, then `.env.local.coston` | `.env.local.coston` |
| `use-chain.sh coston2` | `.env.coston2` | `.env.coston2` |
| `use-chain.sh coston2 --simulated` | `.env.coston2`, then `.env.local.coston2` | `.env.local.coston2` |

`--simulated` runs the TEE yourself against a real chain: the simulated file is
*cloned from the live one* with `SIMULATED_TEE=true`, so the stack runs `MODE=1`
behind a Cloudflare tunnel. Without it the chain is *live* — `MODE=0`, a
devops-hosted TEE, and `start-services.sh` refuses to bring up the local Docker
stack because a plain container cannot attest. The clone happens once: later
edits to `.env.<chain>` never reach an existing `.env.local.<chain>`.

`--language <lang>` switches implementation, rejecting one this repo has no
`<lang>/language.env` for. `--list` shows which chains exist; `--status` prints
the active chain, mode, language and config files; `--versions` shows the
tee-node / tee-proxy pins against the latest upstream tags. The last two also
have `-s` and `-v` shorthands.

| Var | Default | Note |
|---|---|---|
| `LANGUAGE` | `go` | which implementation directory gets built |
| `DEPLOYMENT_PRIVATE_KEY` | Hardhat dev key | funded deployer |
| `CHAIN_URL` / `CHAIN_ID` | written per chain by `use-chain.sh` | local `31337`, coston `16`, coston2 `114`. `CHAIN_ID` is **required** — unset leaves `chainID=0` and every TEE signature comes back empty |
| `SIMULATED_TEE` | `true` unless `use-chain.sh` ran without `--simulated` | drives `MODE` (`1`/`0`) and decides who owns the tunnel; `false` means a devops-hosted TEE |
| `EXT_PROXY_URL` | left empty by `use-chain.sh` | this extension's proxy; must be publicly reachable on testnets |
| `NORMAL_PROXY_URL` | `localhost:6662` local, `tee-proxy-<chain>-1.flare.rocks` testnet | the infrastructure FTDC proxy (post-build) |
| `ADDRESSES_FILE` | `config/<chain>/…` on testnets | empty on local, where the sim_dump is auto-detected |
| `EXTENSION_ID` | from `config/<chain>/extension.env` | bytes32 hex, written by pre-build |
| `INSTRUCTION_SENDER` | from `config/<chain>/extension.env` | contract address, written by pre-build |
| `INITIAL_OWNER` | derived from the deployer key | initial contract owner |
| `PROXY_PRIVATE_KEY` | Hardhat dev key | proxy signing key |
| `EXTENSION_OWNER_KEY` | falls back to `DEPLOYMENT_PRIVATE_KEY` | key override for `allow-tee-version` |
| `TEE_VERSION` | `v0.1.0` | version string for TEE registration |
| `GOVERNANCE_SIGNERS` | `INITIAL_OWNER` | comma-separated 0x addresses — see below |
| `GOVERNANCE_THRESHOLD` | `1` | minimum distinct governance signatures |
| `WAIT_TIMEOUT` | `120` | service wait timeout, seconds |
| `REGISTRY` | (unset) | pull images from a remote registry instead of building |
| `LOG_LEVEL` | `INFO` | `DEBUG` for verbose container logs |

### TEE governance

Every TEE machine registers under a **governance** — a signer set plus a threshold
that authorises governance actions for the extension. Two parties must agree on it or
`register-tee` reverts with `InvalidGovernanceHash`: the **TEE node**, which signs its
machine data with a `governanceHash` derived from `(signers, threshold)`, and the
**on-chain registry** where the governance is registered.

The scaffold keeps them consistent by reading both from `.env`:

```bash
GOVERNANCE_SIGNERS="0xAbc...,0xDef..."   # comma-separated 0x addresses
GOVERNANCE_THRESHOLD=2
```

Unset, both default to the deployer as sole signer, threshold 1 — fine for
development. `post-build.sh` registers the set on-chain idempotently before
`register-tee`, and passes the same values to the node container via Compose.

## Ports

| Port | What |
|---|---|
| 6674 | extension proxy, external (Docker) |
| 6673 | extension proxy, internal (Docker) |
| 6664 | extension proxy when run as a local Go process (`--local`) |
| 6662 | the "normal" FTDC proxy (infrastructure, not this repo) |
| 6382 | this extension's redis |

## Stopping

```bash
./scripts/stop-services.sh --chain local
./scripts/stop-services.sh --chain coston2 --tunnel   # force-stop the tunnel in live mode
```

## Common failures

| Symptom | Cause |
|---|---|
| `docker-compose.<chain>.yaml not found` | run `./scripts/use-chain.sh <chain>` to generate it |
| port `6674`/`6382` already allocated | another chain's stack is still up — every chain binds the same host ports, so stop it first |
| `config/proxy/extension_proxy.<chain>.docker.toml not found` | `use-chain.sh <chain>` generates it; fill in the `[db]` credentials |
| docker `rootfs` mount error, or the path is now a directory | an older run mounted the missing config; `rm -rf` the directory, then re-run `use-chain.sh <chain>` |
| `tee-node v… is below the v0.0.24 minimum` | bump the pin in `go/go.mod` **and** `tools/go.mod` |
| `tee-proxy v… is below the v0.0.19 minimum` | bump the pin in `tools/go.mod` **and** `proxy/Dockerfile` — v0.0.18 cannot advance signing policies |
| `tee-node mismatch` from `check-versions.sh` | the two `go.mod` pins drifted; align them or the Go and non-Go images run different builds |
| `signature must be 65 bytes, got 0` | `CHAIN_ID` unset → `chainID=0` |
| `Verification.ChallengeExpired` | `register-tee` ran without `-command rRap` |
| `InvalidGovernanceHash` | `GOVERNANCE_SIGNERS` / `GOVERNANCE_THRESHOLD` differ between `post-build.sh` and the node container |
| `Extension ID already set.` | `setExtensionId()` is one-shot; redeploy the `InstructionSender` |
| `EXTENSION_ID … not found in proxy /info` | the proxy is filtering for a different extension |
| proxy `/info` wait times out on a testnet | `EXT_PROXY_URL` is not reachable from outside — start a tunnel ([cloudflared.md](cloudflared.md)) |
| `pollAction` timeout / `/action/result` 404 | more than one active TEE machine; see [deployment-steps.md](deployment-steps.md) |
