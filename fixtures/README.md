# Shared frontend fixtures

`mobile-h5/` 与 `admin-web/` 的 MSW handlers 共用同一份 fixture，避免两端漂移。
（见 `docs/tasks/frontend-agent-tasks.md` Task M1 / P1、`docs/integration-protocol.md §3`。）

## 约定

- 字段取自 `docs/api/openapi.yaml` 的 `example`，snake_case，与 `contract-v2.md` 一致。
- 种子 token：卖家 `mock-token-seller-001`，买家 `mock-token-user-001` / `mock-token-user-002`
  （与 `docs/schema-v2.sql` seed 一致；详见 `dev-setup.md §5`）。
- 联调用拍卖 ID 1–10 保留，演示用从 100 开始（`integration-protocol.md §8`）。

## 建议文件（Role B/C 在 D1 落地）

- `auctions.json` — `GET /api/auctions`、`GET /api/auctions/:id` 的列表/详情样例
- `users.json` — 三个种子用户 + `/api/login`、`/api/users/me` 响应
- `events.json` — WS 事件流样例（按 `docs/events/event-contract.md`）

两端通过相对路径 `import` 本目录 JSON；切真接口时按 `integration-protocol.md §4` 逐模块移除对应 MSW handler。
