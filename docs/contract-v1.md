> ⚠️ **DEPRECATED — 请勿使用。** 本文件（含其 A=前端/B=后端/C 维护合同的旧分工）已被
> **`docs/contract-v2.md`** 取代。v2 的角色分工见 `docs/team-assignment.md`（Role A=后端核心 / B=实时+移动端 / C=PC 后台+打磨）。
> 唯一接口事实源是 `contract-v2.md` + `docs/api/openapi.yaml`。保留本文件仅作历史参考。

---

# 直播竞拍系统 · 接口与协议合同 v1（DEPRECATED）

> 本文件是 **A（前端）/ B（后端）共同依赖的合同**，由 C 维护。
> 接口/字段有任何变动，C 必须先改本文，再通知 A、B。
>
> 锁定日期：2026-05-29 · 适用版本：V1 ~ V4

---

## 0. 全局约定

| 项 | 约定 |
|---|---|
| 后端 | Go + Gin + gorilla/websocket |
| 数据库 | MySQL 8.0 (持久化) + Redis 7 (分布式锁/缓存) |
| HTTP 前缀 | `/api` |
| WebSocket 前缀 | `/ws` |
| 编码 | 请求 / 响应均为 JSON, `utf-8` |
| 金额单位 | **分 (int)**, 前端展示时除以 100 |
| 时间格式 | ISO-8601 字符串, 例 `2026-05-29T20:00:00+08:00` |
| 鉴权 | mock 登录, `Authorization: Bearer <token>` |
| 时区 | 服务端、数据库统一 `Asia/Shanghai` |

### 0.1 统一响应格式

**HTTP 状态码恒为 200**（除非服务器崩溃）。业务结果用 `code` 字段表达。

```json
{
  "code": 0,
  "msg": "ok",
  "data": { /* 业务数据, 失败时为 null */ }
}
```

### 0.2 错误码表

| code | 含义 | 前端建议处理 |
|------|------|--------------|
| 0    | 成功 | — |
| 1001 | 参数错误 | toast 错误信息 |
| 1002 | 未登录 / token 失效 | 跳登录页 |
| 1003 | 昵称已被占用 | 表单内联报错 |
| 2001 | 拍卖不存在 | 返回列表 |
| 2002 | 拍卖未开始 | 禁用出价按钮 |
| 2003 | 拍卖已结束 | 展示成交结果 |
| 2004 | 拍卖已取消 | toast 提示 |
| 2101 | 出价低于「当前价 + 加价幅度」 | 预填新价并提示 |
| 2102 | 出价超过封顶价 | 提示并自动按封顶价提交 |
| 2103 | 出价冲突（抢锁失败） | **自动重试 1 次**, 再失败提示用户 |
| 3001 | 订单不存在 | 返回列表 |
| 9999 | 服务器内部错误 | toast「系统繁忙」 |

---

## 1. REST 接口

### 1.1 认证

#### `POST /api/login`

mock 登录。昵称首次使用即注册，再次使用同一昵称返回旧 token。

**请求体**
```json
{ "nickname": "买家张三", "avatar": null }
```

**响应**
```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "token": "mock-token-user-001",
    "user": { "id": 2, "nickname": "买家张三", "avatar": null }
  }
}
```

错误：`1001` 昵称为空 / 超长；`1003` 昵称已被占用且 token 不匹配。

---

### 1.2 拍卖（商家）

#### `POST /api/auctions` 创建拍卖

需登录。请求方即 `seller_id`。

