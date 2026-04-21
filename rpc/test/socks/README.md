# socks —— 基于 `rpc` 双向 RPC 框架的 HTTP / SOCKS 代理示例

本目录是一个基于仓库自研 `rpc` 双向 RPC 框架 + `codec.Upgrade`（类似 WebSocket 的
连接升级）实现的 **双端（client/service）透明代理** 示例。client 侧监听本地 SOCKS
（`:10080`）和 HTTP 代理端口（`:18081` / `:18082`），将流量通过已建立的 TLS 长
连接转发到 service 侧，由 service 侧代发到目标地址。

```
browser ──► socks cli (:10080/:18081/:18082)
              │ TLS + rpc.Peer + codec.Upgrade
              ▼
           socks svc ──► 目标网站
```

目录结构：

| 路径 | 说明 |
| --- | --- |
| `peer_client.go` | SOCKS/HTTP 入口、`OutToTCPPeer`（upgrade 模式）、`OutToTCPPeer1`（stream 模式） |
| `peer_service.go` | 服务端 `Conn`（stream 模式）/ `ConnUpgrade`（upgrade 模式），以及拷贝工具 `Copy` |
| `peer_proxy.go` | 把当前节点当作中继（peer-of-peer）的代理实现 |
| `client/` | 客户端 main + 静态 TLS 配置 |
| `service/` | 服务端 main + 静态 TLS 配置 |
| `pb/` | protobuf 协议定义 |

运行方式：

```bash
# service
cd rpc/test/socks/service && go run .

# client
cd rpc/test/socks/client && go run .
# 然后浏览器配置 HTTP 代理 127.0.0.1:18081 即可
```

---

## 已知问题与本轮修复

### 现象

client 端运行中频繁出现如下 error 级日志：

```json
{
  "level":"error",
  "caller":"socks/peer_service.go:299",
  "error":{
    "code":-1,
    "msg":" n < l, err: write tcp 127.0.0.1:18081->127.0.0.1:52217: wsasend: An established connection was aborted by the software in your host machine.",
    "stack":[
      "(socks/peer_service.go:299) socks.Copy",
      "(socks/peer_client.go:281) socks.(*SocksCli).OutToTCPPeer",
      "(socks/peer_client.go:200) socks.(*SocksCli).connectHttp"
    ]
  },
  "message":"Copy defer"
}
```

### 根因分析

1. 日志调用路径：
   - `peer_service.go:299` 是 `Copy` 函数中 `dst.Write` 失败的错误包装；
   - `peer_client.go:281` 是 `OutToTCPPeer` 里 `Copy(ctx, *inConn, upgrade)`。
2. 此时 `*inConn` 是浏览器到本地代理端口 `127.0.0.1:18081` 的 TCP 连接；
   `write tcp 127.0.0.1:18081->127.0.0.1:52217` 的 `52217` 是浏览器侧端口。
3. Windows 的 `wsasend: An established connection was aborted by the software
   in your host machine.` 属于 `WSAECONNABORTED`，在代理场景里几乎总是出现于
   **浏览器端已关闭/重置连接（关 tab、HTTP keep-alive 过期、RST 等）之后，
   我们还在往 `*inConn` 写数据**。它是一次**完全正常**的连接关闭事件。
4. 原实现中存在三个放大这个问题的缺陷：
   - **日志级别过高**：所有 Write 失败都按 `error` 级别打印，连正常的
     "对端已关闭" 都会产生大量 error 日志，淹没真正的错误。
   - **两方向拷贝未联动**：goroutine 运行 `Copy(ctx, upgrade, *inConn)`、主流程
     运行 `Copy(ctx, *inConn, upgrade)`，任一方向结束时没有唤醒另一方向，
     导致阻塞在 `Read` 上的协程迟迟不退出，产生 goroutine 泄漏。
   - **`src.Read` 不响应 `ctx`**：`Copy` 内部只有 writer 检查 `ctx.Done()`，
     reader 一直阻塞在 `src.Read` 上，必须由外部 `SetDeadline` / `Close`
     才能唤醒。

### 修复点（本轮提交）

