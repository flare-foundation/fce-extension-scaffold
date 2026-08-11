# Cloudflare Tunnel (ngrok replacement)

Exposes the extension proxy's external port (host `6674`) over a public HTTPS URL, so
Flare's TEE infrastructure can reach it. Use instead of ngrok. Compose file:
`docker-compose.cloudflared.yaml`.

Only testnets need this. On `--chain local` nothing is started — a local devnet reaches
the proxy on localhost.

## The scripts drive it

`start-services.sh` syncs the tunnel *before* every other container, then writes the URL
into `.env` as `EXT_PROXY_URL`. `full-setup.sh` calls it in Phase 2, and `post-build.sh` /
`test.sh` re-read `.env`, so a whole run picks the URL up by itself:

```bash
./scripts/full-setup.sh --chain coston2 --tunnel --test
```

| Situation | What `start-services.sh` does |
|---|---|
| Tunnel already running | Reuses it and adopts its URL — no flag needed |
| No tunnel, `--tunnel` passed | Starts it, waits for the URL, writes `EXT_PROXY_URL` |
| No tunnel, no `--tunnel` | Warns; `EXT_PROXY_URL` must already be valid in `.env` |
| `TUNNEL_ARGS` set (named tunnel) | Starts it and leaves your `EXT_PROXY_URL` alone |

`stop-services.sh` leaves the tunnel up unless you pass `--tunnel` — see below.

## One tunnel, shared by every extension

The tunnel does **not** belong to this extension. It runs in a compose project called
`tunnel`, set by the `name:` line at the top of `docker-compose.cloudflared.yaml`.

Compose identifies a container by **project name + service name** and nothing else — not
the file's path, not the folder it sits in. So every extension's copy of this file
addresses the *same* container: the first `up -d` creates it, every later one finds it
running and inherits its URL. That works because every extension here publishes ext-proxy
on host `6674`, so only one stack is ever behind the tunnel; it is handed from extension to
extension, keeping one URL across the switch.

That is also why `stop-services.sh` keeps it alive by default: tearing it down rotates the
URL for every other extension and strands their `EXT_PROXY_URL`.

**Copy `docker-compose.cloudflared.yaml` verbatim.** Compose compares the *resolved service
config* to decide reuse-vs-recreate. Comments and file paths are not part of that
comparison, but `command:` is — so a differing `TUNNEL_TARGET` recreates the container and
mints a new URL for everyone. Setting `TUNNEL_TARGET` by hand warns you for this reason.

Two cases that legitimately need their own tunnel, and get their own project:

| Case | Project | Why |
|---|---|---|
| `--local` (host Go proxy) | `tunnel-local` | Proxy listens on 6664, a different origin — the script handles this |
| `prediction_market` | give it its own | Publishes on 6676, cannot share |

## Manual

From the repo root, in Git Bash:

```bash
# 1. Start it (pulls cloudflare/cloudflared on first run)
docker compose -f docker-compose.cloudflared.yaml up -d

# 2. Read the URL cloudflared printed at startup
docker compose -f docker-compose.cloudflared.yaml logs cloudflared \
  | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' | tail -1
```

Put that URL in `.env` as `EXT_PROXY_URL=<url>`, **then** start the containers.
`start-services.sh` blocks on `$EXT_PROXY_URL/info`, so a stale value there is what makes
it fail. Stop with `docker compose -f docker-compose.cloudflared.yaml down`.

Skipping the copy-paste — does step 2 and writes the result straight into `.env`:

```bash
URL=$(docker compose -f docker-compose.cloudflared.yaml logs cloudflared \
      | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' | tail -1) \
  && sed -i "s|^EXT_PROXY_URL=.*|EXT_PROXY_URL=$URL|" .env \
  && grep '^EXT_PROXY_URL=' .env
```

Rewrites an existing `EXT_PROXY_URL=` line only — if the echo comes back empty, add the
line once by hand and this works from then on.

## Already registered on-chain? Repoint the machine record too

`EXT_PROXY_URL` in `.env` only feeds the local scripts. Flare's TEE infra delivers
instructions to the URL stored in the `MachineManager`, written once at registration —
`post-build.sh` will **not** rewrite it for an already-registered machine. A rotated URL
therefore looks like every on-chain op timing out at `pollAction` even though the chain leg
succeeded.

```bash
./scripts/update-tee-url.sh            # defaults to EXT_PROXY_URL from .env
```

Calls `MachineManager.updateTeeMachineSettings` directly — no attestation, one transaction,
ledger untouched. A full re-registration would trip `ChallengeExpired` on a long-running
TEE. This is the strongest reason to want a stable hostname.

## Making the URL stable

Quick tunnels always rotate and no flag pins them. Cheapest fix first:

**Don't restart the tunnel.** It has no dependency on the main stack, so `down && up` on the
app containers does not touch it. Only restarting `cloudflared` itself rotates the URL.

**Named tunnel — permanently fixed hostname.** Needs a free Cloudflare account with a domain
on it:

1. Zero Trust → Networks → Tunnels → *Create a tunnel* → Cloudflared. Copy the `eyJ...` token.
2. Add a public hostname pointing at `http://host.docker.internal:6674`.
3. In `.env`:
   ```
   TUNNEL_ARGS=run --token eyJhIjoi...
   EXT_PROXY_URL=https://tee.yourdomain.com
   ```
4. `docker compose -f docker-compose.cloudflared.yaml up -d` — same command, named mode now.

`EXT_PROXY_URL` and the on-chain machine record then survive every restart, and
`restart: unless-stopped` becomes safe to add to the service.

## Know this

| | |
|---|---|
| A quick tunnel's URL changes on every start | There is deliberately no `restart:` policy — a silent restart would mint a new URL and strand `EXT_PROXY_URL` |
| The tunnel starts fine with nothing behind it | It just 502s until the containers are up |
| No network dependency on the main stack | Start order does not matter |
| Proxy running as a local Go binary on 6664? | `TUNNEL_TARGET=http://host.docker.internal:6664` — use a separate `-p` project so the shared tunnel is untouched |
