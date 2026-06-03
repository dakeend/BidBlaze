#!/usr/bin/env bash
cd /mnt/e/code/ai_zijie/auction-system || exit 1
echo "--- ps -a (auction containers) ---"
docker ps -a --filter "name=auction" --format 'table {{.Names}}\t{{.Status}}'
echo "--- mysql last logs ---"
docker logs --tail 25 auction-mysql 2>&1
echo "--- redis status ---"
docker inspect --format '{{.State.Status}}' auction-redis 2>/dev/null
