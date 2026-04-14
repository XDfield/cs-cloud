# 云端终端 (Cloud Terminal) - 总进度跟踪

> 技术提案：[docs/cloud-terminal.md](../docs/cloud-terminal.md)

## 总览

| Phase | 名称 | 预计耗时 | 状态 | 进度 |
|-------|------|---------|------|------|
| 0 | 配置基础 | 0.5 天 | `done` | 1/1 |
| 1 | cs-cloud Terminal 核心 | 2-3 天 | `done` | 7/7 |
| 2 | app-ai-native Cloud Terminal | 2-3 天 | `in_progress` | 6/6 (curl✅, 浏览器待联调) |
| 3 | 联调与优化 | 1-2 天 | `in_progress` | 3.1-3.3 完成，3.4 待联调 |

## 里程碑

- [x] cs-cloud 支持环境变量 + config.json 配置默认 shell（Phase 0）
- [x] cs-cloud 本地终端 API 端到端可用（Phase 1）
- [ ] app-ai-native 通过 cloud proxy 连接远程终端（Phase 2）
- [ ] SSE 输出 + WS 输入双通道稳定运行（Phase 3）

## 详细任务文档

- [Phase 0 - 配置基础](./cloud-terminal-phase0-config.md)
- [Phase 1 - cs-cloud Terminal 核心](./cloud-terminal-phase1-terminal-core.md)
- [Phase 2 - app-ai-native Cloud Terminal](./cloud-terminal-phase2-app-terminal.md)
- [Phase 3 - 联调与优化](./cloud-terminal-phase3-integration.md)