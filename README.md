# GopherCaptain

One command to go from a GitHub release to a running, routable, database-backed service.

GopherCaptain is a Go CLI that runs on a Linux server and handles the full stack: fetch a binary from GitHub Releases, configure systemd, allocate a port, set up nginx routing, and create a MariaDB database with scoped credentials.

## What it does

```
gophercaptain deploy myapi -v v1.2.0 -r api.example.com
```

This single command:

1. Downloads the `v1.2.0` release binary from GitHub
2. Creates a MariaDB database and user with a generated password
3. Writes an env file with PORT, DB credentials, and any extra vars
4. Creates a systemd unit, enables and starts the service
5. Writes an nginx config for `api.example.com` → `localhost:<port>`
6. Records everything in a local SQLite state database

Upgrade, rollback, and remove are equally simple. If any step fails during deploy, all completed steps are rolled back automatically.

## Prerequisites

**Server:**
- Linux (amd64 or arm64) with systemd
- nginx installed with `sites-available`/`sites-enabled` layout
- MariaDB installed and running
- Root or sudo access

**For building from source:**
- Go 1.24+

## Installation

### From GitHub Releases (recommended)

Download the latest binary for your architecture:

```bash
# amd64
curl -Lo /usr/local/bin/gophercaptain \
  https://github.com/ecairns22/GopherCaptain/releases/latest/download/gophercaptain-linux-amd64
chmod +x /usr/local/bin/gophercaptain

# arm64
curl -Lo /usr/local/bin/gophercaptain \
  https://github.com/ecairns22/GopherCaptain/releases/latest/download/gophercaptain-linux-arm64
chmod +x /usr/local/bin/gophercaptain
```

### From source

```bash
go install github.com/ecairns22/GopherCaptain/cmd/gophercaptain@latest
```

Or clone and build:

```bash
git clone https://github.com/ecairns22/GopherCaptain.git
cd GopherCaptain
go build -o gophercaptain ./cmd/gophercaptain/
sudo mv gophercaptain /usr/local/bin/
```

## Quick Start

### 1. Initialize

```bash
sudo gophercaptain init
```

This creates the required directories and writes a config template to `/etc/gophercaptain/gophercaptain.conf`.

### 2. Configure

Edit `/etc/gophercaptain/gophercaptain.conf` with your GitHub token and settings:

```toml
[github]
token = "ghp_YOUR_TOKEN"
owner = "your-github-username"

[mariadb]
admin_password_file = "/root/.mariadb_password"
```

Write your MariaDB root password to the file referenced above:

```bash
echo "your-mariadb-password" > /root/.mariadb_password
chmod 600 /root/.mariadb_password
```

Run init again to verify the connection:

```bash
sudo gophercaptain init
```

### 3. Deploy a service

```bash
sudo gophercaptain deploy myapi -v v1.2.0 -r api.example.com -e LOG_LEVEL=info
```

## Commands

| Command | Description |
|---------|-------------|
| `gophercaptain init` | Create directories, write config template, test connections |
| `gophercaptain deploy <repo>` | Deploy a service from a GitHub release |
| `gophercaptain upgrade <service>` | Upgrade to a new version (auto-rollback on failure) |
| `gophercaptain rollback <service>` | Swap back to the previous version |
| `gophercaptain remove <service>` | Stop and remove all artifacts for a service |
| `gophercaptain list` | Show all deployed services with live status |
| `gophercaptain status <service>` | Detailed status for a service |
| `gophercaptain inspect <service>` | Print generated configs (credentials redacted) |

### Deploy flags

```
-v, --version string    Release tag (default: latest)
-p, --port int          Override port (default: auto-assign from range)
-r, --route string      Route rule: "api.example.com" or "/api"
    --route-type string  "subdomain" or "path" (inferred from --route)
-n, --name string       Service name (default: repo name)
-e, --env strings       Extra env vars: -e KEY=VALUE (repeatable)
    --no-db             Skip database creation
    --config-file       Write TOML config file instead of env vars
```

### Remove flags

```
    --drop-db   Also drop the MariaDB database and user
-y, --yes       Skip confirmation prompt
```

## Configuration

Default path: `/etc/gophercaptain/gophercaptain.conf` (override with `GOPHERCAPTAIN_CONFIG` env var)

```toml
[github]
token = "ghp_..."                # GitHub personal access token (repo scope)
owner = "your-username"          # Default repo owner

[ports]
range_start = 3000               # Port allocation range start
range_end   = 4000               # Port allocation range end (exclusive)

[mariadb]
host     = "127.0.0.1"
port     = 3306
admin_user = "root"
admin_password_file = "/root/.mariadb_password"

[nginx]
sites_dir   = "/etc/nginx/sites-available"
enabled_dir = "/etc/nginx/sites-enabled"

[releases]
asset_pattern = "{{.Name}}-linux-amd64"   # Go template for matching release assets
```

## Development

```bash
git clone https://github.com/ecairns22/GopherCaptain.git
cd GopherCaptain

# Run tests (no external services needed)
go test ./...

# Run vet
go vet ./...

# Build
go build ./cmd/gophercaptain/

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o gophercaptain-linux-amd64 ./cmd/gophercaptain/
GOOS=linux GOARCH=arm64 go build -o gophercaptain-linux-arm64 ./cmd/gophercaptain/
```

MariaDB integration tests are gated behind an environment variable:

```bash
GOPHERCAPTAIN_TEST_MARIADB=password go test ./internal/db/...
```

## Architecture

```
cmd/gophercaptain/          CLI entrypoint + commands
internal/
  config/                   TOML config loading
  orchestrator/             Coordinates deploy/upgrade/rollback/remove flows
  state/                    SQLite state store (services + history)
  github/                   GitHub Releases API client
  systemd/                  Unit file generation + service lifecycle
  nginx/                    Config generation + test + reload
  db/                       MariaDB database/user lifecycle
  ports/                    Sequential port allocation
  creds/                    Credential generation + env file writing
  health/                   TCP health check
  runner/                   Command execution abstraction (testable)
```

## License

See [LICENSE](LICENSE) for details.
