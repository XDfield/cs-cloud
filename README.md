# cs-cloud

Go 单二进制设备端 cloud daemon 脚手架。

## 目标

- 跨平台
- 静态编译
- 单包运行
- 低内存占用
- 支持 ACP runtime 接入

## 当前结构

- `cmd/cs-cloud`: 程序入口
- `internal/app`: 应用装配
- `internal/cli`: CLI 命令
- `internal/config`: 配置加载
- `internal/device`: 设备注册/心跳/网关验证
- `internal/provider`: OAuth 登录/凭证/Token 管理
- `internal/tunnel`: tunnel / reconnect
- `internal/localserver`: 本地 control plane
- `internal/runtime`: runtime 管理
- `internal/acp`: ACP 桥接
- `internal/model`: 领域模型
- `internal/platform`: 平台差异处理

## Cloud 配置

当前已支持参考 `opencode` 的 `cs cloud start` 模式读取云端认证与地址配置。

### 1. auth.json

默认读取：

```text
~/.costrict/share/auth.json
```

当前已支持读取字段：

- `access_token`
- `refresh_token`
- `state`
- `machine_id`
- `base_url`
- `expiry_date`
- `updated_at`
- `expired_at`

最小要求：

- `access_token`
- `base_url`

### 2. Cloud Base URL 优先级

当前 `cloud_base_url` 解析规则：

1. `COSTRICT_CLOUD_BASE_URL`
2. `auth.json` 中的 `base_url`
3. `COSTRICT_BASE_URL`
4. 默认值 `https://zgsm.sangfor.com`

补充规则：

- 如果结果已经以 `/cloud-api` 结尾，则直接使用
- 如果显式设置了 `COSTRICT_CLOUD_BASE_URL`，则不自动追加 `/cloud-api`
- 否则自动补成 `${base}/cloud-api`

### 3. CLI 检查

当前可通过以下命令检查是否读取成功：

```bash
go run ./cmd/cs-cloud status
go run ./cmd/cs-cloud doctor
```

输出会包含：

- `cloud_base_url`
- `auth_json_loaded`
- `credential_base_url`
- `machine_id`
- `device_registered`
- `device_id`
- `device_base_url`

### 4. device.json

设备注册成功后会写入：

```text
~/.costrict/share/device.json
```

当前保存字段：

- `device_id`
- `device_token`
- `registered_at`
- `base_url`

可以通过以下命令触发最小注册流程：

```bash
go run ./cmd/cs-cloud register
```

## 登录认证流程

完整流程参考 opencode 的 `cs cloud start` 模式。

### 1. 登录

```bash
go run ./cmd/cs-cloud login
```

流程：
1. 生成 `state` + `machine_id`
2. 构建登录 URL（含 OAuth 参数：`provider=casdoor`, `machine_code`, `state`）
3. 打开浏览器到 CoStrict 登录页
4. 轮询 Token 端点（`/oidc-auth/api/v1/plugin/login/token`），最长 10 分钟
5. 获取 `access_token` + `refresh_token`
6. 保存到 `~/.costrict/share/auth.json`
7. 自动触发设备注册

### 2. 启动（含自动登录）

```bash
go run ./cmd/cs-cloud start
```

流程：
1. 检查是否已在运行
2. 尝试设备注册（`/api/devices/register`）
3. 如果认证缺失/过期 → 自动触发浏览器登录
4. 登录完成后重试注册
5. 验证设备 Token（`/cloud/device/gateway-assign`）
6. 如果 Token 无效 → 清除本地设备 → 重新注册 → 重新验证
7. 启动本地 server

### 3. Token 刷新

注册时如果 `access_token` 过期（三层策略）：
1. `expiry_date`（30 分钟缓冲）
2. `refresh_token` JWT 解析
3. `access_token` JWT 解析（30 分钟缓冲）

刷新使用 `refresh_token` 调用 `/oidc-auth/api/v1/plugin/login/token`（不含 `machine_code`）。

### 4. 登出

```bash
go run ./cmd/cs-cloud logout
```

删除 `auth.json` 和 `device.json`。

## Local Server

当前 `start` 已经会拉起一个最小本地 control plane server。

### 生命周期

- `go run ./cmd/cs-cloud start`：启动后台本地 server（含登录+注册）
- `go run ./cmd/cs-cloud serve`：前台运行本地 server
- `go run ./cmd/cs-cloud stop`：停止后台本地 server
- `go run ./cmd/cs-cloud status`：查看本地 server 地址
- `go run ./cmd/cs-cloud doctor`：输出更完整诊断信息

### 当前已提供的接口

- `GET /health`
- `GET /agents`

示例：

```bash
go run ./cmd/cs-cloud start
go run ./cmd/cs-cloud status
curl http://127.0.0.1:PORT/health
curl http://127.0.0.1:PORT/agents
```

### 当前限制

当前 local server 仍是最小版本：

- 尚未接入 ACP runtime
- `/agents` 仅返回空数组占位
- 尚未实现 `/sessions`、`/events`、`/permissions`
