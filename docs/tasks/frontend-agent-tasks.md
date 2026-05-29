# Frontend Agent Tasks

> 本任务单用于把前端开发拆给 Role B（移动端 H5）和 Role C（PC 商家后台）独立执行。
> 所有 agent 必须以 `docs/contract-v2.md` 为唯一接口合同，以 `docs/api/openapi.yaml` 生成 TS 类型。
> 错误码映射严格按合同 §1.2。
>
> **D1 起步策略（v1.1 解耦）**：B/C 不等 Role A，直接用 MSW 拦截 axios，返回符合 openapi.yaml 的 fixture。
> 每个模块在对应后端接口合并到 main 之后，按 `integration-protocol.md §4` checklist 切真接口。
> MSW handlers 推荐用 `msw` + `@mswjs/data` 或 openapi 样例直出，B/C 共用同一份 fixture（放在仓库根 `fixtures/` 或各自 `src/mocks/`）。

---

## 0. 全局协作规则

### 0.1 技术栈

| 项 | 选择 |
|---|---|
| 构建 | 当前脚手架为 Vite 8 + TypeScript 6 |
| 框架 | 当前脚手架为 React 19 |
| 状态 | Zustand（移动端） + React Context（PC） |
| 样式 | 移动端按现有 CSS/后续 Tailwind 引入；PC 当前为 Ant Design 6 |
| HTTP | Axios + 拦截器 |
| WS | 原生 WebSocket（禁用 socket.io） |
| 时间 | dayjs + timezone 插件，统一 `Asia/Shanghai` |
| 动画 | framer-motion（禁用 lottie） |

### 0.2 目录边界

```text
auction-system/
├── mobile-h5/                 # Role B 主，Role C 配合
│   └── src/
│       ├── pages/             # 登录 / 列表 / 详情 / 订单
│       ├── components/        # BidButton / Countdown / Toast
│       ├── hooks/             # useAuctionSocket / useServerTime / useAuctionAlerts
│       └── store/             # Zustand
└── admin-web/              # Role C 独立
    └── src/
        ├── pages/             # 登录 / 拍卖发布 / 我的拍卖 / 订单 / 监控
        ├── components/
        └── lib/
```

> `packages/shared/` 不是当前仓库启动前置条件。Role C 可以在 P6 中新增该目录，或先在两个前端内各自落地最小 `api/auth/time/error-codes`，联调稳定后再抽取。

### 0.3 必须遵守

- 不允许私自修改接口字段、错误码、WS 事件格式。
- 倒计时必须用 `server_time` + 本地偏移，禁用 `Date.now()`。
- 出价失败以 HTTP 响应为准，不等 WS。
- `viewer_count` 是软事件，缺失不触发 `/events` 补偿。
- token 存 localStorage，每个请求自动加 `Authorization`、`X-Request-Id`、`X-Client-Type`。
- 写接口必须生成 `Idempotency-Key`。

---

## 移动端 H5 任务（Role B 主）

### Task M1: 项目脚手架 + MSW 起步

**负责**: Role B
**预计**: D1 全天（含 MSW 接入）

#### 范围
- `mobile-h5/` 基于现有 Vite + TS + React 脚手架继续开发。
- 接入本地 `src/lib`（与 admin-web 同名同结构）；D3 联调稳定后由 C 抽 `packages/shared/`。
- 路由：`/login` `/auctions` `/auctions/:id` `/orders` `/orders/:id`。
- 全局 `<AuthGate>`、`<ToastHost>`、`<NetworkStatusBar>`。
- **MSW 接入**：
  - `src/mocks/handlers.ts` 覆盖 `/api/login`、`/api/users/me`、`GET /api/auctions`、`GET /api/auctions/:id`、`GET /status`。
  - fixture 取自 openapi.yaml 的 `example` 字段；token 用种子 token。
  - 环境变量 `VITE_USE_MSW=true` 控制是否启用；默认开。

#### 验收
```powershell
cd mobile-h5
npm.cmd install
npm.cmd run dev   # 启动 http://localhost:5173
```
- 视口 375，能跑通空白首页。
- TS 严格模式无 error。
- Axios 拦截器命中 token 注入。
- DevTools Network 能看到 MSW 拦截标识；登录页能用 MSW 拿到种子 token。

---

### Task M2: 登录页 + 鉴权

**负责**: Role C（共享组件，写一次给两端用）

#### 范围
- `<LoginPage>`：单输入框 nickname，提交调 `POST /api/login`。
- `AuthContext`：`user`、`token`、`login()`、`logout()`。
- 401 全局拦截 → 跳 `/login`。
- 区分买家/卖家：买家在 nickname 含「买家」前缀或登录页选角色 tab；PC 后台只允许卖家。

#### 验收
- 输入「买家张三」首次登录创建用户，返回 token。
- 同昵称再次登录返回旧 token。
- 移动端和 PC 后台都能跑通。

