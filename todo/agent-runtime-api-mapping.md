# Agent Runtime 接口对接 TODO

> 基于 `device-client.ts` 三层接口映射，对照 cs-cloud 现状

## 设计原则

**conversation / interaction 接口直接透传 opencode，cs-cloud 不做中间状态管理。**

- sessionID 即 conversation ID，不做映射
- cs-cloud 的职责：spawn 进程 → 拿到 endpoint → 反向代理 HTTP/SSE 到 opencode
- 不在 AgentManager 中维护 conversation 列表、状态等——全部由 opencode 管理

## 已完成

### 核心框架
- [x] `agent.Agent` / `agent.Driver` 接口定义 (`internal/agent/agent.go`, `driver.go`)
- [x] OpenCode HTTP Agent — spawn `cs serve` + stdout 解析端口 (`internal/agent/opencode_agent.go`)
- [x] OpenCode Driver — CLI 检测 + Agent 工厂 (`internal/agent/driver_opencode.go`)
- [x] AgentManager + EventBus 骨架 (`internal/runtime/`)
- [x] serve.go 集成

### runtime 接口（cs-cloud 自有）
- [x] `GET /api/v1/runtime/health` — 健康检查
- [x] `GET /api/v1/runtime/files?path=` — 文件列表
- [x] `GET /api/v1/runtime/files/content?path=` — 文件内容

### agent 接口
- [x] `GET /api/v1/agents` — 检测 CLI 可用性

---

## 需要重构

以下已完成接口需要改为直接透传 opencode，移除 cs-cloud 中间状态：

| 现状 | 问题 | 改为 |
|---|---|---|
| `POST /conversations` 由 cs-cloud 生成 convID + AgentManager 管理状态 | 多了一层映射 | 直接 proxy → `POST /session/`，返回 opencode 的 session |
| `GET /conversations` 从 AgentManager 读取本地列表 | 数据不完整 | 直接 proxy → `GET /session/` |
| `GET /conversations/{id}` 返回本地元信息 | 缺消息内容等 | 直接 proxy → `GET /session/{id}` |
| `DELETE /conversations/{id}` kill 子进程 | 不是删除 session | 直接 proxy → `DELETE /session/{id}` |
| `POST /conversations/{id}/prompt` 经过 Agent.SendMessage() | 不必要的抽象 | 直接 proxy → `POST /session/{id}/prompt_async` |
| `POST /conversations/{id}/abort` 经过 Agent.CancelPrompt() | 同上 | 直接 proxy → `POST /session/{id}/abort` |

---

## 待实现（含 ACP 兼容性分析）

> ACP 协议是 stdio JSON-RPC 2.0，只定义了核心交互方法。opencode serve 在 HTTP 层面提供了大量 ACP 未覆盖的接口（session CRUD、消息历史、diff、todo、share 等）。cs-cloud 需要透传这些 opencode 独有接口。

### conversation 透传

| # | cs-cloud 路由 | → opencode 路由 | ACP 对应 | 说明 |
|---|---|---|---|---|
| 1 | `POST /api/v1/conversations` | `POST /session` | `session/new` | ✅ ACP 支持 |
| 2 | `GET /api/v1/conversations` | `GET /session` | — | ❌ ACP 无列表查询，opencode 独有 |
| 3 | `GET /api/v1/conversations/{id}` | `GET /session/{id}` | — | ❌ ACP 无单会话查询，opencode 独有 |
| 4 | `GET /api/v1/conversations/{id}/messages` | `GET /session/{id}/message` | — | ❌ ACP 无消息历史，opencode 独有 |
| 5 | `POST /api/v1/conversations/{id}/prompt` | `POST /session/{id}/message` | `session/prompt` | ✅ ACP 支持（流式 chunk） |
| 6 | `POST /api/v1/conversations/{id}/prompt/async` | `POST /session/{id}/prompt_async` | — | ❌ ACP 只有同步 prompt，opencode 独有 |
| 7 | `PATCH /api/v1/conversations/{id}` | `PATCH /session/{id}` | `session/set_mode` / `session/set_model` | ⚠️ 部分：mode/model 走 ACP，标题更新走 opencode |
| 8 | `DELETE /api/v1/conversations/{id}` | `DELETE /session/{id}` | — | ❌ ACP 无删除，opencode 独有 |
| 9 | `POST /api/v1/conversations/{id}/abort` | `POST /session/{id}/abort` | `session/cancel` | ✅ ACP 支持 |
| 10 | `GET /api/v1/conversations/{id}/diff` | `GET /session/{id}/diff` | — | ❌ ACP 无 diff，opencode 独有 |
| 11 | `GET /api/v1/conversations/{id}/todo` | `GET /session/{id}/todo` | — | ❌ ACP 无 todo，opencode 独有 |
| 12 | `POST /api/v1/conversations/{id}/revert` | `POST /session/{id}/revert` | — | ❌ ACP 无 revert，opencode 独有 |
| 13 | `POST /api/v1/conversations/{id}/unrevert` | `POST /session/{id}/unrevert` | — | ❌ ACP 无 unrevert，opencode 独有 |
| 14 | `POST /api/v1/conversations/{id}/summarize` | `POST /session/{id}/summarize` | — | ❌ ACP 无 summarize，opencode 独有 |
| 15 | `POST /api/v1/conversations/{id}/shell` | `POST /session/{id}/shell` | — | ❌ ACP 无 shell，opencode 独有 |
| 16 | `POST /api/v1/conversations/{id}/command` | `POST /session/{id}/command` | — | ❌ ACP 无 slash command，opencode 独有 |
| 17 | `GET /api/v1/conversations/status` | `GET /session/status` | — | ❌ ACP 无状态查询，opencode 独有 |
| 18 | `POST /api/v1/conversations/{id}/share` | `POST /session/{id}/share` | — | ❌ ACP 无 share，opencode 独有 |
| 19 | `DELETE /api/v1/conversations/{id}/share` | `DELETE /session/{id}/share` | — | ❌ ACP 无 unshare，opencode 独有 |
| 20 | `POST /api/v1/conversations/{id}/fork` | `POST /session/{id}/fork` | — | ❌ ACP 无 fork，opencode 独有 |
| 21 | `POST /api/v1/conversations/{id}/init` | `POST /session/{id}/init` | — | ❌ ACP 无 init，opencode 独有 |

