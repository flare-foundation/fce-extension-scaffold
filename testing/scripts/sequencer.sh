#!/usr/bin/env bash
# Centralized sequencer — dispatches /heartbeat to agents in weighted rotation.
# Runs as a persistent loop in its own tmux session.
# Usage: bash testing/scripts/sequencer.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CONF="$PROJECT_DIR/shared/rotation.conf"
LOG="$PROJECT_DIR/summary/sequencer.log"

log() {
    local msg="[sequencer $(date '+%Y-%m-%d %H:%M:%S')] $*"
    echo "$msg"
    echo "$msg" >> "$LOG"
}

# --- Load config ---
if [[ ! -f "$CONF" ]]; then
    echo "ERROR: Config not found: $CONF" >&2
    exit 1
fi

source "$CONF"
IFS=',' read -ra AGENTS <<< "$ROTATION"
CYCLE_LEN=${#AGENTS[@]}

# --- Resume from saved state ---
INDEX=0
if [[ -f "$STATE_FILE" ]]; then
    SAVED=$(cat "$STATE_FILE" 2>/dev/null || echo "0")
    if [[ "$SAVED" =~ ^[0-9]+$ ]]; then
        INDEX=$SAVED
    fi
fi

log "Starting sequencer. Rotation: ${AGENTS[*]} (${CYCLE_LEN} slots)"
log "Slot interval: ${SLOT_INTERVAL}s, Lock timeout: ${LOCK_TIMEOUT}s"
log "Resuming at index $INDEX"

# --- Main loop ---
while true; do
    # Re-read config each iteration (allows live changes)
    source "$CONF"
    IFS=',' read -ra AGENTS <<< "$ROTATION"
    CYCLE_LEN=${#AGENTS[@]}

    AGENT="${AGENTS[$((INDEX % CYCLE_LEN))]}"
    SLOT_NUM=$(( (INDEX % CYCLE_LEN) + 1 ))
    log "--- Slot $SLOT_NUM/$CYCLE_LEN: $AGENT ---"

    # 1. Check if agent's tmux session is alive
    if ! tmux has-session -t "testing-$AGENT" 2>/dev/null; then
        log "WARNING: testing-$AGENT session is dead. Advancing to next slot."
        INDEX=$((INDEX + 1))
        echo "$INDEX" > "$STATE_FILE"
        sleep 10
        continue
    fi

    # 2. Wait for lock to clear (previous run finished)
    if [[ -f "$LOCK_FILE" ]]; then
        LOCK_HOLDER=$(cut -d'|' -f1 "$LOCK_FILE" 2>/dev/null || echo "unknown")
        log "Lock held by $LOCK_HOLDER. Waiting for it to clear..."
        while [[ -f "$LOCK_FILE" ]]; do
            LOCK_TIME=$(cut -d'|' -f2 "$LOCK_FILE" 2>/dev/null || echo "0")
            NOW=$(date +%s)
            LOCK_AGE=$(( NOW - LOCK_TIME ))
            if (( LOCK_AGE > LOCK_TIMEOUT )); then
                LOCK_HOLDER=$(cut -d'|' -f1 "$LOCK_FILE" 2>/dev/null || echo "unknown")
                log "WARNING: Stale lock held by $LOCK_HOLDER (${LOCK_AGE}s). Force-clearing."
                rm -f "$LOCK_FILE"
                break
            fi
            sleep 5
        done
    fi

    # 3. Dispatch: send /heartbeat to the agent's tmux session
    log "Dispatching /heartbeat to $AGENT"
    tmux send-keys -t "testing-$AGENT" "/heartbeat" Enter

    # 4. Wait for agent to acquire lock (confirm it started)
    sleep 15
    if [[ -f "$LOCK_FILE" ]]; then
        HOLDER=$(cut -d'|' -f1 "$LOCK_FILE" 2>/dev/null || echo "unknown")
        if [[ "$HOLDER" == "$AGENT" ]]; then
            log "$AGENT acquired lock. Run in progress."
        else
            log "WARNING: Lock held by $HOLDER, expected $AGENT. Proceeding anyway."
        fi
    else
        log "WARNING: $AGENT did not acquire lock within 15s. It may have errored immediately."
    fi

    # 5. Advance rotation and save state
    INDEX=$((INDEX + 1))
    echo "$INDEX" > "$STATE_FILE"

    # 6. Wait the slot interval
    log "Sleeping ${SLOT_INTERVAL}s before next slot..."
    sleep "$SLOT_INTERVAL"
done
