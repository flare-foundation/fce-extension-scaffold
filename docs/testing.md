# Testing

Tests split into two groups: those covering **your extension**, and those covering the **deployment tooling**. Cheapest first — only the last needs a chain.

## Your extension

| Layer | What it tests | How to run | Chain? |
|-------|--------------|------------|--------|
| **Unit** | Your handlers, dispatch, encoding — in your chosen language | `./scripts/test-unit.sh` | no |
| **Conformance** | The wire contract, against golden fixtures | `./scripts/test-conformance.sh` | no |
| **End-to-end** | Full instruction lifecycle (deploy → send → process → verify) | `./scripts/test.sh` | yes |

Both script-driven layers accept a language or `--all`:

```bash
./scripts/test-unit.sh python
```

```bash
./scripts/test-conformance.sh --all
```

They dispatch through `LANGUAGE_TEST_CMD` in each `<lang>/language.env`, so they work for any language you add without modification.

### Conformance testing

This is the layer that keeps multiple implementations honest, and the acceptance test for a new language.

`scripts/test-conformance.sh` starts **only the extension process** — no tee-node, no proxy, no chain, no Docker — replays the 16 fixtures in `testdata/conformance/`, and diffs every response field. The response payload is compared **byte-for-byte**, because tee-node hashes it and signs the result.

```bash
./scripts/test-conformance.sh --all
```

It runs in seconds, which is what makes writing a new language port tractable: a correctness signal without a full deploy cycle.

The fixtures are **order-dependent** and share one process — counters accumulate across cases and the final fixture asserts the resulting state. `index.json` fixes the order.

If you change a request or response shape, regenerate rather than hand-editing:

```bash
./python/.venv/bin/python testdata/conformance/gen_fixtures.py
```

Fixtures are generated from `gen_fixtures.py` so the hex encodings are derived rather than hand-assembled. Adding a case is a code edit there.

> When this suite disagrees with your implementation, check [extension-contract.md](extension-contract.md) before changing the fixture. The contract is normative; the fixtures encode it.

## The deployment tooling

`tools/` is a standalone Go module with no dependency on any language implementation — which is what lets one deployment and test path serve Go, Python and TypeScript alike. It has its own tests.

| Layer | What it tests | How to run |
|-------|--------------|------------|
| **Unit tests** | Revert decoding, state file I/O, env parsing, validation, report formatting | `cd tools && go test ./...` |
| **Integration tests** | On-chain constructor validation, revert reasons, idempotent registration, pre-flight checks | `cd tools && go test -tags integration ./integration/ -v` |

### Tooling unit tests

These require no external services. They cover:

- **Revert reason decoding** (`tools/pkg/fccutils/revert_test.go`) — Verifies `decodeRevertHex` and `DecodeRevertReason` correctly decode ABI-encoded `Error(string)` reverts, including all 7 revert messages from `InstructionSender.sol`. Also tests edge cases: nil errors, wrapped errors, custom error selectors, invalid hex, short data.
- **Support revert decoding** (`tools/pkg/support/support_test.go`) — Tests `decodeRevertFromError` which extracts revert reasons from go-ethereum JSON-RPC error types.
- **State file I/O** (`tools/pkg/fccutils/registration_test.go`) — Tests `loadState`/`saveState` for the TEE machine registration resume flow: missing files, valid/invalid JSON, overwrite behavior, roundtrip consistency, read-only directories.
- **Validation checks** (`tools/pkg/validate/checks_test.go`) — Extension env format validation, deployer key source detection, service/registration/TEE check functions with various config states.
- **Report formatting** (`tools/pkg/validate/report_test.go`) — Report summary, JSON output, colored terminal output, empty reports, unknown statuses.
- **Validation primitives** (`tools/pkg/validate/validate_test.go`) — `AddressNotZero`, `AddressHasCode`, `KeyHasFunds`, `IsUsingDevKey` with nil clients, zero addresses, edge cases.

```bash
cd tools && go test ./... -v
```

### Tooling integration tests

Integration tests run against a live Ethereum node (Hardhat, Anvil, or Coston2). They are excluded from `go test ./...` via the `integration` build tag.

**What they test:**

