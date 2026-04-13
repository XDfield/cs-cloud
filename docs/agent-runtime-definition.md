# Agent Runtime Definition

## Overview

cs-cloud 内置一组预定义的 agent runtime，每个 runtime 描述一种 AI agent CLI 的接入方式。启动时通过 CLI 可用性检测自动启用已安装的 agent，同时支持用户通过配置文件关闭内置 agent 或添加自定义 agent。

> **AionUi 参考映射**
>
> | AionUi 模块 | cs-cloud 对应 |
> |---|---|
> | `AcpDetector.ts` | `agent/detector.go` |
> | `acpConnectors.ts` | `agent/connector.go` + 各 `adapter_*.go` |
> | `AcpConnection.ts` | `acp/connection.go` |
> | `AcpAgent` (index.ts) | `acp/agent.go` |
> | `AcpAdapter.ts` | `acp/adapter.go` |
> | `ApprovalStore.ts` | `acp/approval.go` |
> | `modelInfo.ts` | `acp/model.go` |
> | `mcpSessionConfig.ts` | `acp/mcp.go` |
> | `AgentFactory.ts` | `runtime/registry.go` |
> | `WorkerTaskManager.ts` | `runtime/manager.go` |
> | `BaseAgentManager.ts` | `agent/agent.go` (interface) |
> | `RemoteAgentCore.ts` | 未来 `ws/driver.go` |

---

## AgentRuntime 定义

```go
// internal/agent/runtime.go
package agent

type Runtime struct {
    ID           string            // 唯一标识，如 "claude", "qwen"
    Name         string            // 显示名，如 "Claude Code"
    Driver       string            // 驱动类型: "acp" | "http"
    CLICommand    string            // 主命令，如 "claude"
    FallbackCLI   string            // 备用命令，如 "npx @zed-industries/claude-agent-acp@0.21.0"
    ACPArgs      []string          // ACP 启动参数，如 ["--experimental-acp"]
    AuthRequired bool              // 是否需要认证
    YOLOMode     string            // YOLO 模式标识，如 "bypassPermissions"
    Env          map[string]string // 额外环境变量

    // 运行时状态（检测后填充）
    Available    bool              // CLI 是否可用
    ResolvedPath string            // 解析到的 CLI 绝对路径
}
```

---

## 内置 Agent Runtime 列表

### ACP Driver Agents

| ID | Name | CLICommand | FallbackCLI | ACPArgs | AuthRequired | YOLOMode |
|---|---|---|---|---|---|---|
| `claude` | Claude Code | `claude` | `npx @zed-industries/claude-agent-acp@0.21.0` | `--experimental-acp` | yes | `bypassPermissions` |
| `qwen` | Qwen Code | `qwen` | `npx @qwen-code/qwen-code` | `--acp` | yes | `yolo` |
| `codex` | Codex | `codex` | `npx @zed-industries/codex-acp@0.9.5` | — | yes | — |
| `goose` | Goose | `goose` | — | `acp` | no | — |
| `auggie` | Augment Code | `auggie` | — | `--acp` | no | — |
| `opencode` | OpenCode | `opencode` | — | `acp` | no | — |
| `copilot` | GitHub Copilot | `copilot` | — | `--acp`, `--stdio` | no | — |
| `droid` | Factory Droid | `droid` | — | `exec`, `--output-format`, `acp` | no | — |
| `kimi` | Kimi CLI | `kimi` | — | `acp` | no | — |
| `cursor` | Cursor Agent | `agent` | — | `acp` | yes | — |
| `kiro` | Kiro | `kiro-cli` | — | `acp` | yes | — |
| `codebuddy` | CodeBuddy | `codebuddy` | `npx @tencent-ai/codebuddy-code` | `--acp` | yes | `bypassPermissions` |

### HTTP Driver Agents

| ID | Name | BaseURL | AuthRequired | 备注 |
|---|---|---|---|---|
| — | — | — | — | 当前无内置 HTTP agent，预留扩展 |

---

## CLI 可用性检测

### 检测逻辑

```go
// internal/agent/detector.go
package agent

type Detector struct {
    runtimes []*Runtime
}

func NewDetector(builtIn []*Runtime, userCustom []*Runtime) *Detector

func (d *Detector) Detect(ctx context.Context) []RuntimeStatus

type RuntimeStatus struct {
    ID           string `json:"id"`
    Name         string `json:"name"`
    Driver       string `json:"driver"`
    Available    bool   `json:"available"`
    ResolvedPath string `json:"resolved_path,omitempty"`
    FallbackUsed bool   `json:"fallback_used,omitempty"`
}
```

