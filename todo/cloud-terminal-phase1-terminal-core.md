# Phase 1 - cs-cloud Terminal 核心

> 提案参考：[docs/cloud-terminal.md](../docs/cloud-terminal.md) §3

**预计耗时**：2-3 天
**前置条件**：Phase 0 完成（DefaultShell 配置）
**状态**：`done`

---

## 任务清单

### 1.1 添加 PTY 依赖

- [x] `go get github.com/creack/pty`
- [x] 验证 Linux/macOS/Windows 三平台编译通过

### 1.2 实现 PTY 会话封装 (`internal/terminal/session.go`)

- [x] 定义 `Session` 结构体（ID, Pid, Ptmx, Cols, Rows, Cwd, subscribers）
- [x] `Start()` — 启动 PTY 进程（shell 发现 + creack/pty Start）
- [x] `ReadOutput()` — 后台 goroutine 从 ptmx 读取输出，广播到 subscribers
- [x] `Write(data []byte)` — 写入输入到 ptmx
- [x] `Resize(rows, cols uint16)` — 调整 PTY 窗口大小
- [x] `Close()` — 关闭 PTY + 清理资源
- [x] `Subscribe() (<-chan []byte, func())` — 订阅输出流
- [x] Shell 发现逻辑：
  - Windows: `CS_CLOUD_SHELL` → `ComSpec` → `pwsh` → `powershell` → `cmd`
  - Unix: `CS_CLOUD_SHELL` → config `DefaultShell` → `SHELL` → `/bin/zsh` → `/bin/bash` → `/bin/sh`

### 1.3 实现终端管理器 (`internal/terminal/manager.go`)

- [x] 定义 `TerminalManager` 结构体（sessions map, shell, maxSlots）
- [x] `NewManager(opts ...Option)` — 构造函数
- [x] `Create(cwd string, rows, cols uint16) (*Session, error)`
- [x] `Get(id string) (*Session, error)`
- [x] `Kill(id string) error`
- [x] `Resize(id string, rows, cols uint16) error`
- [x] `Restart(id string, cwd string) (*Session, error)` — kill 旧 + 创建新
- [x] `Write(id string, data []byte) error`
- [x] `Subscribe(id string) (<-chan []byte, func())`
- [x] `CleanupIdle(timeout time.Duration)` — 清理空闲会话
- [x] 最大并发限制（默认 20）

### 1.4 实现 REST Handlers (`internal/terminal/handlers.go`)

- [x] `HandleCreate` — `POST /terminal` 创建会话
- [x] `HandleKill` — `DELETE /terminal/{id}` 终止会话
- [x] `HandleResize` — `POST /terminal/{id}/resize` 调整大小
- [x] `HandleRestart` — `POST /terminal/{id}/restart` 重启会话
- [x] `HandleInput` — `POST /terminal/{id}/input` HTTP 输入（备用）

### 1.5 实现 SSE 输出流 (`internal/terminal/handlers.go`)

- [x] `HandleStream` — `GET /terminal/{id}/stream` SSE 输出
- [x] 事件类型：`connected`、`data`（base64）、`exit`
- [x] 心跳保活（默认 15 秒）
- [x] 客户端断开自动取消订阅

### 1.6 实现 WebSocket 输入 (`internal/terminal/input_ws.go`)

- [x] `HandleInputWs` — `GET /terminal/input-ws` WebSocket 端点
- [x] 多路复用协议：
  - 绑定会话：`{"t":"b","s":"<sessionId>","v":1}`
  - 心跳 ping/pong：`{"t":"p","v":1}` / `{"t":"po","v":1}`
  - 文本帧 = 终端按键数据
- [x] 使用 `nhooyr.io/websocket`（已有依赖）

### 1.7 集成到 localserver + daemon

- [x] `server.go` 中注册所有终端路由到 api mux
- [x] `daemon.go` 中启动 idle 清理 goroutine（60s 检查，30min 超时）
- [x] daemon shutdown 时关闭所有 PTY 会话

---

## 验收标准

- [ ] `POST /api/v1/terminal` 返回 sessionId + pid
- [ ] `GET /api/v1/terminal/{id}/stream` 收到 SSE 输出流
- [ ] 通过 WS 发送按键后终端有响应输出
- [ ] `POST /api/v1/terminal/{id}/resize` 正确调整 PTY 大小
- [ ] `DELETE /api/v1/terminal/{id}` 正确终止 PTY 进程
- [ ] `POST /api/v1/terminal/{id}/restart` 在同一 cwd 重启
- [ ] 30 分钟无输入后会话自动清理
- [ ] 最大 20 并发会话限制生效
- [ ] `CS_CLOUD_SHELL` 和 config.json `default_shell` 生效

## 产出文件

| 文件 | 操作 |
|------|------|
| `go.mod` / `go.sum` | 修改：添加 creack/pty + conpty |
| `internal/terminal/session.go` | 新增 |
| `internal/terminal/session_unix.go` | 新增：Unix PTY 实现 (creack/pty) |
| `internal/terminal/session_windows.go` | 新增：Windows ConPTY 实现 (UserExistsError/conpty) |
| `internal/terminal/kill_unix.go` | 新增：Unix 进程终止 |
| `internal/terminal/kill_windows.go` | 新增：Windows taskkill 进程树终止 |
| `internal/terminal/manager.go` | 新增 |
| `internal/terminal/handlers.go` | 新增 |
| `internal/terminal/helpers.go` | 新增 |
| `internal/terminal/input_ws.go` | 新增 |
| `internal/localserver/server.go` | 修改：注册终端路由 |
| `internal/cli/daemon.go` | 修改：启动 idle 清理 |