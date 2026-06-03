# Part 2：Role B /events 补偿接口工作报告

> 记录日期：2026-06-03  
> 记录人：LSH / Role B  
> 项目：直播竞拍系统 `auction-system`  
> 工作范围：断线补偿接口 `GET /api/auctions/:id/events?after_seq=&limit=`

---

## 1. 完成的业务逻辑

本阶段完成的是直播竞拍系统的“业务事件断线补偿”能力。它解决的问题是：移动端 H5 或 WebSocket 客户端在弱网、断线重连、页面挂起恢复时，可能错过部分竞拍事件；客户端可以用自己最后处理到的业务事件序号 `after_seq` 向服务端补拉缺失事件，从而尽量恢复到正确的房间状态。

已完成业务逻辑：

- 客户端可通过 `GET /api/auctions/:id/events?after_seq=&limit=` 查询指定拍卖房间中 `after_seq` 之后的业务事件。
- 服务端按 `auction_id` 和 `event_seq` 从 `event_outbox` 中顺序拉取事件，保证补偿事件按业务序号递增返回。
- `limit` 默认值为 100，最大值为 500，避免一次补偿请求拉取过多数据。
- 响应中返回 `has_more`，用于告诉客户端后面是否还有更多可补偿事件。
- 当 `after_seq` 太旧、缺失跨度超过阈值，或者事件已经无法完整补齐时，服务端返回 `snapshot_required=true`，提示客户端改走 `/status` 重建状态。
- `viewer_count` 被明确排除在 `/events` 补偿结果之外，因为它是软事件，不推进业务补偿游标，也不触发补偿。
- WebSocket 重连时如果 `last_seq` 对应的补偿结果还有更多页，当前实现不会只推送不完整事件流，而是回退到 `snapshot`，避免客户端拿到半截状态。
- 服务端支持通过 `MYSQL_DSN` 接入真实 MySQL outbox；未配置或 MySQL 不可用时降级为 `StaticProvider`，保证本地开发服务仍可启动。

简单例子：

假设用户正在观看 1 号拍卖房间，客户端已经处理到业务事件 `seq=10`。由于弱网断线，客户端错过了 `seq=11` 和 `seq=12` 两条业务事件。重连后，客户端可以请求：

```text
GET /api/auctions/1/events?after_seq=10&limit=100
```

如果 outbox 中仍然保留完整事件，服务端会返回类似：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "events": [
      { "type": "bid_update", "auction_id": 1, "seq": 11 },
      { "type": "auction_extended", "auction_id": 1, "seq": 12 }
    ],
    "has_more": false,
    "snapshot_required": false,
    "server_time": "2026-06-03T16:40:00+08:00"
  }
}
```

客户端按顺序应用 `seq=11` 和 `seq=12` 后，就可以恢复缺失期间的竞拍状态。如果服务端判断 `after_seq` 太旧，例如当前最大业务事件已经到 `seq=2000`，而客户端仍停在 `seq=10`，则返回 `snapshot_required=true`，客户端应调用 `/status` 重新获取当前完整快照。

---

## 2. 工作背景

本阶段对应 Role B 的“场景 2：/events 补偿接口”。该接口是 WebSocket 弱网重连链路中的关键补偿能力，用于支撑移动端 H5 在断线、重连、事件断档时恢复状态。

根据 `docs/contract-v2.md` 和事件合同要求：

- 业务事件以 `event_outbox.event_seq` 为准。
- 客户端发现业务事件 `seq > last_seq + 1` 时，应调用 `/events` 补齐中间缺失事件。
- `/events` 只补偿 outbox 中的业务事件。
- `viewer_count` 是软事件，不应进入 `/events` 响应。
- 若无法补齐，应返回 `snapshot_required=true`，由客户端改用 `/status` 重建状态。

本次工作仍严格保持 Role B 边界，没有实现出价规则、订单生成、拍卖 lifecycle worker，也没有改动 `internal/bid`、`internal/order`、`internal/worker`。

---

## 3. 本次交付结论

本次已完成 `/events` 补偿接口的后端实现、outbox provider 查询骨架、MySQL 接入开关、静态降级策略以及自动化测试。当前实现已经具备 Role B 场景 2 的主要协议能力，可以作为移动端弱网补偿逻辑和后续真实 outbox publisher 对接的基础。

已实现能力：

- 注册 `GET /api/auctions/:id/events` HTTP 路由。
- 校验 `auction_id`、必填 `after_seq`、可选 `limit`。
- 返回统一响应结构：`code`、`msg`、`data.events`、`data.has_more`、`data.snapshot_required`、`data.server_time`。
- 从 `event_outbox` 查询指定拍卖房间的业务事件。
- 使用 `limit + 1` 判断是否存在下一页。
- 通过 `snapshot_required` 表达事件太旧或无法补齐。
- 过滤 `ViewerCount` / `viewer_count`。
- 通过 `MYSQL_DSN` 启用真实 MySQL provider。
- MySQL 不可用时降级到 `StaticProvider`，避免本地开发直接崩溃。
- 补充自动化测试覆盖主要接口行为。

---

## 4. 涉及文件

### 4.1 修改文件

- `server-go/main.go`
- `server-go/go.mod`
- `server-go/go.sum`
- `server-go/internal/realtime/handler.go`
- `server-go/internal/realtime/hub.go`
- `server-go/internal/realtime/provider.go`
- `server-go/internal/realtime/realtime_test.go`

### 4.2 新增文件

- `server-go/internal/realtime/outbox_provider.go`
- `server-go/main_test.go`
- `LSH_工作报告/Part2_events补偿接口工作报告.md`

---

## 5. 技术实现说明

### 5.1 Handler：HTTP 补偿入口

`handler.go` 中新增：

```go
router.GET("/api/auctions/:id/events", hub.ServeEvents)
```

`ServeEvents` 负责：

- 解析并校验 `auction_id`。
- 要求 `after_seq` 必填且必须大于等于 0。
- 解析 `limit`，未传时使用默认值 100，大于 500 时收敛为 500。
- 调用 `h.provider.EventsAfter(...)` 获取补偿事件。
- 将结果封装为合同要求的统一 JSON 响应。

### 5.2 Provider：补偿能力抽象

`provider.go` 中扩展 `ReplayResult`：

```go
type ReplayResult struct {
    Events           []EventEnvelope
    HasMore          bool
    SnapshotRequired bool
}
```

这样 `Provider` 可以同时表达三种情况：

- 成功返回一批补偿事件。
- 还有更多事件需要继续拉取。
- 当前已经无法补齐，需要客户端回退到 snapshot/status。

### 5.3 OutboxProvider：MySQL outbox 查询实现

新增 `outbox_provider.go`，核心 SQL：

```sql
SELECT payload
  FROM event_outbox
 WHERE aggregate_type = 'auction'
   AND aggregate_id = ?
   AND event_seq > ?
   AND event_type NOT IN ('ViewerCount', 'viewer_count')
 ORDER BY event_seq ASC
 LIMIT ?
