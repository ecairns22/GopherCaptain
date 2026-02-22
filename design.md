---
description: Architecture for GopherCaptain - a Go CLI that deploys Go services from GitHub releases with systemd, nginx, and MariaDB
tags: [design, gophercaptain, deploy, golang, systemd, nginx, mariadb]
audience: { human: 60, agent: 40 }
purpose: { design: 85, flow: 15 }
---

# GopherCaptain — Design

## North Star

One command to go from a GitHub release to a running, routable, database-backed service. Each service lands cleanly alongside others. Everything the tool creates, the tool can remove.

## Context

A Go CLI tool that runs directly on an EC2 instance. You SSH in, run `gophercaptain deploy`, and it handles the full stack: fetch binary from GitHub, configure systemd, allocate a port, set up nginx routing, create a MariaDB database with scoped credentials.

The tool manages its own state so you never have to remember what's deployed, on what port, or with what configuration.

**Informed by:** `north-star.md`

## Constraints

- Runs as root (or with sudo) — needs to write systemd units, nginx configs, manage MariaDB
- GitHub token required for private repos — stored once, reused across deploys
- MariaDB already installed and running — tool connects as admin to create databases and users
- Nginx already installed — tool manages per-service config files
- Linux/amd64 target — release assets must follow a naming convention the tool can match
- Single-machine deployment — no clustering, no remote orchestration

---

## Components

```
┌──────────────────────────────────────────────────────┐
│                     CLI (cobra)                       │
│  deploy | upgrade | remove | list | status | inspect  │
└──────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────┐
│                    Orchestrator                        │
│  Coordinates all managers for deploy/upgrade/remove   │
└──────────────────────────────────────────────────────┘
     │           │           │          │          │
     ▼           ▼           ▼          ▼          ▼
┌─────────┐ ┌─────────┐ ┌────────┐ ┌───────┐ ┌───────┐
│ GitHub  │ │ Systemd │ │ Nginx  │ │  DB   │ │ State │
│ Client  │ │ Manager │ │Manager │ │Manager│ │ Store │
└─────────┘ └─────────┘ └────────┘ └───────┘ └───────┘
```

**GitHub Client** — Calls GitHub Releases API, downloads the correct asset for linux/amd64, verifies checksum if available.

**Systemd Manager** — Generates unit files, runs daemon-reload, enables and starts services. Removes units on teardown.

**Nginx Manager** — Generates per-service server blocks (subdomain) or location blocks (path prefix). Tests config before reloading.

**DB Manager** — Connects to MariaDB, creates databases and users with scoped privileges. Generates random passwords. Drops databases on removal with confirmation.

**State Store** — SQLite database tracking all deployed services, their versions, ports, routes, credentials reference, and history.

---

## File Layout

```
/opt/gophercaptain/
├── bin/
│   ├── api/
│   │   ├── api                    ← current binary (symlink)
│   │   ├── api-v1.2.0             ← versioned binary
│   │   └── api-v1.1.0             ← previous version (for rollback)
│   └── auth/
│       ├── auth
│       ├── auth-v0.5.0
│       └── auth-v0.4.0

/etc/gophercaptain/
├── gophercaptain.conf                ← tool config (GitHub token, MariaDB admin creds, port range)
├── api/
│   └── env                        ← service env file (chmod 600)
└── auth/
    └── env

/etc/systemd/system/
├── gc-api.service
└── gc-auth.service

/etc/nginx/sites-available/
├── gc-api.conf
└── gc-auth.conf

/var/lib/gophercaptain/
└── state.db                       ← SQLite state database
```

Versioned binaries are kept for rollback. The current binary is a symlink to the active version. The previous version is retained; older versions are pruned.

---

## State Schema

```sql
CREATE TABLE services (
    name         TEXT PRIMARY KEY,
    repo         TEXT NOT NULL,          -- github owner/repo
    version      TEXT NOT NULL,
    prev_version TEXT,                   -- for rollback
    port         INTEGER NOT NULL UNIQUE,
    route_type   TEXT NOT NULL,          -- 'subdomain' or 'path'
    route_value  TEXT NOT NULL,          -- 'api.example.com' or '/api'
    db_name      TEXT NOT NULL,
    db_user      TEXT NOT NULL,
    extra_env    TEXT,                   -- JSON key-value pairs
    deployed_at  INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE TABLE history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service     TEXT NOT NULL,
    action      TEXT NOT NULL,           -- deploy, upgrade, rollback, remove
    version     TEXT,
    timestamp   INTEGER NOT NULL,
    detail      TEXT                     -- JSON, what changed
);
```

---

## Tool Configuration

`/etc/gophercaptain/gophercaptain.conf` (TOML):

