#!/usr/bin/env bash
# 青岛大虾直播间 - 200条高并发竞拍测试
set -uo pipefail

CONCURRENCY=40          # 并发买家数
BIDS_PER_BUYER=5        # 每个买家出价轮数  (40×5=200 次)
API_BASE="http://localhost"
MYSQL="docker exec auction-mysql mysql -uroot -pauction_root_prod -s --skip-column-names auction"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
fail() { echo -e "${RED}[✗]${NC} $*"; }
hr()   { echo -e "${CYAN}$(printf '═%.0s' {1..52})${NC}"; }

cleanup() { rm -f /tmp/ct-result-*.txt; }
trap cleanup EXIT

hr
echo -e "  🦐 ${CYAN}青岛大虾直播间 · 高并发竞拍压测${NC}"
echo -e "  并发买家: ${CONCURRENCY}  ×  每人出价: ${BIDS_PER_BUYER}  =  总请求: $((CONCURRENCY*BIDS_PER_BUYER))"
hr

# ── Step 1: 注册卖家「青岛大虾」并创建拍卖 ──
echo -e "\n${CYAN}[Step 1] 创建拍卖间${NC}"

SELLER_TOKEN=$(curl -sf -X POST "$API_BASE/api/login" \
  -H "Content-Type: application/json" \
  -d '{"nickname":"青岛大虾"}' | \
  python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")

log "卖家登录: token=$SELLER_TOKEN"

NOW=$(date -u +%Y-%m-%dT%H:%M:%S+08:00)
AUCTION_RESP=$(curl -sf -X POST "$API_BASE/api/auctions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d "{
    \"title\": \"青岛大虾·原味礼盒装\",
    \"description\": \"新鲜捕捞，当天发货，5斤装礼盒\",
    \"start_price\": 100,
    \"price_step\": 10,
    \"start_time\": \"$NOW\",
    \"duration_seconds\": 600
  }")

AUCTION_ID=$(echo "$AUCTION_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['auction']['id'])" 2>/dev/null)

if [ -z "$AUCTION_ID" ] || [ "$AUCTION_ID" = "None" ]; then
  fail "创建拍卖失败: $AUCTION_RESP"
  exit 1
fi
log "拍卖创建成功: ID=$AUCTION_ID"

# 强制激活（跳过定时器等待）
$MYSQL -e "UPDATE auctions SET status='active', start_time=NOW(), end_time=DATE_ADD(NOW(), INTERVAL 600 SECOND) WHERE id=$AUCTION_ID;" 2>/dev/null
log "拍卖已强制激活"

# 验证状态
STATUS=$(curl -sf "$API_BASE/api/auctions/$AUCTION_ID" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['auction']['status'])" 2>/dev/null)
log "当前状态: $STATUS"

# ── Step 2: 注册买家 ──
echo -e "\n${CYAN}[Step 2] 注册 ${CONCURRENCY} 个买家${NC}"

declare -a BUYER_TOKENS
for i in $(seq 1 $CONCURRENCY); do
  NICK="买家$(printf "%03d" $i)"
  TOKEN=$(curl -sf -X POST "$API_BASE/api/login" \
    -H "Content-Type: application/json" \
    -d "{\"nickname\":\"$NICK\"}" | \
    python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])" 2>/dev/null || echo "")
  BUYER_TOKENS+=("$TOKEN")
done
log "${#BUYER_TOKENS[@]} 个买家就绪"

# ── Step 3: 并发出价 ──
echo -e "\n${CYAN}[Step 3] 启动并发出价...${NC}"

bid_worker() {
  local token="$1"
  local auction_id="$2"
  local rounds="$3"
  local wid="$4"
  local out="/tmp/ct-result-${wid}.txt"

  for r in $(seq 1 $rounds); do
    # 读取当前价格
    local cur=$(curl -sf "$API_BASE/api/auctions/$auction_id/status" 2>/dev/null | \
      python3 -c "import sys,json; d=json.load(sys.stdin)['data']['auction']; print(d.get('current_price',100))" 2>/dev/null || echo "100")

    # 出价 = 当前价 + step + 随机溢价 (0~5步)
    local amount=$((cur + 10 + (RANDOM % 6) * 10))
    local ikey="qdx-${auction_id}-w${wid}-r${r}-${RANDOM}"

    local resp=$(curl -sf -X POST "$API_BASE/api/auctions/$auction_id/bid" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $token" \
      -H "Idempotency-Key: $ikey" \
      -d "{\"amount\":$amount}" 2>/dev/null || echo '{"code":-1}')

    local code=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('code',-1))" 2>/dev/null || echo "-1")
    echo "$code" >> "$out"

    # 随机短暂间隔，模拟真实用户
    sleep "0.0$((RANDOM % 8 + 1))"
  done
}

START_MS=$(date +%s%3N)

for i in $(seq 0 $((CONCURRENCY - 1))); do
  bid_worker "${BUYER_TOKENS[$i]}" "$AUCTION_ID" "$BIDS_PER_BUYER" "$i" &
done

wait

END_MS=$(date +%s%3N)
ELAPSED_MS=$((END_MS - START_MS))

# ── Step 4: 统计 ──
echo -e "\n${CYAN}[Step 4] 统计结果${NC}"

SUCCESS=0; CONFLICT=0; FAIL=0; TOTAL=0

for i in $(seq 0 $((CONCURRENCY - 1))); do
  f="/tmp/ct-result-${i}.txt"
  [ -f "$f" ] || continue
  while IFS= read -r code; do
    TOTAL=$((TOTAL + 1))
    case "$code" in
      0)                          SUCCESS=$((SUCCESS + 1)) ;;
      2103|1005|2102|2101|4029)   CONFLICT=$((CONFLICT + 1)) ;;
      *)                          FAIL=$((FAIL + 1)) ;;
    esac
  done < "$f"