---

### Task M3: 拍卖列表页（移动端）

**负责**: Role C

#### 范围
- `/auctions` 调 `GET /api/auctions?status=active&page=&size=`。
- 卡片：封面 + 标题 + 当前价 + 剩余时间 + viewer_count。
- 下拉刷新 + 上拉加载。
- 空状态、加载状态、错误状态。

#### 验收
- 拍卖卡片每 5 秒刷新一次列表（active 状态）。
- 点击进入 `/auctions/:id`。

---

### Task M4: 拍卖详情/直播间页（核心）

**负责**: Role B
**预计**: D5–D7

#### 范围
布局（375 宽）：
- 顶部：返回 + viewer_count + 网络状态指示器
- 中部：直播视频（stream_url，空则占位）
- 浮层：当前价 / 倒计时 / current_leader（👑 自己）
- 出价按钮（固定底部）+ 自定义出价金额输入
- 出价记录列表（fade-in）

#### 关键要求（合同 §4.1）
- 倒计时用 `server_time` 偏移计算。
- 出价按钮：`idle → pending(HTTP 中) → cooldown(1s) → idle`。
- 出价失败以 HTTP 响应为准，错误码映射：
  - `2101` → 「出价低于最低有效价 ¥{{min_acceptable_amount}}」并预填
  - `2102` → 「已达封顶价 ¥{{ceiling_price}}」
  - `2103` → 自动刷新快照后允许一次重试
  - `1004` → 按钮禁用 1 秒
- 收到 `auction_ended` 立即禁用出价、弹成交模态。

#### 验收
- 手动 + 自动出价均可成功。
- 弱网断开 WS 后 1s 刷新一次 `/status`，5s 内能恢复广播。
- 倒计时与服务端误差 ≤500ms。

---

### Task M5: useAuctionSocket / useServerTime / ConnectionManager

**负责**: Role B
**预计**: D4–D5

#### 范围
- `useAuctionSocket(auctionId)`：连 WS、按 seq 去重、缺事件调 `/events`、`snapshot_required=true` 时调 `/status`。
- `useServerTime()`：用 `/status.server_time` 校准本地偏移。
- `ConnectionManager`：状态机 `connected → reconnecting → polling → connected`，退避 1s/2s/5s/10s/10s。
- `visibilitychange`：回前台必调 `/status`。

#### 验收
- 关闭浏览器网络后 UI 显示「同步中」。
- 恢复网络后 3 秒内重新收到事件。
- 后台 30 秒回前台触发 `/status`。

---

### Task M6: useAuctionAlerts（氛围提醒）

**负责**: Role B 写 hook，Role C 出动画 & 音效

#### 范围
输入 WS 事件流，输出 `Alert[]`：
- 被超越：上条 leader 是我、本条不是 → 震动 + 红光闪 + 「⚡ 被超越，加 ¥{{diff}} 反超」
- 即将结束：剩余 ≤10s → 心跳缩放 + 滴答音
- 延时触发：`auction_extended` → 顶部冲击波 + 「⏰ 延时 30s」
- 成交：`auction_ended` → 撒花 + 赢家头像放大

#### 关键要求
- 音效用 Web Audio API（不依赖外部包）。
- iOS Safari：首次出价时 unlock audio context。
- 尊重 `prefers-reduced-motion`。

#### 验收
- `/demo` 路由可逐个触发四种动画用于录视频。
- 60fps，无 layout 抖动。

---

### Task M7: 我的订单 + 模拟支付

**负责**: Role C

#### 范围
- `/orders` 调 `GET /api/orders/mine`，状态 tab。
- `/orders/:id` 详情。
- 支付按钮调 `POST /api/orders/:id/pay`，必带 `Idempotency-Key`。

#### 验收
- 支付成功后状态变 `paid`。
- 重复点支付仍返回同一订单。

---

### Task M8: 上传组件 useUpload

**负责**: Role B（前后端一手）

#### 范围
- 前端校验类型（jpeg/png/webp）+ 大小（≤5MB）。
- 进度条 + 失败重试一次。
- 后端见 backend Task K。

#### 验收
- 在拍卖发布表单和移动端均可调用。
- 返回 url 可在 `<img>` 直接渲染。

---

## PC 商家后台任务（Role C）

### Task P1: 项目脚手架 + MSW + 共享 lib

**预计**: D1 全天

