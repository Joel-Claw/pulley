# Pulley

A lightweight Linux service that automatically `git pull`s registered repositories on a configurable schedule. Set it up once and your repos stay in sync without thinking about it.

## Why?

You have multiple git repos on a machine, some you want updated every 15 minutes, others twice a day, others at specific times. You don't want to remember to pull. You don't want to write cron jobs for each one. You want one config file, one daemon, done.

## Features

- **Daemon mode** — runs as a systemd service, starts on boot
- **Per-repo scheduling** — each repo gets its own interval, specific pull times, or time range
- **Global defaults** — set default interval, times, and range for all repos at once
- **Time ranges** — restrict pulling to a time window (e.g. business hours only)
- **Safe registration** — verifies the folder is a valid git repo before adding
- **Fast-forward only** — uses `git pull --ff-only` so it never creates unexpected merges
- **Simple CLI** — add, remove, list, pull, daemon, config
- **Zero dependencies** — single static binary, no runtime requirements
- **Config as JSON** — human-readable, version-controllable, easy to edit manually
- **Works everywhere** — Debian, Ubuntu, Arch, NixOS, any Linux with systemd

## Quick Start

### One-line install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/Joel-Claw/pulley/main/install.sh | sudo bash
```

This works on Debian, Ubuntu, Arch, Fedora, openSUSE, Alpine, NixOS, and any Linux with Go and git. It detects your distro, installs dependencies, builds from source, and sets up the systemd service.

To **update** an existing installation:

```bash
curl -fsSL https://raw.githubusercontent.com/Joel-Claw/pulley/main/install.sh | sudo bash
```

Same command. It detects the existing install, rebuilds, and restarts the service.

To **uninstall**:

```bash
curl -fsSL https://raw.githubusercontent.com/Joel-Claw/pulley/main/install.sh | sudo bash -s -- --uninstall
```

To install a **specific version**:

```bash
curl -fsSL https://raw.githubusercontent.com/Joel-Claw/pulley/main/install.sh | sudo bash -s -- --version=v0.2.0
```

### Build and install

```bash
git clone https://github.com/Joel-Claw/pulley.git
cd pulley
make
sudo make install
```

### Or use an installer

```bash
# Debian / Ubuntu
sudo ./install/install-debian.sh

# Arch Linux
sudo ./install/install-arch.sh

# Nix
./install/install-nix.sh
```

### Set defaults and register repos

```bash
# Set global defaults (applied to repos without their own schedule)
pulley config set --interval 15m
pulley config set --at "09:00,18:00"
pulley config set --range "08:00-22:00"

# Add a repo (uses config defaults if no flags given)
pulley add /path/to/my-project

# Override per-repo
pulley add /path/to/my-project --interval 5m
pulley add /path/to/my-project --at "09:00,18:00"
pulley add /path/to/my-project --range "09:00-17:00"

# Start the daemon
sudo systemctl start pulley
```

## Usage

### Commands

| Command | Description |
|---------|-------------|
| `pulley add [path] [flags]` | Register a git repo for auto-pulling |
| `pulley remove <path>` | Unregister a repo |
| `pulley list` | List all registered repos, schedules, and defaults |
| `pulley pull [path]` | Pull all repos (or a specific one) right now |
| `pulley daemon` | Run as foreground daemon (for systemd) |
| `pulley config` | Show current config and defaults |
| `pulley config set [flags]` | Set default interval, times, or range |
| `pulley help` | Show usage information |

### Add flags

| Flag | Example | Description |
|------|---------|-------------|
| `--interval` | `--interval 15m` | Pull every N minutes/hours (Go duration: `10m`, `2h`, `1h30m`) |
| `--at` | `--at "09:00,18:00"` | Pull at specific times (HH:MM, comma-separated, 24h) |
| `--range` | `--range "09:00-17:00"` | Only pull within this time window |

If no flags are given, the repo inherits config defaults. If no defaults are set, the interval is **30 minutes**.

### Config set flags

| Flag | Example | Description |
|------|---------|-------------|
| `--interval` | `--interval 15m` | Default pull interval for all repos |
| `--at` | `--at "09:00,18:00"` | Default pull times for all repos |
| `--range` | `--range "08:00-22:00"` | Default active time range for all repos |

You can also set individual keys:

```bash
pulley config set defaultInterval 2h
pulley config set defaultRange "09:00-17:00"
```

### Examples

```bash
# Set global defaults
pulley config set --interval 15m --range "08:00-22:00"

# Add current directory (inherits defaults)
pulley add

# Add with 5-minute interval (overrides default)
pulley add /home/user/my-app --interval 5m

# Pull three times a day at specific times
pulley add /home/user/docs --at "08:00,12:00,18:00"

# Only pull during business hours
pulley add /home/user/work-repo --range "09:00-17:00"

# Mix interval + specific times + range
pulley add /home/user/monitoring --interval 5m --at "00:00" --range "06:00-23:00"

# Remove a repo
pulley remove /home/user/my-app

# Pull everything now
pulley pull

# See current config and defaults
pulley config

