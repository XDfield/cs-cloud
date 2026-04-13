# Multi-Agent Support Technical Proposal

## Goal

Enable cs-cloud to simultaneously manage multiple AI agent instances across different backends (Claude, Qwen, Goose, etc.), each running in its own process with independent conversation state, while exposing a unified REST + WebSocket surface to upstream consumers.

---

## Problem Statement

Current design assumes a single active agent per conversation. Real-world usage requires:

1. **Parallel conversations** — user opens two terminals, each talking to a different backend
2. **Backend switching** — user starts with Claude, then wants Qwen for a specific task
3. **Multi-agent orchestration** — an agent delegates sub-tasks to another backend
4. **Resource isolation** — each agent process has its own stdin/stdout, permission queue, model config

The existing `acp-agent-integration.md` defines the driver abstraction but leaves the multi-agent lifecycle, routing, and state management unspecified.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Upstream Consumer                            │
│              (app-ai-native / cloud shell / SDK)                  │
└────────────┬────────────────────────────────┬───────────────────┘
             │ REST + X-Backend       │ WS /events/stream
┌────────────▼────────────────────────────────▼───────────────────┐
│                     cs-cloud LocalServer                          │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │              Routing Middleware                            │  │
│  │  X-Backend → resolve driver → validate        │  │
│  └────────────┬───────────────────────────────────────────────┘  │
│  ┌────────────▼───────────────────────────────────────────────┐  │
│  │              AgentManager (core)                          │  │
│  │  agents: map[convID]Agent   drivers: map[name]Driver      │  │
│  │  sessions: SessionStore      events: EventBus             │  │
│  └────┬──────────┬──────────┬──────────┬───────────────────────┘  │
│       │          │          │          │                         │
│  ┌────▼───┐ ┌───▼────┐ ┌──▼───┐ ┌───▼────┐                     │
│  │ACPDrv  │ │ACPDrv  │ │ACPDrv│ │WSDrv   │                     │
│  │claude  │ │qwen    │ │goose │ │remote  │                     │
│  └────┬───┘ └───┬────┘ └──┬───┘ └───┬────┘                     │
└───────┼─────────┼─────────┼─────────┼───────────────────────────┘
        │         │         │         │
   ┌────▼──┐ ┌───▼───┐ ┌──▼───┐ ┌───▼───┐
   │claude │ │qwen   │ │goose │ │ws conn│
   │proc   │ │proc   │ │proc  │ │       │
   └───────┘ └───────┘ └──────┘ └───────┘
```

---

## Core Components

### 1. Agent Interface (`internal/agent/agent.go`)

Unified contract for all agent types. Every agent instance implements this regardless of driver.

```go
package agent

type Agent interface {
    ID() string
    Backend() string
    Driver() string
    State() AgentState

    Start(ctx context.Context) error
    Kill() error

    SendMessage(ctx context.Context, msg PromptMessage) error
    CancelPrompt()

    ConfirmPermission(callID string, optionID string) error
    PendingPermissions() []PermissionInfo

    GetModelInfo() *ModelInfo
    SetModel(ctx context.Context, modelID string) (*ModelInfo, error)
    SetSessionMode(ctx context.Context, mode string) error

    SessionID() string
    SetEventEmitter(emitter func(Event))
}
```

### 2. Driver Interface (`internal/agent/driver.go`)

Factory + detection for a communication protocol.

```go
type Driver interface {
    Name() string
    Detect() ([]DetectedAgent, error)
    CreateAgent(cfg AgentConfig) (Agent, error)
    HealthCheck(ctx context.Context, backend string) (*HealthResult, error)
}
```

### 3. AgentManager (`internal/runtime/manager.go`)

Central orchestrator. Owns the `convID → Agent` map and the `name → Driver` registry.

```go
type AgentManager struct {
    mu      sync.RWMutex
    agents  map[string]agent.Agent     // convID → Agent
    drivers map[string]agent.Driver     // driver name → Driver
    store   *SessionStore
    bus     *EventBus
}

func (m *AgentManager) RegisterDriver(d agent.Driver)
func (m *AgentManager) ResolveDriver(backend string) (agent.Driver, error)

