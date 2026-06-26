#!/usr/bin/env bash
# =============================================================
# k6 性能压测一键执行脚本（在 WSL 中运行）
# 用法: bash scripts/run-k6-perf.sh
#
# 完成后将压测结果写入: scripts/k6-report-latest.txt
# =============================================================
set -euo pipefail

API_BASE="http://127.0.0.1:8080"
WS_BASE="ws://127.0.0.1:8080"
TOKEN_FILE="/tmp/k6-buyer-tokens.json"
REPORT_FILE="$(dirname "$0")/k6-report-latest.txt"
BUYER_COUNT=200
STEP=100
HUGE_CEILING=999999999   # 防止拍卖在测试中途因触顶而结束

# ── 颜色 ──────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'
YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
fail() { echo -e "${RED}[✗]${NC} $1"; exit 1; }

# ── 依赖检查 ────────────────────────────────────────────────
command -v k6     >/dev/null 2>&1 || fail "k6 未安装，请先运行: sudo apt-get install k6"
command -v curl   >/dev/null 2>&1 || fail "curl 未安装"
command -v python3>/dev/null 2>&1 || fail "python3 未安装"
command -v mysql  >/dev/null 2>&1 || warn "mysql-client 未安装，将跳过激活步骤（需手动激活拍卖）"
HAS_MYSQL=$(command -v mysql >/dev/null 2>&1 && echo yes || echo no)

# ── Step 1: 健康检查 ──────────────────────────────────────
echo -e "\n${CYAN}══════ Step 1: 后端健康检查 ══════${NC}"
HEALTH=$(curl -sf "$API_BASE/health" || echo "FAIL")
[[ "$HEALTH" == *"ok"* ]] || fail "后端未启动，请先运行 auction server"
log "后端正常: $HEALTH"

# ── Step 2: 注册卖家并创建拍卖 ───────────────────────────
echo -e "\n${CYAN}══════ Step 2: 创建压测拍卖 ══════${NC}"

SELLER_RESP=$(curl -sf -X POST "$API_BASE/api/login" \
  -H "Content-Type: application/json" \
  -d '{"nickname":"k6-压测卖家"}')
SELLER_TOKEN=$(echo "$SELLER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")
[[ -n "$SELLER_TOKEN" ]] || fail "卖家登录失败: $SELLER_RESP"
log "卖家 token: ${SELLER_TOKEN:0:20}..."

# 开始时间设为 10 秒后（给激活留窗口），结束时间 2 小时后
START_TIME=$(date -d '+10 seconds' '+%Y-%m-%dT%H:%M:%S+08:00' 2>/dev/null \
           || date -v+10S '+%Y-%m-%dT%H:%M:%S+08:00' 2>/dev/null \
           || python3 -c "
from datetime import datetime, timezone, timedelta
t = datetime.now(timezone(timedelta(hours=8))) + timedelta(seconds=10)
print(t.strftime('%Y-%m-%dT%H:%M:%S+08:00'))
")

AUCTION_RESP=$(curl -sf -X POST "$API_BASE/api/auctions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d "{
    \"title\": \"k6压测拍卖-$(date +%H%M%S)\",
    \"start_price\": 1000,
    \"price_step\": $STEP,
    \"start_time\": \"$START_TIME\",
    \"duration_seconds\": 7200,
    \"ceiling_price\": $HUGE_CEILING
  }")

AUCTION_ID=$(echo "$AUCTION_RESP" | python3 -c \
  "import sys,json; print(json.load(sys.stdin)['data']['auction']['id'])" 2>/dev/null || echo "")
[[ -n "$AUCTION_ID" ]] || fail "创建拍卖失败: $AUCTION_RESP"
log "拍卖已创建: ID=$AUCTION_ID"

# ── Step 3: 激活拍卖 ──────────────────────────────────────
echo -e "\n${CYAN}══════ Step 3: 激活拍卖 ══════${NC}"
if [[ "$HAS_MYSQL" == "yes" ]]; then
  mysql -uauction -pauction_root auction \
    -e "UPDATE auctions SET status='active', start_time=NOW() WHERE id=$AUCTION_ID;" 2>/dev/null \
    || warn "MySQL 激活失败，请手动执行: UPDATE auctions SET status='active' WHERE id=$AUCTION_ID"
  log "拍卖已通过 MySQL 激活"
else
  warn "无 mysql-client，请手动激活: UPDATE auctions SET status='active' WHERE id=$AUCTION_ID"
  warn "等待 15s 由生命周期 Worker 自动激活..."
  sleep 15
fi

# 验证激活状态
STATUS_RESP=$(curl -sf "$API_BASE/api/auctions/$AUCTION_ID/status" || echo "{}")
AUCTION_STATUS=$(echo "$STATUS_RESP" | python3 -c \
  "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('auction',{}).get('status','unknown'))" 2>/dev/null || echo "unknown")
[[ "$AUCTION_STATUS" == "active" ]] || warn "拍卖状态=$AUCTION_STATUS（未激活，出价可能失败）"
log "拍卖状态: $AUCTION_STATUS"

# ── Step 4: 注册买家 ──────────────────────────────────────
echo -e "\n${CYAN}══════ Step 4: 注册 ${BUYER_COUNT} 个买家 ══════${NC}"

TOKENS="["
for i in $(seq 1 $BUYER_COUNT); do
  NICK="k6买家$(printf "%03d" $i)"
  TOK=$(curl -sf -X POST "$API_BASE/api/login" \
    -H "Content-Type: application/json" \
    -d "{\"nickname\":\"$NICK\"}" | \
    python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])" 2>/dev/null || echo "")
  [[ -n "$TOK" ]] || { warn "买家 #$i 注册失败"; continue; }
  [[ "$i" -gt 1 ]] && TOKENS+=","
  TOKENS+="\"$TOK\""
  # 每 10 个打一个点
  [[ $((i % 10)) -eq 0 ]] && echo -n "  [$i/$BUYER_COUNT]..."
done
TOKENS+="]"
echo ""

echo "$TOKENS" > "$TOKEN_FILE"
TOKEN_COUNT=$(echo "$TOKENS" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))")
log "$TOKEN_COUNT 个买家 token 已写入 $TOKEN_FILE"

