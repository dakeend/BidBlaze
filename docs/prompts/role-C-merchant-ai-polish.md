# 角色 C · 商家 PC 后台 / 氛围打磨 / AI 工程化 — Prompt 模板

> 适用范围：商家 PC 后台、登录、移动端列表/详情静态层、氛围动画、AI 使用归档、答辩材料。
> 使用方式：复制下面对应场景的模板，替换 `{{...}}` 占位符，喂给 AI 编码助手。

---

## 必读上下文（每次新会话开头喂给 agent）

```
请先读以下文件再开始：
1. docs/team-assignment.md  §3.2, §3.3, §5      # 我的边界（不碰后端、不碰 mobile WS 层）
2. docs/contract-v2.md  §1.2 (错误码), §2.1 (登录), §2.2 (拍卖 CRUD), §2.5 (订单)
3. docs/api/openapi.yaml                         # TS 类型 + MSW handlers 源
4. docs/tasks/frontend-agent-tasks.md（P1–P5 + M2/M3/M7）
5. docs/dev-setup.md  §2, §4, §5                 # 端口 5174、env、mock token 算法
6. docs/integration-protocol.md  §3, §4          # mock-first 框架 + 切真接口 checklist
```

## D1 起步任务（解耦后不等 A）

```
任务：D1 EOD 前用 MSW 把 admin-web 跑起来 + 共享 lib 雏形落地。
1. admin-web 启动验证（5174）；AntD 6 主题确认
2. src/lib（与 mobile-h5 同名同结构）：
   - api-client.ts：axios + 拦截器 + 错误码映射 + Idempotency-Key 生成
   - auth.ts：token 存取、AuthContext、useAuth
   - time.ts：dayjs + server_time 偏移
   - error-codes.ts：合同 §1.2 中文映射
   - types.ts：用 openapi-typescript 从 docs/api/openapi.yaml 生成
3. MSW handlers（src/mocks/handlers.ts）：
   - POST /api/login 返种子 seller token（mock-token-seller-001）
   - POST /api/auctions、GET /api/auctions?seller_id=me、PUT /api/auctions/:id
   - POST /api/uploads 返固定 fixture url
   - fixture 文件建议放仓库根 fixtures/，与 mobile-h5 共用
4. 登录页吃 MSW 跑通

不要在 D1 做：氛围动画、答辩材料、AI log 模板（D6 起）。
切真接口时机：按 integration-protocol §3 表，与 B 互不打扰各自切。
```

---

## 通用前置上下文（每次会话开头先贴一次）

```
你正在帮我开发「直播竞拍系统」的商家后台与体验层。技术栈：

- 前端：现有脚手架为 React 19 + TypeScript 6 + Vite 8
- 商家后台 PC（最小宽度 1280）
- UI 库：Ant Design 5（PC 后台）+ Tailwind（移动端）
- 接口契约以 docs/contract-v2.md 为唯一来源

我负责的模块：
- 商家 PC 后台（拍卖发布/管理/订单）
- 移动端拍卖列表 + 详情静态层（WS 部分 B 接）
- 登录 mock（§2.1）
- 氛围动画（领先/被超越/倒计时/成交）
- AI 使用记录归档（评分 15% 的关键证据）
- 演示视频和项目文档

回答约束：
1. 表单全部用 Ant Design Form + 受控校验
2. 错误码映射严格按合同 §1.2
3. 时间显示用 dayjs，时区 Asia/Shanghai
4. 不要写后端代码（除非是 mock server）
```

---

## 场景 1：商家后台 — 拍卖发布表单

```
任务：实现「创建拍卖」表单 POST /api/auctions

字段（合同 §2.2）：
- title (1-128) / description (0-2000) / cover_url / images (≤9)
- stream_url (可空) / start_price (≥0) / price_step (>0) / ceiling_price (可空)
- start_time (晚于现在) / duration_seconds (30-86400)
- extend_seconds (10-30, 默认 30) / extend_threshold (1-300, 默认 30)

UI 要求：
- 分三栏：基础信息 / 价格规则 / 时间规则
- 图片上传走 /api/uploads（B 提供）
- 「ceiling_price 留空 = 无封顶」要有明显提示
- 「0 元起拍」是亮点，要在 start_price=0 时高亮提示「将以 price_step 作为首笔最低有效价」
- 提交前预览弹窗

请输出：
1. AntD Form 完整代码
2. 校验 schema（可以用 zod 或 AntD rules）
3. 错误码 2002/2103 等的中文提示映射

不要：把这些字段散到 5 个组件里，一个文件搞定。
```

---

## 场景 2：商家后台 — 拍卖列表与管理

