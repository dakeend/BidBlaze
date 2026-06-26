# BidBlaze

> AI-powered live streaming auction platform for e-commerce

直播电商实时竞拍系统，包含 Go 后端、移动端 H5、PC 商家后台、MySQL、Redis 和项目文档。

## Architecture

`server-go` is the single backend entrypoint. Role A backend modules that were previously under `server-go/wjh` have been integrated into `server-go/internal`.

Backend package layout:

```text
server-go/
├── main.go
├── internal/
│   ├── auth        # login, token lookup, auth middleware
│   ├── auction     # auction create/update/list/detail/cancel
│   ├── bid         # bid rules, idempotency, Redis lock, outbox writes
│   ├── config      # environment configuration
│   ├── http        # unified responses, errors, router
│   ├── outbox      # outbox publisher
│   ├── realtime    # WebSocket gateway and event replay
│   ├── storage     # MySQL and Redis constructors
│   ├── upload      # image upload endpoint
│   └── worker      # lifecycle worker
```

Frontend package layout:

```text
admin-web/   # PC merchant/admin frontend
mobile-h5/   # mobile buyer frontend
```

Auxiliary directories:

```text
docs/        # contracts, tasks, milestones, presentation material
fixtures/    # mock fixture data
scripts/     # Windows helper scripts
wjh/         # auxiliary docker-compose/env files, not a Go source module
```

## Prerequisites

- Go 1.26+
- Node.js 20+
- Docker Desktop with WSL2 enabled
- PowerShell 5+

## First Run On Windows

From the repository root:

```powershell
cd E:\code\ai_zijie\auction-system
docker compose up -d
```

Wait until MySQL and Redis are healthy:

```powershell
docker compose ps
```

Start the backend:

```powershell
cd server-go
go run .
```

Start the mobile H5 app:

```powershell
cd mobile-h5
npm.cmd install
npm.cmd run dev -- --host 0.0.0.0 --port 5173
```

Start the admin web app:

```powershell
cd admin-web
npm.cmd install
npm.cmd run dev -- --host 0.0.0.0 --port 5174
```

Open:

- Backend health: http://localhost:8080/health
- Mobile H5: http://localhost:5173
- Admin web: http://localhost:5174

## Verification

```powershell
cd server-go
go test ./...
```

```powershell
cd mobile-h5
npm.cmd run build
```

```powershell
cd admin-web
npm.cmd run build
```

## Key Documents

- API contract: [docs/contract-v2.md](docs/contract-v2.md)
- Backend task split: [docs/tasks/backend-agent-tasks.md](docs/tasks/backend-agent-tasks.md)
- Frontend task split: [docs/tasks/frontend-agent-tasks.md](docs/tasks/frontend-agent-tasks.md)
- Integration test plan: [docs/tasks/integration-test-plan.md](docs/tasks/integration-test-plan.md)
- Architecture notes: [docs/presentation/architecture.md](docs/presentation/architecture.md)
