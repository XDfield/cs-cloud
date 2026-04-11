# Phase 3 - 自动升级核心

> 提案参考：[docs/versioning-and-upgrade.md](../docs/versioning-and-upgrade.md) §4 自动升级设计

**预计耗时**：3-5 天
**前置条件**：Phase 1 完成（version 包），Phase 2 完成（Release 产物）
**状态**：`done`

---

## 任务清单

### 3.1 实现版本检查器 (`internal/updater/checker.go`)

- [x] 定义 `CheckResult` 结构体
- [x] 实现 `Checker.Check(ctx)` 方法
- [x] 调用云端 API：`GET /cloud-api/api/updates/check?platform=&version=`
- [x] 解析响应，处理无更新 / 有更新 / 错误三种场景
- [x] 自动附带 platform 和 version 参数

### 3.2 实现下载管理 (`internal/updater/downloader.go`)

- [x] 下载到临时目录（`~/.cs-cloud/upgrades/tmp/`）
- [x] 下载过程中同时计算 SHA256（流式 hash）
- [x] 支持 HTTP proxy 环境变量（通过标准 http.Client）
- [x] 下载超时控制（默认 10 分钟）
- [x] 下载失败清理临时文件

### 3.3 实现校验器 (`internal/updater/verifier.go`)

- [x] SHA256 校验：文件 hash 与预期值比对
- [x] 空值跳过校验（预留签名验证接口）
- [x] 校验失败返回明确错误

### 3.4 实现平台替换 (`internal/updater/replacer.go`)

- [x] Linux/macOS：`os.Rename()` 直接替换
- [x] Windows：当前 exe → `.old`，新文件 → 当前 exe 名，失败自动回滚 rename
- [x] 备份当前二进制到 `~/.cs-cloud/upgrades/cs-cloud.bak`
- [x] 记录替换状态到 `current.json`
- [x] `Rollback()` 方法支持手动/自动回滚
- [x] `Cleanup()` 清理备份和 `.old` 文件

### 3.5 实现升级管理器 (`internal/updater/manager.go`)

- [x] 编排完整升级流程：check → download → verify → backup → replace
- [x] 定时检查（默认 6 小时间隔，可配置 `WithInterval`）
- [x] 升级策略：auto / download / manual（`WithPolicy`）
- [x] 并发保护（`sync.Mutex` 防止重复升级）
- [x] 启动自检（`pending_verify` → `completed`）
- [x] 自动回滚：版本不匹配时回滚
- [x] `Apply(ctx, targetVersion)` 手动触发升级
- [x] `CheckNow(ctx)` 立即检查
- [x] `Rollback()` 手动回滚
- [x] `History()` 查看升级历史

### 3.6 添加 CLI 子命令

- [x] 创建 `internal/cli/update.go`
- [x] 注册到 `dispatch()` 中
- [x] `update check`：检查可用更新
- [x] `update apply [--version v1.x.x]`：手动触发升级
- [x] `update rollback`：回滚到上一版本
- [x] `update history`：显示升级历史
- [x] daemon 集成：cloud 模式下启动 `Manager.Run()` 定时检查
- [x] `doctor` 输出中添加 `upgrade` 状态字段
- [x] 单元测试 8 个用例全部通过

---

## 验收标准

- [x] `cs-cloud update check` 正确调用版本 API 并显示结果
- [x] `cs-cloud update apply` 完成下载 → 校验 → 替换全流程
- [x] daemon 运行时每 6 小时自动检查升级
- [x] Linux 平台原子替换（os.Rename）
- [x] Windows 平台替换（.old + rename + 失败回滚）
- [x] 升级失败自动回滚到旧版本
- [x] `cs-cloud update rollback` 可手动回滚
- [x] `cs-cloud update history` 显示升级历史
- [x] `cs-cloud doctor` 显示 upgrade 状态
- [x] 全平台交叉编译通过

## 产出文件

| 文件 | 操作 |
|------|------|
| `internal/updater/checker.go` | 新增 |
| `internal/updater/downloader.go` | 新增 |
| `internal/updater/verifier.go` | 新增 |
| `internal/updater/replacer.go` | 新增 |
| `internal/updater/manager.go` | 新增 |
| `internal/updater/manager_test.go` | 新增 |
| `internal/cli/update.go` | 新增 |
| `internal/cli/root.go` | 修改 |
| `internal/cli/daemon.go` | 修改 |
| `internal/cli/doctor.go` | 修改 |
