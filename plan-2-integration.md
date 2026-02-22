---
description: Plan for the GopherCaptain orchestrator, CLI commands, and deploy/upgrade/remove flows
tags: [plan, gophercaptain, integration, orchestrator, cli]
audience: { human: 40, agent: 60 }
purpose: { plan: 95, design: 5 }
---

# Plan: Integration

Implements: [GopherCaptain Design](design.md) — Orchestrator, CLI commands, Deploy/Upgrade/Rollback/Remove flows

## Scope

**Covers:**
- Orchestrator that coordinates all managers for each flow
- Rollback logic for partial deploy failures
- CLI commands: `deploy`, `upgrade`, `rollback`, `remove`, `list`, `status`, `inspect`
- Output formatting for all commands
- End-to-end flows as described in the design

**Does not cover:**
- Individual manager implementations (Plan: Foundation)
- State store schema or CRUD (Plan: Foundation)
- `gophercaptain init` command (Plan: Foundation)

## Enables

Once integration exists:
- **The tool is usable end-to-end** — deploy a real service from GitHub to a running, routable, database-backed state
- **Upgrade and rollback work** — confidence to push new versions to running services
- **Removal is clean** — every artifact the tool created gets removed
- **Visibility commands work** — `list`, `status`, `inspect` give full picture of what's deployed

## Prerequisites

- All foundation components from Plan: Foundation implemented and individually tested
- At least one Go project with a GitHub release containing a linux/amd64 binary for end-to-end testing
- EC2 instance with `gophercaptain init` already run

## North Star

Every flow either completes fully or leaves the system exactly as it was before. The operator never has to clean up after a failed deploy. Every command produces output that tells the operator what happened, not just whether it succeeded.

## Done Criteria

### Orchestrator

- The orchestrator shall accept a deploy request (service name, repo, version, port, route, extra env, flags) and execute the full deploy flow from the design
- The orchestrator shall execute deploy steps in order: validate, fetch binary, create database, write env, write systemd unit, write nginx config, record state
- When any step after validation fails, the orchestrator shall roll back all completed steps in reverse order
  - Binary fetched → remove binary and symlink
  - Database created → drop database and user
  - Env file written → remove env file and directory
  - Systemd unit written → disable, remove unit, daemon-reload
  - Nginx config written → remove config and symlink, reload nginx
- When rollback itself fails, the orchestrator shall report both the original error and the rollback failure, listing what remains to be cleaned up manually
- The orchestrator shall accept an upgrade request and execute: fetch new binary, stop service, update symlink, start service, health check, prune old versions, update state
  - When health check fails after upgrade, the orchestrator shall swap the symlink back to the previous version and restart
- The orchestrator shall accept a rollback request and execute: stop service, swap symlink to previous version, start service, update state
  - When no previous version exists, the orchestrator shall return an error
- The orchestrator shall accept a remove request and execute the remove flow from the design
  - When `--drop-db` is set without `--yes`, the orchestrator shall return the database and user names and require confirmation before proceeding

### Deploy Command

- `gophercaptain deploy <repo>` shall parse all flags from the design CLI specification
- When `--name` is omitted, the command shall derive the service name from the repo name
- When `--route-type` is omitted, the command shall infer type from `--route` format: contains `.` → subdomain, starts with `/` → path
- When `--route` is omitted, the command shall deploy without nginx routing (systemd + database only)
- When `--no-db` is set, the command shall skip database creation and omit `DB_*` variables from the env file
- The deploy command shall print a summary on success matching the design output format:
  ```
  ✓ <name> <version> deployed
    Port:     <port>
    Route:    <route> → localhost:<port>
    Database: <db_name>
  ```
- When deploy fails, the command shall print what went wrong and confirm rollback completed

### Upgrade Command

- `gophercaptain upgrade <service>` shall accept `--version` (default: latest)
- When the service does not exist in state, the command shall fail with a suggestion to run `list`
- When the requested version matches the current version, the command shall report no change needed
- The upgrade command shall print the version transition: `✓ <name> upgraded v1.2.0 → v1.3.0`
- When automatic rollback occurs, the command shall print that it rolled back and why

### Rollback Command

- `gophercaptain rollback <service>` shall swap to the previous version
- When no previous version is recorded, the command shall fail with explanation
- The rollback command shall print: `✓ <name> rolled back to <prev_version>`

### Remove Command

- `gophercaptain remove <service>` shall stop and remove all artifacts
- When `--drop-db` is set, the command shall print the database and user that will be dropped and prompt for confirmation
- When `--yes` is set, the command shall skip confirmation
- The remove command shall print each step as it completes:
  ```
  Stopping gc-<name>...
  Removing systemd unit...
  Removing nginx config...
  Removing env and binaries...
  Dropping database gc_<name>...
  ✓ <name> removed
  ```
- When the service does not exist in state, the command shall fail with a suggestion to run `list`

### List Command

- `gophercaptain list` shall query all services from state and print a table:
  ```
  NAME    VERSION   PORT   ROUTE                STATUS
  ```
- The STATUS column shall reflect live `systemctl is-active` output, not stored state
- When no services are deployed, the command shall print a message suggesting `deploy`

### Status Command

- `gophercaptain status <service>` shall print: service name, repo, version, previous version, port, route, database name, systemd status, deployed/updated timestamps
- The command shall query live systemd status, not stored state

### Inspect Command

- `gophercaptain inspect <service>` shall print the generated systemd unit, nginx config, and env file
- Credential values in the env file shall be redacted (replaced with `****`)
- The command shall read files from disk, not reconstruct from state

### Health Check

- After starting a service (deploy or upgrade), the orchestrator shall poll `localhost:<port>` with a TCP connection
- The health check shall retry every 1 second for up to 10 seconds
- When the check passes, the service is considered healthy
- When the check fails after 10 seconds, the service is considered unhealthy and triggers rollback (upgrade) or deploy failure

## Constraints

- **No partial success for deploy** — either everything succeeds or everything rolls back; design requires atomic deploys
- **Nginx failure is non-fatal for deploy** — design specifies: roll back nginx config, warn, but complete the deploy. The service runs without routing.
- **No interactive prompts except remove confirmation** — all other commands either succeed or fail. Confirmation uses stdin, skippable with `--yes`.
- **No background operations** — all commands run synchronously and return when complete
- **History recorded for every mutation** — deploy, upgrade, rollback, and remove all write to the history table

## References

- [GopherCaptain Design](design.md) — deploy/upgrade/remove flow diagrams, CLI specification, error handling table
- [Plan: Foundation](plan-1-foundation.md) — manager interfaces this plan depends on
- [cobra](https://github.com/spf13/cobra) — `github.com/spf13/cobra` for CLI framework
- [tablewriter](https://github.com/olekukonez/tablewriter) — `github.com/olekukonez/tablewriter` for `list` output formatting (or simpler `text/tabwriter` from stdlib)

## Error Policy

CLI commands print errors to stderr and exit with code 1. Error messages follow the pattern: what failed, why, and what to do about it. The orchestrator's rollback errors are surfaced to the operator with a list of artifacts that may need manual cleanup.

Successful commands exit with code 0 and print results to stdout. This supports scripting (e.g., `gophercaptain deploy myapi && echo "deployed"`).
