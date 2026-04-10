# Runtime

## 已实现的基础配置行为

当前 `cs-cloud` 已补充一部分与 `opencode` 的 `cs cloud` 模式对齐的基础能力。

### auth.json 读取

默认从以下位置读取认证信息：

```text
~/.costrict/share/auth.json
```

结构兼容字段：

- `id`
- `name`
- `access_token`
- `refresh_token`
- `state`
- `machine_id`
- `base_url`
- `expiry_date`
- `updated_at`
- `expired_at`

其中最小有效条件为：

- `access_token` 非空
- `base_url` 非空

如果文件不存在，返回 `nil`，不视为错误。

如果 JSON 损坏，当前实现返回 `nil`，后续可增强为结构化错误。

### Cloud Base URL 解析

当前解析函数位于：

- `internal/device/client.go`

行为与 `opencode` 的 `getCloudBaseUrl()` 对齐：

优先级：

1. `COSTRICT_CLOUD_BASE_URL`
2. credentials `base_url`
3. `COSTRICT_BASE_URL`
4. 默认值 `https://zgsm.sangfor.com`

补充逻辑：

- 如果地址已经包含 `/cloud-api`，则直接返回
- 如果显式设置了 `COSTRICT_CLOUD_BASE_URL`，则直接返回，不自动追加 `/cloud-api`
- 其它情况默认自动追加 `/cloud-api`

### 当前 CLI 暴露信息

`status` 和 `doctor` 已输出：

- 运行状态
- `cloud_base_url`
- `auth_json_loaded`
- `credential_base_url`
- `machine_id`
- `device_registered`
- `device_id`
- `device_base_url`

这可以作为后续调试 device register / tunnel / control plane 的前置自检信息。

### device.json 持久化

当前最小注册流程已实现：

- 优先读取本地 `device.json`
- 若已存在则直接复用
- 若环境变量覆盖导致 base url 变化，则自动更新本地 `device.json`
- 若不存在，则使用 `auth.json` 中的 `access_token` 发起注册请求

默认文件位置：

```text
~/.costrict/share/device.json
```

当前结构：

- `device_id`
- `device_token`
- `registered_at`
- `base_url`

### Local Control Plane

当前 `start` 命令已会拉起一个最小本地 HTTP server，并把地址写入：

```text
~/.cs-cloud/server_url
```

当前行为：

- `start`：启动后台 `serve` 进程
- `serve`：实际监听 `127.0.0.1:0`
- `stop`：结束后台进程，并清理 `server_url`
- `status` / `doctor`：读取并输出 `local_server_url`

当前已提供路由：

- `GET /health` -> `{"status":"ok"}`
- `GET /agents` -> `{"agents":[]}`

这是后续接入 ACP runtime、session manager、event stream 的基础入口。
