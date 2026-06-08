# Part 3：Role B 移动端直播间页面工作报告

> 记录日期：2026-06-03  
> 记录人：LSH / Role B  
> 项目：直播竞拍系统 `auction-system`  
> 工作范围：移动端 H5 直播间页面 `/auctions/:id`、实时事件 hook、服务端时间校准、出价按钮状态机

---

## 1. 完成的业务逻辑

本阶段完成的是移动端 H5 的“拍卖直播间核心交互”能力。也就是说，买家现在可以打开指定拍卖房间，在移动端页面中看到直播区域、在线人数、当前价格、倒计时、当前领先者和出价记录；页面会尝试连接 WebSocket 获取实时事件，并在断线或事件缺失时通过 `/events` 与 `/status` 做恢复。

已完成业务逻辑：

- 用户可通过 `/auctions/:id` 进入指定拍卖直播间，例如 `/auctions/1`。
- 页面顶部展示返回按钮、`viewer_count` 在线人数和网络状态。
- 页面中部展示直播视频区域；当 `stream_url` 为空时，使用本地商品图作为直播占位画面。
- 页面浮层展示当前价、倒计时和当前领先者。
- 倒计时不直接依赖本地时间，而是通过 `/api/auctions/:id/status` 返回的 `server_time` 计算本地时间偏移。
- 页面展示出价记录列表，新出价记录通过淡入效果进入列表。
- 底部固定出价区支持自定义金额和“加一手”出价。
- 出价按钮实现 `idle / pending / cooldown / disabled` 状态机。
- 点击出价后按钮进入 `pending`，直到 HTTP 响应返回；出价失败以 HTTP 响应为准，不等待 WS。
- 当拍卖未开始、已结束、已取消、当前用户已经是领先者或请求冷却中时，出价按钮明确禁用并展示原因。
- 收到 `auction_ended` 事件后，页面立即禁用出价并展示成交结果弹窗。
- `viewer_count` 作为软事件处理，只更新在线人数，不推进 `last_seq`，也不触发 `/events` 补偿。
- `useAuctionSocket` 已具备 WS 连接、`seq` 检查、事件去重、断档补偿、自动重连、轮询降级和页面回前台刷新能力。

简单例子：

买家张三打开：

```text
http://127.0.0.1:5173/auctions/1
```

页面会先调用 `/api/auctions/1/status` 获取快照，并用返回的 `server_time` 校准本地时间偏移。随后页面尝试连接：

```text
ws://localhost:8080/ws/auction/1?token=mock-token-user-001&last_seq=<lastSeq>
```

如果 WS 收到 `bid_update(seq=19)`，而本地最后处理到 `last_seq=18`，页面会正常更新当前价、领先者和出价记录。如果收到 `bid_update(seq=21)`，说明中间缺了 `seq=19/20`，页面会先调用：

```text
GET /api/auctions/1/events?after_seq=18
```

如果 `/events` 返回 `snapshot_required=false`，页面按顺序应用补偿事件；如果返回 `snapshot_required=true`，页面回到 `/status` 重建房间状态。若收到的是 `viewer_count`，页面只更新在线人数，不改变补偿游标。

---

## 2. 工作背景

本阶段对应 Role B 的“场景 3：移动端直播间页面”。Role B 的职责是实时层和移动端 H5，包括 WebSocket 网关、`/status`、`/events`、移动端竞拍页、弱网重连和上传接口。

根据 `docs/team-assignment.md` 和 `docs/tasks/frontend-agent-tasks.md`，移动端拍卖详情/直播间页属于 Role B 负责范围。该页面是移动端用户参与直播竞拍的核心入口，需要把前两部分已经实现的 WS 网关和 `/events` 补偿能力接入前端状态流。

本次工作没有修改 Role A 负责的后端核心模块，包括出价规则、订单、lifecycle worker，也没有改动 `internal/bid`、`internal/order`、`internal/worker`。当前改动集中在 `mobile-h5` 前端目录。

---

## 3. 本次交付结论

本次已完成移动端直播间页面的第一版可运行实现，并通过前端 lint、TypeScript 编译和 Vite 构建验证。页面当前可以在本地通过 `/auctions/1` 访问；在真实后端接口未完全具备时，前端会使用本地 mock 快照和 mock 出价结果作为开发期 fallback。

