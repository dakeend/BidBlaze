# Part 4：Role B 弱网与重连策略工作报告

> 记录日期：2026-06-03  
> 记录人：LSH / Role B  
> 项目：直播竞拍系统 `auction-system`  
> 工作范围：移动端 H5 弱网重连、WebSocket ConnectionManager、`/status` 短轮询降级、页面回前台同步

---

## 1. 完成的业务逻辑

本阶段完成的是移动端 H5 的“弱网连接恢复”能力。它解决的问题是：用户在直播竞拍过程中，网络可能短暂断开、WebSocket 连接可能被关闭、页面可能切到后台再恢复；客户端不能因此停留在旧状态，而应尽快刷新当前拍卖状态，并在 WebSocket 恢复后继续接收实时事件。

已完成业务逻辑：

- WebSocket 断开后，客户端会立即调用 `GET /api/auctions/:id/status` 刷新一次拍卖快照。
- 客户端会按 `1s / 2s / 5s / 10s / 10s...` 退避策略自动重连 WebSocket。
- 每次重连都会携带本地 `last_seq`，使服务端后续可以基于业务事件游标进行 snapshot 或 replay。
- 如果 WebSocket 长时间未恢复，客户端会进入 `polling` 状态，每 2 秒调用一次 `/api/auctions/:id/status` 做短轮询同步。
- 页面从后台回到前台时，会主动调用一次 `/api/auctions/:id/status`，避免页面挂起期间状态过旧。
- 连接状态被统一抽象为 `connected / reconnecting / polling / disconnected`，页面顶部可展示“已连接 / 连接中 / 同步中 / 已断开”。
- 开发环境暴露 `window.__auctionConnectionDebug`，用于手工模拟弱网断线、拒绝重连、恢复重连和强制轮询。
- WebSocket 仍使用原生 `WebSocket`，没有引入 socket.io。
- `viewer_count` 仍作为软事件处理，不推进 `last_seq`，也不触发 `/events` 补偿。

简单例子：

买家张三正在访问：

```text
http://127.0.0.1:5173/auctions/1
```

页面已经连接：

```text
ws://localhost:8080/ws/auction/1?token=mock-token-user-001&last_seq=18
```

如果网络断开，WebSocket close 后客户端会立刻调用：

```text
GET /api/auctions/1/status
```

同时页面状态进入“连接中”，并按 `1s / 2s / 5s / 10s` 尝试重连。重连时 URL 会变为：

```text
ws://localhost:8080/ws/auction/1?token=mock-token-user-001&last_seq=18
```

如果 15 秒后仍未恢复，页面进入“同步中”，每 2 秒继续请求 `/status` 保持房间状态尽量接近服务端。WebSocket 恢复后，客户端停止轮询，状态回到“已连接”。

---

## 2. 工作背景

本阶段对应 Role B 的“场景 4：弱网与重连策略”。该能力属于移动端直播竞拍体验的关键基础，因为竞拍过程对实时性要求较高，但移动端用户经常会遇到网络切换、页面挂起、浏览器节流等情况。

根据 `docs/contract-v2.md`、`docs/tasks/frontend-agent-tasks.md` 和 Role B Prompt，本阶段需要满足：

- WS 断开立即调用 `/status` 刷新一次。
- 重连退避：`1s, 2s, 5s, 10s, 10s...`。
- 重连时携带本地 `last_seq`。
- 15 秒无 WS 后降级为每 2 秒短轮询 `/status`。
- 页面从后台回到前台必须调用 `/status`。
- 使用裸 WebSocket，不使用 socket.io。

本次工作只修改移动端 H5 实时连接层，没有改动 Role A 负责的 `internal/bid`、`internal/order`、`internal/worker`。

---

## 3. 本次交付结论

本次已完成移动端弱网重连策略的前端实现，并把原来分散在 `useAuctionSocket` 中的连接生命周期逻辑抽象成 `ConnectionManager`。当前实现已经通过前端 lint 和生产构建验证，可作为移动端直播间弱网联调的基础版本。

已实现能力：

- 独立 `ConnectionManager` 类。
- `connected / reconnecting / polling / disconnected` 状态机。
- WS 断开后立即刷新 `/status`。
- 退避重连策略。
- 15 秒后进入短轮询降级。
- 每 2 秒短轮询 `/status`。
- 页面回前台刷新 `/status`。
- 重连携带最新 `last_seq`。
- 开发环境弱网模拟入口。
- 与 `useAuctionSocket` 中的事件补偿逻辑保持兼容。

---

## 4. 涉及文件

### 4.1 修改文件

- `mobile-h5/src/hooks/useAuctionSocket.ts`

### 4.2 新增文件

- `mobile-h5/src/lib/connection-manager.ts`
- `LSH_工作报告/Part4_弱网与重连策略工作报告.md`

### 4.3 关联文件

