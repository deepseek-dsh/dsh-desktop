# DeepSeek Harness Desktop (dsh-desktop)

[English](../../README.md) | **中文**

**把 DeepSeek Harness 变成你的桌面应用 —— 双击即用，无需打开终端、无需记任何命令。**

[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/) [![Wails v2](https://img.shields.io/badge/Wails-v2-2F6BFF)](https://wails.io/) [![Vue 3](https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white)](https://vuejs.org/) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](#许可证)

---

DeepSeek Harness（DSH）是一个强大的 AI 智能体工作台，但它的 Web 服务需要通过命令行启动：`dsh web --port xxx`，再手动打开浏览器访问。对普通用户来说，门槛太高了。

**dsh-desktop 把这一切藏起来。** 它不重新实现 Harness，而是做一个合格的「宿主」：双击图标自动拉起服务、管理端口、守护进程、回收资源，然后把窗口直接变成 Harness 界面——你看到的就是一个普通的桌面应用，背后是完整的 DeepSeek Harness。

## 为什么选择 dsh-desktop？

| 场景 | 裸用 `dsh` 命令行 | dsh-desktop |
|---|---|---|
| 启动 | 手动执行 `dsh web`，还要记端口 | 双击图标，自动拉起 |
| 端口 | 随机分配易冲突，关终端服务就没了 | 固定 `127.0.0.1:3080`，已有服务自动复用 |
| 进程 | 终端一关进程就挂，还可能残留僵尸进程 | 生命周期托管，退出即回收整棵进程树 |
| 插件 | 手动敲命令安装 | 插件市场后台静默预装，装完自动提示重启 |
| 界面 | 浏览器标签页，和别的窗口混在一起 | 独立窗口原生承载，无 iframe 嵌套 |

## 核心特性

- **双击即用**：无需手动启动 CLI 或管理端口，打开就是 Harness 主页。
- **固定端口 + 复用探测**：服务固定监听 `127.0.0.1:3080`；启动时先探测该端口，已有 Harness 服务在跑就直接复用，绝不重复拉起、不抢端口。
- **进程托管**：启动、就绪检测、日志、重启、优雅关闭，全部自动完成；以独立进程组运行，退出或超时后强杀兜底，杜绝僵尸进程。
- **直达主页**：启动页只显示一个呼吸动画的图标，就绪后窗口直接跳转承载 Harness Web UI，流畅无 iframe。
- **插件市场自动预装**：`dshmarket` 在进入主页后后台静默安装（幂等，已装则跳过）；需要重启的插件会弹原生提示，一键重启并自动刷新。
- **数据与系统 DSH 完全共用**：与命令行 `dsh` 使用同一套 profiles / sessions / 插件 / 凭据，两边切换无感。
- **退出即回收**：关闭窗口时优雅关闭 Harness 子进程并回收进程树，不留后台残留。

## 运行策略：现状与规划

- **第一阶段（已实现）**：优先复用系统中已安装的 `dsh`；未安装时自动使用系统 Node.js + `npx` 拉起兼容版本。零安装也能跑。
- **第二阶段（规划中）**：内置精简 Node 运行时与 dsh 依赖，实现完全离线、开箱即用。

## 工作原理

```
打开应用
   │
   ▼
探测 127.0.0.1:3080 ──── 已有 Harness 服务？── 是 ──► 直接复用，进入主页
   │                                              （不重复拉起）
   否
   ▼
拉起 dsh web 子进程（系统 dsh 优先，npx 兜底）
   │
   ▼
就绪轮询（每 400ms 探测一次，超时上限 180s）
   │
   ▼
就绪后窗口跳转 Harness Web UI，启动壳退出
   │
   ▼
关闭窗口 ──► 优雅关闭子进程，超时强杀进程组兜底回收
```

后台还有两条静默流水线：**插件预装**（进入主页后自动安装插件市场，不打扰）和**插件变更监控**（检测到新插件需要重启时弹原生提示，一键重启刷新）。

## 架构

```text
dsh-desktop (Wails v2 + Go, 前端 Vue 3)
├── main.go                   # 入口: 加载配置, 绑定 App
├── internal/cfg/             # 端口/数据目录/日志路径解析与目录创建
├── internal/harness/         # dsh web 子进程生命周期(启动/就绪/回收/关闭)
├── internal/app/             # Wails 绑定 API(供前端调用)
├── cmd/smoke/                # 开发期冒烟验证(真实拉起 dsh, 非产品入口)
├── frontend/                 # Vue 3 壳界面(启动/失败/停止状态展示)
└── build/                    # Wails 打包资源与产物(build/bin/)
```

模块职责一览：

| 模块 | 职责 |
|---|---|
| `internal/cfg` | 解析数据目录、DSH 数据根、日志路径，固定端口 3080 |
| `internal/harness` | 拉起/检测/就绪/重启/关闭 `dsh web` 子进程，进程树回收 |
| `internal/app` | 对前端暴露的绑定 API：`Start` / `Status` / `Stop` / `Restart` 等 |
| `frontend` | 启动壳界面（呼吸动画）、失败重试、停止态启动 |

## 数据目录

应用数据与安装目录分离，升级不删用户数据，遵循 XDG 规范。

| 项目 | 默认路径 | 覆盖方式 | 内容 |
|---|---|---|---|
| 桌面数据根 | `~/.local/share/dsh-desktop` | `DSH_DESKTOP_DATA_DIR` | 桌面端自己的数据（如日志） |
| DSH 数据根 | `~/.dsh` | `DSH_HOME` | 与系统 DSH 共用 profiles / sessions / 插件 / 凭据 |
| 日志 | `<桌面数据根>/logs/` | 同 `DSH_DESKTOP_DATA_DIR` | harness 运行日志 |

> 与命令行 DSH 共用同一套 `DSH_HOME`，意味着你在桌面端建的会话、装的插件，回到命令行依然在。

## 端口

- Harness 固定监听 `127.0.0.1:3080`（仅回环地址，不对外暴露）。
- 启动时先探测该端口：**已有 Harness 服务则直接复用，不重复拉起**；否则自动拉起新服务。
- 复用外部已运行的服务时，桌面端不会接管或关闭它——各干各的，互不干扰。

## 配置

全部通过环境变量配置，无需修改文件。

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `DSH_HOME` | `~/.dsh` | DSH CLI 与桌面端共用的数据根目录 |
| `DSH_DESKTOP_DATA_DIR` | `~/.local/share/dsh-desktop` | 桌面应用数据根目录（日志等） |
| `DSH_DESKTOP_SKIP_PLUGINS` | 未设置 | 设为 `1` 关闭全部自动预装插件 |
| `DSH_DESKTOP_USAGE_PLUGIN` | 未设置 | 指定本地/远程用量插件安装来源（开发用） |

## 常见问题

**我电脑上没有装 dsh 能用吗？** 能。应用会自动用 Node.js + `npx` 拉一个兼容版本，首次需要联网。

**端口 3080 被占用了怎么办？** 如果占用的正是 Harness 服务，应用会直接复用；如果是别的程序，请先停掉它再启动应用。

**关闭窗口后服务会残留吗？** 不会。退出时优雅关闭子进程，超时后强杀整个进程组兜底回收。

**桌面端和命令行的数据互通吗？** 互通。两者共用 `~/.dsh`，会话、插件、凭据完全一致。

## 开发

环境要求：Go 1.25+、Node.js 20.19+、Wails CLI v2.14。

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式（热更新 + 绑定自动生成）
wails dev

# 接入本地 @dsh-plugins/usage 用量插件（可选）
DSH_DESKTOP_USAGE_PLUGIN=/path/to/usage wails dev

# 类型检查 / 单元测试
go vet ./...
go test ./internal/harness/...

# 构建桌面端产物（build/bin/dsh-desktop）
wails build
```

**冒烟验证**（真实拉起 `dsh web`，验证就绪与进程回收）：

```bash
DSH_HOME=/tmp/dsh-smoke/home DSH_DESKTOP_DATA_DIR=/tmp/dsh-smoke/data go run ./cmd/smoke
# 期望输出:
# 就绪耗时 ..., URL=http://127.0.0.1:3080, 状态=ready
# 重启完成, 耗时 ..., 状态=ready
# 已优雅关闭, 最终状态=stopped
```

**插件市场自动安装验证**（需网络 + pnpm）：

```bash
go test ./internal/harness/ -tags manual -run TestPreinstallManual -v
```

## 路线图

- [x] 第一阶段：优先复用系统 DSH（含 `DSH_HOME` 数据），npx 兼容回退、生命周期托管、状态界面
- [ ] 内置精简 Node 运行时与 dsh 依赖（完全离线）
- [ ] 版本更新检测与升级引导（GitHub Releases）
- [ ] 系统托盘与多窗口

## 许可证

MIT