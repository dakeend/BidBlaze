#!/usr/bin/env bash
set -euo pipefail

cd /mnt/e/code/ai_zijie/auction-system/server-go

export PATH="$HOME/.local/go/bin:$HOME/go/bin:/home/luwen/.nvm/versions/node/v24.16.0/bin:$PATH"
export MYSQL_DSN="${MYSQL_DSN:-auction:auction_root@tcp(127.0.0.1:3306)/auction?parseTime=true&loc=Asia%2FShanghai&charset=utf8mb4}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"

exec go run .
