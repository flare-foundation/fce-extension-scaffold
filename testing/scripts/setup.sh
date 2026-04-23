#!/bin/bash
# First-time setup for the testing VM
# Usage: bash testing/scripts/setup.sh
#
# This script is interactive — it prompts for configuration values.
# Run it once on a fresh GCP VM.

set -euo pipefail

GREEN='\033[0;32m'; CYAN='\033[0;36m'; RED='\033[0;31m'; NC='\033[0m'
log()  { echo -e "${GREEN}[setup]${NC} $*"; }
step() { echo -e "\n${CYAN}=== $1 ===${NC}"; }
die()  { echo -e "${RED}[setup] ERROR:${NC} $*" >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$PROJECT_DIR/.." && pwd)"

# --- Step 1: Check prerequisites ---
step "Checking prerequisites"

MISSING=()
command -v docker >/dev/null 2>&1 || MISSING+=("docker")
command -v go >/dev/null 2>&1 || MISSING+=("go")
command -v node >/dev/null 2>&1 || MISSING+=("node")
command -v tmux >/dev/null 2>&1 || MISSING+=("tmux")
command -v ngrok >/dev/null 2>&1 || MISSING+=("ngrok")
command -v forge >/dev/null 2>&1 || MISSING+=("foundry (forge)")
command -v claude >/dev/null 2>&1 || MISSING+=("claude-code (npm install -g @anthropic-ai/claude-code)")
command -v jq >/dev/null 2>&1 || MISSING+=("jq")

if [ ${#MISSING[@]} -gt 0 ]; then
  echo "Missing prerequisites:"
  for dep in "${MISSING[@]}"; do
    echo "  - $dep"
  done
  die "Install the above dependencies and re-run setup.sh"
fi

log "All prerequisites found"

# --- Step 2: Check .env ---
step "Checking .env configuration"

ENV_FILE="$REPO_ROOT/.env"
if [ ! -f "$ENV_FILE" ]; then
  if [ -f "$REPO_ROOT/.env.example" ]; then
    log "Copying .env.example to .env"
    cp "$REPO_ROOT/.env.example" "$ENV_FILE"
  else
    die ".env.example not found at $REPO_ROOT"
  fi
fi

# Check for required values
log "Please verify these values in $ENV_FILE:"
echo ""
echo "  CHAIN_URL=https://coston2-api.flare.network/ext/C/rpc"
echo "  LOCAL_MODE=false"
echo "  ADDRESSES_FILE=./config/coston2/deployed-addresses.json"
echo "  DEPLOYMENT_PRIVATE_KEY=<your funded Coston2 key>"
echo "  SIMULATED_TEE=true"
echo ""

read -p "Have you configured .env with a funded Coston2 key? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  die "Configure .env first, then re-run setup.sh"
fi

# --- Step 3: Check Claude Code auth ---
step "Checking Claude Code authentication"

log "Claude Code requires authentication. If not yet authenticated, run:"
echo "  claude /login"
echo ""
read -p "Is Claude Code authenticated? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  log "Run 'claude /login' and then re-run setup.sh"
  exit 0
fi

# --- Step 4: Create directory structure ---
step "Creating directory structure"

mkdir -p "$PROJECT_DIR/agents/smoketest/results"
mkdir -p "$PROJECT_DIR/agents/edgecase/results"
mkdir -p "$PROJECT_DIR/agents/chaos/results"
mkdir -p "$PROJECT_DIR/summary"

log "Directory structure ready"

# --- Step 5: Make scripts executable ---
step "Making scripts executable"

chmod +x "$SCRIPT_DIR/start.sh"
chmod +x "$SCRIPT_DIR/start-agent.sh"
chmod +x "$SCRIPT_DIR/stop.sh"
chmod +x "$SCRIPT_DIR/health-check.sh"
chmod +x "$PROJECT_DIR/shared/hooks/audit-log.sh"
chmod +x "$PROJECT_DIR/shared/hooks/teardown.sh"

log "Scripts are executable"

# --- Step 6: Set up cron watchdog ---
step "Setting up cron watchdog"

CRON_LINE="*/5 * * * * $SCRIPT_DIR/health-check.sh >> $PROJECT_DIR/summary/health-check.log 2>&1"
if crontab -l 2>/dev/null | grep -q "health-check.sh"; then
  log "Cron watchdog already installed"
else
  (crontab -l 2>/dev/null; echo "$CRON_LINE") | crontab -
  log "Cron watchdog installed: every 5 minutes"
fi

# --- Done ---
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN} Setup complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Next steps:"
echo "  1. Start the agents:  bash $SCRIPT_DIR/start.sh"
echo "  2. Attach to session: tmux attach -t testing"
echo "  3. Monitor results:   ls $PROJECT_DIR/agents/*/results/"
echo "  4. Check findings:    cat $PROJECT_DIR/summary/findings.md"
echo ""
