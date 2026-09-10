# Runner role

A pm2/php-fpm-style supervisor **plus** bus integration **plus** sandboxing:
one supervised process hosting many instances and speaking the bus on their
behalf. Always present.

It is **its own process**, because it is the one component that executes code
it did not write — see
[processes § why the runner is its own process](11-processes.md#why-the-runner-is-its-own-process).
So there are two levels: `agent-busd`'s supervisor runs the runner, and the
runner runs user children.

## What the runner does

- **Supervise** — spawn, restart with backoff, stop, reload, log capture, exit
  codes.
- **Represent** — registers each child as an **instance**
  ([services § service and instance](03-services-and-topics.md#service-and-instance)),
  heartbeats and reports stats for it, holds and injects the child's identity
  and private config. Children need not know the bus exists.
- **Is itself an agent** — self-registers, self-reports, has its own key, and
  is controllable over the bus (start/stop children, reload) under its owner's
  ACL.

## Adapters

Children come in a few shapes; each gets the same bus face.

| Shape | Becomes |
|---|---|
| MCP servers (stdio) | MCP-capable services in the registry |
| HTTP/REST/any API | registered with health hints |
| shell processes (stdin/stdout) | request/response or stream services |
| ad-hoc spawn/control | the runner's own API to start/stop things on demand |

**Agent runtimes get one push adapter each**, borrowed from V1's notifiers:
each runtime takes a message differently, so each gets its own adapter that
reads the session's queue and pushes into the *running* session.

| Runtime | Push path | Status |
|---|---|---|
| Claude Code | Channels — `claude --channel`, `notifications/claude/channel`, reply tool | yes |
| Codex | App Server over a private unix socket — `turn/steer` if busy, `turn/start` if idle, `thread/resume` after restart | yes |
| OpenCode (Z.AI) | ❓ to be found — the owner has an account. *Settled by:* one spike | maybe |
| ChatGPT | none — cannot be pushed; pull through the MCP inbox only | pull only |

The adapter acknowledges to the bus only after the runtime has *accepted* the
message. It never lets an incoming message change the session's permissions or
mode — that is the adapter's policy as a receiver
([messaging § envelope](04-messaging.md#envelope)), not a rule of the bus.

## Supervises itself

`agent-busd`'s own children — bus, runner, web, auth, billing, health — are
spawned with the same machinery as any user child: same restart policy, same
limits, no special cases. The list and the privilege each one gets are in
[processes](11-processes.md).

## Who it runs as

- **Separate user** (default for shared/server use): `agent-busd` as the
  `agent-bus` user (or per-tenant users); children as that user or further
  dropped. Privileged installer once; **no root at runtime**, and the only
  capability anywhere is the supervisor's
  ([processes § why the supervisor holds CAP_CHOWN](11-processes.md#why-the-supervisor-holds-cap_chown)).
- **Current user** (personal use): runner and children as you. Zero setup —
  the laptop story with the AUTH role off.

## Sandboxing

On by default, per child: a **profile** attached to each, with a sane default
unless the owner opts out.

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

## In process queue

Local work *inside* a service — not a bus feature. Bounded Go channel per
named queue: `Push` non-blocking → `ErrFull` (backpressure); `Pop(ctx)`
blocking; N goroutines = consumer group. Non-durable by design; if one queue
ever needs durability, back *that one* with a WAL file. Cross-service messages
go through topics ([messaging](04-messaging.md)), not these.

## Fits the other pieces

- Child private config: local file or sealed blob from `agent-busd`
  ([identity § sealed private config](01-identity.md#sealed-private-config)),
  decrypted by the runner and passed via env/fd — never plaintext on disk
  inside the sandbox.
- Per-child identities make each instance individually revocable; the runner's
  key authorises registering them.
- Health hints for children are generated by the runner — it knows how it
  launched them.
- Zero-downtime reload (socket inheritance, `cloudflare/tableflip`-style) for
  `agent-busd` itself; 2× RAM during overlap.
