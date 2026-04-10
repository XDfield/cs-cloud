# CLI 交互增强提案

## 背景

当前 `internal/cli/` 使用原生 `fmt` 输出，体验较为简陋。需要在不引入复杂 TUI 架构的前提下，提升启动信息展示和用户交互体验。

## 需求范围

- 启动时美化信息输出（颜色、图标、对齐）
- 支持用户选择/多选启用的 AI Agent 列表

## 选型

引入 [Charmbracelet](https://charm.sh/) 生态的两个库：

| 库 | 用途 | 引入 bubbletea |
|---|---|---|
| [lipgloss v2](https://github.com/charmbracelet/lipgloss) | 终端样式定义（颜色、边框、对齐） | 否，独立使用 |
| [huh v2](https://github.com/charmbracelet/huh) | 交互式表单（Select、MultiSelect、Confirm、Input） | 间接依赖，无需直接使用 bubbletea API |

### 不引入的库

- **bubbletea** — 完整 TUI 框架，需要自行管理 Model/Update/View 循环，对本项目场景过重
- **bubbles** — bubbletea 组件库，huh 已封装所需组件
- **survey / promptui / go-prompt** — 均已停更，huh 是其现代替代

## 使用方式

### lipgloss — 美化启动输出

替换现有 `fmt.Printf` 为声明式样式定义：

```go
import "github.com/charmbracelet/lipgloss/v2"

var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#7D56F4")).
        MarginBottom(1)

    infoStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#6B6B6B"))

    successStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#04B575"))
)

fmt.Println(titleStyle.Render("cs-cloud"))
fmt.Println(successStyle.Render("  ✓ Started"))
fmt.Println(infoStyle.Render("  root: /path/to/dir"))
```

### huh — Agent 选择/多选

```go
import "github.com/charmbracelet/huh/v2"

var agents []string

form := huh.NewForm(
    huh.NewGroup(
        huh.NewMultiSelect[string]().
            Title("Select AI Agents to enable").
            Options(
                huh.NewOption("Code Assistant", "code-assistant"),
                huh.NewOption("Code Reviewer", "code-reviewer"),
                huh.NewOption("Security Scanner", "security-scanner"),
            ).
            Value(&agents),
    ),
)

if err := form.Run(); err != nil {
    return err
}
```

## 影响范围

- `go.mod` 新增 `lipgloss/v2`、`huh/v2` 及其间接依赖
- `internal/cli/` 各命令文件（start、register、doctor 等）替换输出方式
- 不涉及业务逻辑变更
