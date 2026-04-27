# Runtime Find File Performance Proposal

## 背景

当前 `cs-cloud` 的 `GET /api/v1/runtime/find/file` 由 `internal/localserver/find_files.go` 处理。

该接口的 workspace 通过请求头 `X-Workspace-Directory` 指定，因此在 `cs-cloud` 进程中，请求可能实际搜索的是另一个工程目录，例如：

- `cs-cloud` 作为服务进程运行
- `/runtime/find/file` 的目标 workspace 为 `D:\DEV\opencode`

在实际观察中，该接口对大型 workspace 的首次请求耗时可接近 5s，而后续同类请求明显更快。

## 当前实现

当前实现流程如下：

1. 从 query 中读取：
   - `query`
   - `dirs`
   - `limit`
   - `directory`
2. 通过 `resolvePath()` 将 `directory` 约束到 workspace 下。
3. 对目标目录执行一次 `filepath.WalkDir(absDir, ...)`。
4. 在遍历过程中按如下规则匹配：
   - 若 query 不含 `/`，则匹配 basename
   - 若 query 含 `/`，则匹配相对路径
   - `dirs=false` 时过滤目录结果
5. 返回绝对路径数组。

### 当前实现特点

- **每次请求都触发一次完整递归遍历**
- **没有 workspace 级内存索引或短时缓存**
- **limit 达标后没有真正终止整个遍历**
- **忽略规则只对部分顶层目录生效**

## 性能问题分析

### 1. 主要耗时在文件系统递归遍历

对于大型 monorepo（如 `opencode`），`filepath.WalkDir` 的主要成本包括：

- 目录枚举
- 文件元数据访问
- 相对路径计算
- 字符串归一化与匹配

在 Windows 环境下，这类递归遍历的成本通常更高，且容易受到以下因素影响：

- NTFS 元数据缓存状态
- 防病毒/索引程序
- 冷缓存下的磁盘访问

### 2. “首查慢、后查快”主要来自 OS 缓存，而非应用缓存

当前实现中不存在应用层索引缓存，因此同一 workspace 下后续请求变快，主要应解释为：

- 文件系统目录项缓存变热
- OS 元数据缓存命中率提高

这意味着当前性能改善是偶然的、不可控的，不应视为系统设计能力。

### 3. 忽略规则不足

当前 `shouldSkip()` 只检查相对路径的首段目录名。这会导致很多嵌套的高成本目录无法被剪枝，例如：

- `packages/*/node_modules`
- `apps/*/dist`
- `subprojects/*/build`
- `packages/*/.next`

这在 monorepo 中会显著放大遍历量。

### 4. limit 无法有效阻止全局遍历

当前在 callback 中返回 `filepath.SkipDir` 的方式，只能跳过当前目录，不能作为“全局停止遍历”的可靠机制。因此即使 `limit=10`，仍可能扫描大量无关目录。

## 与 opencode 方案对比

`opencode` 的 `find/file` 方案与 `cs-cloud` 当前实现存在本质差异。

### cs-cloud 当前方案：实时遍历型

特点：

- 每次请求都递归扫描文件系统
- 无索引、无缓存
- 结果天然最新
- 实现简单

优点：

- 一致性简单
- 内存占用低
- 不需要处理缓存失效

缺点：

- 大 workspace 场景下延迟高
- 高频搜索体验差
- 成本与目录规模线性相关

### opencode 方案：索引后查询型

特点：

- 先扫描 workspace，构建 `files` / `dirs` 候选集
- 查询时只在内存中做匹配
- 搜索阶段不再触盘

优点：

- 首次之后搜索非常快
- 非常适合连续输入、多次搜索、自动补全
- 可在内存中灵活做排序和匹配策略

缺点：

- 首次建索引成本较高
- 需要缓存失效策略
- 实现复杂度更高

## 可借鉴点

本提案认为，`cs-cloud` **不应直接照搬 `opencode` 的完整实现**，但应借鉴其核心思想：

### 1. 将“扫描”和“查询”解耦

建议不要在每次 `/runtime/find/file` 请求中同时承担：

- 全量扫描
- 查询匹配

而应将其拆分为：

- 扫描阶段：构建 workspace 候选索引
- 查询阶段：对内存中的候选项做过滤与排序

### 2. 建立 workspace 级短时缓存

建议引入轻量级缓存，而不是完整 watcher/事件驱动索引系统。

可缓存内容：

- `files []string`
- `dirs []string`
- `builtAt time.Time`

缓存 key：

- workspace 绝对路径
- 可选再加 `directory` 子树路径（若后续支持局部索引）

### 3. 查询时只在缓存上匹配

当缓存有效时：

- `dirs=false`：只对 `files` 搜索
- `dirs=true`：对 `files + dirs` 搜索

这样搜索延迟将主要取决于字符串匹配，而非磁盘遍历。

### 4. 忽略规则系统化

应借鉴 `opencode` 中“集中管理忽略模式”的思路，将 skip 规则从“少量顶层目录 map”演进为：

- 可配置
- 集中定义
- 对任意层级目录生效

建议纳入的默认忽略目录至少包括：

