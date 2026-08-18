#!/usr/bin/env bash
# Manage the taolu dev MCP server as a background process with rotated logs
# under ./var.
#
# Usage: ./serve-dev.sh {start|stop|restart|status|logs|build}
#
# Env overrides (optional):
#   TAOLU_HOST      bind host        (default 127.0.0.1)
#   TAOLU_PORT      listen port      (default 8264)
#   TAOLU_REPO      vault path       (default ./var/vault.fossil)
#   MAX_LOG_BYTES   rotate threshold (default 5242880 = 5 MiB)
set -euo pipefail
cd "$(dirname "$0")"

CMD="${1:-start}"

export TAOLU_REPO="${TAOLU_REPO:-$PWD/var/vault.fossil}"
export TAOLU_HOST="${TAOLU_HOST:-0.0.0.0}"
export TAOLU_PORT="${TAOLU_PORT:-8264}"
MAX_LOG_BYTES="${MAX_LOG_BYTES:-5242880}"

VAR=./var
BIN="$VAR/taolu"
LOG="$VAR/taolu.log"
PID="$VAR/taolu.pid"

is_running() {
  [[ -f "$PID" ]] || return 1
  local pid
  pid=$(cat "$PID" 2>/dev/null || true)
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

rotate() {
  [[ -f "$LOG" ]] || return 0
  local size
  size=$(stat -c%s "$LOG" 2>/dev/null || echo 0)
  (( size < MAX_LOG_BYTES )) && return 0
  [[ -f "$LOG.1" ]] && mv -f "$LOG.1" "$LOG.2" 2>/dev/null || true
  mv -f "$LOG" "$LOG.1"
  echo "rotated $LOG -> $LOG.1"
}

build() {
  mkdir -p "$VAR"
  web_build
  echo "building -> $BIN"
  go build -o "$BIN" ./cmd/taolu
}

# web_build produces the embedded frontend bundle (pnpm). Required before go
# build, which embeds pkg/web/dist via //go:embed.
web_build() {
  if [[ -d web/node_modules ]]; then
    echo "building web assets -> pkg/web/dist"
    (cd web && pnpm build)
  else
    echo "web dependencies not installed; run 'cd web && pnpm install' first" >&2
    echo "(building Go binary without embedded UI)" >&2
  fi
}

start() {
  if is_running; then
    echo "already running (pid $(cat "$PID"))"
    return 0
  fi
  build
  rotate
  nohup env TAOLU_REPO="$TAOLU_REPO" TAOLU_HOST="$TAOLU_HOST" TAOLU_PORT="$TAOLU_PORT" \
    "$BIN" serve >>"$LOG" 2>&1 &
  echo $! >"$PID"
  echo "started (pid $!) -> http://${TAOLU_HOST}:${TAOLU_PORT}"
  echo "vault: $TAOLU_REPO  log: $LOG"
}

stop() {
  if ! is_running; then
    echo "not running"
    return 0
  fi
  local pid
  pid=$(cat "$PID")
  kill "$pid" 2>/dev/null || true
  for _ in $(seq 1 50); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.1
  done
  if kill -0 "$pid" 2>/dev/null; then
    echo "graceful stop timed out, forcing"
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$PID"
  echo "stopped (pid $pid)"
}

status() {
  if is_running; then
    echo "taolu dev server: running (pid $(cat "$PID")) -> http://${TAOLU_HOST}:${TAOLU_PORT}"
    echo "vault: $TAOLU_REPO"
    echo "log:   $LOG"
  else
    echo "taolu dev server: not running"
  fi
}

case "$CMD" in
  start)   start ;;
  stop)    stop ;;
  restart) stop; start ;;
  status)  status ;;
  logs)    tail -f "$LOG" ;;
  build)   build ;;
  web)     web_build ;;
  *)       echo "usage: $0 {start|stop|restart|status|logs|build|web}"; exit 2 ;;
esac
