# Testing against a Coston2 deployment

For exercising an extension that is already deployed and registered on Coston2 —
either on a Confidential Space VM, or locally with a public tunnel. To deploy one
first, see [deployment-steps.md](deployment-steps.md).

## What you need

| Need | Note |
|---|---|
| `.env.coston2` | `use-chain.sh coston2` creates and activates it |
| A funded Coston2 key | sends the test instructions; get C2FLR from the [faucet](https://faucet.flare.network/coston2) |
| `config/coston2/deployed-addresses.json` | the `FlareTeeManager` diamond and friends |
| `config/proxy/extension_proxy.coston2.docker.toml` | gitignored; `use-chain.sh coston2` generates it — verify the `[db]` credentials it carried over |
| A reachable `EXT_PROXY_URL` | the VM's URL, or a tunnel if the proxy runs locally |

## Confirm the deployment is live

```bash
curl -s "$EXT_PROXY_URL/info" | jq '{extensionId, codeHash, platform}'
```

| Field | Expect |
|---|---|
| `extensionId` | matches `EXTENSION_ID` in `config/coston2/extension.env` |
| `codeHash` | the hash registered on-chain — `0x194844cf…` means simulated, not real hardware |
| `platform` | `GCP_AMD_SEV` on real hardware, `TEST_PLATFORM` when simulated |

Then check the on-chain side:

```bash
cd tools && source "../config/coston2/extension.env"
A=../config/coston2/deployed-addresses.json
REG=$(jq -r '.[]|select(.name=="FlareTeeManager").address' $A)
go run ./cmd/query-tee -ext "$((EXTENSION_ID))" -reg "$REG" -rpc "$CHAIN_URL"
go run ./cmd/verify-deploy -a $A -c "$CHAIN_URL"
```

`-ext` takes the **decimal** extension id and `-reg` defaults to a stale address —
pass both as above or the query silently returns nothing.

`query-tee` lists the TEE machines registered for the extension. **More than one
active machine is a problem** — instructions are load-balanced across them, so a
stale one swallows roughly half your requests. See
[deployment-steps.md](deployment-steps.md).

## Run the test

```bash
./scripts/use-chain.sh coston2
./scripts/test.sh
```

Sends `SAY_HELLO` then `SAY_GOODBYE` through the deployed extension and validates
the decoded responses. A pass looks like:

```
Response data: {Greeting:Hello, World! … GreetingNumber:1}
Test passed: SAY_HELLO instruction processed successfully
Response data: {Farewell:Goodbye, World! … FarewellNumber:1}
All tests passed.
```

Counters are in memory, so the numbers restart at 1 after any TEE relaunch.

## Testing a local proxy against Coston2

The proxy must be publicly reachable for FTDC data providers to answer it. Start
the tunnel and let the scripts wire the URL in:

```bash
./scripts/full-setup.sh --chain coston2 --tunnel --test
```

That writes the tunnel URL into `.env` as `EXT_PROXY_URL`, so `post-build.sh` and
`test.sh` pick it up. Details in [cloudflared.md](cloudflared.md).

## If something's blocked

| Symptom | Cause |
|---|---|
| `pollAction` timeout, `/action/result` 404 | multiple active TEE machines; pause the stale ones |
| `Verification.ChallengeExpired` | re-registration without `-command rRap` |
| `no round` / 404 from the FTDC proxy | proxy signing policy out of sync with the on-chain reward epoch; `register-tee` pre-flights this |
| `signature must be 65 bytes, got 0` | `CHAIN_ID` unset on the node |
| `InvalidTeePublicKeyOrSignature` | node `CHAIN_ID`, proxy `chain_id` and the registry disagree — all three must say 114 |
| Instructions never arrive | `EXT_PROXY_URL` not reachable from outside, or a rotated tunnel URL left stale in `.env` |
| `Extension ID already set.` | `setExtensionId()` is one-shot; a redeploy is the only reset |
