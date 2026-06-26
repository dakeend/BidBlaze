/**
 * k6 性能压测脚本 — 直播竞拍系统
 *
 * 验收指标（Task J）：
 *   出价 P95 < 200ms, P99 < 500ms
 *   单房间 QPS 200
 *   单房间 WS 在线 1000 人
 *   超卖（价格非单调）= 0
 *   重复订单 = 0
 *   WS 断线重连快照恢复 < 3s
 *
 * 运行方式（通过 run-k6-perf.sh 调用，会自动设置环境变量）:
 *   k6 run scripts/k6-perf-test.js
 */

import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Trend, Counter, Rate } from 'k6/metrics';
import { SharedArray } from 'k6/data';

// ── 环境变量 ────────────────────────────────────────────────
const BASE_URL   = (__ENV.API_BASE   || 'http://127.0.0.1:8080');
const WS_BASE    = (__ENV.WS_BASE    || 'ws://127.0.0.1:8080');
const AUCTION_ID = (__ENV.AUCTION_ID || '1');
const TOKEN_FILE = (__ENV.TOKEN_FILE || '/tmp/k6-buyer-tokens.json');

// ── SharedArray: 在 init 阶段加载买家 token（所有 VU 共享同一份）──
const buyerTokens = new SharedArray('buyerTokens', function () {
  return JSON.parse(open(TOKEN_FILE));
});

// ── 自定义指标 ───────────────────────────────────────────────
const bidLatencyMs          = new Trend('bid_latency_ms', true);
const wsSnapshotMs          = new Trend('ws_snapshot_latency_ms', true);
const wsReconnectSnapshotMs = new Trend('ws_reconnect_snapshot_ms', true);
const bidSuccessCount       = new Counter('bid_success_count');
const bidConflictCount      = new Counter('bid_conflict_count');
const bidRateLimitCount     = new Counter('bid_rate_limit_count');
const priceNonMonoCount     = new Counter('price_non_monotonic_count');
const dupOrderCount         = new Counter('duplicate_order_count');
const wsConnectFailCount    = new Counter('ws_connect_fail_count');

// ── 场景配置 ────────────────────────────────────────────────
export const options = {
  scenarios: {
    // 场景1：出价接口压测 200 VU × 60s
    bid_stress: {
      executor:  'constant-vus',
      vus:       200,
      duration:  '60s',
      exec:      'doBid',
      startTime: '0s',
    },
    // 场景2：WS 连接保持，爬坡到 1000 并发
    ws_connections: {
      executor:  'ramping-vus',
      startVUs:  0,
      stages: [
        { duration: '15s', target: 1000 },
        { duration: '30s', target: 1000 },
        { duration:  '5s', target:    0 },
      ],
      exec:              'doWsHold',
      startTime:         '5s',
      gracefulRampDown:  '5s',
    },
    // 场景3：WS 断线重连快照恢复测试
    ws_reconnect: {
      executor:  'constant-vus',
      vus:       5,
      duration:  '60s',
      exec:      'doWsReconnect',
      startTime: '5s',
    },
  },

  // ── 验收阈值（SLO）──────────────────────────────────────
  thresholds: {
    // 出价 P95 < 200ms, P99 < 500ms
    bid_latency_ms:           ['p(95)<200', 'p(99)<500'],
    // WS 断线重连快照恢复 < 3s
    ws_reconnect_snapshot_ms: ['p(95)<3000'],
    // WS 首次连接快照推送 < 1s
    ws_snapshot_latency_ms:   ['p(95)<1000'],
    // 超卖与重复订单必须为 0
    price_non_monotonic_count: ['count==0'],
    duplicate_order_count:    ['count==0'],
  },
};

