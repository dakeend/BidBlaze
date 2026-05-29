# 时间盘点与里程碑

> 锁定日期：2026-05-29（星期五）
> 答辩日：2026-06-10（星期三）
> 总工期：**13 天**（含 2 个周末）
> 配套：`team-assignment.md`、`tasks/backend-agent-tasks.md`、`tasks/frontend-agent-tasks.md`、`integration-protocol.md`

---

## 0. 工期总览

| 阶段 | 日期 | 天数 | 主线 |
|---|---|---|---|
| **Phase 1: 底座** | 5-29 ~ 5-30 | D1–D2 | A 起骨架 + 登录 + schema |
| **Phase 2: 主干并行** | 5-31 ~ 6-03 | D3–D6 | 后端 D+G+H+I / 前端 M4+P2+P3 |
| **Phase 3: 联调** | 6-04 ~ 6-07 | D7–D10 | 联调日 + 压测 + 弱网验收 |
| **Phase 4: 收尾** | 6-08 ~ 6-09 | D11–D12 | 答辩材料 + 彩排 |
| **Phase 5: 答辩** | 6-10 | D13 | 上场 |

**风险缓冲**：D12（6-09）整天作为"机动日"，不安排新功能，只做 bug fix 和彩排。

---

## 1. D-Day 任务表

> 「★」标记当日**必须交付且可演示**的硬里程碑（不达成则次日全员投入补救）。

### D1 · 5-29 Fri — **全员开工（解耦版）**
| 人 | 任务 |
|---|---|
| A | ★ docker-compose.yml + schema 自动初始化 + Task A 基础工程骨架 push 到 main |
| A | ★ **B1: mock login 桩**（按 dev-setup.md §5 算法，直接读 users 表返 token；含 3 个种子 token） |
| A | `cmd/mockserver/` 或主进程加 `MOCK_MODE=true` 兜底未实现 endpoint 返回 openapi 样例 |
| B | ★ mobile-h5 脚手架（Vite+TS+Tailwind+Zustand）+ 安装 MSW |
| B | ★ 从 `docs/api/openapi.yaml` 生成 MSW handlers（覆盖 /login、/auctions、/status）+ useAuctionSocket hook 骨架（mock WS server） |
| C | ★ admin-web 启动验证 + `src/lib/{api-client,auth,time,types}` + openapi-typescript 生成类型 |
| C | ★ MSW handlers（与 B 共用 fixture 文件），登录页吃 MSW 跑通 |

EOD 验收（三人机器都通）：
- `docker compose up -d` → mysql/redis healthy
- `/health` + `/ready` + `POST /api/login` 桩返回种子 token
- mobile-h5 (5173) 与 admin-web (5174) 启动；MSW 拦截能看到 mock 响应
- 三个种子 token 都可通过桩接口拿到

### D2 · 5-30 Sat
| 人 | 任务 |
|---|---|
| A | ★ **B2: 真鉴权 middleware**（替换 D1 桩；含 mock token 算法落地）+ `/api/users/me` |
| A | 启动 Task C 拍卖创建/详情/列表（先 happy path） |
| B | M4 直播间页骨架（仍走 MSW）；登录页切真 `/api/login` |
| C | P1 PC 后台脚手架 + P2 拍卖发布表单（走 MSW）；登录页切真 |

EOD 验收：登录走真接口；其余功能仍 MSW；前端 axios 拦截器通。

### D3 · 5-31 Sun
| 人 | 任务 |
|---|---|
| A | ★ Task C 拍卖管理完成（含 PUT、cancel、outbox AuctionCancelled） |
| A | 启动 Task D 出价核心：Redis 锁 + 条件更新 SQL |
| B | M3 列表 + M4 详情切真 `/api/auctions`（MSW 关掉）；M8 上传接口（前后端一手）+ M5 useAuctionSocket 骨架（先连 mock 事件） |
| C | P2 拍卖发布表单切真 `POST /api/auctions`；P3 我的拍卖列表（MSW） |