```toml
[github]
token = "ghp_..."
owner = "ecairns22"          # default owner, overridable per deploy

[ports]
range_start = 3000
range_end   = 4000

[mariadb]
host     = "127.0.0.1"
port     = 3306
admin_user = "root"
admin_password_file = "/root/.mariadb_password"   # read from file, not inline

[nginx]
sites_dir = "/etc/nginx/sites-available"
enabled_dir = "/etc/nginx/sites-enabled"

[releases]
asset_pattern = "{{.Name}}-linux-amd64"           # Go template, matched against asset names
```

File permissions: `chmod 600 /etc/gophercaptain/gophercaptain.conf`

---

## CLI Specification

```
gophercaptain deploy <repo> [flags]
    Deploy a service from a GitHub release.

    --version, -v     Release tag (default: latest)
    --port, -p        Override port (default: auto-assign)
    --route, -r       Route rule, e.g. "api.example.com" or "/api"
    --route-type      "subdomain" or "path" (inferred from --route format)
    --name, -n        Service name (default: repo name)
    --env, -e         Extra env vars (repeatable): -e KEY=VALUE
    --no-db           Skip database creation
    --config-file     Write config file instead of env vars

    Example:
    gophercaptain deploy myapi -v v1.2.0 -r api.example.com -e LOG_LEVEL=info

gophercaptain upgrade <service> [flags]
    Upgrade to a new version. Keeps previous binary for rollback.

    --version, -v     Target version (default: latest)

gophercaptain rollback <service>
    Swap back to the previous version. Restarts the service.

gophercaptain remove <service> [flags]
    Stop and remove a service. Cleans up systemd, nginx, env file.

    --drop-db         Also drop the MariaDB database and user (requires confirmation)
    --yes, -y         Skip confirmation

gophercaptain list
    Show all deployed services.

    Output:
    NAME    VERSION   PORT   ROUTE                STATUS
    api     v1.2.0    3000   api.example.com      running
    auth    v0.5.0    3001   /auth                running
    worker  v2.0.1    3002   —                    stopped

gophercaptain status <service>
    Detailed status: systemd state, port, route, database, last deploy time.

gophercaptain inspect <service>
    Print all generated config: systemd unit, nginx config, env file (values redacted).

gophercaptain init
    First-time setup: create directories, write config template, test MariaDB connection.
```

---

## Deploy Flow

```
deploy "myapi" --version v1.2.0 --route api.example.com
│
├─ 1. Validate
│     State store: name "myapi" not already taken?
│     Port allocator: find next free port in range
│
├─ 2. Fetch binary
│     GitHub Client: GET /repos/{owner}/myapi/releases/tags/v1.2.0
│     Download asset matching pattern → /opt/gophercaptain/bin/myapi/myapi-v1.2.0
│     chmod +x, symlink myapi → myapi-v1.2.0
│
├─ 3. Create database
│     DB Manager: CREATE DATABASE gc_myapi;
│     Generate 32-char random password
│     CREATE USER 'gc_myapi'@'localhost' IDENTIFIED BY '...';
│     GRANT ALL ON gc_myapi.* TO 'gc_myapi'@'localhost';
│
├─ 4. Write env file
│     /etc/gophercaptain/myapi/env:
│       PORT=3000
│       DB_HOST=127.0.0.1
│       DB_PORT=3306
│       DB_NAME=gc_myapi
│       DB_USER=gc_myapi
│       DB_PASSWORD=<generated>
│       <extra --env values>
│     chmod 600
│
├─ 5. Write systemd unit
│     /etc/systemd/system/gc-myapi.service
│     systemctl daemon-reload
│     systemctl enable gc-myapi
│     systemctl start gc-myapi
│     Wait for service to be active (up to 10s)
│
├─ 6. Write nginx config
│     /etc/nginx/sites-available/gc-myapi.conf
│     Symlink to sites-enabled
│     nginx -t (test config)
│     If test fails → roll back nginx config, warn, continue
│     systemctl reload nginx
│
├─ 7. Record state
│     INSERT into services and history tables
│
└─ 8. Output
      ✓ myapi v1.2.0 deployed
        Port:     3000
        Route:    api.example.com → localhost:3000
        Database: gc_myapi
```

If any step after fetch fails, previous steps are rolled back: remove binary, drop database, delete unit, delete nginx config. The deploy is atomic from the operator's perspective.

---

## Upgrade Flow

```
upgrade "myapi" --version v1.3.0
│
├─ Fetch new binary → myapi-v1.3.0
├─ Stop service
├─ Update symlink: myapi → myapi-v1.3.0
├─ Start service
├─ Wait for healthy (up to 10s)
│   └─ If unhealthy → automatic rollback to previous symlink, restart
├─ Prune old versions (keep current + previous only)
├─ Update state store
└─ Output result
```

Database schema migrations are the service's responsibility, not the tool's.

---

## Remove Flow

