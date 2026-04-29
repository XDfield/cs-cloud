# 云端指令下发与设备自升级/自重启/自重连 技术提案

## 1. 项目现状

### 1.1 已有能力

| 能力 | 位置 | 状态 |
|------|------|------|
| WebSocket tunnel 反向代理 | `internal/tunnel/connect.go` | 可用，无限重连循环 |
| 自动升级模块（check/download/verify/replace/rollback） | `internal/updater/` | 已实现，**未接入 daemon** |
| 设备注册 + gateway-assign | `internal/device/` | 可用 |
| Heartbeat 方法 | `internal/device/token.go:57` | 死代码，未调用 |
| Token 轮换 | `internal/device/token.go:16` | 死代码，未调用 |
| 本地 HTTP API 服务 | `internal/localserver/` | 可用，约 30 个端点 |
| 跨平台 daemon 后台运行 | `internal/cli/daemon.go` | 可用，`_daemon` 子命令 |

### 1.2 当前架构

```
Cloud Gateway ←── WebSocket + Yamux ──→ cs-cloud daemon
                                              │
                                     localserver (HTTP API)
                                     ├─ /api/v1/runtime/*
                                     ├─ /api/v1/agents/*
                                     ├─ /api/v1/conversations/*
                                     ├─ /api/v1/terminal/*
                                     └─ (无命令控制端点)
```

**tunnel 本质**：纯 HTTP 反向代理。云端通过 WebSocket + Yamux 向 localserver 发起 HTTP 请求，设备代理到本地 HTTP 服务。无消息类型、无控制通道、无命令协议。

### 1.3 关键缺口

| # | 缺口 | 影响 |
|---|------|------|
| 1 | `Manager.Run()` 未在 daemon 中启动 | 自动升级形同虚设 |
| 2 | `Manager.RestartCh` 无人消费 | 升级完成后无法触发重启 |
| 3 | daemon 无自重启机制 | 无法在进程内 re-exec 自身 |
| 4 | localserver 无命令端点 | 云端无法下发控制指令 |
| 5 | `Client.Heartbeat()` 未调用 | 云端无法感知设备在线状态 |
| 6 | 无命令执行结果回报 | 云端不知道指令是否成功 |
| 7 | 无并发命令保护 | 多条指令同时到达可能竞态 |

---

## 2. 设计目标

1. **云端可下发指令**：通过 tunnel 向设备发送 upgrade / restart / reconnect 命令
2. **设备自升级**：下载 → 校验 → 替换二进制 → 自重启 → 验证 → 自动回滚
3. **设备自重启**：daemon 进程可自行 re-exec，保留运行参数和状态
4. **设备自重连**：重启后 tunnel 自动重连（已有，无需额外实现）
5. **结果回报**：设备执行指令后主动上报结果给云端
6. **安全可靠**：命令鉴权、并发保护、失败回滚、审计日志

---

## 3. 总体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Cloud Platform                         │
│  ┌──────────────┐    ┌───────────────┐                      │
│  │ Command API  │    │  Version API  │                      │
│  │ /api/devices │    │ /api/updates  │                      │
│  │  /{id}/cmd   │    │  /check       │                      │
│  └──────┬───────┘    └───────┬───────┘                      │
└─────────┼─────────────────────┼──────────────────────────────┘
          │                     │
          │  HTTP (via tunnel)  │  HTTP (direct)
          ▼                     ▼
