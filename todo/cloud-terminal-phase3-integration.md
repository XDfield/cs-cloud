# Phase 3 - 联调与优化

> 提案参考：[docs/cloud-terminal.md](../docs/cloud-terminal.md) §6

**预计耗时**：1-2 天
**前置条件**：Phase 1 + Phase 2 完成
**状态**：`in_progress`

---

## 任务清单

### 3.1 端到端联调

- [x] cs-cloud 构建成功，修复 AgentManager.CreateAgent 死锁 bug
- [x] cs-cloud serve 启动成功
- [x] REST API curl 测试全部通过（create/kill/resize/restart/input/SSE stream）
- [x] SSE data 事件 base64 解码正确（ANSI 转义序列 + 终端输出）
- [x] app-ai-native build 成功
- [x] 创建 DeviceTerminalProvider — 终端上下文绑定到 DeviceInterface
- [x] 创建 WorkspaceTerminalPanel — 工作区布局中的终端面板
- [x] 修复 deviceId — 从 proxy URL 提取（而非 workspace.id）
- [x] 传递 directory 作为 cwd 参数
- [ ] 浏览器完整联调测试（需 cs-cloud + costrict-web + app-ai-native 同时运行）

### 3.2 断线重连处理

- [x] SSE 连接中断自动重连（指数退避，3 次重试）— CloudTerminalApi 已实现
- [x] WS 输入断连 → 尝试重连 + 重新 bind session — CloudTerminalApi 已实现
- [ ] 设备离线/上线 → 前端状态提示（需联调验证）
- [ ] 重连后恢复终端内容（buffer 序列化已有，需联调验证）

### 3.3 错误提示与状态显示

- [x] 终端连接错误 → showToast 提示 — terminal.tsx 已实现
- [x] SSE exit 事件 → onConnectError 回调 — terminal.tsx 已实现
- [ ] PTY 进程退出时显示 exit code + 重启按钮（需 UI 组件）
- [ ] 会话满（20/20）时提示用户关闭不用的 tab（需 UI 组件）
- [ ] 网络错误重试提示（需联调验证）

### 3.4 性能调优

- [ ] SSE 输出写缓冲（batch write，参考 terminal-writer.ts）— 已使用 terminalWriter
- [ ] WS 心跳间隔调优（cloud proxy 链路较长）— 默认 20s，可调
- [ ] PTY 输出限流（避免大量输出打满 yamux 窗口）— 需联调验证
- [ ] 内存占用监控（长连接 + 大量输出的场景）— 需联调验证

---

## 验收标准

- [ ] 端到端链路稳定运行 30 分钟以上无断连
- [ ] 断线后 10 秒内自动重连并恢复
- [ ] 大量输出（如 `cat /var/log/syslog`）不卡顿
- [ ] 多 tab 并发使用不互相干扰
- [ ] 设备离线/上线状态正确反映到前端
- [ ] 内存占用稳定，无泄漏

## 产出文件

| 文件 | 操作 | 状态 |
|------|------|------|
| `cs-cloud/internal/runtime/manager.go` | 修复：CreateAgent 死锁 | ✅ |
| `app-ai-native/src/context/device-terminal.tsx` | 新增：设备终端上下文 | ✅ |
| `app-ai-native/src/pages/workspace/components/workspace-terminal-panel.tsx` | 新增：工作区终端面板 | ✅ |
| `app-ai-native/src/pages/workspace/components/device-interface.tsx` | 修改：添加 DeviceTerminalProvider | ✅ |
| `app-ai-native/src/components/terminal.tsx` | 修改：import 改为 device-terminal | ✅ |