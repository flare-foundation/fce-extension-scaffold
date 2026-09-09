# Working in Multiple Languages

The scaffold ships the same Hello World extension in **Go, Python and TypeScript**, and is built so you can add a fourth language without touching any shared code.

This is the practical guide. For the normative specification an implementation must satisfy, see [extension-contract.md](extension-contract.md).

---

## The mental model

The repo is one **language-neutral spine** plus **interchangeable language implementations**:

```
        ┌─────────────────────── shared, never language-specific ───────────────────────┐
        │  contracts/     scripts/     tools/     config/     docker-compose.yaml       │
        └───────────────────────────────────┬───────────────────────────────────────────┘
                                            │  LANGUAGE=<dir> in .env
                    ┌───────────────────────┼───────────────────────┐
                    ▼                       ▼                       ▼
               ┌─────────┐            ┌──────────┐           ┌──────────────┐
               │   go/   │            │ python/  │           │ typescript/  │
               └─────────┘            └──────────┘           └──────────────┘
```

Everything on the left of the switch — your Solidity contract, the deployment tooling, registration, the on-chain test, the compose stack — behaves identically no matter which implementation you pick. Only the container built for `extension-tee` changes.

**Discovery is by convention, not by a hardcoded list.** A directory is an extension implementation if and only if it contains a `language.env` manifest. No script in this repo enumerates languages, which is what makes adding one a local change.

---

## Switching language

Set `LANGUAGE` in `.env`:

```bash
LANGUAGE=go
```

Then run the normal flow. Nothing else changes:

```bash
./scripts/full-setup.sh --test
```

To see what is available:

```bash
ls -d */language.env | cut -d/ -f1
```

An invalid value fails fast and lists the real options rather than guessing.

---

## Choosing a language

| | Go | Python | TypeScript |
|---|---|---|---|
| Image size | **~22 MB** | ~268 MB | ~472 MB |
| Base | distroless (no shell) | debian-slim | debian-slim |
| Process model | single process | two processes | two processes |
| Reproducibility | **bit-for-bit, cross-machine** | same-machine | same-machine |
| Extra toolchain needed | none | Python 3.11+ | Node 22+, npm |
| ABI decoding | `go-ethereum/accounts/abi` | `eth-abi` | `viem` |

**Pick Go if** the on-chain code hash needs to be independently verifiable by a third party. It is the only path where someone on different hardware can rebuild your image and get an identical digest. It is also the only one that can embed tee-node as a library, hence the single process and the tiny distroless image.

**Pick Python or TypeScript if** developer velocity or an existing library ecosystem matters more, and same-machine reproducibility is enough. Both are fully supported: identical behaviour, identical wire format, the same deployment path, and the same conformance guarantees. See [REPRODUCIBILITY.md](../REPRODUCIBILITY.md) for exactly what "same-machine" does and does not promise.

There is no functional penalty to the non-Go paths. The difference is image size and the strength of the reproducibility claim.

---

## Anatomy of a language directory

Every implementation splits the same way:

| Layer | Purpose | Touch it? |
|---|---|---|
| **framework** | HTTP server, wire types, hex/bytes32 encoding, dispatch registry, tee-node client | **No** — infrastructure |
| **app** | Your op constants, handlers, state, payload decoding | **Yes** — this is your extension |

Concretely:

```
go/                                  python/                    typescript/
├── internal/extension/utils.go      ├── base/server.py         ├── src/base/server.ts
├── pkg/server/          framework   ├── base/types.py          ├── src/base/types.ts
│                                    ├── base/encoding.py       ├── src/base/encoding.ts
│                                    ├── base/node.py           ├── src/base/node.ts
│
├── internal/config/config.go        ├── app/config.py          ├── src/app/config.ts
├── internal/extension/extension.go  ├── app/handlers.py        ├── src/app/handlers.ts
├── pkg/types/types.go       app     ├── app/abi.py             ├── src/app/abi.ts
```

Go's split is expressed through package boundaries and `DO NOT MODIFY` comments rather than a `base/` directory, but the division is the same.

---

## The same handler, three ways

`GREETING`/`SAY_HELLO` — decode a JSON payload, validate, update state, respond. Every handler in every language follows this 4-step shape.

**Go** — `go/internal/extension/extension.go`

