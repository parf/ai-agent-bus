# TODO — MVP

The active plan. Stable knowledge for this stage is in [README.md](README.md),
the design is [docs/](../../docs/00-overview.md), and open questions and
settled decisions live in [decisions](../../docs/decisions.md). This file holds
only what is being built now.

**Objective**: the MVP as scoped in [stages § MVP](../../docs/12-stages.md#mvp) —
somebody other than the author installs it and uses it safely on a shared host.

**V1 is not the bar here.** It was, for the PoC, where writing a push adapter
from scratch had prior art worth measuring against. This stage is our own
design carried out; V1 is legacy and is not consulted for it. What does still
hold is that nothing is believed until it has been watched failing
([PoC README § mutation first, then belief](../PoC/README.md#mutation-first-then-belief)).

**Next step**: wave B. A.1, A.3, A.4 and A.6 are done; A.2 and A.5 wait on
two ❓ below that only the owner closes, and B has a blocker of its own —
which process owns the store handle — that B.4 and B.2 run into.

## Blockers

Each is already an indexed open decision
([decisions § open](../../docs/decisions.md#open)) — listed here only because a
wave stands on it.

| Gates | ❓ | Where it is settled |
|---|---|---|
| the whole stage | what MVP contains | [stages § MVP](../../docs/12-stages.md#mvp) |
| C, F.1 | who filters, with AUTH off | [discovery § audience](../../docs/05-discovery.md#audience) |
| D | static sessions are not end-to-end against the daemon — so D's own claim may be unreachable in the MVP's key mode | [access § encrypted sessions](../../docs/02-access.md#encrypted-sessions) |
| B.4, E | what else lives in SQLite | [setup § storage](../../docs/09-setup.md#storage) |
| H | npm install vs Go-first, and how a Go binary is installed by npm | [setup § install](../../docs/09-setup.md#install) |
| B, G | which process owns the store handle | [processes § what is shared](../../docs/11-processes.md#what-is-shared) |
| A, the CLI | whether reading an inbox and filtering one become separate options | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| G | what happens to a running service when its configuration changes | [services § configuring a template](../../docs/03-services-and-topics.md#configuring-a-template) |

Five more were raised by this plan and are now indexed with the rest:

| Gates | ❓ | Where it is settled |
|---|---|---|
| A.2 | whether the caller's deadline travels with the request | [messaging § request and reply](../../docs/04-messaging.md#request-and-reply) |
| A.5 | whether several readers may block on one inbox at once | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| C.3 | what a subscriber is, and where a fan-out copy goes | [messaging § push and pull](../../docs/04-messaging.md#push-and-pull) |
| D.1, D.2 | how a queued body is decrypted by a receiver that was not present when it was sent | [access § encrypted sessions](../../docs/02-access.md#encrypted-sessions) |
| F.2 | what carries a service's method information | [services § service and template](../../docs/03-services-and-topics.md#service-and-template) |

⚠️ **The owner check is a caller-name guard, not security** — and registering
over a name nobody owns is not checked at all
([identity § ownership](../../docs/01-identity.md#ownership)). Wave B owns
both, and the ⚠️ in
[services § configuring a template](../../docs/03-services-and-topics.md#configuring-a-template)
is B's to retire.

## Waves

One wave, one deliverable; review and commit at the end of each. The order is
dependency, not importance. Every **Done when** names the mutation that must
turn it red, because two of this plan's first-draft criteria passed on PoC code
before any MVP work existed.

### A — calls grow up

Nothing new is needed underneath, so it goes first and exercises the core we
already trust. **What the PoC already does is not work**: the caller's deadline
already survives receipts, and the daemon already accepts a `done` receipt.

| ID | Task | Notes |
|---|---|---|
| A.1 | ✅ _done_ — a `done` verb, an `ab_receipt` tool, the runner emitting one on silent success, and a caller's wait ending on it ([messaging § receipts](../../docs/04-messaging.md#receipts)) |
| A.2 | the deadline travels to the service | **blocked**: whether it does is the ❓ in [messaging § request and reply](../../docs/04-messaging.md#request-and-reply) |
| A.3 | TTL per message, bounded by the topic's | needs the topic to carry a TTL and a bound at all ([services § topics](../../docs/03-services-and-topics.md#topics)); the queue bound is one constant today |
| A.4 | ✅ _done_ — `reply-to`, refused at accept when the address is not registered; the CLI, runner and face all read the route off the envelope ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)) |
| A.5 | several workers behind one name | competing consumers already work — 500 messages to 8 readers, none twice, none lost. What is missing is N readers **blocking** on an empty inbox; **blocked** on the ❓ above |
| A.6 | ✅ _done_ — `in` and `out` per name, counted in memory, attached to a listing and cleared off anything a caller states ([discovery § what a listing answers](../../docs/05-discovery.md#what-a-listing-answers)) |

**Done when**, and what breaking it must do:

- a sender that asked for `done` gets one, and a face that stops sending it
  turns the check red — the PoC daemon accepts the value, so a check that only
  posts `receipt: done` passes today;
- a request whose `reply-to` names an unregistered address is **refused at
  accept**, while a plain `send` from an unregistered sender still succeeds;
  reverting the check turns the first red without turning the second red;
- a message expires and is counted in a **TTL-specific** counter with no ring
  overflow in play — sharing the ring's counter would let a ring drop pass it;
- a topic whose TTL is shorter than a message's bounds the message;
- N workers idle on an **empty** inbox all receive work as it arrives — a
  prefilled queue passes without that, which is how the first draft of this
  criterion passed on PoC code;
- `ls` shows a service's **own** call count: a known increment on one service
  and an unchanged control beside it, so a constant zero or a daemon-wide
  total copied onto every record fails.

**Cut costs**: A.2 and A.4's third-party routing are the droppable pair. The
accept-time reply check is not: it is the difference between a refusal and a
message that bounces later.

### B — it is many people's

| ID | Task | Notes |
|---|---|---|
| B.1 | principals ([identity § names](../../docs/01-identity.md#names)) | the name is the identity, and the daemon checks it against the credential rather than taking it ([access § two parameters](../../docs/02-access.md#two-parameters)) — the ⚠️ there is B's to retire |
| B.2 | per-user sockets ([access § local socket](../../docs/02-access.md#local-socket)) | needs the capability that only the supervisor may hold ([processes § why the supervisor holds CAP_CHOWN](../../docs/11-processes.md#why-the-supervisor-holds-cap_chown)) — so B.2 pulls the minimal supervisor-and-fd-handoff out of G.1, or declares a temporary violation it must retire |
| B.3 | tokens issued, rotated and durable ([access § token lifetime](../../docs/02-access.md#token-lifetime)) | *previous kept* has nothing to keep until something issues a second one |
| B.4 | the store behind a port ([modules § the rule](../../docs/10-modules.md#the-rule)) | core never imports it; **what goes in it is a blocker**, and tokens are the part the design already requires durable ([setup § storage](../../docs/09-setup.md#storage)) |
| B.5 | `src/static-token` maps an SSH key to a principal | it hands out one shared token today, and its forced command selects no one |
| B.6 | both clients carry a principal | the Go CLI and `src/mcp/` send credentials explicitly, over the socket too; per-user sockets do not reach them for free ([modules § languages](../../docs/10-modules.md#languages)) |
| B.7 | the owner check becomes a check | registration over someone else's name is refused ([identity § ownership](../../docs/01-identity.md#ownership)); the ⚠️ in [services § configuring a template](../../docs/03-services-and-topics.md#configuring-a-template) goes in the same commit |
| B.8 | enrolment: manual, then GitHub with possession proved | fetching a public key is not authentication ([identity § registration](../../docs/01-identity.md#registration)). Where the proof step sits relative to the `directory` port ([modules § modules](../../docs/10-modules.md#modules)) needs an owning edit before it is built — it is more than fetching |
| B.9 | `smoke.sh` gets two principals | one token for every participant is the bypass this wave removes |
| B.10 | `register` means one thing | the verb issues a token in [access § getting a token](../../docs/02-access.md#getting-a-token) and states a record in [services](../../docs/03-services-and-topics.md#service-and-template) |

**Done when**, and what breaking it must do:

- a request on Alice's socket that *claims* to be Bob is refused, and so is a
  remote one whose token belongs to someone else — removing the binding turns
  both red. The face overwriting `from` inside one request does not pass this:
  the forgery to catch is a whole request made under the wrong name. "A token
  survives a restart" is **not** a check either — the PoC already passes it;
- issue, rotate, restart: the current and the previous token both work, the one
  before that is refused;
- Alice cannot re-register Bob's name, and cannot change its address while it
  still reads as Bob's — this fails today, verified;
- an enrolment with a key the account does not hold is refused, and an enrolled
  principal keeps working with the provider unreachable;
- `smoke.sh` runs with two principals and its own sockets, and a mutation that
  makes any route accept the wrong principal turns a named check red.

The chown itself cannot be exercised unprivileged; that one check is a
privileged run, declared as such.

**Cut costs**: GitHub (B.8) is the only droppable line. Without B.1–B.7 and B.9
the stage has no meaning.

### C — who may reach what

| ID | Task | Notes |
|---|---|---|
| C.1 | service ACL, consulted first ([identity § acl](../../docs/01-identity.md#acl)) | **blocked**: where the daemon reads it from is the filtering ❓ — it cannot be the opaque configuration it is forbidden to read |
| C.2 | master ACL, and a service may refuse it | the refusal is the point; a master that cannot be refused is not an ACL |
| C.4 | every write goes through the layers | a send, a publish and a registration are checked like a read ([identity § acl](../../docs/01-identity.md#acl)) |
| C.3 | pub/sub topics | a subscription is a capability that only exists once there is an ACL; **blocked** on the ❓ in [messaging § push and pull](../../docs/04-messaging.md#push-and-pull) |

**Done when**, and what breaking it must do:

- with the request and the subscription held constant and **only the policy
  changed**: a principal the master ACL grants is refused by a service that
  refuses master, allowed by one that does not, and the refusal is its own
  status — not a 404 and not a 204. Rejecting every master request passes a
  weaker check and must not pass this one;
- a subscriber holding a capability for one topic receives it and not another,
  and removing the authorization check — not the filter — turns it red;
- a principal a service will not show is also refused when it **sends** to
  that service, publishes to its topic, or registers over its name — one check
  per write verb, each falsified on its own, because a single shared guard
  passes the whole set while any one path is still open.

**Cut costs**: none. This is the *safely* in the purpose, and pub/sub has been
waiting on it since the PoC.

### D — the bus stops reading payloads

**Blocked** on whether the sentence is reachable at all: in the MVP's key mode
the daemon issues the token and holds it on the local socket
([access § key modes](../../docs/02-access.md#key-modes),
[access § encrypted sessions](../../docs/02-access.md#encrypted-sessions)).
If it is not reachable, the honest outcome is pairwise keys pulled into B — or
the sentence struck from the docs. Deciding that is cheaper than building
twice.

| ID | Task | Notes |
|---|---|---|
| D.1 | AEAD sessions | a well-known library on the hot path, never our own primitive ([modules § the rule](../../docs/10-modules.md#the-rule)) |
| D.2 | a queued body survives its receiver | the second ❓ above; a live handshake does not fit an inbox that outlives its reader |
| D.3 | the TypeScript side interoperates | `src/mcp/` reimplements the protocol, and an encryption Go and bun disagree about is worse than none |
| D.4 | `encryption: off` stays a development path, never the default | ([access § encrypted sessions](../../docs/02-access.md#encrypted-sessions)) |

**Done when**, and what breaking it must do:

- an exact plaintext round trip between two real endpoints, one Go and one
  bun;
- a tampered body, a wrong key and a replayed message are each **rejected**
  ([access § key confirmation](../../docs/02-access.md#key-confirmation)) —
  opaque-looking bytes pass a weaker check, and so does base64;
- the daemon's own key material **cannot** decrypt a captured body. If that
  check cannot be made to pass, the blocker was real and the docs change
  instead.

**Cut costs**: dropping it leaves the PoC's honesty problem — the design says
the bus cannot read bodies and it can. Acceptable only if that sentence goes
too.

### E — a restart is not a loss

| ID | Task | Notes |
|---|---|---|
| E.1 | dump on graceful stop ([messaging § durability](../../docs/04-messaging.md#durability)) | the format is behind the `dump` port, so it is an adapter, not the wave |
| E.2 | reload, and the backlog still decrypts | which is why this follows D |
| E.3 | stats come back too | A.6's counters are state a restart currently loses |

**Done when**, and what breaking it must do:

- stop, start, **drain**, stop, start: nothing is delivered twice, so a stale
  snapshot that resurrects consumed messages turns it red;
- a message that expires while the daemon is down is never delivered;
- stats survive, and deleting E.3 turns a named check red;
- after an ungraceful kill, the **next start says** what it lost, at a named
  place — "says so" with no reporter is satisfied by silence.

**Cut costs**: droppable if users accept that a restart empties the queues.
That is a product decision, not a technical one.

### F — the faces grow up

| ID | Task | Notes |
|---|---|---|
| F.1 | catalog filtered per caller ([discovery § faces](../../docs/05-discovery.md#faces)) | needs C, and needs the filtering ❓ answered; an unfiltered catalog is an ACL leak |
| F.2 | generated docs | **blocked**: nothing on a record carries method information, and the shape is the ❓ in [services § service and template](../../docs/03-services-and-topics.md#service-and-template) |
| F.3 | a basic dashboard | envelopes only, never bodies; it is a child process in the design ([processes § the processes](../../docs/11-processes.md#the-processes)), so it speaks the API from the start rather than being rebuilt at G |

**Done when**, and what breaking it must do:

- two principals ask the MCP face what they can use and get **different**
  answers, each matching what they may actually call;
- a tool's generated documentation changes when the record it is generated
  from changes — deleting F.2 must turn a check red, which "the catalogs
  differ" does not;
- the dashboard renders a live envelope and no body.

**Cut costs**: F.3 is cheap to drop, F.2 with it. F.1 is not, once C exists.

### G — least privilege

| ID | Task | Notes |
|---|---|---|
| G.1 | supervisor and children ([processes § the rule](../../docs/11-processes.md#the-rule)) | B.2 may have taken the first slice already |
| G.2 | the runner supervises and sandboxes ([runner § sandboxing](../../docs/08-runner-role.md#sandboxing)) | ⚠️ *proposed cut*: one backend plus off, where the design selects among several. Needs the owner before it is built |
| G.3 | `stop` and `logs` | deferred out of the PoC explicitly *with the runner* ([stages § PoC](../../docs/12-stages.md#poc)) |

**Done when**, and what breaking it must do:

- the bus process's effective capabilities are empty and the supervisor's hold
  exactly the one it needs — read from the running processes, not from the
  unit file;
- a sandboxed child that tries to write outside its work directory, or to open
  a network socket, **fails**; loosening the profile turns it red;
- a killed child comes back, and `stop` ends it without killing its siblings.

**Cut costs**: the largest single cut available. A one-process MVP is
defensible if the ACL is real; the split is what keeps a compromise of one
role from being a compromise of the host.

### H — somebody else installs it

**Blocked** on the packaging ❓ above.

| ID | Task |
|---|---|
| H.1 | the package ([setup § install](../../docs/09-setup.md#install)) |
| H.2 | `agent-bus setup` creates the service account and starts the daemon as it ([setup § the service account](../../docs/09-setup.md#the-service-account)) — never as the invoking user, never as root |
| H.3 | the unit: the account, its home, the one declarative capability, and restart |

**Done when**, and what breaking it must do:

- a person who has not read this repo installs it on a fresh host and calls a
  service, following only the generated instructions — **checked by hand,
  once**, like the PoC's live criterion;
- the running daemon's uid is the service account's and not the installer's,
  its home is where the docs say, and its store and dumps are under that home
  — read from the running process, so starting it by hand as a developer
  fails the check.

**The last one is a stage gate, not just H's**: every wave from B onward is
about two people on one host, and a daemon running as whoever built it is not
that. Waves B–G may develop against a hand-started daemon, but the stage is
not done until the answer comes from the account.

**Cut costs**: none, and it is last only because it packages what comes before.
The stage is named for this line.