# ── Step 5: 运行 k6 ────────────────────────────────────────
echo -e "\n${CYAN}══════ Step 5: 运行 k6 压测 ══════${NC}"
echo "  拍卖 ID : $AUCTION_ID"
echo "  买家数  : $TOKEN_COUNT"
echo "  报告输出: $REPORT_FILE"
echo ""

K6_SCRIPT="$(dirname "$0")/k6-perf-test.js"
[[ -f "$K6_SCRIPT" ]] || fail "k6 脚本不存在: $K6_SCRIPT"

k6 run \
  --env API_BASE="$API_BASE" \
  --env WS_BASE="$WS_BASE" \
  --env AUCTION_ID="$AUCTION_ID" \
  --env TOKEN_FILE="$TOKEN_FILE" \
  --summary-trend-stats="min,avg,med,p(90),p(95),p(99),max,count" \
  "$K6_SCRIPT" \
  2>&1 | tee "$REPORT_FILE"

echo ""
log "压测完成，报告已保存至 $REPORT_FILE"

# ── Step 6: 超卖与重复订单验证 ───────────────────────────
echo -e "\n${CYAN}══════ Step 6: 超卖 / 重复订单验证 ══════${NC}"
if [[ "$HAS_MYSQL" == "yes" ]]; then
  echo "  出价记录（已接受，按价格排序前 10）:"
  mysql -uauction -pauction_root auction 2>/dev/null -e "
    SELECT b.id, b.amount, b.user_id, b.created_at
      FROM bids b
     WHERE b.auction_id = $AUCTION_ID AND b.status = 'accepted'
     ORDER BY b.created_at
     LIMIT 10;
  " || warn "查询出价记录失败"

  echo ""
  echo "  价格单调性检查（若有非单调行则超卖）:"
  mysql -uauction -pauction_root auction 2>/dev/null -e "
    SELECT COUNT(*) AS non_monotonic_count
      FROM (
        SELECT amount,
               LAG(amount) OVER (ORDER BY id) AS prev_amount
          FROM bids
         WHERE auction_id = $AUCTION_ID AND status = 'accepted'
      ) t
     WHERE amount <= prev_amount;
  " || warn "查询单调性失败"

  echo ""
  echo "  重复订单检查（同一 auction+user 多条 order）:"
  mysql -uauction -pauction_root auction 2>/dev/null -e "
    SELECT auction_id, buyer_id, COUNT(*) AS dup_count
      FROM orders
     WHERE auction_id = $AUCTION_ID
     GROUP BY auction_id, buyer_id
    HAVING dup_count > 1;
  " || warn "查询重复订单失败"
else
  warn "无 mysql-client，跳过数据库验证"
fi

echo ""
log "全部验证完成 ✔"
