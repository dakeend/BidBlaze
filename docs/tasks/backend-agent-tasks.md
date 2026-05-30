# Backend Agent Tasks

> 本任务单用于把后端接口开发拆给多个 agent 独立执行。
> 所有 agent 必须以 `docs/contract-v2.md` 为唯一接口合同，以 `docs/schema-v2.sql` 为当前数据库基线。

---

## 0. 全局协作规则

### 0.1 所有 agent 必须遵守

- 不允许私自修改接口字段、错误码、状态码、WebSocket 事件格式。
- 不允许把自己的模块和其他模块的内部实现强耦合。
- 可以依赖公共 package、数据库表、Redis key、outbox 事件。
- 写接口时必须同时提供测试或可复现 curl 示例。
- 涉及数据库结构变更时，必须新增 migration 或更新 schema，并说明兼容影响。
- 涉及并发、幂等、状态流转的代码必须有测试。
- 每个任务完成后必须能独立运行对应测试。

### 0.2 推荐目录边界

后端建议在 `server-go` 内按以下边界组织。具体命名可按现有项目风格调整，但职责不能混在一起。

| 目录 | 职责 |
|---|---|
| `cmd/server` | 服务启动入口 |
| `internal/config` | 配置加载 |
| `internal/http` | Gin router、中间件、统一响应 |
| `internal/auth` | token 生成、解析、鉴权 |
| `internal/user` | 用户领域 |
| `internal/auction` | 拍卖创建、查询、取消、状态流转 |
| `internal/bid` | 出价核心逻辑 |
| `internal/order` | 订单查询、支付 |
| `internal/upload` | 图片上传、文件校验、URL 生成 |
| `internal/realtime` | WebSocket 房间、连接、广播 |
| `internal/outbox` | 事件 outbox 写入、扫描、发布 |
| `internal/worker` | 生命周期 worker |
| `internal/storage` | MySQL、Redis 初始化和基础封装 |
| `internal/testutil` | 测试辅助 |

### 0.3 框架与分层约定

后端使用 Gin 做 HTTP transport，使用 gorilla/websocket 做实时连接；项目按单体模块化服务组织，不采用 Go-kit、Kratos、go-zero 等重型微服务框架。

每个业务模块优先按以下职责拆分：

| 层 | 建议命名 | 职责 |
|---|---|---|
| Handler | `handler.go` | Gin 参数绑定、鉴权上下文读取、统一响应，不写核心业务规则 |
| Service | `service.go` | 应用编排、事务边界、幂等处理、outbox 写入 |
| Domain | `domain.go` / `rules.go` | 竞拍规则、状态流转、价格校验等纯逻辑 |
| Repository | `repository.go` | MySQL / Redis 访问，复杂事务保留明确 SQL |
| Model | `model.go` | DB model、DTO、响应结构转换 |

全局约束：

- Gin handler 不直接访问数据库，除健康检查外必须调用 service。
- 核心出价事务不得依赖 ORM 自动保存，必须使用明确 SQL 或 repository 方法表达条件更新。
- Redis 锁只能削峰，最终正确性由 MySQL 条件更新、唯一约束和事务保证。
- WebSocket 模块不得直接写 bids、orders、auctions 业务表。
- outbox publisher 只发布事件，不生成业务事件。
- 领域规则必须能脱离 Gin 做单元测试。
- 日志必须字段化，至少包含 `request_id`、`user_id`、`auction_id`、`result_code`、`latency_ms`。

推荐依赖：

| 能力 | 依赖 |
|---|---|
| HTTP | `github.com/gin-gonic/gin` |
| WebSocket | `github.com/gorilla/websocket` |
| MySQL | `database/sql` + `github.com/jmoiron/sqlx` |
| Redis | `github.com/redis/go-redis/v9` |
| 测试断言 | `github.com/stretchr/testify` |
| 日志 | Go `log/slog` 或 zap |

### 0.4 集成顺序

```text
A 基础工程
B 用户认证
C 拍卖管理

D 出价核心
E 状态/历史/事件接口
F WebSocket 网关
I 订单接口
K 资源上传

G Outbox Publisher
H Lifecycle Worker
J 压测与稳定性验收
```

---

## Task A: 基础工程与公共合同（**含 B1 mock login 桩**）

### 目标

建立后端服务底座，让其他 agent 可以共享统一响应、错误码、配置、数据库连接和健康检查。
**D1 EOD 还需交付 mock login 桩 + 种子 token**，解耦 Role B/C 不必等 Task B 完成。