```go
func (e *Extension) processSayHello(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
	var req types.SayHelloRequest
	dec := json.NewDecoder(bytes.NewReader(df.OriginalMessage))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding request: %w", err))
	}
	if req.Name == "" {
		return buildResult(action, df, nil, 0, fmt.Errorf("name must not be empty"))
	}

	e.mu.Lock()                       // Go handlers must lock; the others are serialized for you
	e.greetingCount++
	greeting := fmt.Sprintf("Hello, %s! Welcome to Flare Confidential Compute.", req.Name)
	e.lastGreeting = greeting
	n := e.greetingCount
	e.mu.Unlock()

	data, _ := json.Marshal(types.SayHelloResponse{Greeting: greeting, GreetingNumber: n})
	return buildResult(action, df, data, 1, nil)
}
```

**Python** — `python/app/handlers.py`

```python
def handle_say_hello(msg: str) -> tuple[Optional[str], int, Optional[str]]:
    global _greeting_count, _last_greeting

    try:
        req = json.loads(hex_to_bytes(msg))
    except (json.JSONDecodeError, ValueError) as e:
        return None, 0, f"decoding request: {e}"

    name = req.get("name", "")
    if not name:
        return None, 0, "name must not be empty"

    _greeting_count += 1
    greeting = f"Hello, {name}! Welcome to Flare Confidential Compute."
    _last_greeting = greeting

    resp = {"greeting": greeting, "greetingNumber": _greeting_count}
    return bytes_to_hex(json.dumps(resp, separators=(",", ":")).encode("utf-8")), 1, None
```

**TypeScript** — `typescript/src/app/handlers.ts`

```typescript
export function handleSayHello(msg: string): HandlerResult {
  let req: unknown;
  try {
    req = JSON.parse(Buffer.from(hexToBytes(msg)).toString("utf-8"));
  } catch (e) {
    return [null, 0, `decoding request: ${String(e)}`];
  }

  const name = (req as { name?: unknown }).name;
  if (typeof name !== "string" || name === "") {
    return [null, 0, "name must not be empty"];
  }

  greetingCount++;
  const greeting = `Hello, ${name}! Welcome to Flare Confidential Compute.`;
  lastGreeting = greeting;

  const resp = { greeting, greetingNumber: greetingCount };
  return [bytesToHex(Buffer.from(JSON.stringify(resp), "utf-8")), 1, null];
}
```

### Three differences worth internalising

**Locking.** Go's `Extension` is a struct whose handlers you must guard with `e.mu`. Python and TypeScript serialize handler invocations inside the framework, so plain module-level state is safe. Never add your own lock there — you would deadlock against the framework's.

**Compact JSON is mandatory.** Python needs `separators=(",", ":")`; Go's `encoding/json` and JS's `JSON.stringify` are already compact. tee-node hashes the response `data` and signs it, and the conformance fixtures compare the resulting hex byte-for-byte across all three languages. A stray space is a real failure.

**Declare response fields in the same order everywhere.** JSON object key order follows declaration order in all three, and it is part of the bytes being hashed.

---

## Working in each language

All commands run from the repo root.

### Go

```bash
cd go && go build ./... && go test ./...
```

Needs nothing beyond the Go toolchain. `go/cmd/main.go` is the standalone extension server; `go/cmd/docker/main.go` is the combined entry point the image uses.

### Python

```bash
./scripts/test-unit.sh python
```

Creates `python/.venv` and installs `requirements-dev.txt` on first run. To work in it directly:

```bash
cd python && python3 -m venv .venv && ./.venv/bin/pip install -r requirements-dev.txt
```

Runtime dependencies live in `requirements.txt` with exact pins; only that file is installed into the image. Every pin must ship prebuilt manylinux wheels — the Dockerfile installs without a build toolchain, so anything needing compilation fails the build.

### TypeScript

```bash
./scripts/test-unit.sh typescript
```

Runs `npm ci` then `vitest`. To work in it directly:

```bash
cd typescript && npm ci && npm run build && npm test
```

Add dependencies with an exact version (no `^`), then run `npm install --package-lock-only` and commit `package-lock.json` — the lockfile is the reproducibility anchor for `npm ci` in the Dockerfile.

Relative imports need a `.js` extension even from `.ts` source; the project is ESM with `moduleResolution: NodeNext`.

---

## Testing across languages

| Command | Scope | Needs a chain? |
|---|---|---|
| `./scripts/test-unit.sh [lang\|--all]` | that implementation's own tests | no |
| `./scripts/test-conformance.sh [lang\|--all]` | the wire contract, against golden fixtures | no |
| `./scripts/test.sh` | full on-chain round trip | yes |

Conformance is the one that keeps the implementations honest. It boots just the extension process, replays the 16 fixtures in `testdata/conformance/`, and diffs every response field — including a byte-exact comparison of the response payload. Run it across all languages before you commit a handler change:

