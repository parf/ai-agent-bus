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

**Next step**: **F.2 and H.1**, and both are the owner's — nothing else in
this plan is waiting on anything but work. Every decision the plan stood on
has been taken: the caller's deadline travels (A.2), a pool may share an
inbox (A.5), a subscriber is a registered name (C.3), and the sandbox is one
backend plus off (G.2). What is left:

| | |
|---|---|
| **built** | A, B, C, E, F.1, F.3, G, H.2–H.7 |
| **blocked on the owner** | F.2 — nothing on a record carries method information; H.1 — npm install or Go first |
| **unblocked, in progress** | F.4–F.6, the dashboard the review settled ([discovery § what it shows](../../docs/05-discovery.md#what-it-shows)). Most of the work is in the daemon keeping what a view needs, not in the page — and it now keeps what six of the eight views need. My names and people are still unbuilt, and so is every page |
| **struck** | D, to [Plans/V1](../V1/TODO.md) — the MVP's key mode cannot carry the end-to-end claim |

## Blockers

Each is already an indexed open decision
([decisions § open](../../docs/decisions.md#open)) — listed here only because a
wave stands on it.

| Gates | ❓ | Where it is settled |
|---|---|---|
| the whole stage | what MVP contains | [stages § MVP](../../docs/12-stages.md#mvp) |
| the rest of E | what else lives in SQLite | [setup § storage](../../docs/09-setup.md#storage) |
| H.1 | npm install vs Go-first, and how a Go binary is installed by npm | [setup § install](../../docs/09-setup.md#install) |
| F.2 | what carries a service's method information | [services § service and template](../../docs/03-services-and-topics.md#service-and-template) |
| F.4–F.6 | where the ACL and the user-to-account map are edited — the dashboard shows them and `agent-bus-admin` writes them | [setup § the five programs](../../docs/09-setup.md#the-five-programs) |
| the CLI | whether reading an inbox and filtering one become separate options | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| G | what happens to a running service when its configuration changes | [services § configuring a template](../../docs/03-services-and-topics.md#configuring-a-template) |

Five were raised by this plan; four are settled and the fifth went to V1
with wave D:

| Raised for | ❓ | Settled as |
|---|---|---|
| A.2 | whether the caller's deadline travels | it does, and it is not a TTL — [messaging § request and reply](../../docs/04-messaging.md#request-and-reply) |
| A.5 | whether several readers may block on one inbox | when each asks to share it — [messaging § several readers may wait](../../docs/04-messaging.md#several-readers-may-wait-when-they-say-so) |
| C.3 | what a subscriber is, and where a copy goes | a registered name, and its own inbox — [messaging § subscribers](../../docs/04-messaging.md#subscribers) |
| G.2 | how many sandbox backends | one, plus off — [runner § sandboxing](../../docs/08-runner-role.md#sandboxing) |
| D.1, D.2 | how a queued body is decrypted by a receiver that was not present | **still open**, and now [Plans/V1](../V1/TODO.md)'s — [access § encrypted sessions](../../docs/02-access.md#encrypted-sessions) |

⚠️ **The owner check is a caller-name guard, not security.** B.7 closed
re-registering somebody else's record and B.8 closed claiming a name in a realm
a directory vouches for. What stays open is a realm nobody vouches for: there
ownership is first-come, and a claimed name's credential follows
([access § getting a token](../../docs/02-access.md#getting-a-token)). That is
a property of the host, not a gap in the wave — a local realm is as open as the
accounts on the machine.

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
| A.2 | ✅ _done_ — a request carries how long its caller will wait, the bus turns that into a moment, and a service that sees it already past does not do the work ([messaging § request and reply](../../docs/04-messaging.md#request-and-reply)). It is a second pair beside the TTL and not a spelling of it: the queue bounds a TTL and not this, and the bus carries this and acts on the TTL |
| A.3 | ✅ _done_ — a record carries `--ttl` and `--bound`, a message carries its own `--ttl`, and the record's bounds it: asking for longer than the queue keeps does not get it, and a shorter one on the message still wins ([services § topics](../../docs/03-services-and-topics.md#topics)). Neither is a daemon-wide constant any more |
| A.4 | ✅ _done_ — `reply-to`, refused at accept when the address is not registered; the CLI, runner and face all read the route off the envelope ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)) |
| A.5 | ✅ _done_ — a reader that asks to **share** the inbox is one of a pool, and any number of those may block on an empty one; a reader that does not ask keeps the old refusal in both directions ([messaging § several readers may wait](../../docs/04-messaging.md#several-readers-may-wait-when-they-say-so)). Asking is how the daemon tells a pool from a session's accidental second reader, and it remembers nothing between reads |
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
  criterion passed on PoC code, so nothing is sent until all of them are
  provably blocked. And **one side asking is not enough**: a reader that wants
  the inbox to itself is still refused a pool, and refused *by* one;
- a service is handed work whose caller has gone and **does not run it**,
  with a live request beside it as the control and neither carrying a queue
  TTL — so the deadline, and not expiry, is what made the difference;
- `ls` shows a service's **own** call count: a known increment on one service
  and an unchanged control beside it, so a constant zero or a daemon-wide
  total copied onto every record fails.

**Cut costs**: A.2 and A.4's third-party routing are the droppable pair. The
accept-time reply check is not: it is the difference between a refusal and a
message that bounces later.

### B — it is many people's

| ID | Task | Notes |
|---|---|---|
| B.1 | ✅ _done_ — a token backs one principal and the daemon refuses a request that states another, saying whose token it is ([access § two parameters](../../docs/02-access.md#two-parameters)). The socket half of that ⚠️ is B.2's |
| B.2 | ✅ _done_ — one socket per mapped account, supplying both parameters; a name stated on somebody else's socket is refused ([access § local socket](../../docs/02-access.md#local-socket)). It takes the declared-violation route: the daemon chowns its own sockets and says so when it cannot, and G.1 retires that ([processes § why the supervisor holds CAP_CHOWN](../../docs/11-processes.md#why-the-supervisor-holds-cap_chown)) |
| B.3 | ✅ _done_ — issued, rotated and durable: `--rotate` demotes the current token to previous, both authenticate, the one before them stops, and a restart keeps the pair ([access § token lifetime](../../docs/02-access.md#token-lifetime)) |
| B.4 | ✅ _done_ — credentials sit behind the `store` port with a text-file adapter, and the layer rule is checked both ways: nothing inward names an adapter, and the process that assembles them does ([modules § the rule](../../docs/10-modules.md#the-rule)). What *else* the store holds is still the blocker ([setup § storage](../../docs/09-setup.md#storage)) |
| B.5 | ✅ _done_ — the principal is an argument of the forced command, so the key a person holds picks the line and they cannot ask for another ([access § getting a token](../../docs/02-access.md#getting-a-token)) |
| B.6 | ✅ _done_ — both clients carry a name **and its own token**; the runner swaps both when it becomes the service, and the MCP face can mint one for a name it may have |
| B.7 | ✅ _done_ — re-registering somebody else's record is refused, and told whose it is; the record itself may still refresh its own ([identity § ownership](../../docs/01-identity.md#ownership)). Claiming an unheld name stays open — that half is B.8's |
| B.8 | ✅ _done_ — the `directory` port fetches and nothing else; the proof is a step of its own, verified with the host's `ssh-keygen` ([identity § proving possession](../../docs/01-identity.md#proving-possession)). A vouched realm can only be enrolled into, and the record is its own owner — which is the half of the name-claiming hole that could be closed. Manual (a keys file) and GitHub are two adapters |
| B.9 | ✅ _done_ — every name in the suite has its own credential, minted from the owner on first use; there is no shared token left to bypass with |
| B.10 | ✅ _settled_ — `token` is the credential verb, `register` the registry one ([access § getting a token](../../docs/02-access.md#getting-a-token)); B.1 and B.5 build it |

**Done when**, and what breaking it must do:

- a request on Alice's socket that *claims* to be Bob is refused, and so is a
  remote one whose token belongs to someone else — removing the binding turns
  both red. The face overwriting `from` inside one request does not pass this:
  the forgery to catch is a whole request made under the wrong name. "A token
  survives a restart" is **not** a check either — the PoC already passes it,
  though *every principal* surviving one is new and is checked.
  ⚠️ The remote half is done; the socket half waits on B.2;
- issue, rotate, restart: the current and the previous token both work, the one
  before that is refused;
- Alice cannot re-register Bob's name, and cannot change its address while it
  still reads as Bob's — and a service refreshing its **own** record still
  works, or every service breaks on its second start;
- an enrolment with a key the account does not hold is refused, and an enrolled
  principal keeps working with the provider unreachable;
- `smoke.sh` runs with two principals and its own sockets, and a mutation that
  makes any route accept the wrong principal turns a named check red;
- nothing inward names an adapter, **and** the process that assembles them
  does — a rule that only forbids is satisfied by an empty seam, so the
  positive half is part of the criterion.

The chown itself cannot be exercised unprivileged; that one check is a
privileged run, declared as such. What **is** checked unprivileged is
everything the chown is for: two accounts, two sockets, two principals,
neither able to speak as the other — and that a chown which did not land is
said out loud instead of leaving a socket that looks like somebody else's.

**Cut costs**: GitHub (B.8) is the only droppable line. Without B.1–B.7 and B.9
the stage has no meaning.

### C — who may reach what

| ID | Task | Notes |
|---|---|---|
| C.1 | ✅ _done_ — `allow` is a field on the record, so the daemon holds what it enforces and never reads a private configuration ([identity § acl](../../docs/01-identity.md#acl)). Seeing and using are one question: a listing hides what a lookup denies |
| C.2 | ✅ _done_ — the daemon's owner holds master without being listed, `--master` adds others, and `--no-master` takes the last word back. Master refused everywhere would pass a weaker check and is falsified on its own |
| C.4 | ✅ _done_ — send, publish and register are each refused for a principal the service will not show, each check falsified alone, and the refusal is its own status |
| C.3 | ✅ _done_ — a subscriber is a **registered name** and a publication is one copy into each subscriber's own inbox, so the topic keeps nothing and each copy obeys the subscriber's own bound, overflow and TTL ([messaging § subscribers](../../docs/04-messaging.md#subscribers)). The subscription is a record on the topic, so it survives a restart and a restatement; the ACL is asked again at **every publish**, and one subscriber that will not read cannot stop the topic |

**Done when**, and what breaking it must do:

- with the request and the subscription held constant and **only the policy
  changed**: a principal the master ACL grants is refused by a service that
  refuses master, allowed by one that does not, and the refusal is its own
  status — not a 404 and not a 204. Rejecting every master request passes a
  weaker check and must not pass this one;
- a subscriber holding a capability for one topic receives it and not another,
  and removing the authorization check — not the filter — turns it red. The
  check is asked **again at every publish**, so taking a subscriber off the
  allow list stops its copies without touching its subscription — a check
  only at subscribe passes the first half and none of the second;
- a publication reaches **both** subscribers, which a fan-out that hands the
  message to whichever reads first does not, and the topic's own queue stays
  empty — that is the whole difference from a queue topic;
- a principal a service will not show is also refused when it **sends** to
  that service, publishes to its topic, or registers over its name — one check
  per write verb, each falsified on its own, because a single shared guard
  passes the whole set while any one path is still open.

**Cut costs**: none. This is the *safely* in the purpose, and pub/sub has been
waiting on it since the PoC.

### D — the bus stops reading payloads

🚫 **Cut from this stage.** The sentence is not reachable in the MVP's key
mode: the daemon issues the token and holds it on the local socket
([access § key modes](../../docs/02-access.md#key-modes),
[access § encrypted sessions](../../docs/02-access.md#encrypted-sessions)), so
AEAD over it would pass every check but the one that matters. The docs now say
the bus is trusted on its own host and end to end is Release 1's, on the
pairwise or derived keys that make it true
([stages § release 1](../../docs/12-stages.md#release-1)). Nothing below is
built, and the rows stay as the shape Release 1 inherits.

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
| E.1 | ✅ _done_ — the snapshot is written at start, on a graceful stop and, if asked, once a minute; a start that follows an unclean stop says so and from when ([messaging § durability](../../docs/04-messaging.md#durability)). JSON behind the `dump` port; Parquet is an adapter, not the wave |
| E.2 | ✅ _done_ — records, backlog and counters come back, a drained queue stays drained, and a message past its moment is not delivered late. The half about decrypting went with D |
| E.3 | ✅ _done_ — the per-inbox `in`/`out` counters and the daemon's `dropped`/`expired` survive a restart; uptime does not, because it is this run's |

**Done when**, and what breaking it must do:

- stop, start, **drain**, stop, start: nothing is delivered twice, so a stale
  snapshot that resurrects consumed messages turns it red;
- a message that expires while the daemon is down is never delivered —
  delivery is what decides that, not the reload, so the check earns its place
  by guarding what the snapshot writes down rather than a second expiry rule;
- stats survive, and deleting E.3 turns a named check red;
- after an ungraceful kill, the **next start says** what it lost, at a named
  place — "says so" with no reporter is satisfied by silence.

**Cut costs**: droppable if users accept that a restart empties the queues.
That is a product decision, not a technical one.

### F — the faces grow up

| ID | Task | Notes |
|---|---|---|
| F.1 | ✅ _done_ — the daemon filters, so `ab_ls` and `/ls` answer two principals differently, and each answer matches what that principal may actually send to. The face adds nothing: it asks, like every other client |
| F.2 | generated docs | **blocked**: nothing on a record carries method information, and the shape is the ❓ in [services § service and template](../../docs/03-services-and-topics.md#service-and-template) |
| F.4 | the dashboard's rules | the anonymous page answers what the bus would answer a caller it cannot name; no page renders a credential; no JavaScript, CDN or external asset; the child writes nothing of its own ([discovery § rules it is built to](../../docs/05-discovery.md#rules-it-is-built-to)) |
| F.5 | signing in | the two parameters the caller already has, a session the **bus** holds, a browser carrying only its id — and the child giving up the owner's socket, because a web child with the owner's authority is a credential mint ([discovery § signing in](../../docs/05-discovery.md#signing-in)) |
| F.6 | the views the MVP owes | registry, stuck inboxes, exchanges, my names, loss by name, refusals, node, people. Each row in [discovery § what it shows](../../docs/05-discovery.md#what-it-shows) names what the daemon must start keeping — that is the work, not the page. ⚠️ *the daemon now keeps what six of them need*: registry was already `/ls`, and stuck inboxes, loss by name, node and refusals are fed by `oldest`, per-inbox `dropped`/`expired`, the unclean-restart fact ([discovery § what a listing answers](../../docs/05-discovery.md#what-a-listing-answers)) a count per reason ([discovery § refusals](../../docs/05-discovery.md#refusals)) and a feed each caller may read ([discovery § dashboard](../../docs/05-discovery.md#dashboard)). My names and people are still unbuilt, and so is every page |
| F.3 | ✅ _done_ — `agent-bus-web`, a separate process speaking the API, rendering the records and a bounded feed of routed envelopes ([discovery § dashboard](../../docs/05-discovery.md#dashboard)). Bodies are struck out in the bus, not in the page. It serves HTTPS on its own hostname with a certificate anyone can fetch ([discovery § where it listens](../../docs/05-discovery.md#where-it-listens)). Nothing starts it yet and it is not cgroup-limited — both are G.1's |

**Done when**, and what breaking it must do:

- two principals ask the MCP face what they can use and get **different**
  answers, each matching what they may actually call;
- a tool's generated documentation changes when the record it is generated
  from changes — deleting F.2 must turn a check red, which "the catalogs
  differ" does not;
- the dashboard renders a live envelope and no body;
- an anonymous visitor gets the form and nothing else — putting back the
  uptime, the service count or an operator's name turns a named check red,
  because each of them is what the check is for;
- **a page never carries a token**: what is on the screen is a fingerprint,
  and offering that fingerprint as a credential is refused;
- a web child **restarted mid-session logs nobody out** — a session map
  inside the child passes every other check and fails this one, which is why
  it is the criterion;
- a **stuck** inbox is listed ahead of a healthy one, with a backlog that
  *has* a reader beside it as the control — sorting by depth alone passes
  without the reader half;
- two principals open the same URL and get different pages, each matching
  what they may actually call.

**Cut costs**: F.6's views drop one at a time, and stuck inboxes is the one
worth keeping — it is the view an incident needs. F.4 does not drop: its rules
are what keep the dashboard from being the node's weakest door. F.5 does not
drop either once F.6 exists, because a view with no signed-in caller is the
owner's view served to a stranger, which is what the dashboard does today.

### G — least privilege

| ID | Task | Notes |
|---|---|---|
| G.1 | ✅ _done_ — one binary, two roles: the supervisor opens every listener, chowns the per-user ones and hands the fds down; the bus serves them and holds the store ([processes § how a child is started](../../docs/11-processes.md#how-a-child-is-started)). A dead child is restarted onto the same sockets — the socket file is the same inode after a restart and a different one when it is genuinely remade, which is how that check was falsified, no mutation being able to express it — and a supervisor killed outright takes its children with it. The dashboard is a child under `-web`. ⚠️ *runner, auth and health are still design, and a child's empty capability set can only be read on a host that has one to lose* |
| G.2 | the runner sandboxes ([runner § sandboxing](../../docs/08-runner-role.md#sandboxing)) | the cut is taken: **one backend and off**. `systemd-run --user`, behind a `sandbox` port so a second backend is one adapter and not a redesign |
| G.3 | `stop` and `logs` ([runner § stopping it](../../docs/08-runner-role.md#stopping-it-and-reading-what-it-said)) | a running service leaves a note in its owner's own state directory, which is what makes "only whoever started it may stop it" true with no check in it |

**Done when**, and what breaking it must do:

- the bus process's effective capabilities are empty and the supervisor's hold
  exactly the one it needs — read from the running processes, not from the
  unit file. ⚠️ *only observable under the unit: the suite runs as an account
  with no capability at all, so both sets read as empty either way*;
- a sandboxed child that tries to write outside its work directory, or to open
  a network socket, **fails** — and writing *inside* it succeeds beside it, or
  a child that cannot run at all passes the whole set. Loosening either
  property turns exactly one of them red;
- the service **says which sandbox it got**, so running unconfined is never a
  silence, and asking for one where the host cannot is an error rather than a
  quiet downgrade;
- `stop` ends one service and **leaves its siblings running**, waits for it
  rather than reporting a stop that has not happened, and does **not**
  unregister the name — messages sent afterwards still wait in its queue;
- `logs` shows what a service wrote **after it has stopped**, which is when it
  is wanted; and a second `start` of a name already running here is refused.

⚠️ *"A killed child comes back" belongs to G.1 and is met there*: the
supervisor restarts the bus onto the same sockets. A script service has no
long-lived child to restart — its scripts are one process per message — so
restart-with-backoff for user children stays the runner proper's, and is not
claimed here.

**Cut costs**: the largest single cut available. A one-process MVP is
defensible if the ACL is real; the split is what keeps a compromise of one
role from being a compromise of the host.

### H — somebody else installs it

⚠️ H.1 is **blocked** on the packaging ❓ above. The rest do not depend on how
the binaries arrive, and are built
([setup § the five programs](../../docs/09-setup.md#the-five-programs)).

| ID | Task |
|---|---|
| H.1 | the package ([setup § install](../../docs/09-setup.md#install)) — **still blocked** on the packaging ❓ |
| H.2 | ✅ _done_, and now H.4's program — it creates the account, writes the unit and starts the daemon as it, refusing without root rather than half-installing; `--dry-run` / `--print-unit` need nothing ([setup § the two accounts](../../docs/09-setup.md#the-two-accounts)). Unprivileged checks cover everything it would write; **running it for real is the privileged check below** |
| H.3 | ✅ _done_ — the account, its home, the one declarative capability and restart, all in the unit and each falsified on its own |
| H.4 | ✅ _done_ — `agent-bus-setup` is its own program, root-only, and prints the `sudo` line rather than failing flat ([setup § the five programs](../../docs/09-setup.md#the-five-programs)) |
| H.5 | ✅ _done_ — `agent-bus-admin` writes the account's `authorized_keys`: `user add` / `list` / `remove`, each line a forced command and no shell, an operator's pointing at this program and everybody else's at `agent-bus-token` ([AUTH role § SSH admin](../../docs/06-auth-role.md#ssh-admin)). It re-runs itself under `sudo -u agent-bus` when it is not that account, and hands the shared `token` verb to the smaller program rather than owning a second copy. ⚠️ *ACL editing waits on where an ACL is written down — the ❓ in [setup § the five programs](../../docs/09-setup.md#the-five-programs)* |
| H.6 | ✅ _done_ — `agent-bus-token` prints a credential and nothing else. Under SSH the authorized_keys line is the entitlement and `$SSH_ORIGINAL_COMMAND` is the request, so a key may only ask for the name its line names. The CLI's `token` verb went with it |
| H.7 | ✅ _done_ — `--key` proves the name against what the realm publishes and gets a credential with no credential to start from ([access § getting a token](../../docs/02-access.md#getting-a-token)). It is enrolment's own challenge with a second caller; the one change the daemon needed was to stop asking `/enrol` for a credential, which was a circle ([identity § proving possession](../../docs/01-identity.md#proving-possession)) |

**Done when**, and what breaking it must do:

- a person who has not read this repo installs it on a fresh host and calls a
  service, following only the generated instructions — **checked by hand,
  once**, like the PoC's live criterion;
- each of the five programs refuses the privilege it is not for, and says
  which line to run instead — a root-only installer run as a user, an admin
  run as somebody else;
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
