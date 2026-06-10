#!/usr/bin/env bash
set -euo pipefail

ROOT=/mnt/e/code/ai_zijie/auction-system
LOG_DIR="$ROOT/.data/logs"
mkdir -p "$LOG_DIR"

cd "$ROOT/admin-web"

export PATH="/home/luwen/.nvm/versions/node/v24.16.0/bin:$PATH"
export VITE_API_BASE="${VITE_API_BASE:-http://127.0.0.1:8080}"
export VITE_WS_BASE="${VITE_WS_BASE:-ws://127.0.0.1:8080}"
export VITE_CLIENT_TYPE="${VITE_CLIENT_TYPE:-admin}"
export VITE_USE_MSW="${VITE_USE_MSW:-false}"

nohup npm run dev -- --host 0.0.0.0 --port 5174 >"$LOG_DIR/admin-web.log" 2>&1 </dev/null &
echo $! > /tmp/auction-admin.pid
echo "admin pid: $(cat /tmp/auction-admin.pid)"
