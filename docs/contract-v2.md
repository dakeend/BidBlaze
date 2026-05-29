# 直播竞拍系统 · 接口与协议合同 v2

> 本文件是 Web 端、移动端、后端共同依赖的接口合同。
> 目标：支持高并发出价、弱网移动端、Web 实时看拍、多实例部署、可观测和可恢复的直播竞拍平台。
>
> 锁定日期：2026-05-29 · 适用版本：V2+

---

## 0. 设计目标

### 0.1 平台目标

- 高并发：同一拍卖房间内大量用户同时出价时，只接受满足规则的一笔最新有效出价。
- 高解耦：HTTP 接口、竞拍领域逻辑、WebSocket 推送、订单生成通过事件边界解耦。
- 稳定：服务多实例部署时，拍卖开始、结束、延时、订单生成不能重复或丢失。
- 多端一致：移动端和 Web 端都以服务端时间、服务端事件序号、快照恢复为准。

### 0.2 推荐后端边界

| 模块 | 职责 |
|---|---|
| API Service | 鉴权、参数校验、HTTP 响应 |
| Auction Domain | 出价校验、状态流转、延时规则、成交规则 |
| Storage | MySQL 事务、唯一约束、条件更新 |
| Event Outbox | 记录待发布领域事件，保证 DB 状态和事件一致 |
| Realtime Gateway | WebSocket 连接管理、房间广播、断线恢复 |
| Lifecycle Worker | 多实例安全地开始/结束拍卖、创建订单 |

### 0.3 后端框架与工程风格

本项目后端采用 **Gin + 手写清晰分层**，不引入 Go-kit、Kratos、go-zero 等重型微服务框架。框架选择服务于本课题的核心目标：高并发出价正确性、WebSocket 实时同步、弱网恢复和事务一致性。

推荐分层：

```text
HTTP/WebSocket Transport -> Application Service -> Domain Rules -> Repository/Storage
```

约束：

- Gin handler 只负责鉴权上下文读取、参数绑定、错误映射和响应，不写核心业务规则。
- Application Service 负责事务编排、幂等编排、outbox 写入、跨 repository 协作。
- Domain Rules 负责可单元测试的竞拍规则，例如最低有效价、封顶成交、延时判断、状态流转。
- Repository 负责 MySQL / Redis 访问，复杂一致性场景必须保留明确 SQL，避免 ORM 隐藏条件更新语义。
- WebSocket 网关只负责连接、心跳、房间广播、快照/补偿，不在 WS handler 内处理出价写入。
- Worker 只通过领域服务和 repository 推进状态，不依赖单进程内存 ticker 保证正确性。

推荐基础库：

| 能力 | 推荐 |
|---|---|
| HTTP | Gin |
| WebSocket | gorilla/websocket |
| MySQL | database/sql + sqlx；复杂事务手写 SQL |
| Redis | go-redis |
| 配置 | 环境变量优先，可用 viper |
| 日志 | Go `slog` 或 zap，字段化日志 |
| 测试 | Go testing + testify |
| 压测 | k6 或 Go 并发脚本 |

不推荐在 V2 阶段引入：

- Go-kit endpoint/transport 模板：会增加样板代码，但不能直接解决竞拍一致性问题。
- 重型微服务框架：本课题当前是单体模块化服务，优先把边界和事务做好。
- 全量 ORM 接管核心出价事务：出价条件更新、幂等、订单唯一约束需要显式 SQL 便于审计和压测解释。

---

## 1. 全局约定

| 项 | 约定 |
|---|---|
| 后端 | Go + Gin + gorilla/websocket，单体模块化分层 |
| 数据库 | MySQL 8.0 + Redis 7 |
| HTTP 前缀 | `/api` |
| WebSocket 前缀 | `/ws` |
| 编码 | JSON, `utf-8` |
| 金额单位 | 分，使用 int64 |
| 时间格式 | ISO-8601 字符串，例 `2026-05-29T20:00:00+08:00` |
| 时区 | 服务端、数据库统一 `Asia/Shanghai` |
| 鉴权 | V2 仍允许 mock token；生产版替换为真实登录态 |