#### 范围
- `admin-web/` 基于现有 Vite + TS + React + AntD 脚手架继续开发。
- `src/lib/{api-client,auth,time,types,error-codes}.ts`（与 mobile-h5 同名同结构；D3 后由 C 决定是否抽 `packages/shared/`）。
- `src/lib/types.ts` 用 `openapi-typescript` 从 `docs/api/openapi.yaml` 自动生成。
- 最小宽度 1280，左侧导航 + 右侧内容布局。
- 路由：`/login` `/auctions/new` `/auctions/:id/edit` `/auctions` `/orders` `/monitor/:id`。
- **MSW 接入**：
  - `src/mocks/handlers.ts` 与 mobile-h5 共用 fixture（建议放仓库根 `fixtures/auctions.json` 等，两端 import）。
  - 至少覆盖 `/api/login`、`/api/auctions`（POST/GET/PUT）、`POST /api/uploads`（返回固定 url）。
  - `VITE_USE_MSW=true` 控制启用。

#### 验收
```powershell
cd admin-web
npm.cmd install
npm.cmd run dev   # http://localhost:5174
```
- TS 严格模式无 error；`types.ts` 与 openapi.yaml 同步。
- MSW 起，登录页用种子 token 跑通；发布表单提交看到 MSW 拦截到的成功响应。

---

### Task P2: 拍卖发布表单

**预计**: D4–D5

#### 范围
合同 §2.2 全部字段：
- 分三栏：基础信息 / 价格规则 / 时间规则
- 图片上传组件调 `/api/uploads`
- 「ceiling_price 留空 = 无封顶」明显提示
- 「start_price=0」高亮提示「将以 price_step 作为首笔最低有效价」
- 提交前预览弹窗
- 错误码 1001/2002/2103 中文映射

#### 验收
- 创建后跳「我的拍卖」并高亮新行。
- 字段校验全部命中合同 §2.2 规则。
- 时间组件默认现在 +10 分钟。

---

### Task P3: 我的拍卖列表与管理

**预计**: D5

#### 范围
- `GET /api/auctions?seller_id=me`，状态 Tab。
- 表格列：封面 / 标题 / 当前价 / 状态 / 剩余时间 / 出价数 / 操作。
- pending 行：「修改」「取消」。
- active 行：「取消」「进直播间」（跳 `/monitor/:id`）；「修改」按钮置灰 + tooltip。
- 取消二次确认，文案明确「将向所有买家广播」。

#### 验收
- 剩余时间每秒刷新，用 server_time 偏移。
- 取消调 `POST /:id/cancel` 后立即从列表反映。

---

### Task P4: 卖家订单列表

**预计**: D6 半天

#### 范围
- `GET /api/orders/seller`，按状态过滤。
- 列：成交价 / 买家 / 状态 / 时间。
- 点击查看详情。

---

### Task P5: PC 直播间监控页

**预计**: D6 半天

#### 范围
- 复用移动端的 `useAuctionSocket`，只读视图。
- 实时显示：当前价、leader、出价流、viewer_count、剩余时间。
- 卖家可一键取消（active 状态）。

#### 验收
- 与移动端买家页同步事件 ≤500ms。

---

### Task P6: 共享前端工具

**预计**: D3

#### 范围
- 先在 `admin-web/src/lib/` 与 `mobile-h5/src/lib/` 落地同名工具，保证两个 npm 项目都能独立启动。
- `api-client.ts`：Axios 实例 + Idempotency-Key 生成 + 错误码拦截。
- `auth.ts`：token 存取、AuthContext。
- `time.ts`：dayjs + server_time 偏移工具。
- `error-codes.ts`：合同 §1.2 全部错误码 → 中文。
- 后续如需要减少重复，再新增 `packages/shared/` 并把两个前端切到 file dependency 或 workspace。

#### 验收
- mobile-h5 和 admin-web 都能 import 本地 `src/lib` 并通过 TS 编译。

---

## 体验打磨与文档任务（Role C）

### Task X1: AI 使用归档
见 `prompts/role-C-merchant-ai-polish.md` 场景 5。**贯穿全期**，每天 EOD 补 `docs/ai-log/yyyy-mm-dd.md`。

### Task X2: 答辩材料
见 `prompts/role-C-merchant-ai-polish.md` 场景 6。D10–D11 集中产出。

---

## 集成顺序

```text
D3: Role C 交付 P6 共享库脚手架
    Role B 起 M1 移动端骨架
    Role C 起 P1 PC 骨架 + M2 登录页

D4: M5 WS hook + M8 上传
    P2 拍卖发布表单

D5: M4 直播间页（核心）
    P3 我的拍卖列表

D6: M6 氛围提醒
    P4 订单 + P5 监控页
    M3/M7 移动端列表 + 订单

D7 联调日 1
```

---

## 反模式（请 AI 避免）

- 倒计时用 `setInterval(fn, 1000)`（漂移；用 `requestAnimationFrame` + server_time 偏移）。
- 出价成功立刻更新本地 leader（必须等 HTTP 响应）。
- WS 用 socket.io（合同明确裸 WS）。
- viewer_count 缺失触发 `/events` 补偿（软事件）。
- 商家后台用 Tailwind 自己撸（直接上 AntD）。
- 氛围动画用 lottie（太重）。
- 把 token 写在 URL query（除 WS 外）。