┌─────────────────────────────────────────────────────────────┐
│                    cs-cloud daemon                          │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                  localserver                          │   │
│  │  ┌────────────────────────────────────────────────┐  │   │
│  │  │  Command Handler (NEW)                         │  │   │
│  │  │  POST /api/v1/commands/upgrade                 │  │   │
│  │  │  POST /api/v1/commands/restart                 │  │   │
│  │  │  POST /api/v1/commands/reconnect               │  │   │
│  │  │  GET  /api/v1/commands/status                  │  │   │
│  │  └──────────────┬─────────────────────────────────┘  │   │
│  │                 │                                     │   │
│  │  ┌──────────────▼─────────────────────────────────┐  │   │
│  │  │  Command Dispatcher (NEW)                      │  │   │
│  │  │  ├─ upgrade  → UpdaterManager.Apply()          │  │   │
│  │  │  ├─ restart  → SelfRestarter.Restart()         │  │   │
│  │  │  └─ reconnect→ TunnelManager.Reconnect()       │  │   │
│  │  └──────────────┬─────────────────────────────────┘  │   │
│  └─────────────────┼────────────────────────────────────┘   │
│                     │                                        │
│  ┌──────────────────▼────────────────────────────────────┐   │
│  │  Daemon Lifecycle (MODIFIED)                          │   │
│  │  ├─ 启动 UpdaterManager.Run() (定时检查)              │   │
│  │  ├─ 启动 HeartbeatLoop (心跳)                         │   │
│  │  ├─ 监听 RestartCh → SelfRestart()                    │   │
│  │  ├─ 监听 ReconnectCh → 重置 tunnel                    │   │
│  │  └─ 监听 shutdown → graceful shutdown                 │   │
│  └───────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. 核心设计

### 4.1 命令通道

**复用现有 tunnel**，无需新建传输层。云端通过 tunnel 向 localserver 发送 HTTP 请求。

#### 4.1.1 命令 API 定义

```
POST /api/v1/commands
```

请求体：

```json
{
  "command_id": "cmd-abc123",
  "type": "upgrade",
  "payload": {
    "version": "v1.3.0",
    "force": false
  },
  "timestamp": "2026-04-29T10:00:00Z"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `command_id` | string | 云端生成的唯一命令 ID，用于去重和结果回报 |
| `type` | string | 命令类型：`upgrade` / `restart` / `reconnect` |
| `payload` | object | 命令参数，各类型不同 |
| `timestamp` | string | 云端下发时间（RFC3339） |

响应：

```json
{
  "command_id": "cmd-abc123",
  "status": "accepted",
  "message": "upgrade scheduled"
}
```

| status | 说明 |
|--------|------|
| `accepted` | 命令已接受，异步执行中 |
| `rejected` | 命令被拒绝（如：命令重复、当前正在升级） |
| `executing` | 命令正在执行中 |

#### 4.1.2 命令查询

```
GET /api/v1/commands/status?command_id=cmd-abc123
```

响应：

```json
{
  "command_id": "cmd-abc123",
  "type": "upgrade",
  "status": "completed",
  "started_at": "2026-04-29T10:00:01Z",
  "completed_at": "2026-04-29T10:02:30Z",
  "result": {
    "previous_version": "v1.2.3",
    "current_version": "v1.3.0"
  }
}
```

#### 4.1.3 命令类型定义

**upgrade（升级）**

```json
{
  "type": "upgrade",
  "payload": {
    "version": "v1.3.0",
    "force": false,
    "download_url": "https://...",
    "sha256": "e3b0c442..."
  }
}
```

payload 字段均为可选。若省略 `download_url`，设备通过 `Checker.Check()` 自行查询。

**restart（重启）**

```json
{
  "type": "restart",
  "payload": {
    "reason": "config_update",
    "delay_seconds": 5
  }
}
```

**reconnect（重连）**

```json
{
  "type": "reconnect",
  "payload": {
    "gateway": "gw2.example.com"
  }
}
```

### 4.2 命令调度器

新增 `internal/localserver/command_dispatcher.go`：

```go
type CommandDispatcher struct {
    mu        sync.Mutex
    active    map[string]*CommandStatus
    updater   *updater.Manager
    tunnelMgr *TunnelManager
    restarter *SelfRestarter
    reporter  *CommandReporter
}

