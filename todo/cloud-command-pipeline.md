# Phase 6-9 - 云端指令下发与自升级/自重启/自重连

> 技术提案：[docs/cloud-command-pipeline.md](../docs/cloud-command-pipeline.md)

## 总览

| Phase | 名称 | 预计耗时 | 状态 | 依赖 |
|-------|------|---------|------|------|
| 6 | 自重启与生命周期改造 | 2-3 天 | `pending` | Phase 1-5 完成 |
| 7 | 命令通道 | 2-3 天 | `pending` | Phase 6 |
| 8 | 心跳与 Fallback | 1-2 天 | `pending` | Phase 7 |
| 9 | 集成测试 | 2-3 天 | `pending` | Phase 8 |

---

## Phase 6 - 自重启与生命周期改造

**前置条件**：Phase 1-5 完成（updater 包可用，version 包可用）
**目标**：daemon 具备自升级 + 自重启 + 自动重连能力

### 6.1 实现自重启 (`internal/app/selfrestart.go`)

- [ ] 实现 `SelfRestart(a *App) error` — re-exec 当前二进制
- [ ] 从 `a.LoadArgs()` 读取启动参数（mode、workspace 等）
- [ ] 平台差异：Windows `CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW`，Unix `setsid`
- [ ] 保存 `state = "restarting"` 到 state 文件
- [ ] 新进程启动后覆盖 PID 文件

### 6.2 改造 daemon 生命周期 (`internal/cli/daemon.go`)

- [ ] 接入 `updater.Manager.Run()` — 定时检查升级（每 6 小时）
- [ ] 消费 `updaterMgr.RestartCh` — 升级完成后触发自重启
- [ ] 启动心跳 goroutine — 调用 `device.Client.Heartbeat()`（5 分钟间隔）
- [ ] 检测 `state == "restarting"` — 跳过注册流程，直接复用 `device.json`
- [ ] 新增 `restartCh` 通道 — 统一重启信号入口

### 6.3 启动验证增强

- [ ] 新进程启动时检测 `state == "restarting"`
- [ ] 调用 `updaterMgr.verifyOnStartup()` — 确认升级版本
- [ ] 验证失败 → 自动回滚 + 再次重启
- [ ] 验证成功 → 清理备份 + 记录历史

### 6.4 Tunnel 管理器 (`internal/tunnel/manager.go`)

- [ ] 定义 `Manager` 结构体，持有 `context.CancelFunc`
- [ ] 实现 `Reconnect()` — 取消当前 tunnel context，触发重连
- [ ] 集成到 `runDaemon()` 中

### 验收标准

- [ ] daemon 启动时自动开始定时升级检查
- [ ] 升级完成后 daemon 自动重启
- [ ] 新进程启动后 tunnel 自动重连
- [ ] 心跳每 5 分钟上报一次
- [ ] 版本不匹配时自动回滚

### 产出文件

| 文件 | 操作 |
|------|------|
| `internal/app/selfrestart.go` | 新增 |
| `internal/tunnel/manager.go` | 新增 |
| `internal/cli/daemon.go` | 修改 |
| `internal/device/heartbeat.go` | 新增（从 token.go 拆出心跳循环） |

---

## Phase 7 - 命令通道

**前置条件**：Phase 6 完成
**目标**：云端可通过 tunnel 向设备下发控制指令

### 7.1 命令数据结构

- [ ] 定义 `Command` 结构体（command_id、type、payload、timestamp）
- [ ] 定义 `CommandAck` 结构体（command_id、status、message）
- [ ] 定义 `CommandStatus` 结构体（command_id、type、status、started_at、completed_at、result）

### 7.2 命令 HTTP Handler (`internal/localserver/command_handler.go`)

- [ ] `POST /api/v1/commands` — 接收云端命令
- [ ] `GET /api/v1/commands/status?command_id=` — 查询命令执行状态
- [ ] 请求鉴权：校验 `X-Command-Token` header
- [ ] 参数校验：command_id 非空、type 合法

### 7.3 命令调度器 (`internal/localserver/command_dispatcher.go`)

- [ ] 实现 `Dispatch(cmd *Command) (*CommandAck, error)`
- [ ] 命令去重：`command_id` 不能重复执行
- [ ] 并发保护：同时只允许一个命令执行（`sync.Mutex`）
- [ ] 命令路由：
  - `upgrade` → `updaterMgr.Apply()`
  - `restart` → `restarter.Restart()`
  - `reconnect` → `tunnelMgr.Reconnect()`
