# Extension Development Guide

This guide explains how the extension scaffold works and how to implement your own logic.

> **This guide's code examples are Go.** The scaffold also ships Python (`python/`) and TypeScript (`typescript/`) implementations of the same extension, selected with `LANGUAGE` in `.env`. The architecture, data flow, op-type matching and 4-step handler pattern described here are identical in all three — only the syntax differs. Where this guide says `go/internal/config/config.go`, the equivalents are `python/app/config.py` and `typescript/src/app/config.ts`; for `go/internal/extension/extension.go`, they are `python/app/handlers.py` and `typescript/src/app/handlers.ts`.
>
> For the language-independent specification of the HTTP surface, wire format and container requirements — including what to implement if you are adding a fourth language — see [extension-contract.md](extension-contract.md).

## How an Extension Works

An extension is an HTTP server that runs inside a Trusted Execution Environment (TEE). It receives instructions from the blockchain, processes them, and returns results. The full lifecycle:

```
1. User calls your Solidity contract (on-chain)
2. Contract emits a TeeInstructionsSent event via TeeExtensionRegistry
3. TEE proxy picks up the instruction from the chain
4. TEE node fetches the instruction from the proxy
5. TEE node forwards it as POST /action to your extension server
6. Your extension processes the action and returns a result
7. TEE node sends the result back to the proxy
8. Caller polls the proxy for the result
```

Your extension controls steps 1 (the contract) and 6 (the action handler). Everything else is handled by the TEE infrastructure.

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│  YOUR CODE (what you customize)                     │
│                                                     │
│  contracts/InstructionSender.sol   On-chain entry   │
│  <lang> config                     OPType constants │
│  <lang> handlers                   Action handlers  │
│  tools/cmd/run-test/main.go        E2E tests        │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  INFRASTRUCTURE (do not modify)                     │
│                                                     │
│  <lang> framework layer            Server, routing  │
│  buildResult() / envelope          Result builder   │
│  scripts/*                         Build/deploy     │
│  tools/cmd/deploy-contract/        Deployment       │
│  tools/cmd/register-*/             Registration     │
└─────────────────────────────────────────────────────┘
```

## File map by language

The guide below uses Go for its examples. These are the equivalents:

| Role | Go | Python | TypeScript |
|---|---|---|---|
| Op constants | `go/internal/config/config.go` | `python/app/config.py` | `typescript/src/app/config.ts` |
| Handlers | `go/internal/extension/extension.go` | `python/app/handlers.py` | `typescript/src/app/handlers.ts` |
| Request/response types | `go/pkg/types/types.go` | declared inline in handlers | declared inline in handlers |
| ABI decoding | `go/pkg/types/types.go` (`abi.Argument`) | `python/app/abi.py` | `typescript/src/app/abi.ts` |
| Framework (don't touch) | `internal/extension/utils.go`, `pkg/server/` | `python/base/` | `typescript/src/base/` |
| Entry point | `go/cmd/main.go` | `python/main.py` | `typescript/src/main.ts` |
| Shared, language-neutral | `contracts/`, `scripts/`, `tools/`, `config/` | — | — |

Each language directory has its own README with the specifics: [go/](../go/README.md), [python/](../python/README.md), [typescript/](../typescript/README.md).

## The Files You Modify

### 1. `go/internal/config/config.go` — Operation Type Constants

This file defines the string constants for your operation types. Each constant is hashed to `bytes32` at runtime using `teeutils.ToHash()` and compared against the `OPType` field in incoming actions.

**What to do:** Add one `OPType` constant per logical operation group your extension supports, and one `OPCommand` constant per individual command within that group. The scaffold defines:

```go
const (
    OPTypeGreeting     = "GREETING"
    OPCommandSayHello  = "SAY_HELLO"
    OPCommandSayGoodbye = "SAY_GOODBYE"
)
```

These strings must exactly match the `bytes32` constants in your Solidity contract:

```solidity
bytes32 constant OP_TYPE_GREETING      = bytes32("GREETING");
bytes32 constant OP_COMMAND_SAY_HELLO  = bytes32("SAY_HELLO");
bytes32 constant OP_COMMAND_SAY_GOODBYE = bytes32("SAY_GOODBYE");
```

### 2. `go/pkg/types/types.go` — Request and Response Types

This file defines the JSON structures for your extension's inputs and outputs.

**Request types** are decoded from the instruction's `OriginalMessage` field. This is the payload the user sent through your Solidity contract.

**Response types** are what your extension returns in `ActionResult.Data`. The caller receives this when polling the proxy.

**State type** represents your extension's observable state, returned by `GET /state`. The TEE infrastructure uses this for state synchronization.

The scaffold defines:

```go
// What the user sends for a SAY_HELLO command
type SayHelloRequest struct {
    Name string `json:"name"`
}

