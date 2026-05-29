# 角色 A · 后端核心 / 一致性 — Prompt 模板

> 适用范围：出价、拍卖 CRUD、状态机、lifecycle worker、outbox、订单、索引/DDL、压测。
> 使用方式：复制下面对应场景的模板，替换 `{{...}}` 占位符，喂给 AI 编码助手。

---

## 必读上下文（每次新会话开头喂给 agent）

```
请先读以下文件再开始：
1. docs/team-assignment.md                       # 我的边界与硬约束
2. docs/contract-v2.md                           # 接口合同（唯一事实源）
3. docs/schema-v2.sql                            # DB 基线
4. docs/tasks/backend-agent-tasks.md  §0 + 当前 Task # 具体验收命令
5. docs/dev-setup.md  §3, §5                     # docker-compose、mock token 算法
6. docs/integration-protocol.md  §3              # mock-first 框架
```

## D1 起步任务（解耦后必须最先做）

```
任务：D1 EOD 前交付以下，解锁 B/C 并行开工。
1. docker-compose.yml + schema-v2.sql 自动初始化 + 3 个种子用户
2. Task A 基础工程：/health、/ready、统一响应、错误码、X-Request-Id
3. B1 mock login 桩：POST /api/login（≤30 行）
   - 按 nickname 查 users，命中返旧 token，未命中按 dev-setup §5.2 创建
   - 文件加 // TODO(B2): 替换为带 middleware 的完整实现
4. （可选）MOCK_MODE=true 时未实现的 /api/auctions 等返回 openapi 样例

不要在 D1 做：Task B 真鉴权 middleware、/api/users/me、Task C/D。
B2 真鉴权放 D2，必须不改变 D1 桩的 token 格式（B/C 已经在用）。
```

---

## 通用前置上下文（每次会话开头先贴一次）

```
你正在帮我开发「直播竞拍系统」的后端核心模块。技术栈：

- Go 1.22 + Gin + gorilla/websocket
- MySQL 8.0 + Redis 7
- 不引入 Go-kit / Kratos / go-zero 等重型框架
- 分层：HTTP Handler → Application Service → Domain → Repository
- 复杂事务必须用手写 SQL，不交给 ORM
- 金额单位：分（int64）；时间：ISO-8601 + Asia/Shanghai

接口契约以 docs/contract-v2.md 为唯一来源，禁止偏离。
我负责的模块：出价、拍卖 CRUD、状态机、lifecycle worker、outbox、订单。

回答约束：
1. 先给方案要点（3-5 条），再给代码
2. 关键 SQL 必须显式列出，禁止隐藏在 ORM 里
3. 任何并发/事务相关的代码，必须说明竞态点
4. 不要写无意义注释；注释只写 WHY
```

---

## 场景 1：实现出价接口

```
任务：实现 POST /api/auctions/:id/bid

合同要求（摘自 contract-v2.md §2.3）：
- 必须携带 Idempotency-Key
- 处理顺序：参数校验 → 读拍卖 → 快速校验 → Redis 锁 → MySQL 事务条件更新 → 插 bid → 写幂等记录 → 写 outbox → 提交
- 条件更新 SQL 见合同 §2.3，必须兜底「状态/时间/最低有效价/封顶价」四项
- 封顶价命中时，同一事务内置 ended + 建单 + 写 AuctionEnded outbox，event_seq 连续
- 影响行数=0 时返回 2103
- 同 key 不同 amount 返回 1005

请输出：
1. service 层伪代码（标出每一步对应的合同条款）
2. 完整的 SQL（含条件更新 + outbox 插入）
3. Redis 锁的 Lua 释放脚本
4. 至少 3 个单元测试用例描述（并发出价、幂等重试、封顶成交）

不要写：HTTP handler 模板代码、参数绑定、Swagger 注释。
```

---

## 场景 2：实现 lifecycle worker

```
任务：实现多实例安全的拍卖生命周期 worker

合同要求（§7.5）：
- 不能依赖单进程 ticker
- 启动拍卖：UPDATE ... WHERE status='pending' AND start_time<=NOW() LIMIT 100
- 结束单个拍卖必须事务内：锁行 → 校验 active+到期 → 置 ended → 有 leader 则插 order（orders.auction_id 唯一）→ 写 AuctionEnded outbox

请输出：
1. worker 主循环结构（多实例并行不重复处理的论证）
2. 完整 SQL（启动批处理 + 结束单条事务）
3. 异常处理：DB 抖动、outbox 写失败、order 唯一冲突如何兜底
4. 怎么本地用 docker-compose 起 2 个实例验证不重复建单

注意：orders.auction_id 唯一约束是兜底，不是首选防线。首选防线必须是事务内的状态条件更新。
```

---

## 场景 3：outbox publisher

```
任务：实现 event_outbox 表的 publisher

合同要求（§7.4）：
- 表结构见合同 §7.4
- 扫 pending → 投递到 WS 网关 → 置 published；失败保留重试
- 同 auction 内 event_seq 单调递增；客户端按 event_id 去重，允许重复投递

请输出：
1. publisher 拉取策略：轮询间隔、批大小、单房间顺序保证
2. 与 WS 网关的接口（同进程内 channel？还是 Redis pub/sub？给出权衡）
3. 失败重试和死信策略
4. 监控指标：outbox_pending_total / publish_latency

我倾向：单体内 channel + Redis pub/sub 兼容多实例，请说明哪种更适合 MVP。
```

---

## 场景 4：写 SQL / DDL

```
任务：根据合同 §5 和 §8 生成完整的 schema.sql

要求：
- 表：users / auctions / bids / orders / idempotency_keys / event_outbox
- 索引按合同 §8 一字不差
- 所有金额字段 BIGINT NOT NULL
- 所有时间字段 DATETIME(3)，存 UTC，应用层转 Asia/Shanghai
- 字符集 utf8mb4，引擎 InnoDB
- 必须包含 orders.auction_id UNIQUE、bids 的幂等唯一键、outbox 的 (aggregate_type,aggregate_id,event_seq) 唯一键

输出：可以直接 mysql < 跑通的 schema.sql，附 5 条 seed 数据。
```

---

## 场景 5：写测试

```
任务：为 {{模块名}} 写测试

约束：
- 用 Go testing + testify
- 单测覆盖 domain 层规则，禁止 mock 数据库
- 集成测试用 testcontainers 起真实 MySQL + Redis
- 并发测试至少 100 goroutine 模拟同房间出价，验证 current_price 单调

请输出：
1. 测试矩阵（场景 × 期望结果）
2. 关键并发用例代码
3. 如何在 CI 里跑（不要超过 2 分钟）
```

---

## 场景 6：压测脚本

```
任务：写 k6 压测脚本验证合同 §10.3 验收目标

目标：
- 单房间 200 QPS 出价 5 分钟，最终价格单调递增
- P95 < 200ms，P99 < 500ms
- 1000 并发 WS 连接稳定接收 bid_update

请输出：
1. k6 脚本（bid 接口 + WS 订阅）
2. 如何注入 Idempotency-Key 保证不被去重打死
3. 结果判定脚本：从 DB 读最终 bids 列表验证单调性

不要写：复杂的 Grafana 配置。先跑通再说。
```

---

## 反模式（请 AI 避免）

- 用 `gorm.AutoMigrate` 代替手写 DDL
- 在 handler 里直接写业务规则
- 用单进程 sync.Map 当幂等存储
- 用 `time.AfterFunc` 当 lifecycle ticker
- 用 `SELECT ... FOR UPDATE` + 应用层判断代替条件更新
- 把 Redis 锁当唯一正确性来源

