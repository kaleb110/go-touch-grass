# Architecture

This document describes the internal structure of `go-touch-grass` — how the
packages fit together, where responsibilities live, and why it's laid out the
way it is.

---

## Overview

`go-touch-grass` is a single-binary Linux CLI written in Go. It has two
runtime modes that share the same state file:

- **Daemon mode** — a long-running process (managed by systemd) that wakes up
  every N seconds, adds the elapsed interval to "today's" usage record, and
  saves it.
- **Client mode** — a one-shot process you run from the terminal that reads
  the state file and renders a dashboard or a report.

Both modes operate on the same JSON state file. The daemon only writes; the
client only reads.

```
┌──────────────────────────────────────────────────────────────┐
│                        main.go                               │
│   builds the App, registers subcommands, dispatches          │
└───────────────────────────┬──────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                      cli (registry)                          │
│   Subcommand interface · NewApp · Run · help                 │
│   Registers: TrackerCMD, InstallCMD                          │
└───────┬───────────────────────────────────┬──────────────────┘
        │                                   │
        ▼                                   ▼
┌──────────────────┐               ┌──────────────────┐
│  cli/tracker.go  │               │  cli/install.go  │
│  TrackerCMD      │               │  InstallCMD      │
│  daemon + view   │               │  systemd setup   │
└───┬───────┬──────┘               └──────────────────┘
    │       │
    │       └──────────────┐
    ▼                      ▼
┌──────────┐         ┌──────────┐
│  store   │         │  report  │
│ persist  │         │  render  │
└──────────┘         └──────────┘
                           ▲
                           │
                    ┌──────────┐
                    │   tui    │
                    │ dashboard│
                    └──────────┘
```

---

## Package responsibilities

### `main` (entry point)

`main.go` is deliberately tiny: it constructs the `cli.App`, registers the
subcommands in a deterministic order, and calls `Run()`. It contains no
business logic so that adding a new subcommand never touches the entry point.

### `cli` (command registry)

`cli.go` defines the `Subcommand` interface and the `App` that wires them up.
Each subcommand implements three methods:

```go
type Subcommand interface {
    Name() string
    Description() string
    Init(args []string) error   // parse flags exactly once
    Run() error                 // execute logic
}
```

**Key design decision — single parse.** The original code parsed flags twice
(once in an `Init`-like method and again inside `Run`), which is fragile and
can cause flag-set re-registration panics. In the refactor, `Init` is the sole
owner of flag parsing. `Run` only consumes the already-parsed fields stored on
the receiver. The `App.Run` orchestrator calls `Init` then `Run` in sequence.

Subcommand registration order is preserved so `--help` output is stable
(registration order, not map iteration order).

### `cli/tracker.go` — `TrackerCMD`

The largest command. It has two behaviors selected by the `-update` flag:

| Mode | `-update` | What it does |
|------|-----------|--------------|
| Daemon | `true` | Loops on a ticker, adding elapsed time and saving each tick |
| View | `false` (default) | Reads state once and renders a dashboard or report |

It deliberately stays **thin**: it orchestrates the load → mutate → save
cycle and handles flag-to-behavior mapping, but it delegates the actual work:

- **State I/O** → `store` package
- **Report rendering** → `report` package
- **Dashboard rendering** → `tui` package

Goal resolution (`targetGoal`) follows a clear priority order:
runtime flag (`-daily-goal`) → runtime flag (`-global-goal`) → per-day record
→ persisted global → package default. This keeps the "what goal applies right
now?" decision in one place rather than scattered across render branches.

`tickOnce` is the heart of the command. In daemon mode it calls `recordTick`
(accumulate + save); in view mode it calls `render` (read + display). Both
share the same load-and-locate-today preamble, so day-rollover logic lives in
one spot.

### `cli/install.go` — `InstallCMD`

This is the **new** command that solves the "easy installation" requirement.
Before the refactor, users had to manually copy a `.service` file into
`~/.config/systemd/user/`, edit the `ExecStart` path, and run several
`systemctl` commands by hand.

`install` automates all of it:

1. Locates the running binary (`os.Executable()`).
2. Copies it to `~/.local/bin/go-touch-grass` (refuses to overwrite without
   `-force`).
3. Writes a systemd user unit with the correct absolute `ExecStart` path into
   `~/.config/systemd/user/go-touch-grass.service`.
4. Runs `systemctl --user daemon-reload` and `systemctl --user enable`.
5. Prints the exact `systemctl start` / `journalctl` commands the user needs
   next.

The unit template is embedded in the Go source (`systemdUnitTemplate`), so
there is no longer a static `.service` file in the repo that can drift from
the binary's actual flags. `-dry-run` previews every action without writing.

### `store` — persistence

