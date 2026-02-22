---
description: Plan for building all GopherCaptain managers and the state store independently
tags: [plan, gophercaptain, foundation, managers]
audience: { human: 40, agent: 60 }
purpose: { plan: 95, design: 5 }
---

# Plan: Foundation Components

Implements: [GopherCaptain Design](design.md) — State Store, GitHub Client, Systemd Manager, Nginx Manager, DB Manager, Port Allocator, Credential Generator, Tool Config

## Scope

**Covers:**
- Go module scaffolding and project structure
- Tool configuration loading (`gophercaptain.conf`)
- State store (SQLite) with schema and CRUD operations
- Port allocator
- Credential generator
- GitHub client (release listing, asset download)
- Systemd manager (unit generation, service lifecycle)
- Nginx manager (config generation, test, reload)
- DB manager (MariaDB database/user lifecycle)
- `gophercaptain init` command

**Does not cover:**
- Orchestrator coordination logic (Plan: Integration)
- Deploy/upgrade/rollback/remove flows (Plan: Integration)
- CLI commands beyond `init` (Plan: Integration)
- Rollback on partial failure (Plan: Integration)

## Enables

Once the foundation exists:
- **Plan: Integration** can proceed — all managers have interfaces the orchestrator calls
- Each manager is independently testable against its real dependency (systemd, nginx, MariaDB, GitHub API)
- `gophercaptain init` works, proving the tool can bootstrap itself on a fresh instance

## Prerequisites

- Go 1.22+ installed on development machine
- EC2 instance with: systemd, nginx (with `sites-available`/`sites-enabled`), MariaDB running
- GitHub personal access token with `repo` scope
- A GitHub repo with at least one release containing a `<name>-linux-amd64` asset for testing

## North Star

Each manager does one thing, has a clean interface, and can be tested in isolation. The state store is the single source of truth for what's deployed. Configuration loads once and flows to all consumers.

## Done Criteria

### Project Structure

- The module shall be initialized as `github.com/ecairns22/GopherCaptain`
- The project shall use the following package layout:
  - `cmd/gophercaptain/` — main entrypoint
  - `internal/config/` — configuration loading
  - `internal/state/` — SQLite state store
  - `internal/github/` — release client
  - `internal/systemd/` — unit manager
  - `internal/nginx/` — config manager
  - `internal/db/` — MariaDB manager
  - `internal/ports/` — port allocator
  - `internal/creds/` — credential generator

### Configuration

- The config loader shall read TOML from `/etc/gophercaptain/gophercaptain.conf`
- The config loader shall read the MariaDB admin password from the file path specified in `admin_password_file`
- When the config file is missing, the loader shall return an error naming the expected path
- When a required field is missing, the loader shall return an error naming the field
- The config loader shall accept an override path via environment variable `GOPHERCAPTAIN_CONFIG` for testing

### State Store

- The state store shall initialize the SQLite database at `/var/lib/gophercaptain/state.db` with WAL mode
- The state store shall create `services` and `history` tables matching the design schema
- The state store shall support: insert service, get service by name, list all services, update service, delete service
- The state store shall support: append history entry, list history for a service
- When the database file does not exist, the state store shall create it with the schema
- When the database exists with the schema, the state store shall open without modification

### Port Allocator

- The port allocator shall return the lowest unused port in the configured range
- The port allocator shall query the state store for ports currently in use
- When a specific port is requested, the allocator shall verify it is unused and within range
- When the port range is exhausted, the allocator shall return an error stating the range and suggesting removal of unused services
- When a requested port conflicts, the allocator shall return an error naming the service that holds it

### Credential Generator