```bash
./scripts/test-conformance.sh --all
```

If you change a request or response shape, regenerate the fixtures rather than hand-editing them:

```bash
./python/.venv/bin/python testdata/conformance/gen_fixtures.py
```

The fixtures are order-dependent and share one process — counters accumulate and the final fixture asserts the resulting state.

---

## Adding your own language

Nothing in `scripts/`, `tools/`, `contracts/` or `docker-compose.yaml` needs to change. You create one directory.

**Read [extension-contract.md](extension-contract.md) first** — it is the specification you are implementing, and it documents the traps.

### 1. Create the manifest

`<language>/language.env` is what makes the directory discoverable. All commands run with the language directory as the working directory.

```sh
LANGUAGE_NAME="Rust"
LANGUAGE_DOCKERFILE="Dockerfile"
LANGUAGE_SETUP_CMD=""                          # install deps; empty if implicit
LANGUAGE_BUILD_CMD="cargo build --release"
LANGUAGE_TEST_CMD="cargo test"
LANGUAGE_RUN_CMD="./target/release/extension"  # foreground server, reads EXTENSION_PORT
```

### 2. Port the framework layer

Use `python/base/` or `typescript/src/base/` as the reference. You need:

- hex ⟷ bytes, and bytes32 padding for op identifiers
- the wire types from contract §4
- a `(opType, opCommand)` registry: exact match first, then the empty-opCommand wildcard
- serialization so handlers and state reads never overlap
- an HTTP server implementing `POST /action`, `GET /state`, and the 405/404/501 rules
- a client for the `$SIGN_PORT` API

**The three things ports get wrong most often:**

1. `ActionResult.version` is a **plain string**, while `StateResponse.stateVersion` is **bytes32**. The asymmetry is real.
2. Every `ActionResult` field is always present. `data` and `additionalResultStatus` are `"0x"` when empty, never omitted.
3. The `$SIGN_PORT` API is **base64**, not hex — Go marshals `[]byte` that way.

### 3. Port the app layer

Implement `GREETING`/`SAY_HELLO` (JSON) and `GREETING`/`SAY_GOODBYE` (ABI-encoded `(string,string)`), matching the other languages exactly — same greeting strings, same counters, same state keys. The conformance fixtures and `tools/cmd/run-test` both assert on the literal values.

### 4. Write the Dockerfile

Unless your language can link tee-node as a Go library, use the two-process shape and start from `python/Dockerfile`:

```dockerfile
ARG TEE_NODE_REF
FROM local/tee-node-base:${TEE_NODE_REF} AS node
```

The shared base image (`docker/node-base.Dockerfile`) supplies the tee-node `server` binary and the Confidential Space root cert, so you skip the golang stage and the tee-node fetch entirely. `start-services.sh` builds it automatically for any Dockerfile that references `tee-node-base`.

Then satisfy contract §6 exactly — no `EXPOSE`, same `MODE`/port env, `USER 0:0`, and an **identical** `tee.launch_policy.allow_env_override` label. A label mismatch means Confidential Space silently rejects operator env overrides at attestation time, and you find out at deployment rather than at build.

Add `<language>/Dockerfile.dockerignore` excluding the other language directories and local build artifacts; anything reachable in the build context can perturb layer hashes.

### 5. Prove it

```bash
./scripts/test-conformance.sh <language>
```

16 fixtures, seconds, no chain and no Docker. This is the real gate — it catches wire-format divergence that unit tests miss.

Then the full path:

```bash
LANGUAGE=<language> ./scripts/full-setup.sh --test
```

If `test.sh` passes unmodified, your language is a first-class citizen.

### 6. Document it honestly

Add a row to the comparison table above and a section to [REPRODUCIBILITY.md](../REPRODUCIBILITY.md). Bit-for-bit cross-machine reproducibility needs a static binary and a digest-pinned base; most package-manager-driven runtimes manage same-machine determinism only. The image's code hash is registered on-chain, so overclaiming has real consequences.

---

## Known limitations

**`--local` mode is Go-only.** `./scripts/start-services.sh --local` builds and runs Go binaries in-process, bypassing Docker. Other languages must use Docker Compose mode. The script rejects the combination with a clear message.

**`USE_LOCAL_SIBLINGS=1` is Go-only.** It compiles an on-disk tee-node checkout into the Go binary. Other languages build tee-node from the pinned ref in `go/go.mod`; to test a local tee-node there, push it and update the pin.

**One language is built at a time.** `LANGUAGE` selects a single implementation for the `extension-tee` container. To compare, run them in sequence.