```
remove "myapi" --drop-db
│
├─ systemctl stop gc-myapi
├─ systemctl disable gc-myapi
├─ Remove unit file, daemon-reload
├─ Remove nginx config + symlink, reload nginx
├─ Remove env file and config directory
├─ Remove binaries
├─ If --drop-db: DROP USER, DROP DATABASE (after confirmation)
├─ Delete from state store, record in history
└─ Output result
```

---

## Systemd Unit Template

```ini
[Unit]
Description=GopherCaptain: {{.Name}}
After=network.target mariadb.service

[Service]
Type=simple
ExecStart=/opt/gophercaptain/bin/{{.Name}}/{{.Name}}
EnvironmentFile=/etc/gophercaptain/{{.Name}}/env
Restart=on-failure
RestartSec=5
User=gc-{{.Name}}
Group=gc-{{.Name}}
WorkingDirectory=/opt/gophercaptain/bin/{{.Name}}

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/gophercaptain/bin/{{.Name}}

[Install]
WantedBy=multi-user.target
```

Each service runs as its own system user (`gc-<name>`), created during deploy, removed during teardown. Services cannot read each other's credentials.

---

## Nginx Config Templates

**Subdomain:**

```nginx
server {
    listen 80;
    server_name {{.RouteValue}};

    location / {
        proxy_pass http://127.0.0.1:{{.Port}};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Path prefix:**

```nginx
location {{.RouteValue}} {
    proxy_pass http://127.0.0.1:{{.Port}};
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Path prefix blocks are appended to an existing server block. The tool expects a `gophercaptain-paths.conf` include in the main nginx server block.

---

## Port Allocation

Ports are assigned sequentially from `range_start` (default 3000). The allocator queries the state store for used ports and returns the lowest available port in the range. Explicit `--port` overrides are checked against the state store for conflicts before use.

---

## Credential Management

- MariaDB admin password: read from file path specified in `gophercaptain.conf`, never stored by the tool
- Service database passwords: generated using `crypto/rand`, 32 alphanumeric characters
- Written to `/etc/gophercaptain/<service>/env`, owned by root, chmod 600
- The service's systemd user reads the env file via `EnvironmentFile=` (systemd reads it as root before dropping privileges)
- GitHub token: stored in `gophercaptain.conf`, chmod 600
- `gophercaptain inspect` redacts credential values in output

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| GitHub release not found | Fail with clear message, no state changes |
| Asset name doesn't match pattern | Fail, list available assets for the release |
| Port range exhausted | Fail, suggest expanding range or removing unused services |
| MariaDB connection refused | Fail, suggest checking MariaDB status |
| Database already exists | Fail, suggest `--name` to use a different service name |
| Nginx config test fails | Roll back nginx config, warn, complete deploy without routing |
| Service fails to start | Roll back entire deploy, report journalctl output |
| Rollback target missing | Fail, explain no previous version available |

All failures leave the system in a clean state. Partial deploys are rolled back.

---

## Trade-offs

| Chose | Over | Because |
|-------|------|---------|
| SQLite state store | Flat file (JSON/TOML) | Concurrent safety, query capability, history tracking |
| One system user per service | Shared user | Credential isolation between services |
| Symlink-based versioning | In-place replacement | Instant rollback without re-downloading |
| Sequential port allocation | Random/hash-based | Predictable, debuggable, low port numbers |
| Go template for nginx | Full nginx Go library | Nginx configs are simple; templates suffice |
| `nginx -t` before reload | Reload and hope | Prevents taking down all routing on a bad config |
| `crypto/rand` for passwords | `math/rand` or UUID | Cryptographically secure with no external dependency |

## Alternatives Considered

**Ansible/Terraform:** Declarative and powerful but heavyweight for a personal single-machine tool. The overhead of maintaining playbooks or HCL exceeds the complexity of the problem.

**Docker/Compose:** Would solve isolation and port management but adds a layer of abstraction over systemd, complicates MariaDB access (networking), and requires Docker on the instance.

**Systemd socket activation:** Elegant but requires services to support it. Using explicit ports is simpler and works with any Go HTTP server.

**HashiCorp Nomad:** Production-grade scheduler but overkill for a handful of personal services on one machine.

## Risks

| Risk | Mitigation |
|------|------------|
| Binary naming convention varies across repos | Configurable `asset_pattern` in tool config; fail clearly with available asset list |
| Running as root | Systemd hardening (NoNewPrivileges, ProtectSystem), per-service users for runtime |
| State DB corruption | SQLite WAL mode; `gophercaptain init` can rebuild state from what's on disk |
| Service listens on wrong port | Health check after start confirms port is responding |

## Extension Points

- **Asset pattern:** Go template, configurable per-repo if needed in the future
- **Health check:** Currently "is port responding"; could support custom health endpoints
- **TLS:** Nginx configs are HTTP-only; designed to sit behind a separate TLS terminator or be extended with certbot integration
- **Config file mode:** `--config-file` writes TOML instead of env vars for services that prefer it
