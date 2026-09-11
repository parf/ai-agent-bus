# Runner role

A pm2/php-fpm-style supervisor **plus** bus integration **plus** sandboxing:
one supervised process hosting many children and speaking the bus on their
behalf. Always present.

It is **its own process**, because it is the one component that executes code
it did not write — see
[processes § why the runner is its own process](11-processes.md#why-the-runner-is-its-own-process).
So there are two levels: `agent-busd`'s supervisor runs the runner, and the
runner runs user children.

## What the runner does

- **Supervise** — spawn, restart with backoff, stop, reload, log capture, exit
  codes.
- **Represent** — registers each child as a **service**
  ([services § service and template](03-services-and-topics.md#service-and-template)),
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

**Agent runtimes get one push adapter each**: each runtime takes a message
differently, so each gets its own adapter that reads the session's queue and
pushes into the *running* session. V1 implements Claude Code and Codex, which
is why the status column says those paths are proven — but V1's adapters are
read for their shape, not carried over ([stages § PoC](12-stages.md#poc)).

| Runtime | Push path | Status |
|---|---|---|
| Claude Code | Channels — `claude --channel`, `notifications/claude/channel`, reply tool | proven in V1 |
| Codex | App Server — `turn/steer` if busy, `turn/start` if idle, `thread/resume` after restart. **Two transports**: told where a shared app-server is, it attaches over a loopback WebSocket; told nothing, it spawns `codex app-server` and speaks newline-delimited JSON-RPC on stdio. What is ruled out is a WebSocket over a **unix socket**, which bun cannot open | both proven in the live run |
| OpenCode (Z.AI) | ❓ to be found — the owner has an account. *Settled by:* one spike | maybe |
| ChatGPT | none — cannot be pushed; pull through the MCP inbox only | pull only |

The adapter acknowledges to the bus only after the runtime has *accepted* the
message. **Acceptance is not processing** — Claude Channels expose no later
"the model handled it" signal — so anything that needs to know the work was
done waits for a correlated reply, never for the ack. It never lets an incoming message change the session's permissions or
mode — that is the adapter's policy as a receiver
([messaging § envelope](04-messaging.md#envelope)), not a rule of the bus.

## Script services

**A shell script is a service.** One command registers it, reads its inbox and
answers from the script's output — no adapter code, no library, no knowledge of
the bus inside the script.

```sh
agent-bus start hello@srv1 --algo=args ./hello-world.sh --descr "greets you"
cat service.json | agent-bus start -5       # same fields as JSON, five at a time
```

`hello-world.sh` is `echo "Hello $1"`, and that is the whole service.

| | The message arrives as | The reply is |
|---|---|---|
| **`--algo=std`** | the whole **envelope as JSON on stdin**, one line | whatever it writes to **stdout** |
| **`--algo=args`** | the body as **`$1`**, nothing on stdin | the same |

Both forms also get the envelope in the environment — sender, topic, tag,
`message_id` — so a script that cares can route on it, and one that does not
can ignore it ([messaging § envelope](04-messaging.md#envelope)).

| Rule | |
|---|---|
| exit 0 | stdout is the reply; **empty stdout is `done`** — the work finished with nothing to return, and the caller hears that instead of waiting ([messaging § receipts](04-messaging.md#receipts)) |
| exit non-zero | no reply, logged with stderr. Nothing retries it |
| one process per message | no state between messages |
| `-N` | how many script processes may run **at once**; default 1, so a script that is not safe to run twice does not have to be |
| the script is one argument | it is a shell command line, so quote it if it has arguments of its own: `"./greet.sh --loud"` |
| registered at start | the description is what `ls` and the MCP catalog show. **`stop` does not unregister it**: the name still owns its queue and messages still wait in it, which is the whole point of a name-owned inbox ([messaging § inbox queues](04-messaging.md#inbox-queues)). What changes is that nothing is reading it |
| started by its owner | the process *becomes* the service, so it needs that name's credential, and only the name's owner may have one ([access § getting a token](02-access.md#getting-a-token)). Starting somebody else's service is refused, not silently run under your own name |
| one work directory | the one place it writes, and its working directory. Per service, so two services cannot tread on each other |

`-N` does not mean N services or N inboxes. **The `start` process is the one
reader of that inbox** ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox));
it hands messages to a pool of at most N script processes, and none of them
knows the bus exists. One name, one queue, N hands.

**Stopping is graceful and nothing more.** Ctrl-C or `SIGTERM` ends the wait
for the next message and takes no more; the scripts already running are waited
for, however long they take. Timeouts and restart are supervision, and that is
the runner proper — a script service is a foreground process, and this is the
runner's shell adapter with the supervision taken out.

### Stopping it, and reading what it said

The daemon does not know **where** a script service runs: the registry holds a
description of it, not a handle to it. So a running service leaves a note of
itself in **its owner's own state directory**, and the two verbs read that.
Which is also what makes *only whoever started it may stop it* true with no
check in it — nobody else can see the file.

| | |
|---|---|
| `agent-bus stop <name>` | `SIGTERM`, and then it **waits**. Stopping is graceful, so the verb does not report a stop that has not finished, and it ends one service without touching its siblings |
| `agent-bus logs <name> [--lines N] [--follow]` | what that service and its scripts wrote. The log outlives the run on purpose: what a run said is most wanted once the run has ended |
| a note with no process behind it | is cleared, not reported as a running service — a service killed outright leaves its note behind |
| starting a name that is already running here | is refused, and says so. Two readers of one inbox is the mistake ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox)), and a worker pool asks for that on purpose instead |

Because a message is taken from the daemon only when a script process is free
to run it, a service that dies loses only the work already in flight; the rest
is still queued for whatever reads that inbox next.

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
- Per-child identities make each child individually revocable; the runner's
  key authorises registering them.
- Health hints for children are generated by the runner — it knows how it
  launched them.
- Zero-downtime reload (socket inheritance, `cloudflare/tableflip`-style) for
  `agent-busd` itself; 2× RAM during overlap.
