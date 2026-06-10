#!/usr/bin/env bash
set -euo pipefail

for name in backend mobile admin; do
  pid_file="/tmp/auction-${name}.pid"
  if [ -f "$pid_file" ]; then
    pid=$(cat "$pid_file")
    kill "$pid" 2>/dev/null || true
    rm -f "$pid_file"
    echo "stopped ${name}: ${pid}"
  fi
done