### 检测策略

```
对于每个 Runtime:
  1. 尝试 Lookup(CLICommand)
     → 找到: Available=true, ResolvedPath=绝对路径, FallbackUsed=false
  2. 未找到且有 FallbackCLI:
     → 解析 FallbackCLI: 如果以 "npx " 开头，检查 npx 是否可用
     → 找到: Available=true, ResolvedPath=FallbackCLI, FallbackUsed=true
  3. 均未找到: Available=false
```

### 平台差异

| 平台 | 查找命令 | 特殊处理 |
|---|---|---|
| Linux/macOS | `exec.LookPath(cmd)` | 无 |
| Windows | `exec.LookPath(cmd)` | 额外检查 `.cmd`、`.ps1` 后缀 |
| 全平台 | npx fallback | 检查 `npx` 是否在 PATH 中 |

### 增强路径 (Enhanced PATH)

某些 agent CLI 安装路径不在默认 PATH 中（如 nvm、homebrew）。检测前应从用户 shell 环境增强 PATH：

```go
func EnhancedEnv() []string {
    // 1. 继承当前 process env
    // 2. 在 macOS 上从 login shell 获取完整 PATH
    //    cmd: env -i $SHELL -l -c "echo $PATH"
    // 3. 合并去重
}
```

---

## Adapter 层

每个 agent runtime 可注册一个 `Adapter`，用于处理该 agent 的定制化逻辑。未注册 adapter 的 agent 使用默认行为。

### Adapter 接口

```go
// internal/agent/adapter.go
package agent

type Adapter interface {
    // ID 标识此 adapter 服务的 runtime
    ID() string

    // SpawnEnv 返回启动该 agent 时需要注入的环境变量
    SpawnEnv(resolvedPath string) map[string]string

    // SpawnArgs 返回启动参数（可覆盖 Runtime.ACPArgs）
    SpawnArgs(resolvedPath string) []string

    // OnConnected 在 ACP 连接建立后的回调（如发送初始化配置）
    OnConnected(ctx context.Context, conn ACPConn) error

    // TranslateEvent 将 agent 原始事件转换为统一事件格式
    // 返回 nil 表示使用默认转换
    TranslateEvent(raw map[string]any) *Event

    // TranslatePermission 将 agent 原始权限请求转换为统一格式
    // 返回 nil 表示使用默认转换
    TranslatePermission(raw map[string]any) *PermissionInfo

    // ConfigOptions 返回该 agent 支持的配置选项
    ConfigOptions() []ConfigOption

    // OnConfigChange 当用户修改配置时的回调
    OnConfigChange(optionID string, value string) error
}
```

### 内置 Adapter 示例

#### Claude Adapter

```go
// internal/agent/adapter_claude.go
package agent

type ClaudeAdapter struct{}

func (a *ClaudeAdapter) ID() string { return "claude" }

func (a *ClaudeAdapter) SpawnEnv(path string) map[string]string {
    // Claude 需要 clean env，移除 NODE_OPTIONS、npm_* 等
    return map[string]string{
        "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    }
}

func (a *ClaudeAdapter) SpawnArgs(path string) []string {
    return []string{"--experimental-acp"}
}

func (a *ClaudeAdapter) OnConnected(ctx context.Context, conn ACPConn) error {
    // Claude 需要在 session/new 的 _meta 中传递 resume 信息
    return nil
}

func (a *ClaudeAdapter) TranslateEvent(raw map[string]any) *Event {
    // Claude 的 agent_message_chunk 使用 _meta.claudeCode 扩展字段
    // 需要提取 msg_id 用于消息累积
    return nil // 使用默认转换，adapter 只处理特殊情况
}

func (a *ClaudeAdapter) ConfigOptions() []ConfigOption {
    return []ConfigOption{
        {ID: "bypassPermissions", Name: "YOLO Mode", Type: "boolean"},
    }
}
```

#### Goose Adapter

