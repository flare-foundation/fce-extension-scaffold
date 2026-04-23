#\!/bin/bash
# Stop hook — tears down Docker services and releases the lock file
# Prevents orphaned containers and stale locks on agent crash/exit

LOCK_FILE="/tmp/flare-extension-testing.lock"
AGENT_NAME="${AGENT_NAME:-unknown}"
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

# Find the repo root (go up from agent dir until we find docker-compose.yaml)
SEARCH_DIR="$(pwd)"
REPO_ROOT=""
while [ "$SEARCH_DIR" \!= "/" ]; do
  if [ -f "$SEARCH_DIR/docker-compose.yaml" ]; then
    REPO_ROOT="$SEARCH_DIR"
    break
  fi
  SEARCH_DIR="$(dirname "$SEARCH_DIR")"
done

# Tear down Docker services if we can find the repo
if [ -n "$REPO_ROOT" ]; then
  cd "$REPO_ROOT" && docker compose down 2>/dev/null || true
fi

# Release the lock if it belongs to this agent
if [ -f "$LOCK_FILE" ]; then
  LOCK_OWNER=$(cut -d'|' -f1 "$LOCK_FILE" 2>/dev/null || echo "")
  if [ "$LOCK_OWNER" = "$AGENT_NAME" ]; then
    rm -f "$LOCK_FILE"
  fi
fi

# Log to audit
mkdir -p results
echo "[$TIMESTAMP] SESSION_END (teardown)" >> results/audit.log

exit 0
