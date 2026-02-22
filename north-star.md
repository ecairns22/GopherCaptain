---
description: Vision for GopherCaptain - deploys Go services from GitHub releases to EC2 with systemd, nginx, and MariaDB
tags: [gophercaptain, deploy, golang, ec2, systemd, nginx, mariadb, github-releases]
audience: { human: 75, agent: 25 }
purpose: { north-star: 100 }
---

# Service Deployment: What Great Looks Like

> One command to go from a GitHub release to a running, routable, database-backed service.

You type a command with a repo name and a version. The tool pulls the binary from your GitHub releases, drops it on your EC2 instance, stands up a systemd unit on the next available port, wires nginx to route traffic to it, creates a MariaDB database with scoped credentials, and hands the connection string to the service. You do this again for a second project. And a third. Each one lands cleanly alongside the others, no collisions, no manual config files, no SSH sessions to remember what you did last time. A week later you check status and see every service, its port, its route, and whether it's healthy. You upgrade one of them by running the same command with a new version.

---

## Installation

- You should be able to deploy a Go binary from any of your GitHub repos with one command
- You should be able to specify a release version or default to the latest
- You should be able to install a new service without affecting running services
- You should be able to see what was installed, when, and which version

---

## Service Lifecycle

- You should be able to start, stop, restart, and remove any deployed service
- You should be able to upgrade a service to a new release version in place
- You should be able to roll back to the previous version if an upgrade fails health checks
- You should be able to see the status of all services at a glance

---

## Port Management

- You should be able to deploy a service without manually choosing a port
- You should be able to override the port when you have a reason to
- You should be able to trust that no two services share a port
- You should be able to see which port every service is using

---

## Systemd Integration

- You should be able to trust that a deployed service starts on boot
- You should be able to trust that a crashed service restarts automatically
- You should be able to read a service's logs through standard journalctl
- You should be able to pass environment variables and flags to the service at deploy time

---

## Nginx Routing

- You should be able to route a subdomain or path to a service with one argument
- You should be able to serve multiple services behind a single domain
- You should be able to trust that nginx reloads cleanly after configuration changes
- You should be able to remove a service and have its route disappear

---

## Database Management

- You should be able to deploy a service and have its database created automatically
- You should be able to trust that each service gets its own database and credentials
- You should be able to trust that credentials are never written to disk in plaintext outside of protected locations
- You should be able to drop a service's database when removing the service, with confirmation

---

## Visibility

- You should be able to list all deployed services with their version, port, route, and health
- You should be able to check whether a service is reachable through nginx
- You should be able to see the configuration the tool generated for any service
- You should be able to audit what the tool changed on the system

---

## What Great Looks Like

| Declaration | Why It Matters |
|-------------|----------------|
| One command from release to running service | No multi-step manual process to forget or botch |
| Each service isolated in its own unit, port, database | No collisions, no shared state between services |
| Upgrade and rollback without touching other services | Confidence to ship updates |
| All generated config is visible and auditable | You can debug without reverse-engineering the tool |
| Removal cleans up everything the tool created | No orphaned config, ports, databases, or units |

---

## Anti-Patterns

| Don't | Do Instead |
|-------|------------|
| Require SSH and manual edits after deploy | Everything configured through the tool |
| Store credentials in environment files anyone can read | Scoped credentials in protected locations |
| Assume the operator remembers what port is free | Track and assign ports automatically |
| Leave orphaned nginx configs after removal | Clean up everything the tool created |
| Require the operator to write systemd units | Generate units from deploy-time parameters |

---

*You should be able to point at a GitHub release and have a production-ready service running in under a minute, every time, with nothing left to remember.*