```

实现要点：

- 查询条件使用 `aggregate_type='auction'` 和 `aggregate_id=auctionID` 绑定拍卖房间。
- 使用 `event_seq > afterSeq` 只拉客户端缺失之后的事件。
- 使用 `ORDER BY event_seq ASC` 保证事件按序返回。
- 使用 `limit + 1` 判断是否还有更多事件。
- 过滤 `ViewerCount` 和 `viewer_count`，避免软事件进入补偿链路。
- 从 `payload` 反序列化为 WebSocket 统一事件信封 `EventEnvelope`。

### 5.4 太旧判定

当前判定逻辑：

- 如果当前最大业务事件序号 `maxSeq - afterSeq > 1000`，认为缺失跨度过大，返回 `snapshot_required=true`。
- 如果 outbox 中最早保留的业务事件已经晚于客户端需要的下一条事件，也返回 `snapshot_required=true`。

这对应合同中“after_seq 太旧无法补齐时返回 snapshot_required=true”的要求。

### 5.5 WebSocket replay 保护

`hub.go` 中的 `replayOrSnapshot` 增加了 `HasMore` 保护：

- 如果 replay 成功、没有要求 snapshot、没有更多页，并且事件非空，则推送补偿事件。
- 如果补偿结果还有更多页，则不推送半截事件流，回退到 snapshot。

这样可以避免 WS 重连时只补一部分事件，导致客户端状态处于不完整中间态。

### 5.6 MySQL 接入与降级

`main.go` 中新增 `newRealtimeProvider()`：

- 未配置 `MYSQL_DSN`：使用 `StaticProvider`。
- 配置了 `MYSQL_DSN`：尝试创建 MySQL 连接并执行 `PingContext`。
- MySQL 连接失败：记录 warning，并降级到 `StaticProvider`。
- MySQL 可用：使用 `NewOutboxProvider(db)`。

这保证了本地开发环境即使没有 MySQL，也可以启动服务并验证接口形态。

---

## 6. 协议或数据流说明

### 6.1 客户端补偿调用时机

客户端维护本地业务游标 `last_seq`。当收到 WS 业务事件时：

1. 如果 `event.seq == last_seq + 1`，正常应用事件，并更新 `last_seq`。
2. 如果 `event.seq <= last_seq`，说明是重复事件，按 `event_id` 或 `seq` 去重。
3. 如果 `event.seq > last_seq + 1`，说明中间出现业务事件断档，调用 `/events?after_seq=last_seq`。
4. 如果 `/events` 返回 `snapshot_required=false`，客户端按顺序应用 `events`。
5. 如果 `/events` 返回 `snapshot_required=true`，客户端调用 `/status` 重建当前状态。

### 6.2 `/events` 响应结构

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "events": [],
    "has_more": false,
    "snapshot_required": false,
    "server_time": "2026-06-03T16:40:00+08:00"
  }
}
```