已实现能力：

- `/auctions/:id` 页面入口。
- 375 宽移动端直播间布局。
- 顶部返回、在线人数、连接状态展示。
- 直播视频区与本地占位图。
- 当前价、倒计时、领先者浮层。
- 出价记录列表。
- 底部固定出价按钮与金额输入。
- `useServerTime` 服务端时间偏移校准。
- `useAuctionSocket` WS 连接、补偿、重连和轮询。
- `useBidButton` 出价按钮状态机。
- Zustand 拍卖房间状态管理。
- Axios API client、mock token、mock fixture 和基础 TS 类型。

---

## 4. 涉及文件

### 4.1 修改文件

- `mobile-h5/package.json`
- `mobile-h5/package-lock.json`
- `mobile-h5/src/App.tsx`
- `mobile-h5/src/App.css`
- `mobile-h5/src/index.css`

### 4.2 新增文件

- `mobile-h5/src/pages/AuctionRoomPage.tsx`
- `mobile-h5/src/hooks/useAuctionSocket.ts`
- `mobile-h5/src/hooks/useServerTime.ts`
- `mobile-h5/src/hooks/useBidButton.ts`
- `mobile-h5/src/store/auctionStore.ts`
- `mobile-h5/src/lib/api-client.ts`
- `mobile-h5/src/lib/auth.ts`
- `mobile-h5/src/lib/time.ts`
- `mobile-h5/src/lib/types.ts`
- `mobile-h5/src/mocks/auction-fixture.ts`
- `LSH_工作报告/Part3_移动端直播间页面工作报告.md`

---

## 5. 技术实现说明

### 5.1 页面入口

`App.tsx` 替换了 Vite 默认页面，新增了最小路由解析：

```tsx
function App() {
  const auctionId = parseAuctionId(window.location.pathname)
  return <AuctionRoomPage auctionId={auctionId} />
}
```

当前没有引入完整路由库，原因是本阶段只需要实现 `/auctions/:id` 核心页面。后续登录、列表、订单等页面补齐后，可以再统一接入 `react-router`。

### 5.2 AuctionRoomPage

`AuctionRoomPage.tsx` 负责页面布局和用户交互：

- 顶部：返回按钮、在线人数、连接状态按钮。
- 中部：直播视频或占位图。
- 浮层：当前价、倒计时、领先者。
- 内容区：拍卖标题、状态、结束时间、出价记录。
- 底部：金额输入、“加一手”按钮、主出价按钮。
- 弹窗：收到 `auction_ended` 后展示成交结果。

页面使用 `requestAnimationFrame` 刷新倒计时，倒计时来源为 `end_time - serverNow()`，其中 `serverNow()` 来自服务端时间偏移。

### 5.3 useServerTime

`useServerTime(auctionId)` 负责：

- 调用 `/api/auctions/:id/status`。
- 从响应中读取 `server_time`。
- 计算 `serverOffset = server_time - Date.now()`。
- 对外提供 `serverNow()`，供倒计时使用。
- 每 30 秒重新校准一次。

该实现避免直接使用本地 `Date.now()` 作为倒计时唯一来源，符合合同中“倒计时必须用 server_time 算本地偏移”的要求。

### 5.4 useAuctionSocket

`useAuctionSocket(auctionId)` 负责实时连接和弱网恢复：

- 使用原生 `WebSocket`，没有使用 socket.io。
- 连接地址使用 `VITE_WS_BASE` 或由 `VITE_API_BASE` 推导。
- WS query 中携带 `token` 和 `last_seq`。
- 打开连接后每 25 秒发送一次应用层 `ping`。
- 收到业务事件时检查 `seq`。
- 如果 `seq > last_seq + 1`，调用 `/api/auctions/:id/events?after_seq=last_seq` 补偿。
- 如果补偿返回 `snapshot_required=true`，调用 `/api/auctions/:id/status` 重建快照。
- `viewer_count` 不推进 `last_seq`，也不触发补偿。
- WS 断开后立即刷新一次 `/status`。
- 重连退避为 1s、2s、5s、10s、10s。
- 断开超过 15 秒后进入每 2 秒一次的 `/status` 轮询。
- 页面从后台回到前台时刷新 `/status`。
- 顶部连接状态按钮可手动触发 reconnect。