### 1.1 HTTP 状态码

HTTP 状态码表达协议层结果，`code` 表达业务结果。

| HTTP | 场景 |
|---|---|
| 200 | 成功或业务可预期失败 |
| 400 | 请求参数格式错误 |
| 401 | 未登录或 token 无效 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 409 | 幂等冲突、状态冲突 |
| 429 | 限流 |
| 500 | 服务端异常 |
| 503 | 依赖不可用或系统保护 |

统一响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {}
}
```

### 1.2 错误码

| code | 含义 | HTTP | 前端建议 |
|---|---|---:|---|
| 0 | 成功 | 200 | 正常处理 |
| 1001 | 参数错误 | 400 | 表单提示 |
| 1002 | 未登录 / token 失效 | 401 | 跳登录 |
| 1003 | 无权限 | 403 | 提示无权限 |
| 1004 | 请求过于频繁 | 429 | 禁用按钮后重试 |
| 1005 | 幂等 key 冲突 | 409 | 提示刷新后重试 |
| 2001 | 拍卖不存在 | 404 | 返回列表 |
| 2002 | 拍卖未开始 | 200 | 禁用出价 |
| 2003 | 拍卖已结束 | 200 | 展示成交结果 |
| 2004 | 拍卖已取消 | 200 | toast 提示 |
| 2101 | 出价低于当前价 + 加价幅度 | 200 | 预填最低有效价 |
| 2102 | 出价超过封顶价 | 200 | 提示封顶价 |
| 2103 | 出价竞争失败 | 409 | 自动刷新快照，可重试一次 |
| 3001 | 订单不存在 | 404 | 返回订单列表 |
| 9001 | 系统保护中 | 503 | 提示稍后重试 |
| 9999 | 服务端内部错误 | 500 | toast「系统繁忙」 |

### 1.3 请求追踪

客户端每次写请求建议携带：

| Header | 必填 | 说明 |
|---|---|---|
| `Authorization` | 写接口必填 | `Bearer <token>` |
| `X-Request-Id` | 推荐 | 单次请求追踪 ID |
| `Idempotency-Key` | 出价、支付必填 | 同一业务操作的幂等 key |
| `X-Client-Type` | 推荐 | `web` / `mobile_h5` / `admin` |

---

## 2. REST 接口

### 2.1 认证

#### `POST /api/login`

mock 登录。昵称首次使用即注册，再次使用同一昵称返回旧 token。

请求：

```json
{ "nickname": "买家张三", "avatar": null }
```

响应：

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

### 2.2 拍卖

#### `POST /api/auctions` 创建拍卖

需登录。请求方即卖家。

```json
{
  "title": "天然翡翠吊坠",
  "description": "和田玉籽料，配 18K 金扣...",
  "cover_url": "https://...",
  "images": ["https://...", "https://..."],
  "stream_url": "https://live.example.com/room/1.flv",
  "start_price": 0,
  "price_step": 5000,
  "ceiling_price": 500000,
  "start_time": "2026-05-29T20:00:00+08:00",
  "duration_seconds": 300,
  "extend_seconds": 30,
  "extend_threshold": 30
}
```

校验：

| 字段 | 必填 | 规则 |
|---|---|---|
| `title` | 是 | 1 到 128 字符 |
| `description` | 否 | 0 到 2000 字符 |
| `cover_url` | 否 | URL 字符串 |
| `images` | 否 | URL 数组，最多 9 张 |
| `stream_url` | 否 | 直播流地址（flv/hls/mp4）；为空时前端用占位视频 |
| `start_price` | 是 | >= 0，支持 0 元起拍 |
| `price_step` | 是 | > 0 |
| `ceiling_price` | 否 | 为空表示无封顶；不为空时必须 >= `start_price` 且 > 0 |
| `start_time` | 是 | 必须晚于服务端当前时间 |
| `duration_seconds` | 是 | 30 到 86400 |
| `extend_seconds` | 否 | 默认 30，范围 10 到 30（PDF 规则约束）|
| `extend_threshold` | 否 | 默认 30，范围 1 到 300 |

首笔出价的最低有效价规则：

- 若尚无任何出价且 `start_price > 0`，首笔最低有效价 = `start_price`。
- 若尚无任何出价且 `start_price == 0`，首笔最低有效价 = `price_step`。
- 之后每笔最低有效价 = `current_price + price_step`。

响应：`data.auction`。

#### `PUT /api/auctions/:id` 修改未开始的拍卖

需登录，仅卖家本人。**仅 `status=pending` 可改**，已到达 `start_time` 的不允许修改。

请求体字段与创建一致，全部字段可选；只更新传入字段。`start_time` 修改后必须仍晚于服务端当前时间。

响应：`data.auction`（更新后的快照）。失败：

- `2002` 不是 pending 状态
- `1003` 非卖家
- `1001` 字段非法

无需写 outbox 事件（pending 拍卖尚未广播）。

#### `GET /api/auctions` 拍卖列表

Query：

| 参数 | 说明 |
|---|---|
| `status` | `pending` / `active` / `ended` / `cancelled` |
| `seller_id` | 按卖家过滤 |
| `page` | 默认 1 |
| `size` | 默认 20，最大 100 |

响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "list": [],
    "total": 42,
    "page": 1,
    "size": 20,
    "server_time": "2026-05-29T20:01:23.456+08:00"
  }
}
```

