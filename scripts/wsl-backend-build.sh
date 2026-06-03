#!/usr/bin/env bash
source "$(dirname "$0")/wsl-env.sh"
cd /mnt/e/code/ai_zijie/auction-system/server-go || exit 1
echo "--- go build (GOTOOLCHAIN=auto, may try to download go1.26.3) ---"
go build -o /tmp/auction-server . 2>&1
echo "exit=$?"
