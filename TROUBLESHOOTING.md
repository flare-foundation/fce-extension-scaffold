# Troubleshooting

Symptom → cause → check → fix, for the failures that come up most when registering an
extension on a live network. Addresses are deliberately **not** listed here: read them
from `config/<chain>/deployed-addresses.json`, which is updated when the system
contracts move.

## Quick triage

| Symptom | Section |
|---|---|
| `FunctionNotFound(bytes4)`, `addTeeVersion` signature mismatch, unexplained `register()` revert, `only reward offers manager` | [Stale addresses or bindings](#stale-addresses-or-bindings) |
| Availability check polls forever / `404` from the FTDC proxy / machine stuck at `INITIALIZED` | [Availability check never completes](#availability-check-never-completes) |
| Proxy healthy but no inbound requests from validators | [Registered URL is not the one you serve](#registered-url-is-not-the-one-you-serve) |
| `code hashes do not match` | [Code hash and MODE](#code-hash-and-mode) |
| `Verification.ChallengeExpired` | [Challenge expired](#challenge-expired) |
| `getRandomTeeIds` empty / `MachineManager.TooMany()` | [No machine in PRODUCTION](#no-machine-in-production) |
| Proxy panics on TEE info / `invalid signature` / `could not sign` | [Node and proxy version skew](#node-and-proxy-version-skew) |
| Proxy won't start: `connection refused` / `access denied` | [Indexer database](#indexer-database) |

## How delivery actually works

Knowing the direction of travel saves the most time:

- The proxy reads chain state from a **C-chain indexer database**, not from an RPC node.
- **Data providers push instructions to the proxy URL recorded on-chain** at
  registration. Each provider posts its own signed vote; the proxy forwards the
  instruction to your extension once the signing-policy threshold is met.
- The availability-check verifier fetches your machine's `TEE_ATTESTATION` result from
  **your** proxy at `GET /action/result/<instructionId>`, then publishes an FDC2 proof
  that `toProduction` consumes.

So a locally perfect stack plus a wrong on-chain URL looks exactly like "the network is
ignoring me": your `/info` works, your queue stays empty, and the availability check
404s forever.

Machine status values: `0 NONE · 1 INITIALIZED · 2 PRODUCTION · 3 SUSPENDED · 4 PAUSED
· 5 BANNED`. Registration goes `register()` → `INITIALIZED` → availability proof →
`toProduction(proof)` → `PRODUCTION`; there is no other path.

## Stale addresses or bindings

`FlareTeeManager` is an EIP-2535 diamond, and the FCC system contracts have been
redeployed more than once on test networks. Code holding an old address, or bindings
that encode calls the live facets no longer expose, fails in confusing ways — a selector
that resolves to a facet with a different access modifier reverts with a message that
has nothing to do with your change, such as `only reward offers manager`.

Check that the address you are using is a live deployment — a current diamond exposes
`getLatestTeeGovernanceHash(uint256)` (selector `0x555f9961`):

```bash
DIAMOND=$(jq -r '.[]|select(.name=="FlareTeeManager").address' config/coston2/deployed-addresses.json)
cast call "$DIAMOND" "facetAddress(bytes4)(address)" 0x555f9961 --rpc-url "$CHAIN_URL"
# non-zero: live · 0x0: retired deployment
```

Fix: pull the latest revision of this repo, re-run `./scripts/generate-bindings.sh`, and
read addresses from `config/<chain>/deployed-addresses.json` rather than hardcoding
them. If your extension is branched off an older base, rebase — registration structs
have changed alongside address refreshes.

If the diamond was redeployed *after* you registered, your extension and machine records
no longer exist. Re-run `pre-build` for a fresh `EXTENSION_ID`, restart the TEE with that
ID, then `post-build`. If your extension ID is already present on the live diamond,
don't re-run `pre-build` — you don't need a new one.

## Availability check never completes

`404` on `<NORMAL_PROXY_URL>/action/result/<instructionId>` means no result is stored for
that instruction — the check didn't complete. On a healthy stack the proof appears within
seconds, so minutes of polling means something is misconfigured. In order:

1. Verify the registered URL (next section) — by far the most common cause.
2. Confirm your own proxy can serve the attestation result the verifier needs.
   `register-tee` logs the ID as `tee-attestation requested, instructionId: …`:
   ```bash
   curl -s "$EXT_PROXY_URL/action/result/<attestationInstructionId>" | head -c 200
   ```
   If *your* proxy 404s it, the TEE never produced the attestation; fix that first.
3. Check signing-policy alignment. The proxy's `lastSigningPolicyId` should equal the
   on-chain reward epoch (one ahead is legitimate for a couple of hours after a new
   policy is initialized). While a proxy is behind, provider votes are rejected and no
   proof can form:
   ```bash
   curl -s "$EXT_PROXY_URL/info" | jq .teeInfo.lastSigningPolicyId
   curl -s "$NORMAL_PROXY_URL/info" | jq .teeInfo.lastSigningPolicyId
   ```
   If yours lags, let it catch up and check `initial_signing_policy_offset` plus indexer
   sync.
4. Resume instead of re-registering — `register-tee` persists progress:
   ```bash
   go run ./cmd/register-tee -resume
   # or drive phases explicitly with -command rRap
   #   r register · R fresh challenge · a availability check · p toProduction
   # already have an instruction ID? go run ./cmd/register-tee -command p -i <id>
   ```

## Registered URL is not the one you serve

Providers push to the URL stored on-chain, so an ephemeral tunnel that changes hostname
on restart silently breaks delivery.

```bash
cast call "$DIAMOND" "getTeeMachine(address)((address,address,string))" <teeId> --rpc-url "$CHAIN_URL"
curl -s -o /dev/null -w '%{http_code}\n' "<url from above>/info"
```

If the URL doesn't match `EXT_PROXY_URL`, or doesn't return `200`, that's the bug. Update
`EXT_PROXY_URL` and re-run `post-build` so the on-chain record matches. Quick tunnels
(`cloudflared tunnel --url`, ngrok without a reserved domain) work for a single
uninterrupted run; for anything longer use a named cloudflared tunnel or a reserved
domain.

## Code hash and MODE

`allow-tee-version` whitelists whatever code hash the proxy `/info` reports, so
`SIMULATED_TEE` and the container's `MODE` must agree:

- simulated: `SIMULATED_TEE=true` with `MODE=1` (injected by Docker Compose); the code
  hash is a fixed constant and the platform reports as a test platform.
- real hardware: `SIMULATED_TEE=false` with `MODE=0`, on a GCP Confidential Space VM,
  with a real measured code hash.

Any rebuild that changes the hash needs re-whitelisting. Go builds are reproducible with
`SOURCE_DATE_EPOCH` set; Python/TypeScript are only same-machine deterministic, so
rebuilding elsewhere can change the hash.

## Challenge expired

The attestation challenge has a validity window. Re-run `post-build` and make sure
`register-tee` requests a fresh challenge — the capital `R` in `-command rRap`.

## No machine in PRODUCTION

Instruction routing only selects `PRODUCTION` machines, so `getRandomTeeIds` returns
empty and calls that route an instruction revert while your machine is `INITIALIZED`.

```bash
cast call "$DIAMOND" "getActiveTeeMachines(uint256)(address[])" <extensionId> --rpc-url "$CHAIN_URL"
```

`MachineManager.TooMany()` also fires when `config/extension.env` names a different
extension ID than the one your machine is registered under — typically after
`pre-build --force` created a new extension while an old machine was still registered.
Either do a full reset, or keep the existing `extension.env` and re-run only
`post-build` and `test`.

## Node and proxy version skew

`tee-node` and `tee-proxy` must agree on wire format and instruction signing. Recent
releases changed both, so a mismatched pair fails quietly: the proxy panics while
fetching initial TEE info, rejects responses with `invalid signature`, the extension logs
`error signing: could not sign`, or provider votes are discarded and instructions never
arrive. Keep both on the versions this repo pins (`go.mod` and `docker-compose.yaml`) —
never one old and one new. Rebuild with `--build` after changing versions; a stale
locally-tagged proxy image is reused silently otherwise.

## Indexer database

The proxy needs read access to a C-chain indexer MySQL database and won't start without
it. Copy the example config and fill the `[db]` block:

```bash
cp config/proxy/extension_proxy.<chain>.docker.toml.example \
   config/proxy/extension_proxy.<chain>.docker.toml
```

- `connection refused` / timeouts: network path to the indexer host is blocked (VPN,
  firewall, wrong host or port). Test reachability separately before suspecting auth.
- `Access denied for user …`: wrong or rotated credentials. Request current read-only
  credentials via [Flare technical support](https://flare.network/resources/technical-support);
  credentials found in old revisions or forks are dead. Don't commit credentials.

A proxy that is syncing reports a plausible `lastSigningPolicyId` on `/info`; zero or
absent means the database isn't being read.

## Log lines that are not errors

- `machine_path_manager address not set; machine path list service disabled` — optional
  feature, unrelated to registration.
- `Warning: Error loading .env file` from Go tools run via the scripts — the scripts
  export the environment themselves.
- `NOTE: Code hash is from proxy /info response — not independently verified against
  attestation` — expected in simulated mode.

## Still stuck?

Include the exact error string, the machine status and on-chain URL from the `cast` calls
above, your `tee-node`/`tee-proxy` versions, and whether you are in simulated or real
attestation mode. Those five facts identify almost every case above.
