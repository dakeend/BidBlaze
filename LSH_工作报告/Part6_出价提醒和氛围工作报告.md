# Part 6：Role B 出价提醒和氛围工作报告

> 记录日期：2026-06-04  
> 记录人：LSH / Role B  
> 项目：直播竞拍系统 `auction-system`  
> 工作范围：移动端 H5 出价提醒、`useAuctionAlerts`、Web Audio 提示音、直播间提醒 UI

---

## 1. 完成的业务逻辑

本阶段完成的是移动端直播竞拍页面的“关键竞拍提醒和氛围反馈”能力。也就是说，用户在直播间参与竞拍时，页面不再只是静态展示当前价和倒计时，而是会根据实时事件主动提醒用户：自己被别人超过、拍卖即将结束、拍卖被延时、拍卖已经成交。

已完成业务逻辑：

- 当上一任领先者是当前用户，而新的 `bid_update` 事件显示领先者变成别人时，页面触发“被超越”提醒。
- “被超越”提醒包含震动、toast 文案和提示音。
- 当拍卖剩余时间小于等于 10 秒时，页面进入“即将结束”氛围状态。
- “即将结束”状态包含直播区红光心跳、倒计时数字心跳和滴答提示音。
- 当收到 `auction_extended` 事件时，页面展示延时提醒，例如“⏰ 延时 30s”。
- 当收到 `auction_ended` 事件时，页面展示成交提醒，并复用已有成交结果模态展示赢家和成交价。
- 所有 toast 提醒会自动消失，也可以点击关闭。
- 提示音使用 Web Audio API 实现，不引入外部音频依赖。
- iOS Safari 自动播放限制通过“用户首次出价时解锁 AudioContext”的方式处理。
- 动画尊重 `prefers-reduced-motion`，用户开启减少动态效果时会关闭提醒动画。

简单例子：

假设买家张三当前是 1 号拍卖房间的领先者：

```text
current_leader = { id: 2, nickname: "买家张三" }
```

随后 WebSocket 收到一条新的 `bid_update`：

```json
{
  "type": "bid_update",
  "event_id": "evt_1_1025",
  "auction_id": 1,
  "seq": 1025,
  "server_time": "2026-06-04T10:30:00+08:00",
  "data": {
    "current_price": 105000,
    "current_leader": { "id": 3, "nickname": "买家李四", "avatar": null }
  }
}
```

客户端在应用事件前记录到“上一任领先者是我”，应用事件后发现“当前领先者不是我”，于是触发：

```text
震动 + 提示音 + toast「⚡ 你被超越了！」
```

如果此时倒计时进入最后 10 秒，直播区会出现红色心跳光效，并播放滴答音提醒用户抓紧出价。

---

## 2. 工作背景

本阶段对应 Role B 的“场景 6：出价提醒和氛围”。该能力属于移动端 H5 体验层，目标是把 WebSocket 实时事件转化为用户可以立即感知的视觉、声音和触觉反馈。

根据 Role B Prompt 和 `docs/tasks/frontend-agent-tasks.md` 中的 M6 要求：

- 数据来源为 WS 事件：`bid_update`、`auction_extended`、`auction_ended`。
- 被超越：上一条 `current_leader` 是我，本条不是我。
- 即将结束：剩余时间小于等于 10 秒。
- 延时触发：收到 `auction_extended`。
- 成交：收到 `auction_ended`。
- 提示音使用 Web Audio API，不使用外部依赖。
- iOS Safari 需要通过用户手势解锁 audio context。

本次工作只修改移动端 H5 前端体验层，没有修改后端业务规则，也没有改动 Role A 负责的 `internal/bid`、`internal/order`、`internal/worker`。

---

## 3. 本次交付结论

本次已完成出价提醒和氛围提醒的前端实现，并接入现有移动端直播间页面。当前代码已经通过 `npm.cmd run lint` 和 `npm.cmd run build` 验证，可作为后续真实 WS 事件联调和演示录屏的基础。

