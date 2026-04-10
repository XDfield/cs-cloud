# Runtime / Control Surface RESTful API

Base URL: `http://127.0.0.1:{port}/api/v1`

All request/response bodies use `application/json` unless noted.

## Common Response Envelope

```json
{
  "ok": true,
  "data": { ... },
  "error": null
}
```

On error:

```json
{
  "ok": false,
  "data": null,
  "error": {
    "code": "NOT_FOUND",
    "message": "resource not found"
  }
}
```

---

## 1. Runtime Health

### `GET /runtime/health`

Runtime reachability check.

**Response 200**

```json
{
  "ok": true,
  "data": {
    "status": "ok",
    "uptime": 3600,
    "version": "0.1.0"
  }
}
```

---

## 2. Runtime Target Context

### `GET /runtime/target/context`

Returns the current runtime target directory context.

**Response 200**

```json
{
  "ok": true,
  "data": {
    "home": "/home/user",
    "workdir": "/home/user/project",
    "worktree": "/home/user/project/.git/worktrees/feature",
    "platform": "linux",
    "arch": "amd64"
  }
}
```

---

## 3. Runtime Model Capabilities

### `GET /runtime/models/capabilities`

Lists connected providers and their model capabilities.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `provider` | string | Filter by provider ID (optional) |

**Response 200**

```json
{
  "ok": true,
  "data": {
    "providers": [
      {
        "id": "openai",
        "name": "OpenAI",
        "connected": true,
        "default_model": "gpt-4o",
        "models": [
          {
            "id": "gpt-4o",
            "name": "GPT-4o",
            "context_window": 128000,
            "supports_tools": true,
            "supports_vision": true,
            "supports_streaming": true
          }
        ]
      }
    ]
  }
}
```

---

## 4. Runtime File Operations

### `GET /runtime/files`

List directory contents.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `path` | string | Directory path (required) |
| `recursive` | bool | Recursive listing (default: false) |
| `limit` | int | Max entries (default: 1000) |

**Response 200**

```json
{
  "ok": true,
  "data": {
    "path": "/home/user/project",
    "entries": [
      {
        "name": "src",
        "type": "directory",
        "size": 0,
        "modified": "2025-04-10T12:00:00Z"
      },
      {
        "name": "main.go",
        "type": "file",
        "size": 2048,
        "modified": "2025-04-10T12:00:00Z"
      }
    ]
  }
}
```

### `GET /runtime/files/content`

Read file content.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `path` | string | File path (required) |
| `offset` | int | Line offset, 1-indexed (default: 1) |
| `limit` | int | Max lines (default: 2000) |

**Response 200**

```json
{
  "ok": true,
  "data": {
    "path": "/home/user/project/main.go",
    "content": "package main\n\nfunc main() {}\n",
    "lines": 4,
    "offset": 1,
    "total_lines": 4
  }
}
```

---

## 5. Runtime Find Operations

### `GET /runtime/find/files`

Find files by name pattern.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `pattern` | string | Glob pattern, e.g. `*.go` (required) |
| `path` | string | Search root (default: workdir) |
| `limit` | int | Max results (default: 100) |

**Response 200**

```json
{
  "ok": true,
  "data": {
    "pattern": "*.go",
    "path": "/home/user/project",
    "results": [
      {
        "path": "/home/user/project/main.go",
        "name": "main.go",
        "type": "file"
      }
    ]
  }
}
```

### `GET /runtime/find/text`

Search file contents by text/regex.

**Query Parameters**

| Name | Type | Description |
|------|------|-------------|
| `query` | string | Search query, supports regex (required) |
| `path` | string | Search root (default: workdir) |
| `include` | string | File pattern filter, e.g. `*.go` (optional) |
| `limit` | int | Max results (default: 50) |

**Response 200**

```json
{
  "ok": true,
  "data": {
    "query": "func main",
    "path": "/home/user/project",
    "results": [
      {
        "file": "/home/user/project/main.go",
        "line": 3,
        "column": 1,
        "text": "func main() {}",
        "match_start": 1,
        "match_end": 10
      }
    ]
  }
}
```

---

## 6. Runtime MCP

### `GET /runtime/mcp/status`

Get MCP server connection status.

**Response 200**

```json
{
  "ok": true,
  "data": {
    "servers": [
      {
        "id": "filesystem",
        "name": "Filesystem MCP",
        "status": "connected",
        "tools_count": 5,
        "connected_at": "2025-04-10T12:00:00Z"
      }
    ]
  }
}
```

### `POST /runtime/mcp/connections`

Connect to an MCP server.

**Request Body**

```json
{
  "server_id": "filesystem",
  "config": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user"]
  }
}
```

**Response 201**