字段说明：

- `events`：缺失的业务事件列表。
- `has_more`：是否还有更多可补偿事件。
- `snapshot_required`：是否必须改用 `/status` 重建快照。
- `server_time`：服务端时间，用于前端校准时间偏移。

### 6.3 viewer_count 处理规则

`viewer_count` 不应该出现在 `/events` 响应中。

原因：

- 它是在线人数估算事件，不影响竞拍业务正确性。
- 它不推进业务补偿游标。
- 它的缺失不应该触发 `/events`。
- 客户端只需要按 `event_id` 对它做去重即可。

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
ok   auction-system/server-go
ok   auction-system/server-go/internal/realtime
```

自动化测试覆盖：

- `/events` 正常返回补偿事件。
- `limit=999` 会被收敛为最大值 500。
- 缺少 `after_seq` 时返回 400。
- `StaticProvider` 下 `/events` 返回 `snapshot_required=true`。
- WS replay 结果存在 `has_more=true` 时回退到 snapshot。
- MySQL DSN 不可用时，provider 降级为 `StaticProvider`。

### 7.2 手工验收

无 MySQL 环境下启动服务：

```powershell
cd D:\TRAEProj\auction-system\server-go
go run .
```

另开终端请求：

```powershell
curl.exe "http://localhost:8080/api/auctions/1/events?after_seq=0"
```

预期结果：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "events": null,
    "has_more": false,
    "snapshot_required": true,
    "server_time": "..."
  }
}
```

说明：

- 当前没有配置真实 `MYSQL_DSN` 时，服务使用 `StaticProvider`。
- `snapshot_required=true` 是合理结果，表示客户端应调用 `/status` 获取快照。
- 配置真实 MySQL 且 `event_outbox` 有数据后，该接口会返回真实业务事件。

---

## 8. 当前限制

- 当前 `Snapshot` 仍使用 `newSnapshotEvent` 占位数据，不是真实 `/status` 快照。
- 当前只实现了 outbox 查询 provider，尚未实现 Role A 的 outbox publisher。
- 当前没有实现 Redis pub/sub 多实例广播接入。
- 当前依赖 `event_outbox.payload` 已经是符合 WS 合同的 `EventEnvelope` JSON。
- 当前 `/ready` 仍是占位检查，未真实探测 MySQL 和 Redis。
- 当前没有实现移动端 `useAuctionSocket` 对 `/events` 的自动调用逻辑。

---

## 9. 风险与评审意见

- `event_outbox.payload` 的 JSON 结构必须和 `EventEnvelope` 保持一致，否则 `/events` 反序列化会失败并返回 500。
- `ViewerCount` 和 `viewer_count` 目前都被过滤，后续团队应统一 outbox 中事件类型命名，减少兼容分支。
- `snapshot_required` 阈值当前为 1000，是工程建议值；如果高频竞拍场景事件增长很快，后续需要根据压测结果调整。
- MySQL 不可用时当前选择降级为 `StaticProvider`，适合开发期；生产环境应结合 `/ready`、告警和部署策略判断是否允许降级。
- `has_more=true` 时 WS 首包回退 snapshot 是保守策略；如果后续希望通过 WS 连续补多页，需要设计客户端处理半同步状态的 UI 和状态机。

---

## 10. 后续计划

1. 与 Role A 对齐 `event_outbox.payload` 的最终结构，确保事件信封字段完整，包括 `type`、`event_id`、`auction_id`、`seq`、`server_time`、`data`。
2. 实现真实 `/api/auctions/:id/status`，用于 `snapshot_required=true` 后的快照恢复。
3. 在移动端 `useAuctionSocket` 中实现事件断档检测：仅业务事件推进 `last_seq`，`viewer_count` 不推进。
4. 在移动端接入 `/events` 自动补偿：发现 `seq > last_seq + 1` 后拉取 `/events`，失败或要求 snapshot 时调用 `/status`。
5. 接入 Redis pub/sub 或 Role A outbox publisher，使真实业务事件可以进入 WS 房间广播。
6. 基于真实数据和弱网测试补充端到端验收，包括断线、重连、补偿、快照恢复。

---

## 11. 本阶段评审结论

本阶段已完成 Role B 的 `/events` 补偿接口基础能力，满足合同中“按 `after_seq` 拉取缺失业务事件、限制 `limit`、返回 `has_more`、无法补齐时要求 snapshot、排除 `viewer_count`”的核心要求。

该实现没有侵入 Role A 负责的出价规则、订单和 worker 模块，边界清晰。当前代码已经通过 Go 自动化测试，适合作为后续移动端弱网重连和真实 outbox publisher 对接的基础。
