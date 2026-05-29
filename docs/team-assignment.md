# 团队分工与任务对照表

> 锁定日期：2026-05-29
> 三人小组：Role A / Role B / Role C
> 本文件是「人 ↔ 模块 ↔ Task ↔ 验收命令」的唯一对照表。
> 当 `prompts/role-*.md` 与本文件冲突时，以本文件为准。

---

## 1. 角色总览

| 角色 | 主线职责 | 兼顾 | 工时占比（估）|
|---|---|---|---|
| **Role A** | 后端核心：出价 / 状态机 / outbox / worker / 订单 / DDL / 压测 | DevOps 起底（docker-compose、CI） | 后端 100% |
| **Role B** | 实时层 + 移动端 H5：WS 网关 / `/status` / `/events` / 上传 / 移动端竞拍页 | 弱网重连、氛围提醒 | 后端 40% + 前端 60% |
| **Role C** | PC 商家后台 + 体验打磨 + AI 归档 + 答辩材料 | 登录 mock、移动端列表/详情静态层、动画 | 前端 80% + 文档 20% |

---

## 2. 后端 Task A–K 归属

来源：`docs/tasks/backend-agent-tasks.md`。

| Task | 模块 | 负责人 | 备注 |
|---|---|---|---|
| A | 基础工程（启动、router、配置、health/ready） | **Role A** | D1 完成；**含 mock login 桩 + 3 个种子 token**（解耦 B/C） |
| B | 用户与认证（/login、/users/me、middleware） | **Role A** | 拆分：B1 mock 桩（D1，10 行，归 Task A）；B2 真鉴权 middleware（D2，替换桩） |
| C | 拍卖管理（创建/修改/列表/详情/取消） | **Role A** | |
| D | 出价核心（/bid + Redis 锁 + 条件更新 + outbox） | **Role A** | 项目最高难度，重点投入 |
| E | 状态/历史/事件补偿（/status、/bids、/events） | **Role B** | 与 WS 网关同模块，归 B |
| F | WebSocket 网关（/ws/auction/:id） | **Role B** | |
| G | Outbox Publisher | **Role A** | 与 D 同事务边界，归 A |
| H | Lifecycle Worker | **Role A** | |
| I | 订单接口（mine/seller/:id/pay） | **Role A** | |
| K | 资源上传（/uploads） | **Role B** | 前后端都在 B 手里，避免联调 |
| J | 限流 + 压测 + 稳定性验收 | **Role A 主导，B/C 配合录屏** | D9–D10 集中做 |

> 划分原则：A 把控数据一致性主链路（D→G→H→I），B 把控"事件出口 + 移动端入口"（E→F→K + 移动端），C 不碰核心后端。

---

## 3. 前端模块归属

详细任务单见 `docs/tasks/frontend-agent-tasks.md`。

### 3.1 移动端 H5（`mobile-h5/`）— Role B 主，Role C 配合

| 模块 | 负责人 | 说明 |
|---|---|---|
| 项目脚手架（Vite + TS + Tailwind + Zustand） | Role B | |
| 登录页 + AuthContext + Axios 拦截器 | **Role C** | 与 PC 后台共用 lib，C 写一次 |
| 拍卖列表页 | **Role C** | 静态层 + 接口对接 |
| 拍卖详情/直播间页（核心） | **Role B** | WS + 倒计时 + 出价按钮 |
| useAuctionSocket / useServerTime / ConnectionManager | **Role B** | 弱网重连封装 |
| useAuctionAlerts（被超越/即将结束/延时/成交） | **Role B 写 hook，Role C 出动画素材** | |
| 我的订单页 + 模拟支付 | **Role C** | |
| 上传组件 useUpload | **Role B** | |

### 3.2 PC 商家后台（`admin-web/`）— Role C 独立负责

| 模块 | 负责人 |
|---|---|
| 项目脚手架（现有 Vite + TS + React + AntD） | Role C |
| 登录页（复用 mobile-h5 的 auth lib） | Role C |
| 拍卖发布表单（创建 + 修改） | Role C |
| 我的拍卖列表（含状态 Tab、取消、剩余时间） | Role C |
| 卖家订单列表 | Role C |
| PC 直播间监控页（只读看拍卖进行） | Role C |