### 负责范围

- 服务启动入口。
- Gin router。
- 配置加载。
- MySQL / Redis 连接（schema-v2.sql 自动初始化 + seed 用户）。
- 统一响应结构。
- 错误码定义（至少 1001/1002/1003 + 0）。
- 请求追踪 middleware（注入 `X-Request-Id`）。
- `/health` 和 `/ready`。
- **B1: mock login 桩**（详见下方"B1 桩交付物"）。

### 接口

- `GET /health`
- `GET /ready`
- `POST /api/login` （**D1 桩版本**：按 nickname 查 users 返 token；不做参数校验之外的业务）

### B1 桩交付物（D1 EOD）

按 `dev-setup.md §5` 的 token 算法落地最小可用版本：

1. `schema-v2.sql` 容器初始化时插入 3 个种子用户：
   ```
   (1, '主播阿明', null, 'mock-token-seller-001')
   (2, '买家张三', null, 'mock-token-user-001')
   (3, '买家李四', null, 'mock-token-user-002')
   ```
2. `POST /api/login` 桩逻辑（≤30 行）：
   - 收到 `{nickname, avatar}`。
   - `SELECT * FROM users WHERE nickname=?` 命中 → 返回旧 token。
   - 未命中 → 按 §5.2 创建用户并生成 token。
3. **不要求**鉴权 middleware、`/api/users/me`、role 区分逻辑（这些在 Task B 做）。
4. 文件里加注释 `// TODO(B2): 替换为带 middleware 的完整实现`。

### 关键要求

- `/health` 只检查进程存活。
- `/ready` 需要检查 MySQL 和 Redis。
- 统一响应必须符合 `contract-v2.md`。
- 日志至少包含 `request_id`、method、path、status、latency。

### 禁止修改

- 不实现用户、拍卖、出价、订单业务。
- 不定义和合同冲突的错误码。

### 验收