func (m *AgentManager) CreateAgent(convID string, cfg agent.AgentConfig) error
func (m *AgentManager) GetAgent(convID string) (agent.Agent, bool)
func (m *AgentManager) KillAgent(convID string)
func (m *AgentManager) KillAll()

func (m *AgentManager) DetectAgents() ([]agent.DetectedAgent, error)
func (m *AgentManager) HealthCheck(ctx context.Context, backend string) (*agent.HealthResult, error)
```

**Driver resolution** (internally by `Registry.InferDriver(backend)`):

```go
func (m *AgentManager) ResolveDriver(backend string) (agent.Driver, error) {
    // 1. Known ACP backend → "acp"
    if acp.IsBuiltInBackend(backend) {
        return m.drivers["acp"], nil
    }
    // 2. "remote" → "ws"
    if backend == "remote" {
        return m.drivers["ws"], nil
    }
    // 3. Fallback → "custom"
    if d, ok := m.drivers["custom"]; ok {
        return d, nil
    }
    return nil, fmt.Errorf("no driver for backend: %s", backend)
}
```

### 4. SessionStore (`internal/runtime/session_store.go`)

Persists conversation metadata so agents can be resumed across server restarts.

```go
type SessionRecord struct {
    ID           string `json:"id"`
    ACPSessionID string `json:"acp_session_id"`
    Backend      string `json:"backend"`
    Driver       string `json:"driver"`
    WorkingDir   string `json:"working_dir"`
    ModelID      string `json:"model_id"`
    CreatedAt    int64  `json:"created_at"`
    UpdatedAt    int64  `json:"updated_at"`
}
```

### 5. EventBus (`internal/runtime/eventbus.go`)

Fan-out events to WebSocket subscribers. Supports per-backend filtering.

```go
type Event struct {
    Type           string `json:"type"`
    ConversationID string `json:"conversation_id"`
    Backend        string `json:"backend"`
    MessageID      string `json:"msg_id,omitempty"`
    Data           any    `json:"data"`
}

type EventBus struct {
    mu          sync.RWMutex
    subscribers map[chan Event]*SubFilter
}

type SubFilter struct {
    Backend string // empty = all
}