EOD 验收：商家可在 PC 后台创建真拍卖；移动端能看到真列表。

### D4 · 6-01 Mon
| 人 | 任务 |
|---|---|
| A | ★ Task D 出价接口 happy path + 单测；Task G outbox publisher 启动 |
| B | ★ Task F WS 网关骨架：连接 + 房间 + snapshot + ping/pong |
| B | M5 ConnectionManager 弱网状态机 |
| C | P3 我的拍卖列表（含状态 Tab、剩余时间） |

EOD 验收：移动端能 WS 连上后端，收 snapshot；出价接口能通过 curl 完成首笔出价。

### D5 · 6-02 Tue
| 人 | 任务 |
|---|---|
| A | ★ Task D 完成：幂等、封顶自动成交、并发测试通过 |
| A | Task H Lifecycle Worker：pending→active、active→ended、建单 |
| B | ★ Task E `/status` + `/events` 补偿接口 |
| B | M4 直播间页核心：倒计时 + 出价按钮 + 出价记录 |
| C | P4 卖家订单列表 + P5 监控页骨架 |

EOD 验收：移动端能完整出价 → 看到价格涨 → WS 推送同步。

### D6 · 6-03 Wed
| 人 | 任务 |
|---|---|
| A | ★ Task I 订单接口 + 支付幂等；Task H worker 多实例测试 |
| A | event_outbox publisher 接 WS 网关，实现真广播 |
| B | M4 直播间页完成（错误码映射、ceiling_hit、auction_ended 模态） |
| B | M6 useAuctionAlerts 四种提醒（先无音效） |
| C | M3 移动端拍卖列表 + M7 我的订单页 |
| C | X1 AI log 模板落地，开始每日记录 |

EOD 验收：从「商家发布」→「买家进场」→「出价」→「延时」→「成交」→「订单」全链路跑通。

### D7 · 6-04 Thu — **联调日 1**
| 全员 |
|---|
| ★ 跑 `docs/tasks/integration-test-plan.md` Step 1–8 |
| ★ 录一次完整链路 demo（粗剪，60 秒） |
| 列出 P0/P1 bug，建 issue |
| Role A 准备 k6 压测脚本 |

EOD 验收：主流程 demo 视频可放（接受小 bug）。

### D8 · 6-05 Fri
| 人 | 任务 |
|---|---|
| A | ★ Task J 限流落地 + k6 压测：单房间 200 QPS 出价 |
| A | 多实例 worker 验证不重复建单 |
| B | M6 氛围动画完整化（音效 + iOS unlock + reduced-motion） |
| B | 修联调日发现的 WS / 弱网 bug |
| C | P2/P3 修 bug，开始写 README 架构图 |

EOD 验收：压测报告 P95 < 200ms；氛围动画在 /demo 路由可演示。

### D9 · 6-06 Sat — **合同冻结**
| 人 | 任务 |
|---|---|
| A | 压测目标全部达成；产出压测报告 markdown |
| A | 后端 README + 部署文档 |
| B | 移动端 polish：触感、加载态、空状态 |
| C | ★ X2 答辩材料启动：PPT 大纲 + 架构图 + 演示视频分镜 |

> ⚠️ 从今天起合同（contract-v2/schema/openapi）冻结，仅允许 P0 改动。

### D10 · 6-07 Sun — **联调日 2**
| 全员 |
|---|
| ★ 跑完 integration-test-plan.md 全部 Step |
| ★ 录最终演示视频（5 分钟，按 Role C prompt 分镜） |
| ★ 弱网演示（chrome devtools throttling）录制 |
| ★ AI 协作 60 秒片段录制 |

EOD 验收：终版视频粗剪完成。

### D11 · 6-08 Mon
| 人 | 任务 |
|---|---|
| A | 兜底 bug fix + 备份数据库 dump |
| B | 兜底 bug fix |
| C | ★ PPT 终稿（10 页）+ README 终稿 + AI 贡献报告 |
| C | 演示视频精剪 + 字幕 |

