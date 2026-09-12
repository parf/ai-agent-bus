# Runner role

A pm2/php-fpm-style supervisor **plus** bus integration **plus** sandboxing:
one process hosting many children and speaking the bus on their behalf.

**It is not part of `agent-busd`.** It is a separate program, running as its
own user, that reaches the bus like any other citizen — which is what lets the
daemon claim it never executes anything at all
([processes § nothing the daemon runs may exec](11-processes.md#nothing-the-daemon-runs-may-exec)).

**And it is optional.** A service can always be published by hand with one
command ([script services](#script-services)); the runner is a secure and
convenient way to keep a set of them, not a second way to run one. Effectively
it is a wrapper over a series of `agent-bus start` invocations, which is the
property worth protecting: one mechanism, two entry points, nothing to keep in
step.

## What the runner does

- **Supervise** — spawn, restart with backoff, stop, log capture, exit codes.
  Four verbs and no more: **start, stop, restart, enable/disable** — and it is
  `start`, never `run`, the same word whether you sit in front of it or the
  runner does it for you. There is no `reload`, because for a script service
  there is no long-lived child to signal — children are one process per
  message — and a graceful restart already loses nothing: `stop` waits for the
  work in flight, messages queue in the daemon meanwhile, and the next process
  picks them up.
  ❓ *A long-running non-script child that can take `SIGHUP` would be the one
  reason to keep `reload`, and it would make the runner care what kind of child
  it has. Settled by: the owner.*
- **Represent** — registers each child as a **service**
  ([services § service and template](03-services-and-topics.md#service-and-template)),
  heartbeats and reports stats for it, holds and injects the child's identity
  and private config. Children need not know the bus exists — a script under
  `agent-bus start` never handles a credential at all, and a service that links
  a client library lets the library do it.
- **Is itself an agent** — self-registers, self-reports, has its own key, and
  is controllable over the bus — the same verbs, under its owner's ACL.

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
| one work directory | the one place a sandboxed script may write, and its working directory. Per service, so two services cannot tread on each other ([sandboxing](#sandboxing)) |
| `--sandbox on\|off` | on wherever the host can, and the service says at start which it got — an unsandboxed service is never a silence |

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

## What an instance is

A directory — and the directory being there is the **installed** state, not
the running one. It says the instance exists and is configured, and nothing
more.

Three states, each with exactly one home, the way systemd splits them:

| State | Where it lives | Verb |
|---|---|---|
| **installed** | the directory | the upload that creates it |
| **enabled** | an entry in `autostart.json`, in the runner's home ([setup § the two accounts](09-setup.md#the-two-accounts)) | `enable` / `disable` |
| **running** | neither — it is a process | `start` / `stop` |

A list exists because the runner needs one: **at start it has to know what to
bring up and how many of each**, without walking a tree, and an on-demand
service has to be declared somewhere it is not auto-started from. A directory
with no entry is *staged and not started* — how an instance is put in place
before it is turned on, and what `disable` leaves behind.

The registry still answers what exists and what is alive; the runner answers
only what should be up.

| In `service.d/<name>/` | |
|---|---|
| the description | what to register and how to run it — the same fields `agent-bus start` already takes |
| the config | injected into the child by environment or fd, never as a path it could read twice. Write-only ([access to the runner](#access-to-the-runner)) |
| the code, or a link to a template | a template may be a directory with its own owner, or a symlink to where the code really lives, so the runner need not own it |
| the credential | handed in at install; the runner does not mint it |

The **directory name is the service name**, the way a unit file's name is. If
the description carried one too they could disagree, so it does not.

`templates/` is world-readable because a template is what a service *is* and
holds no secret; `service.d/` is not, because an instance is what a service is
*configured with* ([setup § the two accounts](09-setup.md#the-two-accounts)).
One template with many instances is already in the naming —
`template/instance-name@host` ([identity § names](01-identity.md#names)) — so
per-user instances need no new idea.

## Who it runs as

- **Separate user** (default for shared/server use): the daemon as
  `agent-busd` and the runner as `agent-bus-runner`, two accounts that cannot
  read each other's home ([setup § the two accounts](09-setup.md#the-two-accounts)).
  Privileged installer once; **no root at runtime**, and the only capability
  anywhere is the supervisor's
  ([processes § why the supervisor holds CAP_CHOWN](11-processes.md#why-the-supervisor-holds-cap_chown)).
- **Current user** (personal use): children as you, no runner at all. Zero
  setup — the laptop story with the AUTH role off.

### Where it runs

The runner and the daemon are separate programs, so they need not share a
host. Three arrangements, and the third is what the split buys:

| | daemon | runner |
|---|---|---|
| laptop | yours, local | none — publish by hand ([script services](#script-services)) |
| one host | local, `agent-busd` | local, `agent-bus-runner` |
| **edge box** | **elsewhere** | local, alone — a machine that hosts services and holds no bus state |

A remote daemon is reached the way anything else here is reached, over **ssh**
([access § the three doors](02-access.md#the-three-doors)): no port opened to
a network, and no TLS between bus citizens.

**A bus that is away is not a service that failed.** When the daemon is
unreachable the services are running perfectly well and simply cannot take
work — so the *client* waits and reconnects, and the runner restarts nothing.
Getting that backwards turns one restart of the bus into a restart storm on
every host at once.

| what happened | who deals with it |
|---|---|
| the child died | the runner restarts it |
| the bus is away | the client reconnects, with backoff |

### Access to the runner

**Whoever can reach the runner may install and configure instances on that
host.** The grant is deliberately coarse — the same granularity as write
access to a unit directory, or sudo to a service account — and it is bounded
by the thing that matters:

> runner access lets you run code on that host. It does not let you
> impersonate a name on the bus.

An instance still has to *become* a name, and it can only become one whose
credential it was handed. Which is why the runner is **never given the power
to mint one** — if it could, runner access and impersonation would be the same
thing, and the whole split would be decorative.

**Configuration is write-only.** A config goes in and is never handed back:
anyone who could print one could read every secret on the host, and the
identity in the ssh key would buy nothing. Whoever truly needs the bytes can
become root and read the file — that is the boundary, stated rather than
worked around with a verb.

The consequence worth planning for: nobody can ask the host what is deployed.
`service.d` is a **write-only deployment target**, and the source of truth
lives where it was installed from.

## Sandboxing

**Off by default, opted into per service** in `autostart.json` — the same place
the worker count and the rest of the non-secret run options live
([what an instance is](#what-an-instance-is)).

It is off because the layout no longer needs it to be on. A child runs from
its own `service.d` entry, which is world-readable and holds no secret by
construction; its configuration reaches it **injected as environment before
exec**, never as a path it could open. So nothing secret sits on any path the
child uses, and confinement stops being the thing that makes the arrangement
correct and becomes what it should have been all along: hardening a host asks
for when it wants it.

**One backend, and off.** `systemd-run --user` is it — it gives cgroups and
the `Protect*` / `Private*` set declaratively, on every host that has systemd,
with no privilege of its own. A second backend is one adapter behind the same
port ([modules § the rule](10-modules.md#the-rule)) the day somebody runs
this where systemd is not, and until then it is surface with nothing behind
it. The candidates, so the choice is a record and not a memory:

| Tool | Gives | Why not now |
|---|---|---|
| **`systemd-run`** | cgroups + `Protect*` / `Private*` / seccomp / caps, declarative | — it is the one |
| `bubblewrap` | user namespaces, minimal rootfs, only declared paths; no systemd needed | the adapter to write when a host has no systemd |
| `unshare` | the bare primitive; the runner would do the setup itself | everything `bwrap` already did correctly |
| firejail | more features | weaker security history |

`--user` has one consequence worth writing down: a transient user scope needs
that account's own systemd manager to be running. A person starting a service
in their own session has one. The **service account does not**, having no
login, until `loginctl enable-linger agent-bus-runner` is run — so the day the
runner starts children as `agent-bus-runner`, that is a setup step
([setup § the two accounts](09-setup.md#the-two-accounts)) and not a
mystery about why the sandbox says `off`.

**Off is a setting, not an absence.** Where `systemd-run` cannot run the
runner says so at start and carries on unsandboxed — never silently, because
a sandbox that quietly did nothing is worse than none. Asking for it
explicitly where it is unavailable is an error instead.

Default profile — **nothing writable but the work directory, and no network**:

| Property | |
|---|---|
| `ProtectSystem=strict` | the whole filesystem read-only |
| `ProtectHome` | `read-only` for a service published by hand, whose script usually *is* in a home. **`yes` for anything the runner starts** — its home is `service.d`, so read-only there would hand every child every other service's secrets, and the mode on the directory would be decoration |
| `PrivateTmp=yes` | its own `/tmp` |
| `ReadWritePaths` / `BindPaths` the work dir | the one place it may write, and its working directory. Bound in, so it is reachable even when it is under the private `/tmp` |
| `BindReadOnlyPaths` the script's own directory | the same problem from the other side: a private `/tmp` hides the script too |
| `PrivateNetwork=yes` | unless the service declares it needs one |
| `NoNewPrivileges=yes`, `TasksMax`, `MemoryMax` | no escalation, and a bound on what it can spend |

A child's registration declares what it needs (network, paths, sockets); the
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
