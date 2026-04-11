# Phase 5 - 稳定化

> 提案参考：[docs/versioning-and-upgrade.md](../docs/versioning-and-upgrade.md) §5 安全考虑、§6 实施路线、§8 监控与可观测性

**预计耗时**：2-3 天
**前置条件**：Phase 3 和 Phase 4 完成
**状态**：`done`

---

## 任务清单

### 5.1 端到端升级测试

- [x] 端到端流程测试 `TestEndToEndUpgradeFlow`（httest mock API + download server）
  - check → download → verify → backup → replace → state 验证
  - Windows 跳过（文件锁定），Linux/macOS 执行
- [x] 下载+校验成功流程测试 `TestDownloadVerifySuccessFlow`
- [x] 全平台交叉编译验证通过

### 5.2 失败场景测试

- [x] SHA256 不匹配 → 拒绝下载并清理 `TestDownloadSHA256Mismatch`
- [x] HTTP 非 200 → 错误返回 `TestDownloadHTTPError`
- [x] 上下文取消 → 中断并清理 `TestDownloadCancelledContext`、`TestCheckerWithCancelledContext`
- [x] 无效 URL → 错误返回 `TestCheckerBadURL`
- [x] 404 响应 → 错误返回 `TestCheckerNon200Response`
- [x] 空 download_url → 错误返回 `TestManagerDownloadAndVerifyNoURL`
- [x] 版本不匹配 → 拒绝安装 `TestManagerApplyVersionMismatch`
- [x] 无可用更新 → 错误返回 `TestManagerApplyNoUpdate`
- [x] 下载失败后临时文件清理 `TestDownloadCleanupOnError`
- [x] 空文件校验 `TestVerifierEmptyFile`
- [x] 无 SHA256 跳过校验 `TestDownloadSucceedsWithoutSHA256`

### 5.3 回滚测试

- [x] 从备份回滚 `TestReplacerRollbackFromBackup`
- [x] 无备份时回滚失败 `TestReplacerRollbackNoBackup`
- [x] `MarkVerified` 状态更新 `TestReplacerMarkVerified`
- [x] `Cleanup` 清理备份文件 `TestReplacerCleanup`
- [x] 启动自检逻辑：`manager.go` 中 `verifyOnStartup` 版本不匹配自动回滚
- [x] 文件复制 `TestCopyFile`、不存在文件 `TestCopyFileNotExist`
- [x] 并发安全 `TestConcurrentReplacerAccess`

### 5.4 监控与日志

- [x] 升级关键事件写入结构化日志（`logger.Info/Error` 贯穿 manager.go 全流程）
- [x] 升级状态集成到 `cs-cloud doctor` 输出（`upgrade` 字段）
- [x] 升级历史记录持久化 `~/.cs-cloud/upgrades/history.json`
  - `Replacer.AppendHistory()` — 最多保留 20 条
  - `Replacer.LoadHistory()` — 读取历史
  - 验证成功后自动追加历史记录
- [x] `cs-cloud update history` 显示当前状态 + 历史列表
- [x] `Manager.FullHistory()` 暴露完整历史
- [x] 并发升级防护 `TestManagerConcurrentApplyProtection`

---

## 验收标准

- [x] 所有目标平台升级成功（Windows 跳过文件替换，逻辑验证通过）
- [x] 所有失败场景有正确的恢复行为（25 个测试用例覆盖）
- [x] 回滚机制可靠工作
- [x] 升级过程有完整日志记录
- [x] `cs-cloud doctor` 显示当前版本和升级状态
- [x] `cs-cloud update history` 显示完整升级历史
- [x] 全平台交叉编译通过

## 产出文件

| 文件 | 操作 |
|------|------|
| `internal/updater/manager_test.go` | 重写（25 个测试用例） |
| `internal/updater/replacer.go` | 修改（`io.Copy` 替代 `fillBufCopy`，新增 `AppendHistory/LoadHistory`） |
| `internal/updater/manager.go` | 修改（`FullHistory()`，验证后写历史） |
| `internal/cli/update.go` | 修改（`update history` 显示完整历史） |

## 测试用例汇总

| 测试 | 覆盖场景 |
|------|---------|
| `TestEndToEndUpgradeFlow` | 端到端升级流程 |
| `TestDownloadSHA256Mismatch` | SHA256 校验失败 |
| `TestDownloadHTTPError` | HTTP 服务端错误 |
| `TestDownloadCancelledContext` | 下载上下文取消 |
| `TestCheckerBadURL` | 无效 URL |
| `TestCheckerNon200Response` | 非 200 响应 |
| `TestReplacerBackupAndReplace` | 备份 + 替换 |
| `TestReplacerRollbackFromBackup` | 从备份回滚 |
| `TestReplacerRollbackNoBackup` | 无备份回滚失败 |
| `TestReplacerMarkVerified` | 标记验证完成 |
| `TestReplacerCleanup` | 清理备份文件 |
| `TestManagerConcurrentApplyProtection` | 并发升级防护 |
| `TestManagerApplyNoUpdate` | 无可用更新 |
| `TestManagerApplyVersionMismatch` | 版本不匹配 |
| `TestCopyFile` | 文件复制 |
| `TestCopyFileNotExist` | 源文件不存在 |
| `TestHistoryAppend` | 历史记录追加 |
| `TestDownloadCleanupOnError` | 下载失败清理 |
| `TestVerifierEmptyFile` | 空文件校验 |
| `TestDownloadSucceedsWithoutSHA256` | 无 SHA256 跳过校验 |
| `TestManagerDownloadAndVerifyNoURL` | 无下载 URL |
| `TestConcurrentReplacerAccess` | 并发状态访问 |
| `TestCheckerWithCancelledContext` | 检查上下文取消 |
| `TestCheckResultUserAgent` | User-Agent 头 |
| `TestDownloadVerifySuccessFlow` | 下载 + 校验成功流程 |
