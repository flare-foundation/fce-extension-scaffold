# Autonomous Testing Agents

Three specialized Claude Code agents that continuously test the Flare TEE extension deployment pipeline against Coston2 testnet. They run indefinitely on a GCP VM, finding bugs, edge cases, flaky behavior, and unclear error messages.

## Agents

| Agent | Focus | Cycle | What It Does |
|-------|-------|-------|-------------|
| **Smoketest** | Happy path | ~5 min (2/5 slots) | Runs `full-setup.sh --test` and variants repeatedly. Catches regressions and flaky infrastructure. |
| **Edge Case** | Systematic | ~5 min (2/5 slots) | Works through 20+ documented edge cases (D1-E9). Records whether errors match expectations and how clear the messages are. |
| **Chaos** | Adversarial | ~5 min (1/5 slots) | Tries to break things creatively — concurrent deploys, mid-process kills, code modifications. Runs in its own git worktree so it can modify source code. |

## How It Works

### Architecture

```
GCP VM (e2-standard-4, Ubuntu 22.04)
├── tmux session "testing-sequencer"  (rotation dispatcher)
├── tmux session "testing-smoketest"  (Claude Code CLI, passive)
├── tmux session "testing-edgecase"   (Claude Code CLI, passive)
├── tmux session "testing-chaos"      (Claude Code CLI, passive)
├── ngrok (shared tunnel, exposes port 6674)
├── Docker (shared, one agent at a time)
└── System cron (health-check.sh every 5 min)
```

Each agent is a Claude Code CLI session with its own CLAUDE.md (identity/behavior), skills (heartbeat cycle, run-scenario), and hooks (audit-log, teardown). A centralized sequencer dispatches `/heartbeat` to each agent in a weighted rotation, which picks a scenario, runs it, and logs results.

### Sequencer-Based Scheduling

The deployment stack uses fixed ports (Redis 6382, ext-proxy 6674, etc.) and a single ngrok tunnel. Only one agent can have services running at a time.

**Sequencer:** A persistent bash script (`scripts/sequencer.sh`) runs in its own tmux session. It cycles through a configurable weighted rotation, dispatching `/heartbeat` to each agent via `tmux send-keys`. Agents are passive — they sit idle until the sequencer tells them to go.

**Rotation:** Configured in `shared/rotation.conf`. Default: `smoketest,edgecase,smoketest,edgecase,chaos` (2:2:1 weight). Edit the file to change weights — the sequencer picks up changes on the next cycle.

**Lock file:** `/tmp/flare-extension-testing.lock`
- The dispatched agent writes `agent-name|unix-timestamp` when it starts
- The sequencer waits for the lock to clear before dispatching the next agent
- If the lock exceeds 12 minutes (configurable), it's force-cleared
- The Stop hook automatically clears the lock on crash/exit

### Chaos Worktree

The chaos agent runs in a git worktree (`agents/chaos/worktree/`) so it can modify scripts, Go code, and Solidity without affecting the other agents. It resets the worktree (`git checkout .`) at the start of each cycle.

## Quick Start

### 1. Provision the VM

Create a GCP e2-standard-4 (4 vCPU, 16GB RAM) with Ubuntu 22.04 and 50GB disk. Install:

```bash
# Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Go (check go.mod for version)
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# Node.js (for Claude Code)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# Other deps
sudo apt install -y tmux jq
npm install -g @anthropic-ai/claude-code

# Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# ngrok
curl -sSL https://ngrok-agent.s3.amazonaws.com/ngrok-v3-stable-linux-amd64.tgz | sudo tar xz -C /usr/local/bin
ngrok config add-authtoken <your-token>
```

### 2. Clone and Configure

```bash
git clone <repo-url> tee
cd tee/extension-examples/orderbook

# Configure .env
cp .env.example .env
# Edit .env:
#   CHAIN_URL=https://coston2-api.flare.network/ext/C/rpc
#   LOCAL_MODE=false
#   ADDRESSES_FILE=./config/coston2/deployed-addresses.json
#   DEPLOYMENT_PRIVATE_KEY=<funded Coston2 private key>
#   SIMULATED_TEE=true
```

### 3. Authenticate Claude Code

```bash
claude /login
```

This is interactive — do it once manually.

### 4. Run Setup

```bash
bash testing/scripts/setup.sh
```

