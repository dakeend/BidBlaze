# Part 1：Role B 实时网关 WS 骨架工作报告

> 记录日期：2026-06-03  
> 记录人：LSH / Role B  
> 项目：直播竞拍系统 `auction-system`  
> 工作范围：WebSocket 网关骨架，路径 `GET /ws/auction/:id?token=&last_seq=`

---

## 1. 完成的业务逻辑

本阶段完成的是直播竞拍系统的“实时房间连接与基础事件交互”业务逻辑。也就是说，移动端用户已经可以进入指定拍卖房间，服务端能够建立实时连接、返回房间初始状态、响应心跳，并广播房间在线人数变化。

已完成业务逻辑：

- 用户可通过 `ws://localhost:8080/ws/auction/:id?token=&last_seq=` 进入指定拍卖房间。
- 服务端按 `auction_id` 维护房间，保证不同拍卖房间之间的连接和消息互不串扰。
- WebSocket 建连成功后，服务端会优先返回 `snapshot`，让客户端获得房间初始状态。
- 客户端发送 `ping` 后，服务端返回 `pong`，形成基础心跳确认机制。
- 服务端维护房间连接数，并通过 `viewer_count` 软事件广播在线人数变化。
- `viewer_count` 不参与业务补偿游标，不会触发 `/events` 补偿，避免弱网恢复逻辑被软事件干扰。
- 网关提供 `hub.Publish(event)` 和 `hub.ForwardEvents(ctx, ch)`，为后续 outbox publisher 或 Redis pub/sub 广播真实业务事件预留入口。
- `last_seq` 参数、`Provider` 抽象和 snapshot/replay 流程已经预留，为下一阶段 `/events` 补偿接口接入做好准备。

简单例子：

假设买家张三打开 1 号拍卖直播间，移动端会连接：

```text
ws://localhost:8080/ws/auction/1?token=mock-token-user-001&last_seq=0
```

服务端识别到这是 `auction_id=1` 的房间连接后，会先把张三加入 1 号拍卖房间，并返回一条 `snapshot`，让前端知道当前房间的初始状态。随后前端发送：

```json
{ "type": "ping" }
```

服务端立即返回 `pong`，说明这条实时连接仍然可用。同时，因为 1 号房间新增了一个连接，服务端会广播 `viewer_count`，例如 `viewer_count=1`、`delta=1`。如果此时另一个用户进入 2 号拍卖房间，两个房间会分别维护连接和在线人数，1 号房间不会收到 2 号房间的消息。

当前业务链路已达到“可连接、可收首包、可心跳、可广播在线人数”的阶段，但真实拍卖状态、真实事件补偿和 Redis 多实例同步仍需后续 Task E/Task G 对接。

---

## 2. 工作背景

本阶段对应 Role B 的“场景 1：WS 网关骨架”。Role B 的职责是实时网关、移动端 H5、弱网重连、`/status`、`/events` 和上传接口。

本次工作的目标是先完成 WebSocket 实时层基础设施，使移动端和后续 PC 监控端能够连接到指定拍卖房间，接收房间快照、心跳响应和在线人数变化事件。

按照团队边界约束，本次实现不涉及 Role A 负责的核心业务模块，包括出价规则、订单生成、生命周期 worker、出价事务和数据库一致性逻辑。

---

## 3. 本次交付结论

本次已完成 WebSocket 网关骨架，并通过自动化测试和浏览器手工验收。

已实现能力：

- 提供 `GET /ws/auction/:id?token=<token>&last_seq=<seq>` WebSocket 连接入口。
- 建立 `Hub / Room / Client` 三层结构。
- 支持按 `auction_id` 进行房间隔离。
- WebSocket 建连后优先发送 `snapshot` 首包。
- 支持客户端应用层心跳：客户端发送 `ping`，服务端返回 `pong`。
- 支持 60 秒无心跳自动断开。
- 支持 `viewer_count` 软事件广播。
- 支持 `viewer_count` 节流策略：至少 2 秒间隔，变化不足约 2% 时可跳过。
- 提供 `hub.Publish(event)`，用于后续 Role A 的 outbox publisher 投递事件。
- 提供 `hub.ForwardEvents(ctx, ch)`，用于后续同进程 channel 或 Redis pub/sub 转发。

