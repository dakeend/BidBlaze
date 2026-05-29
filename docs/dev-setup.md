# 开发环境与项目骨架

> 锁定日期：2026-05-29
> 目标：三人 clone 仓库后，**15 分钟内**能跑起完整本地环境（MySQL + Redis + 后端 + 移动端 + PC 后台）。

---

## 1. 仓库目录骨架

单仓多项目仓库。当前 Windows 可运行骨架以现有目录为准：`server-go/`、`mobile-h5/`、`admin-web/`。

```text
auction-system/
├── README.md                  # Role C 维护，D10 完成
├── docker-compose.yml         # Role A 提供，本地起 MySQL + Redis
├── .env.example               # 全局示例
├── docs/                      # 已有
├── server-go/                 # 后端（Role A 主）
│   ├── go.mod
│   ├── go.sum
│   └── main.go                # 当前启动入口：go run .
├── mobile-h5/                 # Role B
│   ├── package.json
│   ├── package-lock.json
│   ├── vite.config.ts
│   └── src/
├── admin-web/                 # Role C
│   ├── package.json
│   ├── package-lock.json
│   ├── vite.config.ts
│   └── src/
└── scripts/
    └── dev-up.ps1             # Windows 一键启动脚本
```

> `packages/shared/` 可在 D3 由 Role C 抽取公共前端代码时新增；当前不是启动本地环境的前置条件。

---

## 2. 端口分配

| 端口 | 服务 | 容器/进程 |
|---|---|---|
| 3306 | MySQL 8.0 | docker `auction-mysql` |
| 6379 | Redis 7 | docker `auction-redis` |
| 8080 | 后端 HTTP + WS | `go run .` |
| 5173 | 移动端 H5 | `cd mobile-h5; npm.cmd run dev -- --host 0.0.0.0 --port 5173` |
| 5174 | PC 商家后台 | `cd admin-web; npm.cmd run dev -- --host 0.0.0.0 --port 5174` |
| 8025 | （预留）邮件/日志 UI | — |

CORS：后端 `internal/http` 中间件白名单 `http://localhost:5173`、`http://localhost:5174`。生产环境通过 `CORS_ORIGINS` 环境变量覆盖。

---

## 3. docker-compose.yml（Role A 提交到 D1）

```yaml
version: "3.9"
services:
  mysql:
    image: mysql:8.0
    container_name: auction-mysql
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: auction_root
      MYSQL_DATABASE: auction
      TZ: Asia/Shanghai
    command:
      - --default-authentication-plugin=mysql_native_password
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci
      - --default-time-zone=+08:00
    ports:
      - "3306:3306"
    volumes:
      - ./.data/mysql:/var/lib/mysql
      - ./docs/schema-v2.sql:/docker-entrypoint-initdb.d/001_schema.sql:ro
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "127.0.0.1", "-pauction_root"]
      interval: 5s
      timeout: 3s
      retries: 20

  redis:
    image: redis:7-alpine
    container_name: auction-redis
    restart: unless-stopped
    ports:
      - "6379:6379"
    command: ["redis-server", "--appendonly", "yes"]
    volumes:
      - ./.data/redis:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 20
```

启动：
```powershell
docker compose up -d
docker compose ps   # 等到 mysql/redis 都是 healthy
```

---

## 4. .env.example

```ini
# server-go/.env
APP_ENV=dev
APP_PORT=8080
APP_TIMEZONE=Asia/Shanghai

MYSQL_DSN=root:auction_root@tcp(127.0.0.1:3306)/auction?parseTime=true&loc=Asia%2FShanghai&charset=utf8mb4
MYSQL_MAX_OPEN=50
MYSQL_MAX_IDLE=10

REDIS_ADDR=127.0.0.1:6379
REDIS_DB=0

CORS_ORIGINS=http://localhost:5173,http://localhost:5174

UPLOAD_DIR=./uploads
UPLOAD_PUBLIC_PREFIX=http://localhost:8080/static

# Mock auth
MOCK_TOKEN_SECRET=auction-dev-secret-2026
MOCK_TOKEN_TTL_HOURS=720

# Outbox publisher
OUTBOX_POLL_INTERVAL_MS=200
OUTBOX_BATCH_SIZE=100

# Lifecycle worker
LIFECYCLE_TICK_MS=500
LIFECYCLE_BATCH_SIZE=100

# Rate limit
RATE_LIMIT_BID_PER_SEC=3
RATE_LIMIT_STATUS_PER_SEC=2

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

前端：
```ini
# mobile-h5/.env / admin-web/.env
VITE_API_BASE=http://localhost:8080
VITE_WS_BASE=ws://localhost:8080
VITE_CLIENT_TYPE=mobile_h5     # admin-web 改为 admin
```

---

## 5. Mock Token 规则（**写死，全员遵守**）

合同 §1 写"V2 仍允许 mock token"，本节锁死实现细节。

### 5.1 Token 格式

```text
mock-token-<role>-<user_id_padded>
```

- `<role>` ∈ `{seller, buyer}`，**仅作语义提示，鉴权不依赖此字段**。
- `<user_id_padded>`：用户 ID 用零补足至 3 位（>999 后自然增长，例 `mock-token-buyer-1024`）。
- 大小写敏感，全小写。
- 长度上限 64 字符（schema `users.token VARCHAR(128)` 兜底）。

示例（与 `schema-v2.sql` seed 一致）：
```text
mock-token-seller-001   → user_id=1 (主播阿明)
mock-token-buyer-001    → user_id=2 (买家张三)
mock-token-buyer-002    → user_id=3 (买家李四)
```

### 5.2 生成与持久化

`POST /api/login` 处理流程：
1. 按 `nickname` 查 `users`。
2. 若存在 → 直接返回旧 `token`。
3. 若不存在：
   - 当 `nickname` 以「主播」「商家」「卖家」开头 → `role=seller`；否则 `role=buyer`。
   - `INSERT users (nickname, avatar, token='__placeholder__')` 获取 `id`。
   - `UPDATE users SET token = CONCAT('mock-token-', role, '-', LPAD(id, 3, '0')) WHERE id = ?`。
   - 返回新 token。

> 角色仅用于前端体验分流（PC 后台拒绝 buyer 登录），不写入业务规则。任何用户都可以创建拍卖、出价、拥有订单。

### 5.3 解析与鉴权

后端 `internal/auth/middleware.go`：
1. 读 `Authorization: Bearer <token>`。
2. `SELECT id, nickname, avatar FROM users WHERE token = ?`。
3. 命中 → 注入 `gin.Context` 里的 `user`；未命中 → 401 + code 1002。
4. **不解析 token 字符串结构**（避免角色与 DB 不一致），DB 是唯一事实来源。

### 5.4 前端使用

- 登录成功后 token 存 `localStorage.auction_token`。
- Axios 拦截器自动加 `Authorization`。
- WebSocket 通过 query string 传 `?token=<token>`（合同 §3.1）。
- 401 → 清 localStorage → 跳 `/login`。

### 5.5 安全声明

> Mock token **仅用于 MVP/答辩**。生产替换为：
> - JWT（HS256，`MOCK_TOKEN_SECRET` → `JWT_SECRET`）
> - 或 OAuth2 + 短期 access token
>
> 代码中所有 mock token 入口必须有 `// TODO(prod): replace with JWT` 注释。

