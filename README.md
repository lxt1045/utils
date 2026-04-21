# utils

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](./LICENSE)

`github.com/lxt1045/utils` 是一套自用的 Go 基础库 + 一个**自研的双向 RPC 框架**。
设计目标是把作者在网络代理 / 消息系统 / 内部服务之间反复用到的"轮子"沉淀
下来，直接 `import` 即可使用。

> ⚠️ **状态：pre-1.0**。对外 API 在 v1.0.0 发布前不保证向后兼容。生产使用前
> 建议先阅读 [代码质量与生产可用性评估](./work.md)。

---

## 目录速览

```
utils/
├── rpc/      ⭐ 自研双向 RPC 框架（TCP / QUIC / KCP / UDP）
├── log/      基于 zerolog 的结构化日志 + ctx 透传 logid
├── config/   基于 yaml + embed.FS 的配置加载，含 TLS 证书加载工具
├── cache/    本地 LoadingCache（singleflight + bigcache 后端）
├── delay/    延时队列 / 一写多读队列（用于超时回调）
├── gid/      高吞吐全局 ID 生成器（snowflake 变体，基于 runtime.nanotime）
├── tag/      reflect-based struct tag 解析与缓存
├── cert/     自签 CA / 服务端 / 客户端证书生成工具
├── socks/    SOCKS / HTTP 代理协议实现
├── sigma/    Sigma 规则引擎（安全告警匹配）
├── grpc/     gRPC server/client 启停工具，集成 mTLS 与中间件
└── work.md   每轮工作上下文记录（含质量评估）
```

更精细的 RPC 框架架构与示例见 `rpc/README.md`（待补）；
SOCKS 代理示例见 `rpc/test/socks/README.md`。

---

## 模块索引