#### `GET /api/auctions/:id`

响应：`data.auction`。

#### `POST /api/auctions/:id/cancel`

需登录，仅卖家本人可取消。仅 `pending` / `active` 可取消。

成功后：

- 事务内更新 `auctions.status = cancelled`。
- 写入 outbox 事件 `AuctionCancelled`。
- WebSocket 广播 `auction_cancelled`。

### 2.3 竞拍

#### `POST /api/auctions/:id/bid` 出价

这是核心高并发接口。必须携带 `Idempotency-Key`。

Header：

```http
Authorization: Bearer mock-token-xxx
Idempotency-Key: bid-2-1-20260529200123001
X-Request-Id: req-20260529200123001
```

请求：

```json
{ "amount": 95000 }
```

成功响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "bid": {
      "id": 123,
      "auction_id": 1,
      "user": { "id": 2, "nickname": "买家张三", "avatar": null },
      "amount": 95000,
      "status": "accepted",
      "created_at": "2026-05-29T20:01:23+08:00"
    },
    "auction_version": 18,
    "current_price": 95000,
    "current_leader": { "id": 2, "nickname": "买家张三", "avatar": null },
    "extended": false,
    "new_end_time": "2026-05-29T20:05:00+08:00",
    "server_time": "2026-05-29T20:01:23.456+08:00"
  }
}
```

业务失败响应示例：

```json
{
  "code": 2101,
  "msg": "出价低于当前价+加价幅度",
  "data": {
    "min_acceptable_amount": 100000,
    "current_price": 95000,
    "price_step": 5000,
    "server_time": "2026-05-29T20:01:23.456+08:00"
  }
}
```

幂等规则：

- 同一用户、同一拍卖、同一 `Idempotency-Key`、同一 `amount` 重复提交，返回第一次处理结果。
- 同一 `Idempotency-Key` 但 `amount` 不同，返回 `1005`。
- 服务端必须持久化幂等记录，建议保留 24 小时。

后端处理顺序：

1. 校验登录、参数、幂等 key。
2. 读取拍卖当前状态。
3. 快速校验状态、价格、封顶价。
4. 尝试获取 Redis 锁 `bid_lock:{auctionId}`，TTL 3 秒。
5. 在 MySQL 事务内再次校验并条件更新拍卖。
6. 插入 accepted bid。
7. 更新幂等记录。
8. 写入 outbox：`BidAccepted`，如触发延时同时写 `AuctionExtended`。
9. 提交事务。
10. outbox publisher 异步投递 WS 事件。

事务内条件更新必须兜底业务规则：

```sql
UPDATE auctions
   SET current_price = ?,
       current_leader_id = ?,
       end_time = ?,
       version = version + 1,
       updated_at = NOW()
 WHERE id = ?
   AND status = 'active'
   AND end_time > NOW()
   AND current_price + price_step <= ?
   AND (ceiling_price IS NULL OR ? <= ceiling_price);
