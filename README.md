# go-touch-grass

A lightweight Linux CLI that tracks how long your machine is turned on each day and nudges you when you've been on too long. It runs as a systemd user service in the background, writes a small JSON state file, and renders a clean terminal dashboard on demand.

> Machine-on time is used as a proxy for screen time / PC usage. It is not per-application tracking.

---

## Features

- **Automatic background tracking** via a systemd user service that starts on login.
- **One-command install** — `go-touch-grass install` copies the binary and sets up the service for you. No manual systemd file editing.
- **Terminal dashboard** with a colored progress bar showing today's usage against your goal.
- **Reports** over the last week / month / year / all-time, as an aggregated total or a per-day list.
- **Configurable goals** — set a global default or override today's goal on the fly.
- **Safe persistence** — atomic file writes and a versioned JSON schema with automatic migration of older state files.

---

## Quick start

### 1. Build

```bash
cd gts
go build -o go-touch-grass .
```

> Requires Go 1.25+ (module declares `go 1.25.0`).

### 2. Install (sets up systemd for you)

```bash
./go-touch-grass install
```

This will:

1. Copy the binary to `~/.local/bin/go-touch-grass`
2. Write a systemd user unit to `~/.config/systemd/user/go-touch-grass.service`
3. Reload the systemd daemon and enable the service so it starts on login

Then start it now (don't want to log out and back in):

```bash
systemctl --user start go-touch-grass
```

That's it — tracking has begun.

> **Want a shorter name?** Install under an alias so you don't have to type
> `go-touch-grass` every time:
>
> ```bash
> ./go-touch-grass install -as gtg
> systemctl --user start gtg
> gtg tracker
> ```
>
> The binary, service unit, and all printed follow-up commands use whatever
> name you choose. See [`install`](#install--automated-setup) for details.

### 3. Check your usage

```bash
go-touch-grass tracker
```

You'll see a dashboard like:

```

  USER@HOSTNAME
  ────────────────────────────────────
  🌱 [████████████░░░░░░░░] 65.0%
  ────────────────────────────────────
  Usage:   1h57m / 3h0m
  Goal:    3h0m
```

---

## Usage

```
go-touch-grass <subcommand> [flags]

Global Options:
  -v, --version  Print version info
  -h, --help     Print application help

Available subcommands:
  tracker       track machine usage
  install       install the binary and systemd user service
```

### `tracker` — view and record usage

The systemd service runs `tracker -update` in the background. You run plain `tracker` to view your dashboard or generate reports.

| Flag | Default | Description |
|------|---------|-------------|
| `-update` | `false` | Run as the background daemon (adds time to today's record) |
| `-tick` | `60` | Seconds between updates in daemon mode |
| `-report` | *(dashboard)* | Report range: `today`, `lastWeek`, `lastMonth`, `lastYear`, `allTime` |
| `-filter-by` | `list` | For multi-day reports: `total` (one summed line) or `list` (one line per day) |
| `-global-goal` | `3` | Set the global daily goal in hours, e.g. `-global-goal 4` |
| `-daily-goal` | `0` | Override just today's goal in hours, e.g. `-daily-goal 5` |
| `-state` | `~/.local/share/go-touch-grass/state.json` | Path to the state file |
| `-sim-tomorrow` | `false` | Pretend it's tomorrow (for testing day rollover) |
| `-version` | `false` | Print version and exit |

**Examples:**

```bash
# Today's dashboard (default)
go-touch-grass tracker

# Last week, one line per day
go-touch-grass tracker -report lastWeek -filter-by list

# Last month as a single total
go-touch-grass tracker -report lastMonth -filter-by total

# All-time total
go-touch-grass tracker -report allTime

# Change your default goal to 4 hours/day
go-touch-grass tracker -global-goal 4

# Give yourself a 5-hour limit just for today
go-touch-grass tracker -daily-goal 5
```

### `install` — automated setup

| Flag | Default | Description |
|------|---------|-------------|
| `-as` | `go-touch-grass` | Name to install the binary and service as (an alias) |
| `-force` | `false` | Overwrite an existing install |
| `-dry-run` | `false` | Show what would be done without making changes |

```bash
go-touch-grass install                  # default name
go-touch-grass install -as gtg          # shorter alias, use as: gtg tracker
go-touch-grass install -force           # reinstall over an existing one
go-touch-grass install -dry-run         # preview only
```

After install, manage the service with standard systemd commands (substitute
your alias if you used `-as`):

```bash
systemctl --user start go-touch-grass      # start now
systemctl --user stop go-touch-grass       # stop
systemctl --user status go-touch-grass     # check status
journalctl --user -u go-touch-grass -f     # live logs
systemctl --user disable go-touch-grass    # stop starting on login
```

---

## How it works

The systemd service runs `go-touch-grass tracker -update` as a long-running
daemon. Every 60 seconds (configurable with `-tick`) it loads the state file,
adds the elapsed interval to today's record (creating a new record on day
rollover), and saves it back atomically.

When you run `go-touch-grass tracker` manually, it only *reads* the state and
renders the dashboard or a report — it doesn't modify anything.

State lives at `~/.local/share/go-touch-grass/state.json` and is versioned
with a `schema_version` field so older formats can be migrated automatically.

For a deeper breakdown, see [ARCHITECTURE.md](./ARCHITECTURE.md).

---

## Development

```bash
# Build
go build -o go-touch-grass .

# Run locally without touching your real state file
APP_ENV=development ./go-touch-grass tracker

# Test day-rollover behavior
APP_ENV=development ./go-touch-grass tracker -update -sim-tomorrow

# Run tests (when available)
go test ./...
```

Setting `APP_ENV=development` makes the tracker read/write `./test_state.json`
in the current directory instead of your real home-directory state file, so
you can experiment freely.

---

## Uninstall

```bash
systemctl --user disable --now go-touch-grass
rm ~/.local/bin/go-touch-grass
rm ~/.config/systemd/user/go-touch-grass.service
systemctl --user daemon-reload
# state file (optional)
rm -rf ~/.local/share/go-touch-grass
```

---

## License

MIT