# See what's registered
pulley list
```

## Configuration

Config is stored at `~/.config/pulley/config.json` (respects `$XDG_CONFIG_HOME`).

You can edit it directly or use the CLI. The daemon reloads the config every minute, so changes are picked up without restarting.

### Example config

```json
{
  "defaultInterval": "15m",
  "defaultTimes": ["09:00", "18:00"],
  "defaultRange": "08:00-22:00",
  "repos": [
    {
      "path": "/home/user/my-project",
      "schedule": {
        "interval": "15m"
      }
    },
    {
      "path": "/home/user/work-repo",
      "schedule": {
        "interval": "2h",
        "times": ["09:00", "18:00"],
        "range": "09:00-17:00"
      }
    },
    {
      "path": "/home/user/daily-sync",
      "schedule": {
        "times": ["08:00"]
      },
      "lastPull": "2026-04-22T08:00:00Z"
    }
  ]
}
```

### Schedule options

| Option | Type | Description |
|--------|------|-------------|
| `interval` | string | Go duration syntax (`"15m"`, `"2h"`, `"1h30m"`). Default: 30m |
| `times` | string[] | Specific clock times in HH:MM format. Checked within a 1-minute window |
| `range` | string | Time window in `"HH:MM-HH:MM"` format. Pulling only happens within this range |
| All combined | — | `range` gates whether pulling is allowed; `interval` and `times` trigger pulls when allowed |

### Defaults hierarchy

1. **Repo-level schedule** takes priority (interval, times, range)
2. **Config defaults** fill in anything the repo doesn't set
3. **Built-in fallback**: 30m interval if nothing else is configured

So if you set `defaultRange: "09:00-17:00"` and a repo has no `range`, it inherits the default. But if the repo sets its own `range`, that wins.

### How scheduling works

The daemon checks every 60 seconds:

1. **Range gate**: If a range is set (on the repo or as default), only pull within that time window
2. **Interval check**: If more time than the configured interval has passed since the last pull, it pulls
3. **Time check**: If the current time (HH:MM) matches any entry in `times`, it pulls
4. If both interval and times are configured, either condition triggers a pull
5. `lastPull` is updated in the config after each successful pull

## Installation

### From source (any Linux)

```bash
git clone https://github.com/Joel-Claw/pulley.git
cd pulley
make
sudo make install
sudo systemctl enable --now pulley
```

Requires: Go 1.23+, git, systemd

### Debian / Ubuntu

```bash
sudo ./install/install-debian.sh
```

This script:
1. Installs Go and git if not present (via apt)
2. Builds pulley with optimized flags
3. Installs the binary to `/usr/local/bin/pulley`
4. Installs the systemd service to `/etc/systemd/system/pulley.service`
5. Sets the service user to the user who ran sudo
6. Enables the service (does not start it, add repos first)

### Arch Linux

```bash
sudo ./install/install-arch.sh
```

Same as Debian installer but uses pacman for dependencies.

### Nix / NixOS

#### Option 1: nix profile install (any Nix system)

```bash
nix profile install github:Joel-Claw/pulley
```

#### Option 2: NixOS module

Add to your `configuration.nix`:

```nix
{
  imports = [ /path/to/pulley/install/flake.nix#nixosModules.pulley ];

  services.pulley = {
    enable = true;
    user = "youruser";
    configPath = "/home/youruser/.config/pulley/config.json";
  };
}
```

Then:

```bash
sudo nixos-rebuild switch
```

#### Option 3: Run without installing

```bash
nix run github:Joel-Claw/pulley -- add /path/to/repo
```

### Manual systemd setup

If you built from source without the Makefile:

```bash
# Copy binary
sudo cp pulley /usr/local/bin/

# Copy and edit service file
sudo cp install/pulley.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now pulley
```

Edit the service file to set `User=` and `Group=` to match your user account.

## Systemd service

The daemon runs as a systemd service. Key behaviors:

- **Restart on failure** — if the process crashes, systemd restarts it after 30 seconds
- **Start after network** — waits for network-online.target before starting
- **Config hot-reload** — the daemon re-reads `config.json` every minute, so you can add/remove repos or change defaults without restarting the service

### Service management

```bash
# Start
sudo systemctl start pulley

# Stop
sudo systemctl stop pulley

# Restart (after manual config changes if you don't want to wait)
sudo systemctl restart pulley

# Check status
systemctl status pulley

# View logs
journalctl -u pulley -f
```

## How it works

1. **Registration**: `pulley add` verifies the path is a git repo using `git rev-parse --show-toplevel`, then stores the absolute path and schedule in `config.json`
2. **Daemon loop**: Every 60 seconds, the daemon:
   - Reloads `config.json` (so you can edit it live)
   - Checks each repo against its schedule (with defaults applied)
   - Runs `git pull --ff-only` for repos that are due
   - Updates `lastPull` timestamps in the config
3. **Fast-forward only**: Uses `--ff-only` to prevent unexpected merges. If a repo has diverged, the pull fails gracefully and the error is logged

## Project structure

```
pulley/
├── cmd/
│   └── pulley/
│       ├── main.go              # CLI commands + daemon loop
│       ├── config.go            # Config types, load/save, scheduling logic, ranges
│       ├── git.go               # Git operations (pull, status, verify)
│       ├── main_test.go         # Unit tests
│       └── integration_test.go  # Integration tests
├── install/
│   ├── pulley.service           # systemd unit file
│   ├── install-arch.sh          # Arch Linux installer
│   ├── install-debian.sh        # Debian/Ubuntu installer
│   ├── install-nix.sh           # Nix/NixOS installer
│   └── flake.nix                # Nix flake + NixOS module
├── docs/
├── LICENSE                      # CC0 1.0 Universal
├── Makefile
├── go.mod
└── README.md
```

## Requirements

- **Runtime**: git, Linux with systemd
- **Build**: Go 1.23+
- **No other dependencies**

## License

CC0 1.0 Universal — public domain dedication. No rights reserved.