```

影响行数为 0 时，回滚并返回 `2103`，客户端刷新快照后再决定是否重试。

**封顶价自动成交**：当出价 `amount == ceiling_price` 且条件更新成功时，必须在同一事务内：

1. 置 `status = 'ended'`、`end_time = NOW()`。
2. 插入订单（`orders.auction_id` 唯一约束兜底）。
3. 写 outbox `BidAccepted` + `AuctionEnded`（同一事务，`event_seq` 连续）。
4. HTTP 响应 `data` 增加 `ceiling_hit: true` 和 `order_id`，前端据此立即切到成交态。

封顶成交不再触发 `auction_extended`，即使落在延时窗口内。

#### `GET /api/auctions/:id/bids`

Query：

| 参数 | 说明 |
|---|---|
| `limit` | 默认 20，最大 50 |
| `before_bid_id` | 可选，历史分页 |

只返回 `status=accepted`。

#### `GET /api/auctions/:id/status`

WS 降级和断线恢复接口。返回当前快照。

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "auction": {},
    "top_bids": [],
    "last_event_seq": 1024,
    "server_time": "2026-05-29T20:01:23.456+08:00"
  }
}
```

#### `GET /api/auctions/:id/events`

断线补偿接口。

Query：

| 参数 | 必填 | 说明 |
|---|---|---|
| `after_seq` | 是 | 客户端最后收到的事件序号 |
| `limit` | 否 | 默认 100，最大 500 |

响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "events": [],
    "has_more": false,
    "snapshot_required": false,
    "server_time": "2026-05-29T20:01:23.456+08:00"
  }
}
```

当 `after_seq` 太旧，服务端无法补齐事件时，返回 `snapshot_required=true`，客户端必须调用 `/status` 重建状态。

### 2.4 订单

#### `GET /api/orders/mine`

我作为买家的订单。需登录。

Query：`status` 可选，`page` 默认 1，`size` 默认 20。

#### `GET /api/orders/seller`

我作为卖家的订单。需登录。

#### `GET /api/orders/:id`

买家或卖家本人可看。

#### `POST /api/orders/:id/pay`

模拟支付。必须携带 `Idempotency-Key`。

支付幂等规则：

- 同一订单重复支付请求返回第一次支付结果。
- 已支付订单再次支付返回当前订单，`code=0`。
- 非赢家支付返回 `1003`。

### 2.5 用户

#### `GET /api/users/me`

需登录。返回 `data.user`。

### 2.6 资源上传

#### `POST /api/uploads`

需登录。`multipart/form-data`，字段 `file`。仅接受 `image/jpeg`、`image/png`、`image/webp`，单文件 <= 5MB。

响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "url": "https://cdn.example.com/u/2026/05/29/abc.jpg",
    "width": 1080,
    "height": 1080,
    "size": 234567
  }
}
```

商家端拿到 `url` 后回填到创建/修改拍卖的 `cover_url` / `images`。MVP 阶段服务端可落本地磁盘，但接口形态保持一致。

### 2.7 健康检查

#### `GET /health`

进程存活检查，不访问外部依赖。

```json
{ "status": "ok" }
```

#### `GET /ready`

就绪检查，需要检查 MySQL 和 Redis。

```json
{
  "status": "ok",
  "mysql": "ok",
  "redis": "ok"
}
```

---

## 3. WebSocket 协议

### 3.1 连接

```text
ws://<host>/ws/auction/:id?token=<token>&last_seq=<lastEventSeq>
```

- `token` 可为空。为空时匿名只读。
- `last_seq` 可为空。非空时服务端尽量补发缺失事件。
- 连接建立后服务端必须发送 `snapshot` 或缺失事件。

### 3.2 消息信封

所有服务端事件统一格式：

