# 联调协议

> 锁定日期：2026-05-29
> 适用：Role A / B / C 三人并行开发期间的协作规则。
> 与 `team-assignment.md`、`milestones.md` 配套使用。

---

## 1. Git 分支策略

```text
main             # 受保护；只接受 PR；任何时刻 main 都能跑通
├── feat/A-task-D-bid           # Role A 的 Task D
├── feat/B-task-F-ws-gateway    # Role B 的 Task F
├── feat/C-task-P2-publish-form # Role C 的 Task P2
├── fix/...
└── chore/...
```

### 规则
- **不允许直接 push main**。
- 分支命名：`<type>/<role>-<scope>`，type ∈ `feat|fix|chore|docs`。
- 单 PR 不超过 800 行 diff（生成代码除外）。
- PR 标题：`[Role A] Task D: 出价核心接口`。
- PR 描述模板：
  ```markdown
  ## 关联
  - Task: D
  - 合同章节: §2.3, §7.1, §7.2

  ## 改动
  - ...

  ## 验收
  ```powershell
  # 复现命令
  ```

  ## Review checklist
  - [ ] 接口字段与 contract-v2.md 一致
  - [ ] 错误码取自合同 §1.2
  - [ ] 有单元测试或 curl 示例
  - [ ] 不修改其他人模块
  ```

### Review
- 后端 PR：Role B 或 Role C 至少一人 review；Task D/G/H 强制 **A+B 双 review**。
- 前端 PR：另一前端负责人 review。
- 合同/schema/openapi 改动 PR：**三人都必须 review**。

### Merge
- 一律 squash merge，commit message 用 PR 标题。
- 周末/晚 22 点后禁止 merge 到 main（除非线下确认）。

---

## 2. 合同变更流程

任何对 `docs/contract-v2.md`、`docs/schema-v2.sql`、`docs/api/openapi.yaml`、`docs/events/event-contract.md` 的改动：

1. 开 issue：「[Contract Change] 描述」。
2. 三人群里 @所有人，等 ≥2 人同意。
3. PR 必须同时更新：
   - 文档本体
   - `contract-v2.md` §11 变更日志（新增一行 + 版本号 +0.0.1）
   - 受影响的代码（后端/前端任一方先改也行，但不能合并到 main 直到对端跟上 mock）
4. 通过后在群里 @所有人广播。

合同冻结期：D9 (6.7) 起冻结，仅允许 critical fix。

---

## 3. Mock 优先策略

为避免 B/C 被 A 卡住，所有跨人接口必须支持 **mock-first**：

| 接口 | 提供者 | 消费者 | Mock 时间窗口 |
|---|---|---|---|
| `/api/login`, `/api/users/me` | A | B, C | D2 真接口；D1 用 hardcoded fixture |
| `/api/auctions/*` | A | B, C | D3 真接口 |
| `/api/auctions/:id/bid` | A | B | D4 真接口 |
| `/ws/auction/:id` 事件流 | B | A 触发, C 监控 | D5 起真广播 |
| `/api/uploads` | B | C | D4 真接口 |

### Mock 方式
- **前端用 MSW** (`msw`)：拦截 Axios，返回符合 `openapi.yaml` 的样例。
- **后端开发期开 `MOCK_MODE=true`**：未实现的 endpoint 返回硬编码 200。

### 切换原则
- 真接口可用后，对应 mock 当天移除。
- mock 与真接口同时存在 ≤24 小时。

---

## 4. 接口联调 checklist

任何"前端调后端真接口"第一次通过前，双方对照执行：

```text
[ ] 后端 endpoint 已合并到 main
[ ] 后端启动后 curl 验证 happy path 返回 code=0
[ ] 后端 curl 验证至少 1 个失败码（401/2101/...）
[ ] 前端切到真接口（去掉 MSW handler）
[ ] 前端发 1 次请求，浏览器 Network 看到 200 + code=0
[ ] 前端发 1 次错误请求（例如缺 token），看到正确错误码映射
[ ] 双方在 PR/群里互相 @ 标记联调通过
```

---

## 5. 每日协作节奏

### Daily Sync（≤15 分钟，每天 10:00）
- 昨日完成的 PR 链接
- 今日目标（对照 milestones.md 的 D-day 任务）
- 阻塞（是否在等别人）

### 阻塞规则
- 任何阻塞超过 2 小时，必须在群里 @ 对应负责人。
- 对端无法立即响应时，**切换到 mock 推进**，回头再联调。

### EOD 推送
- 每日 19:00 前所有人 push 当天工作（即使未完成，开 draft PR）。
- 方便他人 review，也避免本地丢失。

### 联调日（D7、D10）
- 全员上线 ≥4 小时，专门跑 `integration-test-plan.md`。
- 发现 bug 当场建 issue，标 `integration` label。

---

## 6. CI / 质量门禁

`.github/workflows/backend.yml`（Role A 提交）：
- 触发：PR 到 main、push 到任意 feat/* 分支
- 步骤：`go vet` → `go test ./...` → `golangci-lint`
- MySQL/Redis 用 service container

`.github/workflows/frontend.yml`：
- 触发：同上
- 步骤：
  - `cd mobile-h5 && npm.cmd ci && npm.cmd run build`
  - `cd admin-web && npm.cmd ci && npm.cmd run build`

### 必须绿才能 merge
- main 分支保护规则：require status checks pass。
- 例外：docs-only PR 可豁免。

---

## 7. 错误码与日志对齐

### 错误码
- 前端错误码映射文件必须覆盖合同 §1.2 全部码；当前可分别放在 `mobile-h5/src/lib/error-codes.ts` 与 `admin-web/src/lib/error-codes.ts`。
- 新增错误码必须先改合同，再前后端同时实现。

### 日志字段
后端 `slog` 强制字段：
```json
{
  "request_id": "...",
  "user_id": 2,
  "auction_id": 1,
  "idempotency_key": "bid-...",
  "amount": 95000,
  "result_code": 0,
  "latency_ms": 12,
  "auction_version": 18
}
```
前端 console 不强制，但出价/支付/WS 重连必须打 info 日志。

---

## 8. 测试数据约定

- 三个种子用户在 schema-v2.sql 已存在（主播阿明 / 买家张三 / 买家李四）。
- 联调用拍卖 ID：约定 1–10 为「联调专用」，**不要清库**，需要清时群里通知。
- 演示视频用拍卖 ID 从 100 开始。
- 测试图片：`server-go/testdata/product.jpg`（Role A 准备，<200KB）。

---

## 9. 沟通渠道

| 用途 | 渠道 |
|---|---|
| 同步讨论 | 微信群（项目专用） |
| 异步问题 | GitHub Issues |
| Code review | GitHub PR |
| 阻塞告警 | 群里 @ + Issue 标 `blocker` |
| 文档变更广播 | 群里 + commit message 带 `[contract]` |

---

## 10. 反模式

- 直接在 main 上 commit。
- 改了合同不写 changelog。
- 改了别人模块「顺手优化」。
- 真接口好了不通知对端撤 mock。
- PR 描述只写「fix bug」。
- 联调失败后自己 debug 半天不喊人。
- 复制 fixture 到代码里 hardcode（应该走 mock）。

