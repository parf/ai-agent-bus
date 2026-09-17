# Layers and modules

📌 **TL;DR:** Core decides; ports isolate dependencies; faces translate requests.

## Status

The table below describes built code. Future packages and client-library plans
are not dependencies of the current system.

## The rule

Protocol and ports sit inward; core owns registry, queues and access decisions.
Storage, directory, signature and sandbox implementations sit behind their
ports. Command entry points assemble them. HTTP faces translate requests and
clients may compose operations without deciding registry authority.

Core never imports an adapter. External I/O needed by core goes through a port.
Process startup and listener management belong to the command that assembles
the runtime, not to the domain core.

## Modules

| Built area | Responsibility |
|---|---|
| `internal/protocol` | Names, records, envelopes and JSON representation |
| `internal/ports` | Credential storage, snapshot, directory, signature and sandbox interfaces |
| `internal/core` | Registry, queues, ownership, ACL, enrolment and counters |
| `internal/auth` | Tokens and browser sessions |
| `internal/store`, `internal/dump` | Text-file credentials, memory test store and JSON snapshots |
| `internal/directory`, `internal/signature` | Public-key lookup and system signature verification |
| `internal/sandbox` | Current confinement backend |
| `internal/api` | HTTP routes, credentials and errors |
| `internal/version`, `internal/proctitle` | Shared program version, build stamp and process titles |
| `cmd/`, `mcp/` | Program entry points and client faces |

## Languages

Go implements the daemon, CLI and administrative programs. TypeScript on bun
implements the MCP face and push adapters. The process-title helper needs cgo;
[setup § build information](09-setup.md#build-information) owns build requirements.
This is not a claim that the executable has no native dependencies.

## External tools

Prefer the language's built-in, then an existing system tool for occasional
work, then an established library for work that must stay in-process. Never
write custom cryptographic primitives or invent a new protocol for convenience.

### Our own small module

Small shared plumbing belongs in an internal module when repeated code has
established a common responsibility. A dependency is not needed for a line
reader or a pending-request map. This exception does not extend to crypto or
wire-format invention.

### HTTP is built in

Use the built-in HTTP client for API and public-key requests. Shell services
may use `curl`, because that is their ordinary client.

### What we do shell out to

| Built operation | System tool |
|---|---|
| Sign or verify possession | `ssh-keygen` |
| Optional script confinement | `systemd-run --user` |
| Supervised web confinement | `bwrap`; [runtime requirements](09-setup.md#install) |
| Install accounts and daemon unit | Host account tools and systemd |
| Switch to the admin account | `sudo` |

## What this buys

A storage or directory implementation can be replaced behind its port. No
future backend is required to make the current boundary real; smoke checks
both the prohibited inward imports and actual adapter assembly.