Owns everything about how state is represented on disk and how it's read and
written. Other packages never call `os.ReadFile`/`os.WriteFile` on the state
file directly.

**Data model:**

```go
type TrackingData struct {        // one day
    Date        string        // "2006-01-02"
    ElapsedTime time.Duration
    DailyGoal   time.Duration // optional per-day override
}

type Envelope struct {            // the whole file
    SchemaVersion int
    GlobalGoal    time.Duration
    History       []TrackingData
}
```

**Atomic writes.** `Save` writes to a temp file in the same directory, then
renames it over the target. A crash mid-write never corrupts the existing
state file — you either have the old file or the new one, never a truncated
half-write.

**Schema migration.** `Load` first tries to unmarshal as the versioned
`Envelope`. If that fails, it falls back to the legacy format (a bare
`[]TrackingData` array from pre-1.0) and upgrades it in memory. The
`SchemaVersion` constant makes future migrations discoverable.

**Path resolution.** `ResolvePath` honors `APP_ENV=development` to redirect
the default path to `./test_state.json` in the working directory, so local
testing never clobbers a developer's real state file.

### `report` — rendering of summaries

Owns the report *types* (`Length`, `Filter`), validation, range slicing, and
the print functions (`PrintDay`, `PrintTotal`, `PrintMulti`). By extracting
this from `cli/tracker.go`, the rendering logic is unit-testable in isolation
and the tracker command becomes a thin dispatcher.

`Range` slices the history list to the requested window (last 7 / 30 / 365
days, or all-time). Missing days are simply absent from the slice — there's
no need to synthesize zero-fill rows because usage is recorded, not sampled.

### `tui` — dashboard rendering

A small, focused package that knows only how to draw the colored progress bar
and the stats block. It takes plain values (elapsed, goal) and returns/prints
strings — no I/O, no state. This makes it trivial to test and keeps terminal
escape codes out of the command layer.

---

## Data flow

### Daemon tick (every 60s by default)

```
systemd ──▶ tracker -update
              │
              ├─ store.Load(path) ───▶ Envelope (or migrate legacy)
              ├─ Envelope.FindToday(now)
              │     ├─ found  ─▶ History[idx].ElapsedTime += tick
              │     └─ missing─▶ append new TrackingData{Date: today}
              ├─ store.Save(path, env)  [atomic temp+rename]
              └─ report.PrintDay(record) ─▶ journal
```

### Manual view

```
tracker (no -update)
   │
   ├─ store.Load(path) ───▶ Envelope
   ├─ Envelope.FindToday(now)
   └─ render():
         ├─ -report today      ─▶ report.PrintDay
         ├─ -report lastWeek   ─▶ report.PrintMulti(list|total)
         ├─ -report allTime    ─▶ report.PrintTotal
         └─ (no -report)       ─▶ tui.RenderDashboard
```

---

## File layout

```
gts/
├── main.go                  # entry point — registers subcommands
├── go.mod
├── script.sh                # build helper
├── README.md
├── ARCHITECTURE.md
├── cli/
│   ├── cli.go               # Subcommand interface + App registry
│   ├── tracker.go           # TrackerCMD — daemon + view
│   ├── install.go           # InstallCMD — automated systemd setup
│   └── version.go           # version constants
├── store/
│   └── store.go             # state model, load/save, migration, paths
├── report/
│   └── tracker.go           # report types, range slicing, print fns
└── tui/
    └── tui.go               # dashboard / progress-bar rendering
```

Each folder is a Go package with a single responsibility. `cli` depends on
`store`, `report`, and `tui`; those three have no dependencies on `cli` or on
each other, which keeps the dependency graph acyclic and testable.

---

## Design goals & how they're met

| Goal | How |
|------|-----|
| Clean separation of concerns | `store` (I/O), `report` (summaries), `tui` (drawing), `cli` (orchestration) |
| No manual systemd config | `install` subcommand generates the unit with the correct paths and enables it |
| Safe state persistence | Atomic write-via-rename; versioned schema with legacy migration |
| Stable, predictable CLI | Single flag-parse per command; registration-ordered help |
| Local development safety | `APP_ENV=development` redirects state to `./test_state.json` |
| Extensibility | New subcommands implement `Subcommand` and register in `main.go` — nothing else changes |

---

## Extending the app

**Add a new subcommand:** create `cli/<name>.go` with a struct implementing
`Name()`, `Description()`, `Init(args)`, `Run()`. Register it in `main.go`.
No other file needs to change.

**Add a new report range:** add a constant to `report.Length`, handle it in
`report.Range`, and it automatically becomes valid for `-report`.

**Bump the schema:** increment `store.SchemaVersion`, add a migration branch
in `store.Load`, and update the writer if the on-disk shape changes.