EOD 验收：所有交付物归档到 `docs/release/` 目录。

### D12 · 6-09 Tue — **机动日 + 彩排**
| 全员 |
|---|
| ★ 完整彩排 2 次（含 Q&A 演练） |
| 备份方案：本地视频 + 在线 demo 备份链接 |
| 检查答辩现场网络环境，准备离线 demo |

**不允许新功能 / 不允许新合同改动**。

### D13 · 6-10 Wed — 答辩
| 全员 |
|---|
| 上午 review demo |
| 上场答辩 |

---

## 2. 关键里程碑节点（汇总）

| 节点 | 日期 | 含义 |
|---|---|---|
| M0 底座可用 | D1 EOD (5-29) | docker/health/ready/桩登录/MSW 跑通；三人并行解锁 |
| M0.5 真鉴权落地 | D2 EOD (5-30) | B2 替换 D1 桩；登录走真接口；CORS 通 |
| M1 主链路打通 | D6 EOD (6-03) | 全链路 demo 可跑（含 bug） |
| M2 联调通过 | D7 EOD (6-04) | 主流程视频可放 |
| M3 性能达标 | D8 EOD (6-05) | 压测 P95<200ms、订单不重复 |
| M4 合同冻结 | D9 (6-06) | 接口不再变 |
| M5 视频终版 | D10 EOD (6-07) | 5 分钟终版 |
| M6 答辩材料齐 | D11 EOD (6-08) | PPT + README + AI 报告 |
| M7 彩排通过 | D12 EOD (6-09) | 两次彩排无致命 bug |
| **M8 答辩** | D13 (6-10) | — |

---

## 3. 风险与缓冲

### 已识别风险

| 风险 | 概率 | 影响 | 缓冲 |
|---|---|---|---|
| Task D 出价并发正确性反复 | 中 | 高 | A 全程聚焦 D，B/C 不打扰；D6 前必须收敛 |
| WS 多实例广播复杂度爆 | 中 | 中 | MVP 单实例即可演示；多实例放压测脚本兜 |
| 前端动画掉帧 | 低 | 中 | reduced-motion 兜底；演示用高配机 |
| 演示当天网络抖 | 中 | 高 | D11 录离线 demo 视频备份 |
| AI log 漏记 | 高 | 中 | C 每日 EOD 收一次，纳入 daily sync |
| 联调日发现合同 bug | 中 | 高 | D7 留缓冲；D9 后只接受 P0 |

### 时间缓冲分配

- D12 整天机动：~8 小时
- 每日工作 8 小时按 6 小时排，留 2 小时浮动：~24 小时
- 总缓冲 ~32 小时，约 4 个标准工作日

### 红线
- D6 EOD 主链路不通 → 砍 Task J 压测、砍 PC 监控页、砍移动端订单页
- D9 合同冻结后任何修改需三人会议 + 5 分钟决策
- D11 PPT 未完成 → 当晚加班，C 主笔，B/A 提供素材

---

## 4. 每日检查清单（贴墙上）

```text
□ Daily sync 10:00（≤15 分钟）
□ 当前 D-day 任务清晰
□ 是否有阻塞 > 2 小时？喊人。
□ 19:00 push 当天代码（draft PR 也行）
□ EOD 验收命令跑过
□ AI log 当天补完（Role C）
```

---

## 5. 变更日志

| 日期 | 版本 | 变更 |
|---|---|---|
| 2026-05-29 | v1.0 | 首次发布；锁定 13 天里程碑 |
| 2026-05-29 | v1.1 | 解耦 D1–D2：D1 全员开工（A 提供 mock 桩 + 种子 token；B/C 用 MSW 起步）；M0 提前到 D1 EOD，新增 M0.5；D2/D3 改为模块级切真接口 |

