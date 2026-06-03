#!/usr/bin/env bash
cd /mnt/e/code/ai_zijie/auction-system || exit 1
echo "--- docker compose up -d ---"
docker compose up -d 2>&1
echo "--- wait for mysql healthy ---"
for i in $(seq 1 40); do
  status=$(docker inspect --format '{{.State.Health.Status}}' auction-mysql 2>/dev/null)
  echo "attempt $i mysql=$status redis=$(docker inspect --format '{{.State.Health.Status}}' auction-redis 2>/dev/null)"
  [ "$status" = "healthy" ] && break
  sleep 3
done
echo "--- compose ps ---"
docker compose ps
echo "--- verify seed users (token fix) ---"
docker exec auction-mysql mysql -uroot -pauction_root -N -e \
  "SELECT id,nickname,token FROM auction.users;" 2>/dev/null
echo "--- verify tables ---"
docker exec auction-mysql mysql -uroot -pauction_root -N -e \
  "SELECT table_name FROM information_schema.tables WHERE table_schema='auction' ORDER BY table_name;" 2>/dev/null
echo "--- redis ping ---"
docker exec auction-redis redis-cli ping
