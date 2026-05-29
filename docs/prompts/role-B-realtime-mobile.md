# 角色 B · 实时网关 / 移动端 H5 — Prompt 模板

> 适用范围：WebSocket 网关、/status 和 /events 接口、移动端 H5、弱网重连、上传接口。
> 使用方式：复制下面对应场景的模板，替换 `{{...}}` 占位符，喂给 AI 编码助手。

---

## 必读上下文（每次新会话开头喂给 agent）

```
请先读以下文件再开始：
1. docs/team-assignment.md                       # 我的边界（不许碰 internal/bid、order、worker）
2. docs/contract-v2.md  §3 (WS) + §2.3 (/events) + §4 (前端要求)
3. docs/events/event-contract.md                 # 事件序号、去重、补偿
4. docs/api/openapi.yaml                         # MSW handlers 和 TS 类型源
5. docs/tasks/frontend-agent-tasks.md（M1–M8）+ backend-agent-tasks.md Task E/F/G/K
6. docs/dev-setup.md  §2, §4, §5.4               # 端口、env、WS token 传参
7. docs/integration-protocol.md  §3, §4          # mock-first 框架 + 切真接口 checklist
```

## D1 起步任务（解耦后不等 A）

```
任务：D1 EOD 前用 MSW 把 mobile-h5 跑起来。
1. mobile-h5 脚手架：Vite + TS + Tailwind + Zustand；视口 375
2. 路由 /login、/auctions、/auctions/:id、/orders、/orders/:id
3. src/lib：api-client (axios + 拦截器)、auth (token + AuthContext)、time (server_time 偏移)
4. types：用 openapi-typescript 从 docs/api/openapi.yaml 生成
5. MSW handlers（src/mocks/handlers.ts）：
   - POST /api/login → 按 nickname 返种子 token（mock-token-buyer-001 等）
   - GET /api/users/me、GET /api/auctions、GET /api/auctions/:id、GET /status
   - fixture 取自 openapi.yaml example 字段
6. VITE_USE_MSW=true 默认开；每个模块切真接口时关掉对应 handler
7. useAuctionSocket hook 骨架：连本地 mock WS server（fixture 事件按 contract §3）

不要在 D1 做：M4 直播间核心 UI、M6 氛围动画、上传。
切真接口的时机：按 integration-protocol §3 表，A 的接口合到 main 后当天切。
```

---

## 通用前置上下文（每次会话开头先贴一次）

```
你正在帮我开发「直播竞拍系统」的实时层与移动端。技术栈：

- 后端：Go 1.22 + gorilla/websocket（与 A 同一进程）
- 前端：现有脚手架为 React 19 + TypeScript 6 + Vite 8，移动端 H5（视口 375）
- 状态：Zustand 或原生 hooks，不引入 Redux
- WS 协议以 docs/contract-v2.md §3 为唯一来源

我负责的模块：WS 网关、/status、/events、移动端 H5、上传接口、弱网重连。
我不负责：出价业务规则、订单、lifecycle worker（A 负责）；PC 商家后台（C 负责）。

回答约束：
1. 所有事件必须按 seq 单调递增处理，客户端按 event_id 去重
2. 倒计时必须用 server_time 计算本地偏移，不能用本地时间
3. 出价失败以 HTTP 响应为准，不等 WS
4. viewer_count 是软事件，缺失不触发补偿
```

---

## 场景 1：WS 网关骨架

```
任务：实现 ws://host/ws/auction/:id?token=&last_seq= 的网关

合同要求（§3）：
- 连接建立后必须先发 snapshot 或缺失事件
- 同 auction_id 的 seq 单调递增
- 客户端 ping 间隔 20-30s，服务端 60s 无心跳关连接
- 房间级广播，匿名 token=空 时只读
- 多实例部署下，房间路由通过 Redis pub/sub 同步

请输出：
1. Hub / Room / Client 三层结构（goroutine 边界）
2. 连接生命周期：握手 → 鉴权 → 加入房间 → 推 snapshot → 循环读写 → 清理
3. 与 A 的 outbox publisher 的对接方式（同进程 channel + Redis pub/sub 兼容多实例）
4. viewer_count 节流策略（≥2s 一次，变化 <±2% 丢弃）

不要写：业务逻辑、出价校验、订单生成。
```

---

## 场景 2：/events 补偿接口