```go
// internal/agent/adapter_goose.go
package agent

type GooseAdapter struct{}

func (a *GooseAdapter) ID() string { return "goose" }

func (a *GooseAdapter) SpawnEnv(path string) map[string]string {
    env := map[string]string{}
    // Goose 使用环境变量控制 YOLO 模式，而非 ACP 参数
    return env
}

func (a *GooseAdapter) SpawnArgs(path string) []string {
    return []string{"acp"} // subcommand，不是 flag
}

func (a *GooseAdapter) ConfigOptions() []ConfigOption {
    return []ConfigOption{
        {ID: "goose_mode", Name: "Goose Mode", Type: "select",
            Options: []string{"auto", "smart", "normal"}},
    }
}

func (a *GooseAdapter) OnConfigChange(optionID string, value string) error {
    if optionID == "goose_mode" {
        // 设置 GOOSE_MODE 环境变量
    }
    return nil
}
```

#### CodeBuddy Adapter

```go
// internal/agent/adapter_codebuddy.go
package agent

type CodeBuddyAdapter struct{}

func (a *CodeBuddyAdapter) ID() string { return "codebuddy" }

func (a *CodeBuddyAdapter) SpawnEnv(path string) map[string]string {
    // CodeBuddy 需要 detached 模式（/dev/tty 访问）
    return nil
}

func (a *CodeBuddyAdapter) SpawnArgs(path string) []string {
    return []string{"--acp"}
}
```

### 默认 Adapter

未注册 adapter 的 agent 使用 `DefaultAdapter`，行为：

- `SpawnEnv` → 清理 `NODE_OPTIONS`、`npm_*` 等干扰变量
- `SpawnArgs` → 直接使用 `Runtime.ACPArgs`
- `OnConnected` → 无操作
- `TranslateEvent` / `TranslatePermission` → 使用 ACP 标准格式转换
- `ConfigOptions` → 空

---

## 用户配置文件

路径: `~/.cs-cloud/agents.toml`

### 配置格式

```toml
# 关闭内置 agent
[disable]
claude = true      # 禁用 Claude Code
droid = true       # 禁用 Factory Droid

# 自定义 agent（仅支持 ACP 协议）
[[custom]]
id = "my-agent"
name = "My Custom Agent"
cli_command = "my-agent-cli"
fallback_cli = "npx @my-org/my-agent-acp"
acp_args = ["--acp", "--stdio"]
auth_required = false
yolo_mode = "yolo"

[[custom]]
id = "internal-tool"
name = "Internal Tool"
cli_command = "/opt/tools/ai-agent"
acp_args = ["acp"]
auth_required = true
```

### 配置加载逻辑

```go
// internal/agent/config.go
package agent

type UserConfig struct {
    Disable map[string]bool  `toml:"disable"`
    Custom  []CustomAgentDef `toml:"custom"`
}

type CustomAgentDef struct {
    ID           string   `toml:"id"`
    Name         string   `toml:"name"`
    CLICommand    string   `toml:"cli_command"`
    FallbackCLI   string   `toml:"fallback_cli,omitempty"`
    ACPArgs      []string `toml:"acp_args"`
    AuthRequired bool     `toml:"auth_required"`
    YOLOMode     string   `toml:"yolo_mode,omitempty"`
}

func LoadUserConfig(path string) (*UserConfig, error)
```

### 合并流程

```
1. 加载 BuiltInRuntimes（全部内置 agent）
2. 加载 UserConfig
3. 过滤: 移除 UserConfig.Disable 中标记为 true 的 runtime
4. 合并: 将 UserConfig.Custom 转为 Runtime 并追加
5. 执行 Detect: 检测所有保留 runtime 的 CLI 可用性
6. 注册 Adapter: 为有内置 adapter 的 runtime 注册，其余使用 DefaultAdapter
```

---

## Runtime Registry

### 注册表结构

```go
// internal/agent/registry.go
package agent

type Registry struct {
    mu       sync.RWMutex
    runtimes map[string]*Runtime   // id → Runtime
    adapters map[string]Adapter    // id → Adapter
    statuses map[string]RuntimeStatus // id → 检测状态
}

func NewRegistry() *Registry

// 注册内置 runtimes + adapters
func (r *Registry) RegisterBuiltIn()

// 加载用户配置，合并到 runtimes
func (r *Registry) ApplyUserConfig(cfg *UserConfig)

// 执行检测
func (r *Registry) Detect(ctx context.Context) error

// 查询
func (r *Registry) Get(id string) (*Runtime, bool)
func (r *Registry) GetAdapter(id string) (Adapter, bool)
func (r *Registry) ListAvailable() []RuntimeStatus
func (r *Registry) ListAll() []RuntimeStatus
```

### 初始化流程

