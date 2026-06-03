# 技术亮点 PPT 大纲（10 页）

> 每页一个亮点，对应评分四个维度：架构合理性 / 高并发正确性 / 实时与弱网体验 / AI 落地。
> 一页一句话要点，现场口头展开。

| 页 | 标题 | 一句话要点 |
|---|---|---|
| 1 | 直播竞拍系统 · 一句话价值 | 0 元起拍、实时竞价、延时反超的端到端竞拍系统（先放一张竞拍画面，不放团队介绍） |
| 2 | 系统架构总览 | API / WS / Domain / Worker / MySQL / Redis 单体分层，见 `architecture.md` 架构图 |
| 3 | 出价正确性（核心） | Redis 锁 + MySQL 条件更新（`current_price + step <= amount`）+ outbox，保证并发下不超卖、价格单调 |
| 4 | 封顶价自动成交 & 0 元起拍 | 规则收敛到可单测的 Domain：首笔最低有效价 = start_price 或 price_step |
| 5 | 实时同步：WS + 事件序号 | 裸 WebSocket + seq 去重 + `/events` 补偿 + `/status` 快照，禁用 socket.io |
| 6 | 弱网与断线补偿 | 连接状态机 connected→reconnecting→polling，退避 1/2/5/10s，回前台必拉 /status |
| 7 | 时间一致性 | 倒计时用 server_time 偏移 + requestAnimationFrame，误差 ≤500ms，拒绝 setInterval 漂移 |
| 8 | 竞价氛围体验 | 翻牌/领先/被超越/倒计时/延时/成交六类动画，framer-motion + Web Audio，60fps、尊重 reduced-motion |
| 9 | 工程化：合同驱动 + mock-first | openapi → TS 类型；MSW mock-first 让三人解耦并行，按 checklist 切真接口 |
| 10 | AI 使用与落地 | AI 提速样板，关键决策（金额单位/契约/一致性）人工把控；见 ai-contribution-report |

## 备注
- 第 2 页架构图：用 `docs/presentation/architecture.md` 的 mermaid 导出 PNG（Excalidraw/draw.io 美化）。
- 不做：复杂甘特图、团队组织架构图、封面装饰。