// What your extension returns for SAY_HELLO
type SayHelloResponse struct {
    Greeting       string `json:"greeting"`
    GreetingNumber int    `json:"greetingNumber"`
}

// What the user sends for a SAY_GOODBYE command (ABI-encoded, not JSON)
type SayGoodbyeRequest struct {
    Name   string `json:"name"`
    Reason string `json:"reason"`
}

// What your extension returns for SAY_GOODBYE
type SayGoodbyeResponse struct {
    Farewell       string `json:"farewell"`
    FarewellNumber int    `json:"farewellNumber"`
}

// Your extension's cumulative state
type State struct {
    GreetingCount int    `json:"greetingCount"`
    LastGreeting  string `json:"lastGreeting"`
    FarewellCount int    `json:"farewellCount"`
    LastFarewell  string `json:"lastFarewell"`
}
```

### 3. `go/internal/extension/extension.go` — Action Handlers

This is the main file. It contains:

- **Extension struct** — your in-memory state fields
- **processAction()** — routes incoming actions to handlers based on OPType
- **Your handler functions** — the actual business logic

#### The Extension Struct

Add fields to hold your extension's state. Always protect state access with the `mu` mutex. The scaffold tracks greeting count and the last greeting sent:

```go
type Extension struct {
    mu     sync.RWMutex
    Server *http.Server

    greetingCount int
    lastGreeting  string
    farewellCount int
    lastFarewell  string
}
```

#### processAction() — The Router

This function receives every action and routes it. Add a `case` for each operation type. Within a type, a sub-router (e.g. `processGreeting`) dispatches on `OPCommand`:

```go
func (e *Extension) processAction(action teetypes.Action) (int, []byte) {
    dataFixed, err := processorutils.Parse[instruction.DataFixed](action.Data.Message)
    if err != nil {
        return http.StatusBadRequest, []byte(fmt.Sprintf("decoding fixed data: %v", err))
    }

    switch {
    case dataFixed.OPType == teeutils.ToHash(config.OPTypeGreeting):
        ar := e.processGreeting(action, dataFixed)
        b, _ := json.Marshal(ar)
        return http.StatusOK, b

    default:
        return http.StatusNotImplemented, []byte("unsupported op type")
    }
}

