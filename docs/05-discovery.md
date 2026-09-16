# Discovery

## Status

| MVP | Scope |
|---|---|
| Built | Filtered listings and catalog, all [required dashboard tabs](#required-tabs), administration and envelope-only diagnostics. |
| Pending | Installed [browser acceptance](#browser-acceptance), resource confinement and [web authority isolation](11-processes.md#web-authority-boundary). |

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
| **`at_bound`** | its queue is at the limit it is allowed, so the next message is refused or something is lost ([messaging § overflow](04-messaging.md#overflow)) | there is room. The daemon answers it because a record that declares no bound takes the daemon's, and a reader cannot know what that is |

These are **observations attached to the listing**, not values a registrant
may state. Counters survive through snapshots; reader presence does not ([overview § principles](00-overview.md#principles)). They are the difference between
*"this name exists"* and *"a call would reach someone"*, which is the question
a caller is actually asking.

An inbox that was drained and one nobody ever wrote to both read as empty.
`in` and `out` are what tell them apart, and they are per name: a busy bus
does not make a quiet service look busy.

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
| WEB | [Required tabs and controls](#required-tabs), filtered through the caller's API access | Installed browser acceptance, resource confinement and web authority isolation |

## MCP minimum

**Required for MVP:** both [runtime integrations](08-runner-role.md#runtime-integration-delivery)
and their [launchers](08-runner-role.md#smart-launchers) come with the bus MCP
tools configured and callable in the session.

| Capability | Required outcome | Existing implementation |
|---|---|---|
| List the bus | Discover registered agents, services and topics through the caller's [catalogue view](#audience) | `ab_ls` in the [MCP face](../src/mcp/server.ts) |
| Call a service on the bus | Send to a named service and receive its correlated answer or explicit completion under the [request/reply contract](04-messaging.md#request-and-reply); bus acceptance alone is not completion | `ab_send` plus a filtered `ab_consume`, or delivery through the active push adapter; [MCP face](../src/mcp/server.ts) |

These are minimum capabilities, not a restriction on the remaining tools.
The tools and launcher wiring are built; full live-runtime and fresh-host
acceptance remains pending. Both capabilities use the caller's existing
[ACL](01-identity.md#acl); loading the tools grants no additional authority.

## Audience

The daemon filters, because it holds the records and enforces the
[ACL](01-identity.md#acl). A face requests the caller's view and adds no
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
| Registered services | My / all; active / inactive filters; details and [owner controls](01-identity.md#owner-control), with owner, maintainers group, access, reader presence and queue statistics. Administrative availability and serving / offline are distinct observations |
| Users | List and details; add, edit, activate, pause and ban; show owned services, group membership and administrative authority |
| Groups | List and details; create, edit, delete and manage flat membership; basic service and channel access. Include the daemon maintainers group and each record's assigned maintainers group under the [authority rules](01-identity.md#groups-and-maintainers) |
| Activity graphs | Recent traffic, messages dequeued, drops, expirations and refusals; per-service and per-channel filtering. Dequeued messages are not proof of successful execution. Use bounded history and inline SVG; [sampling and retention](#activity-history) are bounded |
| Registered pub/sub channels | List and details for pub/sub and queue topics; create, edit and remove; subscriptions, owner, maintainers group, permissions, TTL, capacity and overflow policy |

[Owner and maintainer authority](01-identity.md#groups-and-maintainers) applies
to every control and to direct API calls. Membership and policy changes must
survive restart. User lifecycle effects are [daemon policy](01-identity.md#user-lifecycle),
not merely labels on the Users page.

Keep the [built views](#what-it-shows), including stuck inboxes, exchanges,
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
also see node refusal totals, including authentication failures. Bodies never
enter this history.

Adding a subscription remains the subscriber's opt-in. Channel owners and
maintainers can remove a subscription; they cannot force another inbox to subscribe.

### Rules it is built to

| Rule | Why |
|---|---|
| **The anonymous page shows what the bus would answer a caller it cannot name: nothing.** A title, the sign-in form, and how to get a token | Nothing on the page can know whether it is exposed — the hostname is public DNS and the bind address is a flag. Uptime is a restart oracle, a service count that moves is a covert channel anyone who can register writes to, and a traffic total is traffic analysis. `GET /healthz`, 200 with an empty body, is the whole public signal |
| **No page ever renders a credential** — a fingerprint of it, when it was issued, when it was last used, and the command that rotates it | A token on a page is in the browser cache, the scrollback and every screenshot, and leaves no trace that it was read, so "was this leaked?" stops being answerable. A fingerprint is enough to match the one in your environment |
| **No JavaScript, no CDN, no external asset** | A signed-in master is looking at the node's whole envelope feed, and the first `<script src=…>` added for a chart inherits that. Graphs are inline SVG or nothing; an avatar is served from this node, never hotlinked, or every page view tells the provider who is looking |
| **The web child writes nothing of its own.** A form posts *as the person*, never as the child | It is the least trusted process and the design gives it no write path ([processes § the processes](11-processes.md#the-processes)). Built administration forms forward the visitor's session to daemon-enforced operations and require an exact matching Origin. Responses are not cached; credentials and existing private configuration are never populated into forms |
| **The sign-in form takes a token and nothing else** | A call carries no name to get wrong ([access § what a call carries](02-access.md#what-a-call-carries)), so there is no second failure message for an anonymous visitor to read as an oracle for which names exist |

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
| enrolling | not here. The proof is a signature made by the host's own `ssh-keygen` ([identity § proving possession](01-identity.md#proving-possession)) and a page with no JavaScript cannot make one, so what the dashboard does for a stranger is print the command |


A bus restart invalidates browser sessions: their map is not persisted.

### What it shows

| View | MVP status | Source or remaining dependency |
|---|---|---|
| the sign-in page and the token help | Built | — |
| **registry**, as this caller may see it: kind, owner, protocol, description, `reading`/`queued`/`in`/`out`, when the record was last written, the configuration's digest | Built | — it is `/ls` |
| **stuck inboxes** — a backlog with nobody reading, oldest first, marked when the queue is at its bound. The one view an incident actually needs | Built | — `oldest` and `reading` on the record ([what a listing answers](#what-a-listing-answers)) |
| **exchanges** — the envelope feed grouped by topic and tag, so a request, its `ack`, its reply and its `done` are one row, and an answer past its deadline is marked late | Built | — the feed is filtered per caller ([dashboard](#dashboard)); grouping and the late mark are the page's |
| **my names** — what I hold a credential for, whose it is and what it is for, its fingerprint, when it was issued and last used, and how to rotate it | Built | — the caller asks for its own, and gets a fingerprint rather than the token ([token lifetime](02-access.md#token-lifetime)). A person's own identity is distinguished from the services they registered. A credential [goes with its address](01-identity.md#unregistering), so the list stays names something answers on |
| **loss by name** — what each inbox dropped to overflow and what expired in it | Built | — `dropped` and `expired` on the record ([what a listing answers](#what-a-listing-answers)) |
| **refusals** — how many calls were refused and why: bad credential, ACL, unknown receiver, second reader, full queue | Built | The built page shows only refusal reasons that occurred; counters are on `status` ([refusals](#refusals)) |
| **node** — its name, uptime, the registry's totals, and whether the last stop was clean | Built | — `status` carries the unclean-restart fact |
| **people** — identities, profiles, local avatars, authority, state, group membership and owned services | Built | [person records](01-identity.md#person-records) and [user lifecycle](01-identity.md#user-lifecycle) |

### Refusals

A bus that is quiet and one that is refusing every call look identical from
outside. `status` carries **how many calls were turned away and for what**,
counted where a refusal becomes a status code so that the reason and the code
cannot drift apart.

| Reason | | |
|---|---|---|
| `credential` | `401` | the token is not one, or none came at all — the only thing a call carries ([access § what a call carries](02-access.md#what-a-call-carries)) |
| `acl` | `403` | the service, the record's owner, or a private configuration said no ([identity § acl](01-identity.md#acl)) |
| `suspended` | `403` | a user state is in the way: the caller's own, or that of the owner of the name being called ([user lifecycle](01-identity.md#user-lifecycle), [services of a paused or banned user](01-identity.md#services-of-a-user-who-is-paused-or-banned)) |
| `enrolment` | `403` | a challenge that did not hold ([identity § proving possession](01-identity.md#proving-possession)) |
| `unknown` | `404` | no such name ([messaging § verbs](04-messaging.md#verbs)) |
| `disabled` | `409` | the receiver's record is turned off by its owner ([owner control](01-identity.md#owner-control)) |
| `busy` | `409` | an address could not be removed because its inbox has queued messages or a waiting reader ([unregistering](01-identity.md#unregistering)) |
| `second-reader` | `409` | an inbox has an incompatible outstanding reader; sharing requires both readers to ask ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox)) |
| `name-taken` | `412` | a registration that asked for an unheld name found it held ([registration](01-identity.md#registration)) |
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
