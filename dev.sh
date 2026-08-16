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
export TAOLU_HOST="${TAOLU_HOST:-127.0.0.1}"
export TAOLU_PORT="${TAOLU_PORT:-8264}"

mkdir -p var
echo "taolu dev server -> http://${TAOLU_HOST}:${TAOLU_PORT}  vault=${TAOLU_REPO}"
exec go run ./cmd/taolu
