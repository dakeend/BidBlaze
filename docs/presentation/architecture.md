# 架构图与 README 首页描述

> 供答辩 PPT 第 2 页与根 `README.md` 首屏使用。mermaid 可在 Excalidraw/draw.io 重绘美化后导出 PNG。
> 注：以下为基于 `docs/contract-v2.md` 的整体描述，后端实现细节以 Role A 代码为准。

## 1. 系统架构

```mermaid
flowchart LR
  subgraph Client
    H[移动端 H5<br/>买家竞拍]
    A[admin-web<br/>商家后台/监控]
  end
  subgraph Server[server-go 单体分层]
    HTTP[HTTP API<br/>Gin]
    WS[WS Gateway<br/>gorilla/websocket]
    DOM[Domain Rules<br/>最低有效价/封顶/延时/状态流转]
    WK[Workers<br/>lifecycle + outbox publisher]
  end
  MY[(MySQL 8<br/>auctions/bids/orders/outbox)]
  RD[(Redis 7<br/>出价锁/快照/计数)]

  H -->|REST 出价/快照/补偿| HTTP
  A -->|REST 发布/管理/订单| HTTP
  H <-->|事件流 seq| WS
  A <-->|只读监控| WS
  HTTP --> DOM
  WS --> DOM
  DOM --> MY
  DOM --> RD
  WK --> MY
  WK -->|广播| WS
```

## 2. 出价时序（核心正确性）

```mermaid
sequenceDiagram
  participant C as 客户端
  participant API as HTTP API
  participant R as Redis
  participant DB as MySQL
  participant OB as Outbox/WS
  C->>API: POST /auctions/:id/bid (Idempotency-Key, amount)
  API->>R: 抢出价锁 (per-auction)
  API->>DB: 条件更新 SET current_price=? WHERE current_price+price_step<=amount AND version=?
  alt 条件命中
    DB-->>API: 1 row, version+1
    API->>DB: 写 bid(accepted) + outbox 事件 (同事务)
    API->>R: 释放锁
    API-->>C: 200 code=0 (current_price/leader/extended/new_end_time)
    OB-->>C: WS bid_update (按 seq)
  else 竞争失败
    DB-->>API: 0 row
    API-->>C: 200 code=2103 (刷新快照后可重试一次)
  end
```

## 3. WS 断线补偿时序

```mermaid
sequenceDiagram
  participant C as 客户端
  participant WS as WS Gateway
  participant API as HTTP API
  Note over C: 网络断开 → UI「同步中」
  C->>WS: 重连 (?token=)
  C->>API: GET /events?after_seq=last
  alt 缺口可补
    API-->>C: events[] (按 seq 去重续播)
  else snapshot_required
    C->>API: GET /status
    API-->>C: 全量快照 + last_event_seq
  end
  Note over C: 回前台(visibilitychange) 必拉 /status
```

## 4. README 首页 outline（根 README 已含启动步骤，可补充以下小节）
1. 一句话价值 + 一张竞拍截图
2. 架构图（本文件 §1）
3. 技术亮点清单（链接到 `slides-outline.md`）
4. 启动步骤（已有：docker compose + 三端 + `dev-up.ps1`）
5. 目录说明（已有）
6. 团队分工入口（已有）
