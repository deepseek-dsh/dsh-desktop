# DeepSeek Harness Desktop (dsh-desktop)

**English** | [中文](./docs/zh/README.md)

**Turn DeepSeek Harness into a desktop app — double-click to run, no terminal, no commands to remember.**

[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/) [![Wails v2](https://img.shields.io/badge/Wails-v2-2F6BFF)](https://wails.io/) [![Vue 3](https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white)](https://vuejs.org/) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](#license)

---

DeepSeek Harness (DSH) is a powerful AI agent workbench, but its web service is CLI-driven: `dsh web --port xxx`, then open a browser tab. That's a high barrier for everyday users.

**dsh-desktop hides all of that.** It doesn't reimplement Harness — it's a proper host: double-click to launch, auto-manage the port, guard the process, and turn the window itself into the Harness UI. You see a normal desktop app; behind it runs the full DeepSeek Harness.

## Why dsh-desktop?

| Scenario | Bare `dsh` CLI | dsh-desktop |
|---|---|---|
| Startup | Type `dsh web`, remember the port | Double-click, auto-launch |
| Port | Random & collides, dies with the terminal | Fixed `127.0.0.1:3080`, reuses an existing service |
| Process | Orphans on terminal close | Full lifecycle management, process-tree reaping on exit |
| Plugins | Manual commands | Market auto-installed in background, native restart prompt |
| UI | A browser tab among others | Native window, no iframe |

## Key Features

- **Double-click to run** — no CLI, no port management; open straight into the Harness home page.
- **Fixed port + reuse probe** — listens on `127.0.0.1:3080`; if a Harness service is already running there, it is reused instead of being started twice.
- **Process hosting** — start, readiness detection, logging, restart, graceful shutdown; runs in its own process group with a kill fallback, so no zombies.
- **Straight to the home page** — a breathing-logo splash, then the window navigates directly to the Harness Web UI.
- **Auto preinstalled plugins** — `dshmarket` installs silently after entering the home page (idempotent); plugins needing a restart trigger a native prompt with one-click restart + refresh.
- **Shared data with system DSH** — the same profiles / sessions / plugins / credentials as the CLI `dsh`.
- **Clean exit** — graceful shutdown of the child process and full process-tree reaping on window close.

## Run Strategy: Now & Next

- **Stage 1 (shipped)** — prefer the system `dsh` on PATH; fall back to Node.js + `npx` when missing. Works on a clean machine.
- **Stage 2 (planned)** — bundle a minimal Node runtime and dsh dependencies for fully offline, out-of-the-box use.

## How It Works

```
Launch app
   │
   ▼
Probe 127.0.0.1:3080 ── Harness already running? ── yes ──► reuse, go to home page
   │                                                       (no duplicate spawn)
   no
   ▼
Spawn `dsh web` (system dsh first, npx as fallback)
   │
   ▼
Readiness polling (every 400ms, 180s timeout)
   │
   ▼
Window navigates to the Harness Web UI; splash exits
   │
   ▼
Window close ──► graceful shutdown, kill-group fallback
```

Two background pipelines also run: **plugin preinstall** (silently installs the market after entering the home page) and **plugin-change monitoring** (native restart prompt when a plugin needs one).

## Architecture

```text
dsh-desktop (Wails v2 + Go, frontend Vue 3)
├── main.go                   # Entry: load config, bind App
├── internal/cfg/             # Data dir / DSH home / log path, fixed port 3080
├── internal/harness/         # dsh web child-process lifecycle (start/ready/reap/stop)
├── internal/app/             # Wails-bound APIs for the frontend (Start/Status/Stop/Restart)
├── cmd/smoke/                # Dev smoke test (spawns real dsh, not a product entry)
├── frontend/                 # Vue 3 shell UI (splash / failure / stopped states)
└── build/                    # Wails packaging assets and artifacts (build/bin/)
```

| Module | Responsibility |
|---|---|
| `internal/cfg` | Resolves data dir, DSH home, log path; fixes port 3080 |
| `internal/harness` | Spawn / probe / ready / restart / stop the `dsh web` child process; reaps the process tree |
| `internal/app` | Wails-bound APIs for the frontend: `Start` / `Status` / `Stop` / `Restart` / ... |
| `frontend` | Splash shell UI (breathing logo), failure retry, stopped-state launch |

## Data Directories

App data is kept separate from the install directory, following the XDG spec.

| Item | Default path | Override | Contents |
|---|---|---|---|
| App data root | `~/.local/share/dsh-desktop` | `DSH_DESKTOP_DATA_DIR` | Desktop-only data (e.g. logs) |
| DSH data root | `~/.dsh` | `DSH_HOME` | Shared with CLI DSH: profiles / sessions / plugins / credentials |
| Logs | `<app data root>/logs/` | same as above | harness runtime logs |

> Sharing `DSH_HOME` with the CLI means sessions and plugins you create in the desktop app are still there in the terminal.

## Port

- Fixed `127.0.0.1:3080` (loopback only, never exposed to the network).
- On startup the port is probed: **an existing Harness service is reused, not duplicated**; otherwise a new one is launched.
- When reusing an external service, the desktop app neither takes it over nor shuts it down.

## Configuration

Environment variables only — no config files to edit.

| Variable | Default | Description |
|---|---|---|
| `DSH_HOME` | `~/.dsh` | Shared data root for DSH CLI and the desktop app |
| `DSH_DESKTOP_DATA_DIR` | `~/.local/share/dsh-desktop` | Desktop app data root (logs, etc.) |
| `DSH_DESKTOP_SKIP_PLUGINS` | unset | Set to `1` to disable all automatic plugin preinstall |
| `DSH_DESKTOP_USAGE_PLUGIN` | unset | Local/remote usage-plugin source (development) |

## FAQ

**Can I use it without installing dsh?** Yes. The app falls back to Node.js + `npx` (first run needs network).

**Port 3080 is taken?** If the occupant is a Harness service, the app reuses it; otherwise stop the other program first.

**Any leftovers after closing?** No. Graceful shutdown with a kill-group fallback.

**Do the desktop app and the CLI share data?** Yes — both use `~/.dsh`.

## Development

Requirements: Go 1.25+, Node.js 20.19+, Wails CLI v2.14.

```bash
cd frontend && npm install && cd ..
wails dev            # dev mode (hot reload + binding generation)
go vet ./...         # vet
go test ./internal/harness/...   # unit tests
wails build          # build desktop artifact (build/bin/dsh-desktop)
```

Smoke test (spawns a real `dsh web`):

```bash
DSH_HOME=/tmp/dsh-smoke/home DSH_DESKTOP_DATA_DIR=/tmp/dsh-smoke/data go run ./cmd/smoke
```

Plugin market preinstall test (needs network + pnpm):

```bash
go test ./internal/harness/ -tags manual -run TestPreinstallManual -v
```

## Roadmap

- [x] Stage 1: reuse system DSH (incl. `DSH_HOME` data), npx fallback, lifecycle hosting, status UI
- [ ] Bundled minimal Node runtime + dsh dependencies (fully offline)
- [ ] Version update detection & upgrade guidance (GitHub Releases)
- [ ] System tray & multiple windows

## License

MIT