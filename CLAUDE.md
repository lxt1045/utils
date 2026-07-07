# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`github.com/lxt1045/utils` is a Go utility library featuring a custom bidirectional RPC framework plus common infrastructure components. It requires Go ≥ 1.24.

**Status**: Pre-1.0. External APIs may change without notice before v1.0.0.

## Core Components

### RPC Framework (`rpc/`)

The repository's centerpiece - a custom bidirectional RPC framework supporting:
- **Bidirectional calls**: Client and service can both initiate calls on the same connection
- **Multiple transports**: TCP, TCP+TLS, QUIC, KCP, UDP
- **Connection upgrade**: Can promote a connection to raw TCP tunnel (similar to WebSocket)
- **Stream multiplexing**: Multiple logical streams per connection

Key concepts:
- `rpc.Peer`: Bidirectional peer containing both client and service functionality
- `codec.Codec`: Protocol layer with ReadLoop, timeout queues, and state machine (0=normal, 1=upgrading, 2=upgraded)
- `codec.Stream`: Logical streams for multiplexing multiple flows over one connection
- `codec.Upgrade`: Connection promotion to raw TCP tunnel - **one upgrade per peer, makes peer unusable for RPC afterward**

### Infrastructure Modules

- `log/`: Structured logging (zerolog) with context logid propagation
- `config/`: YAML configuration loading with embed.FS and TLS cert utilities
- `cache/`: LoadingCache with singleflight and pluggable backends
- `delay/`: Delay queues and one-writer-many-reader queues for timeout handling
- `gid/`: High-throughput global ID generator (snowflake variant using runtime.nanotime)
- `tag/`: Struct tag parsing and caching
- `cert/`: Self-signed CA/server/client certificate generation
- `socks/`: SOCKS5/HTTP proxy protocol implementation
- `sigma/`: Sigma rule engine for security alert matching
- `grpc/`: gRPC server utilities with mTLS and middleware

## Development Commands

### Building
```bash
# Build all packages (warning: may OOM on memory-constrained machines due to large binaries)
go build ./...

# Build specific components
go build ./rpc/... ./log/... ./config/...

# Cross-compile for Linux (Windows PowerShell)
$env:CGO_ENABLED=0; $env:GOOS="linux"; $env:GOARCH="amd64"; go build ./path/to/target
```

### Testing
```bash
# Recommended: Test by module to avoid triggering all demos at once
go test -count=1 -race -timeout 5m ./config/... ./log/... ./cache/... ./delay/... ./tag/... ./cert/... ./gid/... ./rpc/...

# Tests using mockey require disabled optimization
go test -count=1 -gcflags="all=-N -l" ./gid

# Static analysis
go vet ./...
```

### Running Examples
The `rpc/test/` directory contains working examples:

```bash
# Most complete example: SOCKS proxy
cd rpc/test/socks/service && go run .    # Terminal 1
cd rpc/test/socks/client && go run .     # Terminal 2
# Browser: Configure HTTP proxy 127.0.0.1:18081

# Other examples
cd rpc/test/tcp/service && go run .      # Basic TCP RPC
cd rpc/test/socks_quic/service && go run . # QUIC transport
cd rpc/test/socks_stream/service && go run . # Stream mode (no upgrade)
```

## Architecture Notes

### RPC Framework Architecture
```
                ┌──────────────────────────────────────────────┐
                │                    rpc.Peer                  │
                │  ┌───────────────┐         ┌─────────────┐   │
                │  │   Client      │         │   Service   │   │
                │  │ (calls peer)  │         │ (responds)  │   │
                │  └──────┬────────┘         └─────┬───────┘   │
                │         │       shared codec       │        │
                │         └────────► codec ◄─────────┘        │
                └──────────────────────│──────────────────────┘
                                       │
                          ┌────────────┴───────────┐
                          │     codec.Codec        │
                          │  ─ ReadLoop()           │
                          │  ─ resps[uint64]        │
                          │  ─ streams[uint64]      │   multiplexing
                          │  ─ upgrade *Upgrade     │   one per connection
                          │  ─ delay.Queue          │
                          │  ─ status (0/1/2)       │
                          └────────────┬────────────┘
                                       │ io.ReadWriteCloser
                  ┌────────────────────┼────────────────────┐
                  │                    │                    │
              ┌───┴───┐            ┌───┴───┐           ┌────┴────┐
              │  TCP  │            │ QUIC  │           │   KCP   │
              │ +TLS  │            │       │           │  / UDP  │
              └───────┘            └───────┘           └─────────┘
```

### Two-way Copy Pattern
For network proxy code (A⇄B forwarding):
1. Both copy directions must coordinate shutdown - any direction ending should immediately wake the other
2. Use `src.SetDeadline(time.Now())` or `Close()` to wake blocked readers when writer encounters errors
3. Benign close errors (EOF, connection reset, etc.) should log at debug level, not error level

## Code Quality Guidelines

- Error handling uses `github.com/lxt1045/errors` for stack traces
- Windows/Linux cross-platform: Use `isBenignCloseErr()` pattern for network errors
- `Codec` methods must be nil-safe - fields become nil after `Close()`
- Use `sync.RWMutex` correctly: `RLock` for reads, `Lock` for writes
- Tests with goroutines: use `t.Errorf` + `return` instead of `t.Fatal`

## Key Examples

Most comprehensive examples are in `rpc/test/socks/` with detailed README covering:
- Bidirectional RPC setup
- Connection upgrade for tunneling  
- Two-way copy pattern implementation
- Cross-platform error handling
- Windows-specific `wsasend` error debugging

## Known Issues

- Two `unreachable code` warnings in `rpc/conn/kcp.go:178` and `rpc/test/nat/client/client.go:138` (historical)
- No CI/CD pipeline yet
- Test coverage ~15% (focused on critical paths)
- Dependencies need updates (`golang.org/x/net v0.12.0` etc.)