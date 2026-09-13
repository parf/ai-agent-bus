# Discovery

How participants find each other, and what a human or an agent can see.
("Service discovery" is the name; registration is one operation on it.)

- **The required minimum** is `agent-busd` alone, AUTH role off ([overview §
  principles](00-overview.md#principles)) — auth itself is still required
  ([access](02-access.md)).
- **Direct talk is allowed**: if you already know where something lives, skip
  the lookup.
- Registration carries **health hints**: HTTP endpoint + expected status, TCP
  connect, unix-socket ping, command, interval, timeout. Unix-socket services
  are first-class (`unix:/path`).

## What a listing answers

**Being in the registry and being callable are different facts.** "There is a
MySQL on `db1:3306`" is a complete registration ([services § service
kinds](03-services-and-topics.md#service-kinds)) and nothing on this bus
answers for it; a service template is registered and deliberately does not run
([services § service and template](03-services-and-topics.md#service-and-template)).
So a caller reading a listing needs more than a name:

| Field | Says | Absent means |
|---|---|---|
| **`protocol`** | how to call it, when that is not through the bus ([services § how to call it](03-services-and-topics.md#how-to-call-it)) | an ordinary bus service: send to the name |
| **`reading`** | a read on its inbox is outstanding *now* — something is serving it | registered, but nobody is home |
| **`queued`** | how many messages are waiting in it | none are |
| **`in`** · **`out`** | how many messages have arrived for it, and how many a reader has taken, since the daemon started | none have |
| **`dropped`** · **`expired`** | what its queue lost to overflow, and what outlived its TTL in it, since then ([messaging § overflow](04-messaging.md#overflow)) | it has lost nothing |
| **`oldest`** | how long the message at the head of its queue has been waiting | its queue is empty |
| **`on`** | where its members are running — one entry per member, stated by each at registration ([where a member says it is](#where-a-member-says-it-is)) | nobody said |

All of these are **live state, not registry data** — attached to an
answer on its way out and never stored, so nothing in the registry depends on
who happened to be connected ([overview § principles](00-overview.md#principles)). They are the difference between
*"this name exists"* and *"a call would reach someone"*, which is the question
a caller is actually asking.

An inbox that was drained and one nobody ever wrote to both read as empty.
`in` and `out` are what tell them apart, and they are per name: a busy bus
does not make a quiet service look busy.

### Where a member says it is

**A service states its hostname at registration, as a field of its own.** It
became necessary the moment a name stopped carrying one: a pool's realm claims
membership rather than a location ([identity § names](01-identity.md#names)),
which is right, and leaves *which box do I go and look at* unanswered.

| | |
|---|---|
| **stated, never observed** | the daemon cannot see it. A unix socket has no peer host at all, and a TCP peer address is the network's opinion rather than the machine's name. So the machine says |
| **a label, and never an input** | nothing routes, authorises or decides on it, because nothing checked it. It is for the person reading a listing at three in the morning |
| **one entry per member, not one per name** | a pool registers one name from several places, so what is kept is a set — refreshed by the heartbeat that already says a member is alive, and gone when that member is ([health checker](#health-checker)) |
| **not part of the identity** | the name is the identity ([identity § names](01-identity.md#names)); this is a fact about a process serving it. Keeping the two apart is the whole reason the realm stopped carrying a hostname |

It is also where a **version** would go when a service can say one
([Plans/R1](../Plans/R1/TODO.md)) — same shape, same reason: per member,
stated, and the answer to *which of these four is the odd one out*.

**Loss is per inbox, and the node's total is the sum of them.** A node-wide
count says that something is losing work and not which name to go and look
at; and a queue that is long because it is busy and one that is long because
nobody is reading it are the same number until `oldest` separates them. That
pair is what the dashboard's stuck-inboxes and loss-by-name views are
([what it shows](#what-it-shows)), and a restart puts each back on the inbox
that suffered it rather than on a total.

**A false negative is possible and harmless**: between one long poll and the
next a live service shows `reading: false` for as long as it takes to ask
again. Nothing is lost by calling it anyway — the address outlives the process
and the queue waits ([messaging § inbox
queues](04-messaging.md#inbox-queues)) — so this is a hint for choosing, never
a gate on sending.

## Faces

| Face | For | Does |
|---|---|---|
| **API** | programs | register, look up, send / publish / consume |
| **MCP server** | agents | "what can I use, and how" — generated docs for every service available to the caller, plus tool descriptions where the service exposes them |
| **WEB** | humans | registry browser, health, stats dashboards; curl-friendly; runs as a cgroup-limited child |

The MCP face merges chained levels into one catalog, tagged by origin
([overview § chaining](00-overview.md#chaining)).

## Audience

Per service and topic: who may **see and use** it. Discovery is personalised
and MCP catalogs come pre-filtered.

Audience is answered by the two ACL layers — service first, then master; see
[identity § acl](01-identity.md#acl) for the rules. Groups exist only with the
AUTH role on.

**The daemon filters, because it holds the record.** With AUTH off there are
no mapping files to consult: the service ACL is a field on the record
([identity § acl](01-identity.md#acl)), which the daemon already has, so the
answer is worked out in core and every face — catalog, listing, send — gets
the same one. A face that filtered for itself would be a face that could be
asked a different way.

## Health checker

Itself an agent: registered, replicated 2×, holds a `health` role on the
services it probes.

- **generic** → probe per the registered hints.
- **agent** → heartbeat; K missed → down.

## Stats

Agent heartbeats carry a small metrics blob; probe results (latency, up/down)
are the stats for generic services; queue depth and rates for topics.

Kept in memory — ring buffers, last N hours, fixed resolution — and dumped with
the queues ([messaging § durability](04-messaging.md#durability)), so a
graceful restart keeps the window, and a crash loses the last interval — or all
of it, if periodic dumping is off.

## Dashboard

The WEB child is a **read-only view of what the bus already answers**. It holds
nothing and adds nothing: every page is an API call made **as the person
looking**, so the audience rules decide what a page shows and the face has
nothing of its own to filter ([audience](#audience)).

It shows the envelope and nothing else
([messaging § envelope](04-messaging.md#envelope)): no message bodies, here or
anywhere.

⚠️ What the MVP ships today is the envelope half: a bounded feed of what the
bus has routed lately, read by a separate `agent-bus-web` process over the
API. Bodies are struck out **in the bus**, where the feed is written — so no
reader has to be trusted to leave them alone. The feed and the records beside
it are both filtered per caller: you see the exchanges you were **party to**,
sent or addressed to you, and master sees the node's.  Everything in
[what it shows](#what-it-shows) below it is design.

### Rules it is built to

| Rule | Why |
|---|---|
| **The anonymous page shows what the bus would answer a caller it cannot name: nothing.** A title, the sign-in form, and how to get a token | Nothing on the page can know whether it is exposed — the hostname is public DNS and the bind address is a flag. Uptime is a restart oracle, a service count that moves is a covert channel anyone who can register writes to, and a traffic total is traffic analysis. `GET /healthz`, 200 with an empty body, is the whole public signal |
| **No page ever renders a credential** — a fingerprint of it, when it was issued, when it was last used, and the command that rotates it | A token on a page is in the browser cache, the scrollback and every screenshot, and leaves no trace that it was read, so "was this leaked?" stops being answerable. A fingerprint is enough to match the one in your environment |
| **No JavaScript, no CDN, no external asset** | A signed-in master is looking at the node's whole envelope feed, and the first `<script src=…>` added for a chart inherits that. Graphs are inline SVG or nothing; an avatar is served from this node, never hotlinked, or every page view tells the provider who is looking |
| **The web child writes nothing of its own.** A form posts *as the person*, never as the child | It is the least trusted process and the design gives it no write path ([processes § the processes](11-processes.md#the-processes)). Sign in and sign out are the only two in the MVP; what an owner does to a misbehaving service, the page tells them to run |
| **The form takes a token and nothing else** | A call carries no name to get wrong ([access § what a call carries](02-access.md#what-a-call-carries)), so there is no second failure message for an anonymous visitor to read as an oracle for which names exist |

### Signing in

A person signs in with **the token they already hold**
([access § what a call carries](02-access.md#what-a-call-carries)) — there is no
name to type, no other kind of credential and no password anywhere.

The part worth stating is where the session lives: **in the bus**, which is
the process that holds state ([processes § what is shared](11-processes.md#what-is-shared)).
The child forwards it once, the bus answers with an expiring session id,
and from then on the browser carries that id and nothing else.

| | |
|---|---|
| the cookie | the session id alone — `HttpOnly`, `Secure`, `SameSite=Strict`, idle timeout. Never the token, never in a URL |
| a restart | logs nobody out, because the child was holding nothing. A session map inside it would be a second store, of the worst possible contents: every signed-in person's live credential in the one child that is restarted with backoff |
| what the child holds | nothing. It stops reaching the bus over the owner's socket the moment people sign in — a web child with the owner's authority is a credential mint ([access § getting a token](02-access.md#getting-a-token)) |
| enrolling | not here. The proof is a signature made by the host's own `ssh-keygen` ([identity § proving possession](01-identity.md#proving-possession)) and a page with no JavaScript cannot make one, so what the dashboard does for a stranger is print the command |

### What it shows

Staged, because most of it needs the daemon to keep something it does not keep
yet — and that is the cost, not the page. The MVP rows are the ones that make
the bus debuggable by the people sharing it.

| View | Stage | What the daemon must start keeping |
|---|---|---|
| the sign-in page and the token help | MVP | — |
| **registry**, as this caller may see it: kind, owner, protocol, description, `reading`/`queued`/`in`/`out`, the configuration's digest | MVP | — it is `/ls` |
| **stuck inboxes** — a backlog with nobody reading, oldest first, marked when the queue is at its bound. The one view an incident actually needs | MVP | — `oldest` and `reading` on the record ([what a listing answers](#what-a-listing-answers)) |
| **exchanges** — the envelope feed grouped by topic and tag, so a request, its `ack`, its reply and its `done` are one row, and an answer past its deadline is marked late | MVP | — the feed is filtered per caller ([dashboard](#dashboard)); grouping and the late mark are the page's |
| **my names** — what I hold a credential for, its fingerprint, when it was issued and last used, and how to rotate it | MVP | — the caller asks for its own, and gets a fingerprint rather than the token ([token lifetime](02-access.md#token-lifetime)) |
| **loss by name** — what each inbox dropped to overflow and what expired in it | MVP | — `dropped` and `expired` on the record ([what a listing answers](#what-a-listing-answers)) |
| **refusals** — how many calls were refused and why: bad credential, ACL, unknown receiver, second reader, full queue | MVP | ⚠️ the counters are on `status` ([refusals](#refusals)); the short per-caller list is still to come |
| **node** — its name, uptime, the registry's totals, and whether the last stop was clean | MVP | — `status` carries the unclean-restart fact |
| **people** — who holds a credential: name, person name, avatar, master or not, what they own | MVP | the credential store answering *which names*, and the person fields ([identity § registration](01-identity.md#registration)) |
| **groups** and who is in them | R1 | groups themselves ([identity § groups and roles](01-identity.md#groups-and-roles)) |
| **health** — up, down, and how long since the last probe | R1 | the health child ([health checker](#health-checker)) |
| **load** — calls per minute and per hour, per name and for the node | R1 | the ring buffers in [stats](#stats). Inline SVG; `/metrics` ([exports](#exports)) is what a real graphing stack reads |
| **runner** — what a `runner@<host>` manages: every service it knows, which are enabled and which are up ([runner § what an instance is](08-runner-role.md#what-an-instance-is)), with the verbs on each, and a form that installs a new instance | R1 | nothing new — the runner is a service and answers like one ([runner § reaching the runner](08-runner-role.md#reaching-the-runner)). Every control **posts as the person** ([rules it is built to](#rules-it-is-built-to)), so the page drives a runner it has no authority over |
| **children** — bus, web, auth: alive, restarted how often. Not the runner, which the supervisor does not start and has nothing to report about | R1 | the supervisor reporting into the bus. Until then that answer is `agent-bus status` and the page says nothing about it |
| **origin** — which node a record came from | R1 | chained registries ([overview § chaining](00-overview.md#chaining)) |
| phone and IM handles; one person record joining `parf@github` and `parf@realmo` | Future | — |

A **load graph is the one thing here that cannot be faked cheaply**: the
daemon holds counters since start and no history at all, so a graph is the
ring buffers or it is a line that silently restarts at zero after a crash.
Numbers now, graphs at R1 — and never a time-series database
([stats](#stats)).

### Refusals

A bus that is quiet and one that is refusing every call look identical from
outside. `status` carries **how many calls were turned away and for what**,
counted where a refusal becomes a status code so that the reason and the code
cannot drift apart.

| Reason | |
|---|---|
| `credential` | the token is not one, or none came at all — the only thing a call carries ([access § what a call carries](02-access.md#what-a-call-carries)) |
| `acl` | the service, the record's owner, or a private configuration said no ([identity § acl](01-identity.md#acl)) |
| `unknown` | no such name ([messaging § verbs](04-messaging.md#verbs)) |
| `second-reader` | an inbox already has a reader, and neither asked to share it ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox)) |
| `full` | the receiver's queue is at its bound and refuses rather than loses ([messaging § overflow](04-messaging.md#overflow)) |
| `enrolment` | a challenge that did not hold ([identity § proving possession](01-identity.md#proving-possession)) |
| `malformed` | the caller got the request wrong. One reason, not eight: *"you sent nonsense"* is a single answer however many ways there are to send it |

**Only reasons that have happened appear** — a reason with a zero beside it is
noise on every other node. And a fault of the daemon's own is a 500 and is
**not** in here: refusing a caller and failing one are different things to be
told about, and folding them together would answer *"how often am I refusing
callers?"* with a number that includes our bugs.

### Where it listens

The dashboard is **`https://agent-bus.localhost.direct`**. That name, and every
name under `*.localhost.direct`, resolves to `127.0.0.1` in public DNS, so a
developer host needs no `/etc/hosts` line and a browser still gets a real
hostname and a real certificate.

| It wants | Default |
|---|---|
| the certificate and its key | `~/.local/state/agent-bus/agent-bus.localhost.direct.crt` and `.key`, overridable with `-cert` / `-key` |
| the port | 443, falling back to 8443 when the process has no `CAP_NET_BIND_SERVICE` |
| no certificate at all | plain HTTP on `127.0.0.1:7878`, announced in the log |

The pair comes from <https://get.localhost.direct/>: the **self-signed
bundle**, `localhost.direct.SS.zip`, zip password `localhost`, good until
2034-11-17 and costing one trust decision per OS or browser. It is the only
bundle there worth downloading — the CA-signed one beside it expired and no
browser accepts it.

**A `.key` never goes
into a repository or anywhere else public** — that is the publisher's own
condition, and a leaked key is revoked; the repo's `.gitignore` refuses the
extension rather than trusting anyone to remember.

## Exports

Prometheus `/metrics` first (Grafana reads it); OTLP / StatsD secondary.

## Debug mode

An admin may switch a service into debug mode, which keeps a trace of that
service's messages. Admins only. Bodies in the trace are ciphertext — unless
the service has switched encryption off in its own config
([access § encrypted sessions](02-access.md#encrypted-sessions)), the usual
pairing while developing.
