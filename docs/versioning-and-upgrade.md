# cs-cloud 版本管理与自动升级技术提案

## 1. 项目现状分析

### 1.1 当前状态

cs-cloud 是一个 Go 单二进制设备端 cloud daemon，具备以下特征：

- **部署形态**：单二进制，跨平台（Windows / Linux / macOS）
- **运行模式**：守护进程（daemon），通过 `_daemon` 子命令后台运行
- **连接方式**：WebSocket tunnel 连接云端平台
- **当前缺失**：
  - 无版本号定义与注入机制
  - 无构建流水线与 Release 流程
  - 无自动升级能力
  - 无构建元信息（commit、build time 等）

### 1.2 核心挑战

| 挑战 | 说明 |
|------|------|
| 运行时替换 | 需要在 daemon 运行中替换自身二进制文件 |
| 跨平台差异 | Windows 进程锁文件，Linux/macOS 可 unlink 后覆盖 |
| 回滚安全 | 升级失败需能回退到上一版本 |
| 网络环境 | 设备可能处于弱网或代理环境 |
| 版本一致性 | 需确保云端与设备端版本信息同步 |

---

## 2. 版本管理方案

### 2.1 语义化版本（SemVer）

采用 `vMAJOR.MINOR.PATCH` 格式：

```
v1.2.3
 │ │ │
 │ │ └─ PATCH: bug 修复、安全补丁（兼容升级）
 │ └─── MINOR: 新功能（兼容升级）
 └───── MAJOR: 破坏性变更（需迁移）
```

预发布版本追加后缀：

```
v1.2.3-alpha.1
v1.2.3-beta.2
v1.2.3-rc.1
```

### 2.2 版本信息注入

#### 2.2.1 版本包定义

新增 `internal/version/version.go`：

```go
package version

var (
    Version   = "dev"
    Commit    = "none"
    BuildTime = "unknown"
    GoVersion = "unknown"
    Platform  = "unknown"
)

func String() string {
    return Version
}

func FullString() string {
    return Version + " (" + Commit + ", " + BuildTime + ")"
}
```

#### 2.2.2 通过 ldflags 注入

构建时通过 `-ldflags` 注入版本信息：

```bash
VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse HEAD)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
GO_VERSION=$(go version | awk '{print $3}')
PLATFORM=$(go env GOOS)/$(go env GOARCH)

go build -ldflags "-s -w \
  -X cs-cloud/internal/version.Version=${VERSION} \
  -X cs-cloud/internal/version.Commit=${COMMIT} \
  -X cs-cloud/internal/version.BuildTime=${BUILD_TIME} \
  -X cs-cloud/internal/version.GoVersion=${GO_VERSION} \
  -X cs-cloud/internal/version.Platform=${PLATFORM}" \
  -o bin/cs-cloud ./cmd/cs-cloud
```

#### 2.2.3 版本信息集成点

| 集成点 | 用途 |
|--------|------|
| `cs-cloud version` 命令 | CLI 输出版本信息 |
| `cs-cloud doctor` 命令 | 诊断信息中包含版本 |
| daemon 启动日志 | 记录启动版本 |
| `/api/v1/runtime/health` | API 返回版本号 |
| WebSocket tunnel 握手 | 上报设备版本 |
| 升级检查请求 | 携带当前版本 |

### 2.3 Git 标签与分支策略

```
main (稳定分支)
 └── v1.2.3 tag ← Release
dev (开发分支)
 └── feature/xxx
 └── fix/xxx
```

