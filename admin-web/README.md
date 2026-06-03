# admin-web · 商家 PC 后台（Role C）

React 19 + TypeScript 6 + Vite 8 + Ant Design 6。商家发布/管理拍卖、卖家订单、PC 直播间监控、竞价氛围动画。

## 启动

```bash
npm install
npm run dev          # http://localhost:5174
```

> 默认 `VITE_USE_MSW=true`，全部接口走 MSW mock，**不依赖后端**即可跑通。
> 切真接口：把 `.env` 的 `VITE_USE_MSW` 置 `false`（或按模块逐个移除 `src/mocks/handlers.ts` 中对应 handler，见 `docs/integration-protocol.md §3/§4`）。

## 脚本

| 命令 | 作用 |
|---|---|
| `npm run dev` | 开发服务器（5174） |
| `npm run build` | `tsc -b && vite build` 类型检查 + 产物 |
| `npm run lint` | ESLint |
| `npm run gen:types` | 从 `../docs/api/openapi.yaml` 重新生成 `src/lib/openapi.d.ts` |

## 目录

```text
src/
├── lib/                 # 共享基座（与 mobile-h5 同名同结构，见 Task P6）
│   ├── openapi.d.ts     # openapi-typescript 生成，勿手改
│   ├── types.ts         # 精选公共类型别名
│   ├── api-client.ts    # axios + 拦截器 + Idempotency-Key + 错误码映射 + 401
│   ├── auth.tsx         # AuthContext / useAuth / roleFromToken
│   ├── time.ts          # dayjs + server_time 偏移 + 倒计时工具
│   ├── format.ts        # 金额（分→元）格式化
│   ├── error-codes.ts   # 合同 §1.2 错误码→中文
│   └── sound.ts         # Web Audio 音效（无外部依赖）
├── mocks/               # MSW（fixture 取自仓库根 fixtures/，与 mobile-h5 共用）
├── components/
│   ├── AppLayout / AuthGate / Countdown / ImageUploader / AuctionForm
│   └── atmosphere/      # 氛围动画：BidFlip / LeaderBadge / OvertakenFlash
│                        #            CountdownPulse / ExtendShock / WinConfetti
└── pages/               # Login / AuctionList(P3) / AuctionCreate+Edit(P2)
                         # Orders(P4) / Monitor(P5) / Demo(场景4)
```

## 路由

| 路径 | 页面 | 任务 |
|---|---|---|
| `/login` | 登录（仅卖家可进） | §2.1 / M2 |
| `/auctions` | 我的拍卖列表 | P3 |
| `/auctions/new` | 发布拍卖 | P2 |
| `/auctions/:id/edit` | 编辑（仅 pending） | P2 |
| `/orders` | 卖家订单 | P4 |
| `/monitor/:id` | 直播间监控（只读，轮询 /status） | P5 |
| `/demo` | 氛围动画演示（录视频用） | 场景 4 |

## 关键约定

- **金额单位：分（int64）**，见合同 §1。表单按「元」收集，提交 ×100 转分；展示 ÷100。
- **倒计时**用 `server_time` 偏移 + `requestAnimationFrame`，不用 `setInterval` 漂移。
- **写接口**自动带 `Idempotency-Key`、`X-Request-Id`、`X-Client-Type=admin`。
- **角色**仅前端体验分流：token 含 `-seller-` 视为卖家，PC 后台拒绝买家（鉴权以后端 DB 为准）。
- 氛围动画用 framer-motion（**不用 lottie**），尊重 `prefers-reduced-motion`，iOS 首次手势 unlock 音频。