**请求体**
```json
{
  "title": "天然翡翠吊坠",
  "cover_url": "https://...",
  "start_price": 90000,
  "price_step": 5000,
  "ceiling_price": 500000,
  "start_time": "2026-05-29T20:00:00+08:00",
  "duration_seconds": 300,
  "extend_seconds": 30,
  "extend_threshold": 30
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `title` | ✅ | ≤128 字符 |
| `cover_url` | ❌ | 商品图 |
| `start_price` | ✅ | 起拍价（分）, >0 |
| `price_step` | ✅ | 最小加价幅度（分）, >0 |
| `ceiling_price` | ❌ | 封顶价（分）, 不传=无封顶 |
| `start_time` | ✅ | ISO-8601, 必须晚于服务端当前时间 |
| `duration_seconds` | ✅ | 拍卖时长（秒）, 后端算 `end_time = start_time + duration_seconds` |
| `extend_seconds` | ❌ | 默认 30 |
| `extend_threshold` | ❌ | 默认 30 |

**响应**：返回 `data.auction`（结构见 1.6）。

---

#### `GET /api/auctions` 拍卖列表

**Query**

| 参数 | 必填 | 说明 |
|------|------|------|
| `status` | ❌ | `pending`/`active`/`ended`/`cancelled`，不传=全部 |
| `seller_id` | ❌ | 按商家过滤 |
| `page` | ❌ | 从 1 开始，默认 1 |
| `size` | ❌ | 默认 20，最大 100 |

**响应**
```json
{
  "code": 0, "msg": "ok",
  "data": {
    "list": [ /* Auction[] */ ],
    "total": 42,
    "page": 1,
    "size": 20
  }
}
```

---

#### `GET /api/auctions/:id` 拍卖详情

**响应**：`data.auction`（结构见 1.6）。

---

#### `POST /api/auctions/:id/cancel` 取消拍卖

需登录，仅 `seller_id == 当前 user` 可调用。仅 `pending` / `active` 状态可取消。

**响应**：返回更新后的 `data.auction`。同时通过 WS 广播 `auction_cancelled`。

错误：`2001` 不存在；`1002` 非本人；`2003`/`2004` 状态不允许。

---

### 1.3 竞拍

#### `POST /api/auctions/:id/bid` 出价

**这是评分核心接口**。后端流程：
1. 校验 token、拍卖状态、`amount` 范围
2. `SET NX EX 3 bid_lock:{auctionId} {requestId}` 抢锁
3. 抢到锁 → DB 事务：插入 `bids(accepted)` + 更新 `auctions.current_price/leader`
4. 检查是否触发延时（`end_time - now < extend_threshold` 时 `end_time += extend_seconds`）
5. 释放锁（Lua 校验 requestId）
6. WS 广播 `bid_update`（必发）和 `auction_extended`（条件）
7. 抢锁失败 → 插入 `bids(rejected, reject_reason=lock_failed)`，返回 `code: 2103`

**请求体**
```json
{ "amount": 95000 }
```

**响应（成功）**
```json
{
  "code": 0, "msg": "ok",
  "data": {
    "bid": {
      "id": 123, "auction_id": 1, "user_id": 2,
      "amount": 95000, "status": "accepted",
      "created_at": "2026-05-29T20:01:23+08:00"
    },
    "current_price": 95000,
    "current_leader": { "id": 2, "nickname": "买家张三", "avatar": null },
    "extended": false,
    "new_end_time": "2026-05-29T20:05:00+08:00"
  }
}
```

**响应（业务失败）**
```json
{
  "code": 2101,
  "msg": "出价低于当前价+加价幅度",
  "data": null
}
```

校验顺序（前一个失败立即返回，不进入抢锁）：
- 未登录 → 1002
- 拍卖不存在 → 2001
- `status=pending` → 2002
- `status=ended` → 2003
- `status=cancelled` → 2004
- `amount < current_price + price_step` → 2101
- `ceiling_price != null && amount > ceiling_price` → 2102
- 抢锁失败 → 2103

---

#### `GET /api/auctions/:id/bids` 排行榜历史

**Query**

| 参数 | 必填 | 说明 |
|------|------|------|
| `limit` | ❌ | 默认 20，最大 50。只返回 `status=accepted` |

**响应**
```json
{
  "code": 0, "msg": "ok",
  "data": {
    "list": [
      {
        "id": 123, "user": { "id": 2, "nickname": "买家张三", "avatar": null },
        "amount": 95000, "created_at": "2026-05-29T20:01:23+08:00"
      }
    ]
  }
}
```

---

#### `GET /api/auctions/:id/status` 拍卖状态（WS 降级方案）

WS 连不上时前端短轮询。返回内容同 WS 的 `snapshot` payload。

```json
{
  "code": 0, "msg": "ok",
  "data": {
    "auction": { /* Auction */ },
    "top_bids": [ /* 同 1.3.bids，前 10 */ ],
    "server_time": "2026-05-29T20:01:23.456+08:00"
  }
}
```

---

### 1.4 订单

#### `GET /api/orders/mine` 我作为买家的订单

需登录。Query: `status` 可选。返回 `data.list`。

#### `GET /api/orders/seller` 我作为商家的订单

需登录。Query: `status` 可选。返回 `data.list`。

#### `GET /api/orders/:id` 订单详情

需登录。买家或商家本人可看。

#### `POST /api/orders/:id/pay` 模拟支付

需登录，仅 `winner_id == 当前 user`。直接置 `status='paid'`、`paid_at=now`。

---

### 1.5 用户

#### `GET /api/users/me`

需登录。返回 `data.user`。

---

### 1.6 通用数据结构

#### `Auction`
```json
{
  "id": 1,
  "title": "天然翡翠吊坠",
  "cover_url": "https://...",
  "start_price": 90000,
  "price_step": 5000,
  "ceiling_price": 500000,
  "current_price": 95000,
  "current_leader": { "id": 2, "nickname": "买家张三", "avatar": null },
  "start_time": "2026-05-29T20:00:00+08:00",
  "end_time": "2026-05-29T20:05:30+08:00",
  "original_end_time": "2026-05-29T20:05:00+08:00",
  "extend_seconds": 30,
  "extend_threshold": 30,
  "status": "active",
  "seller": { "id": 1, "nickname": "主播阿明", "avatar": null },
  "created_at": "2026-05-29T19:30:00+08:00"
}
```

#### `Bid`
```json
{
  "id": 123,
  "auction_id": 1,
  "user": { "id": 2, "nickname": "买家张三", "avatar": null },
  "amount": 95000,
  "status": "accepted",
  "created_at": "2026-05-29T20:01:23+08:00"
}
```

#### `Order`
```json
{
  "id": 7,
  "auction_id": 1,
  "winner": { "id": 2, "nickname": "买家张三", "avatar": null },
  "seller": { "id": 1, "nickname": "主播阿明", "avatar": null },
  "final_price": 95000,
  "status": "pending_pay",
  "created_at": "2026-05-29T20:05:30+08:00",
  "paid_at": null
}
```

#### `UserBrief`
```json
{ "id": 2, "nickname": "买家张三", "avatar": null }
```

---

### 1.7 健康检查

`GET /health` → `{ "message": "Go server is running" }`

---

## 2. WebSocket 协议

### 2.1 连接

```
ws://<host>/ws/auction/:id?token=<token>
```

- 路径中的 `:id` 即房间 ID（一场拍卖一房间）
- `token` 走 query string（浏览器 WS 不支持自定义 header）
- 无 token = 匿名只读，仍可收推送
- 连接建立后**服务端立即推送一条 `snapshot`**

### 2.2 消息信封

所有消息均为：
```json
{ "type": "<事件名>", "data": { /* 事件 payload */ } }
```

### 2.3 客户端 → 服务端

#### `ping`（心跳）
```json
{ "type": "ping" }
```
- 客户端 **每 30 秒**发一次
- 服务端 60 秒未收到 ping 则关闭连接
- 服务端立即回 `{ "type": "pong" }`

### 2.4 服务端 → 客户端

#### `snapshot` — 连上即推
```json
{
  "type": "snapshot",
  "data": {
    "auction": { /* Auction */ },
    "top_bids": [ /* Bid[], 前 10, 按 amount desc */ ],
    "server_time": "2026-05-29T20:01:23.456+08:00"
  }
}
```

前端用法：
```js
const localOffset = new Date(server_time).getTime() - Date.now();
// 后续倒计时按 (Date.now() + localOffset) 算
```

#### `bid_update` — 有人出价成功
```json
{
  "type": "bid_update",
  "data": {
    "current_price": 95000,
    "current_leader": { "id": 2, "nickname": "买家张三", "avatar": null },
    "latest_bid": { /* Bid */ },
    "top_bids": [ /* Bid[], 前 10 */ ]
  }
}
```

> 失败出价**不广播**，只在该出价者的 HTTP 响应中体现。

#### `auction_extended` — 触发延时
```json
{
  "type": "auction_extended",
  "data": {
    "new_end_time": "2026-05-29T20:06:00+08:00",
    "extended_seconds": 30
  }
}
```

前端必须**根据 `new_end_time` 校正倒计时**，不要靠本地累加。

#### `auction_started` — pending → active
```json
{
  "type": "auction_started",
  "data": { "auction": { /* Auction */ } }
}
```

#### `auction_ended` — 自然结束
```json
{
  "type": "auction_ended",
  "data": {
    "auction": { /* Auction, status=ended */ },
    "winner": { "id": 2, "nickname": "买家张三", "avatar": null },
    "final_price": 95000,
    "order_id": 7
  }
}
```

无人出价（流拍）时 `winner=null`, `final_price=null`, `order_id=null`。

#### `auction_cancelled` — 商家取消
```json
{
  "type": "auction_cancelled",
  "data": {
    "auction": { /* Auction, status=cancelled */ },
    "reason": "seller_cancelled"
  }
}
```

---

## 3. 状态机

```
                 ┌──────────┐
   POST /api/auctions ───→  │ pending  │
                            └────┬─────┘
              start_time ≤ now   │   (后端 1s ticker 扫表)
                                 ↓
                            ┌──────────┐    出价 && (end_time - now) < extend_threshold
              ┌──────────→ │  active  │ ─────→ end_time += extend_seconds
              │            └────┬─────┘        广播 auction_extended
   延时回到本身                  │
              │                  │  end_time ≤ now → 写 orders → 广播 auction_ended
              │                  │  商家 cancel → 广播 auction_cancelled
              │                  ↓
              │            ┌───────────┐
              └────────────│  ended /  │  (终态)
                           │ cancelled │
                           └───────────┘