```go
func InitRegistry(ctx context.Context) (*Registry, error) {
    reg := NewRegistry()
    reg.RegisterBuiltIn()

    cfg, err := LoadUserConfig(filepath.Join(platform.DataDir(), "agents.toml"))
    if err != nil {
        // 配置文件不存在或格式错误，使用默认
        cfg = &UserConfig{}
    }
    reg.ApplyUserConfig(cfg)

    if err := reg.Detect(ctx); err != nil {
        return nil, err
    }
    return reg, nil
}
```

---

## API 暴露

### GET /api/v1/agents

返回所有 agent runtime 的状态：

```json
{
  "ok": true,
  "data": {
    "agents": [
      {
        "id": "claude",
        "name": "Claude Code",
        "driver": "acp",
        "available": true,
        "resolved_path": "/usr/local/bin/claude",
        "fallback_used": false,
        "auth_required": true
      },
      {
        "id": "qwen",
        "name": "Qwen Code",
        "driver": "acp",
        "available": true,
        "resolved_path": "npx @qwen-code/qwen-code",
        "fallback_used": true,
        "auth_required": true
      },
      {
        "id": "goose",
        "name": "Goose",
        "driver": "acp",
        "available": false,
        "auth_required": false
      }
    ]
  }
}
```

### GET /api/v1/agents/:id/health

对指定 agent 执行健康检查（尝试 spawn + initialize + disconnect）：

```json
{
  "ok": true,
  "data": {
    "available": true,
    "latency_ms": 230,
    "version": "1.0.3"
  }
}
```

### GET /api/v1/agents/:id/config

返回该 agent 的配置选项：

```json
{
  "ok": true,
  "data": {
    "options": [
      {
        "id": "bypassPermissions",
        "name": "YOLO Mode",
        "type": "boolean",
        "current_value": "false"
      }
    ]
  }
}
```

---

## 文件结构

```
internal/agent/
├── runtime.go            # Runtime 类型定义
├── adapter.go            # Adapter 接口 + DefaultAdapter
├── adapter_claude.go     # Claude 定制适配
├── adapter_goose.go      # Goose 定制适配
├── adapter_codebuddy.go  # CodeBuddy 定制适配
├── detector.go          # CLI 可用性检测
├── config.go             # UserConfig 加载 (agents.toml)
├── registry.go           # Runtime 注册表
└── builtins.go           # BuiltInRuntimes 列表
```

---

## AionUi 参考实现要点

以下是从 AionUi TypeScript 实现中提炼的关键设计模式，需在 Go 移植中保留或适配。

### 1. JSON-RPC 消息循环 (`AcpConnection.ts`)

**核心机制**：
- `pendingRequests: Map<number, PendingRequest>` — 请求 ID → Promise resolver/rejecter
- 请求发送时注册超时定时器，收到响应后清除
- `session/prompt` 使用可配置超时（默认 300s），其他方法 60s
- **权限请求暂停机制**：收到 `session/request_permission` 时暂停所有 `session/prompt` 超时，用户响应后恢复
- **Keepalive 机制**：每 60s 检查子进程是否存活，若存活则重置超时定时器（防止长时间工具调用误超时）

**Go 移植建议**：
```go
type Connection struct {
    pending    map[int64]chan Result
    nextID     int64
    mu         sync.Mutex
    promptTTL  time.Duration  // 默认 5min
    methodTTL  time.Duration  // 默认 60s
}
// 使用 context.WithTimeout + time.AfterFunc 实现
// 权限暂停：context.WithValue 标记 isPaused，keepalive goroutine 检查并重置
```

### 2. 后端特定 Spawn 逻辑 (`acpConnectors.ts`)

**关键模式**：
- **npx 双阶段重试**：先 `--prefer-offline`，失败后清除缓存重试
- **环境清理**：移除 `NODE_OPTIONS`、`npm_*`、`CLAUDECODE` 等干扰变量
- **Node 版本检查**：`ensureMinNodeVersion()` 自动修正 PATH
- **Codex 多候选策略**：缓存二进制 → 平台包 → meta 包 fallback
- **CodeBuddy detached 模式**：非 Windows 上使用 `detached: true`（解决 `/dev/tty` 访问问题）
- **Windows UTF-8**：spawn 前执行 `chcp 65001`

**Go 移植建议**：每个后端的 spawn 逻辑封装为 `Connector` 接口：
```go
type Connector interface {
    Spawn(workDir string, env []string) (*exec.Cmd, error)
    IsDetached() bool
}
```
内置 `ClaudeConnector`、`CodexConnector`、`CodebuddyConnector`、`GenericConnector`。