```json
{
  "type": "bid_update",
  "event_id": "evt_001",
  "auction_id": 1,
  "seq": 1024,
  "server_time": "2026-05-29T20:01:23.456+08:00",
  "data": {}
}
```

约定：

- 同一 `auction_id` 内 `seq` 单调递增。
- 客户端按 `seq` 去重和排序。
- 收到比本地 `last_seq + 1` 更大的事件时，客户端调用 `/events` 补偿。
- 补偿失败时调用 `/status` 重建快照。

### 3.3 客户端到服务端

#### `ping`

```json
{ "type": "ping", "client_time": "2026-05-29T20:01:23.123+08:00" }
```

客户端每 20 到 30 秒发送一次。服务端 60 秒未收到心跳关闭连接。

#### `ack`

可选。客户端确认已处理事件。

```json
{ "type": "ack", "auction_id": 1, "seq": 1024 }
```

### 3.4 服务端到客户端事件

#### `snapshot`

```json
{
  "type": "snapshot",
  "event_id": "evt_snapshot_001",
  "auction_id": 1,
  "seq": 1024,
  "server_time": "2026-05-29T20:01:23.456+08:00",
  "data": {
    "auction": {},
    "top_bids": []
  }
}
```

#### `bid_update`

```json
{
  "type": "bid_update",
  "event_id": "evt_1025",
  "auction_id": 1,
  "seq": 1025,
  "server_time": "2026-05-29T20:01:24.001+08:00",
  "data": {
    "auction_version": 19,
    "current_price": 100000,
    "current_leader": { "id": 3, "nickname": "买家李四", "avatar": null },
    "latest_bid": {},
    "top_bids": []
  }
}
```

#### `auction_extended`

```json
{
  "type": "auction_extended",
  "event_id": "evt_1026",
  "auction_id": 1,
  "seq": 1026,
  "server_time": "2026-05-29T20:04:50.001+08:00",
  "data": {
    "new_end_time": "2026-05-29T20:05:30+08:00",
    "extended_seconds": 30
  }
}
```

#### `auction_started`

```json
{
  "type": "auction_started",
  "event_id": "evt_1001",
  "auction_id": 1,
  "seq": 1001,
  "server_time": "2026-05-29T20:00:00.001+08:00",
  "data": { "auction": {} }
}
```

#### `auction_ended`

```json
{
  "type": "auction_ended",
  "event_id": "evt_1100",
  "auction_id": 1,
  "seq": 1100,
  "server_time": "2026-05-29T20:05:30.001+08:00",
  "data": {
    "auction": {},
    "winner": { "id": 2, "nickname": "买家张三", "avatar": null },
    "final_price": 95000,
    "order_id": 7
  }
}
```

无人出价时 `winner=null`、`final_price=null`、`order_id=null`。

#### `viewer_count`

房间在线人数变化。服务端节流广播，建议每 2 秒最多一条，且变化幅度小于 ±2% 时可丢弃。该事件 `seq` 仍参与全局自增，但客户端**不应**因为该事件缺失触发补偿；它是「软事件」。

```json
{
  "type": "viewer_count",
  "event_id": "evt_1027",
  "auction_id": 1,
  "seq": 1027,
  "server_time": "2026-05-29T20:01:25.000+08:00",
  "data": {
    "viewer_count": 901,
    "delta": 28
  }
}
```

#### `auction_cancelled`

```json
{
  "type": "auction_cancelled",
  "event_id": "evt_1050",
  "auction_id": 1,
  "seq": 1050,
  "server_time": "2026-05-29T20:03:00.001+08:00",
  "data": {
    "auction": {},
    "reason": "seller_cancelled"
  }
}
```

---

## 4. 客户端策略

### 4.1 Web 和移动端共同规则

- 倒计时必须使用 `server_time` 计算本地偏移。
- 出价按钮提交后进入 pending 状态，直到 HTTP 响应返回。
- 出价失败后以 HTTP 响应为准，不等待 WS。
- WS 事件只更新公共房间状态，不展示本人失败出价。
- 收到 `auction_ended` 后禁用出价并展示成交结果。

### 4.2 弱网和重连