- **main**：稳定版本，所有 Release 从 main 打 tag
- **dev**：日常开发集成分支
- **feature/***：功能分支，合并到 dev
- **fix/***：修复分支，合并到 dev，必要时 cherry-pick 到 main

### 2.4 版本号自动生成

使用 `git describe` 自动推导版本号：

| 场景 | `git describe` 输出 | 含义 |
|------|---------------------|------|
| 正好打 tag | `v1.2.3` | 正式发布 |
| tag 后 3 个 commit | `v1.2.3-3-gabcdef` | 开发中 |
| 无 tag | `abcdef` | 初始开发 |

Release 构建时确保输出为纯 tag（无 `-N-gXXXX` 后缀）。

---

## 3. 构建与发布流程

### 3.1 构建矩阵

| OS | Architecture | 二进制名 |
|----|-------------|---------|
| linux | amd64 | cs-cloud-linux-amd64 |
| linux | arm64 | cs-cloud-linux-arm64 |
| windows | amd64 | cs-cloud-windows-amd64.exe |
| darwin | amd64 | cs-cloud-darwin-amd64 |
| darwin | arm64 | cs-cloud-darwin-arm64 |

### 3.2 Release 产物

每个 Release 包含：

```
dist/
├── cs-cloud-linux-amd64
├── cs-cloud-linux-amd64.sha256
├── cs-cloud-linux-arm64
├── cs-cloud-linux-arm64.sha256
├── cs-cloud-windows-amd64.exe
├── cs-cloud-windows-amd64.exe.sha256
├── cs-cloud-darwin-amd64
├── cs-cloud-darwin-amd64.sha256
├── cs-cloud-darwin-arm64
├── cs-cloud-darwin-arm64.sha256
└── checksums.txt
```

`.sha256` 文件格式：

```
<sha256_hex>  cs-cloud-linux-amd64
```

### 3.3 CI/CD 流水线（GitHub Actions）

```
Push Tag (v*)          Push to main/dev
    │                        │
    ▼                        ▼
┌──────────┐          ┌──────────────┐
│  Test    │          │    Test      │
└────┬─────┘          └──────┬───────┘
     │                       │
     ▼                       ▼
┌──────────┐          ┌──────────────┐
│  Build   │          │    Build     │
│ (matrix) │          │  (snapshot)  │
└────┬─────┘          └──────────────┘
     │
     ▼
┌──────────┐
│ Checksum │
└────┬─────┘
     │
     ▼
┌──────────┐
│  Sign    │  (可选：GPG / cosign 签名)
└────┬─────┘
     │
     ▼
┌──────────┐
│ Release  │  → GitHub Release + 资产上传
└────┬─────┘
     │
     ▼
┌──────────┐
│  Notify  │  → Webhook / 云端版本 API 更新
└──────────┘
```

### 3.4 GoReleaser 配置（.goreleaser.yml）

```yaml
project_name: cs-cloud

before:
  hooks:
    - go mod tidy

builds:
  - main: ./cmd/cs-cloud
    binary: cs-cloud
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X cs-cloud/internal/version.Version={{.Version}}
      - -X cs-cloud/internal/version.Commit={{.Commit}}
      - -X cs-cloud/internal/version.BuildTime={{.Date}}
      - -X cs-cloud/internal/version.GoVersion={{.Env.GO_VERSION}}
    goos:
      - linux
      - windows
      - darwin
    goarch:
      - amd64
      - arm64

checksum:
  name_template: checksums.txt
  algorithm: sha256

archives:
  - format: binary
    name_template: "{{ .ProjectName }}-{{ .Os }}-{{ .Arch }}"

release:
  github:
    owner: <org>
    name: cs-cloud
```

---

## 4. 自动升级设计

### 4.1 架构概览

```
┌─────────────────────────────────────────────────────┐
│                    cs-cloud daemon                   │
│                                                     │
│  ┌─────────────┐   ┌──────────────┐                │
│  │   Updater    │──▶│ Upgrade Mgr  │                │
│  │  (checker)   │   │  (executor)  │                │
│  └──────┬──────┘   └──────┬───────┘                │
│         │                 │                         │
└─────────┼─────────────────┼─────────────────────────┘
          │                 │
          ▼                 ▼
   ┌──────────────┐  ┌──────────────┐
   │ Cloud API    │  │ 下载 + 校验  │
   │ /api/update  │  │ + 原子替换   │
   └──────────────┘  └──────────────┘
```

### 4.2 版本检查 API

#### 请求

```
GET /cloud-api/api/updates/check?platform={os}-{arch}&version={current_version}
```

#### 响应

```json
{
  "available": true,
  "version": "v1.3.0",
  "changelog": "修复 WebSocket 重连问题，新增资源监控",
  "download_url": "https://releases.example.com/cs-cloud/v1.3.0/cs-cloud-linux-amd64",
  "sha256": "e3b0c44298fc1c149afbf4c8996fb924...",
  "sha256_url": "https://releases.example.com/cs-cloud/v1.3.0/cs-cloud-linux-amd64.sha256",
  "signature": "MEUCIQDx...",
  "min_client_version": "v1.0.0",
  "release_date": "2026-04-10T00:00:00Z",
  "force": false,
  "metadata": {
    "size": 12345678,
    "go_version": "go1.25.0"
  }
}
```

| 字段 | 说明 |
|------|------|
| `available` | 是否有可用更新 |
| `force` | 是否强制升级（安全补丁等） |
| `min_client_version` | 最低兼容客户端版本，低于此版本必须升级 |
| `signature` | 可选的二进制签名（cosign / GPG） |

#### 无更新响应

```json
{
  "available": false,
  "version": "v1.2.3"
}
```

### 4.3 升级流程

```
                    ┌─────────────┐
                    │  定时检查    │
                    │ (每 6 小时)  │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐      无更新
                    │ 查询版本 API │────────────▶ 等待下次检查
                    └──────┬──────┘
                           │ 有更新
                           ▼
                    ┌─────────────┐
                    │ 比较版本号   │
                    │ (SemVer)    │
                    └──────┬──────┘
                           │ 新版本
                           ▼
                    ┌─────────────┐      用户取消或
                    │ 策略判断     │      非强制升级
                    │ (force/策略) │◀─────────────┘
                    └──────┬──────┘
                           │ 允许升级
                           ▼
                    ┌─────────────┐
                    │ 下载新二进制 │
                    │ 到临时目录   │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐     校验失败
                    │ SHA256 校验  │────────────▶ 清理 + 等待重试
                    └──────┬──────┘
                           │ 通过
                           ▼
                    ┌─────────────┐
                    │ 可选：签名   │     签名失败
                    │ 校验        │────────────▶ 清理 + 等待重试
                    └──────┬──────┘
                           │ 通过
                           ▼
                    ┌─────────────┐
                    │ 备份当前    │
                    │ 二进制      │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ 原子替换    │
                    │ (平台相关)  │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ 优雅重启    │
                    │ daemon      │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ 启动验证    │
                    │ (健康检查)  │
                    └──────┬──────┘
                           │
                    ┌──────┴──────┐
                    │             │
                 成功 ▼         失败 ▼
            ┌──────────┐  ┌──────────────┐
            │ 清理旧备份 │  │ 自动回滚     │
            │ 上报版本   │  │ 恢复旧二进制  │
            └──────────┘  │ 重启 daemon  │
                          └──────────────┘
```

### 4.4 原子替换策略（平台差异）

#### Linux / macOS

```go
// 直接 rename，UNIX 下 unlink 后 inode 仍有效，
// 运行中的进程不受影响，下次启动使用新文件
os.Rename(newBinary, currentBinary)
```

#### Windows

Windows 下运行中的 exe 文件被锁定，不能直接覆盖。采用分阶段替换：

```go
// Phase 1: 将新二进制写入临时名称
os.Rename(newBinary, currentBinary + ".new")

// Phase 2: 写入升级脚本（batch）
// upgrade.bat:
//   @echo off
//   :retry
//   taskkill /PID <pid> /F
//   timeout /t 1 /nobreak >nul
//   move /Y "cs-cloud.exe.new" "cs-cloud.exe"
//   start cs-cloud.exe _daemon
//

// Phase 3: daemon 收到升级信号后优雅退出
// Phase 4: 升级脚本完成替换并重启
```

更好的方案（推荐）：使用 **rollback wrapper** 模式：

```go
// 将当前 exe 重命名为 cs-cloud.exe.old
// 将新文件重命名为 cs-cloud.exe
// 在 daemon 退出后由父进程或系统服务完成切换
//
// Windows Service 场景下可直接利用 Service Manager
// 的 Stop → 替换 → Start 序列
```

### 4.5 模块设计

#### 新增文件结构

```
internal/
├── version/
│   └── version.go          # 版本信息定义
├── updater/
│   ├── checker.go          # 版本检查逻辑
│   ├── downloader.go       # 下载管理
│   ├── verifier.go         # SHA256 + 签名校验
│   ├── replacer.go         # 平台相关二进制替换
│   ├── manager.go          # 升级流程编排
│   └── manager_test.go
```

#### 核心接口

```go
// internal/updater/checker.go
type CheckResult struct {
    Available   bool
    Version     string
    DownloadURL string
    SHA256      string
    Force       bool
    Changelog   string
}

type Checker struct {
    baseURL    string
    httpClient *http.Client
}

func (c *Checker) Check(ctx context.Context, platform, currentVersion string) (*CheckResult, error)

// internal/updater/manager.go
type Manager struct {
    checker   *Checker
    verifier  *Verifier
    replacer  *Replacer
    currentExe string
    backupDir  string
}

type UpgradePolicy int

const (
    PolicyAuto    UpgradePolicy = iota // 自动下载并升级
    PolicyDownload                     // 仅下载，手动确认后升级
    PolicyManual                       // 仅通知，手动操作
)

func (m *Manager) Run(ctx context.Context, policy UpgradePolicy) error
func (m *Manager) Rollback() error
```

#### 定时检查集成

在 daemon 启动时注册定时检查：

```go
// internal/cli/daemon.go 中集成
func runDaemon(a *app.App) error {
    // ... 现有启动逻辑 ...

    if mode == "cloud" {
        // ... 现有 tunnel 连接 ...

        // 启动升级检查
        updater := updater.NewManager(
            updater.WithCheckInterval(6 * time.Hour),
            updater.WithPolicy(updater.PolicyAuto),
            updater.WithCloudBaseURL(a.CloudBaseURL()),
        )
        go updater.Run(ctx)
    }
}
```

### 4.6 回滚机制

```
~/.cs-cloud/
├── upgrades/
│   ├── current.json        # 当前升级状态
│   ├── cs-cloud.bak        # 上一版本备份
│   └── rollback.json       # 回滚信息
```

`current.json`:

```json
{
  "previous_version": "v1.2.2",
  "current_version": "v1.2.3",
  "upgraded_at": "2026-04-10T12:00:00Z",
  "backup_path": "~/.cs-cloud/upgrades/cs-cloud.bak",
  "status": "completed",
  "verified": true
}
```

启动验证流程：
1. daemon 启动后检查 `current.json` 中的 `status`
2. 若为 `pending_verify`，执行自检（健康探针 + tunnel 连通性）
3. 验证通过 → 更新 status 为 `completed`
4. 验证失败 → 触发 `Rollback()`

### 4.7 升级策略配置

通过配置文件或环境变量控制：

```json
// ~/.cs-cloud/config.json
{
  "update": {
    "enabled": true,
    "policy": "auto",
    "check_interval": "6h",
    "channel": "stable",
    "allowed_major_versions": ["1"]
  }
}
```

| 策略 | 说明 |
|------|------|
| `auto` | 自动下载、替换、重启（推荐） |
| `download` | 自动下载，提示用户确认后升级 |
| `manual` | 仅通知有新版本，由用户手动升级 |

### 4.8 CLI 命令扩展

```bash
# 检查更新
cs-cloud update check

# 手动触发升级
cs-cloud update apply [--version v1.3.0]

# 回滚到上一版本
cs-cloud update rollback

# 查看升级历史
cs-cloud update history
```

---

## 5. 安全考虑

### 5.1 传输安全

- 所有下载必须通过 **HTTPS**
- SHA256 校验确保下载完整性
- 可选：cosign / GPG 签名验证二进制来源

### 5.2 升级源验证

```go
type Verifier struct {
    trustedKeys []string // 可信签名公钥
}

func (v *Verifier) Verify(binary []byte, checksum string, signature string) error {
    // Step 1: SHA256 校验
    // Step 2: 签名验证（如果启用）
}
```

### 5.3 降级保护

- 不允许降级到比 `min_client_version` 更低的版本
- 不允许跨 major 版本自动升级（需要用户显式确认）
- 记录升级审计日志

---

## 6. 实施路线

### Phase 1: 版本基础（1-2 天）

- [ ] 新增 `internal/version` 包
- [ ] 添加 `cs-cloud version` 命令
- [ ] 在 `doctor` 和 `/api/v1/runtime/health` 中集成版本信息
- [ ] 添加 Makefile（build / test / lint）
- [ ] 配置 ldflags 构建注入

### Phase 2: 构建与发布（2-3 天）

- [ ] 配置 GoReleaser
- [ ] 创建 GitHub Actions CI/CD 流水线
- [ ] 建立 tag → Release 自动化流程
- [ ] SHA256 checksum 自动生成

### Phase 3: 自动升级核心（3-5 天）

- [ ] 实现 `internal/updater` 包
  - [ ] `checker.go` - 版本检查
  - [ ] `downloader.go` - 带进度下载
  - [ ] `verifier.go` - SHA256 校验
  - [ ] `replacer.go` - 跨平台二进制替换
  - [ ] `manager.go` - 流程编排
- [ ] 集成到 daemon 生命周期
- [ ] 实现回滚机制
- [ ] 添加 `cs-cloud update` 子命令

### Phase 4: 云端对接（2-3 天）

- [ ] 云端版本检查 API 实现
- [ ] Release 产物 CDN 分发
- [ ] 设备版本上报到云端
- [ ] 云端升级策略下发（force / channel）

### Phase 5: 稳定化（2-3 天）

- [ ] 端到端升级测试（各平台）
- [ ] 失败场景测试（网络中断、校验失败、磁盘满）
- [ ] 回滚测试
- [ ] 监控与告警集成

---

## 7. 备选方案对比

### 7.1 升级实现方案

| 方案 | 优点 | 缺点 | 推荐度 |
|------|------|------|--------|
| **自实现（本文提案）** | 完全可控，轻量 | 需处理平台差异 | ★★★★★ |
| go-selfupdate 库 | 快速集成 | 灵活性不足，依赖 GitHub Release | ★★★ |
| Goa dell <-> Ota | 成熟方案 | 引入重依赖，过于复杂 | ★★ |
| 系统包管理器（apt/yum） | 用户熟悉 | 需要维护各平台仓库 | ★★★（长期） |

### 7.2 分发渠道

| 渠道 | 适用场景 |
|------|---------|
| GitHub Release | 开源项目、默认渠道 |
| 自建 CDN / OSS | 企业级分发、内网环境 |
| 云端 API 代理 | 受控设备环境 |

推荐：**以 GitHub Release 为主，支持自建 CDN 作为 mirror**。`download_url` 由云端 API 返回，可灵活切换分发源。

---

## 8. 监控与可观测性

升级过程需记录关键事件到日志和云端：

| 事件 | 级别 | 说明 |
|------|------|------|
| `upgrade_check` | Info | 定时检查触发 |
| `upgrade_available` | Info | 发现新版本 |
| `upgrade_download_start` | Info | 开始下载 |
| `upgrade_download_complete` | Info | 下载完成 |
| `upgrade_verify_pass` | Info | 校验通过 |
| `upgrade_verify_fail` | Error | 校验失败 |
| `upgrade_replace_start` | Warn | 开始替换二进制 |
| `upgrade_replace_complete` | Info | 替换完成 |
| `upgrade_restart` | Warn | 重启 daemon |
| `upgrade_rollback` | Error | 升级失败回滚 |
| `upgrade_success` | Info | 升级成功并验证 |
