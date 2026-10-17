# Managed runner: implementation

This is implementation design, including pending and historical mechanisms.
For current usage, read the [runner guide](../../docs/runner/README.md).

The principles, use cases and open decisions are the [managed runner](runner.md#managed-runner);
this page holds the detail behind them. Where the two disagree, the main page wins and this one is to be brought in line.

## What the runner does

- **Supervise** — spawn, restart with backoff, stop, log capture, exit codes.
  Five **control** verbs and no more: **start, stop, restart, reload,
  enable/disable** (`logs` reads, it does not control) — and it is
  `start`, never `run`, the same word whether you sit in front of it or the
  runner does it for you. **`reload` exists for a long-lived child and nowhere
  else** ([long-lived agents](#long-lived-agents)): there is something to
  signal only when a process is kept, so on a one-process-per-message agent
  it is refused rather than quietly doing nothing. Everywhere else a graceful
  restart already loses nothing: `stop` waits for the work in flight, messages
  queue in the daemon meanwhile, and the next process picks them up. This is
  the one place the runner **cares what kind of child it has**, and it is the
  price of keeping a process alive.
- **Represent** — registers each child as an **agent**
  ([agents § agent templates](../../docs/03-records-agent.md#agent-templates)),
  heartbeats and reports stats for it, holds and injects the child's identity
  and private config. Children need not know the bus exists — a script under
  `agent-bus start` never handles a credential at all, and an agent that links
  a [client library](../../src/clients/README.md) lets the library do it.
- **Is itself an agent** — self-registers, self-reports, has its own key, and
  is controllable over the bus — the same verbs, under its owner's ACL.

### One name on many hosts

**`--share` makes an agent one of a pool, and a pool is a name rather than a
place.** Four image scalers on four servers all read `#scaler@pool1`'s inbox and
take turns; nothing says where a member runs, and nothing needs to — which is
what the name has to stop saying ([the name a member registers](#the-name-a-member-registers)). The bus has
allowed this from the start — a reader that asks to share is one of several
([messaging § several readers may wait when they say so](../../docs/04-messaging.md#several-readers-may-wait-when-they-say-so))
— so `start --share` would pass that word through and the daemon learn nothing
new. The foreground `start` refuses a second start of one name today; `--share` on it is not built. **One word, two layers, because it is one decision.**
[R1](README.md#scope).

`-N` and `--share` compose and do not overlap: `-N` is how many hands one
runner has, `--share` is how many runners one name spans. Four hosts at `-N 4`
is sixteen scalers on one queue.

What starting a name twice on one host now means:

| | |
|---|---|
| starting the name twice on one host | allowed with `--share` (not built), refused without it — in both directions, the way the daemon already refuses a pool member beside an exclusive reader |
| the note in the owner's state directory | one per **process**, not one per name. `stop <name>` stops this host's members, all of them, and waits for each; `logs <name>` merges what they wrote ([stopping it and reading what it said](../../docs/06-runner-role.md#stopping-it-and-reading-what-it-said)) |
| registering the name N times | an update, not a collision: one record, one owner, and every member says the same thing about it ([identity § ownership](../../docs/01-identity-and-authority.md#ownership)) |
| what the registry shows | the name is up while **any** member is. A pool that is half down is a health matter, not a registry one |
| a reply | goes to whoever sent the message, never to the member that answered, so which one took the work is nobody's business ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)) |
| what it is still not | a group, a lease or a load balancer. Nothing is remembered between reads, so a member that dies leaves nothing to clean up |

#### The name a member registers

Four runners left to themselves would register `#image-scaler@srv1`,
`#image-scaler@srv2` and so on — four names, four inboxes, and no pool at all.
Three rules stop that, and only the last one is new:

| | |
|---|---|
| **a pool is one bus** | members that report to different daemons are not a pool, they are two queues with the same idea in them. So the members point their runners at the one daemon, which is the option the runner already has ([setup § the two units](../../docs/07-setup.md#the-two-units)) and the edge-box arrangement ([where it runs](#where-it-runs)) |
| **the realm is the daemon's, never the runner's host** | a runner on `srv2` reporting to the bus on `srv1` is registering into `srv1`'s realm already. Nothing about where a process sits belongs in the name it serves |
| **so the pool is given a complete name** | `#image-scaler@pool1`, a realm that daemon is told to hold. A complete name is taken whole; only a **bare** one is completed with the local host, and that completion is a convenience carrying no authority ([identity § names](../../docs/01-identity-and-authority.md#names)). `@srv1` would claim a location false for three members out of four; `@pool1` claims membership, which is true for all of them and survives a member moving |

So a pool needs no naming machinery of its own — its members are simply given
the whole name, as a run option beside `-N` and `--share`. **Getting it wrong is
silent**: a member that fell back to the default is a healthy agent nobody
ever calls.

Which is also why a member **states its hostname at registration**, in a field
of its own: the name no longer carries one, and a listing still has to answer
*where* ([where a member says it is](#where-a-member-says-it-is)). It ships
in R1 with pools.

#### Where a member says it is

Moved from R1.1 with pools (owner decision, Q139).

**An agent states its hostname at registration, as a field of its own.** It
became necessary the moment a name stopped carrying one: a pool's realm claims
membership rather than a location ([identity § names](../../docs/01-identity-and-authority.md#names)),
which is right, and leaves *which box do I go and look at* unanswered.

| | |
|---|---|
| **stated, never observed** | the daemon cannot see it. A unix socket has no peer host at all, and a TCP peer address is the network's opinion rather than the machine's name. So the machine says |
| **a label, and never an input** | nothing routes, authorises or decides on it, because nothing checked it. It is for the person reading a listing at three in the morning |
| **one entry per member, not one per name** | a pool registers one name from several places, so what is kept is a set — refreshed by the runner's own heartbeat for each member ([what the runner does](#what-the-runner-does)), and gone when that member is; R1.1's [health checker](../R1.1/discovery.md#health-checker) generalizes it later |
| **not part of the identity** | the name is the identity ([identity § names](../../docs/01-identity-and-authority.md#names)); this is a fact about a process serving it. Keeping the two apart is the whole reason the realm stopped carrying a hostname |

It is also where a **version** would go when an agent can say one
([runner § agent version reporting](#agent-version-reporting)) — same shape, same reason: per member,
stated, and the answer to *which of these four is the odd one out*.

**That the members are interchangeable is the operator's promise**, and the bus
cannot check it any more than it can check that an agent does what its
description says. Two hosts serving one name with different code, or different
`--algo`, is a caller getting different answers to the same question — and it
will look like a flaky agent, not a misconfigured one.

Under the runner, sharing is a **run option like the worker count**, so it
lives in `agents.json` as `share`, beside the sandbox, rather than in
`agent.json` ([agent configuration](runner-config.md#the-hosts-options)). Same reasoning as
the foreground `-N`'s default of 1: whether it is safe to run this thing twice is a decision
the host is already making.

### Many names into one inbox

**Each agent is read on its own inbox, as itself** (runner M.10, 2026-10-17).
The runner holds every served agent's token
([who it runs as](#who-it-runs-as)), so it reads each agent's inbox and secret
with that agent's credential, one read per free process, and answers as it.
Nothing is forwarded and the runner adds no routing: a busy agent never holds
an idle one's message, a message for an agent that is not served waits in that
agent's own inbox, and a reply comes from the name that was addressed.

| | |
|---|---|
| **It serves only what it holds a token for** | a name it created, or one its Owner handed over; never one it could only fetch, since it cannot fetch any |
| **Stopping an agent loses nothing** | its messages stay queued in its inbox, under its own bound, TTL and overflow, until it is served again |
| **The runner's own inbox is its own** | `status` and `reload` arrive there and nothing else does |

<details>
<summary>History: one inbox through <code>deliver_to</code>, superseded by M.10</summary>

Built in M.2 (0.8.123) and replaced on 2026-10-17. Every served agent had a
one-slot `deliver_to` naming the runner, the runner read its single inbox with
`GET /consume?original_to=<name>` (Q162), served only names its own User owned
(Q163), and answered a message for an agent it no longer served as refused,
since holding it would have blocked every other agent. The daemon's
`original_to` filter remains built; the runner no longer uses it.


**The runner uses the daemon's built forwarding, `deliver_to`, and adds no
routing of its own.** Each agent it manages gets a one-slot `deliver_to`
naming the runner's own inbox, so the runner reads one inbox and dispatches on
`original_to`, the name the sender addressed. Forwarding is the daemon's,
built in 0.7.12 ([messaging § Deliver-To](../../docs/04-messaging.md#deliver-to)):

| | Built behaviour |
|---|---|
| The message moves | it keeps its sender and gains `original_to` and a forward count; the forwarding record keeps no copy |
| The ACL | the destination's allow list must name the forwarding record itself, checked when the slot is stored and again at delivery |
| Chains | a forward may forward again, up to ten hops; a cycle ends with an error |
| The queue that holds it | the destination's bound, TTL and overflow apply |
| Publications | a 📣 copy to a managed agent follows that agent's `deliver_to` one step on, so agents on a Deliver-To list need no inbox of their own either |
| Roles | worked out once, at the addressed record, and carried through unchanged; they cross to the runner only if the agent's Owner owns or maintains the runner, otherwise the forward fails loudly ([roles](roles.md#roles-are-for-agents)) |

It is the **dual of a pool**, and the two are worth reading together:

| | Names | Inboxes | Readers |
|---|---|---|---|
| Pool (`--share`) | one | one | many, taking turns ([one name on many hosts](#one-name-on-many-hosts)) |
| Forward to the runner | many | one, the runner's | one |

A pool hides *where* a worker runs. A forward hides *how many* inboxes a
reader has to watch. Neither hides who the message was for: a pool member
reads its own name, and a forwarded message says its own in `original_to`.

What is the runner's own, beyond what the daemon does (owner-settled
2026-09-16, restated for `deliver_to`):

| | |
|---|---|
| **It serves only names it owns** | owner decision, 2026-10-16 (Q163). `#runner` allows `@owner`, the records its own User owns, and `@administrators`; the daemon checks a forward against that list when it is stored and at delivery, so a record somebody else owns cannot forward to the runner. That is what makes the next three rows hold: every `original_to` the runner receives is a name whose credential it may fetch. Dispatch is keyed on `original_to`, not on what the runner starts |
| **An `original_to` it cannot serve is refused to its sender** | no child for that name, or the child is gone: the runner replies with an error and the message is consumed. The runner's inbox is shared by every name forwarded to it, so holding the message would put one misconfigured forward in front of every healthy agent. The cost, stated plainly: a child restarting when a message lands turns a short wait into an error the sender retries through |
| **It reads one agent at a time** | owner decision, 2026-10-16 (Q162); approved as `GET /consume?original_to=<name>`, matching the address a message was sent to: its `original_to` when forwarded, its `to` when not, so the runner's own calls are read as `original_to=#runner@<host>`. The daemon filters a read of the runner's inbox by `original_to`, so the runner takes a message for an agent only when that agent has a free process: a busy agent never holds an idle one's message up, nor the runner's own calls. A read with no filter still takes the next message of any kind |
| **A reply comes from the name that was addressed** | a sender that wrote to `#scaler@h` is answered by `#scaler@h`, so nobody has to learn the deployment to correlate an answer. This grants the runner nothing new: it holds each child's credential to start it ([what the child is told](#what-the-child-is-told)) and may not mint one, so it cannot answer for a name it was never handed |
| **What comes back names the addressed agent, on every path** | served, refused, expired or failed, the sender is told about `#scaler@h` and nothing else. The forward is a field of the record, readable by whoever may read the record ([ACL](../../docs/02-access.md#acl)); the promise is to the sender, and shared fate with the runner's queue is a trace, not a statement |
| **A handover is complete when the old inbox is drained** | repointing `deliver_to` from `#runner-a` to `#runner-b` applies to delivery from then on; messages already in `#runner-a`'s inbox stay there and are served by it, for as long as it can. Tooling that reports a handover done at the write reports it too early |
| **Repointing is not a transfer** | it changes where messages land and needs only the standing to manage the record; who owns the record, and so who may ask for its token, changes only by the Owner's transfer ([ownership](../../docs/01-identity-and-authority.md#ownership)) |

<details>
<summary>History: the 2026-09-16 `route` design, superseded</summary>

Before forwarding was built, this page proposed a separate `route` field
carrying `orig-to`, checked only when written, one hop and never for
publications. The built `deliver_to` replaced it: `original_to` instead of
`orig-to`, the destination's ACL checked at delivery too, chains up to ten
hops, and publications forwarded. The full text is in Git history.

</details>


</details>

### Long-lived agents

**A child in one of the stream forms is started once and kept** — `jsonl` or
`msgpack` — and messages arrive on its stdin, one frame at a time, for as long
as it lives. Everything expensive to
build — a loaded model, an open database handle, a warm cache — survives
between messages, which is the only thing it buys.

The shape forces the lifetime rather than a flag declaring it: `args` cannot be
long-lived, because argv is fixed at exec; a process that reads a *stream* of
messages is by construction one that stays; and a length prefix is only worth
writing when another frame follows it. So there is no second setting to
disagree with the first — the same reason `env.dist` decides whether an agent
needs an instance ([the three env layers](#the-three-env-layers)).

| | |
|---|---|
| **several messages in flight, if asked** | settled (Q13, 2026-09-30), not built. By default the runner writes one frame and waits for its answering frame, so nothing needs correlating and `processes` keeps its meaning: N children, N hands on one inbox. `--in-flight K` lets one child hold up to K frames at once: each frame carries its message's topic and tag, the child echoes them on its answer, and answers may come back in any order ([messaging § request and reply](../../docs/04-messaging.md#request-and-reply)). Hands on the inbox are then N × K |
| **a deadline per message, and only here** | killing a one-per-message script costs the process and nothing else. Killing a kept child throws away everything it warmed up, and there is no way past a wedged one without doing it — so the runner waits a bounded time, then kills, restarts with backoff, and logs the message that was in flight as lost |
| **state is the agent's own business** | the runner promises nothing about which child handles which message, so whatever a child remembers must not belong to one caller. The first bug here will be a per-caller cache that outlives the caller |
| **up is ready** | no readiness handshake. A child that exits before its first reply is a failed start and backs off like any other |
| **`reload` is `SIGHUP`** | and an unhandled one is a restart ([stop and reload](runner.md#stop-and-reload)) |
| **stopping is unchanged** | no further frame is written and every answer in flight is waited for, then `SIGTERM` — bounded by the same deadline, because a wedged child cannot be waited out |
| **a crash still loses only what was taken** | a message leaves the daemon only when a hand is free for it, so the rest is still queued for whatever reads that inbox next; a child with K in flight loses those K |

### On demand

> **Superseded in part by [forwarding to the runner](runner.md#how-it-works).**
> Every runner agent forwards to the runner, so the runner already holds each
> message and starts a process for it itself: the daemon-side fallback channel
> and wrapped call below are no longer needed. The rest of this section, why
> most agents run on demand and what a cold start costs, still holds.

**This is how most runner agents are meant to run, not a special case for
rare ones.** Keeping a process resident is what
[long-lived agents](#long-lived-agents) buys something by — a loaded model,
an open handle, a warm cache. An agent with nothing to keep warm gains nothing
from being up, and **most agents have nothing to keep warm**: the resident
ones are the exception, and on demand is the ordinary arrangement for the rest.

Sending an SMS is only the clearest case — rare, cheap to start and pointless
to keep resident. Today such a name is registered and its inbox fills while
nothing reads it ([messaging § inbox queues](../../docs/04-messaging.md#inbox-queues)); the work
waits for a process somebody has to have started, and a box ends up running a
process per agent for agents that are idle almost always.

**The mechanism is one field on a record and one branch in delivery.**

| | |
|---|---|
| **the record carries a fallback channel** | an agent may register one, and the runner puts it on the registration of every on-demand agent it owns. Declared once, by the thing that can actually act on it |
| **no consumer is the trigger** | a call for a name nobody is reading is **wrapped** by the daemon and delivered to that channel instead of waiting in the inbox |
| **the runner is what reads it** | it takes the wrapper, starts the script for that one task, and hands the result back |
| **nothing is resident in between** | which is the whole point |

**The caller cannot tell.** It made an ordinary call
([messaging § request and reply](../../docs/04-messaging.md#request-and-reply)) and gets an ordinary
reply; only the latency differs. No new verb, no new transport and no second
way to address an agent — the wrapper is an ordinary message on an ordinary
channel, and the execution is the one the runner already performs per message
in the `args` and `json` forms ([what an instance is](#what-an-instance-is)).

| | |
|---|---|
| **the wrapper carries the original, and the answer comes back as the agent** | reply routing is the original envelope's ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)). The runner returns a result and the daemon unwraps it; a caller that sees the runner's name where the agent's belongs is the wrap having leaked |
| **the ACL is applied before the wrap, never after** | the daemon refuses on the record's own ACL exactly as it always does, and only a call that passed is wrapped. Otherwise the fallback channel is a way to reach an agent you may not call |
| **a wrapped call is accepted from the daemon and from nobody else** | the channel is a name with an ACL like any other ([identity § acl](../../docs/02-access.md#acl)) — the daemon writes, one runner reads. The runner checks the sender as well, because a wrapper is the one message that asserts *somebody else* called: accepted from the daemon it is that assertion, accepted from anyone it is a way to run an agent as a caller you are not |
| **the runner still mints nothing** | it executes and answers. Who called is the daemon's assertion inside the wrapper, never something the runner establishes — the same rule as everywhere else ([what the child is told](#what-the-child-is-told)) |
| **no consumer is not a race** | routing is one decision by the one thing that knows both facts: routed to the channel when there is no consumer, delivered to the inbox when there is. There is no window between a check and a hand-off because there is no check and no hand-off — and either answer stays right afterwards, since a consumer that attaches later reads the next message, and one that leaves later leaves a queued message, which is the ordinary case already ([messaging § inbox queues](../../docs/04-messaging.md#inbox-queues)) |
| **cold start is the caller's latency, on the caller's ttl** | the call pays a process start. A message ttl shorter than that start is a caller that has already given up ([messaging § message ttl](../../docs/04-messaging.md#message-ttl)), and being on-demand does not extend it |
| **a runner that is down is an agent that is down** | the wrappers queue on the fallback channel, which is an inbox like any other. Nothing new to reason about |
| **it does not change what a script is** | the same forms, the same env layers ([the three env layers](#the-three-env-layers)), the same sandbox ([sandboxing](#sandboxing)). Only the trigger differs |
| **the kept child is what needs a reason** | the shape already decides the lifetime ([long-lived agents](#long-lived-agents)) and this does not add a flag beside it. What changes is the default expectation: a stream form stays because it has something to keep, and everything else is on demand unless somebody says why not |

**Start on demand is the other kind, settled 2026-10-16** as `processes`
`0..N` on a stream form ([kinds of agents](runner.md#kinds-of-agents)). There the call
starts the **agent**, which then reads its own inbox the ordinary way and may
be stopped again after an idle period.

| | |
|---|---|
| **nothing is wrapped** | the message stays in the inbox it already reached, and the daemon only has to say *somebody should start this*. Less machinery than the wrapped form, not more |
| **the start is paid once, not per call** | a burst warms one process and the rest are ordinary deliveries. Per-call exec is right for an SMS and wrong for anything with a cache to build, so the two answer different questions: **how rare**, and **how expensive to start** |
| **idle shutdown is a second decision** | what counts as idle, and what stopping does to work in flight — a stop already waits for what it took ([what the runner does](#what-the-runner-does)) |

The pair is inetd's per-call exec beside systemd's socket activation, and that
system shipped both for this reason. The idle time is five minutes by default
([stop and reload](runner.md#stop-and-reload)).

## What an instance is

A directory — and the directory being there is the **installed** state, not
the running one. It says the instance exists and is configured, and nothing
more.

Three states, each with exactly one home, the way systemd splits them:

| State | Where it lives | Verb |
|---|---|---|
| **installed** | its directory, and a row in `agents.json` ([the list of what is installed](#the-list-of-what-is-installed)) | the upload that creates it |
| **enabled** | that row's `enabled` | `enable` / `disable` |
| **running** | neither — it is a process | `start` / `stop` |

A list exists because the runner needs one: **at start it has to know what to
bring up and how many of each**, without walking a tree. Installed and enabled
are separate so that an instance can be configured and kept without running —
put in place before it is turned on, started and stopped by hand, and left
alone by a reboot. That is what `disable` leaves behind.

The registry still answers what exists and what is alive; the runner answers
only what should be up.

Two directories hold it between them, and that split is the point — modes and
owners are in [setup § the two accounts](../../docs/07-setup.md#the-two-accounts):

| | Holds |
|---|---|
| `agent.d/<svc>/` | what the agent **is**: the code, or a symlink to the same code elsewhere, plus `agent.json` and `env.dist` |
| `runner/<svc>/<instance>/` | what this instance is **configured with**: an `env` file, and nothing else |

The author and the host each own one file, and they do not overlap:

| | Owned by | Says |
|---|---|---|
| `agent.json` | the **author**, and it travels with the code | how the agent starts, what to register — including its **version**, which is why an agent too simple to answer a `version` call still has one — and what it **depends** on. The command line lives here and nowhere else |
| `agents.json` | the **host** | every agent installed here, and per agent: whether to start it, how many, what sandbox it runs in ([sandboxing](#sandboxing)), and what it comes after ([the list of what is installed](#the-list-of-what-is-installed)) |

So a host never restates the command, and an update to it arrives with a `git
pull` rather than as an edit somebody has to remember to make twice.

`agent.d` is externally controlled — in most cases a `git clone`, and
nothing a host should be editing by hand. So **`git pull` an agent's code and no
local state moves**: every decision this host made — secrets, worker counts,
the sandbox, whether it runs at all — is under `runner/`.

The **directory name ties the two sides together**: `runner/mail-reader/`
configures `agent.d/mail-reader/`. No symlink and no pointer file — a name is
already unambiguous, and a link could not carry the run options anyway. If
`agent.json` named itself too they could disagree, so it does not.

**It is also the default name to register, and a default is all it is.** The
directory gives a bare name, which is completed with the local host the way any
bare name is ([identity § names](../../docs/01-identity-and-authority.md#names)) — and `agents.json`
may state a **complete** name instead, which is then taken whole. That is the
one thing a host must be able to override, because a pool's members sit in
identically named directories on four machines and must register **one** name
between them ([the name a member registers](#the-name-a-member-registers)).
It stays the host's file rather than the author's for the usual reason: which
pool this copy joins is not something the code knows.

One agent template with many instances is already in the naming —
`template/instance-name@realm` ([identity § names](../../docs/01-identity-and-authority.md#names)) — so
per-user instances need no new idea.

### The three env layers

An agent's **environment** arrives injected before exec — never as a path the
child could open. It is assembled from three files that overlay in one order.
This is the runner's half, and it is not the registry's configuration, which an
agent fetches for itself and which the daemon holds
([agents § configuring a template](../../docs/03-records-agent.md#configuring-a-template)):

| | Carries | Wins |
|---|---|---|
| `agent.d/<svc>/env.dist` | the declared surface: every variable the agent wants, with a default for each one that is not secret | lowest |
| `runner/<svc>/env` | what every instance of this agent shares — the one API key they all use | middle |
| `runner/<svc>/<inst>/env` | this instance's own | highest |

The record's bus secret, held by the daemon, sits between `env.dist` and
`runner/<svc>/env`: local wins over remote ([secrets](runner.md#secrets), Q153).

**The more secret it is, the more it wins.** Precedence and visibility run in
opposite directions — a property a reader can check against the modes
([setup § the two accounts](../../docs/07-setup.md#the-two-accounts)) rather than a rule
to remember. What keeps it true: **`env.dist` may carry a default only for
something that is not secret.** Anything secret is declared with no value —
and that is exactly what makes it required.

Two things fall out of declaring the surface at all:

| | |
|---|---|
| **whether an agent needs an instance** | it starts directly if `env.dist` declares nothing without a default; otherwise it needs one. Derived, so there is no second flag to disagree with |
| **what an upload is checked against** | a declared variable still unset after the last layer is **refused** — the instance is incomplete, and starting it would fail later and worse. A variable nobody declared is **accepted and said out loud**: agents grow faster than their `env.dist`, but a typo'd secret name is otherwise silent |

### What the child is told

**A script under the runner holds no credential at all.** The runner owns the
name, does the bus talking, and hands the script a message on stdin and takes
the answer back ([script agents](../../docs/06-runner-role.md#script-agents)) — which is what "the
script need not know anything" means.

| | talks to the bus | holds a token |
|---|---|---|
| a script under the runner | the runner, on its behalf | **no** |
| a linked agent, using a [client library](../../src/clients/README.md) (Python, TypeScript, Go, Rust, Java, PHP) | itself, through the library | yes — a variable in its `env` |

The runner comes by the credential for a name it serves the same way anything
else does — by asking for one for a name it owns, which is already the rule and
already stops it collecting anybody else's
([access § getting a token](../../docs/02-access.md#getting-a-token)). It is never given
the power to mint one.

What a child *is* told is **what it is serving** rather than who it is. Built
in the foreground runner: `AGENT_BUS_MESSAGE_ID`, `AGENT_BUS_FROM`,
`AGENT_BUS_TOPIC` and `AGENT_BUS_TAG` from the envelope, and `AB_ROLE_<NAME>=1`
for each role the sender holds ([agents § roles](../../docs/03-records-agent.md#roles)).
The set is deliberately not closed: it grows as agents are written, and this is
where it is recorded when it does.

### The list of what is installed

`agents.json`, in the runner's home, is **one row per installed agent** —
not per started one. It is the inventory and the host's decisions in the same
file, because they are the same list seen twice, and keeping them apart bought
a second file and nothing else.

| Field | Says | Written by |
|---|---|---|
| the name | which directory, and so which `agent.d/<svc>` ([what an instance is](#what-an-instance-is)) | the install |
| **`enabled`** | on or off: started by the runner, or installed and left alone; on demand is `processes: 0..N` | `enable` / `disable` |
| `share`, `limits`, `sandbox`, a complete `name` | what only the host decides; the author's run options are overridden in `runner/<agent>/agent.json` ([agent configuration](runner-config.md#the-hosts-options)) | the host |
| **`version`**, and the **origin** when it is a checkout | what was installed: the version `agent.json` claimed, and the remote and commit to fetch it again | the runner |
| **`first-started`**, **`last-started`** | when this instance first ran here, and when it last did | the runner |

**The runner keeps the derived half current** — version, origin and the two
dates — at start, or when told to. That is the file being a record of what is
here and not only of what was decided, and it is why a backup needs nothing
generated to go with it.

#### What it comes after

**`depends` is declared by the author and overridden by the host**, the same
two layers as the environment ([the three env layers](#the-three-env-layers)).
What an agent needs is a property of the code, so it arrives in `agent.json`
with a `git pull` and nobody restates it. What is true *here* is the host's, so
the host's override of `agent.json` may add an entry, and may **turn one off** — the case that
makes this necessary is a dependency that moved to another host, where there is
nothing local to come after.

| | Says |
|---|---|
| `agent.json` | what this agent needs, from whoever wrote it |
| `runner/<agent>/agent.json` | what this host adds, and which of the author's entries are not local any more: a list in an override replaces the author's whole ([overriding a shipped file](runner-config.md#overriding-a-shipped-file)) |

**It orders starts and does nothing else.** The runner brings an agent up
after the ones it still names, and a cycle is refused at load with the cycle
named. It does not wait for readiness, because up *is* ready
([long-lived agents](#long-lived-agents)), and it does not watch: whether
anything is actually serving a name is a question the bus already answers
([discovery § what a listing answers](../../docs/05-discovery.md#what-a-listing-answers)).

**An entry that names nothing installed here is a refusal**, not a line quietly
skipped — turning it off is how a host says *this one is remote now*, and that
is a sentence somebody wrote rather than a silence the runner read something
into. It is the same trap as a pool member's name ([the name a member
registers](#the-name-a-member-registers)): a typo that is ignored leaves an
agent that starts in the wrong order and looks healthy doing it.

### Backing it up

**The backup is `runner/`, encrypted, and that is the whole of it.** It is a
runner verb (owner decision, Q33): `agent-bus call '#runner@<host>' backup`
answers with the archive, under the runner's ACL, and any timer may call it.
`agent.d` is a checkout: whatever is in it can be fetched again from where it
came from, and `agents.json` says from where. `runner/` cannot be fetched
again — it is every decision this host made, plus the record of what is
installed, and nothing else holds a copy.

**Use the `age` command-line tool**, encrypting the archive to the user's
Ed25519 SSH public key (`ssh-ed25519`). Backup creation needs only that public
key. Restore uses the matching private SSH key on the user's restore host;
neither the daemon nor the runner stores the user's private key.
[age's SSH key support](https://github.com/FiloSottile/age#ssh-keys) provides this
without a custom encryption format or a new key type of ours.

The archive contains secrets, so encryption is required. The user's key keeps
backup recovery outside the daemon's authority and preserves the
[separate secret domains](../../docs/07-setup.md#the-two-accounts).

Restoring is the archive backwards: fetch each origin at its commit, unpack
`runner/` over it. An agent whose origin is gone is a **named failure** and
not a quieter restore — a host that comes back with four agents out of five
and says nothing is worse than one that will not come back.

## Who it runs as

- **Separate user** (default for shared/server use): the daemon as
  `agent-busd` and the runner as `agent-bus-runner`, two accounts that cannot
  read each other's home ([setup § the two accounts](../../docs/07-setup.md#the-two-accounts)).
  Privileged installer once; **no root at runtime**, and the only capability
  anywhere is the supervisor's
  ([processes § why the supervisor holds CAP_CHOWN](../../docs/08-processes.md#why-the-supervisor-holds-cap_chown)).
- **Current user** (personal use): children as you, no runner at all. Zero
  setup — the laptop story with the AUTH role off.

**On the bus the runner is its person's agent, `#runner/<person>@<realm>`,
with no identity of its own** (owner decisions 2026-10-17, M.10). It is
started with that agent's token and refuses to start with a User's, so it
never holds its person's own credential. Its record and ACL are its person's.

**Its person owns every agent it serves.** The runner creates each one with a
create-only `register`, which answers the new agent's token
([getting a token](../../docs/02-access.md#getting-a-token)), and keeps that
token in `runner/tokens/`, its home's alone. The person keeps the
description, ACL, configuration and secret as for any agent of theirs. A name
that already exists is refused by name until its Owner hands its token over
once into that directory; the runner never registers over it and cannot fetch
it. Built in 0.8.141–0.8.142; the Linux account `$user-ab-runner` and the
setup verbs are [M.11](TODO.md#managed-runner-waves). Until then the shared
`agent-bus-runner` account runs it, with its agent's token in `runner/runner.env`.

<details>
<summary>History: the runner as the User <code>agent-bus-runner</code>, superseded by M.10</summary>

Owner decision 2026-10-16: setup created a keyless User `agent-bus-runner` and
mapped its account's socket to it; it owned `#runner@<host>` and every agent
it registered, the deploying User was each agent's Maintainer, and an existing
name was served once transferred to `agent-bus-runner`. A serve-only grant
that kept the Owner was Q164. Replaced 2026-10-17: a separate bus User made
ownership harder than the person owning their own agents.

</details>

### Where it runs

The runner and the daemon are separate programs, so they need not share a
host. Three arrangements, and the third is what the split buys:

| | daemon | runner |
|---|---|---|
| laptop | yours, local | none — publish by hand ([script agents](../../docs/06-runner-role.md#script-agents)) |
| one host | local, `agent-busd` | local, `agent-bus-runner` |
| **edge box** | **elsewhere** | local, alone — a machine that hosts agents and holds no bus state |

**An edge runner takes the bus's realm** (owner decision, Q15). It has no
daemon to vouch for an `@<edge>` realm, so it registers as an instance under
the daemon it reports to, `#runner/<edge>@<realm>`, the way a
[pool member](#the-name-a-member-registers) takes the pool's name. Where it
runs is the hostname it states, not its name.

A remote daemon is reached over **ssh**
([access § the three doors](../../docs/02-access.md#what-a-call-carries)), or
directly on its TCP address, plain HTTP or
[TLS](../../docs/02-access-remote.md#over-https).

**A bus that is away is not an agent that failed.** When the daemon is
unreachable the agents are running perfectly well and simply cannot take
work — so the *client* waits and reconnects, and the runner restarts nothing.
Getting that backwards turns one restart of the bus into a restart storm on
every host at once.

| what happened | who deals with it |
|---|---|
| the child died | the runner restarts it |
| the bus is away | the client reconnects, with backoff, as the foreground runner already does ([script agents](../../docs/06-runner-role.md#script-agents)) |

### Reaching the runner

**The runner is an agent on the bus**, its person's `#runner/<person>@<realm>` — a name
like any other ([identity § names](../../docs/01-identity-and-authority.md#names)), so a runner on an
edge box stays addressable from the daemon it reports to —
and installing, configuring, enabling and starting are calls to it like any
other. There is **no second ssh door and no account to be let into**: who may
deploy on a host is the ACL on that one agent
([identity § acl](../../docs/02-access.md#acl)) — the mechanism the bus already has
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
where it was installed from. `agent.d` is the opposite — world-readable,
holding no secret, and usually a checkout whose source of truth is the
repository it came from.

## Runner unit

`/etc/systemd/system/agent-bus-runner.service` is the other half, and what it
does *not* say is most of it:

| The runner's unit says | So that |
|---|---|
| `User=agent-bus-runner` | it is the other secret domain, and cannot read the daemon's home |
| `EnvironmentFile=` and `ConditionPathExists=` on `runner/runner.env` | it starts as its person's agent, whose token that file holds, and stays inactive on a host where no runner is installed (M.10) |
| **which bus to use, defaulting to the local one** | the runner is a client, so the bus it serves is a setting rather than an assumption. A host with no daemon points it elsewhere and nothing else changes ([runner § where it runs](#where-it-runs)) |
| `WorkingDirectory` is its own home, and that alone is writable | `agent.d` is a checkout it only reads, and the daemon's home is not on any path it has |
| `Restart=on-failure` | same reason, and it restarts *its own* children itself rather than leaving them to systemd |
| **`Delegate=yes`** | the runner owns a cgroup subtree and makes one cgroup per agent in it, which is how every agent is [sandboxed](#sandboxing) without a user manager |
| **no `AmbientCapabilities`** | the one capability on this host belongs to the supervisor, and the runner is not it |
| **against a local bus**: `Wants=agent-busd.service` and `After=` it | the two come up together and in the right order, which is what a local runner depends on. Not `Requires=`: that would stop the runner — and so every agent it holds — whenever the bus is stopped, and a bus that is away is not an agent that failed ([runner § where it runs](#where-it-runs)) |
| **against a remote bus**: neither | there is nothing on this host to order against, and the unit is the same file otherwise |

**The runner is a bus citizen like anyone else**, which has a consequence
worth stating: reaching the local bus over the socket means it is a *mapped
local account* like every other ([local users](../../docs/07-setup.md#local-users)), so the install
maps it and the daemon opens it a socket. Nothing about the runner is special
to the daemon — which is the whole claim of the split, made concrete.

The runner's unit ships **with the runner**, in R1
([R1 scope](README.md#scope)) — there is no point writing a
unit for a program that is not installed. What exists today is everything it
will need: both accounts, the tree they own, and the runner's socket.

## Fits the other pieces

- Child private config is the runner's own — the env layers
  ([the three env layers](#the-three-env-layers)), never a blob held or sealed
  by `agent-busd`. The daemon holds credentials, the runner holds
  configurations, and neither reads the other's
  ([setup § the two accounts](../../docs/07-setup.md#the-two-accounts)); a config the
  daemon stored would be exactly the case that argument rules out. Sealed
  private config ([identity § sealed private config](../R1.1/identity.md#sealed-private-config))
  is an *agent's* own secret, sealed to its key, which the runner cannot
  open either.
- Per-child identities make each child individually revocable, and the runner
  registers them the way any owner does — it may not mint their credentials
  ([what the child is told](#what-the-child-is-told)).
- Health hints for children are generated by the runner — it knows how it
  launched them.
- Daemon reload is a separate [operations contract](../R1.1/operations.md#reload).

## Additional script forms

The foreground forms remain defined in [runner § script agents](../../docs/06-runner-role.md#script-agents).

| Form | Proposed behavior |
|---|---|
| `std` | Raw body bytes on stdin and stdout, one child per message |
| `jsonl` | An envelope line and an answer line per message, with the child kept alive |
| `msgpack` | Envelope and binary body in a frame, with the child kept alive; `uint32` length prefix in network order |

These are existing design choices, not a newly approved wire specification. Implementation remains blocked where [questions](QUESTIONS.md#open-questions) leave a contract unresolved.

## Sandboxing

The managed runner extends the [current sandbox port](../../docs/06-runner-role.md#sandboxing)
from one process to one agent ([sandboxing](runner.md#sandboxing)). The built
backend sets `NoNewPrivileges`, `ProtectSystem=strict`, a read-only home, a
private `/tmp`, `PrivateNetwork` unless asked, `TasksMax=64` and
`MemoryMax=512M` per child ([sandbox source](../../src/internal/sandbox/sandbox.go)).
Managed children have no home at all, because their code and injected
environment need none.

**The cgroups come from a delegated subtree** (owner decision, Q157), and
**bubblewrap applies everything a cgroup does not** (owner decision, Q158).
The runner's system unit has `Delegate=yes`; the runner makes one child cgroup
per agent or instance and starts each process inside it, under `bwrap`. No
user manager, so no `loginctl enable-linger`, and the runner needs no
capability for either.

| | Today, the foreground runner | The managed runner |
|---|---|---|
| unit of sandboxing | one transient user service per child (`systemd-run --user`, no `--scope`) | one cgroup per agent or instance in the runner's delegated subtree, every process of it inside |
| limits | fixed constants | the host's limits per agent, from `agents.json`, written to the cgroup's own files |
| resources | tasks, memory | also CPU, open files, I/O weight |
| filesystem | systemd unit properties | bubblewrap: the system read-only, an empty home, the checkout read-only, the work directory writable, override files mounted read-only over the shipped paths |
| privilege | `NoNewPrivileges` | also no capabilities, and a system-call filter like systemd's `@system-service`, compiled by the runner and loaded by bubblewrap (`--seccomp FD`) |
| network | `PrivateNetwork` unless asked | a network namespace of its own, loopback only, unless asked |
| network to named hosts only | not offered | a filtering proxy of the runner's, reached through a socket mounted into the sandbox and named in `HTTPS_PROXY`, passing only the hosts in `needs`; it covers what goes through a proxy, so an agent needing raw UDP needs the full network |
| disk space | not bounded | `/tmp` a tmpfs sized by bubblewrap (`--size`), whose pages also count against the agent's memory; the work directory a soft limit: measured, and the agent stopped and reported by name when over, since a real quota needs root |
| siblings | one account; another child can be signalled, traced, its `/proc` read | a PID namespace and a fresh `/proc` per agent, so it sees no other agent's processes, `ptrace_scope` notwithstanding; not an account per agent, which needs root or a setuid helper on every install |
| joining the cgroup | the transient unit's own | each process started directly inside its agent's cgroup (`CLONE_INTO_CGROUP`), never running outside it |
| built, 0.8.126 | — | bubblewrap with `--unshare-all`: `/usr` and the agent's directory read-only, its work directory writable, a private `/tmp`, no `/home` or `/etc`, its own PID namespace, loopback only unless `needs.network` (then the network and the `/etc` it needs to use it), a clean environment; not yet the cgroups, limits, filter or proxy |
| a host without the tool | — | no `bwrap`, user namespaces disabled, or a kernel before 5.7 (no `CLONE_INTO_CGROUP`) refuses to start the agent: it fails closed |
| default | off | on (owner decision, Q157) |

Container choices are in [R1.2 questions](../R1.2/QUESTIONS.md#open-questions).

## Addressing several names

Scatter-gather addresses several configured agents together. It is separate
from a pool, whose workers share one name and inbox. Selection and aggregation
remain release design work after scope confirmation.

## Agent version reporting

A registry record reports the version registered; a standard `version` call
reports what is running. The runner answers from the agent's description for
simple scripts. This depends on [method metadata](method-metadata.md#method-metadata); it does
not create an independently versioned program in this repository.