```powershell
go test ./...
go run .
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

预期：

- 测试通过。
- `/health` 返回 `{"status":"ok"}`。
- `/ready` 在依赖正常时返回 mysql 和 redis 均为 `ok`。

---

## Task B: 用户与认证接口（B2 真鉴权，D2）

> **拆分说明**：Task B 在 v1.1 拆为 B1 + B2。
> - **B1（mock login 桩）**：已合并入 Task A，D1 EOD 交付。仅返回 token，不做鉴权。
> - **B2（真鉴权 middleware）**：本任务，D2 交付。替换 D1 的桩，加上 `/api/users/me` 和完整鉴权。

### 目标

实现 mock 登录的**完整版**和当前用户查询，为其他接口提供可用 token 和鉴权上下文。
替换 D1 的 B1 桩，**不得改变**桩的 token 格式与种子数据（B/C 已经在用）。

### 负责范围

- 用户表读写。
- mock token 生成和持久化。
- 鉴权 middleware 接入。
- 当前用户上下文。

### 接口

- `POST /api/login`
- `GET /api/users/me`

### 关键要求

- 首次昵称登录创建用户。
- 同一昵称再次登录返回旧 token。
- `GET /api/users/me` 必须校验 `Authorization: Bearer <token>`。
- 昵称为空或超长返回 `1001`。
- token 无效返回 `1002`。

### 禁止修改

- 不实现拍卖、出价、订单。
- 不改变公共响应结构。

### 验收

```powershell
go test ./internal/auth/... ./internal/user/...
curl -X POST http://localhost:8080/api/login -H "Content-Type: application/json" -d "{\"nickname\":\"买家张三\",\"avatar\":null}"
curl http://localhost:8080/api/users/me -H "Authorization: Bearer mock-token-xxx"
```

预期：

- 登录返回 token 和 user。
- 使用有效 token 可查询当前用户。
- 无 token 返回 `401` 和 `code=1002`。

---

## Task C: 拍卖管理接口

### 目标

实现拍卖创建、修改、列表、详情和取消，给出价、状态、WebSocket 模块提供拍卖基础数据。

### 负责范围

- 拍卖创建校验。
- 未开始拍卖修改。
- 拍卖列表分页。
- 拍卖详情。
- 商家取消拍卖。
- 取消时写 outbox 事件 `AuctionCancelled`。

### 接口

- `POST /api/auctions`
- `PUT /api/auctions/:id`
- `GET /api/auctions`
- `GET /api/auctions/:id`
- `POST /api/auctions/:id/cancel`

### 关键要求

- 创建拍卖时 `start_time` 必须晚于服务端当前时间。
- `end_time = start_time + duration_seconds`。
- `current_price` 初始值应等于 `start_price`。
- 支持 `description`、`images`、`stream_url`，字段校验必须与 `contract-v2.md` 一致。
- 仅 `status=pending` 且未到达 `start_time` 的拍卖允许卖家修改。
- 只有卖家本人可以取消。
- 仅 `pending` / `active` 可以取消。
- 取消成功必须写 outbox，不能直接依赖 WebSocket 是否在线。

### 禁止修改

- 不实现出价。
- 不实现自动开始/结束。
- 不直接调用 WebSocket 广播内部实现。

### 验收

```powershell
go test ./internal/auction/...
curl -X POST http://localhost:8080/api/auctions -H "Authorization: Bearer mock-token-seller-001" -H "Content-Type: application/json" -d "{\"title\":\"天然翡翠吊坠\",\"start_price\":90000,\"price_step\":5000,\"start_time\":\"2026-05-29T20:00:00+08:00\",\"duration_seconds\":300}"
curl -X PUT http://localhost:8080/api/auctions/1 -H "Authorization: Bearer mock-token-seller-001" -H "Content-Type: application/json" -d "{\"description\":\"和田玉籽料\",\"images\":[\"https://example.com/1.jpg\"],\"stream_url\":\"https://live.example.com/room/1.flv\"}"
curl http://localhost:8080/api/auctions
curl http://localhost:8080/api/auctions/1
curl -X POST http://localhost:8080/api/auctions/1/cancel -H "Authorization: Bearer mock-token-seller-001"
```

预期：

- 创建、列表、详情字段符合合同。
- pending 拍卖修改后返回更新后的 `data.auction`。
- 非卖家取消返回 `403` 和 `code=1003`。
- 取消成功后 outbox 存在 `AuctionCancelled`。

---

## Task D: 出价核心接口

### 目标

实现高并发出价接口，保证价格单调递增、幂等、延时和事件写入正确。

### 负责范围

- `Idempotency-Key` 校验和持久化。
- Redis 锁 `bid_lock:{auctionId}`。
- MySQL 事务内条件更新。
- accepted / rejected bid 写入。
- 拍卖延时。
- outbox 事件 `BidAccepted`、`AuctionExtended`。

### 接口

- `POST /api/auctions/:id/bid`

### 关键要求

- 必须携带 `Idempotency-Key`。
- 同一用户、同一拍卖、同一 key、同一 amount 返回第一次结果。
- 同一 key 但 amount 不同返回 `1005`。
- 事务内必须再次校验：
  - `status='active'`
  - `end_time > NOW()`
  - `current_price + price_step <= amount`
  - `ceiling_price IS NULL OR amount <= ceiling_price`
- Redis 锁只能削峰，不能作为唯一正确性来源。
- 成功出价必须递增 `auction.version`。
- 触发延时时必须更新 `end_time` 并写 `AuctionExtended`。

### 禁止修改

- 不实现 WebSocket 连接管理。
- 不创建订单。
- 不推进拍卖 pending / ended 状态。

### 验收

```powershell
go test ./internal/bid/...
go test ./internal/bid/... -run Concurrent
curl -X POST http://localhost:8080/api/auctions/1/bid -H "Authorization: Bearer mock-token-user-001" -H "Idempotency-Key: bid-demo-001" -H "Content-Type: application/json" -d "{\"amount\":95000}"
```

预期：

- 低价返回 `2101`。
- 超封顶返回 `2102`。
- 重复相同幂等请求返回同一结果。
- 并发测试中最终 accepted bids 的金额严格递增。
- 成功出价写入 outbox。

---

## Task E: 拍卖状态、历史和事件补偿接口

### 目标

为 Web 和移动端提供状态恢复、历史出价和 WS 断线补偿能力。

### 负责范围

- 当前拍卖快照。
- accepted bid 排行和历史。
- outbox/event 表事件查询。
- `snapshot_required` 判断。

### 接口

- `GET /api/auctions/:id/status`
- `GET /api/auctions/:id/bids`
- `GET /api/auctions/:id/events`

### 关键要求

- `/status` 返回 `auction`、`top_bids`、`last_event_seq`、`server_time`。
- `/bids` 只返回 `status=accepted`。
- `/events?after_seq=` 返回同一 auction 内更大的事件。
- `after_seq` 太旧无法补齐时返回 `snapshot_required=true`。
- 所有时间以服务端时间为准。

### 禁止修改

- 不处理出价写入。
- 不处理 WebSocket 连接。

### 验收

```powershell
go test ./internal/auction/... ./internal/outbox/...
curl http://localhost:8080/api/auctions/1/status
curl http://localhost:8080/api/auctions/1/bids?limit=20
curl http://localhost:8080/api/auctions/1/events?after_seq=0
```

预期：

- `/status` 可用于客户端完整重建房间状态。
- `/events` 返回按 `seq` 升序排列的事件。
- 不存在拍卖返回 `404` 和 `code=2001`。

---

## Task F: WebSocket 实时网关

### 目标

实现拍卖房间实时连接、心跳、快照推送和事件广播。

### 负责范围

- `/ws/auction/:id`。
- 房间连接管理。
- 匿名只读连接。
- token 用户识别。
- `ping` / `pong`。
- 建连后 `snapshot`。
- 根据 outbox publisher 输入广播事件。
- `last_seq` 重连补偿。
- 房间 `viewer_count` 估算和节流广播。

### 接口

- `GET /ws/auction/:id?token=<token>&last_seq=<seq>`

### 关键要求

- 服务端事件必须包含 `type`、`event_id`、`auction_id`、`seq`、`server_time`、`data`。
- 同一 auction 内事件按 `seq` 单调递增。
- 客户端 60 秒无心跳则关闭连接。
- 连接建立后必须发送 `snapshot` 或补偿事件。
- 广播必须允许重复投递，客户端靠 `seq` 去重。
- `viewer_count` 是软事件，缺失时客户端不应触发事件补偿。

### 禁止修改

- 不直接写 bids、orders、auctions 业务表。
- 不在 WS handler 内实现出价。

### 验收

```powershell
go test ./internal/realtime/...
```

手工验收：

- 建立 WS 连接后收到 `snapshot`。
- 发送 `{"type":"ping"}` 收到 `pong`。
- 模拟 outbox 事件后，同房间客户端收到广播。
- 在线人数变化时，同房间客户端收到节流后的 `viewer_count`。
- 不同房间不会串消息。

---

## Task G: Outbox Publisher

### 目标

实现数据库事件到 WebSocket 广播的可靠发布链路。

### 负责范围

- 扫描 `event_outbox`。
- pending 事件发布。
- 成功后标记 `published`。
- 失败重试。
- 批量处理和退出控制。

### 关键要求

- publisher 可以重复发布同一事件。
- WebSocket 层和客户端必须按 `event_id` / `seq` 去重。
- publisher 崩溃重启后能继续处理 pending 事件。
- outbox 堆积需要暴露指标或日志。

### 禁止修改

- 不直接执行业务状态变更。
- 不生成新的业务事件，只发布已存在事件。

### 验收

```powershell
go test ./internal/outbox/...
```

预期：

- pending 事件会被发布并标记 published。
- 发布失败时仍保持 pending 或 retry 状态。
- 重启 publisher 后未发布事件继续处理。

---

## Task H: Lifecycle Worker

### 目标

实现多实例安全的拍卖自动开始、结束和订单创建。

### 负责范围

- pending 到 active。
- active 到 ended。
- 成交订单创建。
- 流拍处理。
- outbox 事件 `AuctionStarted`、`AuctionEnded`。
- 多实例并发防重。

### 关键要求

- 不能依赖单进程 ticker 的唯一性。
- 结束拍卖必须在事务内锁定 auction 行。
- 有 leader 时创建订单。
- `orders.auction_id` 必须唯一。
- 重复 worker 并发执行不能重复创建订单。
- 状态变化必须写 outbox。

### 禁止修改

- 不处理用户主动出价。
- 不处理支付。
- 不直接操作 WebSocket 连接。

### 验收

```powershell
go test ./internal/worker/...
go test ./internal/worker/... -run Concurrent
```

预期：

- 到开始时间的拍卖会变为 active。
- 到结束时间的拍卖会变为 ended。
- 有赢家时只创建一条订单。
- 多 worker 并发测试不重复生成订单和 ended 事件。

---

## Task I: 订单接口

### 目标

实现成交后的订单查询和模拟支付。

### 负责范围

- 买家订单列表。
- 卖家订单列表。
- 订单详情。
- 模拟支付。
- 支付幂等。

### 接口

- `GET /api/orders/mine`
- `GET /api/orders/seller`
- `GET /api/orders/:id`
- `POST /api/orders/:id/pay`

### 关键要求

- 只有买家或卖家本人可看订单详情。
- 只有 winner 可以支付。
- 支付必须携带 `Idempotency-Key`。
- 已支付订单重复支付返回当前订单，`code=0`。
- 非赢家支付返回 `403` 和 `code=1003`。

### 禁止修改

- 不在订单接口内结束拍卖。
- 不创建拍卖成交订单，订单由 Lifecycle Worker 创建。

### 验收

```powershell
go test ./internal/order/...
curl http://localhost:8080/api/orders/mine -H "Authorization: Bearer mock-token-user-001"
curl http://localhost:8080/api/orders/seller -H "Authorization: Bearer mock-token-seller-001"
curl -X POST http://localhost:8080/api/orders/1/pay -H "Authorization: Bearer mock-token-user-001" -H "Idempotency-Key: pay-demo-001"
```

预期：

- 买家只能看到自己的订单。
- 卖家只能看到自己卖出的订单。
- 支付幂等。

---

## Task K: 资源上传接口

### 目标

实现商家端商品图片上传，为拍卖创建和修改提供 `cover_url` / `images` 可用 URL。

### 负责范围

- multipart 文件接收。
- 图片类型和大小校验。
- 本地磁盘或对象存储落盘。
- 返回统一上传结果。

### 接口

- `POST /api/uploads`

### 关键要求

- 必须登录。
- 仅接受 `image/jpeg`、`image/png`、`image/webp`。
- 单文件大小 <= 5MB。
- 响应必须包含 `url`、`width`、`height`、`size`。
- MVP 可以落本地磁盘，但 URL 形态必须稳定，方便前端回填到拍卖创建/修改接口。

### 禁止修改

- 不直接创建或修改拍卖。
- 不绕过统一响应结构。

### 验收

```powershell
go test ./internal/upload/...
curl -X POST http://localhost:8080/api/uploads -H "Authorization: Bearer mock-token-seller-001" -F "file=@./fixtures/product.jpg"
```

预期：

- 合法图片返回 `code=0` 和上传元信息。
- 未登录返回 `401` 和 `code=1002`。
- 非图片或超大文件返回 `400` 和 `code=1001`。

---

## Task J: 限流、压测和稳定性验收

### 目标

验证系统在高并发、弱网、多实例场景下符合 `contract-v2.md` 的稳定性目标。

### 负责范围

- 出价限流。
- WS 连接限制。
- 状态轮询限流。
- 并发出价压测脚本。
- 多实例 worker 测试。
- outbox 堆积测试。
- 日志和指标检查。

### 关键要求

- 出价限流建议：用户 + 拍卖维度 3 次/秒。
- 状态轮询限流建议：IP + 拍卖维度 2 次/秒。
- WS 连接限制建议：用户最多 5 条，IP 最多 100 条。
- 压测必须验证最终价格单调递增。
- 压测必须验证订单不重复。

### 禁止修改

- 不改业务规则来适配压测。
- 不降低合同要求。

### 验收

```powershell
go test ./...
```

压测验收目标：

- 单拍卖房间 1000 人在线，WS 可正常广播。
- 单拍卖房间 200 QPS 出价请求，最终 accepted bids 金额严格递增。
- 出价接口 P95 小于 200ms，P99 小于 500ms。
- WS 断线重连后 3 秒内恢复快照。
- 多实例 worker 同时运行时不重复创建订单。

---

## 最终集成验收流程

完成所有任务后，按以下顺序验收完整项目：

1. 启动 MySQL 和 Redis。
2. 执行 schema/migration。
3. 启动后端服务。
4. 登录卖家和买家。
5. 卖家创建一个即将开始的拍卖。
6. Lifecycle Worker 将拍卖推进到 active。
7. Web 和移动端连接同一拍卖房间 WS。
8. 多个买家并发出价。
9. 客户端收到 `bid_update` 和 `auction_extended`。
10. 拍卖结束后生成订单。
11. 买家查询订单并支付。
12. 断开 WS 后用 `last_seq` 重连，验证事件补偿。

最终命令：

```powershell
go test ./...
```

最终结果必须满足：

- 所有测试通过。
- REST 响应符合 `contract-v2.md`。
- WS 事件符合 `contract-v2.md`。
- 出价并发一致。
- 订单不重复。
- 断线可恢复。