func (e *Extension) processGreeting(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
    switch {
    case df.OPCommand == teeutils.ToHash(config.OPCommandSayHello):
        return e.processSayHello(action, df)
    case df.OPCommand == teeutils.ToHash(config.OPCommandSayGoodbye):
        return e.processSayGoodbye(action, df)
    default:
        return buildResult(action, df, nil, 0, fmt.Errorf("unsupported op command"))
    }
}
```

#### Handler Functions

Each handler follows the same 4-step pattern. Here's the scaffold's `processSayHello` (a command under the `GREETING` operation type):

```go
func (e *Extension) processSayHello(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
    // 1. Decode the incoming message
    var req types.SayHelloRequest
    dec := json.NewDecoder(bytes.NewReader(df.OriginalMessage))
    dec.DisallowUnknownFields()
    err := dec.Decode(&req)
    if err != nil {
        return buildResult(action, df, nil, 0, fmt.Errorf("decoding request: %w", err))
    }

    // 2. Validate
    if req.Name == "" {
        return buildResult(action, df, nil, 0, fmt.Errorf("name must not be empty"))
    }

    // 3. Execute business logic
    e.mu.Lock()
    e.greetingCount++
    greetingNumber := e.greetingCount
    greeting := fmt.Sprintf("Hello, %s! Welcome to Flare Confidential Compute.", req.Name)
    e.lastGreeting = greeting
    e.mu.Unlock()

    // 4. Build response
    resp := types.SayHelloResponse{
        Greeting:       greeting,
        GreetingNumber: greetingNumber,
    }
    data, _ := json.Marshal(resp)

    return buildResult(action, df, data, 1, nil)
}
```

The `processSayGoodbye` handler follows the same 4-step pattern. It decodes its request using ABI decoding via `structs.DecodeTo` rather than plain JSON:

```go
func (e *Extension) processSayGoodbye(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
    // 1. Decode the incoming message (ABI-encoded)
    var req types.SayGoodbyeRequest
    if err := structs.DecodeTo(types.SayGoodbyeMessageArg, df.OriginalMessage, &req); err != nil {
        return buildResult(action, df, nil, 0, fmt.Errorf("decoding request: %w", err))
    }

    // 2. Validate
    if req.Name == "" {
        return buildResult(action, df, nil, 0, fmt.Errorf("name must not be empty"))
    }

    // 3. Execute business logic
    e.mu.Lock()
    e.farewellCount++
    farewellNumber := e.farewellCount
    farewell := fmt.Sprintf("Goodbye, %s! Reason: %s", req.Name, req.Reason)
    e.lastFarewell = farewell
    e.mu.Unlock()

    // 4. Build response
    resp := types.SayGoodbyeResponse{
        Farewell:       farewell,
        FarewellNumber: farewellNumber,
    }
    data, _ := json.Marshal(resp)

    return buildResult(action, df, data, 1, nil)
}
```

**`buildResult` parameters:**
- `status = 0` → error. The `err` parameter is logged.
- `status = 1` → success. The `data` parameter is returned to the caller.

### 4. `contracts/InstructionSender.sol` — On-Chain Entry Point

This contract is the only address allowed to submit instructions to your extension. You define `bytes32` constants for each operation type (matching your Go constants) and add one send function per operation type. After modifying, run `./scripts/generate-bindings.sh` to regenerate Go bindings.

See the **[InstructionSender Contract Guide](instruction-sender.md)** for a full walkthrough and examples.

### 5. `tools/cmd/run-test/main.go` — E2E Tests

The test runner sends instructions through the full pipeline (contract → TEE → proxy) and verifies results. You define test payloads, send them, and assert on your response fields.

See the **[Testing Guide](testing.md)** for details on writing and running tests.

## How the Pieces Connect

The critical link between your Solidity contract and Go code is the **OPType + OPCommand pair**. Both must be identical in three places each:

| What | Solidity | Go config | Go router |
|------|----------|-----------|-----------|
| Operation type | `OP_TYPE_GREETING = bytes32("GREETING")` | `OPTypeGreeting = "GREETING"` | `dataFixed.OPType == teeutils.ToHash(config.OPTypeGreeting)` |
| Hello command | `OP_COMMAND_SAY_HELLO = bytes32("SAY_HELLO")` | `OPCommandSayHello = "SAY_HELLO"` | `df.OPCommand == teeutils.ToHash(config.OPCommandSayHello)` |
| Goodbye command | `OP_COMMAND_SAY_GOODBYE = bytes32("SAY_GOODBYE")` | `OPCommandSayGoodbye = "SAY_GOODBYE"` | `df.OPCommand == teeutils.ToHash(config.OPCommandSayGoodbye)` |

If the OPType doesn't match, the action falls through to `default` in `processAction` and returns "unsupported op type". If the OPCommand doesn't match, the action falls through to `default` in the sub-router (e.g. `processGreeting`) and returns "unsupported op command".

## Data Flow Through the Extension

```
Solidity contract
    │
    │  _message (raw bytes, JSON or ABI-encoded)
    ▼
TeeExtensionRegistry.sendInstructions()
    │
    │  wraps into DataFixed{OPType, OPCommand, OriginalMessage}
    ▼
TEE node → POST /action → actionHandler()
    │
    │  decodes teetypes.Action from request body
    ▼
processAction()
    │
    │  parses DataFixed from action.Data.Message
    │  routes based on dataFixed.OPType
    │
    ├─ OPType == GREETING ──▶ processGreeting()
    │                              │
    │                              │  routes based on df.OPCommand
    │                              │
    │                              ├─ OPCommand == SAY_HELLO ──▶ processSayHello()
    │                              │      decodes SayHelloRequest (JSON)
    │                              │      increments greetingCount
    │                              │      returns SayHelloResponse
    │                              │
    │                              └─ OPCommand == SAY_GOODBYE ─▶ processSayGoodbye()
    │                                     decodes SayGoodbyeRequest (ABI via structs.DecodeTo)
    │                                     increments farewellCount
    │                                     returns SayGoodbyeResponse
    │
    └─ default ──▶ "unsupported op type"
    ▼
