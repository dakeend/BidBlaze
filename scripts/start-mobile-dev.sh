#!/usr/bin/env bash
set -euo pipefail

ROOT=/mnt/e/code/ai_zijie/auction-system
LOG_DIR="$ROOT/.data/logs"
mkdir -p "$LOG_DIR"

cd "$ROOT/mobile-h5"

export PATH="/home/luwen/.nvm/versions/node/v24.16.0/bin:$PATH"
export VITE_API_BASE="${VITE_API_BASE:-http://127.0.0.1:8080}"
export VITE_WS_BASE="${VITE_WS_BASE:-ws://127.0.0.1:8080}"

nohup npm run dev -- --host 0.0.0.0 --port 5173 >"$LOG_DIR/mobile-h5.log" 2>&1 </dev/null &
echo $! > /tmp/auction-mobile.pid
echo "mobile pid: $(cat /tmp/auction-mobile.pid)"
