# Runner — the supervisor role of `agent-busd`

Status: design, main ideas only

## Names

- **`agent-busd`** — *the* daemon (`sshd`/`dockerd` convention): registry, queues,
  API, MCP server, WEB, optional AUTH, and the runner, in one binary
- **`agent-bus`** — the CLI: identity (`keygen`), registry (`register`,
  `topic create`), messaging (`send`, `publish`, `consume`), runner control
  (`start`, `stop`, `ls`, `logs`), AUTH admin (`auth sign`, `auth admin`)
- **`agent-bus`** — the keyword everywhere else: `/etc/agent-bus/`,
  `~/.config/agent-bus/`, the `agent-bus` system user, `agent-busd.service`
- **`ab_`** — MCP tool prefix only (`ab_list_services`, `ab_call`); never in CLI or config

## Implementation

- **Go** first (`agent-busd`, `agent-bus`); a **bun/NPM** build later.
- Client libraries: **Go, PHP, Rust, JS, Python**.

## What the runner does

A pm2/php-fpm-style supervisor **plus** bus integration **plus** sandboxing:
one supervised process hosting many instances and speaking the bus on their behalf.

- **Supervise** — spawn, restart with backoff, stop, reload, log capture, exit codes.
- **Adapt** — children come in a few shapes; each gets the same bus face:
  - MCP servers (stdio) → MCP-capable services in the registry
  - HTTP/REST/any API → registered with health hints
  - shell processes (stdin/stdout) → request/response or stream services
  - ad-hoc spawn/control → the runner's own API to start/stop things on demand
  - **agent runtimes → one push adapter per runtime**, borrowed from V1's
    notifiers: each runtime takes a message differently, so each gets its own
    adapter that reads the session's queue and pushes into the *running* session.

    | Runtime | Push path | Status |
    |---|---|---|
    | Claude Code | Channels — `claude --channel`, `notifications/claude/channel`, reply tool | yes |
    | Codex | App Server over a private unix socket — `turn/steer` if busy, `turn/start` if idle, `thread/resume` after restart | yes |
    | OpenCode (Z.AI) | ❓ to be found — the owner has an account. *Settled by:* one spike | maybe |
    | ChatGPT | none — cannot be pushed; pull through the MCP inbox only | pull only |

    The adapter acknowledges to the bus only after the runtime has *accepted*
    the message. It never lets an incoming message change the session's
    permissions or mode — that is the adapter's policy as receiver (`03`), not
    a rule of the bus.
- **Represent** — registers each child as an **instance** (`name@host`),
  heartbeats and reports stats for it, holds and injects the child's identity
  and private config. Children need not know the bus exists.
- **Is itself an agent** — self-registers, self-reports, has its own key, is
  controllable over the bus (start/stop children, reload) under its owner's ACL.

## Supervises itself first

The first children are `agent-busd`'s **own roles**: the **WEB** dashboard
(cgroup-limited) and, with `auth: on`, the **AUTH** role (alone holds
`master_secret`; unix socket to the core). Same spawn/restart/limits machinery
as any child; no special cases. ❓ Further split and what is shared between
core and children — open (`00`).

## Who it runs as

- **Separate user** (default for shared/server use): `agent-busd` as the
  `agent-bus` user (or per-tenant users); children as that user or further
  dropped. Privileged installer once; no root at runtime.
- **Current user** (personal use): runner and children as you. Zero setup —
  the laptop story with the AUTH role off.

## Sandboxing — on by default, per child

A **profile** attached to each child; a sane default applies unless the owner
opts out.

| Layer | Tool | Gives |
|---|---|---|
| Resource limits | cgroups (via systemd or direct) | CPU / memory / pids / io caps |
| Isolation | `systemd-run` (when systemd present) | cgroups + `Protect*` / `Private*` / seccomp / caps, declarative |
| Isolation, rootless | `bubblewrap` | user namespaces, minimal rootfs, only declared paths; no root, no systemd |
| Bare primitive | `unshare` | fallback; runner does the setup itself |
| Alternative | firejail | more features, weaker security history — lowest on the list |

Backend by environment: systemd → `systemd-run`; no systemd or unprivileged →
`bwrap`; otherwise raw `unshare`; `off` allowed explicitly.

Default profile: private `/tmp`, read-only system, only the child's work dir
writable, no network unless declared, pids/memory cap, no new privileges. A
child's registration declares what it needs (network, paths, sockets); the
runner grants exactly that.

## In-process queue — local work inside a service

Bounded Go channel per named queue: `Push` non-blocking → `ErrFull`
(backpressure); `Pop(ctx)` blocking; N goroutines = consumer group.
Non-durable by design; if one queue ever needs durability, back *that one* with
a WAL file. Cross-service messages go through `agent-busd`'s topics, not these.

## Fits the other pieces

- Child private config: local file or sealed blob from `agent-busd`, decrypted
  by the runner and passed via env/fd — never plaintext on disk inside the sandbox.
- Per-child identities make each instance individually revocable; the runner's
  key authorises registering them.
- Health hints for children are generated by the runner — it knows how it
  launched them.
- Zero-downtime reload (socket inheritance, `cloudflare/tableflip`-style) for
  `agent-busd` itself; 2× RAM during overlap.
