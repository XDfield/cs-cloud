# 云端终端 (Cloud Terminal) 技术设计

> 基于 openchamber 的 SSE 输出 + WS 输入双通道架构，由 cs-cloud 管理 PTY，通过 gateway 隧道穿透到浏览器。

## 1. 架构总览

```
┌──────────────────────────────────────────────────────────────────┐
│  app-ai-native (浏览器)                                           │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ TerminalView                                               │  │
│  │  ├─ ghostty-web (canvas 渲染)                               │  │
│  │  ├─ SSE: GET  /cloud/device/{id}/proxy/api/v1/terminal/     │  │
│  │  │         {sessionId}/stream                               │  │
│  │  └─ WS:  WS   /cloud/device/{id}/proxy/api/v1/terminal/     │  │
│  │              input-ws                                       │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────┬───────────────────────────────────────┘
                           │ HTTPS/WSS
┌──────────────────────────▼───────────────────────────────────────┐
│  costrict-web (云端服务器)                                        │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ DeviceProxyHandler                                         │  │
│  │  → gateway.Client.ProxyRequest()                           │  │
│  │    → SSE 流式转发 (已有支持)                                  │  │
│  │    → WebSocket 透传 (已有支持)                                │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────┬───────────────────────────────────────┘
                           │ WebSocket + yamux 多路复用
┌──────────────────────────▼───────────────────────────────────────┐
│  cs-cloud (设备端 daemon, Go)                                     │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ localserver (127.0.0.1:{randomPort})                       │  │
│  │  ├─ /api/v1/terminal              POST   创建 PTY 会话      │  │
│  │  ├─ /api/v1/terminal/{id}         DELETE 终止 PTY 会话      │  │
│  │  ├─ /api/v1/terminal/{id}/resize  POST   调整 PTY 大小      │  │
│  │  ├─ /api/v1/terminal/{id}/restart POST   重启 PTY           │  │
│  │  ├─ /api/v1/terminal/{id}/stream  GET    SSE 输出流         │  │
│  │  ├─ /api/v1/terminal/{id}/input   POST   HTTP 输入(备用)    │  │
│  │  └─ /api/v1/terminal/input-ws     WS     WebSocket 输入     │  │
│  │                                                            │  │
│  │ TerminalManager                                            │  │
│  │  ├─ pty sessions map[string]*Session                       │  │
│  │  ├─ Shell 发现 (SHELL env / platform fallback)              │  │
│  │  └─ creack/pty (Go PTY 库)                                  │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

## 2. 为什么复用现有 proxy 而不加新路由

costrict-web 已有完整的 proxy 链路：

1. **`DeviceProxyHandler`** (`gateway/handlers.go:221`) — 验证用户身份 + 设备归属，然后调用 `client.ProxyRequest()`
2. **`ProxyRequest`** (`gateway/client.go:33`) — 检测 SSE (`text/event-stream`) 和 WebSocket (`Upgrade: websocket`)，分别做流式/透传处理
3. **cs-cloud tunnel** (`tunnel/proxy.go`) — yamux 流上解析 HTTP，检测 WebSocket Upgrade，TCP 透传到 localserver

**结论：不需要在 costrict-web 加任何新路由。** 现有的 `/cloud/device/:deviceID/proxy/*path` 已完全支持 SSE 和 WebSocket 透传。

## 3. cs-cloud 实现方案

### 3.1 新增依赖

```go
// go.mod
require github.com/creack/pty v1.1.21  // 跨平台 PTY 支持
```

### 3.2 新增文件

```
internal/terminal/
├── manager.go       # TerminalManager — PTY 会话管理
├── session.go       # Session — 单个 PTY 会话封装
├── handlers.go      # HTTP handlers (注册到 localserver)
└── input_ws.go      # WebSocket 输入协议 (多路复用)
```

### 3.3 TerminalManager

```go
type TerminalManager struct {
    mu       sync.Mutex
    sessions map[string]*Session
    shell    string   // 检测到的 shell 路径
    maxSlots int      // 最大并发 (默认 20)
}

type Session struct {
    ID        string
    Pid       int
    Ptmx      *os.File
    Cols      uint16
    Rows      uint16
    Cwd       string
    CreatedAt time.Time
    LastInput time.Time
    cancel    context.CancelFunc
    // 输出广播
    subscribers map[string]chan []byte
    subMu       sync.RWMutex
}
```

**关键方法：**
- `Create(cwd string, rows, cols uint16) (*Session, error)`
- `Get(id string) (*Session, error)`
- `Kill(id string) error`
- `Resize(id string, rows, cols uint16) error`
- `Write(id string, data []byte) error`
- `Subscribe(id string) (<-chan []byte, func())` — 返回输出 channel + 取消函数
- `CleanupIdle(timeout time.Duration)` — 清理空闲会话

**Shell 发现逻辑：**
```
Windows: OPENCHAMBER_TERMINAL_SHELL → SHELL → ComSpec → pwsh → powershell → cmd
Unix:    OPENCHAMBER_TERMINAL_SHELL → SHELL → /bin/zsh → /bin/bash → /bin/sh
```

### 3.4 HTTP API

#### 创建会话
```
POST /api/v1/terminal
Body: { "cwd": "/home/user/project", "rows": 24, "cols": 80 }
Response: { "ok": true, "data": { "sessionId": "t_abc123", "pid": 12345 } }
```

#### 删除会话
```
DELETE /api/v1/terminal/{sessionId}
Response: { "ok": true, "data": {} }
```

#### 调整大小
```
POST /api/v1/terminal/{sessionId}/resize
Body: { "rows": 40, "cols": 120 }
Response: { "ok": true, "data": {} }
```

#### 重启
```
POST /api/v1/terminal/{sessionId}/restart
Body: { "cwd": "/home/user/project" }
Response: { "ok": true, "data": { "sessionId": "t_abc123", "pid": 12399 } }
```

#### SSE 输出流
```
GET /api/v1/terminal/{sessionId}/stream?heartbeat=15000
Response: Content-Type: text/event-stream

event: connected
data: {}

event: data
data: <base64-encoded-bytes>

event: data
data: <base64-encoded-bytes>

event: exit
data: {"exitCode":0}
```

SSE 输出是 base64 编码因为 PTY 输出是二进制（可能包含非 UTF-8 序列）。

#### HTTP 输入（备用）
```
POST /api/v1/terminal/{sessionId}/input
Body: { "data": "<base64-encoded-bytes>" }
Response: { "ok": true, "data": {} }
```

#### WebSocket 输入（多路复用）

参考 openchamber 的 `TERMINAL_INPUT_WS_PROTOCOL.md`：

```
WS /api/v1/terminal/input-ws

客户端 → 服务端：
  文本帧: 终端按键原始数据 (热路径，延迟最低)
  文本帧: {"t":"b","s":"<sessionId>","v":1}  — 绑定会话
  文本帧: {"t":"p","v":1}                     — 心跳 ping

服务端 → 客户端：
  文本帧: {"t":"po","v":1}                     — 心跳 pong
```

### 3.5 集成到 localserver

在 `server.go` 的 `New()` 函数中：

```go
termMgr := terminal.NewManager()
// ... 注册路由
api.HandleFunc("POST /terminal", termMgr.HandleCreate)
api.HandleFunc("DELETE /terminal/{id}", termMgr.HandleKill)
api.HandleFunc("POST /terminal/{id}/resize", termMgr.HandleResize)
api.HandleFunc("POST /terminal/{id}/restart", termMgr.HandleRestart)
api.HandleFunc("GET /terminal/{id}/stream", termMgr.HandleStream)    // SSE
api.HandleFunc("POST /terminal/{id}/input", termMgr.HandleInput)     // HTTP
api.HandleFunc("GET /terminal/input-ws", termMgr.HandleInputWs)      // WebSocket
```

### 3.6 idle 清理

在 daemon 启动时开一个后台 goroutine，每 60 秒检查一次，kill 掉超过 30 分钟无输入的会话。

## 4. app-ai-native 实现方案

### 4.1 依赖变更

```diff
- "ghostty-web": "github:anomalyco/ghostty-web#main"
+ "ghostty-web": "0.3.0"                    // npm 版本，与 openchamber 一致
```

复制 openchamber 的 patch 到 `patches/ghostty-web+0.3.0.patch`。

### 4.2 新增/修改文件

```
src/
├── components/
│   └── terminal/
│       ├── TerminalView.tsx          # 新：主终端组件 (SSE+WS)
│       ├── TerminalViewport.tsx      # 新：ghostty-web 渲染层
│       └── cloud-terminal-api.ts     # 新：云端终端 API 客户端
├── addons/
│   └── serialize.ts                  # 保留：已有，无需修改
├── context/
│   └── terminal.tsx                  # 改：增加 cloud 模式分支
└── pages/session/
    └── terminal-panel.tsx            # 改：根据模式选择组件
```

### 4.3 CloudTerminalApi

```typescript
class CloudTerminalApi {
  private sse: EventSource | null = null
  private ws: WebSocket | null = null

  async connect(deviceId: string, sessionId: string): Promise<void>

  // SSE 输出
  private connectSSE(deviceId: string, sessionId: string): void
  onData(callback: (data: Uint8Array) => void): void
  onExit(callback: (exitCode: number) => void): void

  // WS 输入
  private connectInputWs(deviceId: string): void
  sendInput(data: string): void     // 终端按键
  bindSession(sessionId: string): void

  // REST
  create(deviceId: string, cwd: string, rows: number, cols: number): Promise<TerminalSession>
  kill(deviceId: string, sessionId: string): Promise<void>
  resize(deviceId: string, sessionId: string, rows: number, cols: number): Promise<void>
  restart(deviceId: string, sessionId: string, cwd: string): Promise<TerminalSession>

  disconnect(): void
}
```

### 4.4 TerminalView 分支

```typescript
// context/terminal.tsx 中根据连接模式选择实现
const isCloud = server.url.includes('/cloud/device/')

if (isCloud) {
  // 使用 CloudTerminalApi (SSE + WS)
} else {
  // 保持现有 SDK WebSocket 方式不变
}
```

### 4.5 云端 URL 构建

```
REST:  {cloudBase}/cloud/device/{deviceId}/proxy/api/v1/terminal
SSE:   {cloudBase}/cloud/device/{deviceId}/proxy/api/v1/terminal/{sessionId}/stream
WS:    wss://{host}/cloud/device/{deviceId}/proxy/api/v1/terminal/input-ws
```

## 5. costrict-web 变更

**无需任何变更。** 现有 proxy 链路完整支持：

| 功能 | 支持情况 | 实现位置 |
|------|---------|---------|
| SSE 流式转发 | ✅ 已有 | `client.go:43-131` — 检测 `text/event-stream`，streaming flush |
| WebSocket 透传 | ✅ 已有 | `client.go:39-41` + `proxyWebSocket()` — 检测 `Upgrade: websocket`，hijack+relay |
| yamux SSE | ✅ 已有 | `tunnel/proxy.go:proxyHTTP()` — 双向 TCP relay |
| yamux WS | ✅ 已有 | `tunnel/proxy.go:proxyWebSocket()` — WebSocket upgrade 透传 |
| 用户认证 | ✅ 已有 | `DeviceProxyHandler` — RequireAuth + 设备归属检查 |

## 6. 实施计划

### Phase 1: cs-cloud Terminal 核心 (预估 2-3 天)

1. 添加 `creack/pty` 依赖
2. 实现 `terminal/manager.go` — PTY 会话管理 + shell 发现
3. 实现 `terminal/session.go` — 单个 PTY 封装 + 输出广播
4. 实现 `terminal/handlers.go` — REST + SSE handlers
5. 实现 `terminal/input_ws.go` — WebSocket 输入
6. 注册到 localserver
7. 编写测试

### Phase 2: app-ai-native Cloud Terminal (预估 2-3 天)

1. 切换 ghostty-web 到 0.3.0 + patch
2. 实现 `cloud-terminal-api.ts`
3. 修改 `terminal.tsx` — 添加 cloud 模式分支
4. 修改 `terminal-panel.tsx` — 模式判断
5. 处理 SSE base64 解码 + ghostty-web 写入
6. 端到端测试

### Phase 3: 联调与优化 (预估 1-2 天)

1. 端到端联调
2. 断线重连处理
3. 错误提示与状态显示
4. 性能调优 (写缓冲、SSE 心跳)

## 7. 风险与注意事项

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| `creack/pty` Windows 兼容性 | Windows 上可能需要额外处理 | 测试 Windows 平台，备选 `conpty` API |
| SSE 通过多层代理的延迟 | 终端响应可能有额外延迟 | 30s 心跳保活，SSE 自动重连 |
| yamux 流上的 WebSocket 透传 | 需要验证 yamux 长连接稳定性 | 已有先例 (openchamber)，风险可控 |
| ghostty-web 版本切换 | patch 兼容性 | patch 已在 openchamber 验证 |
| PTY 会话泄漏 | 设备资源耗尽 | idle timeout + 最大会话数限制 |