- The credential generator shall produce 32-character alphanumeric strings using `crypto/rand`
- The credential generator shall write env files with `chmod 600` and `root:root` ownership
- The credential generator shall support both env-file format (`KEY=VALUE` lines) and TOML config-file format
- The env file shall include: `PORT`, `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, plus any extra key-value pairs

### GitHub Client

- The GitHub client shall authenticate using the token from configuration
- The GitHub client shall resolve "latest" to the most recent release tag
- The GitHub client shall list release assets and match against the configured `asset_pattern`
- When no asset matches the pattern, the client shall return an error listing all available asset names
- The GitHub client shall download the matched asset to `/opt/gophercaptain/bin/<name>/<name>-<version>`
- The GitHub client shall set the downloaded file to executable (`chmod +x`)
- The GitHub client shall create a symlink `<name>` pointing to the versioned binary

### Systemd Manager

- The systemd manager shall generate unit files from the design template using Go `text/template`
- The systemd manager shall write unit files to `/etc/systemd/system/gc-<name>.service`
- The systemd manager shall create a system user `gc-<name>` for the service if it does not exist
- The systemd manager shall run `systemctl daemon-reload` after writing or removing a unit
- The systemd manager shall support: enable, start, stop, disable, and remove operations
- When asked to start, the manager shall wait up to 10 seconds for the service to reach `active` state
- When the service fails to start, the manager shall return an error including the last 20 lines of journal output
- The systemd manager shall report service status by querying `systemctl is-active`

### Nginx Manager

- The nginx manager shall generate subdomain configs (full `server` block) and path-prefix configs (`location` block)
- When route format contains a dot, the manager shall infer subdomain type; when it starts with `/`, path type
- The nginx manager shall write configs to the configured `sites_dir` as `gc-<name>.conf`
- The nginx manager shall create a symlink in `enabled_dir` pointing to the config
- The nginx manager shall run `nginx -t` before reloading
- When `nginx -t` fails, the manager shall remove the config it just wrote and return an error with the test output
- When `nginx -t` passes, the manager shall run `systemctl reload nginx`
- The nginx manager shall support removal: delete config, delete symlink, reload nginx

### DB Manager

- The DB manager shall connect to MariaDB using admin credentials from configuration
- The DB manager shall create a database named `gc_<name>`
- The DB manager shall create a user `gc_<name>@localhost` with a generated password
- The DB manager shall grant `ALL PRIVILEGES` on `gc_<name>.*` to the created user
- When the database already exists, the manager shall return an error suggesting `--name`
- The DB manager shall support dropping a database and user, returning the names that will be dropped for confirmation
- When MariaDB is unreachable, the manager shall return an error suggesting `systemctl status mariadb`

### Init Command

- `gophercaptain init` shall create all required directories: `/opt/gophercaptain/bin/`, `/etc/gophercaptain/`, `/var/lib/gophercaptain/`
- When `gophercaptain.conf` does not exist, `init` shall write a template config with placeholder values
- `init` shall test the MariaDB connection using configured credentials and report success or failure
- `init` shall verify nginx is installed and the `sites-available`/`sites-enabled` directories exist
- `init` shall initialize the state database

## Constraints

- **SQLite only** — design chose SQLite for state; no alternative databases
- **`text/template` for generation** — no third-party templating engines; Go standard library suffices
- **No `math/rand` for credentials** — design requires `crypto/rand`
- **All file operations use design paths** — `/opt/gophercaptain/`, `/etc/gophercaptain/`, `/var/lib/gophercaptain/`, `/etc/systemd/system/`, nginx dirs from config
- **No orchestration logic** — each manager operates independently; coordination is Plan: Integration scope

## References

- [GopherCaptain Design](design.md) — component descriptions, state schema, file layout, templates, error handling table
- [cobra](https://github.com/spf13/cobra) — `github.com/spf13/cobra` for CLI framework
- [go-toml](https://github.com/pelletier/go-toml) — `github.com/pelletier/go-toml/v2` for config parsing
- [go-sqlite3](https://github.com/mattn/go-sqlite3) — `github.com/mattn/go-sqlite3` for SQLite (CGo) or [modernc sqlite](https://gitlab.com/AliAbdur662/sqlite) — `modernc.org/sqlite` for pure Go alternative
- [go-github](https://github.com/google/go-github) — `github.com/google/go-github/v60` for GitHub API
- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) — `github.com/go-sql-driver/mysql` for MariaDB access

## Error Policy

Each manager returns errors to the caller. Managers do not log directly — they return structured errors that include enough context for the caller (eventually the orchestrator or CLI) to produce a useful message. Errors include the operation attempted, the resource involved, and actionable guidance where applicable.