已实现能力：

- 新增 `useAuctionAlerts` hook。
- 输出 `Alert[]` 给页面渲染 toast。
- 根据 `bid_update` 触发被超越提醒。
- 根据剩余时间触发即将结束提醒。
- 根据 `auction_extended` 触发延时提醒。
- 根据 `auction_ended` 触发成交提醒。
- 使用 Web Audio API 生成提示音。
- 用户首次出价时解锁 AudioContext。
- 支持移动端震动 API。
- 接入直播间 toast、红光心跳和成交弹窗动画。
- 支持 `prefers-reduced-motion`。

---

## 4. 涉及文件

### 4.1 修改文件

- `mobile-h5/src/App.css`
- `mobile-h5/src/hooks/useBidButton.ts`
- `mobile-h5/src/lib/types.ts`
- `mobile-h5/src/pages/AuctionRoomPage.tsx`
- `mobile-h5/src/store/auctionStore.ts`

### 4.2 新增文件

- `mobile-h5/src/hooks/useAuctionAlerts.ts`
- `mobile-h5/src/lib/auction-audio.ts`
- `LSH_工作报告/Part6_出价提醒和氛围工作报告.md`

---

## 5. 技术实现说明

### 5.1 事件流记录

为了让提醒 hook 判断“上一任领先者是不是我”，`auctionStore` 新增了 `lastRealtimeEvent`：

```ts
export type RealtimeEventRecord = {
  event: EventEnvelope
  previousLeaderId: number | null
  currentLeaderId: number | null
  previousPrice: number | null
  currentPrice: number | null
}
```

`applyEvent` 在处理业务事件时会记录事件应用前后的关键上下文：

- `previousLeaderId`
- `currentLeaderId`
- `previousPrice`
- `currentPrice`

这样 `useAuctionAlerts` 可以消费“已处理事件流”，而不是在页面组件里猜测状态变化。

### 5.2 useAuctionAlerts

新增 `useAuctionAlerts`：

```ts
export function useAuctionAlerts({
  latestEvent,
  remainingMs,
  endTime,
  auctionStatus,
}: UseAuctionAlertsOptions)
```

返回值：

```ts
{
  alerts,
  criticalEnding,
  dismissAlert,
  unlockAudio,
  reducedMotion,
}
```

其中：

- `alerts`：当前展示中的提醒列表。
- `criticalEnding`：是否进入最后 10 秒。
- `dismissAlert`：手动关闭 toast。
- `unlockAudio`：解锁 Web Audio。
- `reducedMotion`：是否开启减少动态效果。

### 5.3 被超越提醒

触发条件：

```text
event.type === 'bid_update'
previousLeaderId === currentUserId
currentLeaderId !== currentUserId
```

触发效果：

- `navigator.vibrate([70, 35, 70])`
- `playAlertTone('outbid')`
- toast：`⚡ 你被超越了！`

为了避免重复事件多次提醒，hook 使用 `processedEventIdsRef` 记录已经处理过的 `event_id`。

### 5.4 即将结束提醒

触发条件：

```text
auctionStatus === 'active'
remainingMs > 0
remainingMs <= 10_000
```

触发效果：

- 页面返回 `criticalEnding=true`。
- 直播区添加 `.critical-ending`。
- CSS 展示红光心跳。
- 倒计时数字应用心跳动画。
- 每秒播放一次 `tick` 提示音。
- 每个 `endTime` 只展示一次“即将结束，抓紧出价”toast。

当收到 `auction_extended` 后，会重置即将结束提醒标记，使新的结束时间可以再次触发提醒。

### 5.5 延时触发提醒

触发条件：

```text
event.type === 'auction_extended'
```

触发效果：

- 从事件 data 中读取 `extended_seconds`。
- 若缺失则按合同默认展示 30 秒。
- 播放 notice 提示音。
- 展示 toast：`⏰ 延时 30s`。