- `mobile-h5/src/pages/AuctionRoomPage.tsx`
- `mobile-h5/src/lib/types.ts`
- `mobile-h5/src/store/auctionStore.ts`

说明：关联文件主要负责显示连接状态和维护 `ConnectionState`，本阶段核心改动集中在连接管理层。

---

## 5. 技术实现说明

### 5.1 ConnectionManager

新增 `ConnectionManager`，负责 WebSocket 连接生命周期。它接收以下能力作为参数：

- `buildUrl()`：构造 WS URL，包含 `token` 和 `last_seq`。
- `refreshStatus()`：调用 `/api/auctions/:id/status` 刷新快照。
- `onMessage()`：把 WS 消息交给上层 hook 解析和应用。
- `onStateChange()`：通知页面连接状态变化。
- `onError()`：记录连接错误。

它内部维护：

- `socket`：当前 WebSocket 实例。
- `reconnectTimer`：重连定时器。
- `pollingTimer`：短轮询定时器。
- `pingTimer`：应用层 ping 定时器。
- `reconnectAttempt`：当前重连次数。
- `disconnectedAt`：断线开始时间。
- `state`：当前连接状态。

### 5.2 状态机

连接状态定义为：

```ts
export type ConnectionState = 'connected' | 'reconnecting' | 'polling' | 'disconnected'
```

状态转换：

```text
start()
  -> reconnecting
  -> connected

WS close
  -> refresh /status
  -> reconnecting
  -> connected

WS close 超过 15s
  -> polling
  -> connected

stop()
  -> disconnected
```

说明：

- `reconnecting` 表示正在尝试恢复 WebSocket。
- `polling` 表示 WebSocket 长时间不可用，客户端通过 `/status` 做短轮询同步。
- `connected` 表示 WebSocket 已恢复。
- `disconnected` 用于组件卸载或连接管理停止。

### 5.3 退避重连

默认退避配置：

```ts
const defaultReconnectBackoffMs = [1000, 2000, 5000, 10000]
```

实现逻辑：

- 第 1 次重连等待 1 秒。
- 第 2 次重连等待 2 秒。
- 第 3 次重连等待 5 秒。
- 第 4 次及以后等待 10 秒。

### 5.4 轮询降级

默认降级配置：

```ts
const defaultPollingAfterMs = 15_000
const defaultPollingIntervalMs = 2000
```

实现逻辑：

- WebSocket 断开后立即刷新一次 `/status`。
- 如果断线时长未达到 15 秒，维持 `reconnecting` 状态。
- 如果断线时长达到 15 秒，进入 `polling`。
- `polling` 状态下每 2 秒调用一次 `refreshStatus()`。
- WebSocket 成功打开后停止轮询。

### 5.5 页面回前台刷新

`ConnectionManager` 注册：

```ts
document.addEventListener('visibilitychange', this.handleVisibilityChange)
```

`handleVisibilityChange` 是箭头函数，内部 `this` 绑定的是 `ConnectionManager` 实例，不会丢失上下文。页面从后台回到前台时：

```ts
if (document.visibilityState === 'visible') {
  void this.refreshStatus()
}
```

### 5.6 与 useAuctionSocket 集成

`useAuctionSocket` 中创建 `ConnectionManager`：

```ts
const manager = new ConnectionManager({
  buildUrl: () => {
    const lastSeq = useAuctionStore.getState().lastSeq
    return `${wsBaseUrl()}/ws/auction/${auctionId}?token=${encodeURIComponent(
      getAuthToken(),
    )}&last_seq=${lastSeq}`
  },
  refreshStatus: async () => {
    await refreshSnapshot()
  },
  onMessage: (payload) => {
    if (isEventEnvelope(payload)) {
      void handleServerEvent(payload)
    }
  },
  onStateChange: setConnectionState,
  onError: setLastError,
})
```

这样连接层不直接理解业务事件，只负责连接、重连、轮询和状态通知；业务事件的 `seq` 校验、`/events` 补偿、`viewer_count` 处理仍留在 `useAuctionSocket` 中。

### 5.7 开发调试入口

开发环境下暴露：

```ts
window.__auctionConnectionDebug = {
  closeSocket,
  rejectReconnects,
  allowReconnects,
  forcePolling,
  reconnect,
  state,
}
```

用途：

- `closeSocket()`：模拟 WS 被动断开。
- `rejectReconnects()`：模拟一直无法重连，进入 polling。
- `allowReconnects()`：允许恢复重连。
- `forcePolling()`：直接观察轮询 UI。
- `state()`：读取当前连接状态。

---

## 6. 协议或数据流说明

### 6.1 正常连接数据流

```text
AuctionRoomPage 挂载
  -> useAuctionSocket 创建 ConnectionManager
  -> ConnectionManager.start()
  -> refreshStatus()
  -> connect()
  -> WebSocket open
  -> state = connected
  -> startPing()
```

