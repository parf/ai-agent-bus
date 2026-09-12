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
  Five **control** verbs and no more: **start, stop, restart, reload,
  enable/disable** (`logs` reads, it does not control) — and it is
  `start`, never `run`, the same word whether you sit in front of it or the
  runner does it for you. **`reload` exists for a long-lived child and nowhere
  else** ([long-lived services](#long-lived-services)): there is something to
  signal only when a process is kept, so on a one-process-per-message service
  it is refused rather than quietly doing nothing. Everywhere else a graceful
  restart already loses nothing: `stop` waits for the work in flight, messages
  queue in the daemon meanwhile, and the next process picks them up. This is
  the one place the runner **cares what kind of child it has**, and it is the
  price of keeping a process alive.
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
| shell processes (args or stdin → stdout) | request/response or stream services |
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

| | The message arrives as | The reply is | Processes |
|---|---|---|---|
| **`--algo=args`** | the body as **`$1`**, nothing on stdin | whatever it writes to **stdout**, at exit | one per message |
| **`--algo=std`** | the **body itself on stdin**, bytes, nothing wrapped around it | the same, bytes | one per message |
| **`--algo=json`** | the whole **envelope as JSON on stdin**, one line | the same | one per message |
| **`--algo=jsonl`** | that same line, **and the next, and the next** | one JSON line per message | **one, kept** ([long-lived services](#long-lived-services)) |
| **`--algo=msgpack`** | `uint32` length + **msgpack**, frame after frame | `uint32` length + **msgpack** back, the same framing both ways | **one, kept** |

The names are a ladder, each rung saying one thing more. `args` and `std` name
a **channel** and claim nothing about what travels on it; `json` names the
channel *and* the payload; `jsonl` and `msgpack` are that payload repeated for
as long as the process lives.

**`std` is the binary form, and an image scaler is the whole case for it.**
`convert - -resize 800x -` is already a service — bytes in, bytes out, nothing
to parse and nothing to encode — where argv cannot carry a JPEG at all and a
JSON line can only carry one base64'd, in both directions, for no one's
benefit.

**`msgpack` is the one that gives up nothing**, which is what the set was
missing rather than something added to it:

| | the envelope | a JPEG body | the process |
|---|---|---|---|
| `args` | in the environment only | cannot carry it | one per message |
| `std` | in the environment only | **as bytes** | one per message |
| `json` | in-band | base64, both ways | one per message |
| `jsonl` | in-band | base64, both ways | **kept** |
| **`msgpack`** | **in-band** | **as bytes** | **kept** |

It is not a new encoding either — msgpack is already the envelope's negotiated
binary form ([messaging § envelope](04-messaging.md#envelope)), arriving here
at the child's boundary. **The length prefix is `uint32`, network order**, and
it is what makes the form a stream: a frame that had to be alone would not need
one, which is the same reason `jsonl` keeps its process.

`args` and `json` are the MVP. `std`, `jsonl` and `msgpack` arrive in
[Release 1](12-stages.md#release-1).

All five forms also get the envelope in the environment — sender, topic, tag,
`message_id` — so a script that cares can route on it, and one that does not
can ignore it ([messaging § envelope](04-messaging.md#envelope)).

| Rule | |
|---|---|
| exit 0 | *(the one-per-message forms)* stdout is the reply; **empty stdout is `done`** — the work finished with nothing to return, and the caller hears that instead of waiting ([messaging § receipts](04-messaging.md#receipts)) |
| exit non-zero | *(the same)* no reply, logged with stderr. Nothing retries it. For a **stream form** an exit is not an answer at all, it is the child dying ([long-lived services](#long-lived-services)) |
| one process per message | no state between messages. The **stream forms** are the exception, and keeping state is the whole reason they exist ([long-lived services](#long-lived-services)) |
| `-N` | how many script processes may run **at once**; default 1, so a script that is not safe to run twice does not have to be |
| the script is one argument | it is a shell command line, so quote it if it has arguments of its own: `"./greet.sh --loud"` |
| registered at start | the description is what `ls` and the MCP catalog show. **`stop` does not unregister it**: the name still owns its queue and messages still wait in it, which is the whole point of a name-owned inbox ([messaging § inbox queues](04-messaging.md#inbox-queues)). What changes is that nothing is reading it |
| started by its owner | the process *becomes* the service, so it needs that name's credential, and only the name's owner may have one ([access § getting a token](02-access.md#getting-a-token)). Starting somebody else's service is refused, not silently run under your own name |
| one work directory | the one place a sandboxed script may write, and its working directory. Per service, so two services cannot tread on each other ([sandboxing](#sandboxing)) |
| `--sandbox on\|off` | off unless asked for ([sandboxing](#sandboxing)), and the service says at start which it got — an unsandboxed service is never a silence |

`-N` does not mean N services or N inboxes. **The `start` process is the
reader of that inbox** ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox))
— the only one, unless it said `--share` ([one name on many
hosts](#one-name-on-many-hosts)) — and it hands messages to a pool of at most N
script processes, none of which knows the bus exists. One name, one queue, N
hands.

**Stopping is graceful and nothing more.** Ctrl-C or `SIGTERM` ends the wait
for the next message and takes no more; the scripts already running are waited
for, however long they take. Timeouts and restart are supervision, and that is
the runner proper — a script service is a foreground process, and this is the
runner's shell adapter with the supervision taken out.

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
| starting a name that is already running here | is refused unless it said `--share`. Two readers of one inbox is the mistake ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox)); a pool is not a mistake and says so ([one name on many hosts](#one-name-on-many-hosts)) |

Because a message is taken from the daemon only when a script process is free
to run it, a service that dies loses only the work already in flight; the rest
is still queued for whatever reads that inbox next.

### One name on many hosts

**`--share` makes a service one of a pool, and a pool is a name rather than a
place.** Four image scalers on four servers all read `scaler@pool1`'s inbox and
take turns; nothing says where a member runs, and nothing needs to — which is
what the name has to stop saying ([the name a member registers](#the-name-a-member-registers)). The bus has
allowed this from the start — a reader that asks to share is one of several
([messaging § several readers may wait when they say so](04-messaging.md#several-readers-may-wait-when-they-say-so))
— so `start --share` passes that word through and the daemon learns nothing
new. **One word, two layers, because it is one decision.**
[Release 1](12-stages.md#release-1).

`-N` and `--share` compose and do not overlap: `-N` is how many hands one
process has, `--share` is how many processes there are. Four hosts at `-N 4`
is sixteen scalers on one queue.

What the local refusal was protecting, and what takes its place:

| | |
|---|---|
| starting the name twice on one host | allowed with `--share`, refused without it — in both directions, the way the daemon already refuses a pool member beside an exclusive reader |
| the note in the owner's state directory | one per **process**, not one per name. `stop <name>` stops this host's members, all of them, and waits for each; `logs <name>` merges what they wrote ([stopping it and reading what it said](#stopping-it-and-reading-what-it-said)) |
| registering the name N times | an update, not a collision: one record, one owner, and every member says the same thing about it ([identity § ownership](01-identity.md#ownership)) |
| what the registry shows | the name is up while **any** member is. A pool that is half down is a health matter, not a registry one |
| a reply | goes to whoever sent the message, never to the member that answered, so which one took the work is nobody's business ([messaging § reply routing](04-messaging.md#reply-routing)) |
| what it is still not | a group, a lease or a load balancer. Nothing is remembered between reads, so a member that dies leaves nothing to clean up |

#### The name a member registers

Four runners left to themselves would register `image-scaler@srv1`,
`image-scaler@srv2` and so on — four names, four inboxes, and no pool at all.
Three rules stop that, and only the last one is new:

| | |
|---|---|
| **a pool is one bus** | members that report to different daemons are not a pool, they are two queues with the same idea in them. So the members point their runners at the one daemon, which is the option the runner already has ([setup § the two units](09-setup.md#the-two-units)) and the edge-box arrangement ([where it runs](#where-it-runs)) |
| **the realm is the daemon's, never the runner's host** | a runner on `srv2` reporting to the bus on `srv1` is registering into `srv1`'s realm already. Nothing about where a process sits belongs in the name it serves |
| **so the pool is given a complete name** | `image-scaler@pool1`, a realm that daemon is told to hold. A complete name is taken whole; only a **bare** one is completed with the local host, and that completion is a convenience carrying no authority ([identity § names](01-identity.md#names)). `@srv1` would claim a location false for three members out of four; `@pool1` claims membership, which is true for all of them and survives a member moving |

So a pool needs no naming machinery of its own — its members are simply given
the whole name, as a run option beside `-N` and `--share`. What to watch for is
that **getting it wrong is silent**: a member that fell back to the default is a
perfectly healthy service nobody ever calls.

Which is also why a member **states its hostname at registration**, in a field
of its own: the name no longer carries one, and a listing still has to answer
*where* ([discovery § where a member says it is](05-discovery.md#where-a-member-says-it-is)).

**That the members are interchangeable is the operator's promise**, and the bus
cannot check it any more than it can check that a service does what its
description says. Two hosts serving one name with different code, or different
`--algo`, is a caller getting different answers to the same question — and it
will look like a flaky service, not a misconfigured one.

Under the runner, sharing is a **run option like the worker count**, so it
lives in `autostart.json` with `-N` and the confinement rather than in
`config.json` ([what an instance is](#what-an-instance-is)). Same reasoning as
`-N`'s default of 1: whether it is safe to run this thing twice is a decision
the host is already making.

### Long-lived services

**A child in one of the stream forms is started once and kept** — `jsonl` or
`msgpack` — and messages arrive on its stdin, one frame at a time, for as long
as it lives. Everything expensive to
build — a loaded model, an open database handle, a warm cache — survives
between messages, which is the only reason to want this and the only thing it
buys.

The shape forces the lifetime rather than a flag declaring it: `args` cannot be
long-lived, because argv is fixed at exec; a process that reads a *stream* of
messages is by construction one that stays; and a length prefix is only worth
writing when another frame follows it. So there is no second setting to
disagree with the first — the same reason `env.dist` decides whether a service
needs an instance ([the three env layers](#the-three-env-layers)).

| | |
|---|---|
| **one message at a time** | the runner writes a frame and waits for the answering frame before writing the next. Nothing has to be correlated because nothing is out of order, and `-N` keeps its meaning: N children, N hands on one inbox ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox)) |
| **a deadline per message, and only here** | killing a one-per-message script costs the process and nothing else. Killing a kept child throws away everything it warmed up, and there is no way past a wedged one without doing it — so the runner waits a bounded time, then kills, restarts with backoff, and logs the message that was in flight as lost |
| **state is the service's own business** | the runner promises nothing about which child handles which message, so whatever a child remembers must not belong to one caller. The first bug here will be a per-caller cache that outlives the caller |
| **up is ready** | no readiness handshake. A child that exits before its first reply is a failed start and backs off like any other |
| **`reload` is `SIGHUP`** | and it is the one verb that exists only for these shapes ([what the runner does](#what-the-runner-does)) |
| **stopping is unchanged** | no further frame is written and the answer in flight is waited for, then `SIGTERM` — bounded by the same deadline, because a wedged child cannot be waited out |
| **a crash still loses only what was taken** | a message leaves the daemon only when a hand is free for it, so the rest is still queued for whatever reads that inbox next |

❓ **Several messages in flight inside one child.** One at a time needs no
correlation; letting a child work on several would, and the envelope already
carries what that costs — a reply matches on topic and tag
([messaging § request and reply](04-messaging.md#request-and-reply)), so the
child would echo the tag and replies could come back in any order. Left open
because nothing needs it yet and adding it later breaks nothing.
*Settled by:* owner, when a service asks for it.

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

❓ **On-demand start is Release 1** ([stages § release 1](12-stages.md#release-1)),
and it is the one place the daemon learns something runner-shaped: a dormant
name needs a **wake-up procedure** configured on it, so that a message arriving
for nobody starts the thing that serves it instead of being refused. Only a
runner can be woken, which is why the mechanism belongs to it rather than to
services in general. *Settled by:* owner, in Release 1.

The registry still answers what exists and what is alive; the runner answers
only what should be up.

Two directories hold it between them, and that split is the point
([setup § the two accounts](09-setup.md#the-two-accounts)):

| | Mode | Holds |
|---|---|---|
| `service.d/<svc>/` | 755 | what the service **is**: the code, or a symlink to the same code elsewhere, plus `config.json` and `env.dist` |
| `runner/<svc>/<instance>/` | 700 | what this instance is **configured with**: an `env` file, and nothing else |

The author and the host each own one file, and they do not overlap:

| | Owned by | Says |
|---|---|---|
| `config.json` | the **author**, and it travels with the code | how the service starts, and what to register. The command line lives here and nowhere else |
| `autostart.json` | the **host** | whether to start it at all, how many, and what to confine it with ([sandboxing](#sandboxing)) |

So a host never restates the command, and an update to it arrives with a `git
pull` rather than as an edit somebody has to remember to make twice.

`service.d` is externally controlled — in most cases a `git clone`, and
nothing a host should be editing by hand. So **`git pull` a service and no
local state moves**: every decision this host made — secrets, worker counts,
confinement, whether it runs at all — is under `runner/`.

The **directory name ties the two sides together**: `runner/mail-reader/`
configures `service.d/mail-reader/`. No symlink and no pointer file — a name is
already unambiguous, and a link could not carry the run options anyway. If
`config.json` named itself too they could disagree, so it does not.

**It is also the default name to register, and a default is all it is.** The
directory gives a bare name, which is completed with the local host the way any
bare name is ([identity § names](01-identity.md#names)) — and `autostart.json`
may state a **complete** name instead, which is then taken whole. That is the
one thing a host must be able to override, because a pool's members sit in
identically named directories on four machines and must register **one** name
between them ([the name a member registers](#the-name-a-member-registers)).
It stays the host's file rather than the author's for the usual reason: which
pool this copy joins is not something the code knows.

One service with many instances is already in the naming —
`template/instance-name@realm` ([identity § names](01-identity.md#names)) — so
per-user instances need no new idea.

### The three env layers

A service's **environment** arrives injected before exec — never as a path the
child could open. It is assembled from three files that overlay in one order.
This is the runner's half, and it is not the registry's configuration, which a
service fetches for itself and which the daemon holds
([services § configuring a template](03-services-and-topics.md#configuring-a-template)):

| | Mode | Carries | Wins |
|---|---|---|---|
| `service.d/<svc>/env.dist` | 755 | the declared surface: every variable the service wants, with a default for each one that is not secret | lowest |
| `runner/<svc>/env` | 700 | what every instance of this service shares — the one API key they all use | middle |
| `runner/<svc>/<inst>/env` | 700 | this instance's own | highest |

**The more secret it is, the more it wins.** Precedence and visibility run in
opposite directions, which is a property a reader can check rather than a rule
to remember. What keeps it true: **`env.dist` may carry a default only for
something that is not secret.** Anything secret is declared with no value —
and that is exactly what makes it required.

Two things fall out of declaring the surface at all:

| | |
|---|---|
| **whether a service needs an instance** | it starts directly if `env.dist` declares nothing without a default; otherwise it needs one. Derived, so there is no second flag to disagree with |
| **what an upload is checked against** | a declared variable still unset after the last layer is **refused** — the instance is incomplete, and starting it would fail later and worse. A variable nobody declared is **accepted and said out loud**: services grow faster than their `env.dist`, but a typo'd secret name is otherwise silent |

### What the child is told

**A script under the runner holds no credential at all.** The runner owns the
name, does the bus talking, and hands the script a message on stdin and takes
the answer back ([script services](#script-services)) — which is what "the
script need not know anything" means in practice. A service that links a
client library is the other case: it talks to the daemon itself, so it needs a
token, and that token is a variable in its `env` like any other.

| | talks to the bus | holds a token |
|---|---|---|
| a script under the runner | the runner, on its behalf | **no** |
| a linked service (php, go, …) | itself, through the client library | yes — a variable in its `env` |

The runner comes by the credential for a name it serves the same way anything
else does — by asking for one for a name it owns, which is already the rule and
already stops it collecting anybody else's
([access § getting a token](02-access.md#getting-a-token)). It is never given
the power to mint one.

What a child *is* told is **what it is serving** rather than who it is:
`topic` and `tag` reach it as environment. The set is deliberately not closed —
it will grow when services are actually being written, and this is where it is
recorded when it does.

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

❓ **Who vouches for `runner@<edge>` when that host runs no daemon.** A
`user@host` realm is vouched for by that host's `agent-busd`
([identity § names](01-identity.md#names)), and an edge box has none — so the
name it registers under is the one case the realm rule does not already
answer. *Settled by:* owner, with the runner.

**A bus that is away is not a service that failed.** When the daemon is
unreachable the services are running perfectly well and simply cannot take
work — so the *client* waits and reconnects, and the runner restarts nothing.
Getting that backwards turns one restart of the bus into a restart storm on
every host at once.

| what happened | who deals with it |
|---|---|
| the child died | the runner restarts it |
| the bus is away | the client reconnects, with backoff |

### Reaching the runner

**The runner is a service on the bus**, registered as `runner@<host>` — a name
like any other ([identity § names](01-identity.md#names)), so a runner on an
edge box stays addressable from the daemon it reports to —
and installing, configuring, enabling and starting are calls to it like any
other. There is **no second ssh door and no account to be let into**: who may
deploy on a host is the ACL on that one service
([identity § acl](01-identity.md#acl)) — the mechanism the bus already has
rather than a new one beside it.

What the grant is bounded by has not changed:

> deploying on a host lets you run code there. It does not let you impersonate
> a name on the bus.

An instance still has to *become* a name, and it can only become one whose
credential it was handed. Which is why the runner is **never given the power
to mint one** — if it could, deploy access and impersonation would be the same
thing, and the whole split would be decorative.

**Configuration is write-only.** A config goes in and is never handed back:
anyone who could print one could read every secret on the host, and the ACL
that let them deploy would buy nothing. Whoever truly needs the bytes can
become root and read the file — that is the boundary, stated rather than
worked around with a verb.

The consequence worth planning for: nobody can ask the host what is deployed.
`runner/` is a **write-only deployment target** and the source of truth lives
where it was installed from. `service.d` is the opposite — world-readable,
holding no secret, and usually a checkout whose source of truth is the
repository it came from.

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

**Off is a setting, not an absence.** It is also the default, so the common
case says `off` and means it. Asking for confinement where the host cannot
provide it is an **error**, never a quiet downgrade — a sandbox that silently
did nothing is worse than none.

Default profile — **nothing writable but the work directory, and no network**:

| Property | |
|---|---|
| `ProtectSystem=strict` | the whole filesystem read-only |
| `ProtectHome` | `read-only` for a service published by hand, whose script usually *is* in a home. **`yes` for anything the runner starts** — nothing it needs lives in a home, and its own secrets never reach it as a file at all ([the three env layers](#the-three-env-layers)) |
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

- Child private config is the runner's own — the env layers
  ([the three env layers](#the-three-env-layers)), never a blob held or sealed
  by `agent-busd`. The daemon holds credentials, the runner holds
  configurations, and neither reads the other's
  ([setup § the two accounts](09-setup.md#the-two-accounts)); a config the
  daemon stored would be exactly the case that argument rules out. Sealed
  private config ([identity § sealed private config](01-identity.md#sealed-private-config))
  is a *service's* own secret, sealed to its key, which the runner cannot
  open either.
- Per-child identities make each child individually revocable, and the runner
  registers them the way any owner does — it may not mint their credentials
  ([what the child is told](#what-the-child-is-told)).
- Health hints for children are generated by the runner — it knows how it
  launched them.
- Zero-downtime reload (socket inheritance, `cloudflare/tableflip`-style) for
  `agent-busd` itself; 2× RAM during overlap.
