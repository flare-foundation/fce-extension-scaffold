# Architecture

A minimal Flare Confidential Compute extension: two instructions (`SAY_HELLO`,
`SAY_GOODBYE`) that echo a greeting and a per-caller counter. It exists to be
copied, so the interesting part is the shape, not the logic.

## Components

| Piece | Where | Role |
|---|---|---|
| `InstructionSender` | `contracts/InstructionSender.sol` | on-chain entry point; emits instructions the TEE picks up |
| Extension | `<language>/` | your handler — decodes the instruction, returns a response |
| tee-node | pinned dep (`go/go.mod`) | runs inside the TEE, signs responses, talks to the proxy |
| tee-proxy | `proxy/Dockerfile` | bridges TEE ↔ chain/FTDC; serves `/info` and `/action/result` |
| redis | compose service | the proxy's queue |
| Deployment tooling | `tools/cmd/*` | deploy, register, allow versions, query state |

On-chain the extension is identified by an **extension id**, assigned by
`TeeExtensionRegistry` and latched into the contract by `setExtensionId()`.

## Request flow

```
caller → InstructionSender.sendInstruction()   (on-chain tx)
       → tee-proxy picks up the instruction
       → tee-node (inside the TEE) → your handler
       → signed response → proxy → /action/result
```

The node signs every response against `CHAIN_ID`; a mismatch with the proxy's
`chain_id` or the on-chain registry fails verification.

## Language-neutral spine

The repo root is language-agnostic. Each implementation is a sibling directory
marked by a `language.env` manifest, and scripts glob `*/language.env` to discover
them — there is no hardcoded list.

| Directory | Layout |
|---|---|
| `go/` | `cmd/docker` (image entry), `cmd/start-tee` (local), `internal/extension` (handler), `internal/config` (op codes), `pkg/types` |
| `python/` | `app/handlers.py` (handler), `app/config.py`, `base/` (server + encoding) |
| `typescript/` | mirrors `python/` |

`language.env` declares the Dockerfile plus the setup / build / test / run commands,
so the scripts never need to know a language exists. See
[languages.md](languages.md) and [extension-contract.md](extension-contract.md).

Go links tee-node as a library; the other languages run it as a separate prebuilt
binary and speak HTTP to it, which is why `build-node-base.sh` exists.

## Op codes

Instructions carry an op command hashed to `bytes32`. The Solidity side and the
handler must agree on the string.

| Op | Handler | Response |
|---|---|---|
| `SAY_HELLO` | `processSayHello` | `Greeting`, `GreetingNumber` |
| `SAY_GOODBYE` | `processSayGoodbye` | `Farewell`, `FarewellNumber` |

Defined in `go/internal/config/config.go` (and the equivalent per language). An
unmatched op is rejected with the expected hash logged — the usual cause of a
mismatch is editing one side only.

## State

In-memory counters, keyed by caller. Nothing is persisted: **the TEE has no durable
storage**, so a relaunch resets counts and mints a new `teeId`. Real extensions
that need state must encrypt and export it.

## Entry points

| Script | Does |
|---|---|
| `use-chain.sh` | create + activate everything one chain needs (env, proxy toml, compose override) |
| `chain-env.sh` | the chain table every script sources — chain ids, RPCs, `config/<chain>/` |
| `pre-build.sh` | generate bindings, compile, deploy `InstructionSender`, register extension |
| `start-services.sh` | build + start node/proxy/redis, sync the tunnel on testnets |
| `post-build.sh` | allow TEE version, set governance, register the TEE machine |
| `test.sh` | end-to-end round-trip through a running deployment |
| `full-setup.sh` | all of the above in order |
| `check-versions.sh` | fails the build when dependency pins drift or fall below the floor |

## Version pinning

`go/go.mod` is the single source of truth for the tee-node version;
`scripts/lib/versions.sh` derives `TEE_NODE_REF` from it so non-Go images clone the
same ref. `tools/go.mod` must match, and `check-versions.sh` enforces both that and
the minimum version.

## Where to look next

[getting-started.md](getting-started.md) to run it ·
[extension-guide.md](extension-guide.md) to write your own handler ·
[deployment-steps.md](deployment-steps.md) for Coston2
