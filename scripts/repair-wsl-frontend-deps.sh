#!/usr/bin/env bash
set -euo pipefail

ROOT=/mnt/e/code/ai_zijie/auction-system
NODE_BIN=/home/luwen/.nvm/versions/node/v24.16.0/bin
NPM_CLI=/home/luwen/.nvm/versions/node/v24.16.0/lib/node_modules/npm/bin/npm-cli.js
LOG_DIR="$ROOT/.data/logs"
mkdir -p "$LOG_DIR"

export PATH="$NODE_BIN:$PATH"

repair_one() {
  local app_dir=$1
  local log_file=$2

  cd "$app_dir"
  {
    echo "== $(date -Iseconds) =="
    echo "PWD=$PWD"
    "$NODE_BIN/node" -v
    "$NODE_BIN/node" "$NPM_CLI" -v
    "$NODE_BIN/node" "$NPM_CLI" install @rolldown/binding-linux-x64-gnu@1.0.2 --save-optional
    ls -la node_modules/@rolldown/binding-linux-x64-gnu
  } >>"$log_file" 2>&1
}

repair_one "$ROOT/mobile-h5" "$LOG_DIR/mobile-repair.log"
repair_one "$ROOT/admin-web" "$LOG_DIR/admin-repair.log"
