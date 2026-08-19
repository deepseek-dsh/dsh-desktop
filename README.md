# DeepSeek Harness Desktop (dsh-desktop)

将 [DeepSeek Harness](https://github.com/dataelement/dsh-desktop) 的 `dsh web` 服务封装为 PC 端桌面应用。本应用不重新实现 Harness,而是补齐桌面产品所需的「宿主」能力:自动拉起子进程、固定回环端口、就绪检测、进程树回收、日志落地,以及状态展示界面。

技术栈:**Wails v2 (Go)** + **Vue 3**。后端负责进程托管与本地能力,应用启动后直接进入 Harness 主页(无启动过渡页),就绪后壳体通过短暂状态轮询 **将窗口直接跳转到 Harness Web UI**,由窗口原生承载、无 iframe 嵌套,渲染流畅。

> 本实现参照 [dataelement/dsh-desktop](https://github.com/dataelement/dsh-desktop)(Electron 壳)的产品职责,以 Wails 形态落地。

## 现状与运行策略

- **第一阶段(已实现)**:优先复用 PATH 中的系统 `dsh`;未安装时使用系统 Node.js + `npx` 拉起兼容版本。
- **第二阶段(规划)**:内置精简 Node 运行时与 dsh 依赖,实现完全离线开箱即用。

## 功能

- 双击即用,自动拉起 Harness,无需手动启动 CLI 或管理端口。
- 固定 `127.0.0.1:3080` 回环端口;启动时探测该端口,已有 Harness 服务则直接复用,否则自动拉起新服务。
- 应用专属数据目录(与安装目录分离,升级不删用户数据),遵循 XDG 规范。
- 子进程托管:启动、就绪轮询(400ms,超时 180s)、日志、重启、优雅关闭,并以独立进程组在退出/超时后强杀兜底回收整棵进程树,杜绝僵尸进程。
- 复用探测:端口上已有 Harness 服务时直接复用,不重复拉起。
- **直达主页**:应用启动后显示透明底 favicon 图标(呼吸动画),就绪后通过低频状态检查直接跳转承载 Harness 主页;插件市场在后台静默安装,不阻塞进入。
- **默认预装插件**:自动安装插件市场 `dshmarket`(已装则跳过,幂等);开发时可通过 `DSH_DESKTOP_USAGE_PLUGIN` 接入本地用量插件。其它插件需要时在 Harness 的「插件市场」里安装。安装为尽力而为:失败仅记录、不阻断整体;可用环境变量 `DSH_DESKTOP_SKIP_PLUGINS=1` 关闭全部自动预装。插件安装沿用 Harness profile 已选择的 pnpm store，避免 store 不一致。插件无法热加载时由市场提示“立即重启”，重启动作交给桌面端托管，完成后自动刷新页面。

## 架构

```text
dsh-desktop (Wails, Go)
├── main.go                   # 入口: 加载配置, 绑定 App
├── internal/cfg/             # 端口/数据目录/日志路径解析与目录创建
├── internal/harness/         # dsh web 子进程生命周期(启动/就绪/回收/关闭)
├── internal/app/             # Wails 绑定 API(供前端调用)
├── cmd/smoke/                # 开发期冒烟验证(真实拉起 dsh, 非产品入口)
├── frontend/                 # Vue 3 壳界面(仅启动失败/已停止时显示, 其余阶段空白)
└── build/                    # Wails 打包资源与产物(build/bin/)
```

### 数据目录

| 项目 | 默认路径 | 覆盖 |
|---|---|---|
| 桌面数据根 | `~/.local/share/dsh-desktop` | 环境变量 `DSH_DESKTOP_DATA_DIR`，仅存桌面端日志等数据 |
| DSH 数据根 | `~/.dsh` | 环境变量 `DSH_HOME`，与系统 DSH 共用 profiles/sessions/plugins/凭据 |
| 日志 | `<root>/logs/` | 同上 |

### 端口

- Harness 固定监听 `127.0.0.1:3080`。
- 启动时探测该端口:已有 Harness 服务则直接复用,不重复拉起;否则自动拉起新服务。

### Wails 绑定 API

| 方法 | 说明 |
|---|---|
| `Start()` | 异步拉起 harness；前端短暂轮询 `Status()`，就绪后直接跳转 |
| `Status()` | 当前状态快照(state/url/port/logPath/error) |
| `Stop()` | 优雅关闭子进程 |
| `Restart()` | 异步重启子进程，完成后刷新当前 Harness 页面 |
| `OpenInterface()` | 用系统浏览器打开就绪后的 Harness 界面 |
| `Platform()` | 当前平台信息 |

### 生命周期与运维

- **直达主页 + 就绪跳转**:应用打开后启动页为透明底 favicon 图标(呼吸动画);壳前端每 500ms 检查一次状态，就绪后用 `window.location.href` 直接导航到 Harness。该轮询只存在于启动壳阶段，进入主页后即销毁。窗口主体即为 Harness，无 iframe 嵌套。启动失败或应用被停止时才显示壳的错误/操作界面。
- **插件后台静默安装**:进入主页后,`dshmarket` 在后台异步安装(不阻塞进入)。桌面端监控 profile 清单，并在安装稳定后读取市场 activation 状态；只有 `restart`/`inert` 插件才弹出原生“稍后 / 立即重启”对话框，热加载成功的插件不会打扰用户。点击重启后桌面端会回收旧进程、重新拉起并自动刷新页面。前置依赖:`pnpm` 可执行与网络;安装沿用 Harness profile 的 pnpm store。
- **原生重启提示**:桌面端监控 web profile 清单；安装结束后若市场报告插件状态为 `restart`/`inert`，会显示原生“稍后 / 立即重启”对话框。点击后由桌面端回收并重新拉起受管进程，避免依赖跳转后的 Harness 页面仍持有 Wails runtime。
- **原生构建白名单**:安装前会往 `profiles/web/pnpm-workspace.yaml` 写入 `allowBuilds`,放行常用安全原生依赖 `node-pty / cloudflared / cpu-features / ssh2`——从市场安装用到这些依赖的插件(如 `dsh-plugin-terminal`、`dsh-web-ui-all`)不会因 pnpm 默认拦截而失败。白名单外的包仍按默认拦截,保留供应链安全;可在该文件 `allowBuilds` 里按需增删。
- **退出即回收**:应用不设菜单栏,退出通过窗口关闭按钮触发,`OnShutdown` 会优雅回收 Harness 子进程树。当前重启入口由插件市场按需显示，尚未提供独立的停止入口；如需要可在后续用托盘补充。

## 开发

环境要求:Go 1.25+、Node.js 20.19+、Wails CLI v2.14。

### 应用图标(Linux)

- **窗口/任务栏图标**:构建时由 `//go:embed build/appicon.png` 注入 `options.App.Linux.Icon`,运行时通过 `gtk_window_set_icon` 设置(favicon 白底黑鲸鱼)。
- **桌面/启动器图标**:GNOME/KDE 等桌面环境的启动器图标取自 `.desktop` 文件的 `Icon=` 字段,不读运行时的 GTK 图标。安装到系统后使用提供的 `build/dsh-desktop.desktop` 与 `build/dsh-desktop.png`:

```bash
sudo install -D build/dsh-desktop.png /usr/local/share/icons/dsh-desktop.png
sudo install -D build/dsh-desktop.desktop /usr/local/share/applications/dsh-desktop.desktop
sudo install -m755 build/bin/dsh-desktop /usr/local/bin/dsh-desktop
sudo update-desktop-database /usr/local/share/applications
```

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式(热更新 + 绑定自动生成)
wails dev

# 接入本地 @dsh-plugins/usage，安装后重启一次并打开「设置 → 用量」
DSH_DESKTOP_USAGE_PLUGIN=/home/halo/code/dsh-plugins/packages/usage wails dev

# 类型检查 / 单元测试
go vet ./...
go test ./internal/harness/...

# 构建桌面端产物(build/bin/dsh-desktop)
wails build
```

> 说明:绑定生成产物位于 `frontend/wailsjs/go/app/`(命名空间取决于 App 所属包),`wails dev`/`wails build` 会自动生成,无需手工维护。

### 冒烟验证(核心流程端到端)

`cmd/smoke` 直接复用 `cfg` 与 `harness` 包,真实拉起 `dsh web`,确认就绪后优雅关闭,验证进程组回收:

```bash
DSH_HOME=/tmp/dsh-smoke/home DSH_DESKTOP_DATA_DIR=/tmp/dsh-smoke/data go run ./cmd/smoke
# 期望输出:
# 就绪耗时 ..., URL=http://127.0.0.1:3080, 状态=ready
# 重启完成, 耗时 ..., 状态=ready
# 已优雅关闭, 最终状态=stopped
```

在未配置 GUI 的无头环境应同时把 `DSH_HOME` 和 `DSH_DESKTOP_DATA_DIR` 指向可写目录完成验证，避免修改日常使用的共享 profile。

### 插件市场自动安装验证(需网络 + pnpm)

```bash
go test ./internal/harness/ -tags manual -run TestPreinstallManual -v
```

该用例对全新目录真实执行 `EnsurePreinstalled`(依序检测/安装默认插件,即插件市场),断言事件序列产生、`profiles/web/node_modules/dshmarket` 出现、二次调用幂等(不再 `install`)以及 skip 开关行为。默认 `go test ./...` 不带 `manual` tag 会跳过此网络依赖用例。

## 已知边界

- PATH 中没有 `dsh` 时依赖系统 Node.js，npx 首次装配 dsh 依赖需要网络；第二阶段将内置离线运行时消除此限制。
- 就绪后窗口 WebView 直接承载 Harness(启动壳低频状态检查后跳转,无 iframe),与参考项目的 Electron 原生窗口承载在形态上一致。
- 窗口跳转 Harness 后壳页面不再可达；插件市场需要重启时可通过其提示按钮触发桌面端受管重启，但尚未提供独立的停止/常驻运维入口。
- 无头环境无法启动 GUI,故跳转的运行时表现需在带桌面的 Linux 以 `wails dev` 人工验收。

## 路线图

- [x] 第一阶段:优先复用系统 DSH（含 `DSH_HOME` 数据），npx 兼容回退、生命周期托管、状态界面
- [ ] 内置精简 Node 运行时与 dsh 依赖(完全离线)
- [ ] 版本更新检测与升级引导(GitHub Releases)
- [ ] 系统托盘与多窗口(可选)