| 模块 | 用途 | 主要 API | 测试覆盖 | 备注 |
| --- | --- | --- | --- | --- |
| `rpc` | 双向 RPC 框架；同一连接上 client/service 都能主动调用 | `rpc.StartPeer`, `rpc.NewPeer`, `Peer.Stream`, `Peer.Upgrade` | 中（连接 / 服务 / 协议 各 1-2 文件） | ⭐ 仓库核心 |
| `rpc/codec` | wire 层：编解码、ReadLoop、Stream 复用、Upgrade 升级 | `codec.NewCodec`, `Codec.Stream`, `Codec.Upgrade` | 通过 `rpc/` 间接覆盖 | 状态机：normal / upgrading / upgraded |
| `rpc/conn` | 传输层适配：QUIC / KCP / UDP | `conn.SetReadWriteBuff` 等 | 低 | TCP 走 `net.Conn` 即可 |
| `rpc/socket` | TCP/UDP `Dial/Listen` 包装，含端口复用 | `socket.Dial`, `socket.DialTLS`, `socket.Listen` | ✓ | Windows 用 `SO_REUSEADDR/PORT` |
| `log` | zerolog + lumberjack + ctx logid + Gin/GORM 适配 | `log.Ctx(ctx).Info()`, `log.WithLogid` | ✓ | 默认 RFC3339Nano 时间戳 |
| `config` | yaml 配置加载 + embed.FS 支持 + `LoadTLSConfig` | `config.UnmarshalFS`, `config.LoadTLSConfig` | ✓ | 内置 `Conn`/`DB`/`GRPC`/`Log`/`TLS` 结构 |
| `cache` | LoadingCache（factory + load + singleflight） | `cache.NewLoadingCache` | ✓ | 后端可选 bigcache / noop |
| `delay` | 一写多读延时队列 | `delay.NewQueue[T]` | ✓ | 用于 `rpc.Codec` 的超时队列 |
| `gid` | 全局 ID 生成器（agent-id + 时间 + 序列） | `gid.GetGID`, `gid.InitClient/InitService` | ✓ | 基于 `//go:linkname runtime.nanotime` |
| `tag` | struct tag 反射解析与缓存 | `tag.NewTagInfos` | ✓ | 用于 yaml/json 之外的自定义反序列化 |
| `cert` | 自签 CA / 服务端 / 客户端 / 多级证书生成 | `cert.NewRootCA`, `cert.GenServerCert` | ✓ | 默认 10 年有效期 |
| `socks` | SOCKS5 / HTTP CONNECT 协议解析与代理 | `socks.Handshake`, `socks.TCPLocalOnly` | 低 | 被 `rpc/test/socks/` 复用 |
| `sigma` | [Sigma 规则](https://github.com/SigmaHQ/sigma)引擎，泛型匹配任意结构 | `sigma.NewRulesetByDir[T]` | ✓ | 含 KMP no-case 加速 |
| `grpc` | gRPC server 启停 + mTLS + validator/recovery 中间件 | `grpc.NewServer` | 低 | 仅薄封装 |

---

## 架构概览：RPC 框架

```
                ┌──────────────────────────────────────────────┐
                │                    rpc.Peer                  │
                │  ┌───────────────┐         ┌─────────────┐   │
                │  │   Client      │         │   Service   │   │
                │  │ (调用对端)    │         │ (响应对端)  │   │
                │  └──────┬────────┘         └─────┬───────┘   │
                │         │       共享同一条        │           │
                │         └────────► codec ◄───────┘           │
                └──────────────────────│───────────────────────┘
                                       │
                          ┌────────────┴───────────┐
                          │     codec.Codec        │
                          │  ─ ReadLoop()           │
                          │  ─ resps[uint64]        │
                          │  ─ streams[uint64]      │   多路复用
                          │  ─ upgrade *Upgrade     │   一连接一升级
                          │  ─ delay.Queue (timeout)│
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

三个核心抽象：

- **`Peer`**：双向对等体。`StartPeer` 时同时注册 server 端的方法与 client 端
  要调用的方法签名，握手后两侧地位完全对等。
- **`Stream`**：一条连接上的逻辑流，可并发多个；`Send/Recv` 类似 gRPC
  双向 stream，但开销更轻。
- **`Upgrade`**：把整条连接接管为裸 TCP 隧道（类似 WebSocket）。
  **升级后这条 codec 不能再跑普通 RPC**，需要 `peer.Close()` 释放；非常适合
  把 RPC 当作"信令通道"+ 隧道并存的场景（比如本仓库里的 SOCKS 代理示例）。

更详细的协议字段、状态机迁移、与 gRPC 的对比，见 `rpc/README.md`（计划中）。

---

## 与 gRPC / net/rpc 的简短对比

| 维度 | `lxt1045/utils/rpc` | gRPC | net/rpc |
| --- | --- | --- | --- |
| 双向调用 | ✅ 同一连接，client/service 对等 | 仅 stream 双向 | ❌ |
| 连接升级（裸 TCP 隧道） | ✅ `codec.Upgrade` | ❌ | ❌ |
| 多路复用 | ✅ `Stream` | ✅ HTTP/2 | ❌ |
| 协议依赖 | 自定义二进制（含 protobuf payload） | HTTP/2 + protobuf | gob / json |
| 传输层 | TCP / TCP+TLS / QUIC / KCP / UDP | HTTP/2 over TCP | TCP / HTTP |
| 生态（reflection / gateway / 负载均衡） | ❌ 无 | ✅ 完善 | ❌ |
| 二进制体积（最小 hello-world） | 小 | 大（拉 grpc-go） | 极小 |
| 适合场景 | 内网双向控制面、代理隧道、长连接推送 | 跨语言、对外开放 API | 同语言、简单内部调用 |

---

## 版本策略

- 当前 `master` 分支处于 **pre-1.0**：无 git tag，外部依赖请使用 commit
  pseudo-version 或 vendor。
- v1.0.0 之前 API **可能破坏性变更**（最近一次例子：`socket.Dial` 增加
  `localAddr` 参数；`rpc/test/quic` 整目录删除）。
- v1.0.0 发布时会冻结 `rpc.Peer` / `rpc.Client` / `rpc.Service` /
  `codec.Stream` / `codec.Upgrade` 的对外签名；之后破坏性修改一律走 `v2/`
  子目录（Go module semver 标准做法）。
- 子模块（`log` / `config` / `cache` / `delay` / `gid` / `tag` / `cert` /
  `sigma`）目前更稳定，破坏性改动概率低。

---

## 快速上手

### 1. 引入依赖

```bash
go get github.com/lxt1045/utils@latest
```

要求 Go ≥ 1.24（见 [`go.mod`](./go.mod)）。

### 2. 用 RPC 框架写一个最小 echo 服务

```go
// pb/service.proto 用 gogo/protobuf 生成 EchoReq/EchoRsp 与
// Service{ Echo(EchoReq) returns (EchoRsp) }，此处省略生成步骤。

// service main
package main

import (
    "context"
    "net"

    "github.com/lxt1045/utils/log"
    "github.com/lxt1045/utils/rpc"
    "yourmod/pb"
)

type echoSvc struct{}

func (s *echoSvc) Echo(ctx context.Context, req *pb.EchoReq) (*pb.EchoRsp, error) {
    return &pb.EchoRsp{Msg: "hi, " + req.Msg}, nil
}

func main() {
    ctx := context.Background()
    ln, _ := net.Listen("tcp", ":18080")
    for {
        conn, err := ln.Accept()
        if err != nil {
            log.Ctx(ctx).Error().Err(err).Send()
            continue
        }
        go func(c net.Conn) {
            _, err := rpc.StartPeer(ctx, c, &echoSvc{}, pb.RegisterServiceServer)
            if err != nil {
                log.Ctx(ctx).Error().Err(err).Send()
            }
        }(conn)
    }
}
```

```go
// client main
package main

import (
    "context"
    "fmt"
    "net"

    "github.com/lxt1045/utils/rpc"
    "yourmod/pb"
)

func main() {
    ctx := context.Background()
    conn, _ := net.Dial("tcp", "127.0.0.1:18080")

    peer, err := rpc.StartPeer(ctx, conn, nil, pb.NewServiceClient)
    if err != nil {
        panic(err)
    }
    defer peer.Close(ctx)

    var rsp pb.EchoRsp
    if err := peer.Invoke(ctx, "Echo", &pb.EchoReq{Msg: "world"}, &rsp); err != nil {
        panic(err)
    }
    fmt.Println(rsp.Msg) // hi, world
}
```

### 3. 真实可运行的完整示例

仓库 `rpc/test/` 下提供了**多个可直接 `go run` 的端到端示例**，建议从这里开始：

| 路径 | 演示内容 |
| --- | --- |
| `rpc/test/tcp/` | 最小化 RPC over TCP，单方向调用 |
| `rpc/test/tcp_upgrade/` | 演示 `codec.Upgrade` 升级隧道 |
| `rpc/test/socks/` | ⭐ HTTP/SOCKS 代理（最完整，含 README 与故障复盘） |
| `rpc/test/socks_stream/` | 用 `Stream` 而非 `Upgrade` 实现的 SOCKS 代理 |
| `rpc/test/socks_quic/` | RPC over QUIC（`quic-go`）的 SOCKS 代理 |
| `rpc/test/socks_quic2/` | QUIC 版本 + 独立 proxy 进程 |
| `rpc/test/socks_kcp/` | RPC over KCP 的 SOCKS 代理 |
| `rpc/test/socks_udp/` | RPC over UDP 的 SOCKS 代理 |
| `rpc/test/socks_nat/` | TCP NAT 打洞（simultaneous open）+ SOCKS 代理 |
| `rpc/test/test_broadcast/` | 一对多广播 |

运行任意一个示例：

```bash
# 终端 A
cd rpc/test/socks/service && go run .

# 终端 B
cd rpc/test/socks/client && go run .

# 浏览器配 HTTP 代理 127.0.0.1:18081 即可
```

### 4. 其他基础组件示例

```go
// log + ctx logid
import (
    "context"
    "github.com/lxt1045/utils/gid"
    "github.com/lxt1045/utils/log"
)

ctx, _ := log.WithLogid(context.Background(), gid.GetGID())
log.Ctx(ctx).Info().Str("user", "alice").Msg("login")
// {"level":"info","time":"...","user":"alice","logid":1907..., "message":"login"}
```

```go
// config + embed.FS + TLS
//go:embed static/conf/default.yml static/ca/*
var staticFS embed.FS

var cfg myConf
config.UnmarshalFS("static/conf/default.yml", staticFS, &cfg)

tlsCfg, _ := config.LoadTLSConfig(staticFS,
    cfg.Conn.TLS.ServerCert, cfg.Conn.TLS.ServerKey, cfg.Conn.TLS.CACert)
```

```go
// LoadingCache
c, _ := cache.NewLoadingCache(ctx, cache.Factory(...),
    cache.WithLoader(func(ctx context.Context, k string) ([]byte, error) {
        return queryDB(ctx, k)
    }),
)
val, _ := c.Get(ctx, "user:42")
```

更多用法直接看对应包的 `*_test.go`。

---

## 开发与测试

```bash
# 全量构建（注意：链接 socks_quic/service 等大二进制可能在内存紧张机器上 OOM）
go build ./...

# 单元测试（推荐分模块跑，避免一次性触发所有 demo 链接）
go test -count=1 -race -timeout 5m ./config/... ./log/... ./cache/... \
       ./delay/... ./tag/... ./cert/... ./gid/... ./rpc/...

# 静态检查
go vet ./...

# 涉及 mockey 的测试需要禁优化
go test -count=1 -gcflags="all=-N -l" ./gid

# Windows 交叉编译为 Linux 二进制
$env:CGO_ENABLED=0; $env:GOOS="linux"; $env:GOARCH="amd64"
go build ./rpc/test/socks/service
```

已知 `go vet ./...` 会报两处历史遗留的 `unreachable code`
（`rpc/conn/kcp.go:178`、`rpc/test/nat/client/client.go:138`），不影响功能，
将在后续 PR 中清理。

---

## 项目状态与已知限制

详见 [`work.md`](./work.md) 中的逐轮工作记录与代码质量评估。摘要：

- ✅ 核心功能（`rpc` / `log` / `config` / `cache` / `gid` / `cert` / `sigma`）
  在作者自用环境长期稳定运行。
- ✅ 跨平台（Linux / Windows）；2026-04-21 修复了 Windows 上 `wsasend`
  benign-close 错误被错误打 `error` 级别的问题，详见
  `rpc/test/socks/README.md`。
- ⚠️ 测试覆盖率约 15%（文件级），主要集中在 `rpc/`、`sigma/engine/`、
  `log/`、`config/` 等关键路径。
- ⚠️ 暂无 CI/CD pipeline、暂未跑过 `govulncheck`，依赖版本偏老
  （`golang.org/x/net v0.12.0` 等）。
- ❌ `rpc` 框架对外 API 尚未冻结，请自行评估升级风险。

---

## 贡献

欢迎以 issue / PR 的方式提出问题和改进。提交 PR 前请确保：

1. `go build ./...` 与 `go vet ./...` 通过（已知的 2 处 `unreachable code`
   除外）；
2. 涉及目录的 `go test -count=1 -race -timeout 60s ./...` 通过；
3. 改动同步到 [`work.md`](./work.md) 与（如适用）相关目录的 `README.md`。

详细约定见 [`.cursor/rules/project.mdc`](./.cursor/rules/project.mdc)。

---

## License

[MIT](./LICENSE) © lxt1045
