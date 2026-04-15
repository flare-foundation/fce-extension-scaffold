#!/bin/bash
# Watchdog script — checks if testing agents are alive, restarts dead ones
# Run via system cron every 5 minutes:
#   */5 * * * * /absolute/path/to/testing/scripts/health-check.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
RESTART_LOG="$PROJECT_DIR/summary/restarts.log"
HEALTH_LOG="$PROJECT_DIR/summary/health-check.log"

echo "$(date '+%Y-%m-%d %H:%M:%S') - health check" >> "$HEALTH_LOG"

# Respect intentional shutdown — don't restart anything if stop.sh was run
if [ -f /tmp/flare-testing-shutdown ]; then
  echo "$(date '+%Y-%m-%d %H:%M:%S') - shutdown flag present, skipping" >> "$HEALTH_LOG"
  exit 0
fi

# Check if the sequencer is alive
if ! tmux has-session -t "testing-sequencer" 2>/dev/null; then
  echo "$(date '+%Y-%m-%d %H:%M:%S') - sequencer session dead, restarting" >> "$RESTART_LOG"
  SEQUENCER_SCRIPT="$SCRIPT_DIR/sequencer.sh"
  tmux new-session -d -s testing-sequencer \
    "bash $SEQUENCER_SCRIPT 2>&1 | tee -a $PROJECT_DIR/summary/sequencer.log"
fi

# Check each agent's session independently
for agent in smoketest edgecase chaos; do
  SESSION="testing-$agent"

  # Check if the tmux session exists
  if ! tmux has-session -t "$SESSION" 2>/dev/null; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $agent session dead, restarting" >> "$RESTART_LOG"
    bash "$SCRIPT_DIR/start-agent.sh" "$agent"
    continue
  fi

  # Check if the pane has a running claude process
  PANE_PID=$(tmux list-panes -t "$SESSION" -F '#{pane_pid}' 2>/dev/null | head -1)
  if [ -n "$PANE_PID" ]; then
    if ! pgrep -P "$PANE_PID" -f "claude" >/dev/null 2>&1; then
      echo "$(date '+%Y-%m-%d %H:%M:%S') - $agent process dead, restarting" >> "$RESTART_LOG"
      bash "$SCRIPT_DIR/start-agent.sh" "$agent"
    fi
  fi
done
