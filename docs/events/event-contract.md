# Event Contract v2

> 本文件约束 `event_outbox`、WebSocket 广播、断线补偿接口之间的事件格式。
> 所有后端 agent 必须按本文生成、发布、消费事件。

---

## 1. 事件原则

- 事件由业务事务写入 `event_outbox`，publisher 只负责发布，不生成新业务事件。
- 同一 `auction_id` 内 `event_seq` 必须单调递增。
- WebSocket 事件允许重复投递，客户端必须按 `event_id` 或 `seq` 去重。
- 断线补偿接口 `/api/auctions/:id/events` 返回的事件格式必须和 WebSocket 服务端事件一致。
- 事件 payload 使用 JSON，字段名使用 snake_case。

---

## 2. Outbox 存储格式

`event_outbox.payload` 存储完整 WebSocket 事件信封。

```json
{
  "type": "bid_update",
  "event_id": "evt_1_1025",
  "auction_id": 1,
  "seq": 1025,
  "server_time": "2026-05-29T20:01:24.001+08:00",
  "data": {}
}
```

对应表字段：

| 字段 | 来源 |
|---|---|
| `aggregate_type` | 固定为 `auction` |
| `aggregate_id` | `auction_id` |
| `event_type` | PascalCase 领域事件名，例如 `BidAccepted` |
| `event_seq` | 同 payload 内 `seq` |
| `payload` | 完整事件信封 |
| `status` | `pending` / `published` / `failed` |

---

## 3. 序号生成

推荐实现：

1. 在写业务事务时锁定 auction 行。
2. 读取当前 `auctions.version`。
3. 每次状态变化使 `version = version + 1`。
4. outbox `event_seq` 使用更新后的 `auctions.version`。

约束：

- 同一事务如果产生多个事件，必须分配不同 `seq`。
- 如果出价同时触发延时，先写 `bid_update`，再写 `auction_extended`。
- 多事件事务可以连续递增版本两次，也可以使用独立 `auction_event_sequences` 表。实现必须保证同一 auction 内唯一递增。

---

## 4. 事件类型

### 4.1 BidAccepted -> `bid_update`

产生时机：出价事务成功提交。

Outbox:

```json
{
  "aggregate_type": "auction",
  "aggregate_id": 1,
  "event_type": "BidAccepted",
  "event_seq": 1025,
  "payload": {
    "type": "bid_update",
    "event_id": "evt_1_1025",
    "auction_id": 1,
    "seq": 1025,
    "server_time": "2026-05-29T20:01:24.001+08:00",
    "data": {
      "auction_version": 1025,
      "current_price": 100000,
      "current_leader": { "id": 3, "nickname": "买家李四", "avatar": null },
      "latest_bid": {
        "id": 124,
        "auction_id": 1,
        "user": { "id": 3, "nickname": "买家李四", "avatar": null },
        "amount": 100000,
        "status": "accepted",
        "reject_reason": null,
        "idempotency_key": "bid-3-1-20260529200124001",
        "created_at": "2026-05-29T20:01:24+08:00"
      },
      "top_bids": []
    }
  }
}
```

客户端处理：

- 更新当前价、领先者、排行榜。
- 如果本地 `seq` 已处理过，忽略。
- 如果收到的 `seq > last_seq + 1`，调用 `/api/auctions/:id/events` 补偿。

### 4.2 AuctionExtended -> `auction_extended`

产生时机：成功出价且满足延时条件。

```json
{
  "type": "auction_extended",
  "event_id": "evt_1_1026",
  "auction_id": 1,
  "seq": 1026,
  "server_time": "2026-05-29T20:04:50.001+08:00",
  "data": {
    "new_end_time": "2026-05-29T20:05:30+08:00",
    "extended_seconds": 30
  }
}
```

客户端处理：

- 用 `new_end_time` 覆盖本地结束时间。
- 不允许用本地时间累加延时。

### 4.3 AuctionStarted -> `auction_started`

产生时机：Lifecycle Worker 将拍卖从 `pending` 推进到 `active`。

```json
{
  "type": "auction_started",
  "event_id": "evt_1_1001",
  "auction_id": 1,
  "seq": 1001,
  "server_time": "2026-05-29T20:00:00.001+08:00",
  "data": {
    "auction": {}
  }
}
```

客户端处理：

- 将拍卖状态置为 `active`。
- 启用出价入口。

### 4.4 AuctionEnded -> `auction_ended`

产生时机：Lifecycle Worker 将拍卖从 `active` 推进到 `ended`。

有赢家：

```json
{
  "type": "auction_ended",
  "event_id": "evt_1_1100",
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

流拍：

```json
{
  "type": "auction_ended",
  "event_id": "evt_1_1100",
  "auction_id": 1,
  "seq": 1100,
  "server_time": "2026-05-29T20:05:30.001+08:00",
  "data": {
    "auction": {},
    "winner": null,
    "final_price": null,
    "order_id": null
  }
}
```

客户端处理：

- 禁用出价。
- 有 `order_id` 时展示去支付入口。
- 无 `order_id` 时展示流拍。

### 4.5 AuctionCancelled -> `auction_cancelled`

产生时机：卖家取消拍卖。

```json
{
  "type": "auction_cancelled",
  "event_id": "evt_1_1050",
  "auction_id": 1,
  "seq": 1050,
  "server_time": "2026-05-29T20:03:00.001+08:00",
  "data": {
    "auction": {},
    "reason": "seller_cancelled"
  }
}
```

客户端处理：

- 禁用出价。
- 展示取消提示。

### 4.6 snapshot

产生时机：

- WebSocket 建连后。
- 客户端断线恢复失败后调用 `/status`。

```json
{
  "type": "snapshot",
  "event_id": "evt_snapshot_1_1024",
  "auction_id": 1,
  "seq": 1024,
  "server_time": "2026-05-29T20:01:23.456+08:00",
  "data": {
    "auction": {},
    "top_bids": []
  }
}
```

### 4.7 ViewerCount -> `viewer_count`

产生时机：WebSocket 房间在线连接数发生变化，服务端按节流策略广播。

```json
{
  "type": "viewer_count",
  "event_id": "evt_1_1027",
  "auction_id": 1,
  "seq": 1027,
  "server_time": "2026-05-29T20:01:25.000+08:00",
  "data": {
    "viewer_count": 901,
    "delta": 28
  }
}
```

客户端处理：

- 更新在线人数展示。
- 该事件是软事件，不应因为缺失 `viewer_count` 事件触发 `/events` 补偿。

---

## 5. 事件保留和补偿

- outbox published 事件至少保留 24 小时。
- `/events?after_seq=` 默认返回 100 条，最大 500 条。
- 如果 `after_seq` 小于服务端可补偿的最小 seq，返回 `snapshot_required=true`。
- 如果还有更多事件，返回 `has_more=true`，客户端继续请求。

补偿响应：

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

---

## 6. Agent 分工约束

| Agent | 可写事件 | 可读事件 |
|---|---|---|
| Task C 拍卖管理 | `AuctionCancelled` | 无 |
| Task D 出价核心 | `BidAccepted`, `AuctionExtended` | 无 |
| Task E 状态/事件接口 | 无 | 全部 |
| Task F WebSocket | `ViewerCount` | 全部 |
| Task G Outbox Publisher | 无 | 全部 pending |
| Task H Lifecycle Worker | `AuctionStarted`, `AuctionEnded` | 无 |

任何 agent 不得绕过 outbox 直接调用其他业务模块完成事件发布。

