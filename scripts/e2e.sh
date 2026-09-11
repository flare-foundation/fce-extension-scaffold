#!/usr/bin/env bash
#
# TEMPORARY: This helper manages background Go processes for local development.
# It will be removed once Docker Compose replaces the go run approach.
#
# Usage:
#   e2e.sh start <name> <pidfile> <logfile> <command...>
#   e2e.sh stop <pidfile>
#   e2e.sh stop-all <piddir>
#   e2e.sh status <piddir>
#   e2e.sh wait-for-url <url> <timeout_seconds>

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

log() { echo -e "${GREEN}[e2e]${NC} $*"; }
warn() { echo -e "${YELLOW}[e2e]${NC} $*"; }
err() { echo -e "${RED}[e2e]${NC} $*" >&2; }

cmd_start() {
    local name="$1"
    local pidfile="$2"
    local logfile="$3"
    shift 3
    local cmd=("$@")

    mkdir -p "$(dirname "$pidfile")" "$(dirname "$logfile")"

    # Check if already running
    if [ -f "$pidfile" ]; then
        local old_pid
        old_pid=$(cat "$pidfile")
        if kill -0 "$old_pid" 2>/dev/null; then
            warn "$name is already running (PID $old_pid)"
            return 0
        fi
        rm -f "$pidfile"
    fi

    log "Starting $name → ${cmd[*]}"
    log "  log: $logfile"

    "${cmd[@]}" > "$logfile" 2>&1 &
    local pid=$!

    echo "$pid" > "$pidfile"
    log "$name started (PID $pid)"
}

cmd_stop() {
    local pidfile="$1"
    local name
    name=$(basename "$pidfile" .pid)

    if [ ! -f "$pidfile" ]; then
        warn "$name: no PID file found"
        return 0
    fi

    local pid
    pid=$(cat "$pidfile")

    if ! kill -0 "$pid" 2>/dev/null; then
        warn "$name (PID $pid): not running, cleaning up PID file"
        _kill_children "$pid" "$name"
        rm -f "$pidfile"
        return 0
    fi

    log "Stopping $name (PID $pid)..."

    _kill_tree "$pid"

    # Wait up to 5 seconds for graceful shutdown
    local waited=0
    while kill -0 "$pid" 2>/dev/null && [ $waited -lt 5 ]; do
        sleep 1
        waited=$((waited + 1))
    done

    if kill -0 "$pid" 2>/dev/null; then
        warn "$name did not stop gracefully, forcing..."
        _kill_tree "$pid" 9
    fi

    rm -f "$pidfile"
    log "$name stopped"
}

_kill_tree() {
    local pid="$1"
    local sig="${2:-15}"

    local children
    children=$(pgrep -P "$pid" 2>/dev/null) || true
    for child in $children; do
        _kill_tree "$child" "$sig"
    done

    kill -"$sig" "$pid" 2>/dev/null || true
}

_kill_children() {
    local dead_pid="$1"
    local name="$2"

    local pgid
    pgid=$(ps -o pgid= -p "$dead_pid" 2>/dev/null | tr -d ' ') || true

    if [ -n "$pgid" ] && [ "$pgid" != "0" ]; then
        local orphans
        orphans=$(pgrep -g "$pgid" 2>/dev/null) || true
        for orphan in $orphans; do
            if kill -0 "$orphan" 2>/dev/null; then
                warn "$name: killing orphaned child process (PID $orphan)"
                kill "$orphan" 2>/dev/null || true
            fi
        done
    fi
}

cmd_stop_all() {
    local piddir="$1"

    if [ ! -d "$piddir" ]; then
        warn "No PID directory found at $piddir"
        return 0
    fi

    local found=0
    for pidfile in "$piddir"/*.pid; do
        [ -f "$pidfile" ] || continue
        found=1
        cmd_stop "$pidfile"
    done

    if [ $found -eq 0 ]; then
        log "No services running"
    fi
}

cmd_status() {
    local piddir="$1"

    if [ ! -d "$piddir" ]; then
        echo "No PID directory found at $piddir"
        return 0
    fi

    local found=0
    for pidfile in "$piddir"/*.pid; do
        [ -f "$pidfile" ] || continue
        found=1
        local name pid status
        name=$(basename "$pidfile" .pid)
        pid=$(cat "$pidfile")
        if kill -0 "$pid" 2>/dev/null; then
            status="${GREEN}RUNNING${NC} (PID $pid)"
        else
            status="${RED}STOPPED${NC} (stale PID $pid)"
        fi
        echo -e "  $name: $status"
    done

    if [ $found -eq 0 ]; then
        echo "  No services tracked"
    fi
}

# curl, not nslookup: Windows nslookup exits 0 on NXDOMAIN, so its status proves nothing.
diagnose_dns() {
    local url="$1" out host
    out=$(curl -sS --max-time 8 -o /dev/null "$url" 2>&1) || true
    case "$out" in *"Could not resolve"*|*"Resolving timed out"*) ;; *) return 0 ;; esac

    host="${url#*://}"; host="${host%%/*}"
    if curl -sS --max-time 8 -H 'accept: application/dns-json' "https://1.1.1.1/dns-query?name=$host&type=A" 2>/dev/null | grep -q '"Answer"'; then
        err "$host resolves on 1.1.1.1 but not on your DNS — a VPN or split-DNS resolver is blocking it"
        err "  Windows (elevated): Add-DnsClientNrptRule -Namespace \".${host#*.}\" -NameServers \"1.1.1.1\"; Clear-DnsClientCache"
        err "  or disconnect the VPN and re-run"
    else
        err "$host does not resolve anywhere — the tunnel probably never came up"
    fi
}

cmd_wait_for_url() {
    local url="$1"
    local timeout="${2:-120}"

    # Wall clock, not iteration count: a hanging DNS lookup makes one pass cost far more than 2s.
    SECONDS=0
    log "Waiting for $url (timeout: ${timeout}s)..."
    while [ "$SECONDS" -lt "$timeout" ]; do
        if curl -sf --max-time 5 "$url" > /dev/null 2>&1; then
            log "$url is ready (${SECONDS}s)"
            return 0
        fi
        sleep 2
    done

    err "Timed out waiting for $url after ${timeout}s"
    diagnose_dns "$url"
    return 1
}

# Main dispatch
case "${1:-help}" in
    start)
        shift
        cmd_start "$@"
        ;;
    stop)
        shift
        cmd_stop "$@"
        ;;
    stop-all)
        shift
        cmd_stop_all "$@"
        ;;
    status)
        shift
        cmd_status "$@"
        ;;
    wait-for-url)
        shift
        cmd_wait_for_url "$@"
        ;;
    help|*)
        echo "Usage: e2e.sh <command> [args...]"
        echo ""
        echo "Commands:"
        echo "  start <name> <pidfile> <logfile> <command...>  Start a process in background"
        echo "  stop <pidfile>                                  Stop a process by PID file"
        echo "  stop-all <piddir>                               Stop all processes in PID dir"
        echo "  status <piddir>                                 Show status of all processes"
        echo "  wait-for-url <url> [timeout]                    Wait for URL to respond (default 120s)"
        ;;
esac
