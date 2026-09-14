# Bundled services

What ships in the box beyond the daemon itself: a catalogue, and the rules
every entry in it obeys. Nothing here is a feature of `agent-busd` — each one
is an ordinary service with a name, an inbox and an ACL the daemon enforces for
it, written the way this design tells anyone else to write one.

Proposed for **R1.1** ([stages § R1.1](README.md#scope)),
after the runner exists to keep them.

## Rules they all obey

The rules matter more than the list, because the list will grow and they
will not.

| | |
|---|---|
| **no daemon change, ever** | if a bundled service needs the bus to learn something, that is the signal to stop and design rather than to build. The catalogue is a test of the design as much as a set of tools |
| **danger is per name, because access is** | the ACL is attached to a name ([identity § acl](../../docs/01-identity.md#acl)), so anything you would grant to different people must **be** a different name. `shell@h` and `root-shell@h`; `billing-ro@db1` and `billing-rw@db1`. Never one service with a privilege flag on the call — a flag cannot be granted, only a name can. **Spend divides the same way**: generation and embeddings are two names because a team may be allowed one and not the other |
| **none of them enforces access** | the ACL is on the record and **`agent-busd` applies it before a call is delivered** ([identity § acl](../../docs/01-identity.md#acl)). A bundled service that checks who is calling is writing a second, weaker copy of something already done — and this is the single biggest reason most of these are a hundred lines rather than a project |
| **inbound is a publisher, outbound is a call** | every *send/read* pair is two shapes, not one service with two verbs: reading Slack **publishes into a topic** and has no caller ([messaging § push and pull](../../docs/04-messaging.md#push-and-pull)); sending is request/reply. One pattern for Slack, Telegram, SMS, mail and webhooks |
| **secrets are the instance's** | a bot token or a database password is an `env` file under `runner/`, never in the checkout ([runner § the three env layers](../R1/runner.md#the-three-env-layers)). `service.d` is world-readable by construction |
| **one template, many instances** | `slack/team-a@pool1`, `mysql/billing@db1` — configured copies of one thing, which the naming already carries ([identity § names](../../docs/01-identity.md#names)) |
| **the form is part of the design** | a picture service is `--algo=std`, a database gateway is `--algo=msgpack` (binary values, the envelope in-band, and a kept process is what holds the connection), a tail is `--algo=jsonl`, a notification is `--algo=args` ([runner § script services](../../docs/08-runner-role.md#script-services)) |
| **confinement is not what makes the risky ones safe** | a shell service's whole job is to run what it is told, so sandboxing it confines the thing you asked for ([runner § sandboxing](../../docs/08-runner-role.md#sandboxing)). What contains it is the ACL and the account it runs as — nothing else pretends otherwise |
| **no layer of ours between a caller and the tool** | no retry policy, no cache, no request rewriting, no validation of what is being asked. Each of those is the caller's decision, and one made silently inside a gateway is one nobody can debug from outside. If a retry or a cache is wanted it is **a service with a name of its own**, which is this catalogue's rule for everything else too |
| **stateless ones are the pool cases** | scaling a picture or fetching a page is work anybody can do, so it pools across hosts ([runner § one name on many hosts](../R1/runner.md#one-name-on-many-hosts)). Anything about *this box* is per host by definition: `shell@srv1` means that machine and nothing else |

## One contract for the set

These are many tools and should feel like one set. What makes them one is the
**envelope, never the payload** — the same split the bus itself is built on
([messaging § envelope](../../docs/04-messaging.md#envelope)).

| The same for every tool | |
|---|---|
| how it is addressed | a name and an inbox, `template/instance@realm` where there are several copies ([identity § names](../../docs/01-identity.md#names)) |
| who may call it | the record's ACL, applied by the daemon before delivery ([identity § acl](../../docs/01-identity.md#acl)) |
| what it needs | `env.dist`, the declared surface — which is also what makes an upload checkable and says whether an instance is required at all ([runner § the three env layers](../R1/runner.md#the-three-env-layers)) |
| what it costs | counters per (principal, service), the same pair everywhere ([discovery § stats](../R1/discovery.md#stats)) |
| what it is | the version it answers with, when that arrives ([Plans/R1](../R1/TODO.md#todo-r1)) |
| **how it fails** | **an upstream refusing is an answer, not a failure.** A rate limit or a provider error comes back as the reply, as it was given; no reply is reserved for the **tool itself** being broken. Otherwise every caller learns two error channels and guesses which one it is in |

| Deliberately its own | |
|---|---|
| the body | the bus does not read it and a gateway does not either. A caller that named a provider wanted that provider's request and that provider's answer |

**Extending the set is adding an instance, not changing the contract.** A
provider that speaks an API we already have is configuration; one that does not
is another template obeying the same contract. Neither touches a row above,
which is what makes the set extendable rather than merely long.

## The catalogue

**From** says who put it here: the owner asked for it, or it is proposed and
still wants a yes.

### People and the world outside

Every row is two services, one each way.

| | From | |
|---|---|---|
| **slack** | owner | send, and read into a topic |
| **telegram · discord · whatsapp** | owner | the same shape, one instance per account |
| **sms** | owner | send only — there is no inbound half worth having on most carriers |
| **mail** | owner | **IMAP** in, **SMTP** out. The design's own running example is a mail reader ([runner § what an instance is](../R1/runner.md#what-an-instance-is)), and it is the one channel every business already has |
| **webhook** | proposed | an inbound URL that publishes what it receives, and an outbound that calls one. The highest-leverage entry here: with it, the next SaaS integration is configuration rather than a new service |
| **im** | owner | the **router** in front of the rows above. Give it a *person* — by name, or by any alias they are known by — and it delivers to the first destination on their list that takes it |
| **user-locator** | owner | *who is this?* — a name, a partial one, a real name, one of several emails, a nick, and back come the principals that might be meant, **each with a confidence**. It reads the daemon's own records ([identity § registration](../../docs/01-identity.md#registration)) and invents no directory of its own |

**`im` is the one that knows who somebody is.** Every other row speaks one
service and takes an address that service understands; this one takes a person
and picks. Two things it adds, and nothing else here does:

| | |
|---|---|
| **many ids, one person** | a Telegram handle, a Slack member id, an address somebody typed out of habit — all of them resolve onto `parf@srv1`, which is the identity ([identity § how to reach a person](people.md#how-to-reach-a-person)). **An alias is a lookup key and never a principal**: what travels is the name |
| **first one wins** | the destinations are ordered and the first that takes the message ends it. Same rule as the alerter's, because it is **the same list** — the person's, in their record |

**What the locator answers with, and what nobody may do with it:**

| | |
|---|---|
| **a ranked guess, never a decision** | it returns `user@realm` values with a confidence, which is the shape of the question — *parf* is not an identity, and two people may answer to it. **Nothing authorises on a locator answer**: it turns human input into a name that then has to hold a token like anyone else ([identity § how to reach a person](people.md#how-to-reach-a-person)) |
| **an official name outranks everything else** | a hit on the registered `user@realm` ([identity § names](../../docs/01-identity.md#names)) comes first, then the rest — a real name, a nick, one of several emails, an alias from somewhere else. Those are how people are *found*; the username is what they **are**, and a nick that happens to match somebody else's real name must not outrank the person actually called that |
| **the asker narrows it, when they ask for that** | a request may say *rank by who I share a realm or a group with*, which is what makes `parf` mean `parf@realmo` to somebody on that team and something else elsewhere. It is a **ranking input**, not a permission: what a requester may see at all is the ordinary ACL question |
| **one human, two names** | `parf@realmo` and `parf@github` are two principals and may be one person, which is what the record's aliases and its `GithubUser` are for ([identity § registration](../../docs/01-identity.md#registration)). **A link counts only if both sides state it** — otherwise claiming somebody as an alias of mine is how I become findable as them. Nothing proves such a link before R1.1 is done, so until then it is a **hint offered to the asker**, never a reason to rank one answer above another |
| **`im` is its first caller** | `im` needs *this alias is that person*; the locator answers *these people might be meant*. The exact case is the locator's top answer with nothing close behind it, so one of them is not a special case of the other — but the second is where the first gets its data |

**`im` and `alerter` are one delivery with two front doors.** The alerter is
woken by a subscription and picks the list by severity
([the bus watching itself](#the-bus-watching-itself)); `im` is called by
anybody holding a person. Neither keeps anything about people, both stop at
the first success, and if a third caller of this kind appears that is the
signal to make it one service with three ways in rather than three services.

### Reading the box

Safe to hand out widely, which is why they are not in the group below.

| | From | |
|---|---|---|
| **health** | owner | `df`, `free`, `lsblk`, `uptime`, `who` — what an incident asks first |
| **info** | owner | `ps`, and what the hardware and the OS are |
| **logwatch** | owner | the last N lines of everything a glob matched, cleaned, and the new ones as they arrive. An adaptation of `/rd/service/tail-ring-buffer`, which exists to close exactly this loop for an agent: make a change, then see what the logs said |

**`logwatch` is the one here that holds state.** It
keeps a bounded ring of cleaned lines from every file its globs matched,
rescanning for new files as they appear, so a caller asks *what just happened*
instead of finding the right file and scrolling. **The last N verbatim is the
default and a filter is asked for** — the original makes that point and it is
worth keeping: a filter silently hides the error nobody thought to grep for. That ring is state between
messages, which makes it a **kept child** — the same argument a warm cache makes
([runner § long-lived services](../R1/runner.md#long-lived-services)). It is
also bounded by construction, so a log that floods cannot take the host, which
is the instinct the queues are already built on
([messaging § overflow](../../docs/04-messaging.md#overflow)).

Three things the bus changes about it:

| | |
|---|---|
| **both a poll and a feed, and the poll is the common one** | *deploy, then ask for the last N* is the usage, and **a subscription cannot answer it** — a topic carries what happens next, the ring holds what already did. So the ring is asked like any service, and the tail is *also* published into a topic for whoever wants to watch it live. Neither replaces the other |
| **another box is another name** | the original reaches a second host over ssh, with that host written into a local env file. Here it is a call to `logwatch@srv2`, and nothing local has to know where that is |
| **the globs are the grant** | one instance per set of sources, so `logwatch/nginx@srv1` hands over nginx errors and not `/var/log/auth.log` — danger is a name here as everywhere |

**journald is a source, not a second service.** `journalctl -f` tails like
anything else, so unit logs are an instance's configuration rather than another
entry in this catalogue — which is also how the half you can safely hand out
stays separate from **systemd** below: different instance, different grant.

### The bus watching itself

The two that are installed **and enabled** by default, because the first
question after anything goes wrong is *what did it say*, and having to set
something up beforehand is how that question goes unanswered.

| | From | |
|---|---|---|
| **log** | owner | a ring per level — **warning · error · alert** — and the same lines published as they happen. `logwatch` pointed at the bus instead of at a glob |
| **alerter** | owner | a **router**. It decides *who* to reach and *in what order*, and every actual delivery is a call to a service that already does it ([people and the world outside](#people-and-the-world-outside)) |

| | |
|---|---|
| **one contract, not a second one** | ring for the past, topic for the future, the poll the common one — exactly what `logwatch` does and for the same reason ([reading the box](#reading-the-box)). A level is an instance of its own, so a flood of warnings cannot push the errors out of theirs |
| **the ring needs nothing new in the daemon** | the daemon already publishes its own events onto a topic — a key that does not match is written down that way today ([access § key confirmation](../R1/access.md#key-confirmation)). Anything on the bus can publish to the same topics, and the service is only what **remembers** them |
| **the alerter delivers nothing** | it speaks no SMTP, no Telegram API, nothing. Each hop out is an ordinary call to `telegram`, `sms`, `mail` or `slack`, so a new way to reach people is a new instance of something already here and not a change to this service. An alerter that learned to send mail would be a second, worse copy of `mail` |
| **an alert names a person, not a channel** | which is the same identity everything else uses ([identity § names](../../docs/01-identity.md#names)). Turning `parf@srv1` into *telegram first, then SMS* **is** the whole job, and it is the reason this is a service rather than a rule in whoever raised the alert |
| **the order is per person and per severity, and it is not the alerter's** | the list is part of the person's record in the daemon ([identity § how to reach a person](people.md#how-to-reach-a-person)), because it is the person's: several alerters reach the same human, and a phone that changed has to change once |
| **it routes and does not judge** | the level was set by whoever published the line, and it is the level that picks the list. The alerter never re-rates a message, and never drops one for being noisy — that is the publisher's call, and silently overruling it is how an outage gets missed |
| **a bus that is down takes the ring with it** | it is a service, and pretending otherwise would put a log inside the daemon. The journal is still the daemon's own record ([setup § the two units](../../docs/09-setup.md#the-two-units)); this is for everything the bus carries, which is the part no journal sees |

**So the alerter holds nothing about people.** It reads the list from the
record and calls the channel it names; the instance's `env` holds only what the
**host** decided — which channels exist on this box at all
([runner § the three env layers](../R1/runner.md#the-three-env-layers)). Two
alerters given the same alert reach the same person the same way, because
neither of them is where the answer is kept.

**The order is a fallback chain: it stops at the first success.** SMS, then
Telegram — and Telegram only because the SMS did not go. Not a fan-out, and not
an escalation that keeps going until somebody replies.

| | |
|---|---|
| **what counts as a failure** | anything that is not a delivery. A channel refusing is an answer rather than a failure everywhere else in this catalogue ([one contract for the set](#one-contract-for-the-set)); here a refusal and a silence mean the same thing — this person was not reached this way — so both move to the next entry |
| **success is *sent*, not *read*** | a delivered SMS nobody looked at ends the chain. That is the accepted cost of the simple rule, and it is what the ordering is for: the first entry should be the one that reaches you |

⏸️ Waking somebody until they acknowledge is a different mechanic, and may
arrive later rather than complicating this one now.

### Acting on the box

The dangerous tier. Each is a separate name so that each is a separate grant.

| | From | |
|---|---|---|
| **shell** | owner | run a command as the service's own account |
| **root-shell** | owner | the same as root. A second name, not a flag — and the one entry in this catalogue that hands over the machine, so it is written down as exactly that |
| **systemd** | owner | inspect · start/stop · enable/disable · add/remove, each a grant of its own |
| **dnf** | owner | inspect and install. One service with a backend behind it, so `apt` is an adapter rather than a second service ([modules § the rule](../../docs/10-modules.md#the-rule)) |
| **tmux** | owner | start, stop, read a pane, post into one |
| **files** | proposed | read, write and list under one declared root. Almost everything else needs it — a picture service has to get the picture from somewhere |
| **git** | proposed | clone, pull, status. `service.d` is a checkout ([runner § what an instance is](../R1/runner.md#what-an-instance-is)), so this is how a deploy actually happens |
| **cron** | proposed | send this message later, or on a schedule. A bus with queues and no clock is missing the one thing every automation wants, and as a service it costs the daemon nothing |

### Data

| | From | |
|---|---|---|
| **mysql · postgres** | owner | one instance per account, granted read-only or read-write **as two names** |
| **redis · kvrocks** | owner | the same shape |
| **mongo** | owner | the same shape |
| **kv** | owner | **shared, secure service state and configuration** — the small state that otherwise becomes a database nobody wanted. **The first version is an access wrapper around `kvrocks`**: the surface below is that server's own, and what this adds is who may touch which part of it. Reuse for daemon and runner storage remains an [unassigned proposal](../Future/storage.md#storage) |
| **elastic** | owner | search, and where logs go to live. `logwatch` answers about the last few minutes ([reading the box](#reading-the-box)); this answers about last month |
| **clickhouse** | owner | analytics. Querying and ingesting are two names, as everywhere — they are rarely the same grant |
| **object storage** | proposed | S3 and what speaks it. **files** for a disk, this for a bucket |

**What `kv` provides** is redis's shape, and in the first version it *is*
redis's shape: `kvrocks` speaks that protocol over RocksDB, so the whole list
below already exists and is somebody else's to maintain
([modules § the rule](../../docs/10-modules.md#the-rule)). What is ours
is the access model above it.

| | |
|---|---|
| **values** | `get` and `set`, with a **ttl** per key |
| **conditional set** | **`setNX`** — memcached's `add` — sets only if the key is **absent**, and **`setX`** — its `replace` — only if it is **present**. The first is what turns a value into a claim: whoever's `setNX` succeeded is the one who has it |
| **atomics** | `incBy` (one by default), `and` · `or` · `xor`, and **`cas`** — compare-and-swap is the one that lets a caller be safe without taking a lock at all |
| **hashes** | a hash, and a **hash of hashes** |
| **lists** | as a queue or as a deque, each operation in a **blocking and a non-blocking** form — and the redis **pull-push**, which is **atomic**: pull from `q1`, push onto `q2`, and no state in between where the item is in neither. That is what makes a worker queue survive the worker — the item sits in `q2` until it is done, and a crash leaves it there to be found |
| **hash of lists** | many named lists under one key |

| | |
|---|---|
| **blocking is why this is a service and not a file** | a lock-free `set` is something any script could do to a file it shares. Waiting on an empty list is not, and it is the whole reason a pool reaches for one of these |
| **three of these claim work, and none of them is a lock** | `cas`, `setNX` and the atomic pull-push each answer *exactly one worker takes this item* on their own — which is the batcher case, and worth knowing before reaching for a lock ([messaging § shared locks](../R1/locks.md#shared-locks)) |
| **the wrapper is the product, not a layer over one** | the rule against putting something of ours between a caller and a tool ([rules they all obey](#rules-they-all-obey)) is about silent policy — a retry, a cache, a rewrite the caller cannot see. Here the namespace **is** what the caller asked for: they address their own key space and never the server underneath, so there is no second thing being decided behind their back |
| **and it is two names, not one** | `kvrocks` in the table above hands over a server, and this hands over a namespace inside one. A person who should have the first is not the same person who should have the second — which is how everything here is granted ([rules they all obey](#rules-they-all-obey)) |

**Two namespaces, and only one of them can be shared.**

| | |
|---|---|
| **the personal one** | every service has one, without registering anything and without being granted anything. It is **not sharable** — there is no ACL on it to widen, so nothing can be given away by accident, and a service always has somewhere to put state with nobody to ask |
| **a registered instance** | a service *or* a person registers one, with a name and an `allow` like every other record ([identity § acl](../../docs/01-identity.md#acl)). This is the **only** way two principals share a key, which is what makes sharing something you can see in the registry rather than infer |

| | |
|---|---|
| **the personal namespace belongs to the name, so a pool shares one** | members of a pool are one name ([runner § one name on many hosts](../R1/runner.md#one-name-on-many-hosts)), so they land in the same namespace — the batcher's shared list, with nothing configured and nothing granted |
| **read · write · rw is a role, not a flag** | roles are how the daemon says *what* a principal may do, already — `@team(rw)`, `parf@srv1(read)` ([identity § sigils](../R1/identity.md#sigils)): it is stated per principal in the record, delivered with the call, and the service reads what it was handed. That is not the same as a service checking who is calling, which the rule above forbids — and it is why this does not need `kv-ro` and `kv-rw` as two names. **A flag cannot be granted; a role is nothing but a grant** |

### Other buses

| | From | |
|---|---|---|
| **nats · kafka · rabbitmq** | owner | one instance per cluster, and the two directions are two services as everywhere here: consuming a foreign topic **publishes into a bus topic**, and sending out is **a call** |

**A gateway to a broker is not a broker.** The rule is that `agent-busd` *is*
the broker and no external one is introduced
([overview § goal](../../docs/00-overview.md#goal)) — and it is untouched by this. Nothing
in the bus's own transport changes; what changes is that a company's existing
Kafka becomes reachable from it, on the far side of a name, like every other
gateway in this catalogue. The day one of these is load-bearing *inside* the
design rather than at its edge is the day the rule was broken, and it will be
visible as a daemon change ([rules they all obey](#rules-they-all-obey)).

It is also the same shape as Slack, which is the point: a broker is another
channel, and the catalogue already knows what to do with one.

### Media

| | From | |
|---|---|---|
| **image** | owner | ImageMagick or GraphicsMagick. The reason `--algo=std` exists — bytes in, bytes out, no base64 either way |
| **video** | proposed | `ffmpeg`, the same argument one size up |

### For the agents themselves

Proposed as a group: nothing in the list above is aimed at the things this bus
was built for.

| | From | |
|---|---|---|
| **generation** — `openai` · `anthropic` · `google-ai` · `deepseek` · `groq` · `minimax` · `zhipu` · `bedrock` | owner | one name per provider. A caller picks the provider, so each is a name and none is hidden behind another |
| **routing** — `openrouter` | owner | one name in front of many models, because the router already picks the provider. It is the answer for *"one API over all of them"*, and the reason nothing else here tries to be |
| **retrieval** — `voyage` · `zeroentropyai` | owner | embeddings, reranking, search. Separate names from generation and not verbs on it: **spend is granted per name**, and embedding a corpus is cheap where answering with a frontier model is not — a team may well be allowed the first and not the second |
| **local** — `ollama` · `llama-server` | owner | no key to keep private, so the gateway is here for the other half: the daemon's ACL decides who may spend the box's GPU, and the counters say who did |
| **fetch** | proposed | a URL in, readable text out. What an agent reaches for more than anything else, and a pool case |

**Minimal is the word that matters.** A gateway holds the key, counts, and
passes the call through as it was made. It writes **no access control at all** —
the daemon refused the call before it ever arrived
([identity § acl](../../docs/01-identity.md#acl)) — which is what makes minimal possible:
the hard half was done by the bus. It does **not** normalise
one provider's API into another's — a caller that wants one API over many uses
`openrouter`, which is a product that already does it, and a caller that names
a provider wants that provider's own shape.

Which is also why this is **not** ten programs. Most of that list speaks the
OpenAI API, so it is one service template configured into instances that differ
by a base URL and a key ([identity § names](../../docs/01-identity.md#names)); code of its
own is written only where the API genuinely differs — Anthropic, Google, Bedrock
and the retrieval pair.

**A backend is chosen by the host and is invisible; a gateway is chosen by the
caller and is a name.** That is the line `apt`-inside-**dnf** sits on one side
of and every row above on the other, and it is why one router pointed at
another is a hop nobody asked for.

Each of these holds an API key that **nothing calling it ever sees**, which is
the argument this design already makes about its own tokens
([runner § what the child is told](../R1/runner.md#what-the-child-is-told)).
The account is one instance's `env` and nothing else's
([runner § the three env layers](../R1/runner.md#the-three-env-layers)): one
place the key lives, the daemon's own ACL over who may spend it, and — because **every
message carries a sender principal the bus verified**
([messaging § envelope](../../docs/04-messaging.md#envelope)) — counters that say *which
consumer* spent it rather than only how much the key did.

**Which is what makes them the case billing was designed for.** A gateway needs
nothing new to be billed: the billing role records exactly the (principal,
service) pair a gateway already sees, and asks *may this principal call this
service* through the [balance check](../Future/billing.md#billing-role--future). Until it is turned
on the same pair is a count and a dashboard row; after it, a cap. Splitting
generation from embeddings **by name** rather than by verb is what makes that
work with no new mechanism — price is declared per service, so two prices need
two services, and the split we wanted for access turns out to be the one
billing wanted too.

### Agent runtimes

Self-hosted agents that are already running somewhere: **Hermes Agent** (Nous
Research) and **OpenClaw**. Both are long-lived services with their own chat
gateways and their own model routing, which makes them **peers of this bus
rather than tools on it** — the only entries here that talk in both directions.

| | From | |
|---|---|---|
| **hermes** | owner | one instance per running agent |
| **openclaw** | owner | the same shape |

**Two directions, and each one ships something different.**

| | What ships | |
|---|---|---|
| **the runtime uses the bus** | an MCP server entry in the runtime's own configuration, pointed at this bus's MCP face ([discovery § faces](../../docs/05-discovery.md#faces)), and a name and token for the runtime to hold | every service that name may call becomes one of its skills, pre-filtered, because the catalog is answered per caller ([discovery § audience](../../docs/05-discovery.md#audience)) |
| **the bus reaches the runtime** | `hermes@srv1` · `openclaw@srv1` — a gateway instance per running agent, holding that agent's own credential in its `env` | a citizen sends to the name and gets the agent's answer back. An ordinary call, with nothing of ours between the caller and the tool ([rules they all obey](#rules-they-all-obey)) |

| | |
|---|---|
| **only the second is code, and that is the point** | the first is a configuration file on their side and **no daemon change and no service of ours**. That is what this catalogue is for ([what the catalogue is for](#what-the-catalogue-is-for)) — an entry that needs no new bus feature is the design being right, and this is the strongest one in the list |
| **a runtime is one principal, whoever is typing** | everybody in its chats acts with **the runtime's** token, not their own: it is a shared credential by construction, the way a web child on the owner's socket would have been ([discovery § signing in](../../docs/05-discovery.md#signing-in)). So it gets the grants you would give the least trusted person who can reach it, and a second agent that needs more is **a second name** — danger is a name here as everywhere |
| **which chat an answer lands in is the instance's** | `hermes/ops@srv1` delivers into the ops conversation and `hermes/me@srv1` into a private one. Configured copies of one thing, which the naming already carries ([rules they all obey](#rules-they-all-obey)); the bus routes to a name and never to a conversation |
| **not a launcher, and no push adapter** | `ab-claude` and its siblings attach to a session a person is watching, and a push adapter exists because a running turn has to be interrupted ([runner § adapters](../../docs/08-runner-role.md#adapters)). These are simply up, so the second direction is a `consume` loop like every other row here — the hard part of the coding runtimes is absent |
| **their channels stay theirs** | each already fronts Telegram, Discord, WhatsApp and the rest, and we do not proxy those ([people and the world outside](#people-and-the-world-outside)). A message through their gateway is their principal's and one through ours is ours: two paths to the same app, two grants, on purpose |
| **point their models at ours** | both route to providers of their own. Configured onto `openrouter`, `ollama` or a named provider ([for the agents themselves](#for-the-agents-themselves)), their spend lands under the same ACL and the same counters — otherwise a self-hosted agent is the one consumer whose bill nobody can attribute |
| **two directions make a cycle possible, and nothing stops one** | a runtime can call a service that reaches the runtime. It is the same hazard as any two services calling each other and is the caller's business, not the bus's ([one contract for the set](#one-contract-for-the-set)) — worth naming only because these are the first entries where **both ends are an agent**, and an agent will build the loop without being asked |
| **one being down is not an incident here** | a runtime is a registered name whose queue waits, like any consumer ([messaging § inbox queues](../../docs/04-messaging.md#inbox-queues)). Nothing about the bus is arranged around either of them being up |

## What the catalogue is for

Two things beyond the tools. It is the **proof that the design carries real
work** — every entry that needs no new bus feature is evidence, and the first
one that does is worth more than the tool it was going to be. And it is the
**shape of the ACL a company actually wants**: reading the box and acting on
it are separate groups here because they are separate grants there, and the
ladder is written once, in names rather than in flags.


Unresolved details: [questions](QUESTIONS.md#open-questions).