- WS 断开后立即调用 `/status` 刷新一次。
- 之后按 1s、2s、5s、10s、10s 间隔重连。
- 重连时携带本地 `last_seq`。
- 如果 15 秒内无法恢复 WS，降级为每 2 秒短轮询 `/status`。
- 页面从后台回到前台时，必须调用 `/status` 校准状态。

---

## 5. 数据结构

### 5.1 Auction

```json
{
  "id": 1,
  "title": "天然翡翠吊坠",
  "description": "和田玉籽料，配 18K 金扣...",
  "cover_url": "https://...",
  "images": ["https://...", "https://..."],
  "stream_url": "https://live.example.com/room/1.flv",
  "start_price": 0,
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
  "version": 18,
  "viewer_count": 873,
  "bid_count": 24,
  "seller": { "id": 1, "nickname": "主播阿明", "avatar": null },
  "created_at": "2026-05-29T19:30:00+08:00",
  "updated_at": "2026-05-29T20:01:23+08:00"
}
```

`viewer_count` 为房间当前在线 WS 连接数估算（同一用户多端连接计多个），`bid_count` 为该拍卖累计 `accepted` 出价数。两者均为可能轻度滞后的快照值，前端不应用于业务正确性判断。

### 5.2 Bid

```json
{
  "id": 123,
  "auction_id": 1,
  "user": { "id": 2, "nickname": "买家张三", "avatar": null },
  "amount": 95000,
  "status": "accepted",
  "reject_reason": null,
  "idempotency_key": "bid-2-1-20260529200123001",
  "created_at": "2026-05-29T20:01:23+08:00"
}
```

### 5.3 Order

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

---

## 6. 状态机

### 6.1 拍卖状态

```text
pending -> active -> ended
        \          \
         \          -> cancelled
          -> cancelled
```

约束：

- `pending`：只能被启动或取消。
- `active`：可以接受出价、延时、自然结束、取消。
- `ended`：终态。
- `cancelled`：终态。

### 6.2 订单状态

```text
pending_pay -> paid
pending_pay -> closed
```

约束：

- 拍卖结束且有赢家时创建订单。
- `orders.auction_id` 必须唯一，防止重复创建订单。
- 支付接口必须幂等。

---

## 7. 高并发与一致性约定

### 7.1 出价一致性

出价一致性不能只依赖 Redis 锁，必须同时依赖：

- Redis 锁：降低同一拍卖内数据库竞争。
- MySQL 条件更新：最终业务规则兜底。
- 幂等记录：处理移动端重试和重复点击。
- 唯一约束：防止重复 bid 或重复订单。
- outbox：保证状态变更和事件发布一致。

### 7.2 Redis 锁

```text
key: bid_lock:{auctionId}
value: request_id
ttl: 3s
release: Lua 校验 value 后删除
```

要求：

- 锁只用于削峰，不能作为唯一正确性来源。
- 锁超时后事务仍必须靠 MySQL 条件更新保证正确。
- 获取锁失败返回 `2103`，客户端刷新快照后最多自动重试一次。

### 7.3 幂等表建议

```sql
CREATE TABLE idempotency_keys (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  scope VARCHAR(32) NOT NULL,
  idempotency_key VARCHAR(128) NOT NULL,
  request_hash CHAR(64) NOT NULL,
  response_json JSON NULL,
  status VARCHAR(16) NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE KEY uk_user_scope_key (user_id, scope, idempotency_key)
);
```

### 7.4 事件 outbox

所有会影响客户端状态的事务必须写 outbox。

```sql
CREATE TABLE event_outbox (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  aggregate_type VARCHAR(32) NOT NULL,
  aggregate_id BIGINT NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  event_seq BIGINT NOT NULL,
  payload JSON NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  created_at DATETIME NOT NULL,
  published_at DATETIME NULL,
  UNIQUE KEY uk_auction_seq (aggregate_type, aggregate_id, event_seq)
);
```

publisher 负责：

1. 扫描 `pending` 事件。
2. 投递到 WebSocket 房间。
3. 成功后置为 `published`。
4. 失败保留重试。

