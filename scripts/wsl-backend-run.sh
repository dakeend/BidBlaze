#!/usr/bin/env bash
source "$(dirname "$0")/wsl-env.sh"
export GIN_MODE=release
/tmp/auction-server >/tmp/auction-server.log 2>&1 &
PID=$!
echo "server pid=$PID, waiting for boot..."
sleep 2
echo "--- GET /health ---"; curl -s -w "\nHTTP %{http_code}\n" http://localhost:8080/health
echo "--- GET /ready  ---"; curl -s -w "\nHTTP %{http_code}\n" http://localhost:8080/ready
echo "--- server log ---"; cat /tmp/auction-server.log
kill "$PID" 2>/dev/null
echo "stopped."