### 3.3 共享前端工具 — Role C 维护

- 当前仓库没有 workspace，两个前端先在各自 `src/lib/` 下维护同名工具文件。
- `api-client.ts`：Axios 实例 + 错误码映射 + Idempotency-Key 生成。
- `auth.ts`：token 存取、AuthContext。
- `time.ts`：dayjs + server_time 偏移。
- `types.ts`：从 `openapi.yaml` 生成或手写最小 TS 类型。
- 如后续确实需要复用，再由 Role C 新增 `packages/shared/` 并统一迁移。

---

## 4. 跨人依赖与解锁顺序

**解耦策略**：通过 `integration-protocol.md §3` 的 mock-first 框架，D1 起三人并行，不再串行等待。

```text
D1: A 交付 docker-compose + Task A (基础工程) + B1 mock login 桩 + 种子 token
    B 启动 mobile-h5 脚手架 + MSW handlers（吃 openapi.yaml）+ useAuctionSocket 骨架
    C 启动 admin-web 验证 + src/lib + openapi-typescript 生成类型 + MSW handlers
        ↓
D2: A 交付 B2 真鉴权 middleware（替换 D1 桩）+ Task C 拍卖 CRUD happy path
    B 移动端登录页 + 列表页（仍走 MSW，A 接口好了按模块切真）
    C PC 登录页 + 拍卖发布表单（仍走 MSW）
        ↓
D3: A Task C 完成 + 启动 Task D 出价核心
    B 切真 /api/auctions 接口；M8 上传 + M5 ConnectionManager
    C 切真 /api/auctions 接口；P2 发布表单接真
        ↓
D4–D6: 三人并行（见 milestones.md），按 integration-protocol.md §3 表逐项切真
        ↓
D7: 联调日 1
        ↓
D8–D9: Role A 主导压测 J，B/C 配合
        ↓
D10–D11: 联调日 2 + 演示视频
        ↓
D12 (6.10): 答辩
```

**关键解锁点（更新）**：D1 EOD 前必须合并到 `main`：
1. `docker-compose.yml` + schema 自动初始化
2. `/health`、`/ready`、`POST /api/login` 桩（返回种子 token）
3. `openapi.yaml` 三人确认锁定（B/C 据此生成 MSW handlers 和 TS 类型）

> 任一项缺失 → 次日全员补救。**B2 真鉴权晚一天没关系，桩在 D1 必须有。**

### 4.1 模块级切真接口流程

参考 `integration-protocol.md §4` 的 checklist。每个前端模块（登录、列表、详情、订单…）按以下顺序：

```text
1. 后端模块 PR 合并到 main
2. 后端 owner 在群里 @ 前端 owner，给出 happy path + 1 个错误码 curl
3. 前端 owner 删除该模块对应的 MSW handler
4. 前端浏览器跑通 200 + code=0
5. 双方在 PR 评论里互相 @ 确认联调通过
```

MSW handler 和真接口同时存在 ≤24 小时（避免漂移）。

---

## 5. 责任边界硬约束

| 不允许 | 原因 |
|---|---|
| Role B/C 改后端 `internal/bid` `internal/order` `internal/worker` | 一致性主链路 Role A 独占 |
| Role A 改前端 `mobile-h5/` `admin-web/` | 减少 review 来回 |
| Role C 改 `internal/realtime` `internal/outbox` | B 的事件出口 |
| 任何人私自改 `docs/contract-v2.md` `docs/schema-v2.sql` `docs/api/openapi.yaml` | 必须三人 review + 改 changelog |
| 任何人私自加错误码 | 用合同 §1.2 已有码，不够先讨论 |

---

## 6. 变更日志

| 日期 | 版本 | 变更 |
|---|---|---|
| 2026-05-29 | v1.0 | 首次发布；锁定后端 Task A–K、前端模块归属与跨人解锁顺序 |
| 2026-05-29 | v1.1 | 解耦 D1–D2 串行依赖：Task B 拆分 B1（D1 mock 桩，归 Task A）+ B2（D2 真鉴权）；§4 改为 D1 三人并行 + 模块级切真接口流程 |

