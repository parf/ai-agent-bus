# Discovery

📌 **TL;DR:** Discover accessible records and inspect what the daemon actually knows.

## Status

| MVP | Scope |
|---|---|
| Built | Filtered listings and catalog, all [required dashboard tabs](#required-tabs), administration, envelope-only diagnostics and [web authority isolation](11-processes.md#web-authority-boundary). |
| Pending | [All-reader count](#readers), installed [browser acceptance](#browser-acceptance) and resource limits. |

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
| **`reading`** | an unfiltered read is outstanding now; current implementation excludes filtered reads | no unfiltered read observed; this does not mean nobody is attached |
| **`queued`** | how many messages are waiting in it | none are |
| **`in`** · **`out`** | how many messages have arrived for it, and how many a reader has taken, since the daemon started | none have |
| **`dropped`** · **`expired`** | what its queue lost to overflow, and what outlived its TTL in it, since then ([messaging § overflow](04-messaging.md#overflow)) | it has lost nothing |
| **`oldest`** | how long the message at the head of its queue has been waiting | its queue is empty |
| **`at_bound`** | its queue held the limit it is allowed **when the question was asked**. Not a prediction about the next message: a waiting reader is handed one without it ever queueing, and enqueueing prunes what has expired before it tests fullness, so if the queue is still full then the record's [overflow policy](04-messaging.md#overflow) applies — refuse, or forget the oldest | there is room. The daemon answers it because a record that declares no bound takes the daemon's, and a reader cannot know what that is |

These are **observations attached to the listing**, not values a registrant
may state. Counters survive through snapshots; reader presence does not ([overview § principles](00-overview.md#principles)). They describe the observed inbox,
not service health or a guarantee that a particular request will be handled.

An inbox that was drained and one nobody ever wrote to both read as empty.
`in` and `out` are what tell them apart, and they are per name: a busy bus
does not make a quiet service look busy.

## Readers

**Accepted; implementation pending (Q70).** The web shows one **Readers** count
per visible inbox: all currently outstanding consume requests, filtered and
unfiltered together. No separate counters or breakdown by filter type.

This helps an operator see whether anything is waiting to read. Zero means no
read is outstanding at that instant, not that the service is dead; a reader may
be processing a message between reads. A positive count does not promise that
a particular message matches, or that any work has finished. Count waiting
requests, not processes, sessions or completed reads.

The current `reading` flag cannot provide this count. Normal service behavior
follows the [full-inbox reading rule](04-messaging.md#one-reader-per-inbox).

## CLI listing

**Built:** `agent-bus ls -h` renders a table; plain `ls` retains JSON.
Both accept a single name or `--kind` filtering through the same API calls.
The table shows name, kind, owner, reader presence, queued count and description.
Reader presence is `yes` or `no`, or `-` for an external protocol; it is
the [listing observation](#what-a-listing-answers), not a health check.
An empty result says `No matching records.`; lookup errors remain errors.

## Faces

| Face | Built | Pending MVP |
|---|---|---|
| API | Registry, messaging, credentials, sessions and dashboard administration | — |
| MCP | Bus tools and a catalog filtered by the daemon | — |
| WEB | [Required tabs and controls](#required-tabs), filtered through the caller's API access | Installed browser acceptance and resource limits |

## MCP minimum

**Required for MVP:** the [runtime integrations](08-runner-role.md#runtime-integration-delivery)
and every shipped [launcher](08-runner-role.md#smart-launchers) come with the bus MCP
tools configured and callable in the session.

| Capability | Required outcome | Existing implementation |
|---|---|---|
| List the bus | Discover registered agents, services and topics through the caller's [catalogue view](#audience) | `ab_ls` in the [MCP face](../src/mcp/server.ts) |
| Call a service on the bus | Send to a named service and receive its correlated answer or explicit completion under the [request/reply contract](04-messaging.md#request-and-reply); bus acceptance alone is not completion | `ab_send` plus a filtered `ab_consume`, or delivery through the active push adapter; [MCP face](../src/mcp/server.ts) |

These are minimum capabilities, not a restriction on the remaining tools.
The tools and launcher wiring are built; full live-runtime and fresh-host
acceptance remains pending. Both capabilities use the caller's existing
[ACL](02-access.md#acl); loading the tools grants no additional authority.

## Audience

The daemon filters, because it holds the records and enforces the
[ACL](02-access.md#acl). A face requests the caller's view and adds no
independent access policy. There are no chained catalogs or group expressions
in the MVP.

## Dashboard

The WEB child presents the bus and forwards administration **as the person
looking**. The daemon owns state and authorization: every page and form uses
the visitor's authority, and the face has no independent privileges
([audience](#audience)). The [required tabs](#required-tabs) use the same daemon authorization as direct API calls.

It shows the envelope and nothing else
([messaging § envelope](04-messaging.md#envelope)): no message bodies, here or
anywhere.

Bodies are struck out **in the bus**, where the feed is written — so no reader
has to be trusted to leave them alone. The feed and the records beside it are
both filtered per caller: you see the exchanges you were **party to**, sent or
addressed to you, and master sees the node's.

**The node's own counters are the node's, not the caller's.** Uptime, the
registry's totals and the [refusal counts](#refusals) describe the daemon, and
anybody who may ask sees the same numbers — there is no narrower audience for
them. Somebody with no access sees nothing at all, which is the whole of the
restriction. A page showing them beside a caller-filtered list must say which
is which: *the node refused 40 calls* and *you were refused 2* are both true,
and neither is the other.

The diagnostics page carries the built rows of [what it shows](#what-it-shows);
service/channel, user and group administration and activity graphs have separate pages. The ordering, the grouping and the late mark
are the page's; everything else on it is an answer the bus gave that caller,
so a view cannot show more than the caller may ask for.

### Required tabs

**Required MVP, built.** Owner confirmation of scope, 2026-09-13.
The daemon owner has the administrative view; other visitors see and change only
what the daemon permits. “All” means all visible to that visitor.

| Tab | Required functionality |
|---|---|
| Registered services | My / all; active / inactive filters; details and [owner controls](01-identity-and-roles.md#services), with owner, maintainers group, access, reader presence and queue statistics. Excludes Personal services, which have their own tab. Administrative availability and serving / offline are distinct observations |
| Personal services | Owner-tagged services grouped separately without changing access. Ordinary visitors see their own; the daemon owner may filter by owner among ACL-visible records |
| Users | List and details; add, edit, activate, pause and ban; show owned services, group membership and administrative authority |
| Groups | List and details; create, edit and manage flat membership; basic service and channel access. Include the daemon Administrator group and each record's assigned maintainers group under the [authority rules](01-identity-and-roles.md#groups); retire groups by emptying them, with no delete control |
| Activity graphs | Recent traffic, messages dequeued, drops, expirations and refusals; per-service and per-channel filtering. Dequeued messages are not proof of successful execution. Use bounded history and inline SVG; [sampling and retention](#activity-history) are bounded |
| Registered pub/sub channels | List and details for pub/sub and queue topics; create, edit and remove; subscriptions, owner, maintainers group, permissions, TTL, capacity and overflow policy |

The [Personal Services view](03-services-and-topics.md#personal-and-shared) is built.

[Administrative and record authority](01-identity-and-roles.md#groups) applies
to every control and to direct API calls. Membership and policy changes must
survive restart. User lifecycle effects are [daemon policy](01-identity-and-roles.md#user-states),
not merely labels on the Users page.

Keep the [built views](#what-it-shows), including inboxes holding messages, exchanges,
credential fingerprints, losses, refusals and node status, accessible in the
new navigation. Their existing functionality is not deferred by this split.
[Optional additions](../Plans/R1/discovery.md#dashboard-extensions) belong to R1.

### Activity history

The bus samples record counters once a minute, independently of dashboard visits.
It keeps one hour plus the baseline for differences; the dashboard also includes
the current partial interval. History is in memory and starts fresh after restart.
These are the initial implementation defaults; longer history and export remain
[R1](../Plans/R1/discovery.md#dashboard-extensions).

Graphs and their accessible value table report accepted, dequeued, dropped,
expired and refused counts. Aggregates include only currently visible records;
pub/sub copies count in the subscriber inboxes that accept them. Per-record
refusals count failed sends and reads; daemon administrators with master access
also see node refusal totals, on the terms [refusals](#refusals) sets — every
endpoint refusal, including authentication failures and malformed requests, and
not the router's rejections or our own failures. Bodies never
enter this history.

Adding a subscription remains the subscriber's opt-in. Channel owners and
maintainers can remove a subscription; they cannot force another inbox to subscribe.

### Rules it is built to

| Rule | Why |
|---|---|
| **The anonymous page shows what the bus would answer a caller it cannot name — except for the facts the owner named.** A title, the sign-in form, how to get a token, and [what a node says about itself](#what-a-node-says-about-itself) | The default is still nothing, and the reasons hold: uptime is a restart oracle, a service count that moves is a covert channel anyone who can register writes to, and a traffic total is traffic analysis. The owner weighed each of those against a stranger being unable to tell what this node is or whose it is, and published a **closed list** anyway. Everything not on that list stays behind the gate, and the list grows only by an owner decision |
| **No page ever renders a credential** — a fingerprint of it, when it was issued, when it was last used, and the command that rotates it | A token on a page is in the browser cache, the scrollback and every screenshot, and leaves no trace that it was read, so "was this leaked?" stops being answerable. A fingerprint is enough to match the one in your environment |
| **No JavaScript, no CDN, no external asset** | A signed-in master is looking at the node's whole envelope feed, and the first `<script src=…>` added for a chart inherits that. Graphs are inline SVG or nothing; an avatar is served from this node, never hotlinked, or every page view tells the provider who is looking |
| **The web child writes nothing of its own.** A form posts *as the person*, never as the child | It is the least trusted process and the design gives it no write path ([processes § the processes](11-processes.md#the-processes)). Built administration forms forward the visitor's session to daemon-enforced operations and require an exact matching Origin. Responses are not cached; credentials and existing private configuration are never populated into forms |
| **The sign-in form takes a token and nothing else** | A call carries no name to get wrong ([access § what a call carries](02-access.md#what-a-call-carries)), so there is no second failure message for an anonymous visitor to read as an oracle for which names exist |

### What a node says about itself

**Owner-settled, 2026-09-16.** A short closed list of facts about the node is
published to **anybody who can reach the dashboard, signed in or not**: what it
is, where it runs, whose it is, how long it has been up and how many calls it
has served. They appear in the shell every page shares and on the **sign-in page**,
which is the whole point — somebody who arrives at a bus they do not have a
credential for should be able to tell what it is and whose it is without asking
anybody.

| | |
|---|---|
| what is published | release, host name, daemon owner name, uptime, **calls served** — and the build. **Nothing else**: no record names, no principals, no refusal counts, nothing about who is using it |
| where each one shows | **the header has a two-row bus mark** derived from the [project artwork](img/agent-bus.png) at the far left. To its right, one line carries release, `@` host, owner, **`uptime:`**, then **`calls:`** with `minute:`, `hour:` and `total:`; the navigation sits below that line, beside the logo. **The footer carries the build and nothing else** — one line, no explanatory text. The owner set the one-line identity and footer rules, chose those three figures (a five-minute and a day window were each tried and removed), and set the labels |
| to whom | any caller that reaches the face, with no credential and no session |
| host name | the machine's hostname as the OS reports it, `srv1`. The daemon has no node name of its own, so this is a new field rather than a restatement of one; it is not the realm, which the owner's name already carries |
| uptime | a plain figure behind an explicit label, `uptime: 1h23m`, as the owner asked. It is what was true when the page rendered, and the page does not refresh itself |
| calls served | the count of **HTTP requests the bus process has served**, over the **last minute** and **the last hour**, plus the **total since the daemon started**. Every request on every listener, gated or refused or served — it is counted before the handler runs, so it is traffic reaching the daemon rather than work it agreed to do. A node-wide figure, not this caller's. **No host reading is published**: the owner asked for the daemon's own calls only, and an OS load average is a fact about the machine rather than about this node |
| what it costs | an unauthenticated visitor learns the host's name, who runs this node, how long it has been running and how much traffic it carries. Each was put to the owner and accepted. The call counts are the most revealing of these and were accepted explicitly: a total that moves is traffic analysis, and the answer is that it is a total — it names no record, no principal, no endpoint and no direction of business |
| how it is read | **`GET /identity`**, a public daemon call answering these fields and nothing else to a caller with no credential. There was no such call: every route but enrolment and a root redirect sits behind the token gate, and `GET /status` is authenticated and answers refusals and the caller's own standing besides. So this is a new endpoint rather than a relaxation of `/status`, which keeps its gate and its contents |
| the face's part | it asks as anybody does. The face still holds no credential and still acts as the visitor for everything else ([web authority boundary](11-processes.md#web-authority-boundary)): this is a fact the daemon publishes, not a privileged call the face makes |

The total needs no sampler at all: it is the counter itself. The minute and
hour windows come from a **separate 61-sample history of that counter** — plain
readings, no records and no names in it — driven by the **existing minute
ticker**, so there is no additional timer. It is not the per-record activity
the dashboard graphs; those samples stay what they were and are counted
per record.

**Why a total rather than a day.** A day window was asked for and withdrawn.
The counter is process-local and resets with the daemon, so on any node
restarted within 24 hours `day:` would report everything since start while
calling itself a day. `total:` reports the same number and is honest about it —
and it needs no span of its own, because `uptime:` is on the same line and
scopes it.

**This figure is nobody's listing.** It is a process-local counter, incremented
where every HTTP request enters the bus process, with no caller, no record and
no ACL anywhere in it. That is what lets it be published to a stranger at all:
there is no per-caller view to get wrong, because there is no per-caller view.
The dashboard's activity series is the other thing entirely — filtered to what
one named caller may see, and unable to answer this question for a caller it
cannot name, since for a stranger it would report a confident **zero** on a
busy node.

**What the counter can and cannot answer.** These are properties of what is
being counted, not defects to be fixed here. They are documented here; the
compact dashboard does not explain them:

| | |
|---|---|
| it counts requests **admitted**, not work completed | the counter increments before the handler runs, so a refusal, a router 404, a bad token and a served call all count the same. A node being hammered with rejected requests reads as busy, which is correct — it is traffic reaching the daemon, not work the daemon agreed to do |
| a long poll counts when it **starts** | a waiting `consume` holds one request open for as long as it waits, and it was counted on arrival. On a bus whose faces sit in long polls, a quiet minute in which several readers attach still shows calls. Nothing is wrong with the figure; it is answering a different question than "how much happened" |
| the dashboard counts itself | `GET /identity` is a request like any other, so loading a page adds to the figures that page then shows. No page refreshes itself ([what it shows](#what-it-shows)), so an open page left alone generates nothing; it is a **person** reloading who moves the number they are watching |
| the windows are **sampled**, and the header does not say by how much | a window begins at the newest reading at or before its cutoff, so its span is **whatever the readings allow**, not what its name says. With the usual minute cadence and enough history, `minute:` covers roughly one to two minutes and `hour:` a little over the hour; a delayed tick widens it, a young node shortens it below the nominal span entirely, and it matches exactly when the cutoff falls on a retained reading. None of those is the guaranteed case, which is why the daemon measures each span rather than reasoning about it. **The header prints the label alone**, by owner decision at 0.5.40, taken with this stated: `uptime:` beside it shows how young the node is, and that was judged enough for a glance. The span is still measured and still published — `observed` on each window of `GET /identity` — but **no page says so**: the answer lives on the API, not in the dashboard |
| history is **shorter than the window** after a restart | the counter starts at zero with the process, and an hour of readings takes an hour to accumulate. **A window's span is its own, not the node's age**: on a node up three minutes, `minute:` still finds a baseline near its own cutoff and covers about a minute, while `hour:` covers the three minutes it has. `observed` carries this on the wire; the header does not. **Unobserved history is never shown as zero** — that is the distinction this whole layer exists for |
| `total:` is **exact**; the windows are not | the total is the counter read directly, with no sampling in it. Only `minute:` and `hour:` are differences between a reading and now, and only they carry an `observed` span on the wire |
| the three may legitimately be **equal** | during the first minute, and whenever every call the node has served falls inside the shortest observed window. Nothing is wrong and nothing should be built assuming they differ — but nor do they collapse merely because the daemon is young: at ten minutes, `minute:` covers the last one while `hour:` and `total:` cover all ten |
| all three reset on restart | the counter lives with the process. A restart is not a quiet node, and `uptime:` on the same line is what tells them apart |

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
| a web-child restart | logs nobody out, because the child was holding nothing. A session map inside it would be a second store, of the worst possible contents: every signed-in person's live credential in the one child that is restarted with backoff |
| what the child holds | nothing. It stops reaching the bus over the owner's socket the moment people sign in — a web child with the owner's authority is a credential mint ([access § getting a token](02-access.md#getting-a-token)) |
| enrolling | not here. The proof is a signature made by the host's own `ssh-keygen` ([identity § proving possession](02-access.md#proving-possession)) and a page with no JavaScript cannot make one, so what the dashboard does for a stranger is print the command |


A bus restart invalidates browser sessions: their map is not persisted.

**So does removing the credential they came from.** A session is a credential
without being a token, so one that outlived its token would leave a name
answering for up to the idle timeout after the daemon decided it answers for
nothing — [no registration, no access](01-identity-and-roles.md#unregistering) not
holding, quietly. Unregistering, deleting a service and the
[ownerless sweep](02-access.md#ownerless-credentials) all end that name's
sessions with its token.

### What it shows

| View | MVP status | Source or remaining dependency |
|---|---|---|
| the sign-in page and the token help | Built | — |
| **registry**, as this caller may see it: kind, owner, protocol, description, `reading`/`queued`/`in`/`out`, when the record was last written, the configuration's digest | Built | — it is `/ls` |
| **inboxes holding messages** — a backlog, oldest first, marked when the queue is at its bound and when no unfiltered read is outstanding. Holding is not being stuck, and `reading` excludes a filtered read, so neither is stated as more than it is. The one view an incident actually needs | Built | — `oldest` and `reading` on the record ([what a listing answers](#what-a-listing-answers)) |
| **exchanges** — retained messages and referenced receipt evidence | Built | [correlation and limits](#retained-exchanges) |
| **my names** — what I hold a credential for, whose it is and what it is for, its fingerprint, when it was issued and last used, and how to rotate it | Built | — the caller asks for its own, and gets a fingerprint rather than the token ([token lifetime](02-access.md#token-lifetime)). A person's own identity is distinguished from the services they registered. A credential [goes with its address](01-identity-and-roles.md#unregistering), so the list stays names something answers on |
| **loss by name** — what each inbox dropped to overflow and what expired in it | Built | — `dropped` and `expired` on the record ([what a listing answers](#what-a-listing-answers)) |
| **refusals** — how many calls were refused and why: bad credential, ACL, unknown receiver, second reader, full queue | Built | The built page shows only refusal reasons that occurred; counters are on `status` ([refusals](#refusals)) |
| **node** — its name, uptime, the registry's totals, and whether the last stop was clean | Built | — `status` carries the unclean-restart fact |
| **people** — identities, profiles, local avatars, authority, state, group membership and owned services | Built | [person records](01-identity-and-roles.md#users-and-profiles) and [user lifecycle](01-identity-and-roles.md#user-states) |

### Retained exchanges

**Built.** Diagnostics preserves each message's identity, routing labels and
receipt references within the caller's [visible feed](#dashboard). A bounded
history is an observation window: absent evidence does not establish failure
or unfinished work, and a failed load is shown separately from an empty feed.

| Evidence | Presentation |
|---|---|
| Receipt references one retained ordinary message, matches its receiver and full return route, and was observed no earlier | Group with that message; expose the receipt's identity, reference, sender and time |
| Missing, ambiguous or inconsistent reference; reference to another receipt | Keep the receipt separate and explain what prevents correlation |
| Ordinary message with a tag matches an earlier message's return route | Keep a separate row and link possible responses; a route match proves neither a reply nor completion |
| Acknowledgement or completion receipt is grouped with the original | Report that receipt as observed; do not infer execution success from a dequeue or ordinary message |
| Grouped receipt arrives after the original's deadline | Mark it after that request's deadline; equality is not late |
| Ordinary message has exactly one possible response match with a deadline | Qualify the late mark by that match; ambiguous matches supply no single deadline |

Return-route overrides replace the original destination and labels for these
comparisons. Receipts from topic subscribers or queue workers stay separate
when their identity differs from the addressed topic; untagged ordinary
messages get no inferred response links. Rows sort by newest observed activity,
with message identity breaking ties; displayed times include their UTC offset.
Bodies never enter this view. [MVP verification](../Plans/MVP/done/exchange-evidence.md#checks)
records the implemented checks; installed browser acceptance remains separate.

### Refusals

A bus that is quiet and one that is refusing every call look identical from
outside. `status` carries **how many calls were turned away and for what**,
counted where a refusal becomes a status code so that the reason and the code
cannot drift apart. Every refusing path calls one shared counter, which is what
keeps the figures whole — a fact about the paths there are, not a guarantee that
a future one cannot answer around it.

**An addressed endpoint's refusals are counted whatever the caller's standing**
— a bad token and a malformed request alike. Two things are not counted, and
neither is a caller being turned away: a route the router rejects before any
handler runs, and a failure of ours, which is a `500`.

| Reason | | |
|---|---|---|
| `credential` | `401` | the token is not one, none came at all, or it backs a name the daemon knows nothing about — all three are *who are you*, and none of them is a state anybody can lift ([access § what a call carries](02-access.md#what-a-call-carries)) |
| `acl` | `403` | the service, the record's owner, or a private configuration said no ([identity § acl](02-access.md#acl)) |
| `suspended` | `403` | a user state is in the way: the caller's own, or that of the owner of the name being called ([user lifecycle](01-identity-and-roles.md#user-states), [services of a paused or banned user](01-identity-and-roles.md#user-states)) |
| `enrolment` | `403` | a challenge that did not hold ([identity § proving possession](02-access.md#proving-possession)) |
| `unknown` | `404` | no such name ([messaging § verbs](04-messaging.md#verbs)) |
| `disabled` | `409` | the receiver's record is turned off by its owner ([owner control](01-identity-and-roles.md#services)) |
| `busy` | `409` | removal conflicts with current state: an inbox has queued messages or a waiting reader ([unregistering](01-identity-and-roles.md#unregistering)), or a credential is backed by a user, record or retained service ([cleanup](02-access.md#ownerless-credentials)) |
| `second-reader` | `409` | an inbox has an incompatible outstanding reader; sharing requires both readers to ask ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox)) |
| `name-taken` | `412` | a registration that asked for an unheld name found it held ([registration](01-identity-and-roles.md#registration)) |
| `full` | `429` | the receiver's queue is at its bound and refuses rather than loses ([messaging § overflow](04-messaging.md#overflow)) |
| `malformed` | `400` | the caller got the request wrong. One reason, not eight: *"you sent nonsense"* is a single answer however many ways there are to send it |

**Two answers a caller must never confuse.** `401` is *who are you* — the
credential is missing or is not one. `403` is *I know who you are and you may
not* — authenticated, and refused for lack of permission. Retrying a `403` with
the same credential is a caller asking the same question twice; a `401` is
worth presenting a credential for.

**One suspension, one reason.** A call refused because the caller is banned and
one refused because the *owner of the name they called* is banned are the same
fact seen from two sides, and get the same code and the same reason rather than
a second of each. The caller is told what they can act on either way, which is
nothing: no credential they could present and no permission anybody could grant
makes a suspended name answer, and the code already says do not retry this.

A record's own state is a reason of its own, and deliberately not `unknown`:
*turned off* and *no such name* send a caller to different places, so they are
never the same answer. Declared states beyond `disabled` are
[R1.1 work](../Plans/R1.1/records.md#down-and-retired) and would each bring
their own reason rather than borrow one.

**Only reasons that have happened appear** — a reason with a zero beside it is
noise on every other node. And a fault of the daemon's own is a 500 and is
**not** in here: refusing a caller and failing one are different things to be
told about, and folding them together would answer *"how often am I refusing
callers?"* with a number that includes our bugs.

### Where it listens

The dashboard is **`http://127.0.0.1:6780`**, and that is the whole of it: an
address it binds, not a name anybody has to make resolve.

| It wants | Default |
|---|---|
| where to listen | `127.0.0.1:6780`, with `-addr` or `AGENT_BUS_WEB_ADDR` |
| a certificate | **none.** Supply `-cert` *and* `-key` and it serves HTTPS on the address it was given. Ask for one and miss it and it **refuses to start**: either flag is the ask, either without the other is the same refusal, and a log line nobody reads is not an answer when the page they open is unencrypted |
| a port it may not bind | an error. No port is a default any more, so every one was asked for on purpose and none is silently traded for another |

**The bus does not listen off this machine**, so the page is for the person at
it. A borrowed public hostname bought a certificate a browser would accept and
nothing else; the one this project used had expired, which left the trust
decision without even that to show for it. Anybody who wants HTTPS here has a
certificate of their own and says where it is.

**The API's own root sends a browser here.** `http://127.0.0.1:6767/` is the
daemon, which has no page: a person who typed it in wanted the dashboard, and
gets `301` to it. Only the exact root — a mistyped route stays the 404 it is.
The daemon is told where to point, with `-dashboard` or `AGENT_BUS_DASHBOARD`,
defaulting to the address above; empty serves no root at all. The redirect is
permanent because the root will never grow a page of its own, so a browser may
stop asking: moving the dashboard afterwards is a thing to clear from a cache.

## Browser acceptance

**Required MVP, pending installed acceptance.** Verify the dashboard in a real
browser under its installed scheme, hostname and cookie policy. Exercise
sign-in/out, the required controls, activity graphs and denial paths at each
authority level. Web and bus restarts follow the [session contract](#signing-in).
HTTP handler tests and command-line cookie jars remain useful evidence but
do not establish this browser workflow. [F.12](../Plans/MVP/TODO.md#remaining-work)
owns the installed exercise and mutation checks.

## Identity labels in web and CLI

**Accepted display requirement; implementation pending.** In the web interface
and human-readable CLI output, use:

| Label | Entity |
|---|---|
| 👤 User | Registered person |
| 🤖 Agent | Agent identity |
| ⚙️ Service | Service identity |

These glyphs label entity types, not authority or health. Keep the visible text
beside the glyph; Owner, Administrator, Maintainer and Member remain separate
[role labels](01-identity-and-roles.md#role-names-and-scopes). Use the identity
and record facts returned by the daemon rather than guessing type from a name.
This vocabulary is for displayed labels; it does not rename API kinds, alter
JSON output or prescribe MCP output.

### ACL editing

ACL textareas use the project's plain-text ACL syntax, not the display glyphs.
Users must not need to type Unicode to identify a user, agent, service or group.
Prefill editable values with the textual expression; keep glyphs in surrounding
labels or read-only views. Saving an ACL preserves its syntax and does not add
display symbols to it. The same rule applies to CLI command arguments and
copyable ACL examples.

The [ACL contract](02-access.md#acl) defines access terms and their implementation status.
The [proposed role syntax](../Plans/R1/identity.md#sigils) remains separately
identified as proposed; this display rule does not introduce new parser syntax.