当前 `snapshot` 使用 `StaticProvider` 占位，尚未接入真实拍卖状态和 `event_outbox` 数据。这是本阶段预期内的骨架实现。

---

## 4. 涉及文件

### 4.1 修改文件

- `server-go/main.go`

主要变化：

- 创建 realtime hub。
- 启动 hub 主循环。
- 注册 `/ws/auction/:id` 路由。

### 4.2 新增文件

- `server-go/internal/realtime/event.go`
- `server-go/internal/realtime/provider.go`
- `server-go/internal/realtime/hub.go`
- `server-go/internal/realtime/room.go`
- `server-go/internal/realtime/client.go`
- `server-go/internal/realtime/handler.go`
- `server-go/internal/realtime/realtime_test.go`

---

## 5. 技术实现说明

### 5.1 Hub

`Hub` 是实时网关的总入口，负责维护所有拍卖房间。

核心职责：

- 管理客户端注册与注销。
- 按 `auction_id` 获取或创建房间。
- 接收外部业务事件并转发给对应房间。
- 提供 `Publish(event)` 作为 outbox publisher 的同进程接入点。
- 提供 `ForwardEvents(ctx, ch)` 作为未来 Redis pub/sub 或 channel 转发的统一桥接点。

当前实现保持轻量，不直接访问数据库，不生成业务事件。

### 5.2 Room

`Room` 代表单个拍卖房间。

核心职责：

- 保存当前房间内的客户端连接。
- 向同一房间广播事件。
- 确保不同 `auction_id` 的连接不会串消息。
- 估算并广播 `viewer_count`。
- 对 `viewer_count` 做节流，避免频繁广播。

`viewer_count` 是软事件，不作为业务补偿事件使用。

### 5.3 Client

`Client` 代表一个 WebSocket 连接。

核心职责：

- 处理 WebSocket 读循环。
- 处理 WebSocket 写循环。
- 接收客户端 `ping` 并返回 `pong`。
- 接收可选 `ack`，当前只记录调试日志。
- 在异常断开或退出时触发清理。

当前只支持协议层消息，不处理出价、订单等业务行为。

### 5.4 Handler

`handler.go` 提供 `/ws/auction/:id` 握手入口。

处理流程：

1. 解析 `auction_id`。
2. 解析可选 `last_seq`。
3. 读取 query 中的 `token`。
4. 升级 HTTP 连接为 WebSocket。
5. 创建 `Client`。
6. 先发送 `snapshot` 或补偿事件。
7. 再注册进入房间。
8. 开始读写循环。

这里特意保证 `snapshot` 首包优先于 `viewer_count`，符合“连接建立后必须先发 snapshot 或缺失事件”的协议要求。

### 5.5 Provider 抽象

本次新增 `Provider` 接口：

- `Snapshot(ctx, auctionID)`
- `EventsAfter(ctx, auctionID, afterSeq, limit)`

当前默认实现是 `StaticProvider`，只返回占位 snapshot。

后续做 Task E 时，可以将其替换为真实实现：

- `/status` 查询当前拍卖快照。
- `/events` 查询 `event_outbox` 中 `event_seq > last_seq` 的事件。

---

## 6. 事件协议处理

### 6.1 snapshot

当前建连后返回占位 snapshot：

```json
{
  "type": "snapshot",
  "event_id": "evt_snapshot_1_0",
  "auction_id": 1,
  "seq": 0,
  "server_time": "2026-06-03T15:18:59.6150174+08:00",
  "data": {
    "auction": {
      "id": 1
    },
    "top_bids": []
  }
}
```

说明：

- `seq=0` 是当前占位状态。
- 后续接入真实 provider 后，应使用真实 `last_event_seq`。

### 6.2 pong

客户端发送：

```json
{ "type": "ping" }
```

服务端返回：

```json
{
  "type": "pong",
  "auction_id": 1,
  "server_time": "2026-06-03T15:18:59.615522+08:00"
}
```

说明：

- 用于验证连接仍然活跃。
- 服务端读超时为 60 秒。

### 6.3 viewer_count

当前收到示例：