### 6.2 断线恢复数据流

```text
WebSocket close
  -> stopPing()
  -> refreshStatus()
  -> disconnectedAt = now
  -> state = reconnecting
  -> scheduleReconnect()
  -> connect() with last_seq
```

### 6.3 轮询降级数据流

```text
WebSocket close 超过 15 秒
  -> state = polling
  -> refreshStatus()
  -> setInterval(refreshStatus, 2000)
  -> WebSocket open
  -> stopPolling()
  -> state = connected
```

### 6.4 业务事件与补偿关系

弱网重连只负责连接恢复，不直接生成业务状态。业务事件仍按以下规则处理：

- `viewer_count`：只更新在线人数，不推进 `last_seq`。
- `snapshot`：覆盖当前快照。
- 业务事件：如果 `seq > last_seq + 1`，先调用 `/events` 补偿。
- `/events` 返回 `snapshot_required=true` 时，调用 `/status` 重建状态。

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
✓ 1807 modules transformed.
✓ built
```

### 7.2 手工验收

本地页面可访问性检查：

```powershell
Invoke-WebRequest -Uri http://127.0.0.1:5173/auctions/1 -UseBasicParsing
```

实际结果：

```text
StatusCode: 200
```

浏览器控制台弱网模拟：

```js
window.__auctionConnectionDebug.closeSocket()
window.__auctionConnectionDebug.state()
```

预期结果：

```text
reconnecting
```

拒绝重连并观察轮询：

```js
window.__auctionConnectionDebug.rejectReconnects()
window.__auctionConnectionDebug.state()
```

预期结果：

```text
polling
```

恢复重连：

```js
window.__auctionConnectionDebug.allowReconnects()
```

预期结果：

- 后端 WS 可用时，状态恢复为 `connected`。
- 页面顶部状态文案从“同步中”或“连接中”恢复为“已连接”。

说明：本阶段尝试使用本地浏览器自动化插件进行页面检查时，浏览器插件启动阶段出现本地资产写入问题，未完成自动截图。因此本报告不声称已完成视觉截图验收，验收依据为 lint、build、HTTP 200 和控制台弱网测试入口。

---

## 8. 当前限制

- 当前弱网策略已经在前端实现，但仍需要配合真实后端 WS 服务做完整端到端验收。
- 当前 `/status` 在真实接口不可用时仍可能走前端 mock fallback，因此真实状态一致性需要在后端接口稳定后复验。
- 当前短轮询只刷新 `/status` 快照，不会单独拉取 `/events`；事件断档补偿仍由收到业务事件时的 `seq` 检查触发。
- 当前开发调试入口只在 `import.meta.env.DEV` 下暴露，不作为正式用户功能。
- 当前没有写独立的 ConnectionManager 单元测试，主要通过 TypeScript 编译、lint 和页面手工入口验证。

---

## 9. 风险与评审意见

- `refreshStatus()` 请求失败时会记录错误并继续运行，不会抛出未捕获异常；但如果后端长时间不可用，页面会持续处于 `polling` 或 `reconnecting`，需要 UI 明确提示用户。
- `visibilitychange` 监听使用的是同一个箭头函数引用，因此不会丢失 `this`，也可以被 `removeEventListener` 正确移除。
- 轮询频率当前为每 2 秒一次，符合 Prompt 要求；后续压测时需要确认服务端 `/status` 限流策略。
- 应避免把 `window.__auctionConnectionDebug` 当作生产能力，它只用于本地验收和弱网模拟。
- WebSocket token 仍通过 query 传递，符合 MVP 约定，但代码和报告中应持续标记生产阶段需要替换为更安全的认证方式。

---

## 10. 后续计划

1. 在真实后端 `8080` WS 服务启动后，用浏览器 DevTools Offline / Slow 3G 模式复验断线、重连和轮询状态。
2. 补充 ConnectionManager 单元测试，覆盖退避序列、15 秒降级、stop 清理、visibilitychange 刷新。
3. 与后端 `/status` 限流策略联调，确认 2 秒短轮询不会触发错误限流。
4. 在真实 outbox 事件流中验证重连携带 `last_seq` 后，服务端返回 snapshot 或缺失事件的行为。
5. 将弱网验收过程录屏或截图，作为后续答辩材料。

---

## 11. 本阶段评审结论

本阶段已完成 Role B 移动端弱网重连策略的核心实现，满足场景四提出的断线刷新、退避重连、携带 `last_seq`、15 秒降级轮询、页面回前台刷新和状态指示要求。实现边界集中在 `mobile-h5` 前端实时连接层，没有改动 Role A 的出价、订单和 worker 模块。

当前代码已经通过 `npm.cmd run lint` 和 `npm.cmd run build`，可进入真实后端环境下的弱网端到端联调。
