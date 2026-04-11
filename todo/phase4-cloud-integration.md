# Phase 4 - 云端对接

> 提案参考：[docs/versioning-and-upgrade.md](../docs/versioning-and-upgrade.md) §4.2 版本检查 API、§4.3 升级流程

**预计耗时**：2-3 天
**前置条件**：Phase 2 完成（Release 产物可下载），Phase 3 完成（updater 包可用）
**状态**：`done`

---

## 任务清单

### 4.1 云端版本检查 API

- [x] `updater/checker.go` 已实现 `GET /cloud-api/api/updates/check?platform=&version=`
- [x] 响应解析：`available`, `version`, `download_url`, `sha256`, `force`, `changelog`
- [ ] 云端 API 端点实现（独立服务，不在本项目中）

### 4.2 Release 产物分发

- [x] `checker.go` 使用 API 返回的 `download_url` 下载
- [x] `downloader.go` 流式 SHA256 校验 + 文件完整性验证
- [ ] 云端分发源配置（CDN / GitHub Release URL）

### 4.3 设备版本上报

- [x] WebSocket tunnel 握手 URL 携带 `client_version` 参数
  - 文件：`internal/tunnel/connect.go`
- [x] 设备注册请求中 `version` 字段使用 `version.Get()`（替换硬编码 `"dev"`）
  - 文件：`internal/device/client.go`
- [x] gateway-assign 请求中 `version` 字段使用 `version.Get()`
  - 文件：`internal/device/gateway.go`
- [x] 实现 `Client.Heartbeat()` 方法，POST 到 `/api/devices/{id}/heartbeat` 携带 version
  - 文件：`internal/device/token.go`

### 4.4 云端升级策略下发

- [x] `CheckResult` 结构体已包含 `Force` 字段
- [x] `manager.go` 根据 `force` 标记决定是否强制升级
- [ ] 云端策略配置界面（独立服务）
- [ ] 云端灰度发布机制（独立服务）

---

## 验收标准

- [x] tunnel WebSocket 握手 URL 包含 `client_version` 参数
- [x] 设备注册、gateway-assign、心跳均携带实际版本号
- [x] `updater/checker.go` 已适配云端 API 响应格式
- [x] 全平台交叉编译通过
- [ ] 云端 API 端点部署上线（独立服务）
- [ ] 支持强制升级标记下发（需云端配合）

## 产出文件

| 文件 | 操作 |
|------|------|
| `internal/tunnel/connect.go` | 修改（握手携带 `client_version`） |
| `internal/device/client.go` | 修改（注册使用 `version.Get()`） |
| `internal/device/gateway.go` | 修改（gateway-assign 使用 `version.Get()`） |
| `internal/device/token.go` | 修改（实现 `Heartbeat()` 方法） |

## 备注

云端 API 端点（`/api/updates/check`、`/api/devices/{id}/heartbeat`）为独立服务，需后端团队实现。
设备端改动已全部完成，版本上报和升级检查机制已就绪，等待云端 API 对接即可启用。