- [ ] 异步执行：命令在 goroutine 中执行
- [ ] 状态跟踪：记录每个命令的执行状态

### 7.4 命令结果回报 (`internal/localserver/command_reporter.go`)

- [ ] 实现 `Reporter.Report(commandID, status, result, error)`
- [ ] 调用 `POST {cloudBaseURL}/api/devices/{deviceID}/commands/{commandID}/result`
- [ ] 携带 `device_token` 鉴权
- [ ] 回报失败时重试（最多 3 次）

### 7.5 注册路由 (`internal/localserver/server.go`)

- [ ] 注册 `POST /api/v1/commands` 路由
- [ ] 注册 `GET /api/v1/commands/status` 路由
- [ ] 注入 `CommandDispatcher` 到 Server 结构体

### 验收标准

- [ ] 云端可通过 tunnel 发送 upgrade 命令，设备执行升级
- [ ] 云端可通过 tunnel 发送 restart 命令，设备执行重启
- [ ] 云端可通过 tunnel 发送 reconnect 命令，设备执行重连
- [ ] 重复 command_id 被拒绝
- [ ] 命令执行中时新命令被拒绝
- [ ] 命令结果成功回报给云端

### 产出文件

| 文件 | 操作 |
|------|------|
| `internal/localserver/command_handler.go` | 新增 |
| `internal/localserver/command_dispatcher.go` | 新增 |
| `internal/localserver/command_reporter.go` | 新增 |
| `internal/localserver/server.go` | 修改（注册路由 + 注入 dispatcher） |

---

## Phase 8 - 心跳与 Fallback

**前置条件**：Phase 7 完成
**目标**：设备在线状态感知 + tunnel 断开时命令可达

### 8.1 心跳增强

- [ ] `Client.Heartbeat()` 返回响应体（包含 `pending_commands`）
- [ ] 心跳循环解析响应中的 `pending_commands`
- [ ] 收到 pending_commands 时交给 `CommandDispatcher` 执行
- [ ] 心跳失败时指数退避重试（1s → 60s）

### 8.2 双通道命令去重

- [ ] tunnel 命令和心跳轮询命令共享同一个 `active` map
- [ ] `command_id` 全局去重，先到先执行
- [ ] 防止同一条命令被执行两次

### 8.3 设备状态上报

- [ ] 心跳中携带设备状态信息（version、uptime、last_upgrade）
- [ ] 云端可据此做健康评分

### 验收标准

- [ ] 心跳每 5 分钟上报，云端可感知设备在线
- [ ] Tunnel 断开时，心跳仍能触发云端命令
- [ ] 双通道命令不重复执行
- [ ] 心跳失败时自动退避重试

### 产出文件

| 文件 | 操作 |
|------|------|
| `internal/device/heartbeat.go` | 修改（解析 pending_commands） |
| `internal/device/token.go` | 修改（Heartbeat 返回响应） |
| `internal/localserver/command_dispatcher.go` | 修改（双通道去重） |

---

## Phase 9 - 集成测试

**前置条件**：Phase 8 完成
**目标**：全流程端到端验证

### 9.1 升级流程测试

- [ ] 正常升级：cloud → upgrade → download → replace → restart → reconnect → verify
- [ ] 强制升级：force=true 时无论策略都执行
- [ ] 指定版本升级：payload 包含 version 时的行为
- [ ] 升级回滚：版本不匹配时自动回滚

### 9.2 失败场景测试

- [ ] 下载中断（网络超时）
- [ ] SHA256 校验失败
- [ ] 二进制替换失败（权限不足）
- [ ] 新进程启动失败
- [ ] 回滚失败

### 9.3 重启与重连测试

- [ ] restart 命令：graceful shutdown + 新进程启动 + tunnel 重连
- [ ] reconnect 命令：tunnel 断开后自动重连
- [ ] 连续 restart：多次重启不丢失状态

### 9.4 并发与边界测试

- [ ] 同时发送 upgrade + restart（后者被拒绝）
- [ ] 重复 command_id（被拒绝）
- [ ] Tunnel 断开时通过心跳下发命令
- [ ] 心跳连续失败后的退避行为

### 9.5 平台测试

- [ ] Windows：exe 替换 + 进程创建 + PID 交接
- [ ] Linux：原子 rename + 信号处理
- [ ] macOS：同 Linux

### 验收标准

- [ ] 全部升级/回滚/重启/重连场景通过
- [ ] 全平台（Windows / Linux / macOS）端到端通过
- [ ] 无命令丢失或重复执行
- [ ] 失败场景均有正确处理和日志