- `node_modules`
- `.git`
- `.svn`
- `.hg`
- `dist`
- `build`
- `target`
- `.next`
- `.nuxt`
- `.cache`
- `.turbo`
- `coverage`
- `.venv`
- `venv`
- `__pycache__`
- `.idea`
- `.sst`

## 推荐方案

本提案建议采用 **“轻量索引缓存 + 遍历优化”** 的中间路线，而不是继续完全依赖 `WalkDir`，也不是一步到位引入完整复杂索引系统。

### 方案概述

为 `/runtime/find/file` 引入以下能力：

1. **遍历优化（立即收益）**
   - skip 规则对任意层级目录生效
   - 达到 `limit` 后可真正终止遍历

2. **workspace 级短时索引缓存（核心收益）**
   - 首次扫描 workspace，构建 `files` / `dirs`
   - 在 TTL 内复用
   - 过期后按需重建

3. **内存查询（体验收益）**
   - 查询阶段不再触盘
   - 支持更好的排序策略

## 建议的数据结构

```go
type FileSearchIndex struct {
    Workspace string
    Files     []string
    Dirs      []string
    BuiltAt   time.Time
}

type FileSearchIndexStore struct {
    mu      sync.RWMutex
    entries map[string]*FileSearchIndex
    ttl     time.Duration
}
```

### 说明

- `Workspace` 使用绝对路径，避免相对路径歧义
- `Files` / `Dirs` 建议存储为相对 workspace 的标准化路径
- 返回响应时再拼装绝对路径，保持兼容当前 API 输出

## 建议的请求处理流程

### 命中缓存

1. resolve workspace
2. 查找 workspace 对应索引
3. 若索引存在且未过期：
   - 从内存取候选集
   - 执行字符串匹配
   - 返回结果

### 未命中缓存

1. resolve workspace
2. 触发一次扫描构建索引
3. 写入缓存
4. 立即在新索引上查询

## 匹配策略建议

初期不建议引入复杂 fuzzy search，可先采用轻量策略：

1. basename contains 优先
2. 相对路径 contains 次之
3. 路径更短优先
4. 目录层级更浅优先

这样可以在保持实现简单的前提下，显著优于当前“遍历顺序即结果顺序”的行为。

## 缓存一致性策略

初期建议采用 **TTL 驱动的一致性模型**：

- TTL：30s 或 60s
- 在 TTL 内允许轻微陈旧
- TTL 过期后重建

理由：

- `/runtime/find/file` 多用于交互式搜索和候选定位
- 对完全实时一致性的要求通常低于文件内容读取接口
- 相比 watcher 方案，TTL 实现成本更低、失败面更小

后续如确有需要，可再演进为：

- watcher 失效
- 主动预热
- 后台异步刷新

## 分阶段落地建议

### 阶段 1：最小性能修复

目标：在不引入缓存的情况下，先把最明显的遍历浪费去掉。

改动：

1. `shouldSkip()` 改为任意层级命中即跳过
2. 使用 sentinel error 在 `limit` 达标后终止整个 `WalkDir`
3. 增加耗时日志与扫描计数日志

收益：

- 对大仓库立即有效
- 风险低
- 可快速验证性能改善幅度

### 阶段 2：轻量索引缓存

目标：借鉴 `opencode` 的“扫描后查询”思路，把搜索从磁盘 IO 转为内存匹配。

改动：

1. 增加 workspace -> index 缓存
2. 首次或过期时构建 `files` / `dirs`
3. 查询阶段改为只在缓存中匹配

收益：

- 连续搜索延迟显著下降
- “首查慢、后查快”从 OS 偶然现象变成系统能力

### 阶段 3：体验优化

改动：

1. 更好的排序策略
2. 可选预热
3. 可选 watcher 失效

收益：

- 搜索结果质量提升
- 与 UI 自动补全场景更加契合

## 风险与权衡

### 1. 缓存过期带来的短暂陈旧

引入 TTL 缓存后，新增/删除文件可能不会立刻反映到搜索结果中。

权衡：

- 对搜索接口通常可接受
- 通过较短 TTL 可降低影响

### 2. 内存占用增加

大型 workspace 的 `files` / `dirs` 索引会占用一定内存。

权衡：

- 可限制缓存条目数量
- 仅缓存近期 workspace
- 过期后淘汰

### 3. 并发构建重复索引

多个请求同时命中同一 workspace 冷缓存时，可能重复触发扫描。

建议：

- 对每个 workspace 增加单飞保护（singleflight）
- 避免重复构建

## 推荐结论

本提案建议 `cs-cloud` 对 `/runtime/find/file` 采用以下演进方向：

1. **短期**：修复当前 `WalkDir` 遍历策略
   - 任意层级 skip
   - limit 达标后全局终止

2. **中期**：借鉴 `opencode` 的核心思想，引入 **workspace 级短时索引缓存**
   - 扫描与查询分离
   - 查询走内存
   - 保持实现轻量

3. **长期**：按需要逐步增加预热、排序优化、watcher 失效等能力

该路线能够在控制实现复杂度的同时，显著改善大型 workspace 的文件搜索体验，并为后续 UI 高频搜索场景提供稳定基础。