func (d *CommandDispatcher) Dispatch(cmd *Command) (*CommandAck, error)
func (d *CommandDispatcher) Status(commandID string) (*CommandStatus, error)
```

调度逻辑：

1. 检查 `command_id` 是否已存在（去重）
2. 检查是否有正在执行的命令（互斥）
3. 根据 `type` 路由到对应 handler
4. 返回 `accepted` / `rejected`
5. 异步执行命令
6. 执行完成后通过 `CommandReporter` 回报结果

### 4.3 自重启机制

新增 `internal/app/selfrestart.go`：

#### 核心原则

daemon 是通过 `_daemon` 子命令启动的 detached 子进程。自重启的核心思路是：**先启动新进程，再退出当前进程**。

```go
func SelfRestart(a *app.App) error {
    exe, err := os.Executable()
    if err != nil {
        return err
    }
    exe, err = filepath.EvalSymlinks(exe)
    if err != nil {
        return err
    }

    args := a.LoadArgs()
    if len(args) == 0 {
        args = []string{"_daemon"}
    }

    a.SaveState("restarting")

    cmd := exec.Command(exe, args...)
    // detached，与 start.go 中的 newDaemonCmd 一致
    startDetached(cmd)

    // 触发 graceful shutdown
    os.Exit(0)
    return nil
}
```

#### PID 文件交接

```
时间线：
  1. 旧进程 SaveState("restarting")
  2. 旧进程启动新进程（新 PID）
  3. 新进程启动 → 检测 state == "restarting"
  4. 新进程覆盖 PID 文件（WritePID）
  5. 旧进程 os.Exit(0)
```

新进程在 `runDaemon()` 开头检测 `state == "restarting"` 时跳过 `start` 中的注册流程（因为 `device.json` 仍然有效）。

#### 平台差异

| 平台 | 启动新进程 | 退出旧进程 |
|------|-----------|-----------|
| Linux/macOS | `exec.Command()` + `cmd.Start()` | `os.Exit(0)` |
| Windows | `CREATE_NEW_PROCESS_GROUP \| CREATE_NO_WINDOW` | `os.Exit(0)` |

Windows 上旧进程退出后，新进程可以正常接管。由于二进制替换已完成（`replacer.go` 中的 `.old` 策略），新进程加载的是新二进制。

### 4.4 Tunnel 重连管理

新增 `internal/tunnel/manager.go`：

```go
type Manager struct {
    cancel context.CancelFunc
    localPort int
}

func (m *Manager) Reconnect() {
    m.cancel()  // 取消当前 tunnel.Connect 循环
    // 外层 runDaemon 中的 select 监听到 tunnel 断开
    // 重新启动 tunnel.Connect
}
```

`tunnel.Connect()` 本身已经是无限重连循环。要强制重连，只需取消当前 context，外层逻辑重新调用 `Connect()`。

若需要切换 gateway，可：
1. 更新 `device.json` 中的 `base_url`
2. 触发 reconnect
3. `Connect()` 重新 `AssignGateway()` 时会使用新的 base_url

### 4.5 心跳循环

在 daemon 中启动心跳 goroutine，复用已有的 `Client.Heartbeat()` 方法：

```go
func heartbeatLoop(ctx context.Context, client *device.Client) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := client.Heartbeat(); err != nil {
                logger.Warn("[heartbeat] failed: %v", err)
            }
        }
    }
}
```

心跳间隔建议 5 分钟，云端可据此判断设备是否在线。若 3 个周期未收到心跳，标记设备离线。

### 4.6 命令结果回报

新增 `internal/localserver/command_reporter.go`：

设备执行完命令后，主动向云端 API 回报结果：

```
POST {cloudBaseURL}/api/devices/{deviceID}/commands/{commandID}/result
Authorization: Bearer {deviceToken}