```
任务：实现「我的拍卖」列表页

合同要求：
- GET /api/auctions?seller_id=me&status=&page=&size=
- 状态 tab：全部 / pending / active / ended / cancelled
- pending 行可点「修改」（跳到编辑页 PUT /api/auctions/:id）
- pending / active 行可点「取消」（POST /:id/cancel，二次确认）
- active 行可点「进直播间」（跳移动端预览或 PC 监控页）

请输出：
1. AntD Table + 状态 Tabs
2. 列定义：封面 / 标题 / 当前价 / 状态 / 剩余时间 / 出价数 / 操作
3. active 行的剩余时间需要每秒刷新（用 server_time 偏移）
4. 取消的 confirm 文案：明确说明会向所有买家广播

注意：active 拍卖不允许修改，按钮置灰并 tooltip 说明原因。
```

---

## 场景 3：登录 mock

```
任务：实现 §2.1 的 mock 登录

要求：
- POST /api/login {nickname, avatar}
- 同 nickname 返回旧 token（首次注册）
- 前端：登录页 + token 存 localStorage + Axios 拦截器加 Authorization
- 401 全局跳登录
- 区分「买家」和「卖家」角色（PC 后台需要卖家身份，移动端任意）

请输出：
1. 登录页（极简，只有 nickname 输入）
2. AuthContext + useAuth hook
3. Axios 实例配置：自动加 token、自动加 X-Request-Id、自动加 X-Client-Type
4. 401 拦截

不需要：注册页、密码、找回密码、二维码登录。
```

---

## 场景 4：氛围动画（亮点项）

```
任务：把出价氛围做到极致（对应评分加分项「竞价氛围体验」）

需要的动画：
1. 出价成功：按钮 → 价格数字 翻牌动画（react-spring 或 framer-motion）
2. 我领先时：「👑 你正在领先」常驻徽章，金色光晕呼吸
3. 被超越：屏幕闪红 + 震动 + 「⚡ 被超越了，加价 ¥{{diff}} 反超」CTA
4. 倒计时 ≤10s：数字变红 + 滴答音 + 心跳缩放
5. 延时触发：「⏰ 延时 30 秒！」从顶部弹入，背景一道冲击波
6. 成交：撒花 + 赢家头像放大 + 成交价高亮

请输出：
1. 每个动画的实现方案（CSS / framer-motion / Web Audio 选哪个）
2. 性能预算：60fps，单帧 <16ms，避免触发 layout
3. 移动端 iOS Safari 兼容点（audio unlock、prefers-reduced-motion）
4. 一个 Storybook 或单独的 /demo 路由方便录视频

资源约束：不能引入 lottie（太重），不能用付费动画库。
```

---

## 场景 5：AI 使用归档（评分 15%）

```
任务：建立 AI 使用记录归档，对应评分「AI 使用与落地效果」

需要的产物：
1. docs/ai-log/ 目录，按日期组织
2. 每次 AI 编码会话记录：日期 / 角色 / 任务 / prompt 摘要 / 生成代码量 / 人工修改量 / 决策点
3. 一份 ai-contribution-report.md 汇总：
   - AI 代码贡献率（按行数估算，按模块拆分）
   - 高质量产出的 prompt 模板沉淀（即本目录文件）
   - 不适合 AI 的场景（如复杂事务一致性，必须人工把控）
4. 演示视频里 60 秒展示 AI 协作流程

请帮我：
1. 设计 ai-log/ 的目录结构和单条记录模板
2. 写一个 git pre-commit 钩子或 CI 脚本，统计代码行数变化
3. 输出 ai-contribution-report.md 的章节框架

注意：评委关心的不是「AI 写了多少」，而是「关键决策点的人工把控」。模板要突出这一点。
```

---

## 场景 6：答辩材料

```
任务：6.10 前产出答辩材料

清单：
1. README.md：架构图 + 启动步骤 + 技术亮点
2. 演示视频（5 分钟）：
   - 00:00-00:30 项目背景
   - 00:30-02:00 商家发布 + 移动端竞拍全流程
   - 02:00-03:30 高并发演示（k6 压测 + 实时排名稳定）
   - 03:30-04:30 弱网重连演示 + 断线补偿
   - 04:30-05:00 AI 协作流程展示
3. 架构图（Excalidraw 或 draw.io）：
   - 系统架构（API/WS/Domain/Worker/MySQL/Redis）
   - 出价时序图（含 Redis 锁 + MySQL 条件更新 + outbox）
   - WS 断线补偿时序图
4. 技术亮点 PPT（10 页）：每页一个亮点，对应评分四个维度

请帮我：
1. README.md 完整 outline 和首页架构图描述
2. 演示视频分镜脚本
3. PPT 每页的标题 + 一句话要点

不要：复杂的甘特图、团队组织架构图、不必要的封面装饰。
```

---

## 反模式（请 AI 避免）

- 商家后台用 Tailwind 自己撸（直接上 AntD）
- 把氛围动画做成 lottie 文件（太重，且不可定制）
- 倒计时用 setInterval(fn, 1000)（漂移；用 requestAnimationFrame + server_time 偏移）
- AI 日志只记录「prompt 是什么」，不记录「我改了什么、为什么改」（评分要的是后者）
- 演示视频开头放 1 分钟背景介绍（评委时间宝贵，前 30 秒必须见东西）