WebSocket 广播必须允许重复投递，客户端按 `event_id` 或 `seq` 去重。

### 7.5 多实例生命周期 Worker

不能依赖单进程 ticker。多实例必须通过数据库抢占或分布式任务保证安全。

启动拍卖：

```sql
UPDATE auctions
   SET status = 'active', version = version + 1, updated_at = NOW()
 WHERE status = 'pending'
   AND start_time <= NOW()
 LIMIT 100;
```

结束单个拍卖时必须在事务内：

1. 锁定 auction 行。
2. 确认 `status='active' AND end_time <= NOW()`。
3. 更新 `status='ended'`。
4. 如果有 leader，插入订单，`orders.auction_id` 唯一。
5. 写 outbox `AuctionEnded`。

---

## 8. 数据库索引与约束建议

```sql
ALTER TABLE auctions
  ADD INDEX idx_status_start_time (status, start_time),
  ADD INDEX idx_status_end_time (status, end_time),
  ADD INDEX idx_seller_status (seller_id, status);

ALTER TABLE bids
  ADD INDEX idx_auction_status_amount (auction_id, status, amount),
  ADD INDEX idx_auction_created (auction_id, created_at),
  ADD UNIQUE KEY uk_bid_idempotency (auction_id, user_id, idempotency_key);

ALTER TABLE orders
  ADD UNIQUE KEY uk_order_auction (auction_id),
  ADD INDEX idx_winner_status (winner_id, status),
  ADD INDEX idx_seller_status (seller_id, status);
```

---

## 9. 限流与保护

建议限流：

| 对象 | 规则 |
|---|---|
| 登录 | IP 维度 10 次/分钟 |
| 出价 | 用户 + 拍卖维度 3 次/秒 |
| 状态轮询 | IP + 拍卖维度 2 次/秒 |
| WS 连接 | 用户维度最多 5 条，IP 维度最多 100 条 |

系统保护：

- Redis 不可用：出价可降级为 MySQL 条件更新，但需要更严格限流。
- MySQL 不可用：写接口返回 `9001` 或 `9999`。
- outbox 堆积：HTTP 写入仍可成功，但监控必须告警；客户端可通过 `/status` 获取最终状态。

---

## 10. 可观测性

### 10.1 日志字段

每个写请求至少记录：

- `request_id`
- `user_id`
- `auction_id`
- `idempotency_key`
- `amount`
- `result_code`
- `latency_ms`
- `auction_version`

### 10.2 指标

| 指标 | 说明 |
|---|---|
| `bid_requests_total` | 出价请求数 |
| `bid_accepted_total` | 成功出价数 |
| `bid_rejected_total` | 失败出价数，按 reason 区分 |
| `bid_latency_ms` | 出价接口耗时 |
| `ws_connections` | 当前 WS 连接数 |
| `ws_broadcast_latency_ms` | 事件从 outbox 到客户端广播耗时 |
| `outbox_pending_total` | 待发布事件数 |
| `auction_lifecycle_lag_ms` | 拍卖应开始/结束时间与实际处理时间差 |

### 10.3 压测目标

V2 建议先定义以下验收目标：

- 单拍卖房间 1000 人在线，WS 正常广播。
- 单拍卖房间 200 QPS 出价请求，最终价格单调递增。
- 出价接口 P95 小于 200ms，P99 小于 500ms。
- WS 断线重连后 3 秒内恢复快照。
- 多实例同时运行 lifecycle worker，不重复创建订单。

---

## 11. 变更日志

| 日期 | 版本 | 变更 |
|---|---|---|
| 2026-05-29 | v2.0 | 增加 HTTP 状态码、幂等、WS 序号/补偿、多实例生命周期、outbox、索引、限流和可观测性 |
| 2026-05-29 | v2.1 | 对齐课题要求：支持 0 元起拍、`description`/`images`/`stream_url` 字段、修改未开始拍卖接口 `PUT /auctions/:id`、封顶价自动成交、`viewer_count` 字段与 WS 事件、`POST /uploads` 上传接口、延时范围收敛到 10–30 秒 |