### 5.6 成交提醒

触发条件：

```text
event.type === 'auction_ended'
```

触发效果：

- 播放 ended 提示音。
- 展示 toast：`{{winner.nickname}} 成交` 或 `本场拍卖已结束`。
- 继续复用 store 中已有 `ended` 状态，展示成交结果模态。
- 成交模态增加轻量入场动画。

### 5.7 Web Audio API

新增 `auction-audio.ts`：

- 懒创建 `AudioContext`。
- 使用 oscillator 和 gain node 生成提示音。
- 不依赖外部音频文件。
- 不引入外部音频包。

音效类型：

```ts
type ToneType = 'tick' | 'notice' | 'outbid' | 'ended'
```

### 5.8 iOS Safari 音频限制处理

iOS Safari 要求音频必须由用户手势触发后才能播放。本次实现将解锁放在用户首次点击出价时：

```ts
void unlockAlertAudio()
```

位置在 `useBidButton` 的提交出价流程中。这样用户主动点击出价按钮后，`AudioContext` 会尝试 resume，后续 WS 提醒才可以播放声音。

### 5.9 UI 接入

`AuctionRoomPage` 接入：

```tsx
const { alerts, criticalEnding, dismissAlert } = useAuctionAlerts(...)
```

页面新增：

- `.alert-stack`：toast 容器。
- `.auction-alert-toast`：提醒条。
- `.live-stage.critical-ending`：即将结束红光。

toast 使用 `aria-live="polite"`，便于辅助技术感知提醒内容。

### 5.10 动画与可访问性

CSS 新增：

- toast 入场动画。
- 红光心跳动画。
- 倒计时数字缩放动画。
- 成交弹窗入场动画。

同时支持：

```css
@media (prefers-reduced-motion: reduce) {
  ...
  animation: none;
}
```

用户开启减少动态效果时，相关动画会关闭。

---

## 6. 协议或数据流说明

### 6.1 WS 事件到提醒的数据流

```text
WebSocket 收到事件
  -> useAuctionSocket 校验 EventEnvelope
  -> applyEvent(event)
  -> auctionStore 更新拍卖状态
  -> auctionStore 记录 lastRealtimeEvent
  -> AuctionRoomPage 读取 lastRealtimeEvent
  -> useAuctionAlerts 生成 Alert[]
  -> 页面渲染 toast / 红光 / 模态
```

### 6.2 被超越判断流程

```text
bid_update 到达
  -> store 记录 previousLeaderId
  -> store 应用 current_leader
  -> hook 判断 previousLeaderId === currentUserId
  -> hook 判断 currentLeaderId !== currentUserId
  -> 触发被超越提醒
```

### 6.3 即将结束判断流程

```text
AuctionRoomPage 使用 serverNow() 计算 remainingMs
  -> remainingMs <= 10_000
  -> useAuctionAlerts 返回 criticalEnding=true
  -> live-stage 添加 critical-ending
  -> CSS 红光心跳
  -> 每秒播放 tick
```

说明：倒计时仍沿用 Part3 的 `server_time` 偏移计算，不使用本地 `Date.now()` 直接作为唯一来源。

### 6.4 音频解锁流程

```text
用户点击出价按钮
  -> useBidButton.submitBid()
  -> unlockAlertAudio()
  -> AudioContext.resume()
  -> 后续提醒可播放 Web Audio tone
```

---

## 7. 验收记录

