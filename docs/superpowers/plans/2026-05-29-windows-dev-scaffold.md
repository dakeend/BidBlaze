# Windows Dev Scaffold Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the docs and root scaffold with the current Windows-friendly repository layout so three developers can start from the same commands.

**Architecture:** Keep existing `admin-web/`, `mobile-h5/`, and `server-go/main.go` layout. Add root-level dependency services and startup helpers, then update docs to describe npm-based frontend startup instead of a missing pnpm workspace.

**Tech Stack:** Docker Compose, PowerShell, Go, npm, Vite, MySQL 8, Redis 7.

---

### Task 1: Add Root Runtime Files

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `scripts/dev-up.ps1`
- Create: `README.md`

- [ ] **Step 1: Add Docker Compose for dependencies**

Create MySQL and Redis services with fixed ports `3306` and `6379`, mounting `docs/schema-v2.sql` into MySQL initialization.

- [ ] **Step 2: Add environment examples**

Create a root `.env.example` with backend and frontend settings matching the documented local ports.

- [ ] **Step 3: Add a Windows startup script**

Create `scripts/dev-up.ps1` that starts Docker dependencies, waits for MySQL health, then launches backend, mobile H5, and admin web in separate hidden PowerShell windows.

- [ ] **Step 4: Add a concise README**

Document prerequisites, first-time setup, daily startup, and individual service commands for Windows users.

### Task 2: Align Docs With Current Repo

**Files:**
- Modify: `docs/dev-setup.md`
- Modify: `docs/team-assignment.md`
- Modify: `docs/tasks/frontend-agent-tasks.md`
- Modify: `docs/integration-protocol.md`
- Modify: `docs/milestones.md`
- Modify: `docs/prompts/role-C-merchant-ai-polish.md`

- [ ] **Step 1: Rename documented PC app path**

Replace `admin-web/` and `admin-web` with `admin-web/` and `admin-web`.

- [ ] **Step 2: Replace pnpm workspace assumptions**

Replace root workspace startup commands with per-app npm commands.

- [ ] **Step 3: Align backend entrypoint**

Replace `go run .` with `go run .` where the command is intended to run inside `server-go/`.

- [ ] **Step 4: Preserve future shared package notes as optional**

Where docs mention `packages/shared/`, describe it as a future shared extraction instead of a currently required directory.

### Task 3: Verify

**Files:**
- Read-only verification across repo.

- [ ] **Step 1: Confirm root files exist**

Run path checks for `docker-compose.yml`, `.env.example`, `scripts/dev-up.ps1`, and `README.md`.

- [ ] **Step 2: Confirm docs no longer reference missing PC path**

Search docs for `admin-web`.

- [ ] **Step 3: Check frontend builds if dependencies are already installed**

Run `npm run build` in `mobile-h5/` and `admin-web/`.

- [ ] **Step 4: Check backend compiles**

Run `go test ./...` in `server-go/`.

