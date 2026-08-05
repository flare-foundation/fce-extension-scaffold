# Reproducible Builds

The image's code hash is what gets registered on-chain, so build determinism is
a security property rather than a nicety.

## Reproducibility is not equal across languages

Be precise about what is actually guaranteed. Overclaiming here is worse than
underclaiming, because the on-chain registration is what depends on it.

| Language | Guarantee | Why |
| --- | --- | --- |
| **Go** | **Bit-for-bit across machines** | Static `CGO_ENABLED=0` binary with `-trimpath -buildid=`, on a digest-pinned distroless base. Nothing host-specific survives. |
| **Python** | **Same-machine only** | pip installs prebuilt manylinux wheels whose contents are fixed, but `.dist-info` metadata and installation layout can vary with the pip/setuptools version present in the base image. |
| **TypeScript** | **Same-machine only** | `npm ci` reproduces the dependency *tree* from `package-lock.json`, but `node_modules` layout, hoisting and file ordering vary across npm versions. |

For Python and TypeScript, "same-machine" means: rebuilding on the same host
with the same Docker/pip/npm versions and the same `SOURCE_DATE_EPOCH` yields
the same digest. It does **not** mean an auditor on different hardware can
independently reproduce your hash.

If independent third-party verification of the code hash matters for your
deployment, use the Go path. To tighten Python/TypeScript, pin the runtime base
images by `sha256` digest (both currently use tag form, marked with a `NOTE:` in
their Dockerfiles) — this is required before cutting a testnet release.

All three share the same tee-node build: `docker/node-base.Dockerfile` compiles
it once from a pinned ref on the same digest-pinned golang image, so the
`server` binary bytes are identical across language images.
`scripts/check-versions.sh` fails the build if that pin drifts from `go/go.mod`.

## How it works

- `SOURCE_DATE_EPOCH` is set to the commit timestamp and passed as a build arg
  to clamp all timestamps
- Go binary is built with `-trimpath -ldflags="-buildid= -s -w"` and
  `-buildvcs=false` to strip non-deterministic metadata; `CGO_ENABLED=0`
  produces a static binary so link-time libc variance cannot leak in
- Base image digest is pinned in the Dockerfile
- Debian package versions are pinned via apt's native snapshot support
  (Debian 13+): `Snapshot: true` in the sources file plus
  `apt-get install --snapshot <SOURCE_DATE_EPOCH>` redirects every fetch to
  [snapshot.debian.org](https://snapshot.debian.org) at the exact instant of
  the commit, so the same `SOURCE_DATE_EPOCH` always yields the same package
  bytes. Adapted from
  [reproducible-containers/repro-sources-list.sh](https://github.com/reproducible-containers/repro-sources-list.sh/blob/master/alternative/Dockerfile.debian-13)
- CI uses BuildKit's [`rewrite-timestamp=true`](https://github.com/moby/buildkit/pull/4057)
  exporter option to normalize layer timestamps

## Build context

The default build is self-contained: the build context is the repo root
(`docker-compose.yaml` sets `context: .`, `dockerfile: ${EXTENSION_DOCKERFILE}`,
resolved from `LANGUAGE`). `go/go.mod` pins
`github.com/flare-foundation/tee-node` to a released version and fetches it from
the network (verified against `go.sum`), so the build needs only this repo's own
sources — no sibling `tee-node/` checkout.

Each language ships a `<lang>/Dockerfile.dockerignore`. BuildKit prefers those
over the root `.dockerignore`, and each one excludes the *other* language
directories along with `node_modules/`, `__pycache__/` and `.venv/`. This is not
only a build-speed concern: anything reachable in the context can perturb layer
hashes, so a stray local `node_modules` would otherwise undermine determinism.

> **Developing `tee-node`/`tee-proxy` locally?** Run
> `USE_LOCAL_SIBLINGS=1 ./scripts/start-services.sh`, which builds from on-disk
> sibling checkouts via `go/Dockerfile.siblings` (build context `tee/`). That
> path is Go-only and is for local iteration — it uses whatever is checked out
> and is **not** reproducible. `start-services.sh` rejects it for other
> languages, which build tee-node from the pinned ref instead.

## Verifying a remote image

The default Docker builder does not properly support `rewrite-timestamp`
([moby/buildkit#4230](https://github.com/moby/buildkit/issues/4230)). You need
a BuildKit builder using the `docker-container` driver.

Create the builder (one-time setup):

```sh
docker buildx create --driver=docker-container --name=moby-buildkit --driver-opt image=moby/buildkit --bootstrap
```

Clone the extension repository (self-contained — no sibling `tee-node/` needed;
the pinned module is fetched from the network at build time):

```sh
git clone https://github.com/flare-foundation/fce-extension-scaffold.git
cd fce-extension-scaffold
```

Checkout the tag you want to verify, build locally, and compare the image ID
against the registry image. The Dockerfile lives under the language directory:

```sh
TAG=$(git describe --tags --abbrev=0)
git checkout "$TAG"

docker buildx build --builder moby-buildkit --platform linux/amd64 --no-cache --build-arg SOURCE_DATE_EPOCH=$(git log -1 --format=%ct) --output "type=docker,rewrite-timestamp=true" -t local/extension-scaffold:verify --load -f go/Dockerfile .

docker pull --platform linux/amd64 ghcr.io/flare-foundation/extension-scaffold:"$TAG"

docker inspect --format='{{.Id}}' local/extension-scaffold:verify
docker inspect --format='{{.Id}}' ghcr.io/flare-foundation/extension-scaffold:"$TAG"
```

Both IDs should be identical.

### Verifying the Python or TypeScript images

`python/Dockerfile` and `typescript/Dockerfile` begin
`FROM local/tee-node-base:${TEE_NODE_REF}`. That image is produced by
`scripts/build-node-base.sh`, which uses plain `docker build` and therefore
writes it to the **host** image store. The `docker-container` driver required
above keeps its own isolated store and cannot see it, so the build fails to
resolve the base image.

Publish the base image to a throwaway local registry and point the build at it
with `--build-context`:

```sh
docker run -d --rm -p 5000:5000 --name repro-registry registry:2

./scripts/build-node-base.sh
docker tag  local/tee-node-base:"$TEE_NODE_REF" localhost:5000/tee-node-base:"$TEE_NODE_REF"
docker push localhost:5000/tee-node-base:"$TEE_NODE_REF"

docker buildx build --builder moby-buildkit --platform linux/amd64 --no-cache   --build-arg SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)   --build-context local/tee-node-base:"$TEE_NODE_REF"=docker-image://localhost:5000/tee-node-base:"$TEE_NODE_REF"   --output "type=docker,rewrite-timestamp=true"   -t local/extension-scaffold:verify --load -f python/Dockerfile .

docker stop repro-registry
```

Per the table above, Python and TypeScript remain same-machine only: a matching
digest here confirms your own build is deterministic, not that an auditor on
other hardware will reproduce the published hash.

## Upstream references

- [moby/buildkit#3180](https://github.com/moby/buildkit/issues/3180) -
  `rewrite-timestamp` only clamps timestamps *down* to `SOURCE_DATE_EPOCH`,
  older timestamps are left unchanged. The Dockerfile works around this with
  an explicit `find + touch` to normalize all timestamps before COPY.
- [moby/buildkit#4057](https://github.com/moby/buildkit/pull/4057) - PR that
  added `rewrite-timestamp` support to BuildKit exporters
- [moby/buildkit#4230](https://github.com/moby/buildkit/issues/4230) - open
  issue tracking `rewrite-timestamp` incompatibility with the default Docker
  builder and `--load` (`unpack` conflict)