- **Constructor validation** — Deploys `InstructionSender` with zero addresses, EOA addresses, and valid addresses. Verifies revert messages are decoded correctly (not binary garbage).
- **setExtensionId errors** — Calls `setExtensionId` before registration ("Extension ID not found.") and after it's already set ("Extension ID already set."). Verifies the full revert decoding chain works: `DecodeRevertReason` → `SimulateAndDecodeRevert` fallback.
- **CheckTx revert reasons** — Submits transactions that revert on-chain (with manual gas limit to bypass estimation), then verifies `CheckTx` replays the call and returns human-readable revert reasons.
- **Idempotent registration** — Runs `SetupExtension` twice with the same instruction sender address. Verifies the second run detects the existing registration and returns the same extension ID without submitting duplicate transactions.
- **Pre-flight validation** — Tests `AddressHasCode` against deployed registry contracts and random EOAs. Tests `KeyHasFunds` against the funded deployer and unfunded random keys.

**Running against a local node:**

```bash
cd tools && go test -tags integration ./integration/ -v -count=1
```

Defaults: `CHAIN_URL=http://127.0.0.1:8545`, addresses file at `config/coston2/deployed-addresses.json`.

**Running against Coston2:**

```bash
cd tools && CHAIN_URL=https://coston2-api.flare.network/ext/C/rpc \
  DEPLOYMENT_PRIVATE_KEY=<your-funded-key> \
  go test -tags integration ./integration/ -v -count=1
```

**Environment variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `CHAIN_URL` | `http://127.0.0.1:8545` | RPC endpoint |
| `ADDRESSES_FILE` | `../../config/coston2/deployed-addresses.json` | Path to deployed registry addresses |
| `DEPLOYMENT_PRIVATE_KEY` | Hardhat dev key | Funded private key for deployments and transactions |

**Note:** Integration tests deploy fresh contracts on each run. On Coston2, this costs gas. On a local node, it's free.

## End-to-End Tests

After post-build completes, you can send instructions to your extension and verify the results:

```bash
./scripts/test.sh
```

Or run everything in one shot:

```bash
./scripts/full-setup.sh --test
```

## What the test does

The test runner (`tools/cmd/run-test/main.go`) executes this lifecycle:

```
1. SetExtensionId()         ← Generic: tells the contract its extension ID (idempotent)
2. Send instruction          ← YOUR CODE: call your contract function with your payload
3. Wait for TEE processing   ← Generic: time.Sleep(5s)
4. Poll for result            ← Generic: fccutils.ActionResult() polls proxy (15 retries, 2s apart)
5. Validate response          ← YOUR CODE: unmarshal Data into your type, check your fields
```

Steps 1, 3, and 4 are the same for every extension. Steps 2 and 5 are what you customize.

## How the scaffold test works

The scaffold's test sends instructions via `SendSayHello` and `SendSayGoodbye` and verifies the responses. Here's how each part works — when you build your own extension, you'll replace these with your own types, payloads, and assertions.

### 1. Define your message and response types

The scaffold defines `sayHelloResponse` and `sayGoodbyeResponse` at the top of the test file. They **mirror** your extension's response shapes rather than importing them — `tools/` is deliberately independent of every language implementation, which is what lets this one test run unchanged against Go, Python and TypeScript:

```go
type sayHelloResponse struct {
    Greeting       string `json:"greeting"`
    GreetingNumber int    `json:"greetingNumber"`
}

type sayGoodbyeResponse struct {
    Farewell       string `json:"farewell"`
    FarewellNumber int    `json:"farewellNumber"`
}
```

Replace these with structs matching your extension's response shapes.

### 2. Send your instructions

The scaffold builds JSON payloads and sends them through the contract. The scaffold includes two test cases:

```go
// SAY_HELLO test case
payload, _ := json.Marshal(map[string]interface{}{
    "name": "World",
})
instructionId, _, err := instrutils.SendSayHello(s, addr, payload)

// SAY_GOODBYE test case — ABI-encoded (string,string), so the binding
// takes typed args rather than a JSON payload
instructionId, _, err = instrutils.SendSayGoodbye(s, addr, "World", "heading out")
```

Replace the payloads with whatever your contract functions expect. If your Solidity contract has multiple send functions, you'll need to add corresponding Go helpers in `tools/pkg/utils/instructions.go` and call them here.

### 3. Validate your responses

The `verifyHelloResult` and `verifyGoodbyeResult` functions each receive the raw response from the proxy. The response envelope is always the same:

```json
{
  "result": {
    "id": "0x...",
    "status": 1,
    "log": "",
    "opType": "0x...",
    "opCommand": "0x...",
    "data": "<your extension's JSON response>"
  }
}
```