### 5.5 Zustand 状态层

`auctionStore.ts` 管理直播间状态：

- `auction`：当前拍卖快照。
- `bids`：出价记录。
- `viewerCount`：在线人数。
- `lastSeq`：最后处理过的业务事件序号。
- `seenEventIds`：已处理事件 ID，用于去重。
- `connectionState`：`connected / reconnecting / polling / disconnected`。
- `ended`：成交弹窗状态。

状态更新规则按事件类型拆分：

- `snapshot`：覆盖拍卖快照和出价列表。
- `bid_update`：更新当前价、领先者、出价记录和 `lastSeq`。
- `auction_extended`：更新 `end_time`。
- `auction_started`：更新状态为 active。
- `auction_ended`：更新状态为 ended 并打开成交弹窗。
- `auction_cancelled`：更新状态为 cancelled。
- `viewer_count`：只更新在线人数。

### 5.6 出价按钮状态机

`useBidButton(auctionId)` 管理出价按钮：

```text
idle -> pending -> cooldown -> idle
```

禁用条件：

- 拍卖数据未加载。
- 请求正在 pending。
- 冷却中。
- 拍卖未开始。
- 拍卖已结束。
- 拍卖已取消。
- 当前用户已经是领先者。

出价请求调用 `POST /api/auctions/:id/bid`，并携带 `Idempotency-Key`。成功时使用 HTTP 响应更新本地出价记录和当前价；失败时根据错误码展示提示，不等待 WS 判断失败。

### 5.7 API 与 mock fallback

`api-client.ts` 提供：

- `getAuctionStatus(auctionId)`
- `getEventsAfter(auctionId, afterSeq, limit)`
- `placeBid(auctionId, amount)`
- `wsBaseUrl()`

当前为了不阻塞前端开发，在真实后端接口不可用时会 fallback 到 `src/mocks/auction-fixture.ts`。这属于开发期 mock-first 策略，不代表生产行为。

### 5.8 依赖

本次新增依赖：

- `axios`：HTTP client 与请求拦截器。
- `zustand`：移动端直播间状态管理。
- `lucide-react`：按钮和状态图标。

---

## 6. 协议或数据流说明

### 6.1 页面初始化数据流

```text
用户打开 /auctions/:id
  -> App 解析 auctionId
  -> AuctionRoomPage 挂载
  -> useServerTime 调 /status 校准 server_time
  -> useAuctionSocket 调 /status 获取 snapshot
  -> Zustand applySnapshot
  -> 页面渲染拍卖状态、当前价、倒计时、出价记录
  -> useAuctionSocket 建立 WS 连接
```

### 6.2 WS 事件处理数据流

```text
收到 WS 消息
  -> 校验是否 EventEnvelope
  -> viewer_count: 只更新 viewerCount
  -> 业务事件:
       如果 seq <= lastSeq: 忽略或去重
       如果 seq == lastSeq + 1: applyEvent
       如果 seq > lastSeq + 1: 调 /events 补偿
```

### 6.3 断线恢复数据流

```text
WS close
  -> 立即调用 /status
  -> connectionState = reconnecting
  -> 按 1s / 2s / 5s / 10s / 10s 重连
  -> 15s 内未恢复则 connectionState = polling
  -> 每 2s 调一次 /status
  -> WS 恢复后停止 polling
```

### 6.4 出价数据流

```text
用户输入金额
  -> 点击出价
  -> buttonState = pending
  -> POST /api/auctions/:id/bid
  -> HTTP code=0: 用 HTTP 响应更新本地当前价和出价记录
  -> HTTP code!=0: 展示错误提示
  -> buttonState = cooldown
  -> 1s 后回 idle
```

说明：出价失败不等待 WS，符合合同 §4.1。

---

## 7. 验收记录

### 7.1 自动化测试

执行命令：

```powershell
cd D:\TRAEProj\auction-system\mobile-h5
npm.cmd run lint
```

测试结果：

