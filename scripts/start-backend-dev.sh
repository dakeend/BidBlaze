#!/usr/bin/env bash
set -euo pipefail

ROOT=/mnt/e/code/ai_zijie/auction-system
LOG_DIR="$ROOT/.data/logs"
mkdir -p "$LOG_DIR"

cd "$ROOT/server-go"

export PATH="$HOME/.local/go/bin:$HOME/go/bin:/home/luwen/.nvm/versions/node/v24.16.0/bin:$PATH"
export MYSQL_DSN="${MYSQL_DSN:-auction:auction_root@tcp(127.0.0.1:3306)/auction?parseTime=true&loc=Asia%2FShanghai&charset=utf8mb4}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"

nohup go run . >"$LOG_DIR/backend.log" 2>&1 </dev/null &
echo $! > /tmp/auction-backend.pid
echo "backend pid: $(cat /tmp/auction-backend.pid)"