### SSE 事件透传

| # | cs-cloud 路由 | → opencode 路由 | ACP 对应 | 说明 |
|---|---|---|---|---|
| 22 | `GET /api/v1/events` (SSE) | `GET /event` (SSE) | `session/update` notifications | ✅ ACP 通过 JSON-RPC notification 推送，opencode 转为 SSE |

### interaction 透传

| # | cs-cloud 路由 | → opencode 路由 | ACP 对应 | 说明 |
|---|---|---|---|---|
| 23 | `GET /api/v1/permissions` | `GET /permission` | — | ❌ ACP 权限通过 `session/request_permission` 推送，无列表查询 |
| 24 | `POST /api/v1/permissions/{id}/reply` | `POST /permission/{id}/reply` | `session/request_permission` 响应 | ⚠️ ACP 是 JSON-RPC request 的 response，非独立 REST |
| 25 | `GET /api/v1/questions` | `GET /question` | — | ❌ ACP 无 question，opencode 独有 |
| 26 | `POST /api/v1/questions/{id}/reply` | `POST /question/{id}/reply` | — | ❌ ACP 无 question，opencode 独有 |
| 27 | `POST /api/v1/questions/{id}/reject` | `POST /question/{id}/reject` | — | ❌ ACP 无 question，opencode 独有 |

---

## 架构调整

### 需移除
- AgentManager 中的 conversation 状态管理（agents map、GetAgent、ListAgents）
- Agent 接口中的 SendMessage / CancelPrompt（改为直接 proxy）
- cs-cloud 自生成的 convID

### 需保留
- spawn 进程管理（spawn `cs serve` → 解析端口 → 拿到 endpoint）
- 进程生命周期（kill 子进程 = 销毁 agent runtime 实例）
- EventBus（SSE 转发仍需要）

### 新增：反向代理层
cs-cloud 需要 a generic HTTP reverse proxy，将 `/api/v1/conversations/**`、`/api/v1/permissions/**` 等路由直接转发到 opencode 的 endpoint，同时做路由路径重写：

```
/api/v1/conversations/{id}/messages  →  /session/{id}/message
/api/v1/conversations/{id}/prompt    →  /session/{id}/message
/api/v1/permissions/{id}/reply       →  /permission/{id}/reply
```

无需逐个实现 handler，用路由表 + httputil.ReverseProxy 即可。

---

## ACP 兼容性总结

### ACP 协议覆盖范围（仅 5 个核心方法）

| ACP 方法 | 对应 cs-cloud 接口 |
|---|---|
| `session/new` | `POST /conversations` |
| `session/prompt` | `POST /conversations/{id}/prompt` |
| `session/cancel` | `POST /conversations/{id}/abort` |
| `session/set_mode` | `PATCH /conversations/{id}`（部分） |
| `session/set_model` | `PATCH /conversations/{id}`（部分） |

加上 `initialize` / `authenticate` 的连接管理，和 `session/update` / `session/request_permission` 的通知推送。

### ACP 不覆盖、必须透传 opencode 的接口

| 类别 | 接口数量 | 关键缺失 |
|---|---|---|
| conversation 查询 | 17 个 | 列表、详情、消息历史、diff、todo、share、fork、revert、summarize、shell、command、prompt_async、status、init |
| interaction | 4 个 | question（列表/回复/拒绝）、permission 列表 |
| runtime | 12 个 | path、command、find、mcp状态、lsp、vcs、pty（CRUD）、instance dispose、commands |

**结论**：ACP 协议只覆盖了 ~12% 的接口。cs-cloud 的 opencode driver 必须以 HTTP 反向代理为主，ACP 方法仅用于 stdio 类 agent（claude、qwen 等）。