### 7.1 自动化验证

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
✓ 1809 modules transformed.
✓ built
```

### 7.2 页面可访问性检查

执行命令：

```powershell
Invoke-WebRequest -Uri http://127.0.0.1:5173/auctions/1 -UseBasicParsing -TimeoutSec 3
```

实际结果：

```text
200
```

说明：

- 前端开发服务当前可访问。
- 后端 `8080` 当前未启动时，页面会走已有 fallback/mock 数据。

### 7.3 手工验收建议

在真实 WS 事件联调时，可通过以下方式验收：

1. 登录买家张三，进入 `/auctions/1`。
2. 让张三成为当前领先者。
3. 通过另一买家出价，触发 `bid_update`，且 `current_leader.id` 不再等于张三。
4. 检查是否出现 toast：`⚡ 你被超越了！`。
5. 检查移动端是否触发震动。
6. 张三首次点击出价后，检查后续提醒音是否可以播放。
7. 将拍卖倒计时推进到最后 10 秒，检查红光心跳和滴答音。
8. 触发 `auction_extended`，检查延时 toast。
9. 触发 `auction_ended`，检查成交 toast 和成交模态。

本次尝试使用浏览器自动化插件打开页面时，插件启动阶段仍出现本地资产写入问题，未完成自动截图验收。因此本报告不声明已完成视觉截图验收。

---

## 8. 当前限制

- 当前没有新增 `/demo` 路由逐个触发四类动画，M6 文档中提到的 demo 路由仍待补充。
- 当前提醒依赖真实或 mock WS 事件进入 `applyEvent`，没有单独做事件模拟面板。
- 当前提示音是简单 oscillator 合成音，不是最终设计音效。
- 当前成交动画是轻量弹窗入场，不包含撒花或头像放大等复杂动画。
- 当前“被超越”toast 文案按用户 prompt 使用“⚡ 你被超越了！”，未实现 M6 文档中的“加 ¥{{diff}} 反超”扩展文案。
- 当前没有为 `useAuctionAlerts` 写独立单元测试，主要通过 lint、build 和页面可访问性检查验证。
- 当前后端未启动时无法完成真实 WS 事件端到端验收。

---

## 9. 风险与评审意见

- `navigator.vibrate` 在 iOS Safari 上支持有限，震动效果不能作为唯一提醒方式。
- Web Audio 在 iOS Safari 上必须依赖用户手势解锁，因此用户未点击过出价按钮前，声音提醒可能不会播放。
- 提醒逻辑依赖 `current_leader.id` 与当前用户 ID 的准确性；后续接入真实 AuthContext 后应从真实 `/users/me` 获取当前用户。
- 即将结束滴答音按每秒播放，后续应在真实设备上确认不会过于频繁或影响用户体验。
- `prefers-reduced-motion` 已关闭动画，但声音和震动仍需后续提供用户级开关。
- 如果 WS 事件重复投递，当前通过 `event_id` 去重避免重复提醒；前提是服务端事件必须稳定提供唯一 `event_id`。

---

## 10. 后续计划

1. 增加 `/demo` 或开发调试入口，手工触发 outbid、ending、extended、ended 四类提醒，便于录屏和验收。
2. 接入真实 WS 事件流后完成端到端验收，覆盖两名买家互相超越的场景。
3. 为 `useAuctionAlerts` 补充单元测试，验证事件去重、被超越判断、延时重置和即将结束触发。
4. 将当前合成提示音替换为更适合演示的音效参数，或由 Role C 提供设计素材后统一调整。
5. 增加用户设置开关，允许关闭声音、震动或氛围动画。
6. 扩展“被超越”文案，显示反超所需差价。
7. 实现成交撒花或赢家头像放大动画，用于最终演示视频。

---

## 11. 本阶段评审结论

本阶段已完成 Role B 场景 6 的核心前端提醒能力：被超越、即将结束、延时触发和成交四类提醒均已接入移动端直播间。实现遵守 WS 事件消费边界，不生成业务事件，不修改出价规则和订单逻辑。

当前代码通过 `npm.cmd run lint` 和 `npm.cmd run build`，适合作为后续真实 WebSocket 事件联调、演示录屏和体验打磨的基础版本。后续重点是补充 demo 路由、真实事件端到端验收和更完整的动效素材。
