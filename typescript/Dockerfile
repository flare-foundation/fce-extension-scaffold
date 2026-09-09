# TypeScript extension image — two-process topology (docs/extension-contract.md §1).
#
# Build context is the repo root. The tee-node `server` binary comes from the
# shared base image built by docker/node-base.Dockerfile, which is why this file
# is short: the golang stage, apt snapshot pinning and tee-node fetch all live
# there and are shared by every non-Go language.
#
# scripts/start-services.sh builds the base image automatically. To build by hand:
#   ./scripts/build-node-base.sh && docker build -f typescript/Dockerfile -t ext-typescript .

# TEE_NODE_REF must match the Go module pin — scripts/lib/versions.sh derives it
# and scripts/check-versions.sh enforces it. Declared before FROM so it can be
# interpolated into the base image tag.
ARG TEE_NODE_REF
FROM local/tee-node-base:${TEE_NODE_REF} AS node

# --- Compile the extension ---------------------------------------------------
# NOTE: pin by digest before cutting a testnet release — see REPRODUCIBILITY.md.
FROM node:22-trixie AS ext-builder

WORKDIR /ext

# npm ci installs exactly the tree the lockfile declares; package-lock.json is
# the reproducibility anchor here. Copied separately so the install layer is
# cached independently of source edits.
COPY --chmod=644 --chown=0:0 typescript/package.json typescript/package-lock.json ./
RUN npm ci

COPY --chmod=644 --chown=0:0 typescript/tsconfig.json ./
COPY --chmod=644 --chown=0:0 typescript/src/ ./src/
RUN npm run build

# Drop devDependencies so the runtime image carries only what main.js needs.
RUN npm prune --omit=dev

# --- Runtime -----------------------------------------------------------------
FROM debian:trixie-slim AS runtime

# ARG, not ENV: this is a build-time input. Promoting it to ENV would leak a
# build parameter into the running container's environment.
ARG SOURCE_DATE_EPOCH

WORKDIR /app

# Redirect apt at snapshot.debian.org keyed on SOURCE_DATE_EPOCH so nodejs and
# ca-certificates resolve to the same package set on every rebuild.
# Adapted from https://github.com/reproducible-containers/repro-sources-list.sh/blob/master/alternative/Dockerfile.debian-13
RUN \
  --mount=type=cache,target=/var/cache/apt,sharing=locked \
  --mount=type=cache,target=/var/lib/apt,sharing=locked \
  if [ "${SOURCE_DATE_EPOCH:-0}" -gt 0 ] 2>/dev/null; then \
    snapshot="$(/bin/bash -euc "printf \"%(%Y%m%dT%H%M%SZ)T\n\" \"${SOURCE_DATE_EPOCH}\"")" && \
    sed -i -e '/Types: deb/ a\Snapshot: true' /etc/apt/sources.list.d/debian.sources && \
    rm -f /etc/apt/apt.conf.d/docker-clean && \
    echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' >/etc/apt/apt.conf.d/keep-cache && \
    apt-get install --update --snapshot "${snapshot}" -o Acquire::Check-Valid-Until=false -o Acquire::https::Verify-Peer=false -y ca-certificates && \
    apt-get install --snapshot "${snapshot}" -y ca-certificates nodejs && \
    rm -rf /var/log/* /var/cache/ldconfig/aux-cache; \
  else \
    apt-get update && apt-get install -y ca-certificates nodejs && \
    rm -rf /var/lib/apt/lists/* /var/log/* /var/cache/ldconfig/aux-cache; \
  fi

# tee-node server binary + Confidential Space root cert from the shared base.
COPY --chmod=755 --chown=0:0 --from=node /app/server /app/server
COPY --chmod=644 --chown=0:0 --from=node /app/assets/google_confidential_space_root.crt /app/assets/google_confidential_space_root.crt

# The compiled extension with its production dependencies.
COPY --chmod=644 --chown=0:0 --from=ext-builder /ext/dist /app/extension/dist
COPY --chmod=644 --chown=0:0 --from=ext-builder /ext/node_modules /app/extension/node_modules
COPY --chmod=644 --chown=0:0 --from=ext-builder /ext/package.json /app/extension/package.json

# Normalize mtimes so layer hashes are stable in both directions — buildkit's
# rewrite-timestamp only clamps downward (moby/buildkit#3180).
RUN if [ "${SOURCE_DATE_EPOCH:-0}" -gt 0 ] 2>/dev/null; then find /app -exec touch -h -d @${SOURCE_DATE_EPOCH} {} +; fi

# Ports and mode match go/Dockerfile so compose works uniformly across languages.
#
# Production mode by default; docker-compose.yaml overrides to MODE=1 for local dev.
# FTDC rejects simulated attestation, so a real deploy MUST stay 0.
ENV MODE=0 CONFIG_PORT=5501 SIGN_PORT=7701 EXTENSION_PORT=7702

# Match tee-node: the TEE workload itself is the isolation boundary.
USER 0:0

# Must match go/Dockerfile's list exactly — a mismatch means Confidential Space
# silently rejects operator env overrides at attestation time.
LABEL "tee.launch_policy.allow_env_override"="LOG_LEVEL,PROXY_URL,INITIAL_OWNER,EXTENSION_ID,CHAIN_URL,MODE"

# Two-process layout: tee-node ./server handles config/sign and forwards actions
# to the extension on EXTENSION_PORT. If either dies the container exits so the
# orchestrator restarts it.
# /bin/bash, not /bin/sh: dash does not implement `wait -n`.
CMD ["/bin/bash", "-c", "/app/server & SERVER_PID=$!; node /app/extension/dist/main.js & EXT_PID=$!; wait -n $SERVER_PID $EXT_PID; kill -TERM $SERVER_PID $EXT_PID 2>/dev/null; exit 1"]
