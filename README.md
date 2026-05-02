# onix-timer

A smart CLI timer for the [Onix](https://github.com/sadirano/onix) ecosystem. Supports countdowns, stopwatches, and repeating intervals — all managed by a lightweight background daemon with desktop notifications.

## Features

- **Countdown timers** — start with natural language or colon notation
- **Stopwatch** — count up with lap splits
- **Repeat timers** — fire on a fixed interval, optionally anchored to a wall-clock time
- **Background daemon** — single hidden process polls all timers; spawns automatically, exits after 60 s of inactivity
- **Desktop notifications** — Windows toast (WinRT) with `msg.exe` fallback
- **fzf integration** — interactive picker with inline pause/resume/cancel/reset hotkeys
- **Live watch mode** — `--watch` refreshes the timer table in-place every second
- **Per-project scopes** — timers are isolated by `ONIX_TARGET` / `ONIX_ALIAS` environment variables
- **`--exec`** — run an arbitrary shell command each time a timer fires

## Installation

```sh
go install github.com/sadirano/onix-timer@latest
```

Or build from source:

```sh
git clone https://github.com/sadirano/onix-timer
cd onix-timer
go build -o onix-timer.exe .   # Windows
go build -o onix-timer .       # macOS / Linux
```

State files and config are stored under `~/.onix/timer/`.

## Quick start

```sh
# Start a 25-minute countdown (implicit start)
timer 25m

# Same, explicit
timer start 25m

# Name it inline
timer start 25m standup

# List active timers (opens fzf if available)
timer ls

# Pause / resume
timer stop standup

# Cancel
timer cancel standup
```

## Timer specs

### Duration formats

| Input | Meaning |
|-------|---------|
| `25` | 25 minutes (bare number) |
| `25m` | 25 minutes |
| `1.5h` | 90 minutes |
| `1h30m` | 1 hour 30 minutes |
| `1h 30m 20s` | 1 hour 30 minutes 20 seconds |
| `30 minutes` | 30 minutes (word form) |
| `2 hours 30 minutes` | 2 hours 30 minutes |
| `90s` | 90 seconds |
| `27:30` | 27 min 30 sec (MM:SS) |
| `1:30:00` | 1 hour 30 minutes (HH:MM:SS) |

### Trailing text becomes the timer name

```sh
timer start 25m standup        # name = "standup"
timer start 1h30m deep work    # name = "deep work"
timer start 30 minutes meeting # name = "meeting"
```

### Repeating timers

```sh
timer start every 1h                    # repeat every hour
timer start every 30m standup           # repeat every 30 min, named "standup"
timer start every 2h from 1pm          # repeat every 2 h, first fire at 13:00
timer start from 9am every 30m standup # same, alternate word order
```

### One-shot delayed start

```sh
timer start at 14:30        # fires at 14:30 today (tomorrow if past)
timer start at 3pm          # fires at 15:00
```

### Stopwatch

```sh
timer start --stopwatch          # anonymous stopwatch
timer start --stopwatch workout  # named stopwatch
timer lap workout                # record a split
```

## Commands

| Command | Description |
|---------|-------------|
| `timer [start] <spec>` | Start a timer (explicit `start` optional) |
| `timer stop <id\|name>` | Pause a running timer (run again to resume) |
| `timer cancel <id\|name>` | Delete a timer permanently |
| `timer reset <id\|name>` | Restart from original duration |
| `timer lap <id\|name>` | Record a split (stopwatch only) |
| `timer ls` | List active timers |
| `timer ls --watch` | Live-refresh list (updates every second) |
| `timer ls --raw` | TSV output with raw seconds |
| `timer status` | One-line summary per active timer |
| `timer help` | Show usage |

## Flags

### Start flags

| Flag | Description |
|------|-------------|
| `--name`, `-n <name>` | Override the timer name |
| `--stopwatch` | Count up instead of down |
| `--every <dur>` | Repeat on interval |
| `--at <time>` | Delay start to wall-clock time (`HH:MM` or `3pm`) |
| `--exec <cmd>` | Shell command to run on each expiry |
| `--notify` | Force notifications on (overrides config default) |
| `--no-notify` | Disable notifications for this timer |
| `--global`, `-g` | Use the global scope (ignore project context) |

### ls flags

| Flag | Description |
|------|-------------|
| `--raw` | TSV output with raw second values |
| `--watch` | Refresh table every second |
| `--global`, `-g` | Show global scope timers |

## fzf controls

When `fzf` is on `PATH`, `timer ls` opens an interactive picker:

| Key | Action |
|-----|--------|
| `Enter` | Show timer details |
| `ctrl-s` | Pause / resume selected timer |
| `ctrl-d` | Cancel / delete selected timer |
| `ctrl-r` | Reset selected timer to original duration |

## Notifications and `--exec`

Notifications are **on by default**. Override globally in config or per-timer at start time:

```sh
timer start 25m standup --no-notify   # silent timer
timer start 5m break --notify         # force on even if config says off
```

Run a command each time a timer fires:

```sh
timer start every 1h --exec "espanso trigger :hydrate"
timer start 25m pomodoro --exec "notify-send Done"
```

## Configuration

`~/.onix/timer/config.toml`:

```toml
[timer]
raw_seconds = false   # show seconds as raw numbers instead of HH:MM:SS
notify      = true    # desktop notifications (default on)

[timer.fzf]
prompt = "timer> "    # fzf prompt string
layout = "default"    # fzf layout (default | reverse | reverse-list)
```

## Scopes

Timers are isolated by scope. The active scope is resolved in this order:

1. `--global` / `-g` flag → `global`
2. `ONIX_TARGET` environment variable
3. `ONIX_ALIAS` environment variable
4. Fallback: `global`

State is stored at `~/.onix/timer/<scope>.json`. The background daemon polls all scopes.

## Daemon

The first command that creates or checks active timers automatically spawns a hidden background daemon (`timer daemon`). The daemon:

- Polls all scope state files every second
- Fires notifications and `--exec` commands when a timer expires
- Reschedules repeat timers (`NextFireAt += RepeatEvery`)
- Exits automatically after 60 seconds of no active timers
- On Windows: spawned with `DETACHED_PROCESS` so closing the originating terminal has no effect

The daemon PID is stored at `~/.onix/timer/daemon.pid`.

## State files

Each scope persists to `~/.onix/timer/<scope>.json`. The file is written atomically (write to temp, rename) to prevent corruption.

Example entry:

```json
{
  "id": "t1",
  "name": "standup",
  "kind": "countdown",
  "duration_s": 1500,
  "started_at": "2026-05-02T09:00:00Z",
  "remaining_s": 1500,
  "next_fire_at": "2026-05-02T09:25:00Z"
}
```