{
  "command_id": "cmd-abc123",
  "status": "completed",
  "result": { ... },
  "error": "",
  "completed_at": "2026-04-29T10:02:30Z"
}
```

回报时机：
- 命令执行成功后立即回报
- 命令执行失败时回报 error 信息
- 升级场景：新进程启动验证后回报（跨进程）

### 4.7 Daemon 生命周期改造

改造 `internal/cli/daemon.go` 中的 `runDaemon()`：

```go
func runDaemon(a *app.App) error {
    // ... 现有信号、日志、localserver 初始化 ...

    // NEW: 命令调度器
    dispatcher := NewCommandDispatcher(srv, a)

    if mode == "cloud" {
        info, _ := device.LoadDevice()

        // NEW: 启动心跳
        deviceClient := device.NewClient(a.Config())
        go heartbeatLoop(ctx, deviceClient)

        // NEW: 启动 updater 定时检查
        updaterMgr := updater.NewManager(
            a.CloudBaseURL(), a.RootDir(),
            updater.WithPolicy(updater.PolicyAuto),
        )
        go updaterMgr.Run(ctx)

        // 启动 tunnel
        go tunnel.Connect(ctx, srv.Port())

        // NEW: tunnel 管理器
        tunnelMgr := tunnel.NewManager(ctx, srv.Port())

        // NEW: 自重启器
        restarter := NewSelfRestarter(a)

        // 绑定到 dispatcher
        dispatcher.Bind(updaterMgr, tunnelMgr, restarter)

        select {
        case <-updaterMgr.RestartCh:
            restarter.Restart()
        case <-tunnelMgr.ReconnectCh:
            tunnelMgr.Reconnect()
        case <-shutdown:
            // 现有 graceful shutdown 逻辑
        }
    } else {
        <-shutdown
        // ... 现有 shutdown ...
    }
}
```

---

## 5. 完整升级流程（端到端）

```
1. Cloud 调用 POST /api/v1/commands
   {type: "upgrade", command_id: "cmd-001"}
        │
        │ (via tunnel HTTP request)
        ▼
2. localserver CommandHandler 接收请求
        │
        ▼
3. CommandDispatcher.Dispatch()
   ├─ 检查 command_id 去重 ✓
   ├─ 检查无正在执行的命令 ✓
   ├─ 返回 {status: "accepted"}
   └─ 异步启动升级流程
        │
        ▼
4. UpdaterManager.Apply()
   ├─ Checker.Check() → 查询/使用 payload 中的 download_url
   ├─ Downloader.Download() → 下载新二进制到临时目录
   ├─ Verifier.Verify() → SHA256 校验
   ├─ Replacer.Replace() → 原子替换 + 备份 + 记录 pending_verify
   └─ signal RestartCh
        │
        ▼
5. Daemon 收到 RestartCh
   ├─ SaveState("restarting")
   ├─ Reporter.Report("restarting") → 告知云端即将重启
   ├─ 启动新 _daemon 子进程
   └─ 当前进程 os.Exit(0)
        │
        ▼
6. 新 Daemon 启动
   ├─ 检测 state == "restarting" → 加载已保存参数
   ├─ WritePID(当前 PID) → 覆盖 PID 文件
   ├─ SaveState("running")
   ├─ UpdaterManager.verifyOnStartup()
   │   ├─ 版本匹配 → MarkVerified() → AppendHistory()
   │   └─ 版本不匹配 → Rollback() → 再次重启
   ├─ tunnel.Connect() → 自动重连云端
   └─ Heartbeat → 上报新版本
        │
        ▼
7. Cloud 收到心跳
   ├─ 版本号已更新 → 确认升级成功
   └─ 回报命令结果 POST /api/devices/{id}/commands/cmd-001/result