```json
{
  "ok": true,
  "data": {
    "server_id": "filesystem",
    "status": "connected",
    "tools_count": 5
  }
}
```

### `DELETE /runtime/mcp/connections/:server_id`

Disconnect an MCP server.

**Response 200**

```json
{
  "ok": true,
  "data": {
    "server_id": "filesystem",
    "status": "disconnected"
  }
}
```

---

## 7. Runtime LSP

### `GET /runtime/lsp/status`

Get LSP server status.

**Response 200**

```json
{
  "ok": true,
  "data": {
    "servers": [
      {
        "id": "gopls",
        "language": "go",
        "status": "running",
        "initialized": true
      }
    ]
  }
}
```

---

## 8. Runtime VCS

### `GET /runtime/vcs`

Get version control info for the current target.

**Response 200**

```json
{
  "ok": true,
  "data": {
    "system": "git",
    "branch": "feature/api-design",
    "commit": "abc1234",
    "dirty": true,
    "remote": "origin",
    "root": "/home/user/project"
  }
}
```

**Response 200 (no VCS)**

```json
{
  "ok": true,
  "data": null
}
```

---

## 9. Runtime Terminal

### `POST /runtime/terminals`

Create a new terminal instance.

**Request Body**

```json
{
  "shell": "/bin/bash",
  "cwd": "/home/user/project",
  "env": {
    "TERM": "xterm-256color"
  },
  "cols": 80,
  "rows": 24
}
```

**Response 201**

```json
{
  "ok": true,
  "data": {
    "id": "pty-001",
    "shell": "/bin/bash",
    "cwd": "/home/user/project",
    "cols": 80,
    "rows": 24,
    "created_at": "2025-04-10T12:00:00Z"
  }
}
```

### `POST /runtime/terminals/:id/input`

Send input to a terminal.

**Request Body**

```json
{
  "data": "ls -la\n"
}
```

**Response 200**

```json
{
  "ok": true,
  "data": {
    "bytes_written": 7
  }
}
```

### `PATCH /runtime/terminals/:id/size`

Resize a terminal.

**Request Body**

```json
{
  "cols": 120,
  "rows": 40
}
```

**Response 200**

```json
{
  "ok": true,
  "data": {
    "id": "pty-001",
    "cols": 120,
    "rows": 40
  }
}
```

### `DELETE /runtime/terminals/:id`

Remove a terminal instance.

**Response 200**

```json
{
  "ok": true,
  "data": {
    "id": "pty-001",
    "status": "disposed"
  }
}
```

### `WS /runtime/terminals/:id/stream`

WebSocket endpoint for terminal output stream.

Connect via WebSocket to receive real-time terminal output as binary frames.

**Message Format (server → client)**

Binary frames containing raw PTY output bytes.

---

## 10. Runtime Instance

### `DELETE /runtime/instances/:id`

Dispose a runtime instance and clean up resources.

**Response 200**

```json
{
  "ok": true,
  "data": {
    "id": "inst-001",
    "status": "disposed"
  }
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request parameters |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource already exists or state conflict |
| `INTERNAL` | 500 | Internal server error |
| `UNAVAILABLE` | 503 | Runtime dependency unavailable (e.g. MCP, LSP) |

---

## Summary Table

| Capability | Method | Path | Priority |
|---|---|---|---|
| `runtime.health` | GET | `/runtime/health` | bootstrap |
| `runtime.target.context` | GET | `/runtime/target/context` | bootstrap |
| `runtime.model.capabilities.list` | GET | `/runtime/models/capabilities` | bootstrap |
| `runtime.file.list` | GET | `/runtime/files` | standard |
| `runtime.file.read` | GET | `/runtime/files/content` | standard |
| `runtime.find.files` | GET | `/runtime/find/files` | standard |
| `runtime.find.text` | GET | `/runtime/find/text` | standard |
| `runtime.mcp.status` | GET | `/runtime/mcp/status` | lazy |
| `runtime.mcp.connect` | POST | `/runtime/mcp/connections` | lazy |
| `runtime.mcp.disconnect` | DELETE | `/runtime/mcp/connections/:server_id` | lazy |
| `runtime.lsp.status` | GET | `/runtime/lsp/status` | lazy |
| `runtime.vcs.get` | GET | `/runtime/vcs` | lazy |
| `runtime.terminal.create` | POST | `/runtime/terminals` | standard |
| `runtime.terminal.input` | POST | `/runtime/terminals/:id/input` | standard |
| `runtime.terminal.resize` | PATCH | `/runtime/terminals/:id/size` | standard |
| `runtime.terminal.remove` | DELETE | `/runtime/terminals/:id` | standard |
| `runtime.instance.dispose` | DELETE | `/runtime/instances/:id` | standard |
