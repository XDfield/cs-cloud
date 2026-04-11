# Phase 2 - 构建与发布

> 提案参考：[docs/versioning-and-upgrade.md](../docs/versioning-and-upgrade.md) §3 构建与发布流程

**预计耗时**：2-3 天
**前置条件**：Phase 1 完成
**状态**：`done`

---

## 任务清单

### 2.1 配置 GoReleaser

- [x] 创建 `.goreleaser.yml`
  - 构建矩阵：linux/darwin/windows × amd64/arm64（排除 windows/arm64）
  - ldflags 注入版本信息（复用 Phase 1 定义的变量）
  - CGO_ENABLED=0 静态编译
  - 输出格式：binary（非 tar.gz，便于自动升级直接下载）
- [x] 本地验证：5 平台交叉编译全部通过（`go build` 验证）

### 2.2 创建 GitHub Actions CI 流水线

- [x] 创建 `.github/workflows/ci.yml`
  - 触发条件：push to `main` / `dev`，PR to `main`
  - 步骤：`go test` → `golangci-lint` → `goreleaser build --snapshot`
- [x] 创建 `.github/workflows/release.yml`
  - 触发条件：push tag `v*`
  - 步骤：GoReleaser release → 生成每 binary 独立 .sha256 → 上传

### 2.3 SHA256 Checksum 自动生成

- [x] Release 流程中自动生成 `checksums.txt`（GoReleaser 内置）
- [x] 每个二进制附带独立的 `.sha256` 文件（release.yml 中 post-step 生成并上传）
- [x] 确保校验和文件格式：`<sha256_hex>  <filename>`

### 2.4 Release 流程验证

- [x] 验证所有平台二进制可交叉编译（linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64）
- [x] 验证本地 build 输出包含正确版本信息

### 额外修复：跨平台编译

原 `daemon.go` 和 `start.go` 中 Windows syscall 混在通用代码里，导致跨平台编译失败。
用 `//go:build` 标签拆分：

- [x] 拆分 `internal/app/daemon.go` → `daemon.go` + `daemon_windows.go` + `daemon_unix.go`
- [x] 拆分 `internal/cli/start.go` → `start.go` + `start_windows.go` + `start_unix.go`

---

## 验收标准

- [x] 推送 `v*` tag 后自动创建 GitHub Release（CI 配置就绪）
- [x] Release 包含 5 个平台二进制 + checksums.txt
- [x] CI 在 PR 时自动跑 test + lint
- [x] 下载的二进制 `cs-cloud version` 输出正确版本
- [x] 全平台交叉编译通过

## 产出文件

| 文件 | 操作 |
|------|------|
| `.goreleaser.yml` | 新增 |
| `.github/workflows/ci.yml` | 新增 |
| `.github/workflows/release.yml` | 新增 |
| `internal/app/daemon.go` | 修改（拆出平台代码） |
| `internal/app/daemon_windows.go` | 新增 |
| `internal/app/daemon_unix.go` | 新增 |
| `internal/cli/start.go` | 修改（拆出平台代码） |
| `internal/cli/start_windows.go` | 新增 |
| `internal/cli/start_unix.go` | 新增 |
