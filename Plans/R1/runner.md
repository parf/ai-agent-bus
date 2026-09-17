# Managed runner

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

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
  ([services § service and template](../../docs/03-services-and-topics.md#service-and-template)),
  heartbeats and reports stats for it, holds and injects the child's identity
  and private config. Children need not know the bus exists — a script under
  `agent-bus start` never handles a credential at all, and a service that links
  a client library lets the library do it.
- **Is itself an agent** — self-registers, self-reports, has its own key, and
  is controllable over the bus — the same verbs, under its owner's ACL.

### One name on many hosts

**`--share` makes a service one of a pool, and a pool is a name rather than a
place.** Four image scalers on four servers all read `scaler@pool1`'s inbox and
take turns; nothing says where a member runs, and nothing needs to — which is
what the name has to stop saying ([the name a member registers](#the-name-a-member-registers)). The bus has
allowed this from the start — a reader that asks to share is one of several
([messaging § several readers may wait when they say so](../../docs/04-messaging.md#several-readers-may-wait-when-they-say-so))
— so `start --share` passes that word through and the daemon learns nothing
new. **One word, two layers, because it is one decision.**
[R1](README.md#scope).

`-N` and `--share` compose and do not overlap: `-N` is how many hands one
process has, `--share` is how many processes there are. Four hosts at `-N 4`
is sixteen scalers on one queue.

What starting a name twice on one host now means:

| | |
|---|---|
| starting the name twice on one host | allowed with `--share`, refused without it — in both directions, the way the daemon already refuses a pool member beside an exclusive reader |
| the note in the owner's state directory | one per **process**, not one per name. `stop <name>` stops this host's members, all of them, and waits for each; `logs <name>` merges what they wrote ([stopping it and reading what it said](../../docs/08-runner-role.md#stopping-it-and-reading-what-it-said)) |
| registering the name N times | an update, not a collision: one record, one owner, and every member says the same thing about it ([identity § ownership](../../docs/01-identity-and-roles.md#ownership)) |
| what the registry shows | the name is up while **any** member is. A pool that is half down is a health matter, not a registry one |
| a reply | goes to whoever sent the message, never to the member that answered, so which one took the work is nobody's business ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)) |
| what it is still not | a group, a lease or a load balancer. Nothing is remembered between reads, so a member that dies leaves nothing to clean up |

#### The name a member registers

Four runners left to themselves would register `image-scaler@srv1`,
`image-scaler@srv2` and so on — four names, four inboxes, and no pool at all.
Three rules stop that, and only the last one is new:

| | |
|---|---|
| **a pool is one bus** | members that report to different daemons are not a pool, they are two queues with the same idea in them. So the members point their runners at the one daemon, which is the option the runner already has ([setup § the two units](../../docs/09-setup.md#the-two-units)) and the edge-box arrangement ([where it runs](#where-it-runs)) |
| **the realm is the daemon's, never the runner's host** | a runner on `srv2` reporting to the bus on `srv1` is registering into `srv1`'s realm already. Nothing about where a process sits belongs in the name it serves |
| **so the pool is given a complete name** | `image-scaler@pool1`, a realm that daemon is told to hold. A complete name is taken whole; only a **bare** one is completed with the local host, and that completion is a convenience carrying no authority ([identity § names](../../docs/01-identity-and-roles.md#names)). `@srv1` would claim a location false for three members out of four; `@pool1` claims membership, which is true for all of them and survives a member moving |

So a pool needs no naming machinery of its own — its members are simply given
the whole name, as a run option beside `-N` and `--share`. **Getting it wrong is
silent**: a member that fell back to the default is a healthy service nobody
ever calls.

Which is also why a member **states its hostname at registration**, in a field
of its own: the name no longer carries one, and a listing still has to answer
*where* ([discovery § where a member says it is](discovery.md#where-a-member-says-it-is)).

**That the members are interchangeable is the operator's promise**, and the bus
cannot check it any more than it can check that a service does what its
description says. Two hosts serving one name with different code, or different
`--algo`, is a caller getting different answers to the same question — and it
will look like a flaky service, not a misconfigured one.

Under the runner, sharing is a **run option like the worker count**, so it
lives in `services.json` with `-N` and the confinement rather than in
`config.json` ([what an instance is](#what-an-instance-is)). Same reasoning as
`-N`'s default of 1: whether it is safe to run this thing twice is a decision
the host is already making.

### Many names into one inbox

**Owner-settled, 2026-09-16.** Registration takes an optional **`route`**. When
a record carries one, a message addressed to that record is delivered into the
routed queue instead of its own, carrying **`orig-to`** — the name the sender
actually addressed. Spelled the way `reply-to` already is.

**A runner registers every service it manages with `route` pointing at
itself**, so it reads one inbox and dispatches on `orig-to` rather than polling
a queue per child. That is the motivating case and the reason this exists.

It is the **dual of a pool**, and the two are worth reading together:

| | Names | Inboxes | Readers |
|---|---|---|---|
| Pool (`--share`) | one | one | many, taking turns ([one name on many hosts](#one-name-on-many-hosts)) |
| Route | many | one, the target's | one |

A pool hides *where* a worker runs. A route hides *how many* inboxes a reader
has to watch. Neither hides who the message was for: a pool member reads its
own name, and a routed message says its own in `orig-to`.

This is a **daemon** capability that the runner consumes. Delivery and the
registry are the daemon's; starting children with a route is the runner's use
of it. A client that wants the same fan-in gets it the same way.

**A route governs direct delivery, and subscriptions are not that.**
Owner-settled, 2026-09-16: routing does not apply to pub/sub. A subscriber
receives its copy into its own inbox whatever its `route` says, because
`fanout` is a separate delivery path rather than a send to the subscriber's
name ([topics](../../docs/03-services-and-topics.md#topics)).

The cost is real and is accepted rather than argued away: **a runner whose
children subscribe still reads one inbox per subscribed child**, so routing
does not simplify the pub/sub case at all. What it buys instead is that
`orig-to` keeps one meaning — the name a *sender* addressed — and a reader
subscribed under several routed names cannot be handed several copies of one
message in one inbox with no way to tell which subscription matched.

#### Routing is invisible to the sender, on every path

**Owner-settled, 2026-09-16.** Whatever becomes of a message sent to
`scaler@h` — served, refused, expired, answered with an error — what comes back
names `scaler@h` and nothing else. The sender is not told that a route exists,
where it points, or that something other than the name it addressed did the
work. This is the rule the reply and error paths below are instances of, not a
summary of them.

Two boundaries, because *invisible* cannot mean invisible to everyone:

| | |
|---|---|
| **The reader sees it by construction** | `orig-to` is the dispatch key. A runner that could not see which name a message was for could not serve it. The promise is to the **sender** |
| **The record carries it** | `route` is a field of the record, and whoever may read the record reads it ([ACL](../../docs/02-access.md#acl)). *Where is this served* is answered there — to people with standing, rather than to anyone who can send |

**What cannot be hidden is shared fate**, and this does not pretend otherwise. A
routed name lives in somebody else's queue and is subject to that queue's bound
and retention ([the queue that holds it](#the-rules-that-follow-from-it)), so a
sender can be refused for `scaler@h` because of traffic to names it has never
heard of. Nothing *says* so, and a sender watching closely enough infers it.
The rule governs **what the daemon states**, not whether routing leaves a trace.

#### A route may only name a queue you could send to

**Owner-settled, 2026-09-16.** Without it, `route` is an ACL bypass with extra
steps: register a name, point it at somebody else's inbox, and anyone permitted
to send to *your* name is now writing into *theirs*. **The authority to route
into a queue is the authority to put a message in it**, so routing is not a way
to acquire reach you did not have. This is the shape of the escalation closed
in [0.5.31](../../CHANGELOG.md).

Because a route can be changed later
([a route changes after registration](#a-route-changes-after-registration)),
the check belongs on **every write of the field**. A check on the first write
only is one somebody routes around by writing twice.

**Checked on save, and not again at delivery.** Owner-settled, 2026-09-16.
Delivery stays a queue operation with no ACL evaluation per message, which is
what keeps a routed name as cheap to deliver to as an ordinary one.

The consequence, stated rather than discovered: **a route outlives the
permission that justified it.** Write it while you may send to the target, and
it keeps delivering after the target tightens its ACL. What it cannot do is
*grow* — the traffic still originates from a principal the target admitted at
the time, so the route preserves reach rather than granting it, and that bound
is what makes checking once defensible. Revocation is by the record: whoever may
manage it repoints or clears the route. The target cannot refuse a route on its
own.

#### The rules that follow from it

Still **derived here rather than stated by the owner**, so they can be refused:

| | |
|---|---|
| **The addressed record's ACL is what admits the message** | The sender asked for `scaler@h` and passes `scaler@h`'s ACL. The route target's ACL governs who may *route to* it, which is a different question asked of a different principal at a different time |
| **Routes do not chain** | One hop. If the target is itself routed, delivery stops at the target rather than following the second route — otherwise a cycle is an unbounded loop, and a two-hop route is a thing nobody asked for that arrives free with the wrong rule |
| **The queue that holds it is the queue whose policy applies** | Bound, overflow and retention are the route target's, because that is the queue the message physically sits in. The addressed record's own queue settings describe a queue nothing is delivered to |
| **A routed record's liveness is the target's** | A routed name has no reader and no queue of its own, so reporting its own would say *no reader waiting, nothing queued* forever, about a service that is being served normally. Either it reports the target's or it says the question does not apply to it — what it must not do is answer with a measurement of an empty queue nobody uses ([declared and observed are never merged](../MVP/web/data-dictionary.md#the-rule)). Whichever it is, this is a **record-side** answer given to somebody who may already read `route`, so it is outside the sender promise |

#### A route changes after registration

**Owner-settled, 2026-09-16: yes.** A route is an ordinary field of the record,
changed the way the record's other fields are — re-registering a name is
already an update rather than a collision
([one name on many hosts](#one-name-on-many-hosts)) — and clearing it is a
change like any other, returning the name to its own inbox. Changing it needs
both authorities at once: the standing to manage the record, and the standing
to route into the new target, which is the same rule that governs setting one.

**A route is not ownership, and repointing one is not a transfer.**
Owner-settled, 2026-09-16: **an owner may pass ownership on, and only the owner
may** — which is what the daemon already does
([owner control](../../docs/01-identity-and-roles.md#services),
[who manages a record](../../docs/01-identity-and-roles.md#groups)).
Routing changes nothing about that. The two operations are simply different:

| | Changes | Who may |
|---|---|---|
| Transfer | who the record belongs to, and so who may ask for its token | the owner, and nobody else |
| Repoint a route | which inbox its messages land in | whoever may manage the record |

So a handover of *work* leaves the record, its credential and its owner where
they were, and the owner remains the one who can end the arrangement — while a
handover of the *record* is a transfer, which the owner makes deliberately and
which carries its own conditions ([transfer](../../docs/01-identity-and-roles.md#ownership)).
What routing must not become is a way to do the second by doing the first.

**A change applies to delivery from that moment, and nothing already delivered
moves.** Derived, on the daemon's own precedent rather than invented: an
envelope's expiry is worked out once at accept and changing the queue's TTL
afterwards does not revisit it
([message TTL](../../docs/04-messaging.md#message-ttl)). Delivery is the same
kind of fact. The alternative is a daemon that reaches into a queue somebody
else is reading and takes work back out of it, which is a much larger promise
than routing asked to make.

So re-pointing `scaler@h` from `runner-a` to `runner-b` leaves messages for
`scaler@h` sitting in `runner-a`'s inbox, addressed to a service `runner-a` no
longer manages.

#### Dispatch is keyed on `orig-to`, not on what the runner owns

**Owner-settled, 2026-09-16: a runner keeps dispatching an `orig-to` it no
longer owns — and being routed a name you do not manage is usually deliberate
rather than left over.** That is the general rule, and a handover is only its
most obvious case. A name can be pointed at a runner as a front door, by
somebody who owns neither, and the runner serves it because the message says
what it is for.

**The instance list is not the dispatch table.** It says what this runner
*starts*; `orig-to` says what a message is *for*, and the two answer different
questions. A runner that dispatches by walking its instances will strand work
in the ordinary case, not merely at the edges — which is the whole reason this
is worth stating before it is built rather than discovered by a queue nobody
drains.

Two things follow:

| | |
|---|---|
| **A handover is not complete when the route changes** | it is complete when the old target's queue no longer holds anything for that name. Tooling that reports the handover done at the moment of the write reports it too early. The tail itself is benign — the old runner simply finishes what it was already holding — for as long as it can still serve the name |
| **An `orig-to` the runner cannot serve is answered, not held** | [below](#an-unservable-orig-to-is-refused-to-its-sender). Since being routed an unowned name is normal, being routed an *unservable* one is routine too |

#### An unservable `orig-to` is refused to its sender

**Owner-settled, 2026-09-16: the runner replies with an error and the message
is consumed.** No child for that name, or the child is gone, and the sender is
told so rather than left waiting.

**The shared inbox is what makes holding the wrong answer here.** A runner's
inbox is not one service's queue — it is the queue every name routed to that
runner shares. Work held for one unservable name sits in front of work for
every other, and one misconfigured route becomes an outage for children that
are running perfectly well. Bound and retention belong to that shared queue
([the queue that holds it](#the-rules-that-follow-from-it)), so the damage is
capped but it is not contained to the name that caused it.

The cost, stated plainly: **a transient gap becomes a hard failure.** A child
restarting when a message lands produces an error the sender must retry
through, rather than a short wait nobody notices. That is the trade the owner
made, and it says something about what a route promises — a route is an address,
not a guarantee that something is listening at it. Durability across a restart
is the sender's to arrange by retrying, not the queue's to fake by holding.

By [reply identity](#a-reply-comes-from-the-name-that-was-addressed), that error
comes **from the name the sender addressed** — `scaler@h`, not the runner. The
sender learns its message failed; it does not learn where the name was being
served, or that it was routed at all. **A failure is a path like any other**,
and it is the path where naming the target would be most tempting and most
wrong: a diagnostic that leaks the deployment to whoever can provoke an error
is a worse leak than one that leaks it to whoever can succeed.

#### A reply comes from the name that was addressed

**Owner-settled, 2026-09-16: the reply's `from` is the addressed name.** A
sender that wrote to `scaler@h` is answered by `scaler@h` — the reply path's
instance of [invisible to the sender](#routing-is-invisible-to-the-sender-on-every-path).
Routing is a deployment arrangement, and a caller should not have to learn it
to correlate an answer.

This grants the runner nothing new. It already holds the child's credential in
order to start it ([what the child is told](#what-the-child-is-told)), and it
**may not mint one** — so answering as the child is a use of authority it was
given, not an acquisition. A runner that was never handed the credential for a
name cannot reply for that name either, which is the bound that keeps this from
being an impersonation primitive.

Stated rather than glossed: the `from` is then **the name the work was for, not
the process that did it**. Anyone who needs to know which target served a
message is asking a question the reply does not answer, and is asking it in the
wrong place — the record's `route` answers it, to anyone allowed to look at the
record.

### Long-lived services

**A child in one of the stream forms is started once and kept** — `jsonl` or
`msgpack` — and messages arrive on its stdin, one frame at a time, for as long
as it lives. Everything expensive to
build — a loaded model, an open database handle, a warm cache — survives
between messages, which is the only thing it buys.

The shape forces the lifetime rather than a flag declaring it: `args` cannot be
long-lived, because argv is fixed at exec; a process that reads a *stream* of
messages is by construction one that stays; and a length prefix is only worth
writing when another frame follows it. So there is no second setting to
disagree with the first — the same reason `env.dist` decides whether a service
needs an instance ([the three env layers](#the-three-env-layers)).

| | |
|---|---|
| **one message at a time** | the runner writes a frame and waits for the answering frame before writing the next. Nothing has to be correlated because nothing is out of order, and `-N` keeps its meaning: N children, N hands on one inbox ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)) |
| **a deadline per message, and only here** | killing a one-per-message script costs the process and nothing else. Killing a kept child throws away everything it warmed up, and there is no way past a wedged one without doing it — so the runner waits a bounded time, then kills, restarts with backoff, and logs the message that was in flight as lost |
| **state is the service's own business** | the runner promises nothing about which child handles which message, so whatever a child remembers must not belong to one caller. The first bug here will be a per-caller cache that outlives the caller |
| **up is ready** | no readiness handshake. A child that exits before its first reply is a failed start and backs off like any other |
| **`reload` is `SIGHUP`** | and it is the one verb that exists only for these shapes ([what the runner does](#what-the-runner-does)) |
| **stopping is unchanged** | no further frame is written and the answer in flight is waited for, then `SIGTERM` — bounded by the same deadline, because a wedged child cannot be waited out |
| **a crash still loses only what was taken** | a message leaves the daemon only when a hand is free for it, so the rest is still queued for whatever reads that inbox next |

### On demand

**This is how most runner services are meant to run, not a special case for
rare ones.** Keeping a process resident is what
[long-lived services](#long-lived-services) buys something by — a loaded model,
an open handle, a warm cache. A service with nothing to keep warm gains nothing
from being up, and **most services have nothing to keep warm**: the resident
ones are the exception, and on demand is the ordinary arrangement for the rest.

Sending an SMS is only the clearest case — rare, cheap to start and pointless
to keep resident. Today such a name is registered and its inbox fills while
nothing reads it ([messaging § inbox queues](../../docs/04-messaging.md#inbox-queues)); the work
waits for a process somebody has to have started, and a box ends up running a
process per service for services that are idle almost always.

**The mechanism is one field on a record and one branch in delivery.**

| | |
|---|---|
| **the record carries a fallback channel** | a service may register one, and the runner puts it on the registration of every on-demand service it owns. Declared once, by the thing that can actually act on it |
| **no consumer is the trigger** | a call for a name nobody is reading is **wrapped** by the daemon and delivered to that channel instead of waiting in the inbox |
| **the runner is what reads it** | it takes the wrapper, starts the script for that one task, and hands the result back |
| **nothing is resident in between** | which is the whole point |

**The caller cannot tell.** It made an ordinary call
([messaging § request and reply](../../docs/04-messaging.md#request-and-reply)) and gets an ordinary
reply; only the latency differs. No new verb, no new transport and no second
way to address a service — the wrapper is an ordinary message on an ordinary
channel, and the execution is the one the runner already performs per message
in the `args` and `json` forms ([what an instance is](#what-an-instance-is)).

| | |
|---|---|
| **the wrapper carries the original, and the answer comes back as the service** | reply routing is the original envelope's ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)). The runner returns a result and the daemon unwraps it; a caller that sees the runner's name where the service's belongs is the wrap having leaked |
| **the ACL is applied before the wrap, never after** | the daemon refuses on the record's own ACL exactly as it always does, and only a call that passed is wrapped. Otherwise the fallback channel is a way to reach a service you may not call |
| **a wrapped call is accepted from the daemon and from nobody else** | the channel is a name with an ACL like any other ([identity § acl](../../docs/02-access.md#acl)) — the daemon writes, one runner reads. The runner checks the sender as well, because a wrapper is the one message that asserts *somebody else* called: accepted from the daemon it is that assertion, accepted from anyone it is a way to run a service as a caller you are not |
| **the runner still mints nothing** | it executes and answers. Who called is the daemon's assertion inside the wrapper, never something the runner establishes — the same rule as everywhere else ([what the child is told](#what-the-child-is-told)) |
| **no consumer is not a race** | routing is one decision by the one thing that knows both facts: routed to the channel when there is no consumer, delivered to the inbox when there is. There is no window between a check and a hand-off because there is no check and no hand-off — and either answer stays right afterwards, since a consumer that attaches later reads the next message, and one that leaves later leaves a queued message, which is the ordinary case already ([messaging § inbox queues](../../docs/04-messaging.md#inbox-queues)) |
| **cold start is the caller's latency, on the caller's ttl** | the call pays a process start. A message ttl shorter than that start is a caller that has already given up ([messaging § message ttl](../../docs/04-messaging.md#message-ttl)), and being on-demand does not extend it |
| **a runner that is down is a service that is down** | the wrappers queue on the fallback channel, which is an inbox like any other. Nothing new to reason about |
| **it does not change what a script is** | the same forms, the same env layers ([the three env layers](#the-three-env-layers)), the same sandbox ([sandboxing](#sandboxing)). Only the trigger differs |
| **the kept child is what needs a reason** | the shape already decides the lifetime ([long-lived services](#long-lived-services)) and this does not add a flag beside it. What changes is the default expectation: a stream form stays because it has something to keep, and everything else is on demand unless somebody says why not |

**Start on demand is the other kind, and it is not settled.** There the call
starts the **service**, which then reads its own inbox the ordinary way and may
be stopped again after an idle period.

| | |
|---|---|
| **nothing is wrapped** | the message stays in the inbox it already reached, and the daemon only has to say *somebody should start this*. Less machinery than the wrapped form, not more |
| **the start is paid once, not per call** | a burst warms one process and the rest are ordinary deliveries. Per-call exec is right for an SMS and wrong for anything with a cache to build, so the two answer different questions: **how rare**, and **how expensive to start** |
| **idle shutdown is a second decision** | what counts as idle, and what stopping does to work in flight — a stop already waits for what it took ([what the runner does](#what-the-runner-does)) |

The pair is inetd's per-call exec beside systemd's socket activation, and that
system shipped both for this reason. Whether both are built is
[open](QUESTIONS.md#open-questions).

## What an instance is

A directory — and the directory being there is the **installed** state, not
the running one. It says the instance exists and is configured, and nothing
more.

Three states, each with exactly one home, the way systemd splits them:

| State | Where it lives | Verb |
|---|---|---|
| **installed** | its directory, and a row in `services.json` ([the list of what is installed](#the-list-of-what-is-installed)) | the upload that creates it |
| **enabled** | that row's `autostart` | `enable` / `disable` |
| **running** | neither — it is a process | `start` / `stop` |

A list exists because the runner needs one: **at start it has to know what to
bring up and how many of each**, without walking a tree. Installed and enabled
are separate so that an instance can be configured and kept without running —
put in place before it is turned on, started and stopped by hand, and left
alone by a reboot. That is what `disable` leaves behind.

The registry still answers what exists and what is alive; the runner answers
only what should be up.

Two directories hold it between them, and that split is the point — modes and
owners are in [setup § the two accounts](../../docs/09-setup.md#the-two-accounts):

| | Holds |
|---|---|
| `service.d/<svc>/` | what the service **is**: the code, or a symlink to the same code elsewhere, plus `config.json` and `env.dist` |
| `runner/<svc>/<instance>/` | what this instance is **configured with**: an `env` file, and nothing else |

The author and the host each own one file, and they do not overlap:

| | Owned by | Says |
|---|---|---|
| `config.json` | the **author**, and it travels with the code | how the service starts, what to register — including its **version**, which is why a service too simple to answer a `version` call still has one — and what it **depends** on. The command line lives here and nowhere else |
| `services.json` | the **host** | every service installed here, and per service: whether to start it, how many, what to confine it with ([sandboxing](#sandboxing)), and what it comes after ([the list of what is installed](#the-list-of-what-is-installed)) |

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
bare name is ([identity § names](../../docs/01-identity-and-roles.md#names)) — and `services.json`
may state a **complete** name instead, which is then taken whole. That is the
one thing a host must be able to override, because a pool's members sit in
identically named directories on four machines and must register **one** name
between them ([the name a member registers](#the-name-a-member-registers)).
It stays the host's file rather than the author's for the usual reason: which
pool this copy joins is not something the code knows.

One service with many instances is already in the naming —
`template/instance-name@realm` ([identity § names](../../docs/01-identity-and-roles.md#names)) — so
per-user instances need no new idea.

### The three env layers

A service's **environment** arrives injected before exec — never as a path the
child could open. It is assembled from three files that overlay in one order.
This is the runner's half, and it is not the registry's configuration, which a
service fetches for itself and which the daemon holds
([services § configuring a template](../../docs/03-services-and-topics.md#configuring-a-template)):

| | Carries | Wins |
|---|---|---|
| `service.d/<svc>/env.dist` | the declared surface: every variable the service wants, with a default for each one that is not secret | lowest |
| `runner/<svc>/env` | what every instance of this service shares — the one API key they all use | middle |
| `runner/<svc>/<inst>/env` | this instance's own | highest |

**The more secret it is, the more it wins.** Precedence and visibility run in
opposite directions — a property a reader can check against the modes
([setup § the two accounts](../../docs/09-setup.md#the-two-accounts)) rather than a rule
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
the answer back ([script services](../../docs/08-runner-role.md#script-services)) — which is what "the
script need not know anything" means.

| | talks to the bus | holds a token |
|---|---|---|
| a script under the runner | the runner, on its behalf | **no** |
| a linked service (php, go, …) | itself, through the client library | yes — a variable in its `env` |

The runner comes by the credential for a name it serves the same way anything
else does — by asking for one for a name it owns, which is already the rule and
already stops it collecting anybody else's
([access § getting a token](../../docs/02-access.md#getting-a-token)). It is never given
the power to mint one.

What a child *is* told is **what it is serving** rather than who it is: the
envelope reaches it as environment — sender, `topic`, `tag`, `message_id`
([messaging § envelope](../../docs/04-messaging.md#envelope)). The set is deliberately not
closed — it will grow when services are actually being written, and this is
where it is recorded when it does.

### The list of what is installed

`services.json`, in the runner's home, is **one row per installed service** —
not per started one. It is the inventory and the host's decisions in the same
file, because they are the same list seen twice, and keeping them apart bought
a second file and nothing else.

| Field | Says | Written by |
|---|---|---|
| the name | which directory, and so which `service.d/<svc>` ([what an instance is](#what-an-instance-is)) | the install |
| **`autostart`** | `on` · `off` · `on-demand` — at boot, never, or when first addressed | `enable` / `disable` |
| `-N`, `--share`, confinement, a complete name | the run options, which are the host's and not the author's | the host |
| **`depends`** | what the host adds to, or turns off in, the author's list ([what it comes after](#what-it-comes-after)) | the host |
| **`version`**, and the **origin** when it is a checkout | what was installed: the version `config.json` claimed, and the remote and commit to fetch it again | the runner |
| **`first-started`**, **`last-started`** | when this instance first ran here, and when it last did | the runner |

**The runner keeps the derived half current** — version, origin and the two
dates — at start, or when told to. That is the file being a record of what is
here and not only of what was decided, and it is why a backup needs nothing
generated to go with it.

#### What it comes after

**`depends` is declared by the author and overridden by the host**, the same
two layers as the environment ([the three env layers](#the-three-env-layers)).
What a service needs is a property of the code, so it arrives in `config.json`
with a `git pull` and nobody restates it. What is true *here* is the host's, so
`services.json` may add an entry, and may **turn one off** — the case that
makes this necessary is a dependency that moved to another host, where there is
nothing local to come after.

| | Says |
|---|---|
| `config.json` | what this service needs, from whoever wrote it |
| `services.json` | what this host adds, and which of the author's entries are not local any more |

**It orders starts and does nothing else.** The runner brings a service up
after the ones it still names, and a cycle is refused at load with the cycle
named. It does not wait for readiness, because up *is* ready
([long-lived services](#long-lived-services)), and it does not watch: whether
anything is actually serving a name is a question the bus already answers
([discovery § what a listing answers](../../docs/05-discovery.md#what-a-listing-answers)).

**An entry that names nothing installed here is a refusal**, not a line quietly
skipped — turning it off is how a host says *this one is remote now*, and that
is a sentence somebody wrote rather than a silence the runner read something
into. It is the same trap as a pool member's name ([the name a member
registers](#the-name-a-member-registers)): a typo that is ignored leaves a
service that starts in the wrong order and looks healthy doing it.

### Backing it up

**The backup is `runner/`, encrypted, and that is the whole of it.**
`service.d` is a checkout: whatever is in it can be fetched again from where it
came from, and `services.json` says from where. `runner/` cannot be fetched
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
[separate secret domains](../../docs/09-setup.md#the-two-accounts).

Restoring is the archive backwards: fetch each origin at its commit, unpack
`runner/` over it. A service whose origin is gone is a **named failure** and
not a quieter restore — a host that comes back with four services out of five
and says nothing is worse than one that will not come back.

## Who it runs as

- **Separate user** (default for shared/server use): the daemon as
  `agent-busd` and the runner as `agent-bus-runner`, two accounts that cannot
  read each other's home ([setup § the two accounts](../../docs/09-setup.md#the-two-accounts)).
  Privileged installer once; **no root at runtime**, and the only capability
  anywhere is the supervisor's
  ([processes § why the supervisor holds CAP_CHOWN](../../docs/11-processes.md#why-the-supervisor-holds-cap_chown)).
- **Current user** (personal use): children as you, no runner at all. Zero
  setup — the laptop story with the AUTH role off.

### Where it runs

The runner and the daemon are separate programs, so they need not share a
host. Three arrangements, and the third is what the split buys:

| | daemon | runner |
|---|---|---|
| laptop | yours, local | none — publish by hand ([script services](../../docs/08-runner-role.md#script-services)) |
| one host | local, `agent-busd` | local, `agent-bus-runner` |
| **edge box** | **elsewhere** | local, alone — a machine that hosts services and holds no bus state |

A remote daemon is reached the way anything else here is reached, over **ssh**
([access § the three doors](../../docs/02-access.md#what-a-call-carries)): no port opened to
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

### Reaching the runner

**The runner is a service on the bus**, registered as `runner@<host>` — a name
like any other ([identity § names](../../docs/01-identity-and-roles.md#names)), so a runner on an
edge box stays addressable from the daemon it reports to —
and installing, configuring, enabling and starting are calls to it like any
other. There is **no second ssh door and no account to be let into**: who may
deploy on a host is the ACL on that one service
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
where it was installed from. `service.d` is the opposite — world-readable,
holding no secret, and usually a checkout whose source of truth is the
repository it came from.

## Fits the other pieces

- Child private config is the runner's own — the env layers
  ([the three env layers](#the-three-env-layers)), never a blob held or sealed
  by `agent-busd`. The daemon holds credentials, the runner holds
  configurations, and neither reads the other's
  ([setup § the two accounts](../../docs/09-setup.md#the-two-accounts)); a config the
  daemon stored would be exactly the case that argument rules out. Sealed
  private config ([identity § sealed private config](identity.md#sealed-private-config))
  is a *service's* own secret, sealed to its key, which the runner cannot
  open either.
- Per-child identities make each child individually revocable, and the runner
  registers them the way any owner does — it may not mint their credentials
  ([what the child is told](#what-the-child-is-told)).
- Health hints for children are generated by the runner — it knows how it
  launched them.
- Daemon reload is a separate [operations contract](operations.md#reload).

## Additional script forms

The foreground forms remain defined in [runner § script services](../../docs/08-runner-role.md#script-services).

| Form | Proposed behavior |
|---|---|
| `std` | Raw body bytes on stdin and stdout, one child per message |
| `jsonl` | An envelope line and an answer line per message, with the child kept alive |
| `msgpack` | Envelope and binary body in a frame, with the child kept alive; `uint32` length prefix in network order |

These are existing design choices, not a newly approved wire specification. Implementation remains blocked where [questions](QUESTIONS.md#open-questions) leave a contract unresolved.

## Sandboxing

The managed runner uses the [current sandbox port](../../docs/08-runner-role.md#sandboxing). Its service account needs a user manager before that backend can run: `loginctl enable-linger agent-bus-runner`. Managed children would use `ProtectHome=yes`, because their code and injected environment need no home-directory access. A second backend remains a candidate, not a built dependency; container choices are in [R1.1 questions](../R1.1/QUESTIONS.md#open-questions).


Unresolved details: [questions](QUESTIONS.md#open-questions).

## Addressing several names

Scatter-gather addresses several configured services together. It is separate
from a pool, whose workers share one name and inbox. Selection and aggregation
remain release design work after scope confirmation.

## Service version reporting

A registry record reports the version registered; a standard `version` call
reports what is running. The runner answers from the service description for
simple scripts. This depends on [method metadata](discovery.md#method-metadata); it does
not create an independently versioned program in this repository.
