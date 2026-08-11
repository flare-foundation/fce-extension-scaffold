# Docs index

Standard layout shared by every FCC extension example. Validate with
`./scripts/check-docs.sh`.

## Required — every extension has these

| Doc                                                      | Answers                                       |
| -------------------------------------------------------- | --------------------------------------------- |
| [getting-started.md](getting-started.md)                 | run it locally, end to end                    |
| [deployment-steps.md](deployment-steps.md)               | deploy to Coston2 and operate it              |
| [testing.md](testing.md)                                 | test suites and what they cover               |
| [testing-against-coston2.md](testing-against-coston2.md) | test against a deployed extension             |
| [architecture.md](architecture.md)                       | how this extension works                      |
| [cloudflared.md](cloudflared.md)                         | expose a local proxy for testnet registration |

`deployment-steps.md` must cover the platform-wide traps, because they are not
obvious and every extension hits them:

- the TEE key is in memory only — every relaunch mints a new identity, and the old
  machine stays **active** and keeps receiving instructions
- one-shot bindings (here `setExtensionId`) must be written **last**
- the Confidential Space launch policy aborts on the first env var outside
  `tee.launch_policy.allow_env_override`
- deploy by **digest**, not tag — the code hash is registered on-chain
- `SIMULATED_TEE=false` on real hardware

## Scaffold — keep byte-identical across extensions

Provided by the scaffold; do not fork these:

[extension-guide.md](extension-guide.md) ·
[instruction-sender.md](instruction-sender.md) ·
[manual-setup.md](manual-setup.md)

`types-server.md` does not apply here — this repo has no types server.

## Extension-specific

This repo is the scaffold itself, so its specifics are about being copied:

[languages.md](languages.md) — the multi-language layout, and adding your own ·
[extension-contract.md](extension-contract.md) — the contract an implementation must satisfy

## Style

Written for testers, not authors. Short, plain, skimmable: tables over prose,
runnable commands over description, and a symptom→cause table for failures. If a
section does not change what someone types or decides, cut it.
