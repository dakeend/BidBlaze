#!/usr/bin/env bash
# ============================================================
# 高并发竞拍测试脚本
# 用法: bash concurrency-test.sh [并发数默认10] [每买家出价次数默认5]
# ============================================================
set -euo pipefail

CONCURRENCY="${1:-10}"
BIDS_PER_BUYER="${2:-5}"
API_BASE="http://127.0.0.1:8080"
SELLER_TOKEN="mock-token-seller-001"
DATE_PREFIX=$(date +%Y-%m-%d)

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
fail() { echo -e "${RED}[✗]${NC} $1"; }

cleanup() {
  rm -f /tmp/concurrency-test-results-*.txt
}
trap cleanup EXIT

# ── Step 1: 创建拍卖并激活 ──
echo -e "\n${CYAN}══════ Step 1: 创建测试拍卖 ══════${NC}"

FUTURE_TIME="${DATE_PREFIX}T12:00:00+08:00"
RESP=$(curl -sf -X POST "$API_BASE/api/auctions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d "{
    \"title\": \"高并发测试-$(date +%H%M%S)\",
    \"start_price\": 1000,
    \"price_step\": 100,
    \"start_time\": \"$FUTURE_TIME\",
    \"duration_seconds\": 3600
  }")

AUCTION_ID=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['auction']['id'])" 2>/dev/null)

if [ -z "$AUCTION_ID" ]; then
  fail "创建拍卖失败: $RESP"
  exit 1
fi
log "拍卖已创建: ID=$AUCTION_ID"

# 手动激活
mysql -uauction -pauction_root -e "UPDATE auction.auctions SET status='active' WHERE id=$AUCTION_ID;" 2>/dev/null
log "拍卖已激活"

# ── Step 2: 注册买家 ──
echo -e "\n${CYAN}══════ Step 2: 注册 ${CONCURRENCY} 个买家 ══════${NC}"

declare -a BUYER_TOKENS
for i in $(seq 1 $CONCURRENCY); do
  NICK="并发买家$(printf "%02d" $i)"
  TOKEN=$(curl -sf -X POST "$API_BASE/api/login" \
    -H "Content-Type: application/json" \
    -d "{\"nickname\":\"$NICK\"}" | \
    python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)
  BUYER_TOKENS+=("$TOKEN")
  echo "  买家 #$i: $NICK → $TOKEN"
done
log "${#BUYER_TOKENS[@]} 个买家就绪"

# ── Step 3: 并发出价 ──
echo -e "\n${CYAN}══════ Step 3: 并发出价 (${CONCURRENCY} 并发 × ${BIDS_PER_BUYER} 次) ══════${NC}"

START_TIME=$(date +%s%3N)
SUCCESS=0
FAIL=0
CONFLICT=0
TOTAL=$((CONCURRENCY * BIDS_PER_BUYER))

bid_worker() {
  local token="$1"
  local auction_id="$2"
  local rounds="$3"
  local worker_id="$4"
  local result_file="/tmp/concurrency-test-results-$worker_id.txt"
  local base_price=1000
  local step=100

  for r in $(seq 1 $rounds); do
    # 获取当前拍卖状态
    local status_resp=$(curl -sf "$API_BASE/api/auctions/$auction_id/status" 2>/dev/null || echo "{}")
    local current_price=$(echo "$status_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('auction',{}).get('current_price',1000))" 2>/dev/null || echo "$base_price")
    local current_leader=$(echo "$status_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('auction',{}).get('current_leader',{}).get('id',0) or 0)" 2>/dev/null || echo "0")

    # 出价 = 当前价 + 加价幅度 + 随机溢价
    local amount=$((current_price + step + (RANDOM % 10) * step))
    local idem_key="conc-${auction_id}-w${worker_id}-r${r}-$(date +%s%3N)"

    local resp=$(curl -sf -X POST "$API_BASE/api/auctions/$auction_id/bid" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $token" \
      -H "Idempotency-Key: $idem_key" \
      -d "{\"amount\": $amount}" 2>&1 || echo "NETWORK_ERROR")

    local code=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('code', -1))" 2>/dev/null || echo "-1")
    echo "$code" >> "$result_file"

    # 每次出价后短暂随机延迟，模拟真实用户行为
    sleep "0.0$((RANDOM % 5 + 1))"
  done
}

# 启动并发
for i in $(seq 0 $((CONCURRENCY - 1))); do
  bid_worker "${BUYER_TOKENS[$i]}" "$AUCTION_ID" "$BIDS_PER_BUYER" "$i" &
done

# 等待全部完成
wait

# ── Step 4: 统计结果 ──
echo -e "\n${CYAN}══════ Step 4: 结果统计 ══════${NC}"

for i in $(seq 0 $((CONCURRENCY - 1))); do
  f="/tmp/concurrency-test-results-$i.txt"
  while read -r code; do
    if [ "$code" = "0" ]; then
      SUCCESS=$((SUCCESS + 1))
    elif [ "$code" = "2103" ] || [ "$code" = "1005" ]; then
      CONFLICT=$((CONFLICT + 1))
    else
      FAIL=$((FAIL + 1))
    fi
  done < "$f"
done

END_TIME=$(date +%s%3N)
ELAPSED=$((END_TIME - START_TIME))

echo ""
printf "  总请求数:       %d\n" $TOTAL
printf "  ${GREEN}出价成功:${NC}       %d (%.1f%%)\n" $SUCCESS $(echo "scale=1; $SUCCESS*100/$TOTAL" | bc 2>/dev/null || echo "0")
printf "  ${YELLOW}竞争冲突:${NC}       %d (%.1f%%)\n" $CONFLICT $(echo "scale=1; $CONFLICT*100/$TOTAL" | bc 2>/dev/null || echo "0")
printf "  ${RED}失败:${NC}           %d\n" $FAIL
printf "  总耗时:         %.2f 秒\n" $(echo "scale=2; $ELAPSED/1000" | bc 2>/dev/null || echo "0")
printf "  QPS:            %.1f\n" $(echo "scale=1; $TOTAL*1000/$ELAPSED" | bc 2>/dev/null || echo "0")

# ── Step 5: 查看拍卖最终状态 ──
echo -e "\n${CYAN}══════ Step 5: 拍卖最终状态 ══════${NC}"

mysql -uauction -pauction_root -e "
SELECT id, title, status, current_price, bid_count, version
  FROM auction.auctions WHERE id=$AUCTION_ID;
" 2>/dev/null

echo ""
mysql -uauction -pauction_root -e "
SELECT u.nickname, COUNT(*) as bid_count
  FROM auction.bids b
  JOIN auction.users u ON u.id = b.user_id
 WHERE b.auction_id = $AUCTION_ID AND b.status = 'accepted'
 GROUP BY b.user_id
 ORDER BY bid_count DESC
 LIMIT 10;
" 2>/dev/null

echo ""
log "测试完成！刷新浏览器查看拍卖 ID=$AUCTION_ID 的竞拍结果"
