# Foreground runner

## Status

| MVP | Scope |
|---|---|
| Built | Script services, bounded parallel execution, graceful stop, logs, optional systemd sandbox and Claude/Codex push adapters. |
| Pending | Installed [runtime sidecar isolation and recovery](#runtime-isolation-and-recovery). |

## What the runner does

`agent-bus start` stays in the foreground, registers a service, obtains its
credential and reads its inbox. It runs the script once per message. It does
not manage installed instances, autostart, reload or restart policies.

## Script services

| Form | Input | Output |
|---|---|---|
| `--algo=args` | Message body as the script's argument | Stdout at exit |
| `--algo=json` | Envelope JSON on stdin; the default form | Stdout at exit |

```sh
agent-bus start hello@srv1 --algo=args ./hello-world.sh --descr "greets you"
cat service.json | agent-bus start -5
```

| Rule | Built behavior |
|---|---|
| Script argument | One shell command line; quote it if it contains arguments |
| `-N` | At most N script processes; default 1. One runner reads one inbox |
| Success with output | Send the output as the answer |
| Silent success | Send `done` so the caller can stop waiting |
| Nonzero exit | Log failure and send no answer; no retry |
| Expired caller deadline | Skip the script; message TTL is enforced separately in the daemon |
| Stop | Stop taking work and wait for scripts already running; leave the registry record and queue intact |
| Work directory | One per service; the script starts there |
| Credentials | The launcher must be allowed to obtain the service's credential; it cannot become somebody else's service |

### Stopping it and reading what it said

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
| starting a name that is already running here | is refused; the foreground `start` command has no shared-start mode |

Because a message is taken from the daemon only when a script process is free
to run it, a service that dies loses only the work already in flight; the rest
is still queued for whatever reads that inbox next.

## What the child is told

The script receives message id, sender, receiver, topic, tag and its work path
through environment variables. Their exact names are owned by
[the runner source](../src/cmd/agent-bus/start.go). These values describe the
work and authenticate nothing. Managed configuration layers are not built.

## Who it runs as

The foreground runner executes as its launching user. It is not a child of the
daemon. The separate account prepared by setup does not imply a managed runner
program or unit is installed; see [setup](09-setup.md#the-two-accounts).

## Adapters

**Agent runtimes get one push adapter each**: each runtime takes a message
differently, so each gets its own adapter that reads the session's queue and
pushes into the *running* session. Claude Code and Codex are proven here — `src/mcp`, run by `smoke.sh`.

| Runtime | Push path | Status |
|---|---|---|
| Claude Code | Channels — `claude --channel`, `notifications/claude/channel`, reply tool | built and smoke-tested here |
| Codex | App Server — `turn/steer` if busy, `turn/start` if idle, `thread/resume` after restart. **Two transports**: told where a shared app-server is, it attaches over a loopback WebSocket; told nothing, it spawns `codex app-server` and speaks newline-delimited JSON-RPC on stdio. What is ruled out is a WebSocket over a **unix socket**, which bun cannot open | both proven in the live run |
| ChatGPT | none — cannot be pushed; pull through the MCP inbox only | pull only |

The adapter acknowledges to the bus only after the runtime has *accepted* the
message. **Acceptance is not processing** — Claude Channels expose no later
"the model handled it" signal — so anything that needs to know the work was
done waits for a correlated reply, never for the ack. It never lets an incoming message change the session's permissions or
mode — that is the adapter's policy as a receiver
([messaging § envelope](04-messaging.md#envelope)), not a rule of the bus.


## Runtime isolation and recovery

**Required MVP, pending acceptance.** Every shipped runtime's session-control
endpoints must refuse other OS accounts while the intended terminal, tools
and pusher remain usable. Loopback reachability and distinct session names
alone do not establish that boundary. The current Codex WebSocket path needs
an installed cross-account check; the review has not established unauthorized
attachment. [H.9.5](../Plans/MVP/TODO.md#remaining-work) owns this verification
and any necessary correction.

After a daemon restart or sidecar failure, an open interactive session must
resume bus delivery or clearly report that integration is inactive and how
to recover. Recovery instructions must lead to a successful correlated
exchange in that session. This adds no replay or exactly-once guarantee:
[consumption](04-messaging.md#one-reader-per-inbox) and
[snapshots](04-messaging.md#durability) retain their loss boundaries.
[H.9.6](../Plans/MVP/TODO.md#remaining-work) owns live acceptance.

## Sandboxing

Off by default; `--sandbox on|off` selects it explicitly. The built backend is
`systemd-run --user` behind the sandbox port. Asking for confinement without a
working user manager fails rather than silently running unconfined. Startup
reports the selected backend.

| Profile | Behavior |
|---|---|
| Filesystem | `ProtectSystem=strict`, home read-only, private temporary directory |
| Work | Work directory bound in and writable; script directory bound read-only |
| Network | Private network unless explicitly requested |
| Privilege and resources | No new privileges; process and memory limits |

The exact invocation is in [sandbox source](../src/internal/sandbox/sandbox.go).
The built backend is tested with positive and negative controls: a script can
write inside its work directory and cannot write outside it or use an ungranted
network.
