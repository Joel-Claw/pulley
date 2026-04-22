# AutoPull

A lightweight Linux service that automatically `git pull`s registered repositories on a configurable schedule.

## Features

- **Daemon mode** — runs as a systemd service
- **Per-repo scheduling** — set interval or specific times for each repo
- **Safe registration** — verifies folder is a valid git repo before adding
- **Simple CLI** — add, remove, list, pull, daemon

## Install

```bash
go build -o autopull .
sudo cp autopull /usr/local/bin/
sudo cp autopull.service /etc/systemd/system/
sudo systemctl enable --now autopull
```

## Usage

```bash
# Register a repo (must be inside a git repo or pass a path)
autopull add                        # current directory
autopull add /path/to/my/repo       # specific path

# Configure schedule (default: every 30 minutes)
autopull add /path/to/repo --interval 15m
autopull add /path/to/repo --at "09:00,18:00"

# List registered repos
autopull list

# Remove a repo
autopull remove /path/to/repo

# Pull all repos now
autopull pull

# Run as daemon (foreground, for systemd)
autopull daemon
```

## Configuration

Config is stored at `~/.config/autopull/config.json` (or `$XDG_CONFIG_HOME/autopull/config.json`).

```json
{
  "repos": [
    {
      "path": "/home/user/my-project",
      "schedule": {
        "interval": "30m"
      }
    },
    {
      "path": "/home/user/another-repo",
      "schedule": {
        "times": ["09:00", "18:00"]
      }
    }
  ]
}
```

### Schedule options

- `interval` — pull every N minutes/hours (e.g. `"15m"`, `"2h"`)
- `times` — pull at specific times (e.g. `["09:00", "18:00"]`)
- If both are set, both apply
- If neither is set, defaults to every 30 minutes

## License

MIT