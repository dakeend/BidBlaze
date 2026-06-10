# Backend Structure Integration Plan

Goal: make `server-go` the single backend entrypoint.

Implemented scope:

- Migrate Role A backend modules from `server-go/wjh/internal` into `server-go/internal`.
- Restore `auth` and `auction` packages from the earlier Role A commit.
- Add `internal/http` for unified envelopes, error codes, request IDs, CORS, readiness, and route registration.
- Add `internal/storage` for MySQL and Redis constructors.
- Replace root `main.go` so auth, auction, bid, upload, realtime, outbox publisher, and lifecycle worker start from one process.
- Remove the nested `server-go/wjh` Go module to avoid two backend entrypoints.
- Add `server-go/Dockerfile`.
- Update README and auxiliary lifecycle compose path.

Verification:

- `go test ./...` from `server-go` in WSL Ubuntu with `GOPROXY=https://goproxy.cn,direct`.
- `npm.cmd run build` from `admin-web`.
- `npm.cmd run build` from `mobile-h5`.
