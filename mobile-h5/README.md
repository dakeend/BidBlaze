# React + TypeScript + Vite

This template provides a minimal setup to get React working in Vite with HMR and some ESLint rules.

Currently, two official plugins are available:

- [@vitejs/plugin-react](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react) uses [Oxc](https://oxc.rs)
- [@vitejs/plugin-react-swc](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react-swc) uses [SWC](https://swc.rs/)

## React Compiler

The React Compiler is not enabled on this template because of its impact on dev & build performances. To add it, see [this documentation](https://react.dev/learn/react-compiler/installation).

## Expanding the ESLint configuration

If you are developing a production application, we recommend updating the configuration to enable type-aware lint rules:

```js
export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      // Other configs...

      // Remove tseslint.configs.recommended and replace with this
      tseslint.configs.recommendedTypeChecked,
      // Alternatively, use this for stricter rules
      tseslint.configs.strictTypeChecked,
      // Optionally, add this for stylistic rules
      tseslint.configs.stylisticTypeChecked,

      // Other configs...
    ],
    languageOptions: {
      parserOptions: {
        project: ['./tsconfig.node.json', './tsconfig.app.json'],
        tsconfigRootDir: import.meta.dirname,
      },
      // other options...
    },
  },
])
```

You can also install [eslint-plugin-react-x](https://github.com/Rel1cx/eslint-react/tree/main/packages/plugins/eslint-plugin-react-x) and [eslint-plugin-react-dom](https://github.com/Rel1cx/eslint-react/tree/main/packages/plugins/eslint-plugin-react-dom) for React-specific lint rules:

```js
// eslint.config.js
import reactX from 'eslint-plugin-react-x'
import reactDom from 'eslint-plugin-react-dom'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      // Other configs...
      // Enable lint rules for React
      reactX.configs['recommended-typescript'],
      // Enable lint rules for React DOM
      reactDom.configs.recommended,
    ],
    languageOptions: {
      parserOptions: {
        project: ['./tsconfig.node.json', './tsconfig.app.json'],
        tsconfigRootDir: import.meta.dirname,
      },
      // other options...
    },
  },
])
```



你作为 **Role B**，本质上负责这条线：

**“移动端用户进直播间，看实时价格变化，能稳定出价，断线也能恢复。”**

不用先啃文档。你先按下面这个整体流程走，到具体接口/字段时再去查对应文档。

**你大概要干 4 件事**

1. **移动端 H5 基础**
   你负责 `mobile-h5/` 的主要体验，尤其是直播竞拍页。前期先用 MSW/mock 数据跑起来，不等后端全完成。

2. **实时 WebSocket 网关**
   你负责后端的实时层：用户连进某个拍卖房间，服务端发 `snapshot`，后续广播出价、延时、结束、取消、在线人数等事件。

3. **状态与补偿接口**
   你负责 `/status` 和 `/events` 这类接口。它们是弱网兜底：WebSocket 断了、漏事件了，前端靠它们恢复状态。

4. **上传接口**
   你还负责 `/api/uploads`，给商家端/移动端上传商品图片用。这个不算核心难点，但要尽早打通，因为 Role C 会用。

**整体开发流程**

第一阶段：先让移动端能假跑起来  
目标是 `mobile-h5` 能启动，有登录、拍卖列表、直播间雏形。接口先走 mock，不用等 Role A。你要准备好：

- 页面路由：登录、拍卖列表、拍卖详情/直播间、订单页
- mock 数据：用户、拍卖、事件流
- `useAuctionSocket` 的空壳
- 出价按钮和倒计时的 UI 骨架

第二阶段：做实时网关  
这是你最核心的后端任务。你要实现：

- `ws://localhost:8080/ws/auction/:id?token=xxx`
- 用户进房间后立即收到 `snapshot`
- 客户端发 `ping`，服务端回 `pong`
- 按 auctionId 管理房间连接
- 广播事件给房间内所有用户
- `viewer_count` 在线人数事件

注意：WebSocket 只负责连接和广播，不处理真正的出价写库。出价核心是 Role A 的。

第三阶段：做状态恢复能力  
移动端不能只依赖 WebSocket。你要实现：

- `GET /api/auctions/:id/status`：返回当前拍卖快照
- `GET /api/auctions/:id/events?after_seq=xxx`：返回断线期间漏掉的事件
- 前端按 `seq` 去重
- 如果发现事件断档，就调 `/events`
- 如果补偿不了，就调 `/status` 重建状态

这部分是“弱网能恢复”的关键。

第四阶段：把直播间做完整  
等 Role A 的出价接口可用后，你把移动端直播间接真接口：

- 倒计时用服务端时间校准
- 出价按钮调用 `POST /api/auctions/:id/bid`
- 出价成功后等 HTTP 响应更新，也监听 WS 的 `bid_update`
- 处理低价、封顶、拍卖结束、延时等状态
- 断网时进入 polling，恢复后重新连 WS

第五阶段：联调和演示  
最后你主要负责证明这条链路稳定：

- 手机端进直播间能收到 snapshot
- 出价后价格实时变化
- 多个端同时看，价格同步
- 断网后重连能恢复
- 弱网下不会乱跳价格
- 录制弱网恢复和实时出价演示

**你当前最该先做什么**

今天是 2026-06-01，对应文档里的 D4。你现在最应该优先做：

1. 检查 `mobile-h5` 是否能启动。
2. 做或补齐 `useAuctionSocket` 骨架。
3. 做后端 WS 网关骨架：连接、房间、snapshot、ping/pong。
4. 做前端 ConnectionManager：`connected -> reconnecting -> polling -> connected`。
5. 先别纠结动画、音效、订单页，那些后面再补。

**到时候具体查哪些文档**

你不用现在看完，只记住查表位置：

- WS 事件格式：`docs/events/event-contract.md`
- `/status`、`/events`、`/uploads` 字段：`docs/contract-v2.md`
- 你的后端任务：`docs/tasks/backend-agent-tasks.md` 里的 Task E/F/K
- 你的前端任务：`docs/tasks/frontend-agent-tasks.md` 里的 M4/M5/M6/M8
- 联调怎么切真接口：`docs/integration-protocol.md`

一句话：你先把 **移动端直播间 + WebSocket 实时同步 + 断线恢复** 这条链路打通，其他都围着这条主线排优先级。