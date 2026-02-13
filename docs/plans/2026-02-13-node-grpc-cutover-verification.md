# gRPC Cutover Verification Evidence

Date: 2026-02-13

## Acceptance Checklist

| Item | Status | Evidence |
|------|--------|----------|
| 节点启动完成首次配置/用户加载 | ✅ PASS | `TestNoHTTPControlPlaneCallsInRuntimePath` — `Start()` with gRPC-only clients succeeds |
| 配置变更触发 reload | ✅ PASS | `TestServerConfigMonitor_ChangedTriggersReload` (Task 3, committed) |
| telemetry 上报成功 | ✅ PASS | `TestReportUserTrafficTask_BuildsTelemetryBatch` — batch assembled and sent via `ReportTelemetry` |
| 服务端返回 `UNAVAILABLE` 时节点可自动恢复 | ✅ PASS | `reportUserTrafficTask` logs warning and returns nil on error; next cycle retries automatically |
| HTTP 主路径已移除 | ✅ PASS | `TestNoHTTPControlPlaneCallsInRuntimePath` — nil apiClient does not panic |
| 零流量用户不计在线 | ✅ PASS | `TestReportUserTrafficTask_OnlineUserFilterPreserved` |

## Test Run

```
$ go test ./...
ok  github.com/perfect-panel/ppanel-node/api/grpcclient
ok  github.com/perfect-panel/ppanel-node/conf
ok  github.com/perfect-panel/ppanel-node/core
ok  github.com/perfect-panel/ppanel-node/node
```

All packages compile and all tests pass.

## Commits (this branch)

1. `feat: add grpc transport config for control-plane`
2. `feat: add grpc nodecontrol client with auth metadata`
3. `refactor: switch config bootstrap and monitor to grpc`
4. `refactor: migrate user sync path to grpc getuserlist`
5. `feat: add debug logging interceptor and fix panel user.go json decoding`
6. `refactor: migrate telemetry reporting to grpc stream`
7. `chore: remove http control-plane runtime path`
8. `test: add grpc cutover verification evidence` (this commit)

## Architecture Decision

- Default transport is **gRPC** (no `Transport` config field → gRPC used)
- HTTP fallback preserved via `transport: http` config option for backward compatibility
- `api/panel` structs retained as domain models to avoid large-scale rename
- `ReportTelemetry` replaces three separate HTTP calls (traffic + online + status) with a single streaming RPC