均位于 `rpc/test/socks/peer_service.go` 和 `rpc/test/socks/peer_client.go`：

1. **新增 `isBenignCloseErr`**：识别常见 benign-close 错误（跨平台）——
   - `io.EOF` / `io.ErrUnexpectedEOF` / `io.ErrClosedPipe` / `net.ErrClosed`；
   - Linux: `connection reset by peer` / `broken pipe`；
   - Windows: `forcibly closed` / `aborted by the software`；
   - 框架内部显式关闭: `has been closed` / `Codec is closed` / `upgrade closed`。
2. **重写 `Copy`**：
   - writer 主循环不再把 short write / 关闭错误强制包装成 `" n < l"`；
   - `defer` 统一做收尾：
     - benign close → 打 `debug` 日志并把 `err` 置为 `nil`（正常退出）；
     - 真错误 → 打 `error` 日志；
     - 通过 `src.SetDeadline(time.Now())` 唤醒阻塞在 `Read` 的 reader 协程；
   - 使用 `context.WithCancel`，writer 出错即 `cancel()`，reader 立即收手。
3. **`OutToTCPPeer` / `ConnUpgrade` 的收尾联动**：
   - 两个方向 `Copy` 退出后都会 `upgrade.Close()` + 关闭/唤醒本地 `rc`/`inConn`，
     确保对端拿到 EOF，另一方向的拷贝也能立刻退出；
   - 对应的 `peer.Close()` 只在 `OutToTCPPeer` 主流程（而非子协程）里执行一次。
4. **`rpc/codec/stream.go` `Codec.Stream`**：增加对已关闭 codec 的判断，
   避免在复用关闭的 peer 时 panic 于 `c.streams == nil`。

### 效果

- 浏览器侧主动断开不再打 error，日志从几百条/分钟的噪声恢复安静；
- 任一方向任一层（浏览器 / upgrade 隧道 / 目标网站）关闭都会触发另一方向
  立即退出，goroutine 不再泄漏；
- 运行过程中若出现真正的网络异常（DNS 失败、RPC 心跳超时等），仍会按
  `error` 级别打印，便于排查。

### 手工回归

```bash
# 先启动 service
cd rpc/test/socks/service && go run .

# 再启动 client（默认同时监听 :10080 SOCKS、:18081 HTTP-upgrade、:18082 HTTP-stream）
cd rpc/test/socks/client && go run .

# 浏览器配 HTTP 代理 127.0.0.1:18081 访问大站 / 刷新 / 关 tab，
# 观察 logs/*.log：应不再出现 "wsasend: ... aborted by the software" 的 error 日志。
```

---

## 与仓库内其他 rpc 框架对比

`rpc` 相比 gRPC / kitex / net/rpc 的差异，在顶层 `rpc/` 的 `README` 有详细
说明，此处仅列出本示例用到的两个核心能力：

- **双向 RPC**：同一条 TCP 连接上，service 也可以反向调用 client。本示例里
  `RegisterSocksCliServer` + `NewSocksSvcClient` 同时注册，使得 client 既能
  发起 `ConnUpgrade`，service 也能调用 `Close` 之类命令。
- **`codec.Upgrade`**：借鉴 WebSocket，把 rpc 连接"升级"为裸 TCP 隧道；
  升级之后这条连接不再跑 rpc 协议，所有字节原样透传，**一个 peer 只能承载
  一个 upgrade stream**。这也解释了 `OutToTCPPeer` 为什么每个请求都要 `peer.Close()`
  —— 不是 bug，是协议本身的限制。stream 模式 (`OutToTCPPeer1`) 则可以复用 peer。

### 适用场景

- 需要自己掌控协议细节、传输不规则（二进制流 / 需要 pass-through 透传）的
  场景，比如本例的 HTTP/SOCKS 代理；
- 客户端与服务端地位对等、需要双向主动调用；
- 对部署简单性要求高（单一 TLS 端口、无需 HTTP/2 依赖）。

### 不太适合

- 需要和 gRPC 生态打通（反射 / grpc-gateway / proto3 严格兼容）；
- 复杂的负载均衡、熔断、观测（需要生态支持）。
