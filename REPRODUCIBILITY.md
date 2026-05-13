# Reproducible Builds

This extension produces reproducible Docker images. Given the same source code,
builds produce bit-for-bit identical image layers regardless of when or where
they are built.

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

The build context must be the parent `tee/` directory, not the
`extension-scaffold/` directory itself. This is because `go.mod` declares
`replace github.com/flare-foundation/tee-node => ../../tee-node`, so the
builder needs both `tee-node/` and `extension-examples/extension-scaffold/`
visible side-by-side.

## Verifying a remote image

The default Docker builder does not properly support `rewrite-timestamp`
([moby/buildkit#4230](https://github.com/moby/buildkit/issues/4230)). You need
a BuildKit builder using the `docker-container` driver.

Create the builder (one-time setup):

```sh
docker buildx create --driver=docker-container --name=moby-buildkit --driver-opt image=moby/buildkit --bootstrap
```

Clone the repositories so `tee-node/` and `extension-examples/` sit next to
each other under a shared parent (matches the layout the Dockerfile expects):

```sh
mkdir tee && cd tee
git clone https://github.com/flare-foundation/tee-node.git
git clone https://github.com/flare-foundation/extension-examples.git
```

Checkout the tag you want to verify, build locally, and compare the image ID
against the registry image. Run from `tee/extension-examples/extension-scaffold/`:

```sh
TAG=$(git describe --tags --abbrev=0)
git checkout "$TAG"

docker buildx build --builder moby-buildkit --platform linux/amd64 --no-cache --build-arg SOURCE_DATE_EPOCH=$(git log -1 --format=%ct) --output "type=docker,rewrite-timestamp=true" -t local/extension-scaffold:verify --load -f Dockerfile ../..

docker pull --platform linux/amd64 ghcr.io/flare-foundation/extension-scaffold:"$TAG"

docker inspect --format='{{.Id}}' local/extension-scaffold:verify
docker inspect --format='{{.Id}}' ghcr.io/flare-foundation/extension-scaffold:"$TAG"
```

Both IDs should be identical.

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