buildResult() → JSON response → TEE node → proxy → caller
```

Key types in the flow:
- `teetypes.Action` — the envelope from the TEE node (contains `Data.Message`, `Data.ID`, etc.)
- `instruction.DataFixed` — parsed from `Action.Data.Message` (contains `OPType`, `OPCommand`, `OriginalMessage`)
- `df.OriginalMessage` — the raw `_message` bytes from the Solidity contract (your JSON payload)
- `teetypes.ActionResult` — what you return (contains `Status`, `Data`, `Log`)

## Using the TEE Signing Port

Extensions can request the TEE to sign data or perform cryptographic operations through the sign port (default: 9090, configurable via `SIGN_PORT`). This is useful for extensions that need to:

- Sign transactions on behalf of the TEE
- Generate verifiable attestations
- Encrypt/decrypt data using TEE-managed keys

The sign port is available at `localhost:{SIGN_PORT}` from within the extension.

**The sign port speaks base64, not hex.** tee-node is Go, and Go marshals `[]byte` as base64 in JSON — so `/decrypt` takes `{"encryptedMessage": "<base64>"}` and returns `{"decryptedMessage": "<base64>"}`, unlike the hex used everywhere else on the wire. Python and TypeScript wrap this in `base/node.py` and `src/base/node.ts` respectively, so prefer `NodeClient` over hand-rolling the call. See [extension-contract.md §3](extension-contract.md).

## Step-by-Step: Adding a New Operation

Steps 1–6 are language-specific; the rest are shared. Substitute your language's files using the map above, or let the `/create-extension` skill dispatch for you.

1. **Add constants** — one `OPType` for the operation group, one `OPCommand` per command
2. **Define request/response shapes** — a pair per command, plus any new state fields
3. **Route the operation** — Go adds a `case` in `processAction()`; Python and TypeScript call `framework.handle(opType, opCommand, handler)` in `register()`
4. **Write the sub-router** (Go only) — switch on `OPCommand` and dispatch to individual handlers. Python and TypeScript get this from the registry
5. **Write each handler** following the 4-step pattern: decode → validate → execute → respond. For ABI-encoded messages use `structs.DecodeTo` (Go), `eth_abi.decode` (Python), or `decodeAbiParameters` (TypeScript); for JSON use the language's decoder
6. **Expose new state** — Go adds fields to the `Extension` struct and maps them in `stateHandler()`; Python and TypeScript extend `report_state()` / `reportState()`. Keep the JSON keys identical across languages
7. **Add the Solidity constants and send function** in `contracts/InstructionSender.sol`
8. **Regenerate bindings**: `./scripts/generate-bindings.sh` — required in every language, since the deployment tooling is Go
9. **Update the Go tooling** — in `tools/pkg/utils/instructions.go`, update the import path from `helloworld` to your package, rename type references (e.g. `helloworld.DeployHelloWorldInstructionSender` → `orderbook.DeployOrderbookInstructionSender`), and rename the send function call (e.g. `sender.SendSayHello` → `sender.SendPlaceOrder`) to match your new Solidity function name
10. **Add a test case** in `tools/cmd/run-test/main.go`
11. **Regenerate the conformance fixtures** — `./python/.venv/bin/python testdata/conformance/gen_fixtures.py` — then `./scripts/test-conformance.sh --all`

> If you maintain more than one language, apply steps 1–6 to each. `./scripts/test-conformance.sh --all` is what catches a language you forgot: the implementations must stay byte-identical on the wire.

## Common Patterns

### Returning errors to the caller

Use `status = 0` in `buildResult`. The error message goes into `ActionResult.Log`:

```go
if req.Name == "" {
    return buildResult(action, df, nil, 0, fmt.Errorf("name must not be empty"))
}
```

### Maintaining state across actions

Add fields to the `Extension` struct and protect with the mutex:

```go
e.mu.Lock()
e.greetingCount++
e.lastGreeting = greeting
e.mu.Unlock()
```

Return state in `stateHandler()` via the `types.State` struct.

### Multiple operation types

Each operation type gets its own handler function. They share the same `Extension` struct, so they can read/write the same state.
