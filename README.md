# 直播竞拍系统 · Auction System

一个面向直播电商场景的**实时竞拍系统**：0 元起拍、毫秒级实时竞价、临近结束自动延时反超、封顶价自动成交。
前端 mock-first 可独立跑通，无需后端在场。

> Windows-friendly local setup for the live auction MVP.

## 架构

整体为 `server-go` 单体分层（HTTP API / WS Gateway / Domain Rules / Workers）+ MySQL 8 + Redis 7，
前端分移动端 H5（买家）与 admin-web（商家后台/监控）。
完整架构图、出价时序图、断线补偿时序图见 **[docs/presentation/architecture.md](docs/presentation/architecture.md)**。

## 技术亮点

- **出价正确性**：Redis 锁 + MySQL 条件更新（`current_price + step <= amount`）+ outbox，并发下不超卖、价格单调递增。
- **0 元起拍 / 封顶自动成交**：规则收敛到可单测的 Domain 层。
- **实时同步**：裸 WebSocket + 事件序号去重 + `/events` 补偿 + `/status` 快照（禁用 socket.io）。
- **弱网恢复**：连接状态机（connected→reconnecting→polling）+ 退避重连，回前台必拉 `/status`。
- **时间一致性**：倒计时用 `server_time` 偏移 + `requestAnimationFrame`，误差 ≤500ms。
- **竞价氛围**：翻牌/领先/被超越/倒计时/延时/成交六类动画（framer-motion + Web Audio，60fps）。
- **工程化**：openapi → TS 类型；MSW mock-first 让三人解耦并行，按 checklist 切真接口。

完整答辩材料见下方 [答辩材料](#答辩材料)。

## Prerequisites

- Go 1.26+ (matches `server-go/go.mod`)
- Node.js 20+
- Docker Desktop with WSL2 enabled
- PowerShell 5+

## First Run on Windows

From the repository root:

```powershell
cd E:\code\ai_zijie\auction-system
docker compose up -d
```

Wait until MySQL and Redis are healthy:

```powershell
docker compose ps
```

Start each app in separate terminals:

```powershell
cd server-go
go run .
```

```powershell
cd mobile-h5
npm.cmd install
npm.cmd run dev -- --host 0.0.0.0 --port 5173
```

```powershell
cd admin-web
npm.cmd install
npm.cmd run dev -- --host 0.0.0.0 --port 5174
```

Open:

- Backend health: http://localhost:8080/health
- Mobile H5: http://localhost:5173
- Admin web: http://localhost:5174

## One-Command Startup

After Docker Desktop is running:

```powershell
.\scripts\dev-up.ps1
```

This starts MySQL, Redis, the Go backend, the mobile H5 app, and the admin web app.

## Project Layout

```text
auction-system/
├── admin-web/       # PC merchant/admin frontend
├── mobile-h5/       # Mobile buyer frontend
├── server-go/       # Go backend
├── docs/            # Contracts, tasks, milestones
├── scripts/         # Windows helper scripts
└── docker-compose.yml
```

## Team Entry Points

- Role A starts in `server-go/` and `docs/tasks/backend-agent-tasks.md`.
- Role B starts in `mobile-h5/` and the realtime/mobile sections of `docs/tasks/frontend-agent-tasks.md`.
- Role C starts in `admin-web/` and the PC/admin sections of `docs/tasks/frontend-agent-tasks.md`.

## 答辩材料

| 材料 | 文件 |
|---|---|
| 架构图 / 出价时序 / 断线补偿 | [docs/presentation/architecture.md](docs/presentation/architecture.md) |
| 演示视频分镜脚本（5 分钟） | [docs/presentation/demo-script.md](docs/presentation/demo-script.md) |
| 技术亮点 PPT 大纲（10 页） | [docs/presentation/slides-outline.md](docs/presentation/slides-outline.md) |
| AI 使用与落地效果报告 | [docs/ai-log/ai-contribution-report.md](docs/ai-log/ai-contribution-report.md) |
| AI 协作日志（按日期） | [docs/ai-log/](docs/ai-log/) |

> admin-web 模块说明见 [admin-web/README.md](admin-web/README.md)。