```

**后端 ticker 实现要点**（B 参考）：
- 单进程内 1 个 goroutine, 1 秒一次
- `SELECT * FROM auctions WHERE status='pending' AND start_time <= NOW()` → 置 active + 广播 `auction_started`
- `SELECT * FROM auctions WHERE status='active'  AND end_time   <= NOW()` → 事务内：置 ended + 写 orders（如有 leader）+ 广播 `auction_ended`
- 整个流程必须加分布式锁 `auction_lifecycle:{id}`，防止与出价/取消并发

---

## 4. 鉴权细节

### 4.1 mock 登录规则

- 首次用某昵称 `POST /api/login` → 新建 user + 新发 token
- 再次用同昵称 → 返回该 user 的 **旧 token**（演示方便，不要求记忆 token）
- token 形如 `mock-token-{uuid}`，**永不过期**（V4 前不做过期处理）

### 4.2 HTTP 携带

```
Authorization: Bearer mock-token-xxx
```

### 4.3 WS 携带

```
ws://host/ws/auction/1?token=mock-token-xxx
```

### 4.4 需登录的接口

| 接口 | 是否需登录 |
|------|------|
| `POST /api/login` | ❌ |
| `GET /api/auctions*` | ❌ |
| `GET /health` | ❌ |
| 其余所有 | ✅ |

---

## 5. 高并发约定（B 的核心实现指南）

### 5.1 Redis 分布式锁

```
key:    bid_lock:{auctionId}
value:  uuid (用于安全释放)
cmd:    SET key uuid NX EX 3
释放:   Lua: if GET == uuid then DEL
```

### 5.2 出价事务

抢锁成功后单事务内：
```sql
BEGIN;
INSERT INTO bids (auction_id, user_id, amount, status) VALUES (?, ?, ?, 'accepted');
UPDATE auctions
   SET current_price = ?, current_leader_id = ?, end_time = ?, updated_at = NOW()
 WHERE id = ?
   AND status = 'active'
   AND current_price < ?;   -- 双重保险, 防止极端时序
COMMIT;
```

`UPDATE` 影响行数为 0 → 回滚，返回 2103。

### 5.3 失败入库

抢锁失败 / 校验失败的出价也写一条：
```sql
INSERT INTO bids (auction_id, user_id, amount, status, reject_reason)
VALUES (?, ?, ?, 'rejected', 'lock_failed');
```

> 写入失败入库**不影响**用户 HTTP 响应耗时，B 可以用 goroutine 异步写。

### 5.4 数据清理

后台每天凌晨清一次：
```sql
DELETE FROM bids
 WHERE status = 'rejected'
   AND created_at < NOW() - INTERVAL 7 DAY;
```

---

## 6. 变更日志

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-05-29 | v1.0 | 初版定稿（C） |