- `status`: `0` = failed, `1` = success, `2` = pending
- `log`: error message when `status == 0`
- `data`: your extension's response bytes — this is whatever your `processAction` handler returned via `buildResult`

The generic status checks are already in `verifyHelloResult` and `verifyGoodbyeResult`. The scaffold validates the SAY_HELLO response like this:

```go
// verifyHelloResult
var resp sayHelloResponse
err = json.Unmarshal(actionResult.Data, &resp)
if err != nil {
    return errors.Errorf("failed to unmarshal response: %s", err)
}

if resp.Greeting == "" {
    return errors.New("expected non-empty Greeting")
}
if resp.GreetingNumber < 1 {
    return errors.Errorf("expected GreetingNumber >= 1, got %d", resp.GreetingNumber)
}
```

And the SAY_GOODBYE response like this:

```go
// verifyGoodbyeResult
var resp sayGoodbyeResponse
err = json.Unmarshal(actionResult.Data, &resp)
if err != nil {
    return errors.Errorf("failed to unmarshal response: %s", err)
}

if resp.Farewell == "" {
    return errors.New("expected non-empty Farewell")
}
if resp.FarewellNumber < 1 {
    return errors.Errorf("expected FarewellNumber >= 1, got %d", resp.FarewellNumber)
}
```

Replace the response types, unmarshal targets, and field assertions with your own.

### 4. Add more test cases

The scaffold shows two send+verify pairs (SAY_HELLO and SAY_GOODBYE). For a real extension, add multiple test cases covering:

- Each op type your extension supports
- Success cases with valid inputs
- Edge cases (empty fields, boundary values)
- Error cases (invalid payloads that should return `status == 0`)

### Matching op types between Solidity and your extension

Your Solidity contract defines op types and op commands as `bytes32` constants:

```solidity
bytes32 constant OP_TYPE_GREETING  = bytes32("GREETING");
bytes32 constant OP_COMMAND_SAY_HELLO   = bytes32("SAY_HELLO");
bytes32 constant OP_COMMAND_SAY_GOODBYE = bytes32("SAY_GOODBYE");
```

Your extension routes on the same values. Go matches them explicitly in `processAction`:

```go
case dataFixed.OPType == teeutils.ToHash(config.OPTypeGreeting) &&
    dataFixed.OPCommand == teeutils.ToHash(config.OPCommandSayHello):
    return e.processSayHello(action, dataFixed)
```

Python and TypeScript register the pair up front and let the framework dispatch:

```python
framework.handle(OP_TYPE_GREETING, OP_COMMAND_SAY_HELLO, handle_say_hello)
```

```typescript
framework.handle(OP_TYPE_GREETING, OP_COMMAND_SAY_HELLO, handleSayHello);
```

All three compare the same bytes32 encoding — the UTF-8 string right-padded with zeros to 32 bytes. **The strings must match Solidity exactly**; a mismatch is not a compile error, it silently falls through to "unsupported op type" (HTTP 501) at runtime.

The test sends instructions through the contract functions (`sendSayHello`, `sendSayGoodbye`), which set the OPType to `GREETING` and the corresponding OPCommand, then verifies the response matches whatever your handler returned. Because the test asserts on the wire format rather than on language types, it is identical for every implementation.

## What you need to change (summary)

| Step | What to change | Where |
|------|---------------|-------|
| Response types | Define structs for your extension's responses | `tools/cmd/run-test/main.go` (top of file) |
| Message payloads | Create the JSON your contract function expects | `main()` in `run-test/main.go` |
| Send instructions | Call your contract's specific function(s) (e.g. `SendSayHello`, `SendSayGoodbye`) | `main()` in `run-test/main.go` |
| Validate responses | Unmarshal `Data` and assert your fields | `verifyHelloResult()` / `verifyGoodbyeResult()` in `run-test/main.go` |
| Op type + command routing | Match the constants across Solidity and your extension | `go/internal/config/config.go`, `python/app/config.py`, or `typescript/src/app/config.ts` |
| Add test scenarios | Add more send+verify pairs for each op command | `main()` in `run-test/main.go` |
| Conformance fixtures | Regenerate after any request/response shape change | `testdata/conformance/gen_fixtures.py` |

None of the `run-test` changes are language-specific: the tool asserts on the wire format, so one set of edits covers every implementation you maintain.
