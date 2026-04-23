#!/bin/bash
# Launch all 3 testing agents in separate tmux sessions
# Usage: bash testing/scripts/start.sh
#
# Prerequisites:
#   - Claude Code CLI installed and authenticated
#   - tmux installed
#   - Docker running
#   - ngrok configured
#   - .env configured with Coston2 credentials
#
# Attach to an agent:
#   tmux attach -t testing-smoketest
#   tmux attach -t testing-edgecase
#   tmux attach -t testing-chaos

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$PROJECT_DIR/.." && pwd)"
AGENTS_DIR="$PROJECT_DIR/agents"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[start]${NC} $*"; }
die()  { echo -e "${RED}[start] ERROR:${NC} $*" >&2; exit 1; }

# --- Clear shutdown flag (if stop.sh was run previously) ---
rm -f /tmp/flare-testing-shutdown

# --- Verify prerequisites ---
command -v tmux >/dev/null 2>&1 || die "tmux is not installed"
command -v claude >/dev/null 2>&1 || die "Claude Code CLI is not installed"
command -v docker >/dev/null 2>&1 || die "Docker is not installed"
command -v ngrok >/dev/null 2>&1 || die "ngrok is not installed"

# --- Check if already running ---
for agent in smoketest edgecase chaos; do
  if tmux has-session -t "testing-$agent" 2>/dev/null; then
    die "Session 'testing-$agent' already running. Run stop.sh first."
  fi
done

if tmux has-session -t "testing-sequencer" 2>/dev/null; then
  die "Session 'testing-sequencer' already running. Run stop.sh first."
fi

# --- Start ngrok (if not already running) ---
if ! curl -sf http://localhost:4040/api/tunnels >/dev/null 2>&1; then
  log "Starting ngrok tunnel for port 6674..."
  ngrok http 6674 --log=stdout > /tmp/ngrok-testing.log 2>&1 &
  sleep 3
fi

# Capture ngrok URL
NGROK_URL=$(curl -sf http://localhost:4040/api/tunnels | jq -r '.tunnels[] | select(.config.addr | test("6674")) | .public_url' 2>/dev/null || echo "")
if [ -z "$NGROK_URL" ]; then
  log "WARNING: Could not detect ngrok URL. EXT_PROXY_URL will need to be set manually."
else
  log "ngrok URL: $NGROK_URL"
  # Update .env with ngrok URL (only if EXT_PROXY_URL line exists)
  if grep -q "^EXT_PROXY_URL=" "$REPO_ROOT/.env" 2>/dev/null; then
    sed -i "s|^EXT_PROXY_URL=.*|EXT_PROXY_URL=$NGROK_URL|" "$REPO_ROOT/.env"
  else
    echo "EXT_PROXY_URL=$NGROK_URL" >> "$REPO_ROOT/.env"
  fi
fi

# --- Create/reset Chaos worktree ---
log "Setting up Chaos worktree..."
if [ -d "$AGENTS_DIR/chaos/worktree" ]; then
  cd "$AGENTS_DIR/chaos/worktree" && git checkout . 2>/dev/null && cd "$SCRIPT_DIR"
else
  cd "$REPO_ROOT" && git worktree add "$AGENTS_DIR/chaos/worktree" HEAD 2>/dev/null || true
fi

# --- Launch each agent in its own tmux session ---
for agent in smoketest edgecase chaos; do
  AGENT_DIR="$AGENTS_DIR/$agent"
  SESSION="testing-$agent"

  RESUME_FLAG=""
  SESSION_FILE="$AGENT_DIR/.session-id"
  if [ -f "$SESSION_FILE" ]; then
    RESUME_FLAG="--resume $(cat "$SESSION_FILE")"
  fi

  log "Starting $agent agent (session: $SESSION)..."
  tmux new-session -d -s "$SESSION" -c "$AGENT_DIR"
  tmux send-keys -t "$SESSION" "claude --dangerously-skip-permissions $RESUME_FLAG" Enter
done

# Wait for Claude Code to show the --dangerously-skip-permissions confirmation
log "Waiting for agents to show bypass-permissions prompt (8s)..."
sleep 8

# Confirm the bypass-permissions prompt in each session (press Enter)
for agent in smoketest edgecase chaos; do
  log "Confirming bypass-permissions for $agent..."
  tmux send-keys -t "testing-$agent" "" Enter || true
done

# Wait for Claude Code to fully initialize after confirmation
log "Waiting for agents to initialize after confirmation (15s)..."
sleep 15

# --- Launch the sequencer ---
log "Starting sequencer..."
tmux new-session -d -s testing-sequencer \
  "bash $PROJECT_DIR/scripts/sequencer.sh 2>&1 | tee -a $PROJECT_DIR/summary/sequencer.log"

log ""
log "All agents and sequencer started!"
log ""
log "  Attach to a session:"
log "    tmux attach -t testing-sequencer   (rotation control)"
log "    tmux attach -t testing-smoketest"
log "    tmux attach -t testing-edgecase"
log "    tmux attach -t testing-chaos"
log ""
log "  Edit rotation: $PROJECT_DIR/shared/rotation.conf"
log "  Sequencer log: $PROJECT_DIR/summary/sequencer.log"
log ""
log "  Detach from a session: Ctrl+B then D"
log "  Stop all:  bash $SCRIPT_DIR/stop.sh"
