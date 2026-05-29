# Backend Integration Test Plan

> 本计划用于验证各 agent 独立实现的接口能按 `contract-v2.md`、`openapi.yaml`、`event-contract.md` 集成运行。

---

## 1. 验收前置条件

- MySQL 8.0 可用。
- Redis 7 可用。
- 已执行 `docs/schema-v2.sql` 或等价 migration。
- 后端服务运行在 `http://localhost:8080`。
- WebSocket 服务运行在 `ws://localhost:8080/ws/auction/:id`。
- 系统时间使用 `Asia/Shanghai`。

---

## 2. 全链路主流程

### Step 1: 健康检查

```powershell
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

预期：

- `/health` 返回 `{"status":"ok"}`。
- `/ready` 返回 `status=ok`，`mysql=ok`，`redis=ok`。

### Step 2: 登录卖家和买家

```powershell
curl -X POST http://localhost:8080/api/login -H "Content-Type: application/json" -d "{\"nickname\":\"主播阿明\",\"avatar\":null}"
curl -X POST http://localhost:8080/api/login -H "Content-Type: application/json" -d "{\"nickname\":\"买家张三\",\"avatar\":null}"
curl -X POST http://localhost:8080/api/login -H "Content-Type: application/json" -d "{\"nickname\":\"买家李四\",\"avatar\":null}"
```

预期：

- 每个响应 `code=0`。
- 返回 `token` 和 `user`。
- 同一昵称重复登录返回旧 token。

### Step 3: 上传图片并创建拍卖

使用晚于当前时间 1 到 2 分钟的 `start_time`。

```powershell
curl -X POST http://localhost:8080/api/uploads -H "Authorization: Bearer mock-token-seller-001" -F "file=@./fixtures/product.jpg"
curl -X POST http://localhost:8080/api/auctions -H "Authorization: Bearer mock-token-seller-001" -H "Content-Type: application/json" -d "{\"title\":\"天然翡翠吊坠\",\"description\":\"和田玉籽料，配 18K 金扣\",\"cover_url\":\"https://cdn.example.com/u/1.jpg\",\"images\":[\"https://cdn.example.com/u/1.jpg\"],\"stream_url\":\"https://live.example.com/room/1.flv\",\"start_price\":0,\"price_step\":5000,\"ceiling_price\":500000,\"start_time\":\"2026-05-29T20:00:00+08:00\",\"duration_seconds\":300,\"extend_seconds\":30,\"extend_threshold\":30}"
```

预期：

- 上传返回 `url`、`width`、`height`、`size`。
- 返回 `data.auction.status=pending`。
- `current_price=start_price`。
- `description`、`images`、`stream_url` 原样返回。
- `version` 存在。

### Step 3.5: 修改未开始拍卖

```powershell
curl -X PUT http://localhost:8080/api/auctions/1 -H "Authorization: Bearer mock-token-seller-001" -H "Content-Type: application/json" -d "{\"description\":\"更新后的商品介绍\",\"images\":[\"https://cdn.example.com/u/1.jpg\",\"https://cdn.example.com/u/2.jpg\"]}"
```

预期：

- pending 且未到 `start_time` 的拍卖可修改。
- 返回更新后的 `data.auction`。
- 非卖家修改返回 `403` 和 `code=1003`。

### Step 4: 查询拍卖

```powershell
curl http://localhost:8080/api/auctions
curl http://localhost:8080/api/auctions/1
curl http://localhost:8080/api/auctions/1/status
```

预期：

- 列表、详情、状态快照字段与 OpenAPI 一致。
- `/status` 包含 `last_event_seq` 和 `server_time`。
- auction 快照包含 `viewer_count` 和 `bid_count`。

### Step 5: WebSocket 建连

连接：

```text
ws://localhost:8080/ws/auction/1?token=mock-token-user-001
```

发送：

```json
{ "type": "ping", "client_time": "2026-05-29T20:00:01+08:00" }
```

预期：

- 建连后收到 `snapshot`。
- `snapshot` 包含 `event_id`、`auction_id`、`seq`、`server_time`、`data`。
- ping 后收到 `pong`。

### Step 6: Worker 启动拍卖

等待 `start_time` 到达，或在测试环境使用专用 fixture 创建已到开始时间的拍卖。

预期：

- 拍卖状态变为 `active`。
- outbox 写入 `AuctionStarted`。
- WebSocket 收到 `auction_started`。

### Step 7: 正常出价

```powershell
curl -X POST http://localhost:8080/api/auctions/1/bid -H "Authorization: Bearer mock-token-user-001" -H "Idempotency-Key: bid-user-001-001" -H "Content-Type: application/json" -d "{\"amount\":95000}"
```

预期：

- HTTP 返回 `code=0`。
- `current_price=95000`。
- `current_leader.id` 为买家张三。
- outbox 写入 `BidAccepted`。
- WebSocket 收到 `bid_update`。

### Step 8: 低价出价

```powershell
curl -X POST http://localhost:8080/api/auctions/1/bid -H "Authorization: Bearer mock-token-user-002" -H "Idempotency-Key: bid-user-002-low" -H "Content-Type: application/json" -d "{\"amount\":96000}"
```

预期：

- 返回 `code=2101`。
- 返回 `min_acceptable_amount`。
- 不广播 WebSocket 失败事件。

### Step 9: 幂等出价

重复 Step 7 的请求：

```powershell
curl -X POST http://localhost:8080/api/auctions/1/bid -H "Authorization: Bearer mock-token-user-001" -H "Idempotency-Key: bid-user-001-001" -H "Content-Type: application/json" -d "{\"amount\":95000}"
```

预期：

- 返回第一次请求的结果。
- 不新增 accepted bid。
- 不新增重复 `BidAccepted` 事件。

### Step 10: 幂等冲突

```powershell
curl -X POST http://localhost:8080/api/auctions/1/bid -H "Authorization: Bearer mock-token-user-001" -H "Idempotency-Key: bid-user-001-001" -H "Content-Type: application/json" -d "{\"amount\":100000}"
```

预期：

- HTTP 409。
- `code=1005`。

### Step 11: 事件补偿

```powershell
curl "http://localhost:8080/api/auctions/1/events?after_seq=0&limit=100"
```

预期：

- 返回 `AuctionStarted`、`BidAccepted` 等事件。
- 事件按 `seq` 升序。
- 每个事件格式与 `docs/events/event-contract.md` 一致。

### Step 12: 拍卖结束和订单创建

等待 `end_time` 到达，或使用测试 fixture 创建即将结束的 active 拍卖。

预期：

- 拍卖状态变为 `ended`。
- 有领先者时创建一条订单。
- `orders.auction_id` 唯一。
- outbox 写入 `AuctionEnded`。
- WebSocket 收到 `auction_ended`。

### Step 13: 查询和支付订单

```powershell
curl http://localhost:8080/api/orders/mine -H "Authorization: Bearer mock-token-user-001"
curl http://localhost:8080/api/orders/seller -H "Authorization: Bearer mock-token-seller-001"
curl http://localhost:8080/api/orders/1 -H "Authorization: Bearer mock-token-user-001"
curl -X POST http://localhost:8080/api/orders/1/pay -H "Authorization: Bearer mock-token-user-001" -H "Idempotency-Key: pay-user-001-001"
```

预期：

- 买家和卖家都能查询对应订单。
- 非买家和非卖家不能看订单。
- 支付成功后 `status=paid`。
- 重复支付返回当前订单，`code=0`。

---

## 3. 并发验收

### 3.1 同拍卖并发出价

场景：

- 一个 active 拍卖。
- 100 到 1000 个并发请求。
- amount 从低到高生成。
- 每个请求使用唯一 `Idempotency-Key`。

必须验证：

- accepted bids 金额严格递增。
- `auctions.current_price` 等于最高 accepted bid。
- `auctions.current_leader_id` 等于最高 accepted bid 用户。
- rejected bid 有明确 reason。
- 没有重复 idempotency key 记录。

### 3.2 多实例 Lifecycle Worker

场景：

- 同时启动 2 到 3 个 worker。
- 创建多个到期 active 拍卖。

必须验证：

- 每个拍卖最多创建一条订单。
- 每个拍卖最多写一条 `AuctionEnded` 业务事件。
- worker 重启后可以继续处理未完成拍卖。

### 3.3 Outbox 重试

场景：

- 写入 pending outbox。
- 模拟 WebSocket 发布失败。
- 重启 publisher。

必须验证：

- 失败事件不会丢失。
- 恢复后 pending 事件会发布。
- 重复发布不会导致客户端状态错误。

---

## 4. 弱网和断线恢复验收

### 4.1 WS 断开后重连

场景：

- 客户端收到 `seq=10`。
- 断开 WS。
- 期间产生 `seq=11`、`seq=12`。
- 使用 `last_seq=10` 重连。

预期：

- 服务端补发 `seq=11`、`seq=12`，或发送 snapshot。
- 客户端最终状态与 `/status` 一致。

### 4.2 补偿窗口过旧

场景：

- 客户端使用很旧的 `after_seq` 请求 `/events`。

预期：

- 返回 `snapshot_required=true`。
- 客户端调用 `/status` 重建状态。

---

## 5. OpenAPI 合同验收

每个 REST 接口响应必须满足：

- HTTP 状态码符合 `docs/api/openapi.yaml`。
- JSON 字段符合对应 schema。
- 错误响应包含 `code`、`msg`、`data`。
- 写接口缺少 token 时返回 401。
- 出价和支付缺少 `Idempotency-Key` 时返回 400。

建议在实现后加入 OpenAPI response validator，自动校验关键接口响应。

---

## 6. 最终通过标准

```powershell
go test ./...
```

最终必须满足：

- 所有单元测试和集成测试通过。
- REST 合同与 `docs/api/openapi.yaml` 一致。
- WS/outbox 事件与 `docs/events/event-contract.md` 一致。
- 数据库结构与 `docs/schema-v2.sql` 或 migration 一致。
- 高并发出价后价格单调递增。
- 多实例 worker 不重复创建订单。
- 断线重连能恢复最终状态。

