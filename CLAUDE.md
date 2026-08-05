# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

dionysus is a Go microservice framework (module `github.com/gowins/dionysus`, Go 1.19) that unifies three server types — **gin** (HTTP), **grpc**, and **ctl** (long-running task/worker) — under a single Cobra-based CLI. Registering a sub-command with the framework wires in logging, config, tracing, metrics, health checks, and graceful shutdown automatically. It also ships utility packages (orm, redis, memcache, grpool, httpclient, kafka, rmq, errors) meant to be imported à la carte.

README.md and the sub-package READMEs are written in Chinese; the `.qoder/repowiki/` directory holds generated English wiki docs.

## Commands

```sh
# Run tests (CI uses -short to skip integration tests needing external services)
go test -short ./...

# Full test suite (may require nacos/etcd/redis/mysql/rocketmq to be reachable)
go test ./...

# Single package / single test
go test ./healthy/...
go test -run TestGetGrpcHealthyServer ./healthy/

# Coverage (matches .github/workflows/ci.yml)
go test -short ./... -coverprofile=./coverage.txt -covermode=atomic

go vet ./...
go build ./...
```

Tests use both `testing` and `github.com/smartystreets/goconvey/convey`. CI (`.github/workflows/ci.yml`) runs on pull requests only.

## Architecture

### Lifecycle (the core abstraction)

Everything runs through `Dio` (`dio.go`). `dionysus.NewDio()` creates the root Cobra command; `d.DioStart(projectName, cmds...)` registers sub-commands and executes. The ordered lifecycle is:

```
PersistentPreRun → PreRun → Run → (shutdown) → PostRun → PersistentPostRun
```

- **Framework-owned steps** run in `PersistentPreRunE`/`PersistentPostRunE`: logger → conf → tracing → metric (pprof). These are fixed and registered as priority 1–4 system steps.
- **User steps** are registered via `d.RegUserNthPreRunStep` / `d.RegUserNthPostRunStep` (`dio_steps.go`) — use these for dependency init (DB connections, etc.) and cleanup. Append-style registration is available via `PreRunStepsAppend` / `PostRunStepsAppend`.
- Only the sub-command's `Run` is required from users; PreRun/PostRun are opt-in.

### Step ordering (`step/`)

`step.Steps` is a priority queue executed lowest-priority-first. Priority bands are meaningful and must be respected:
- **System steps**: 0–100 (`SystemPrioritySteps = 100`)
- **User steps**: 100–10100 (`UserPrioritySteps` — user steps add `SystemPrioritySteps` to their ordinal so they always run after system steps)
- **User append steps**: 10101+ (`UserAppendPrioritySteps`, auto-incrementing)

When adding a new framework init stage, pick a system priority (1–10 via `RegSysNthSteps`) that reflects its dependency order.

### Sub-commands (`cmd/`)

Each server type implements the `cmd.Commander` interface (`GetCmd() *cobra.Command`, `GetShutdownFunc() StopFunc`, `RegShutdownFunc(...StopStep)`):
- `NewGinCommand(opts...)` — wraps go-zero-style gin router (`ginx.ZeroGinRouter`); addr from `GAPI_ADDR` env (default `:8080`).
- `NewGrpcCmd(opts...)` — wraps `grpc/server.GrpcServer`; add interceptors via `AddUnaryServerInterceptors`; default `:8081`.
- ctl command — for workers with no request traffic.

`DioStart` wraps each sub-command's `RunE` with `wrapCobrCmdRun` (`dio.go`), which runs `Run` in a goroutine and blocks on OS signals (SIGINT/SIGTERM/SIGQUIT). On signal it invokes the registered `shutdownFunc`; if `Run` exits on its own, shutdown is **not** called. `SIGHUP` is ignored. Shutdown/start panics are recovered and logged.

### Health checks (`healthy/`, `cmd/healthy_cmd.go`)

Three probe types — **startup**, **liveness**, **readiness** — each gated by both a registered `Checker` hook *and* a manual status switch (both must pass). Register app checkers with `healthy.RegLivenessCheckers` / `RegReadinessCheckers` / `RegStartupCheckers`.

`DioStart` auto-registers the matching health sub-commands based on the sub-command's `Use` (`addHealthCmd` in `dio.go`). The binary probes itself via exec, e.g. `{binary} liveness`. Toggle a probe with the `HEALTH_STATUS` env var (`open`/`close`), e.g. `HEALTH_STATUS=close {binary} readiness` to drain traffic.

Per type: gin exposes `/healthx/{startup,liveness,readiness}` HTTP routes; grpc registers a health gRPC service; ctl (no traffic concept, liveness only) writes a timestamp to a temp file (`os.TempDir()/dio.healthy`) that the liveness check reads.

### gRPC service discovery (`grpc/registry/`, `grpc/balancer/`)

`registry.Registry` is the discovery abstraction with etcdv3 and nacos implementations. Registries self-register into a global map; `registry.Init(rawUrl, opts...)` selects one by URL scheme (e.g. `nacos://...`, `etcd://...`) and parses `secure`/`timeout`/`ttl` from query params. Client-side load balancing and custom resolvers (direct + discovery-based) live in `grpc/balancer/`; the grpc client connection pool is in `grpc/client/pool/`.

### Config & logging

- `config/` — viper-based, reads `./etc/config.yaml` by default, supports hot-reload via `WatchConfigHandler` registered in `config.Setup`.
- `log/` — zap-based structured logger; `log.Setup(log.SetProjectName(...), log.WithWriter(...))` is called by the framework during PreRun. Use the package-level `log.Infof`/`log.Errorf` etc.

## Conventions

- Interceptors/middleware are split by side: `grpc/serverinterceptors/` and `grpc/clientinterceptors/` (auth, hystrix breaker, ratelimit, timeout, recovery, opentracing).
- gin handlers return a `ginx.Render` (`ginx.Success(data)` / `ginx.Error(ginx.NewGinError(code, msg))`); business error codes are separate from HTTP status codes and must be ≥ 100000 (`ginx.SetDefaultErrorCode`).
- `example/` contains runnable demos for every major feature (ginx, ctl, grpc, grpool, httpclient, kafka, rmq, log, opentelemetry) — the best reference for intended usage.
- `Start()` and `defaultDio` in `dio.go` are deprecated; use `NewDio()` + `DioStart`.
