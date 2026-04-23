#!/usr/bin/env bash
set -euo pipefail

# Install all dependencies for the Flare TEE autonomous testing agents.
# Run this on a fresh Ubuntu 22.04 GCP VM.
#
# Usage (on the VM):
#   bash install.sh
#
# Installs:
#   - Docker
#   - Go 1.25.1
#   - Node.js 20 + Claude Code CLI
#   - Foundry (forge, cast, anvil)
#   - ngrok
#   - tmux, jq, git, curl

set -euo pipefail

GO_VERSION="1.25.1"
GO_ARCH="linux-amd64"

echo "=== Flare TEE Testing VM — Dependency Installer ==="
echo "Ubuntu version: $(lsb_release -rs)"
echo ""

# ─── System packages ────────────────────────────────────────────────────────

echo "--- Installing system packages ---"
sudo apt-get update -q
sudo apt-get install -y \
    tmux \
    jq \
    git \
    curl \
    wget \
    unzip \
    build-essential \
    ca-certificates \
    gnupg \
    lsb-release

# ─── Docker ─────────────────────────────────────────────────────────────────

echo ""
echo "--- Installing Docker ---"
if command -v docker &>/dev/null; then
    echo "Docker already installed: $(docker --version)"
else
    curl -fsSL https://get.docker.com | sh
    sudo usermod -aG docker "$USER"
    echo "Docker installed. NOTE: You must log out and back in (or run 'newgrp docker') for group to take effect."
fi

# ─── Go ─────────────────────────────────────────────────────────────────────

echo ""
echo "--- Installing Go $GO_VERSION ---"
if command -v go &>/dev/null && go version | grep -q "$GO_VERSION"; then
    echo "Go $GO_VERSION already installed."
else
    GO_TAR="go${GO_VERSION}.${GO_ARCH}.tar.gz"
    wget -q "https://go.dev/dl/${GO_TAR}" -O "/tmp/${GO_TAR}"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "/tmp/${GO_TAR}"
    rm "/tmp/${GO_TAR}"

    # Add to PATH for this session and future sessions
    export PATH=$PATH:/usr/local/go/bin
    if ! grep -q '/usr/local/go/bin' ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    echo "Go installed: $(go version)"
fi

# ─── Node.js 20 + Claude Code CLI ───────────────────────────────────────────

echo ""
echo "--- Installing Node.js 20 ---"
if command -v node &>/dev/null && node --version | grep -q '^v20\.'; then
    echo "Node.js 20 already installed: $(node --version)"
else
    curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
    sudo apt-get install -y nodejs
    echo "Node.js installed: $(node --version)"
fi

echo ""
echo "--- Installing Claude Code CLI ---"
if command -v claude &>/dev/null; then
    echo "Claude Code already installed: $(claude --version 2>/dev/null || echo 'version unknown')"
else
    sudo npm install -g @anthropic-ai/claude-code
    echo "Claude Code installed."
fi

# ─── Foundry ────────────────────────────────────────────────────────────────

echo ""
echo "--- Installing Foundry (forge, cast, anvil) ---"
if command -v forge &>/dev/null; then
    echo "Foundry already installed: $(forge --version)"
else
    curl -L https://foundry.paradigm.xyz | bash
    # foundryup modifies ~/.bashrc; source it and run
    export PATH="$HOME/.foundry/bin:$PATH"
    if ! grep -q '\.foundry/bin' ~/.bashrc; then
        echo 'export PATH="$HOME/.foundry/bin:$PATH"' >> ~/.bashrc
    fi
    "$HOME/.foundry/bin/foundryup"
    echo "Foundry installed: $(forge --version)"
fi

# ─── ngrok ──────────────────────────────────────────────────────────────────

echo ""
echo "--- Installing ngrok ---"
if command -v ngrok &>/dev/null; then
    echo "ngrok already installed."
else
    curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc \
        | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null
    echo "deb https://ngrok-agent.s3.amazonaws.com buster main" \
        | sudo tee /etc/apt/sources.list.d/ngrok.list
    sudo apt-get update -q
    sudo apt-get install -y ngrok
    echo "ngrok installed: $(ngrok --version)"
fi

# ─── Summary ────────────────────────────────────────────────────────────────

echo ""
echo "=== Installation complete ==="
echo ""
echo "Versions installed:"
echo "  Docker:      $(docker --version 2>/dev/null || echo 'check after re-login')"
echo "  Go:          $(go version 2>/dev/null || echo '/usr/local/go/bin/go version')"
echo "  Node.js:     $(node --version)"
echo "  npm:         $(npm --version)"
echo "  Claude Code: $(claude --version 2>/dev/null || echo 'installed')"
echo "  forge:       $( (command -v forge && forge --version) 2>/dev/null || echo 'installed (re-login to PATH)')"
echo "  ngrok:       $(ngrok --version 2>/dev/null || echo 'installed')"
echo "  tmux:        $(tmux -V)"
echo "  jq:          $(jq --version)"
echo ""
echo "=== Post-install steps ==="
echo ""
echo "1. Re-login or run 'newgrp docker' so your user can run Docker without sudo."
echo ""
echo "2. Configure ngrok auth token:"
echo "   ngrok config add-authtoken <your-token>"
echo ""
echo "3. Authenticate Claude Code:"
echo "   claude /login"
echo ""
echo "4. Clone the repo and start the agents:"
echo "   git clone <repo-url> tee"
echo "   cd tee/extension-examples/orderbook"
echo "   cp .env.example .env"
echo "   # Edit .env: set CHAIN_URL, DEPLOYMENT_PRIVATE_KEY, etc."
echo "   bash testing/scripts/setup.sh"
echo "   bash testing/scripts/start.sh"