done

ELAPSED_SEC=$(echo "scale=2; $ELAPSED_MS/1000" | bc)
QPS=$(echo "scale=1; $TOTAL*1000/$ELAPSED_MS" | bc 2>/dev/null || echo "?")
SUCCESS_PCT=$(echo "scale=1; $SUCCESS*100/$TOTAL" | bc 2>/dev/null || echo "?")
CONFLICT_PCT=$(echo "scale=1; $CONFLICT*100/$TOTAL" | bc 2>/dev/null || echo "?")

hr
echo -e "  📊 压测报告"
hr
printf "  总请求数   : %d\n" $TOTAL
printf "  ${GREEN}出价成功${NC}   : %d  (%.1f%%)\n" $SUCCESS $SUCCESS_PCT
printf "  ${YELLOW}竞争冲突${NC}   : %d  (%.1f%%)  ← 正常：被人抢先，乐观锁拒绝\n" $CONFLICT $CONFLICT_PCT
printf "  ${RED}失败${NC}       : %d\n" $FAIL
printf "  总耗时     : %s 秒\n" "$ELAPSED_SEC"
printf "  QPS        : %s 次/秒\n" "$QPS"
hr

# ── Step 5: 最终拍卖状态 ──
echo -e "\n${CYAN}[Step 5] 拍卖最终状态${NC}"

FINAL=$(curl -sf "$API_BASE/api/auctions/$AUCTION_ID")
echo "$FINAL" | python3 -c "
import sys, json
d = json.load(sys.stdin)['data']['auction']
print(f\"  标题        : {d['title']}\")
print(f\"  状态        : {d['status']}\")
print(f\"  最终价格    : ¥{d['current_price']}\")
print(f\"  领先买家    : {d['current_leader']['nickname']}\")
print(f\"  总出价次数  : {d['bid_count']}\")
print(f\"  版本号      : {d['version']}\")
"

echo -e "\n${CYAN}  Top 10 活跃买家${NC}"
$MYSQL -e "
SELECT u.nickname, COUNT(*) AS bids, MAX(b.amount) AS max_bid
  FROM bids b JOIN users u ON u.id=b.user_id
 WHERE b.auction_id=$AUCTION_ID AND b.status='accepted'
 GROUP BY b.user_id ORDER BY bids DESC LIMIT 10;
" 2>/dev/null | awk '{printf "  %-20s 成功出价: %s次  最高: ¥%s\n", $1, $2, $3}'

echo ""
log "压测完成！访问 http://114.55.252.115:8081 查看管理端竞拍详情 (拍卖ID: $AUCTION_ID)"
