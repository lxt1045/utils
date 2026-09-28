# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`github.com/lxt1045/utils` is a personal Go infrastructure library: a collection of reusable
components the author settled on across network-proxy / messaging / internal-service work.
It requires **Go ≥ 1.27** (see `go.mod`).

**Status**: Pre-1.0. External APIs may change without notice before v1.0.0.

> **Important change**: the bidirectional RPC framework that used to live in `rpc/` has been
> **extracted to the standalone module `github.com/lxt1045/rpc`** (now a plain dependency in
> `go.mod`). Do not look for `rpc/` sources in this repo; read that module's source in the
> module cache when RPC behavior matters (e.g. `grpc/grpc_test.go` still exercises it).

## Module Layout

Single root module `github.com/lxt1045/utils`, plus **nested independent modules**
(no `go.work` — build them from their own directories):

- `atlas/` — own `go.mod` (`github.com/lxt1045/utils/atlas`): DB schema migration toolkit
  built on ariga.io/atlas (migrate diff/apply/hash, schema inspect), runnable as a cobra CLI.
- `geohash/geos/` — own `go.mod` (`geos`): geometry/merge experiments on top of
  `peterstace/simplefeatures`; has its own README/TODO and `run*.sh` scripts.
- `geohash/` — in root module; has its own [geohash/CLAUDE.md](geohash/CLAUDE.md).
  High-performance geohash encode/decode with AMD64 BMI2 (PDEP) assembly path,
  benchmarked against several third-party geohash libraries.

Root-module packages:

| Package | Purpose | Notes |
| --- | --- | --- |
| `log/` | Structured logging (zerolog + lumberjack), ctx logid propagation, Gin/GORM/slog adapters | `log.Ctx(ctx).Info()` |
| `config/` | YAML config loading with embed.FS, env-var chained assignment, TLS cert loading | `config.UnmarshalFS` |
| `cache/` | LoadingCache (factory + singleflight) with pluggable backends: bigcache, fastcache, redis | `cache.NewLoadingCache` |
| `delay/` | Delay queues / one-writer-many-reader queues for timeout callbacks | used by rpc timeouts |
| `gid/` | High-throughput global ID generator (snowflake variant on `runtime.nanotime` via `//go:linkname`) | tests need mockey flags |
| `tag/` | reflect-based struct tag parsing and caching | |
| `cert/` | Self-signed CA / server / client cert generation | |
| `socks/` | SOCKS5 / HTTP proxy protocol implementation (+ `socks/http` proxy server with tunnel bridging) | |
| `sigma/` | Sigma rule engine for security alert matching (`sigma/engine`: KMP nocase acceleration) | generic over any struct |
| `grpc/` | gRPC server utilities with mTLS and middleware (validator/recovery/logging) | thin wrapper |
| `channel/` | `ChanN[T]` — overwrite-not-block multi-item channel built on native chan | |
| `db/` | Multi-DB connection helpers (MySQL/Postgres/ClickHouse) via gorm + sqlx | |
| `ck/` | ClickHouse sqlx helpers (flatten, disk, `ToSqlxResult`) | |
| `etcd/` | etcd client wrapper: init, watch (with reconnect), typed local cache | |
| `crypto/` | `aes` encryption helpers, `hash` (string→int64) | |
| `tools/` | Generic helpers: deep copy, sortmap, uuid, str, temp dir, time, `Must`/`ToP`/`IfV` etc. | |
| `postgresql/` | Postgres CDC demo using `Trendyol/go-pq-cdc` (logical replication slot) | `package main` demo |

## Development Commands

### Building
```bash
# Build all root-module packages (may OOM on memory-constrained machines at link stage)
go build ./...

# Build specific components
go build ./log/... ./config/... ./cache/...

# Nested modules must be built from their own directory
cd atlas && go build ./...
cd geohash/geos && go build ./...
```

### Testing
```bash
# Test by module to avoid triggering everything at once
go test -count=1 -race -timeout 5m ./config/... ./log/... ./cache/... ./delay/... ./tag/... ./cert/... ./gid/... ./channel/... ./tools/...

# Tests using mockey require disabled optimization
go test -count=1 -gcflags="all=-N -l" ./gid

# Static analysis
go vet ./...
```

## Engineering Conventions (from `.cursor/rules/project.mdc`)

- Error wrapping: **always** use `github.com/lxt1045/errors` for stack-traced error chains;
  keep compatibility with stdlib `errors.Is/As`.
- No `panic` / `os.Exit` in production code — return errors and let callers decide.
- Comments may be Chinese, but must explain *why*, not restate *what*.
- Logging: never log benign connection-close errors at `error` level. Benign set includes
  `io.EOF`, `io.ErrUnexpectedEOF`, `io.ErrClosedPipe`, `net.ErrClosed`,
  Linux `connection reset by peer` / `broken pipe`, Windows `forcibly closed` /
  `aborted by the software`, framework `has been closed` / `Codec is closed` /
  `upgrade closed`. Use an `isBenignCloseErr()`-style check in proxy/forwarding code.
- Tests with goroutines: use `t.Errorf` + `return` instead of `t.Fatal` (Fatal in a
  non-test goroutine does not stop the test cleanly).

### Two-way Copy Pattern (network proxy code)
1. Both copy directions must coordinate shutdown — either direction ending should
   immediately wake the other (`src.SetDeadline(time.Now())` or `Close()`).
2. Benign close errors log at debug level, not error level.

## External RPC Framework (`github.com/lxt1045/rpc`)

Historical context from when it lived here (still relevant when reading dependent code):
- `rpc.Peer`: bidirectional peer — client and service on the same connection.
- `codec.Codec`: wire layer with ReadLoop, timeout queue, stream multiplexing, and a
  status state machine (0=normal, 1=upgrading, 2=upgraded).
- `codec.Upgrade`: promotes the connection to a raw TCP tunnel — **one upgrade per peer;
  the peer is unusable for RPC afterward and must be closed**.
- Transports: TCP, TCP+TLS, QUIC, KCP, UDP.

## Context Files

- `work.md` — per-round work log; append new sections, never delete history.
- `TODO.md` — current task list (in Chinese); requires following `.cursor/rules/` and
  writing a summary + work-context entry to `work.md` after each task.
- `geohash/CLAUDE.md` — package-specific guidance for the geohash module.

## Known Issues

- `go build ./...` can OOM at link stage on memory-constrained machines; build per-package.
- No CI/CD pipeline yet.
- Test coverage is uneven (focused on critical paths); some historical `*_test.go` files
  may no longer compile/run — check before assuming they pass.
