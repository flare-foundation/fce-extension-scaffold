# Python extension image — two-process topology (docs/extension-contract.md §1).
#
# Build context is the repo root. The tee-node `server` binary comes from the
# shared base image built by docker/node-base.Dockerfile, which is why this file
# is ~40 lines instead of ~110: the golang stage, apt snapshot pinning and
# tee-node fetch all live there and are shared by every non-Go language.
#
# scripts/start-services.sh builds the base image automatically. To build by hand:
#   ./scripts/build-node-base.sh && docker build -f python/Dockerfile -t ext-python .

# TEE_NODE_REF must match the Go module pin — scripts/lib/versions.sh derives it
# and scripts/check-versions.sh enforces it. Declared before FROM so it can be
# interpolated into the base image tag.
ARG TEE_NODE_REF
FROM local/tee-node-base:${TEE_NODE_REF} AS node

# --- Runtime -----------------------------------------------------------------
# NOTE: pin this with a sha256 digest before cutting a testnet release. The tag
# form keeps the same-machine reproducibility story honest — see REPRODUCIBILITY.md.
FROM debian:trixie-slim AS runtime

# ARG, not ENV: this is a build-time input. Promoting it to ENV would leak a
# build parameter into the running container's environment.
ARG SOURCE_DATE_EPOCH

WORKDIR /app

# Redirect apt at snapshot.debian.org keyed on SOURCE_DATE_EPOCH so python3/pip
# resolve to the same package set on every rebuild.
# --no-install-recommends drops python3-pip's recommended build toolchain
# (build-essential, python3-dev, ~300MB): every pin in requirements.txt ships a
# prebuilt manylinux wheel, so nothing is compiled here.
# Adapted from https://github.com/reproducible-containers/repro-sources-list.sh/blob/master/alternative/Dockerfile.debian-13
RUN \
  --mount=type=cache,target=/var/cache/apt,sharing=locked \
  --mount=type=cache,target=/var/lib/apt,sharing=locked \
  if [ "${SOURCE_DATE_EPOCH:-0}" -gt 0 ] 2>/dev/null; then \
    snapshot="$(/bin/bash -euc "printf \"%(%Y%m%dT%H%M%SZ)T\n\" \"${SOURCE_DATE_EPOCH}\"")" && \
    sed -i -e '/Types: deb/ a\Snapshot: true' /etc/apt/sources.list.d/debian.sources && \
    rm -f /etc/apt/apt.conf.d/docker-clean && \
    echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' >/etc/apt/apt.conf.d/keep-cache && \
    apt-get install --update --snapshot "${snapshot}" -o Acquire::Check-Valid-Until=false -o Acquire::https::Verify-Peer=false --no-install-recommends -y ca-certificates && \
    apt-get install --snapshot "${snapshot}" --no-install-recommends -y ca-certificates python3 python3-pip && \
    rm -rf /var/log/* /var/cache/ldconfig/aux-cache; \
  else \
    apt-get update && apt-get install --no-install-recommends -y ca-certificates python3 python3-pip && \
    rm -rf /var/lib/apt/lists/* /var/log/* /var/cache/ldconfig/aux-cache; \
  fi

# --no-cache-dir keeps pip's wheel cache out of the layer.
# --break-system-packages is required on Debian 13: Debian marks the system
# python externally managed (PEP 668). The TEE boundary provides isolation here,
# not python's user separation.
COPY --chmod=644 --chown=0:0 python/requirements.txt /tmp/requirements.txt
RUN pip3 install --no-cache-dir --break-system-packages -r /tmp/requirements.txt && \
    rm /tmp/requirements.txt

# tee-node server binary + Confidential Space root cert from the shared base.
COPY --chmod=755 --chown=0:0 --from=node /app/server /app/server
COPY --chmod=644 --chown=0:0 --from=node /app/assets/google_confidential_space_root.crt /app/assets/google_confidential_space_root.crt

# The extension itself.
COPY --chmod=644 --chown=0:0 python/base/ /app/extension/base/
COPY --chmod=644 --chown=0:0 python/app/ /app/extension/app/
COPY --chmod=644 --chown=0:0 python/main.py /app/extension/main.py

# Normalize mtimes so layer hashes are stable in both directions — buildkit's
# rewrite-timestamp only clamps downward (moby/buildkit#3180).
RUN if [ "${SOURCE_DATE_EPOCH:-0}" -gt 0 ] 2>/dev/null; then find /app -exec touch -h -d @${SOURCE_DATE_EPOCH} {} +; fi

# Ports and mode match go/Dockerfile so compose works uniformly across languages.
#
# Production mode by default; docker-compose.yaml overrides to MODE=1 for local dev.
# FTDC rejects simulated attestation, so a real deploy MUST stay 0.
ENV MODE=0 CONFIG_PORT=5501 SIGN_PORT=7701 EXTENSION_PORT=7702
# Unbuffered stdout so container logs appear promptly.
ENV PYTHONUNBUFFERED=1
# Keep .pyc out of the image; they would vary by build host.
ENV PYTHONDONTWRITEBYTECODE=1

# Match tee-node: the TEE workload itself is the isolation boundary.
USER 0:0

# Must match go/Dockerfile's list exactly — a mismatch means Confidential Space
# silently rejects operator env overrides at attestation time.
LABEL "tee.launch_policy.allow_env_override"="LOG_LEVEL,PROXY_URL,INITIAL_OWNER,EXTENSION_ID,CHAIN_URL,MODE"

# Two-process layout: tee-node ./server handles config/sign and forwards actions
# to the extension on EXTENSION_PORT. If either dies the container exits so the
# orchestrator restarts it.
# /bin/bash, not /bin/sh: dash does not implement `wait -n`.
CMD ["/bin/bash", "-c", "/app/server & SERVER_PID=$!; python3 /app/extension/main.py & EXT_PID=$!; wait -n $SERVER_PID $EXT_PID; kill -TERM $SERVER_PID $EXT_PID 2>/dev/null; exit 1"]