```

**总耗时估算**：下载（取决于网络）+ 校验（<1s）+ 替换（<1s）+ 重启（~5s）+ 重连（~3s）= 约 10-60s

---

## 6. 安全设计

### 6.1 命令鉴权

tunnel WebSocket 连接已通过 `device_token` 鉴权（`connect.go:58`）。通过 tunnel 发送的 HTTP 请求被视为已认证。

额外安全层：
- 命令请求需携带 `X-Command-Token` header（云端签发的短期 token）
- 设备校验 token 的签名和有效期
- 防止 replay 攻击：每个 `command_id` 只能执行一次

### 6.2 并发保护

```go
type CommandDispatcher struct {
    mu       sync.Mutex
    active   *CommandStatus   // 当前最多一个活跃命令
}
```

- 同时只允许一个命令执行
- 新命令到来时，如果已有命令在执行，返回 `rejected`
- 升级过程中的 `Manager.mu` 提供第二层保护

### 6.3 超时与取消

- 命令执行有全局超时（默认 10 分钟）
- 升级下载超时（默认 10 分钟，已有）
- 重启超时（新进程 30s 内未上报 → 标记失败）

### 6.4 审计日志

所有命令执行记录到日志和本地文件：

| 事件 | 级别 |
|------|------|
| `command_received` | Info |
| `command_accepted` | Info |
| `command_rejected` | Warn |
| `command_executing` | Info |
| `command_completed` | Info |
| `command_failed` | Error |
| `upgrade_download_start` | Info |
| `upgrade_replace_complete` | Warn |
| `self_restart` | Warn |
| `rollback_triggered` | Error |

---

## 7. Tunnel 断开时的 Fallback

**问题**：tunnel 断开时，云端无法通过 tunnel 下发命令。

**方案：轮询 fallback**

在心跳循环中，每次心跳响应可携带待执行命令：

```
POST /api/devices/{deviceID}/heartbeat
响应：
{
  "pending_commands": [
    {
      "command_id": "cmd-002",
      "type": "restart",
      "payload": {...}
    }
  ]
}
```

设备收到 `pending_commands` 后，逐一执行。这样即使 tunnel 断开，只要设备能与云端 HTTP 通信，就能接收命令。

优先级：**tunnel 命令 > 心跳轮询命令**。若两者同时存在，以 `command_id` 去重。

---

## 8. 新增文件结构

```
internal/
├── app/
│   └── selfrestart.go              # 自重启逻辑
├── localserver/
│   ├── command_handler.go          # 命令 HTTP handler
│   ├── command_dispatcher.go       # 命令调度器
│   └── command_reporter.go         # 命令结果回报
├── tunnel/
│   └── manager.go                  # tunnel 生命周期管理
├── device/
│   └── heartbeat.go                # 心跳循环（从 token.go 拆出）
```

修改文件：

| 文件 | 修改内容 |
|------|---------|
| `internal/cli/daemon.go` | 接入 updater、heartbeat、command dispatcher、restart 监听 |
| `internal/localserver/server.go` | 注册 `/commands` 路由 |
| `internal/device/token.go` | Heartbeat 增加返回 pending_commands |

---

## 9. 实施路线

### Phase 6: 自重启与生命周期改造（2-3 天）

- 实现 `SelfRestart()` — re-exec + PID 交接
- 改造 `runDaemon()` 接入 updater + heartbeat + restart 监听
- 端到端验证：升级 → 自重启 → 自动重连

### Phase 7: 命令通道（2-3 天）

- 实现命令 API 端点
- 实现 CommandDispatcher
- 实现 CommandReporter
- 并发保护与去重

### Phase 8: 心跳与 Fallback（1-2 天）

- 启动心跳循环
- 心跳响应携带 pending_commands
- Tunnel 断开场景测试

### Phase 9: 集成测试（2-3 天）

- 全流程端到端测试
- 失败场景：下载中断、校验失败、重启失败、回滚
- 并发命令测试
- 各平台测试（Windows 重点）

---

## 10. 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| 重启后新进程无法启动 | verifyOnStartup + 自动回滚 + 保留 .bak |
| Windows exe 被锁无法替换 | 已有 .old 方案 + 重启后替换 |
| 升级过程中 tunnel 断开 | 状态持久化到 current.json，重启后恢复 |
| 新版本有 bug 导致服务不可用 | 回滚机制 + 云端可下发 rollback 命令 |
| 心跳超时误判离线 | 3 次超时才标记 + 指数退避重试 |
| 命令被重复执行 | command_id 去重 + 互斥锁 |
