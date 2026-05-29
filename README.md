# Auction System

Windows-friendly local setup for the live auction MVP.

## Prerequisites

- Go 1.22+
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
