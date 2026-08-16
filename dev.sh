#!/usr/bin/env bash
# Start the taolu dev MCP server in the foreground.
#
# Env overrides (optional):
#   TAOLU_HOST  bind host     (default 127.0.0.1)
#   TAOLU_PORT  listen port   (default 8264)
#   TAOLU_REPO  vault path    (default ./var/vault.fossil)
set -euo pipefail
cd "$(dirname "$0")"

export TAOLU_REPO="${TAOLU_REPO:-$PWD/var/vault.fossil}"
export TAOLU_HOST="${TAOLU_HOST:-0.0.0.0}"
export TAOLU_PORT="${TAOLU_PORT:-8264}"

mkdir -p var
echo "taolu dev server -> http://${TAOLU_HOST}:${TAOLU_PORT}  vault=${TAOLU_REPO}"

# Build the embedded web UI if its dependencies are installed. The Go binary
# embeds pkg/web/dist via //go:embed, so it must exist before go run.
if [[ -d web/node_modules ]]; then
  echo "building web assets -> pkg/web/dist"
  (cd web && pnpm build)
else
  echo "warning: web/node_modules missing; run 'cd web && pnpm install' to build the UI" >&2
fi

exec go run ./cmd/taolu