```text
> mobile-h5@0.0.0 lint
> eslint .
```

执行命令：

```powershell
cd D:\TRAEProj\auction-system\mobile-h5
npm.cmd run build
```

构建结果：

```text
✓ 1806 modules transformed.
✓ built
```

### 7.2 手工验收

本地开发页验收：

```powershell
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:5173/auctions/1
```

实际结果：

```text
200
```

浏览器访问地址：

```text
http://127.0.0.1:5173/auctions/1
```

可检查内容：

- 页面不是 Vite 默认页，而是移动端拍卖直播间。
- 顶部存在返回按钮、在线人数和连接状态。
- 中部存在直播占位画面。
- 下方浮层存在当前价、倒计时和领先者。
- 底部存在金额输入和出价按钮。
- 没有真实后端接口时，页面仍能用 mock 数据渲染。

---

## 8. 当前限制

- 当前还没有完整接入登录页、拍卖列表页、订单页等移动端路由。
- 当前没有引入 Tailwind，样式先使用 `App.css` 和 `index.css` 实现。
- 当前 `/status`、`/bid`、`/events` 在不可用时会 fallback 到本地 mock，用于开发期解耦。
- 当前出价成功会根据 HTTP mock 响应更新本地状态，真实竞拍一致性仍依赖后端出价接口和 outbox 事件。
- 当前没有实现被超越、即将结束、延时、成交等氛围提醒 hook。
- 当前没有实现音效、震动和 iOS Safari audio unlock。
- 当前没有做 Playwright 视觉截图验证；本阶段以 lint、build 和 HTTP 200 作为基础验收。
- 当前没有接入 MSW service worker，而是 API client catch fallback；后续如果严格执行 D1 mock-first，可以再改为 MSW handlers。

---

## 9. 风险与评审意见

- `api-client.ts` 中的 fallback 适合开发期，但真接口可用后应逐项关闭，避免 mock 与真实接口长期并存。
- `useAuctionSocket` 当前把最多 5 页补偿作为保护上限，超过后回 `/status`；该策略保守，但需要在弱网压测中确认体验是否足够平滑。
- `viewer_count` 当前不推进 `lastSeq`，符合 Role B prompt 的最终口径；但 `contract-v2.md` 中曾出现 viewer_count seq 表述差异，后续团队应统一文档口径。
- 出价按钮当前根据 `current_leader.id === currentUser.id` 禁用“我是领先者”状态，依赖 mock token 与用户 ID 映射正确；后续真实 `/users/me` 接入后应改为从 AuthContext 获取当前用户。
- 页面当前没有完整路由体系，后续接入 `/login`、`/auctions`、`/orders` 时建议统一引入路由层。

---

## 10. 后续计划

1. 接入真实 `/api/auctions/:id/status`，替换当前快照 fallback。
2. 接入真实 `/api/auctions/:id/bid`，验证 HTTP 成功、失败码、幂等 key 和按钮状态机。
3. 接入真实 WS 业务事件流，验证 `bid_update`、`auction_extended`、`auction_ended` 对页面状态的影响。
4. 用浏览器手工模拟断网和恢复，验证重连退避、15 秒轮询降级和回前台刷新。
5. 补充 `useAuctionAlerts`，实现被超越、即将结束、延时触发、成交提醒。
6. 后续接入登录页和 AuthContext，用真实 `/api/users/me` 替换当前 mock token 用户映射。
7. 如果团队要求严格使用 Tailwind 和 MSW，补充 Tailwind 配置与 `src/mocks/handlers.ts`。

---

## 11. 本阶段评审结论

本阶段已完成 Role B 移动端直播间页面的核心前端能力，覆盖页面布局、状态管理、服务端时间校准、WS 事件处理、断线补偿、弱网重连和出价按钮状态机。代码改动集中在 `mobile-h5`，没有侵入 Role A 的核心后端模块。

当前实现已经通过 `npm.cmd run lint` 和 `npm.cmd run build`，并且本地 `/auctions/1` 可访问。它适合作为后续移动端直播竞拍联调的基础版本，但仍需在真实 `/status`、`/bid`、WS 业务事件和 AuthContext 接入后完成端到端验收。