```json
{
  "type": "viewer_count",
  "event_id": "evt_viewer_1_1780471139615017400",
  "auction_id": 1,
  "seq": 0,
  "server_time": "2026-06-03T15:18:59.615522+08:00",
  "data": {
    "viewer_count": 1,
    "delta": 1,
    "reason": "viewer_count_changed"
  }
}
```

说明：

- `viewer_count=1` 表示当前房间有 1 个连接。
- `delta=1` 表示相比上一次广播增加 1 个连接。
- `viewer_count` 是软事件，不推进 outbox 补偿游标。
- 前端不应因缺失 `viewer_count` 触发 `/events` 补偿。

---

## 7. 验收记录

### 7.1 自动化测试

执行命令：

```powershell
cd D:\TRAEProj\auction-system\server-go
go test ./...
```

测试结果：

```text
?    auction-system/server-go    [no test files]
ok   auction-system/server-go/internal/realtime
```

自动化测试覆盖：

- 不同房间广播隔离。
- `viewer_count` 软事件。
- replay 不可用时回退 snapshot。
- WebSocket 建连后首包为 snapshot。
- 客户端发送 ping 后服务端返回 pong。

### 7.2 浏览器手工验收

验收代码：

```js
const ws = new WebSocket("ws://localhost:8080/ws/auction/1?token=mock-token-user-001&last_seq=0");

ws.onmessage = (e) => console.log("WS:", JSON.parse(e.data));
ws.onopen = () => ws.send(JSON.stringify({ type: "ping" }));
```

实际结果：

- 收到 `snapshot`。
- 收到 `pong`。
- 收到 `viewer_count`。

验收结论：

- WebSocket 路由可连接。
- snapshot 首包正常。
- ping/pong 心跳正常。
- viewer_count 广播正常。
- Origin 白名单生效，需要从 `localhost:5173` 或 `localhost:5174` 页面发起连接。

---

## 8. 当前限制

当前实现仍是场景一骨架，存在以下限制：

- `snapshot` 是占位数据，不是真实拍卖状态。
- `last_seq` 当前会进入 provider 流程，但因为 `StaticProvider` 未接入 outbox，所以会回退 snapshot。
- 未实现 `/api/auctions/:id/events` 补偿接口。
- 未接入 Redis pub/sub。
- 未实现真实鉴权，只保留 query token 传递和生产替换 TODO。
- 未接入 MySQL、Redis、真实拍卖数据。

---

## 9. 风险与评审意见

### 9.1 事件序号口径风险

当前实现按最终 Role B prompt 处理：

- 业务事件 `seq` 来自 `event_outbox.event_seq`。
- `viewer_count` 不推进补偿游标。

但 `contract-v2.md` 中曾出现“viewer_count 的 seq 仍参与全局自增”的表述。后续建议团队统一该口径，否则前端补偿逻辑可能出现歧义。

建议最终规则：

- `bid_update`、`auction_extended`、`auction_started`、`auction_ended`、`auction_cancelled` 使用 outbox seq。
- `viewer_count` 是软事件，仅按 `event_id` 去重，不触发补偿。

### 9.2 数据源替换风险

当前 `StaticProvider` 只是临时占位。后续 Task E 必须补齐真实 provider，否则移动端只能看到占位 snapshot，无法恢复真实拍卖状态。

### 9.3 多实例广播风险

当前提供了 `ForwardEvents(ctx, ch)` 接口，但尚未接 Redis pub/sub。单实例演示可用，多实例部署还需要后续补齐 Redis 订阅与发布。

---

## 10. 后续计划

建议下一阶段进入场景 2：`/events` 补偿接口。

优先事项：

1. 实现 `GET /api/auctions/:id/events?after_seq=&limit=`。
2. 从 `event_outbox` 查询 `event_seq > after_seq` 的事件。
3. 返回 `has_more` 和 `snapshot_required`。
4. 过滤或排除 `viewer_count` 软事件。
5. 将 `StaticProvider` 替换为真实 provider。
6. 让 WS 重连时携带 `last_seq` 后能够补发缺失事件或回退 snapshot。

---

## 11. 本阶段评审结论

本阶段已完成 Role B WebSocket 网关骨架，满足场景一验收要求。

该实现没有侵入 Role A 的核心业务模块，结构上保留了与 outbox publisher、Redis pub/sub 和 Task E provider 的对接空间，适合作为后续实时竞拍链路的基础。
