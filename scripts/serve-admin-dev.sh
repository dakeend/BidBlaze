#!/usr/bin/env bash
set -euo pipefail

cd /mnt/e/code/ai_zijie/auction-system/admin-web

NODE_BIN=/home/luwen/.nvm/versions/node/v24.16.0/bin
export PATH="$NODE_BIN:$PATH"
export DEV_PROXY_TARGET="${DEV_PROXY_TARGET:-http://127.0.0.1:8080}"
export VITE_CLIENT_TYPE="${VITE_CLIENT_TYPE:-admin}"
export VITE_USE_MSW="${VITE_USE_MSW:-false}"

exec "$NODE_BIN/node" ./node_modules/vite/bin/vite.js --host 0.0.0.0 --port 5174