```
任务：实现 GET /api/auctions/:id/events?after_seq=&limit=

合同要求（§2.3）：
- 从 event_outbox 表按 (auction_id, event_seq) 拉取 > after_seq 的事件
- after_seq 太旧无法补齐时返回 snapshot_required=true
- limit 默认 100 最大 500

请输出：
1. SQL（带 limit 和 has_more 判断）
2. 「太旧」的判定阈值（建议：当前 max_seq - after_seq > 1000 或事件已归档）
3. 客户端调用时机：WS 收到 seq > last_seq+1 时触发

注意：viewer_count 事件不应该出现在 /events 响应里（软事件不补偿）。
```

---

## 场景 3：移动端直播间页面

```
任务：实现移动端直播间页面 /auction/:id

布局（375 宽）：
- 顶部：返回 + viewer_count
- 中部：直播视频区（stream_url，空时用本地占位 mp4）
- 下方浮层：当前价 / 倒计时 / current_leader
- 出价按钮：固定底部，禁用态需明确（未开始/已结束/我是 leader）
- 出价记录列表：从下往上滚动，新出价 fade-in

合同要求（§4.1）：
- 倒计时用 server_time 算本地偏移
- 出价按钮点击后进入 pending，直到 HTTP 响应
- 失败以 HTTP 响应为准，不等 WS
- 收到 auction_ended 立即禁用出价

请输出：
1. 组件树和数据流
2. useAuctionSocket hook（封装 WS + seq 校验 + 自动补偿 + 自动重连）
3. useServerTime hook（用 /status 的 server_time 校准 Date.now()）
4. 出价按钮的状态机（idle/pending/cooldown/disabled）

技术栈：React + TS + Zustand + Tailwind。
```

---

## 场景 4：弱网与重连策略

```
任务：实现 §4.2 的弱网重连

要求：
- WS 断开立即调 /status 刷一次
- 重连退避：1s, 2s, 5s, 10s, 10s...
- 重连时携带本地 last_seq
- 15s 无 WS 降级为每 2s 短轮询 /status
- 页面从后台回前台必须调 /status

请输出：
1. 一个 ConnectionManager 类，状态机：connected → reconnecting → polling → connected
2. visibilitychange 监听代码
3. 如何在控制台模拟弱网（mock WS 关闭 + 拒绝重连）测试
4. 状态指示器 UI：连接中 / 已断开 / 同步中

不要用 socket.io，按合同就是裸 WebSocket。
```

---

## 场景 5：上传接口（前端 + 后端）

```
任务：POST /api/uploads（合同 §2.6）

后端：
- multipart/form-data，字段 file
- 仅接受 image/jpeg, png, webp，单文件 ≤5MB
- MVP 落本地磁盘 ./uploads/yyyy/mm/dd/，前端通过 /static/ 访问
- 返回 url + width + height + size

前端（在商家后台和移动端都会用）：
- 上传前本地校验大小和类型
- 显示上传进度
- 失败重试一次

请输出：
1. 后端 handler + 文件名去重策略（hash + 短随机）
2. 前端 useUpload hook（含进度）
3. 安全：拒绝路径穿越、检查 magic number 而不是只看 MIME

注意 url 形态要和未来切 CDN 兼容，前端别 hardcode 域名。
```

---

## 场景 6：出价提醒和氛围

```
任务：实现「被超越 / 即将结束 / 延时触发 / 成交」四种关键提醒

数据来源：WS 事件（bid_update / auction_extended / auction_ended）
触发规则：
- 被超越：上一条 current_leader 是我，本条不是我 → 震动 + toast「⚡ 你被超越了！」
- 即将结束：剩余 ≤10s 时心跳红光 + 滴答音
- 延时触发：收到 auction_extended → toast「⏰ 延时 30s」
- 成交：收到 auction_ended → 模态展示赢家

请输出：
1. 一个 useAuctionAlerts hook，输入 WS 事件流，输出 Alert[]
2. 提示音用 Web Audio API（不要外部依赖）
3. iOS Safari 自动播放限制如何处理（用户首次出价时解锁 audio context）
```

---

## 反模式（请 AI 避免）

- 用本地 Date.now() 计算倒计时
- 用 setInterval 重发 ping（应用层心跳要可暂停）
- 出价成功立即更新本地 leader（必须等 HTTP 响应或 WS 事件）
- 用 socket.io（合同明确裸 WS）
- viewer_count 缺失触发 /events 补偿（软事件）
- 把 token 写在 localStorage 然后塞 query（MVP 可以，但要在代码注释里标 TODO）