This checks prerequisites, verifies your `.env`, and installs the cron watchdog.

### 5. Start the Agents

```bash
bash testing/scripts/start.sh
```

This starts ngrok, creates the chaos worktree, launches three Claude Code sessions in tmux, and registers their heartbeat schedules.

### 6. Monitor

```bash
# Attach to a specific agent
tmux attach -t testing-smoketest
tmux attach -t testing-edgecase
tmux attach -t testing-chaos

# Detach: Ctrl+B then D
```

## Monitoring & Debugging

### Check Results

```bash
# Latest status for all agents
cat testing/summary/latest-status.md

# Notable findings (bugs, unclear errors)
cat testing/summary/findings.md

# Individual run logs
ls testing/agents/smoketest/results/
ls testing/agents/edgecase/results/
ls testing/agents/chaos/results/

# Read a specific result
cat testing/agents/smoketest/results/2026-04-11T14-30-00-full-setup-standard.md
```

### Check Agent Health

```bash
# Are the tmux sessions alive?
tmux has-session -t testing-sequencer && echo "sequencer alive" || echo "sequencer dead"
tmux has-session -t testing-smoketest && echo "smoketest alive" || echo "smoketest dead"
tmux has-session -t testing-edgecase  && echo "edgecase alive"  || echo "edgecase dead"
tmux has-session -t testing-chaos     && echo "chaos alive"     || echo "chaos dead"

# Check the health-check log
cat testing/summary/health-check.log

# Check restart history
cat testing/summary/restarts.log
```

### Check the Lock

```bash
# Who holds the lock?
cat /tmp/flare-extension-testing.lock

# How old is it?
LOCK_TIME=$(cut -d'|' -f2 /tmp/flare-extension-testing.lock)
echo "Age: $(( $(date +%s) - LOCK_TIME ))s"

# Force-clear a stuck lock
rm -f /tmp/flare-extension-testing.lock
```

### Check Edge Case Progress

```bash
# How many scenarios tested?
grep -c '\[x\]' testing/agents/edgecase/scenario-tracker.md
grep -c '\[ \]' testing/agents/edgecase/scenario-tracker.md

# View the tracker
cat testing/agents/edgecase/scenario-tracker.md
```

### Check Docker State

```bash
# Are containers running?
docker compose ps

# Container logs
docker compose logs extension-tee --tail 50
docker compose logs ext-proxy --tail 50

# Force teardown
docker compose down
```

### Check ngrok

```bash
# Is ngrok running?
curl -sf http://localhost:4040/api/tunnels | jq '.tunnels[].public_url'

# ngrok logs
cat /tmp/ngrok-testing.log
```

### Audit Logs

Each agent has an audit log of every tool call:

```bash
cat testing/agents/smoketest/results/audit.log
cat testing/agents/edgecase/results/audit.log
cat testing/agents/chaos/results/audit.log
```

## Stopping

```bash
bash testing/scripts/stop.sh
```

This kills the tmux session, stops Docker containers, stops ngrok, and clears the lock file.

To also remove the cron watchdog:

```bash
crontab -l | grep -v health-check.sh | crontab -
```

## Directory Structure

```
testing/
├── scripts/
│   ├── setup.sh              # First-time VM setup (interactive)
│   ├── start.sh              # Launch ngrok + sequencer + 3 agents in tmux
│   ├── sequencer.sh          # Rotation dispatcher (persistent loop)
│   ├── start-agent.sh        # Restart a single dead agent
│   ├── stop.sh               # Stop everything
│   └── health-check.sh       # Cron watchdog (every 5 min)
│
├── agents/
│   ├── smoketest/
│   │   ├── CLAUDE.md          # Agent identity and behavior
│   │   ├── .claude/
│   │   │   ├── settings.json  # Hooks (audit-log, teardown)
│   │   │   └── skills/        # heartbeat + run-scenario
│   │   └── results/           # One .md file per test run
│   │
│   ├── edgecase/
│   │   ├── CLAUDE.md
│   │   ├── .claude/           # Same structure as smoketest
│   │   ├── results/
│   │   └── scenario-tracker.md
│   │
│   └── chaos/
│       ├── CLAUDE.md
│       ├── .claude/           # Same structure as smoketest
│       ├── results/
│       └── worktree/          # Git worktree (created by start.sh)
│
├── shared/
│   ├── rotation.conf          # Sequencer config (rotation order, timing)
│   ├── hooks/
│   │   ├── audit-log.sh       # PostToolUse: logs tool calls
│   │   └── teardown.sh        # Stop: docker down + release lock
│   └── scenarios/
│       ├── smoketest-scenarios.md   # 9 happy-path scenarios
│       ├── edgecase-scenarios.md    # 20+ edge cases (D1-E9)
│       └── chaos-scenarios.md       # 17 chaos scenarios + invention
│
└── summary/
    ├── findings.md            # Bugs and notable behavior (all agents write here)
    ├── latest-status.md       # Last result per agent
    ├── sequencer.log          # Sequencer dispatch log
    ├── restarts.log           # Agent restart history
    └── health-check.log       # Watchdog output
```

