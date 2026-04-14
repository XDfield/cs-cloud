# Phase 2 - app-ai-native Cloud Terminal

> 提案参考：[docs/cloud-terminal.md](../docs/cloud-terminal.md) §4

**预计耗时**：2-3 天
**前置条件**：Phase 1 完成（cs-cloud 终端 API 可用）
**状态**：`in_progress` (2.1-2.5 完成，2.6 待验证)

---

## 任务清单

### 2.1 切换 ghostty-web 依赖

- [x] 将 `ghostty-web` 从 `github:anomalyco/ghostty-web#main` 切换为 npm `0.3.0`
- [x] 使用 `bun patch` 生成 Bun 格式的 patch（从 openchamber 的 pnpm 格式 patch 移植）
- [x] patch 内容：Unicode surrogate 范围防护 + lineHeight 支持
- [x] `bun install` 验证 patch 正确应用
- [ ] 浏览器验证 ghostty-web 加载 + 渲染正常（2.6）

**实际产出**：
- `opencode/package.json` — `patchedDependencies` 新增 `ghostty-web@0.3.0`
- `opencode/packages/app-ai-native/package.json` — `"ghostty-web": "0.3.0"`
- `opencode/patches/ghostty-web@0.3.0.patch` — Bun 格式 patch（7 处修改）

### 2.2 实现 CloudTerminalApi (`src/lib/cloud-terminal-api.ts`)

- [x] `create(cwd, rows, cols)` — POST 创建会话
- [x] `kill(sessionId)` — DELETE 终止会话
- [x] `resize(sessionId, rows, cols)` — POST 调整大小
- [x] `restart(sessionId, cwd)` — POST 重启
- [x] `connectSse(sessionId, onEvent, onError)` — SSE 输出流连接（含重连）
- [x] `connectInputWs()` — WebSocket 输入连接（含重连 + keepalive）
- [x] `sendInput(sessionId, data)` — WS 发送终端按键（含 bind 多路复用）
- [x] `unbindSession(sessionId)` — 解绑 WS 会话
- [x] `disconnect()` — 清理所有连接
- [x] 事件回调：`onEvent({ type, data?, exitCode? })`, `onError(error, fatal?)`
- [x] SSE base64 解码由调用方（terminal.tsx）处理
- [x] `isCloudMode(server)` 工具函数

**实际产出**：`src/lib/cloud-terminal-api.ts`（444 行）

### 2.3 修改 Terminal Context (`src/context/terminal.tsx`)

- [x] Cloud-only 模式：移除 SDK PTY 依赖，使用 `CloudTerminalApi`
- [x] `new()` → `CloudTerminalApi.create()`
- [x] `close()` → `CloudTerminalApi.kill()`
- [x] `clone()` → `CloudTerminalApi.restart()`
- [x] `update()` → `CloudTerminalApi.resize()`
- [x] 保持 `LocalPTY` 类型（仅作为类型名，不含本地 PTY 语义）
- [x] 保持 persistence 逻辑不变

**实际产出**：`src/context/terminal.tsx`（304 行，完全重写）

### 2.4 修改 Terminal Component (`src/components/terminal.tsx`)

- [x] Cloud-only 模式：SSE 输出 + WS 输入替代直接 WebSocket
- [x] SSE `data` 事件 → base64 decode → terminal.write()
- [x] WS 连接 → terminal.onData → cloudApi.sendInput()
- [x] 处理 SSE `exit` 事件 → onConnectError 回调
- [x] SSE 断线重连（指数退避，3 次重试）— 由 CloudTerminalApi 处理
- [x] WS 重连 + keepalive — 由 CloudTerminalApi 处理
- [x] 移除本地模式逻辑

**实际产出**：`src/components/terminal.tsx`（494 行，完全重写）

### 2.5 修改 Terminal Panel (`src/pages/session/terminal-panel.tsx`)

- [x] 审查确认：无需修改
- [x] Panel 仅负责 tab 布局管理，所有 cloud 逻辑在 context/component 层
- [x] `useTerminal()` → cloud-only context
- [x] `<Terminal pty={pty}>` → cloud-only component
- [x] `terminal.clone(id)` → cloud restart

### 2.6 端到端手动验证

- [x] cs-cloud 构建成功（Windows amd64）
- [x] cs-cloud serve 启动成功（修复了 AgentManager.CreateAgent 死锁 bug）
- [x] POST /api/v1/terminal → 创建会话成功，返回 sessionId + pid
- [x] GET /api/v1/terminal/{id}/stream → SSE 连接成功，收到 connected + data 事件
- [x] SSE data 事件 → base64 解码正确（ANSI 转义序列 + 终端输出）
- [x] POST /api/v1/terminal/{id}/resize → 调整大小成功
- [x] POST /api/v1/terminal/{id}/input → HTTP 输入成功
- [x] DELETE /api/v1/terminal/{id} → 终止会话成功
- [x] POST /api/v1/terminal/{id}/restart → 重启会话成功（新 pid）
- [x] WS /api/v1/terminal/input-ws → TCP 可达（WS 需浏览器验证）
- [x] app-ai-native build 成功（ghostty-web 0.3.0 + patch 正确打包）
- [ ] 浏览器 cloud 模式 → 打开终端 → 输入输出正常（需联调环境）
- [ ] 调整终端大小 → 输出正确重排（需联调环境）
- [ ] 关闭终端 tab → PTY 会话清理（需联调环境）
- [ ] 网络断开 → SSE 自动重连（需联调环境）

---

## 验收标准

- [ ] Cloud 模式下终端 panel 可正常打开
- [ ] 按键输入 → 远程 PTY 执行 → 输出回显
- [ ] Tab 创建/关闭正确管理 PTY 生命周期
- [ ] SSE 断线自动重连
- [ ] WS 多路复用：多个 tab 共用一个 WS 连接
- [ ] ghostty-web 0.3.0 渲染正常（含 patch 修复）

## 产出文件

| 文件 | 操作 | 状态 |
|------|------|------|
| `opencode/package.json` | 修改：patchedDependencies 新增 | ✅ |
| `opencode/patches/ghostty-web@0.3.0.patch` | 新增：Bun 格式 patch | ✅ |
| `opencode/packages/app-ai-native/package.json` | 修改：ghostty-web 版本 | ✅ |
| `src/lib/cloud-terminal-api.ts` | 新增 | ✅ |
| `src/components/terminal.tsx` | 重写：cloud-only | ✅ |
| `src/context/terminal.tsx` | 重写：cloud-only | ✅ |
| `src/pages/session/terminal-panel.tsx` | 无需修改 | ✅ |
