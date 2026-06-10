#!/usr/bin/env bash
set -euo pipefail

cd /mnt/e/code/ai_zijie/auction-system/server-go

export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
export MYSQL_DSN="${MYSQL_DSN:-auction:auction_root@tcp(127.0.0.1:3306)/auction?parseTime=true&loc=UTC&charset=utf8mb4}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"

rm -f /tmp/auction-ready.log /tmp/auction-health.out /tmp/auction-ready.out

(go run . >/tmp/auction-ready.log 2>&1) &
pid=$!

cleanup() {
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
}
trap cleanup EXIT

health_ok=0
ready_ok=0

for _ in $(seq 1 25); do
  if [ "$health_ok" -eq 0 ] && curl -sf http://127.0.0.1:8080/health >/tmp/auction-health.out 2>/dev/null; then
    health_ok=1
  fi
  if [ "$ready_ok" -eq 0 ] && curl -sf http://127.0.0.1:8080/ready >/tmp/auction-ready.out 2>/dev/null; then
    ready_ok=1
  fi
  if [ "$health_ok" -eq 1 ] && [ "$ready_ok" -eq 1 ]; then
    break
  fi
  sleep 1
done

echo "HEALTH_OK=$health_ok"
if [ "$health_ok" -eq 1 ]; then
  cat /tmp/auction-health.out
fi
echo
echo "READY_OK=$ready_ok"
if [ "$ready_ok" -eq 1 ]; then
  cat /tmp/auction-ready.out
fi
echo
echo "---LOG---"
sed -n '1,240p' /tmp/auction-ready.log 2>/dev/null || true

if [ "$health_ok" -eq 1 ] && [ "$ready_ok" -eq 1 ]; then
  exit 0
fi

exit 1