## Scheduling

The sequencer dispatches agents in a weighted rotation with a 5-minute minimum gap between slots:

| Rotation | Weight | Slots per cycle |
|----------|--------|----------------|
| Smoketest | 2 | 2 of every 5 |
| Edge Case | 2 | 2 of every 5 |
| Chaos | 1 | 1 of every 5 |

Full cycle: ~25 minutes (5 slots x 5 min). Actual throughput adapts — if a run takes longer than 5 min, the next dispatch waits for it to finish.

Edit `shared/rotation.conf` to change the rotation, slot interval, or lock timeout. Changes take effect on the next cycle without any restarts.

The health-check cron (`health-check.sh`, every 5 min) monitors the sequencer and all three agents, restarting any that have died.

## Troubleshooting

### Agent not receiving dispatches

1. Check if the sequencer is alive: `tmux has-session -t testing-sequencer`
2. Check the sequencer log for this agent: `grep <agent> testing/summary/sequencer.log | tail -5`
3. Check if the agent's tmux session is alive: `tmux has-session -t testing-<agent>`
4. Attach and check if the agent is idle or stuck: `tmux attach -t testing-<agent>`

### Lock stuck / agents all skipping

```bash
# Check who holds it and how old it is
cat /tmp/flare-extension-testing.lock
# Clear it manually
rm -f /tmp/flare-extension-testing.lock
```

The teardown hook should auto-clear on agent crash, but if the whole process was killed with SIGKILL, the hook won't fire.

### Docker containers orphaned

```bash
docker compose down
docker ps  # check for stragglers
```

### ngrok tunnel died

```bash
# Check if it's running
curl -sf http://localhost:4040/api/tunnels

# Restart it
ngrok http 6674 --log=stdout > /tmp/ngrok-testing.log 2>&1 &
sleep 3

# Update .env with new URL
NGROK_URL=$(curl -sf http://localhost:4040/api/tunnels | jq -r '.tunnels[0].public_url')
sed -i "s|^EXT_PROXY_URL=.*|EXT_PROXY_URL=$NGROK_URL|" .env
```

### Chaos worktree broken

```bash
# Reset it
cd testing/agents/chaos/worktree && git checkout . && cd -

# Or nuke and recreate
rm -rf testing/agents/chaos/worktree
git worktree prune
git worktree add testing/agents/chaos/worktree HEAD
```

### Out of disk space (result logs accumulating)

```bash
# Check disk usage per agent
du -sh testing/agents/*/results/

# Archive old results
tar czf testing/results-archive-$(date +%Y%m%d).tar.gz testing/agents/*/results/
rm testing/agents/*/results/2026-04-1[0-5]*.md  # delete old ones
```

### Changing the rotation weights

Edit `testing/shared/rotation.conf` and change the `ROTATION` line. The sequencer re-reads the config on each cycle, so changes take effect within one cycle without restarts.

Example weights:
- Equal (1:1:1): `ROTATION=smoketest,edgecase,chaos`
- Heavy edge case (1:3:1): `ROTATION=smoketest,edgecase,edgecase,edgecase,chaos`
- Smoketest only: `ROTATION=smoketest`

## Design Spec & Plan

- **Design:** `docs/superpowers/specs/2026-04-11-autonomous-testing-agent-design.md`
- **Implementation plan:** `docs/superpowers/plans/2026-04-11-autonomous-testing-agent.md`
- **Edge case reference:** `notes/EXTENSION-DEPLOYMENT-EDGE-CASES.md`
- **Stability audit:** `notes/EDGE-CASES-AND-STABILITY-AUDIT.md`
