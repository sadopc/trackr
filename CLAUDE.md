# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is trackr

A terminal-based time tracker TUI built with Go and Bubble Tea. Features: start/stop/pause timer, project/task management, daily/weekly reports with bar charts, Pomodoro mode, idle detection, CSV/JSON export. Data stored in SQLite at `~/.config/trackr/trackr.db` (macOS: `~/Library/Application Support/trackr/trackr.db`).

## Commands

```bash
go build -o trackr .           # Build binary
go test ./... -count=1         # Run all tests (122 tests)
go test ./internal/store/...   # Run store tests only
go test ./internal/tui/...     # Run TUI tests only
go test ./internal/export/...  # Run export tests only
go test ./internal/tui/... -run TestTimerStartStop -v  # Run single test
CGO_ENABLED=0 go build -o trackr .  # Build without CGO (pure Go SQLite)
```

No linter configured. No CI pipeline yet.

## Architecture

**Elm Architecture (Bubble Tea):** The TUI follows Bubble Tea's Model-Update-View pattern. `tui.App` is the root model that holds child models (dashboard, projects, reports, pomodoro, settings) and routes messages based on `activeView`. Every child model has `update(msg) (model, tea.Cmd)` and `view() string` methods but does NOT implement `tea.Model` — only `App` does.

**Critical pattern — tick routing:** `tickMsg` is always routed to both the dashboard timer and pomodoro model regardless of active view (see `app.go` Update). This keeps timers running when the user is on other tabs.

**Value receiver + pointer fields for huh forms:** Bubble Tea copies models on every Update cycle (value semantics). Form field bindings (`huh.NewInput().Value(ptr)`) must point to heap-allocated `*string` fields, not struct value fields, or the form data is lost on copy. See `projects.go` and `settings.go` — form fields are `*string` initialized in constructors.

**Store layer:** `internal/store` wraps `*sql.DB` with typed CRUD methods. Uses `modernc.org/sqlite` (pure Go, no CGO). Migrations via `PRAGMA user_version`. All timestamps stored as ISO 8601 TEXT, durations as INTEGER seconds. `NewMemory()` creates an in-memory DB for tests.

**Data flow:** `main.go` → opens `Store` → creates `tui.App(store)` → `tea.NewProgram`. Child models receive `*store.Store` and issue async commands (`tea.Cmd`) that query the DB and return typed messages (e.g., `dashboardDataMsg`, `projectsDataMsg`).

## Key design decisions

- **No CGO:** `modernc.org/sqlite` enables cross-compilation. Verify with `CGO_ENABLED=0 go build`.
- **Single DB connection:** `MaxOpenConns(1)` + WAL mode. SQLite is single-writer.
- **View switching:** Number keys `1`-`5` or `tab`. Each view refreshes its data on activation.
- **Forms (huh):** Embedded in Bubble Tea's update loop. When `formActive` is true, the parent model delegates all key input to the form. `esc` cancels, form completion triggers DB write + refresh.
- **Timer model:** Separate from dashboard. Tracks `startTime`, `pauseGap`, `lastActivity` for idle detection. On stop, persists to DB via `store.StopEntry()`.
- **Export:** Writes to `~/trackr-export-{date}.{csv|json}`. The export picker is an overlay managed by `App`, not a child model.
