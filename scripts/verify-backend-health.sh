#!/usr/bin/env bash
set -euo pipefail

cd /mnt/e/code/ai_zijie/auction-system/server-go

export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"

rm -f /tmp/auction-inline.log /tmp/auction-health.out

(go run . >/tmp/auction-inline.log 2>&1) &
pid=$!

cleanup() {
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
}
trap cleanup EXIT

for _ in $(seq 1 25); do
  if curl -sf http://127.0.0.1:8080/health >/tmp/auction-health.out 2>/dev/null; then
    echo "HEALTH_OK"
    cat /tmp/auction-health.out
    echo "---LOG---"
    sed -n '1,240p' /tmp/auction-inline.log
    exit 0
  fi
  sleep 1
done

echo "HEALTH_FAIL"
echo "---LOG---"
sed -n '1,240p' /tmp/auction-inline.log 2>/dev/null || true
exit 1