---

## 6. 一键启动脚本

`scripts/dev-up.ps1`（Role A 提交，跨平台用 Make 或 PowerShell）：

```powershell
# 1. 起依赖
docker compose up -d

# 2. 等 MySQL 健康
do {
    Start-Sleep -Seconds 2
    $health = docker inspect --format='{{.State.Health.Status}}' auction-mysql
    Write-Host "mysql: $health"
} while ($health -ne "healthy")

# 3. 后端
Start-Process powershell -ArgumentList "cd server-go; go run ."

# 4. 前端（并行）
Start-Process powershell -ArgumentList "cd mobile-h5; npm.cmd install; npm.cmd run dev -- --host 0.0.0.0 --port 5173"
Start-Process powershell -ArgumentList "cd admin-web; npm.cmd install; npm.cmd run dev -- --host 0.0.0.0 --port 5174"
```

---

## 7. 工具版本（写死，避免环境差异）

| 工具 | 版本 | 安装提示 |
|---|---|---|
| Go | 1.22.x | `go version` |
| Node | 20.x LTS | nvm-windows |
| npm | Node 20 自带版本 | `npm -v` |
| Docker Desktop | 4.30+ | Windows 需开 WSL2 |
| MySQL Client | 8.0+ | 调试用 |
| k6 | 0.50+ | 压测，Role A 用 |

---

## 8. 第一天检查清单（D1 EOD）— 解耦版

> v1.1：D1 EOD 三人并行解锁不再依赖 Task B；A 只需交付 mock login 桩。

**Role A 必须交付**：

- [ ] `docker compose up -d` → mysql/redis healthy
- [ ] `docs/schema-v2.sql` 自动初始化（含 3 个种子用户与 token）
- [ ] `cd server-go && go run .` 监听 8080
- [ ] `curl http://localhost:8080/health` → `{"status":"ok"}`
- [ ] `curl http://localhost:8080/ready` → mysql/redis 均 ok
- [ ] `curl -X POST http://localhost:8080/api/login -d '{"nickname":"买家张三"}'` → 返回 `mock-token-buyer-001`（**B1 桩**）
- [ ] `.env.example` 复制到 `.env` 即可跑
- [ ] CORS 允许 5173/5174

**Role B 必须交付**：

- [ ] `cd mobile-h5 && npm.cmd install && npm.cmd run dev` 在 5173 起
- [ ] `src/mocks/handlers.ts` 至少覆盖 `/api/login`、`/api/users/me`、`GET /api/auctions(:id)?`、`GET /status`
- [ ] `VITE_USE_MSW=true` 时浏览器 Network 能看到 MSW 标识
- [ ] `useAuctionSocket` 骨架可连本地 mock WS（事件结构按 contract-v2 §3）

**Role C 必须交付**：

- [ ] `cd admin-web && npm.cmd install && npm.cmd run dev` 在 5174 起
- [ ] `src/lib/types.ts` 由 `openapi-typescript` 从 `docs/api/openapi.yaml` 生成
- [ ] `src/lib/{api-client,auth,time,error-codes}.ts` 雏形完成
- [ ] `src/mocks/handlers.ts` 与 mobile-h5 共用 fixture
- [ ] 登录页吃 MSW 跑通

**D2 EOD 追加**：

- [ ] Task B2 真鉴权 middleware + `/api/users/me` 通过 curl
- [ ] mobile-h5 和 admin-web 的登录页切到真 `/api/login`（删除对应 MSW handler）
- [ ] 其余模块仍走 MSW，按 integration-protocol §3 表逐项切真

