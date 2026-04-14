# Phase 0 - 配置基础

> 提案参考：[docs/cloud-terminal.md](../docs/cloud-terminal.md) §3

**预计耗时**：0.5 天
**状态**：`done`

---

## 任务清单

### 0.1 扩展配置结构 (`internal/config/config.go`)

- [x] 添加 `DefaultShell` 字段
- [x] JSON tag: `"default_shell"`
- [x] 环境变量: `CS_CLOUD_SHELL`

### 0.2 实现配置文件加载 (`internal/config/load.go`)

- [x] 支持 `~/.cs-cloud/config.json` 文件读取
- [x] 优先级：环境变量 > config.json > 代码默认值
- [x] 添加 `CLOUD_BASE_URL` 作为 `COSTRICT_CLOUD_BASE_URL` 的简写别名
- [x] 文件不存在时静默忽略（不报错）

---

## 验收标准

- [x] `CS_CLOUD_SHELL=/bin/zsh` 环境变量可覆盖默认 shell
- [x] `~/.cs-cloud/config.json` 中 `"default_shell": "/bin/fish"` 生效
- [x] 环境变量优先于配置文件
- [x] 编译通过

## 产出文件

| 文件 | 操作 |
|------|------|
| `internal/config/config.go` | 修改：添加 DefaultShell 字段 |
| `internal/config/load.go` | 修改：添加 config.json 文件加载逻辑 |