func (b *EventBus) Subscribe(filter *SubFilter) chan Event
func (b *EventBus) Unsubscribe(ch chan Event)
func (b *EventBus) Emit(event Event)
```

---

## Request Routing

### Middleware: `X-Backend` Resolution

All `/api/v1/conversations/*`, `/api/v1/interactions/*`, and `/api/v1/agents/*` routes pass through a routing middleware:

```go
func (s *Server) agentRoutingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        backend := r.Header.Get("X-Backend")
        if backend == "" {
            backend = s.registry.DefaultBackend()
        }

        ctx := context.WithValue(r.Context(), ctxKeyBackend, backend)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
        driverName := r.Header.Get("X-Driver")

        ctx := context.WithValue(r.Context(), ctxKeyBackend, backend)
        ctx = context.WithValue(ctx, ctxKeyDriver, driverName)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Routing Flow per Endpoint

| Endpoint | Route Source | Fallback |
|---|---|---|
| `POST /conversations` | `X-Backend` → resolve driver → `driver.CreateAgent(cfg)` | — |
| `GET /conversations/:id` | `convID → Agent` from AgentManager | `X-Backend` as validation hint |
| `POST /conversations/:id/prompt` | `convID → Agent` | Error if agent not found or backend mismatch |
| `GET /interactions/permissions` | `X-Backend` → filter all agents with matching backend | — |
| `WS /events/stream` | `X-Backend` → EventBus SubFilter | No header = all events |

### Validation Hint

For `/conversations/:id/*` endpoints, if `X-Backend` is present and the resolved agent's backend doesn't match, return `409 Conflict`. This catches stale routing after a backend switch.

---

## Agent Lifecycle

### Creation (`POST /conversations`)

```
Client → POST /conversations (X-Backend: claude)
  → RoutingMiddleware: resolve backend="claude", driver="acp"
  → Handler: mgr.ResolveDriver("claude") → ACPDriver
  → Handler: ACPDriver.CreateAgent(AgentConfig{Backend:"claude", ...}) → ACPAgent
  → Handler: ACPAgent.Start(ctx) → spawn process → initialize → authenticate → session/new
  → Handler: mgr.agents[convID] = ACPAgent
  → Handler: store.Save(record)
  → Response 201: { ok: true, data: { id: convID, backend: "claude", driver: "acp", state: "session_active" } }
```

### Prompt (`POST /conversations/:id/prompt`)

```
Client → POST /conversations/abc/prompt (X-Backend: claude)
  → RoutingMiddleware: extract X-Backend
  → Handler: mgr.GetAgent("abc") → ACPAgent
  → Handler: validate agent.Backend() == "claude" (if X-Backend present)
  → Handler: agent.SendMessage(ctx, PromptMessage{Content: "..."})
  → ACPAgent: conn.SendPrompt(content)
  → ACPAgent: session/update notifications → adapter.ConvertSessionUpdate → events
  → EventBus.Emit(events)
  → WS subscribers receive events
  → Response 200: { ok: true, data: null }
```

### Kill (`DELETE /conversations/:id`)

```
Client → DELETE /conversations/abc
  → Handler: mgr.GetAgent("abc") → ACPAgent
  → Handler: agent.Kill() → conn.Disconnect()
  → Handler: mgr.KillAgent("abc") → delete from map
  → Handler: store.Delete("abc")
  → EventBus.Emit(Event{Type: "conversation.deleted", ConversationID: "abc"})
  → Response 200: { ok: true, data: { id: "abc", status: "disposed" } }
```

### Auto-Reconnect

When an agent process exits unexpectedly:

```
1. Connection.OnDisconnect callback fires
2. Agent state → StateDisconnected
3. EventBus.Emit(Event{Type: "agent.disconnected", ConversationID: id, Backend: backend})
4. If within grace period (30s):
   - AgentManager attempts reconnect: agent.Start(ctx) with session resume
   - If success: EventBus.Emit(Event{Type: "agent.reconnected"})
   - If fail: EventBus.Emit(Event{Type: "agent.failed"})
5. Upstream consumer sees events via WS and can decide to retry or switch backend
```

---

## Concurrency Model

Each agent runs in its own goroutine group:

```
AgentManager
  ├── ACPAgent(claude, convID=abc)
  │     ├── goroutine: stdout reader → handleMessage → EventBus.Emit
  │     └── goroutine: keepalive ticker
  ├── ACPAgent(qwen, convID=def)
  │     ├── goroutine: stdout reader → handleMessage → EventBus.Emit
  │     └── goroutine: keepalive ticker
  └── WSAgent(remote, convID=ghi)
        └── goroutine: ws read loop → EventBus.Emit
```

- `AgentManager.mu` protects the `agents` map
- Each `Connection` has its own `sync.Mutex` for stdin writes
- `EventBus` uses `sync.RWMutex` for subscriber management
- HTTP handlers are per-request goroutines (standard Go net/http)

---

## REST API Endpoints

Base: `http://127.0.0.1:{port}/api/v1`

All conversation/interaction endpoints require `X-Backend` header.

### Conversation Flow

| Method | Path | Description |
|---|---|---|
| `POST` | `/conversations` | Create conversation (body: `{ working_dir?, model_id?, session_mode? }`) |
| `GET` | `/conversations/:id` | Get conversation info |
| `GET` | `/conversations` | List conversations (query: `?backend=claude`) |
| `PATCH` | `/conversations/:id` | Update config (body: `{ model_id?, session_mode? }`) |
| `DELETE` | `/conversations/:id` | Delete conversation and kill agent |
| `POST` | `/conversations/:id/abort` | Abort current prompt |
| `POST` | `/conversations/:id/revert` | Revert conversation |
| `POST` | `/conversations/:id/unrevert` | Unrevert conversation |
| `GET` | `/conversations/:id/messages` | Get message history |
| `POST` | `/conversations/:id/prompt` | Send prompt (body: `{ content, files? }`) |
| `POST` | `/conversations/:id/command` | Execute slash command |
| `POST` | `/conversations/:id/shell` | Execute shell command |
| `GET` | `/conversations/:id/diff` | Get pending diffs |
| `GET` | `/conversations/:id/todo` | Get todo list |

### Interaction (Human-in-the-loop)

| Method | Path | Description |
|---|---|---|
| `GET` | `/interactions/permissions` | List pending permissions (filter by `X-Backend`) |
| `POST` | `/interactions/permissions/:id/respond` | Respond to permission (body: `{ option_id }`) |
| `GET` | `/interactions/questions` | List pending questions (filter by `X-Backend`) |
| `POST` | `/interactions/questions/:id/reply` | Reply to question |
| `POST` | `/interactions/questions/:id/reject` | Reject question |

### Event Stream

| Method | Path | Description |
|---|---|---|
| `WS` | `/events/stream` | WebSocket event stream (filter by `X-Backend`) |

### Agent Detection & Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/agents` | List all detected agents |
| `GET` | `/agents/:backend/health` | Health check for specific backend |
| `GET` | `/agents/:backend/models` | Probe model info for backend |
| `GET` | `/drivers` | List registered drivers |

---

## Package Layout

```
internal/
├── agent/
│   ├── agent.go          # Agent interface + shared types
│   ├── driver.go         # Driver interface + AgentConfig + DetectedAgent
│   └── driver_custom.go  # CustomDriver skeleton
├── acp/
│   ├── types.go          # JSON-RPC 2.0 + ACP method constants
│   ├── backend.go        # BackendConfig registry (BuiltInBackends)
│   ├── detector.go       # CLI detection (which/where)
│   ├── connection.go     # Process spawn + JSON-RPC message loop
│   ├── connector.go      # Clean env + per-backend spawn logic
│   ├── agent.go          # ACPAgent implementing agent.Agent
│   ├── driver.go         # ACPDriver implementing agent.Driver
│   ├── adapter.go        # ACP session/update → agent.Event conversion
│   ├── approval.go       # Permission auto-approve store
│   ├── model.go          # Model info resolution
│   ├── mcp.go            # MCP session config injection
│   └── constants.go      # Session mode + YOLO mode constants
├── runtime/
│   ├── manager.go        # AgentManager (multi-agent orchestrator)
│   ├── eventbus.go       # Event fan-out with per-backend filtering
│   ├── session_store.go  # Session persistence (bbolt)
│   └── routing.go        # X-Backend middleware + context helpers
├── model/
│   ├── agent.go          # AgentInfo (API response types)
│   ├── session.go        # SessionInfo
│   ├── event.go          # Event types
│   └── permission.go     # Permission types
└── localserver/
    ├── server.go         # Router + route registration
    ├── response.go       # { ok, data, error } envelope
    ├── health_handler.go
    ├── runtime_files.go
    ├── handlers/
    │   ├── conversations.go  # Conversation REST handlers
    │   ├── interactions.go   # Permission/Question REST handlers
    │   ├── agents.go         # Agent detection/health handlers
    │   └── events.go        # WebSocket event stream handler
    └── middleware.go     # Auth + routing middleware
```

---

## Implementation Phases

### Phase 1: Core Abstraction (bootstrap)

Define interfaces and types. No process spawning yet.

| # | File | What |
|---|---|---|
| 1 | `agent/agent.go` | `Agent` interface, `AgentState`, `PromptMessage`, `PermissionInfo`, `ModelInfo`, `Event` |
| 2 | `agent/driver.go` | `Driver` interface, `AgentConfig`, `DetectedAgent`, `HealthResult` |
| 3 | `acp/types.go` | JSON-RPC 2.0 types, ACP method constants, session update types |
| 4 | `acp/backend.go` | `BackendConfig` + `BuiltInBackends` map |
| 5 | `runtime/routing.go` | `X-Backend` middleware, context helpers |
| 6 | `runtime/eventbus.go` | `EventBus` with `SubFilter` |
| 7 | `runtime/manager.go` | `AgentManager` skeleton (RegisterDriver, ResolveDriver, map management) |

### Phase 2: ACP Driver (standard)

Implement the primary ACP driver end-to-end with one backend (Claude).

| # | File | What |
|---|---|---|
| 8 | `acp/detector.go` | CLI detection via `which`/`where` |
| 9 | `acp/connector.go` | Clean env + spawn logic + npx retry |
| 10 | `acp/connection.go` | Process spawn, JSON-RPC message loop, pending request map |
| 11 | `acp/agent.go` | `ACPAgent` implementing `agent.Agent` |
| 12 | `acp/driver.go` | `ACPDriver` implementing `agent.Driver` |
| 13 | `acp/adapter.go` | ACP session/update → `agent.Event` conversion |
| 14 | `acp/approval.go` | Permission auto-approve store |
| 15 | `acp/model.go` | Model info resolution |
| 16 | `acp/constants.go` | Session mode + YOLO mode constants |

### Phase 3: Conversation Handlers (standard)

Wire REST endpoints to AgentManager.

| # | File | What |
|---|---|---|
| 17 | `localserver/handlers/conversations.go` | POST/GET/PATCH/DELETE /conversations, prompt, abort, revert |
| 18 | `localserver/handlers/interactions.go` | Permission + question handlers |
| 19 | `localserver/handlers/agents.go` | Agent detection + health handlers |
| 20 | `localserver/handlers/events.go` | WebSocket event stream handler |
| 21 | `localserver/middleware.go` | Auth + routing middleware |
| 22 | `localserver/server.go` | Register all conversation routes |

### Phase 4: Multi-Backend + Resilience (lazy)

Expand to all built-in backends, add reconnection, session persistence.

| # | File | What |
|---|---|---|
| 23 | `acp/connector.go` | Per-backend connectors (Qwen, Goose, Codex, etc.) |
| 24 | `runtime/session_store.go` | bbolt-based session persistence |
| 25 | `acp/agent.go` | Auto-reconnect + session resume |
| 26 | `acp/mcp.go` | MCP session config injection |
| 27 | `agent/driver_custom.go` | CustomDriver skeleton |

---

## Key Design Decisions

### 1. Header-based routing vs path-based routing

**Decision**: Use `X-Backend` header instead of `/agents/:backend/conversations/:id`.

**Rationale**:
- GET requests don't have request bodies; headers are natural for routing metadata
- Path stays RESTful and stable; adding a new backend doesn't change URL structure
- Upstream consumers set headers once per session, not per request
- WebSocket upgrade can carry headers in the initial HTTP request

### 2. AgentManager owns convID → Agent mapping

**Decision**: Single map in AgentManager, not per-driver maps.

**Rationale**:
- O(1) lookup by conversation ID regardless of driver
- Cross-driver operations (list all conversations) don't require iterating multiple maps
- Validation (backend mismatch detection) is centralized

### 3. EventBus with per-backend filtering

**Decision**: Subscribers specify `SubFilter{Backend: "claude"}` to receive only relevant events.

**Rationale**:
- WS client typically cares about one backend at a time
- Reduces noise and bandwidth for multi-agent scenarios
- Empty filter = all events (for admin/debug UIs)

### 4. Session persistence via bbolt

**Decision**: Use bbolt (pure Go, no CGO) for session storage.

**Rationale**:
- Simple key-value store sufficient for session records
- No external dependency (SQLite requires CGO or WASM)
- Fast for read-heavy workloads (session lookup on every request)
- Portable: works on Linux, macOS, Windows, ARM

### 5. Transparent ACP proxy

**Decision**: cs-cloud does not reinterpret ACP messages, only routes them.

**Rationale**:
- Future ACP method additions are automatically supported
- Backend-specific quirks handled at connector layer, not protocol layer
- REST surface is a convenience mapping, not a replacement for direct ACP access

---

## Error Handling

| Scenario | HTTP Status | Error Code | Behavior |
|---|---|---|---|
| Missing `X-Backend` header | 400 | `BAD_REQUEST` | Reject immediately |
| Unknown backend | 404 | `NOT_FOUND` | No driver can handle this backend |
| Agent process crashed | 200 | — | Event `{type: "agent.disconnected"}` emitted, auto-reconnect attempted |
| Backend mismatch on `:id` route | 409 | `CONFLICT` | `X-Backend` doesn't match agent's backend |
| Agent busy (prompt in progress) | 409 | `CONFLICT` | Another prompt is already running |
| Permission timeout | 200 | — | Event `{type: "permission.timeout"}`, auto-reject or auto-approve per config |
| Driver spawn failure | 503 | `UNAVAILABLE` | CLI not found, auth failed, or process exit on startup |
