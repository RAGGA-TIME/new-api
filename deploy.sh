#!/usr/bin/env bash
set -euo pipefail

# === Config ===
BINARY_NAME="new-api"
PID_FILE="new-api.pid"
LOG_FILE="new-api.log"
GO_MODULE="github.com/QuantumNous/new-api"
VERSION_FILE="VERSION"
DEFAULT_PORT=3000
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"

# === Colors ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }

# === Helpers ===

get_pid() {
    # 1. Try PID file first
    if [[ -f "$PROJECT_DIR/$PID_FILE" ]]; then
        local pid
        pid=$(cat "$PROJECT_DIR/$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo "$pid"
            return 0
        fi
        # Stale PID file
        rm -f "$PROJECT_DIR/$PID_FILE"
    fi
    # 2. Fallback: find process whose executable path is in this project directory
    local bin_path="$PROJECT_DIR/$BINARY_NAME"
    local pid
    pid=$(pgrep -f "^${bin_path}" 2>/dev/null | head -1)
    if [[ -n "$pid" ]]; then
        echo "$pid"
        return 0
    fi
    # 3. Fallback: find process listening on the service port
    local port="${PORT:-$DEFAULT_PORT}"
    pid=$(lsof -i :"$port" -sTCP:LISTEN -t 2>/dev/null | head -1)
    if [[ -n "$pid" ]]; then
        echo "$pid"
        return 0
    fi
    return 1
}

wait_for_exit() {
    local pid=$1
    local timeout=10
    local elapsed=0
    while kill -0 "$pid" 2>/dev/null; do
        if (( elapsed >= timeout )); then
            return 1
        fi
        sleep 1
        ((elapsed++))
    done
    return 0
}

# === Commands ===

cmd_build_frontend_default() {
    info "Building default frontend..."
    if [[ ! -d "$PROJECT_DIR/web/default/node_modules" ]]; then
        (cd "$PROJECT_DIR/web/default" && bun install)
    fi
    (cd "$PROJECT_DIR/web/default" && bun run build)
    ok "Default frontend built."
}

cmd_build_frontend_classic() {
    info "Building classic frontend..."
    if [[ ! -d "$PROJECT_DIR/web/classic/node_modules" ]]; then
        (cd "$PROJECT_DIR/web/classic" && bun install)
    fi
    (cd "$PROJECT_DIR/web/classic" && bun run build)
    ok "Classic frontend built."
}

cmd_build_frontend() {
    cmd_build_frontend_default
    cmd_build_frontend_classic
}

cmd_build_backend() {
    info "Building backend..."
    local version="dev"
    if [[ -f "$PROJECT_DIR/$VERSION_FILE" ]]; then
        local v
        v=$(cat "$PROJECT_DIR/$VERSION_FILE")
        [[ -n "$v" ]] && version="$v"
    fi
    (cd "$PROJECT_DIR" && go build -ldflags "-s -w -X '${GO_MODULE}/common.Version=${version}'" -o "$BINARY_NAME" .)
    ok "Backend built → ./${BINARY_NAME}"
}

cmd_build() {
    local target="${1:-all}"
    case "$target" in
        all)
            cmd_build_frontend
            cmd_build_backend
            ;;
        frontend)
            cmd_build_frontend
            ;;
        frontend-default)
            cmd_build_frontend_default
            ;;
        frontend-classic)
            cmd_build_frontend_classic
            ;;
        backend)
            cmd_build_backend
            ;;
        *)
            error "Unknown build target: $target"
            echo "Usage: $0 build [all|frontend|frontend-default|frontend-classic|backend]"
            exit 1
            ;;
    esac
}

cmd_start() {
    local pid
    if pid=$(get_pid); then
        warn "Already running (PID $pid)"
        return 0
    fi

    if [[ ! -x "$PROJECT_DIR/$BINARY_NAME" ]]; then
        error "Binary not found: $PROJECT_DIR/$BINARY_NAME — run '$0 build' first"
        return 1
    fi

    info "Starting ${BINARY_NAME}..."

    # Load .env if present (safe parse: skip comments/empty lines, quote values)
    if [[ -f "$PROJECT_DIR/.env" ]]; then
        while IFS='=' read -r key value; do
            key=$(echo "$key" | xargs)
            [[ -z "$key" || "$key" == \#* ]] && continue
            export "$key=$value"
        done < "$PROJECT_DIR/.env"
    fi

    nohup "$PROJECT_DIR/$BINARY_NAME" >> "$PROJECT_DIR/$LOG_FILE" 2>&1 &
    local new_pid=$!
    echo "$new_pid" > "$PROJECT_DIR/$PID_FILE"

    # Brief check that process didn't die immediately
    sleep 1
    if kill -0 "$new_pid" 2>/dev/null; then
        ok "Started (PID $new_pid)"
    else
        error "Process exited immediately. Check ${LOG_FILE} for details."
        rm -f "$PROJECT_DIR/$PID_FILE"
        return 1
    fi
}

cmd_stop() {
    local pid
    if ! pid=$(get_pid); then
        warn "Not running"
        return 0
    fi

    info "Stopping ${BINARY_NAME} (PID $pid)..."
    kill "$pid"

    if wait_for_exit "$pid"; then
        ok "Stopped"
    else
        warn "Did not exit in 10s, sending SIGKILL..."
        kill -9 "$pid" 2>/dev/null || true
        ok "Force stopped"
    fi
    rm -f "$PROJECT_DIR/$PID_FILE"
}

cmd_restart() {
    cmd_stop
    cmd_start
}

cmd_deploy() {
    cmd_build all
    cmd_stop
    cmd_start
    ok "Deploy complete!"
}

cmd_status() {
    local pid
    if pid=$(get_pid); then
        ok "Running (PID $pid)"
    else
        info "Not running"
    fi
}

cmd_logs() {
    if [[ ! -f "$PROJECT_DIR/$LOG_FILE" ]]; then
        info "No log file found (${LOG_FILE})"
        return 0
    fi
    tail -f "$PROJECT_DIR/$LOG_FILE"
}

# === Main ===

usage() {
    cat <<EOF
Usage: $0 <command> [args]

Commands:
  build [all|frontend|frontend-default|frontend-classic|backend]
      Build project components (default: all)
  start       Start service in background
  stop        Stop service (SIGTERM, SIGKILL after 10s)
  restart     Stop then start
  deploy      Build all + stop + start
  status      Show service status
  logs        Tail the log file

Examples:
  $0 build                     # Build frontend + backend
  $0 build frontend-default    # Build default frontend only
  $0 deploy                    # One-command deploy
EOF
}

case "${1:-}" in
    build)
        cmd_build "${2:-all}"
        ;;
    start)
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    deploy)
        cmd_deploy
        ;;
    status)
        cmd_status
        ;;
    logs)
        cmd_logs
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        error "Unknown command: ${1:-}"
        usage
        exit 1
        ;;
esac
