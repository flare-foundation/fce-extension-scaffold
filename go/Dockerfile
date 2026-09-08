# Default node build: self-contained and reproducible. Build context is this extension dir.
# The extension module pins github.com/flare-foundation/tee-node to a released version in go.mod
# and fetches it from the network (verified against go.sum) — no on-disk sibling repo or replace
# directive is needed, so only the module's own sources are copied into the build.
#
# To build against an on-disk sibling tee-node instead (while developing it), use
# `USE_LOCAL_SIBLINGS=1 ./scripts/start-services.sh`, which selects ./Dockerfile.siblings.

# pin base image by digest so every build starts from the same bytes
FROM golang:1.25.1-trixie@sha256:ff83f3762390c2cccb53618ccc18af23e556aff9b1db4428637e9f63287c8171 AS builder

# commit timestamp, propagated through the build to clamp file mtimes and normalize embedded dates
ARG SOURCE_DATE_EPOCH
ENV SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH

WORKDIR /build

# apt normally resolves to whatever package versions the mirror serves at build time, so two builds days apart install different bytes
# redirect apt at snapshot.debian.org keyed on SOURCE_DATE_EPOCH so every build installs the exact package set that existed at that instant
# NOTE: taken verbatim from https://github.com/reproducible-containers/repro-sources-list.sh/blob/master/alternative/Dockerfile.debian-13
RUN \
  --mount=type=cache,target=/var/cache/apt,sharing=locked \
  --mount=type=cache,target=/var/lib/apt,sharing=locked \
  if [ "${SOURCE_DATE_EPOCH:-0}" -gt 0 ] 2>/dev/null; then \
    : "${SOURCE_DATE_EPOCH:=$(stat --format=%Y /etc/apt/sources.list.d/debian.sources)}" && \
    snapshot="$(/bin/bash -euc "printf \"%(%Y%m%dT%H%M%SZ)T\n\" \"${SOURCE_DATE_EPOCH}\"")" && \
    : "Enabling snapshot" && \
    sed -i -e '/Types: deb/ a\Snapshot: true' /etc/apt/sources.list.d/debian.sources && \
    : "Enabling cache" && \
    rm -f /etc/apt/apt.conf.d/docker-clean && \
    echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' >/etc/apt/apt.conf.d/keep-cache && \
    : "Fetching the snapshot and installing ca-certificates in one command" && \
    apt-get install --update --snapshot "${snapshot}" -o Acquire::Check-Valid-Until=false -o Acquire::https::Verify-Peer=false -y ca-certificates && \
    : "Installing ca-certificates" && \
    apt-get install --snapshot "${snapshot}" -y ca-certificates && \
    : "Clean up for improving reproducibility (optional)" && \
    rm -rf /var/log/* /var/cache/ldconfig/aux-cache; \
  else \
    apt-get update && apt-get install -y ca-certificates && \
    rm -rf /var/log/* /var/cache/ldconfig/aux-cache; \
  fi

# Build context is the repo root (compose builds from `.`), so module paths are
# prefixed with go/ — the language directories are siblings under that root.
#
# Copy only the module manifest first so `go mod download` is cached independently of source edits.
# tee-node and every other dependency are fetched from the network during download.
COPY --chmod=644 --chown=0:0 go/go.mod go/go.sum ./
RUN go mod download
RUN go mod verify

# Copy the module's own source packages (cmd/docker is the node entrypoint).
COPY --chmod=644 --chown=0:0 go/cmd/ ./cmd/
COPY --chmod=644 --chown=0:0 go/internal/ ./internal/
COPY --chmod=644 --chown=0:0 go/pkg/ ./pkg/

# -trimpath strips build-host paths from the binary
# -buildid= clears go's non-deterministic build id
# -s -w drop symbol and dwarf tables, which contain build-time data
# -buildvcs=false omits embedded vcs metadata
# CGO_ENABLED=0 produces a static binary, avoiding link-time libc variance
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOFLAGS="-buildvcs=false" \
    go build -trimpath -ldflags="-buildid= -s -w" -o /app/extension-tee ./cmd/docker

# NOTE: buildkit's rewrite-timestamp only clamps mtimes down to SOURCE_DATE_EPOCH (moby/buildkit#3180)
# files older than SOURCE_DATE_EPOCH are left at their original non-deterministic mtime
# touch every path to SOURCE_DATE_EPOCH explicitly so timestamps are normalized in both directions
RUN if [ "${SOURCE_DATE_EPOCH:-0}" -gt 0 ] 2>/dev/null; then find /app -exec touch -h -d @${SOURCE_DATE_EPOCH} {} +; fi

# empty base image so nothing outside these explicit copies ends up in the final layers
FROM gcr.io/distroless/static

WORKDIR /app

# ca-certificates.crt is the tls trust store for outbound https (e.g. flare rpc, oauth)
# re-apply chmod/chown on each COPY so metadata is pinned here and does not depend on whatever the builder stage left behind
COPY --chmod=644 --chown=65532:65532 --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chmod=755 --chown=65532:65532 --from=builder /app/extension-tee /app/extension-tee

# MODE=1 is SIMULATED attestation (local dev). MODE=0 is production attestation.
# Set to 1 here so a bare `docker run` works against the local devnet; compose
# overrides it via MODE=${MODE:-1}.
#
# For a real Confidential Space deploy this MUST be 0 — FTDC rejects simulated
# attestation. See docs/deployment-steps.md §5. Keep this value identical across
# go/, python/ and typescript/ Dockerfiles.
ENV MODE=1 CONFIG_PORT=5501 SIGN_PORT=7701 EXTENSION_PORT=7702

# match tee-node: run as root (USER 0:0) — the TEE workload itself is the isolation boundary
USER 0:0

# confidential space launch policy label: allow the operator to override these env vars at workload launch
# without this, the confidential space VM rejects overrides at attestation time and the values baked here are final
LABEL "tee.launch_policy.allow_env_override"="LOG_LEVEL,PROXY_URL,INITIAL_OWNER,EXTENSION_ID,CHAIN_URL,MODE"

CMD ["/app/extension-tee"]
