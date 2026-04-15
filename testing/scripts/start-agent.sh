#!/bin/bash
# Launch a single testing agent in its own tmux session
# Used by health-check.sh to restart dead agents
# Usage: bash testing/scripts/start-agent.sh <agent-name>
#   agent-name: smoketest | edgecase | chaos

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
AGENTS_DIR="$PROJECT_DIR/agents"

AGENT_NAME="${1:-}"
if [ -z "$AGENT_NAME" ]; then
  echo "Usage: start-agent.sh <smoketest|edgecase|chaos>"
  exit 1
fi

AGENT_DIR="$AGENTS_DIR/$AGENT_NAME"
if [ ! -d "$AGENT_DIR" ]; then
  echo "Error: Agent directory not found: $AGENT_DIR"
  exit 1
fi

SESSION="testing-$AGENT_NAME"

# Kill existing session if it's dead/stuck
if tmux has-session -t "$SESSION" 2>/dev/null; then
  tmux kill-session -t "$SESSION"
fi

SESSION_FILE="$AGENT_DIR/.session-id"
RESUME_FLAG=""
if [ -f "$SESSION_FILE" ]; then
  RESUME_FLAG="--resume $(cat "$SESSION_FILE")"
fi

tmux new-session -d -s "$SESSION" -c "$AGENT_DIR"
tmux send-keys -t "$SESSION" "claude --dangerously-skip-permissions $RESUME_FLAG" Enter

# Confirm bypass-permissions prompt
sleep 8
tmux send-keys -t "$SESSION" "" Enter || true
sleep 15
# Agent is now idle — the sequencer will dispatch /heartbeat when it's this agent's turn
