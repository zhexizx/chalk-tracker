# chalk-tracker

运行在命令行里的类 Todoist 任务管理工具，命令为 `todo`。

## 安装

```bash
go install ./cmd/todo
```

可执行文件安装到 `$(go env GOPATH)/bin`，该目录需要在 `PATH` 中。

## 技术栈

- [Cobra](https://github.com/spf13/cobra)：命令结构
- [Viper](https://github.com/spf13/viper)：配置管理
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)：全屏交互界面（TUI）
- [Bubbles](https://github.com/charmbracelet/bubbles)：输入框、快捷键和帮助组件
- [Lip Gloss](https://github.com/charmbracelet/lipgloss)：终端样式

## 需求

### v1

**任务管理**

- `todo add`：添加任务，支持快速语法
- `todo list`：按项目分组查看未完成任务，可用 `--project`/`-p`、`--label`/`-l`、`--priority` 筛选，`--all`/`-a` 包含已完成任务
- `todo today`：查看今天到期和已逾期的任务
- `todo upcoming`：查看未来 7 天的任务
- `todo done <id>`：标记完成，`--undo` 取消完成
- `todo edit <id> ["快速语法"]`：只修改写出的字段，如 `todo edit 3 p1`；写了文本或标签时替换全部文本或标签；`--no-due`、`--no-labels` 清除到期日或标签
- `todo delete <id>`：删除任务，需要确认，`--yes`/`-y` 跳过；不在终端中运行时必须加 `--yes`。已用过的 ID 不再重复使用

**任务属性**

- 项目：一个任务属于一个项目，默认为 Inbox
- 标签：一个任务可以有多个标签
- 优先级：`p1` 到 `p4`，默认为 `p4`
- 到期日：支持 `today`、`tomorrow` 和 `YYYY-MM-DD`

**快速语法**

与 Todoist 一致，`#` 表示项目，`@` 表示标签：

```bash
todo add "买牛奶 #生活 @超市 p1 tomorrow"
```

- 在命令行里要给内容加引号，否则 shell 会把 `#` 开头的部分当作注释丢弃
- 标记以空格分隔，可以写在任意位置，区分大小写
- 可以有多个标签；项目、优先级和到期日写了多个时，以最后一个为准
- 项目和标签的名称不能包含空格
- 在标记前加反斜杠可以保留原文，如 `\#123`、`\p1`、`\today`

**配置（Viper）**

- 配置文件：`~/.config/chalk-tracker/config.yaml`
- 可用 `--config` 指定其他配置文件，指定的文件必须存在；默认路径下没有配置文件时使用默认值
- 可配置项：

| 配置文件 | 环境变量 | 命令行参数 | 默认值 |
|---|---|---|---|
| `data_file` | `TODO_DATA_FILE` | `--data-file` | `~/.config/chalk-tracker/store.json` |
| `default_project` | `TODO_DEFAULT_PROJECT` | 无 | `Inbox` |
| `color` | `TODO_COLOR` | `--color` | `true` |

- 优先级：命令行参数 > 环境变量（`TODO_` 前缀）> 配置文件 > 默认值

**交互界面（TUI）**

- 直接运行 `todo` 进入全屏界面；不在终端中运行时（如输出到管道）显示帮助，不进入界面
- 左侧为视图导航：Inbox、Today、Upcoming 和各项目；右侧为任务列表
- 键盘操作：

| 按键 | 操作 |
|---|---|
| `↑`/`k`、`↓`/`j` | 上下移动 |
| `tab`、`←`/`h`、`→`/`l` | 切换导航和列表 |
| `a` | 添加任务，输入框支持快速语法 |
| `e` | 编辑任务 |
| `space` | 标记完成或取消完成 |
| `d` | 删除任务，需要确认 |
| `/` | 筛选任务 |
| `?` | 显示全部快捷键 |
| `q` | 退出 |

**显示**

- 优先级用不同颜色区分，已逾期的日期醒目显示，已完成的任务弱化显示
- 任务按项目或日期分组，各列对齐，界面随终端大小调整
- 列表为空时显示提示，不输出空白
- 错误信息清晰，命令失败时返回非零退出码
- 子命令（如 `todo list`）仍直接输出结果后退出，便于脚本使用
- 输出到管道或设置了 `NO_COLOR` 时不带颜色

### 暂不包含

- 重复任务
- 子任务
- 与 Todoist 云端同步
