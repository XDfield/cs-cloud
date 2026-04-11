# Phase 1 - 版本基础

> 提案参考：[docs/versioning-and-upgrade.md](../docs/versioning-and-upgrade.md) §2.2 版本信息注入、§2.3 Git 标签与分支策略

**预计耗时**：1-2 天
**前置条件**：无
**状态**：`done`

---

## 任务清单

### 1.1 新增 `internal/version` 包

- [x] 创建 `internal/version/version.go`
  - 定义 `Version`, `Commit`, `BuildTime`, `GoVersion`, `Platform` 变量
  - 实现 `Get()` 和 `FullString()` 方法
- [x] 编写单元测试 `internal/version/version_test.go`

### 1.2 添加 `cs-cloud version` 命令

- [x] 创建 `internal/cli/version.go`
- [x] 在 `root.go` 的 `dispatch()` 中注册 `version` 命令
- [x] 输出格式：版本号、commit、build time、platform

### 1.3 在现有命令中集成版本信息

- [x] `doctor` 命令输出中添加 `version` 和 `commit` 字段
  - 文件：`internal/cli/doctor.go`
- [x] `/api/v1/runtime/health` 响应中添加 `version` 字段
  - 文件：`internal/localserver/server.go` — 使用已有的 `WithVersion` Option
  - 文件：`internal/localserver/health_handler.go`（已有 `version` 字段，通过 `WithVersion` 传入）
- [x] daemon 启动日志中打印完整版本信息
  - 文件：`internal/cli/daemon.go`

### 1.4 添加构建脚本

- [x] 创建 `build.sh`，包含以下命令：
  - `build`：带 ldflags 注入构建
  - `test`：运行测试
  - `lint`：运行 golangci-lint
  - `clean`：清理构建产物
- [x] ldflags 注入 `Version`, `Commit`, `BuildTime`, `GoVersion`, `Platform`

### 1.5 Git 分支策略初始化

- [ ] 创建 `dev` 分支（如需）
- [ ] 约定分支命名规范（`feature/*`, `fix/*`）写入贡献文档

---

## 验收标准

- [x] `cs-cloud version` 输出正确版本信息
- [x] `cs-cloud doctor` 显示版本号
- [x] daemon 日志中包含版本信息
- [x] `/api/v1/runtime/health` 返回 version 字段
- [x] `bash build.sh build` 可正确注入版本信息

## 产出文件

| 文件 | 操作 |
|------|------|
| `internal/version/version.go` | 修改 |
| `internal/version/version_test.go` | 新增 |
| `internal/cli/version.go` | 新增 |
| `internal/cli/root.go` | 修改 |
| `internal/cli/doctor.go` | 修改 |
| `internal/cli/daemon.go` | 修改 |
| `build.sh` | 新增 |