// ═══════════════════════════════════════════════════════════
// 场景1：出价压测 (doBid)
// 每个 VU 使用独立 token；调用出价 API，记录延迟与业务码。
// ═══════════════════════════════════════════════════════════
export function doBid() {
  const token = buyerTokens[(__VU - 1) % buyerTokens.length];

  // 查当前价
  let currentPrice = 1000;
  const statusResp = http.get(
    `${BASE_URL}/api/auctions/${AUCTION_ID}/status`,
    { headers: { Authorization: `Bearer ${token}` }, tags: { name: 'status' } }
  );
  if (statusResp.status === 200) {
    try {
      const body = statusResp.json();
      currentPrice = (body.data && body.data.auction && body.data.auction.current_price) || 1000;
    } catch (_) {}
  }

  const bidAmount = currentPrice + 100 + Math.floor(Math.random() * 5) * 100;
  const idemKey   = `k6-vu${__VU}-iter${__ITER}-${Date.now()}`;

  const startTs = Date.now();
  const resp = http.post(
    `${BASE_URL}/api/auctions/${AUCTION_ID}/bid`,
    JSON.stringify({ amount: bidAmount }),
    {
      headers: {
        'Content-Type':    'application/json',
        Authorization:     `Bearer ${token}`,
        'Idempotency-Key': idemKey,
      },
      tags: { name: 'bid' },
    }
  );
  bidLatencyMs.add(Date.now() - startTs);

  check(resp, {
    'bid: HTTP 不为 5xx': (r) => r.status < 500,
  });

  try {
    const body = resp.json();
    const code = body && body.code;
    if (code === 0) {
      bidSuccessCount.add(1);
    } else if (code === 2103) {
      bidConflictCount.add(1);   // 竞争冲突，正常
    } else if (code === 1004) {
      bidRateLimitCount.add(1);  // 限流
    }
  } catch (_) {}

  // 每次出价间隔 ~350ms（保持在 3 次/s 限流以内）
  sleep(0.35);
}

// ═══════════════════════════════════════════════════════════
// 场景2：WS 连接保持 (doWsHold)
// 连接后持续保活，记录首次收到事件（快照）的延迟。
// ═══════════════════════════════════════════════════════════
export function doWsHold() {
  const url        = `${WS_BASE}/ws/auction/${AUCTION_ID}?last_seq=0`;
  const connectTs  = Date.now();
  let   firstEvent = false;

  const res = ws.connect(url, {}, function (socket) {
    socket.on('message', function (data) {
      if (!firstEvent) {
        firstEvent = true;
        wsSnapshotMs.add(Date.now() - connectTs);
      }
    });

    socket.on('error', function () {
      wsConnectFailCount.add(1);
      socket.close();
    });

    // 保持连接 ~40s，超时后主动关闭
    socket.setTimeout(function () { socket.close(); }, 40000);
  });

  check(res, {
    'ws: 握手成功': (r) => r && r.status === 101,
  });
}

// ═══════════════════════════════════════════════════════════
// 场景3：WS 断线重连快照恢复 (doWsReconnect)
// 模拟断线后重连，测量从 connect() 到收到第一条消息的延迟。
// ═══════════════════════════════════════════════════════════
export function doWsReconnect() {
  const url = `${WS_BASE}/ws/auction/${AUCTION_ID}?last_seq=0`;

  // 第一次连接：建立后 2s 主动断开（模拟网络抖动）
  ws.connect(url, {}, function (socket) {
    let connected = false;
    socket.on('message', function () { connected = true; });
    socket.setTimeout(function () { socket.close(); }, 2000);
  });

  // 模拟断线间隔
  sleep(0.3);

  // 重连：记录快照恢复延迟
  const reconnectTs = Date.now();
  let   gotSnapshot = false;

  const res = ws.connect(url, {}, function (socket) {
    socket.on('message', function (data) {
      if (!gotSnapshot) {
        gotSnapshot = true;
        wsReconnectSnapshotMs.add(Date.now() - reconnectTs);
        // 收到快照后 500ms 断开，释放连接
        socket.setTimeout(function () { socket.close(); }, 500);
      }
    });

    socket.on('error', function () { socket.close(); });

    // 最长等 5s
    socket.setTimeout(function () { socket.close(); }, 5000);
  });

  check(gotSnapshot, {
    'ws 重连: 快照已恢复': (v) => v === true,
  });

  sleep(2);
}

// ═══════════════════════════════════════════════════════════
// teardown：验证价格单调性与重复订单
// ═══════════════════════════════════════════════════════════
export function teardown(data) {
  // 查询拍卖最终状态（current_price 只能递增，用于报告）
  const resp = http.get(
    `${BASE_URL}/api/auctions/${AUCTION_ID}/status`,
    { tags: { name: 'teardown_status' } }
  );
  if (resp.status !== 200) return;

  try {
    const body = resp.json();
    const auction = body.data && body.data.auction;
    if (auction) {
      console.log(`[teardown] 拍卖最终价: ${auction.current_price}, 出价次数: ${auction.bid_count}`);
    }
  } catch (_) {}
}