### 3. 会话恢复策略 (`AcpConnection.ts:847-918`)

不同后端使用不同恢复方式：

| 后端 | 恢复方式 |
|---|---|
| Claude / CodeBuddy | `session/new` + `_meta.claudeCode.options.resume` |
| Codex | `session/load` + `sessionId` |
| 其他 | `session/new` + `resumeSessionId` 参数 |

**Go 移植建议**：在 `Adapter` 接口中添加 `ResumeMethod()`：
```go
type ResumeMethod int
const (
    ResumeClaude  ResumeMethod = iota
    ResumeCodex
    ResumeGeneric
)

func (a *ClaudeAdapter) ResumeMethod() ResumeMethod { return ResumeClaude }
func (a *CodexAdapter) ResumeMethod() ResumeMethod  { return ResumeCodex }
func (a *DefaultAdapter) ResumeMethod() ResumeMethod { return ResumeGeneric }
```

### 4. 权限自动审批 (`ApprovalStore.ts`)

**核心逻辑**：
- `serializeKey()` 只取操作标识字段（`command`、`path`、`file_path`），不包含描述文本
- 仅存储 `allow_always` 决策
- 会话级别缓存，`clear()` 在新会话时调用

**Go 移植建议**：
```go
type ApprovalKey struct {
    Command string
    Path    string
}

type ApprovalStore struct {
    mu    sync.RWMutex
    store map[ApprovalKey]string  // key → "allow_always"
}
```

### 5. 模型信息双源解析 (`modelInfo.ts`)

1. 优先从 `configOptions`（稳定 API）提取模型信息
2. 回退到 `models`（不稳定 API）
3. 统一返回 `ModelInfo{ currentModelID, availableModels, canSwitch, source }`

### 6. 流式消息缓冲 (`AcpAgentManager.ts:88-155`)

- `bufferedStreamTextMessages: Map<string, {content, timer}>`
- 收到 `agent_message_chunk` 时累积文本，每 120ms 刷新到 UI
- 避免高频 DB 写入

**Go 移植建议**：EventBus 中使用带缓冲的 channel + ticker：
```go
type StreamBuffer struct {
    mu            sync.Mutex
    buffers       map[string]*bufferEntry
    flushInterval time.Duration  // 120ms
}
```

### 7. 优雅关闭 (`AcpAgentManager.ts:1099-1131`)

```go
func (a *ACPAgent) Kill() error {
    if a.conn != nil {
        _ = a.conn.Disconnect()
    }
    select {
    case <-time.After(500 * time.Millisecond):
    case <-a.done:
    }
    if a.cmd.Process != nil {
        return a.cmd.Process.Kill()
    }
    return nil
}
```

### 8. 远程 Agent (WebSocket 驱动) (`RemoteAgentCore.ts`)

**与 ACP 的关键差异**：
- 传输层：WebSocket 而非 stdin/stdout
- 会话管理：`sessionsResolve`/`sessionsReset` 而非 ACP `session/new`
- 事件格式：`EventFrame` 对象（`chat`、`agent`、`exec.approval.request`）
- 权限处理：70s 超时自动拒绝
- **复用 AcpAdapter**：工具调用转换共用
- **复用 ApprovalStore**：自动审批逻辑共用

### 9. 增强路径检测 (`AcpDetector.ts`)

- `which`/`where` 查找主命令
- Windows 上 PowerShell `Get-Command` fallback
- `getEnhancedEnv()` 从 login shell 获取完整 PATH（解决 nvm、homebrew 等路径问题）

### 10. 错误分类与重试 (`AcpConnection.ts`)

- `AcpErrorType` 枚举：`CONNECTION`、`TIMEOUT`、`AUTH`、`PROTOCOL`、`PROCESS_EXIT`
- `retryable` 布尔标记：`CONNECTION` 和 `TIMEOUT` 可重试
- npx 缓存损坏检测：`_npx` + `ENOENT`/`ERR_MODULE_NOT_FOUND` → 清理缓存重试

```go
type AcpErrorType int
const (
    ErrConnection AcpErrorType = iota
    ErrTimeout
    ErrAuth
    ErrProtocol
    ErrProcessExit
)

type AcpError struct {
    Type      AcpErrorType
    Message   string
    Retryable bool
}
```
