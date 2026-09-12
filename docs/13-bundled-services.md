# Bundled services

What ships in the box beyond the daemon itself: a catalogue, and the rules
every entry in it obeys. Nothing here is a feature of `agent-busd` — each one
is an ordinary service with a name, an inbox and an ACL the daemon enforces for
it, written the way this design tells anyone else to write one.

Proposed for **Release 1.1** ([stages § release 1.1](12-stages.md#release-11)),
after the runner exists to keep them.

## Rules they all obey

The rules matter more than the list, because the list will grow and they
will not.

| | |
|---|---|
| **no daemon change, ever** | if a bundled service needs the bus to learn something, that is the signal to stop and design rather than to build. The catalogue is a test of the design as much as a set of tools |
| **danger is per name, because access is** | the ACL is attached to a name ([identity § acl](01-identity.md#acl)), so anything you would grant to different people must **be** a different name. `shell@h` and `root-shell@h`; `billing-ro@db1` and `billing-rw@db1`. Never one service with a privilege flag on the call — a flag cannot be granted, only a name can. **Spend divides the same way**: generation and embeddings are two names because a team may be allowed one and not the other |
| **none of them enforces access** | the ACL is on the record and **`agent-busd` applies it before a call is delivered** ([identity § acl](01-identity.md#acl)). A bundled service that checks who is calling is writing a second, weaker copy of something already done — and this is the single biggest reason most of these are a hundred lines rather than a project |
| **inbound is a publisher, outbound is a call** | every *send/read* pair is two shapes, not one service with two verbs: reading Slack **publishes into a topic** and has no caller ([messaging § push and pull](04-messaging.md#push-and-pull)); sending is request/reply. One pattern for Slack, Telegram, SMS, mail and webhooks |
| **secrets are the instance's** | a bot token or a database password is an `env` file under `runner/`, never in the checkout ([runner § the three env layers](08-runner-role.md#the-three-env-layers)). `service.d` is world-readable by construction |
| **one template, many instances** | `slack/team-a@pool1`, `mysql/billing@db1` — configured copies of one thing, which the naming already carries ([identity § names](01-identity.md#names)) |
| **the form is part of the design** | a picture service is `--algo=std`, a database gateway is `--algo=msgpack` (binary values, the envelope in-band, and a kept process is what holds the connection), a tail is `--algo=jsonl`, a notification is `--algo=args` ([runner § script services](08-runner-role.md#script-services)) |
| **confinement is not what makes the risky ones safe** | a shell service's whole job is to run what it is told, so sandboxing it confines the thing you asked for ([runner § sandboxing](08-runner-role.md#sandboxing)). What contains it is the ACL and the account it runs as — nothing else pretends otherwise |
| **no layer of ours between a caller and the tool** | no retry policy, no cache, no request rewriting, no validation of what is being asked. Each of those is the caller's decision, and one made silently inside a gateway is one nobody can debug from outside. If a retry or a cache is wanted it is **a service with a name of its own**, which is this catalogue's rule for everything else too |
| **stateless ones are the pool cases** | scaling a picture or fetching a page is work anybody can do, so it pools across hosts ([runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts)). Anything about *this box* is per host by definition: `shell@srv1` means that machine and nothing else |

## One contract for the set

These are many tools and should feel like one set. What makes them one is the
**envelope, never the payload** — the same split the bus itself is built on
([messaging § envelope](04-messaging.md#envelope)).

| The same for every tool | |
|---|---|
| how it is addressed | a name and an inbox, `template/instance@realm` where there are several copies ([identity § names](01-identity.md#names)) |
| who may call it | the record's ACL, applied by the daemon before delivery ([identity § acl](01-identity.md#acl)) |
| what it needs | `env.dist`, the declared surface — which is also what makes an upload checkable and says whether an instance is required at all ([runner § the three env layers](08-runner-role.md#the-three-env-layers)) |
| what it costs | counters per (principal, service), the same pair everywhere ([discovery § stats](05-discovery.md#stats)) |
| what it is | the version it answers with, when that arrives ([Plans/V1](../Plans/V1/TODO.md)) |
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
| **mail** | owner | **IMAP** in, **SMTP** out. The design's own running example is a mail reader ([runner § what an instance is](08-runner-role.md#what-an-instance-is)), and it is the one channel every business already has |
| **webhook** | proposed | an inbound URL that publishes what it receives, and an outbound that calls one. The highest-leverage entry here: with it, the next SaaS integration is configuration rather than a new service |

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
([runner § long-lived services](08-runner-role.md#long-lived-services)). It is
also bounded by construction, so a log that floods cannot take the host, which
is the instinct the queues are already built on
([messaging § overflow](04-messaging.md#overflow)).

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
| **the ring needs nothing new in the daemon** | the daemon already publishes its own events onto a topic — a key that does not match is written down that way today ([access § key confirmation](02-access.md#key-confirmation)). Anything on the bus can publish to the same topics, and the service is only what **remembers** them |
| **the alerter delivers nothing** | it speaks no SMTP, no Telegram API, nothing. Each hop out is an ordinary call to `telegram`, `sms`, `mail` or `slack`, so a new way to reach people is a new instance of something already here and not a change to this service. An alerter that learned to send mail would be a second, worse copy of `mail` |
| **an alert names a person, not a channel** | which is the same identity everything else uses ([identity § names](01-identity.md#names)). Turning `parf@srv1` into *telegram first, then SMS* **is** the whole job, and it is the reason this is a service rather than a rule in whoever raised the alert |
| **the order is per person and per severity, and it is not the alerter's** | the list is part of the person's record in the daemon ([identity § how to reach a person](01-identity.md#how-to-reach-a-person)), because it is the person's: several alerters reach the same human, and a phone that changed has to change once |
| **it routes and does not judge** | the level was set by whoever published the line, and it is the level that picks the list. The alerter never re-rates a message, and never drops one for being noisy — that is the publisher's call, and silently overruling it is how an outage gets missed |
| **a bus that is down takes the ring with it** | it is a service, and pretending otherwise would put a log inside the daemon. The journal is still the daemon's own record ([setup § the two units](09-setup.md#the-two-units)); this is for everything the bus carries, which is the part no journal sees |

**So the alerter holds nothing about people.** It reads the list from the
record and calls the channel it names; the instance's `env` holds only what the
**host** decided — which channels exist on this box at all
([runner § the three env layers](08-runner-role.md#the-three-env-layers)). Two
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
| **dnf** | owner | inspect and install. One service with a backend behind it, so `apt` is an adapter rather than a second service ([modules § the rule](10-modules.md#the-rule)) |
| **tmux** | owner | start, stop, read a pane, post into one |
| **files** | proposed | read, write and list under one declared root. Almost everything else needs it — a picture service has to get the picture from somewhere |
| **git** | proposed | clone, pull, status. `service.d` is a checkout ([runner § what an instance is](08-runner-role.md#what-an-instance-is)), so this is how a deploy actually happens |
| **cron** | proposed | send this message later, or on a schedule. A bus with queues and no clock is missing the one thing every automation wants, and as a service it costs the daemon nothing |

### Data

| | From | |
|---|---|---|
| **mysql · postgres** | owner | one instance per account, granted read-only or read-write **as two names** |
| **redis · kvrocks** | owner | the same shape |
| **mongo** | owner | the same shape |
| **kv** | owner | ours, not a gateway: memory-only or persistent, chosen per instance. The small shared state that otherwise becomes a database nobody wanted |
| **elastic** | owner | search, and where logs go to live. `logwatch` answers about the last few minutes ([reading the box](#reading-the-box)); this answers about last month |
| **clickhouse** | owner | analytics. Querying and ingesting are two names, as everywhere — they are rarely the same grant |
| **object storage** | proposed | S3 and what speaks it. **files** for a disk, this for a bucket |

**What `kv` provides**, and it is **redis's shape on purpose** — proven,
familiar, and every language already thinks in it:

| | |
|---|---|
| **values** | `get` and `set`, with a **ttl** per key |
| **atomics** | `incBy` (one by default), `and` · `or` · `xor`, and **`cas`** — compare-and-swap is the one that lets a caller be safe without taking a lock at all |
| **hashes** | a hash, and a **hash of hashes** |
| **lists** | as a queue or as a deque, each operation in a **blocking and a non-blocking** form — and the redis **pull-push**: take from one list and put it on another in one step, which is what makes a worker queue survive the worker |
| **hash of lists** | many named lists under one key |

| | |
|---|---|
| **blocking is why this is a service and not a file** | a lock-free `set` is something any script could do to a file it shares. Waiting on an empty list is not, and it is the whole reason a pool reaches for one of these |
| **`cas` and `pull-push` are the two that claim work** | between them, *exactly one worker takes this item* needs no lock — which is the batcher case, and worth knowing before reaching for one ([messaging § shared locks](04-messaging.md#shared-locks)) |
| **it is ours, so it stays small** | anything past this list is redis, and redis is already in the table above. Ours exists for the case where a dependency is not wanted, and it stops being that the moment it chases the feature list |

**Two namespaces, and only one of them can be shared.**

| | |
|---|---|
| **the personal one** | every service has one, without registering anything and without being granted anything. It is **not sharable** — there is no ACL on it to widen, so nothing can be given away by accident, and a service always has somewhere to put state with nobody to ask |
| **a registered instance** | a service *or* a person registers one, with a name and an `allow` like every other record ([identity § acl](01-identity.md#acl)). This is the **only** way two principals share a key, which is what makes sharing something you can see in the registry rather than infer |

| | |
|---|---|
| **the personal namespace belongs to the name, so a pool shares one** | members of a pool are one name ([runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts)), so they land in the same namespace — the batcher's shared list, with nothing configured and nothing granted |
| **read · write · rw is a `#role`, not a flag** | roles are how the daemon says *what* a principal may do, already ([identity § sigils](01-identity.md#sigils)): it is stated per principal in the record, delivered with the call, and the service reads what it was handed. That is not the same as a service checking who is calling, which the rule above forbids — and it is why this does not need `kv-ro` and `kv-rw` as two names. **A flag cannot be granted; a role is nothing but a grant** |

❓ **A hash of locks.** Asked for, and the one item here that would be a
**second lock authority**: the daemon grants named locks as of Release 1
([messaging § shared locks](04-messaging.md#shared-locks)), and two things
granting locks is exactly what that section argues against. Either these *are*
the daemon's locks under a name, or the kv holds them itself and then a `kv`
that is a **pool** cannot be correct. *Settled by:* owner.

### Other buses

| | From | |
|---|---|---|
| **nats · kafka · rabbitmq** | owner | one instance per cluster, and the two directions are two services as everywhere here: consuming a foreign topic **publishes into a bus topic**, and sending out is **a call** |

**A gateway to a broker is not a broker.** The rule is that `agent-busd` *is*
the broker and no external one is introduced
([overview § goal](00-overview.md#goal)) — and it is untouched by this. Nothing
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
([identity § acl](01-identity.md#acl)) — which is what makes minimal possible:
the hard half was done by the bus. It does **not** normalise
one provider's API into another's — a caller that wants one API over many uses
`openrouter`, which is a product that already does it, and a caller that names
a provider wants that provider's own shape.

Which is also why this is **not** ten programs. Most of that list speaks the
OpenAI API, so it is one service template configured into instances that differ
by a base URL and a key ([identity § names](01-identity.md#names)); code of its
own is written only where the API genuinely differs — Anthropic, Google, Bedrock
and the retrieval pair.

**A backend is chosen by the host and is invisible; a gateway is chosen by the
caller and is a name.** That is the line `apt`-inside-**dnf** sits on one side
of and every row above on the other, and it is why one router pointed at
another is a hop nobody asked for.

Each of these holds an API key that **nothing calling it ever sees**, which is
the argument this design already makes about its own tokens
([runner § what the child is told](08-runner-role.md#what-the-child-is-told)).
The account is one instance's `env` and nothing else's
([runner § the three env layers](08-runner-role.md#the-three-env-layers)): one
place the key lives, the daemon's own ACL over who may spend it, and — because **every
message carries a sender principal the bus verified**
([messaging § envelope](04-messaging.md#envelope)) — counters that say *which
consumer* spent it rather than only how much the key did.

**Which is what makes them the case billing was designed for.** A gateway needs
nothing new to be billed: the billing role records exactly the (principal,
service) pair a gateway already sees, and asks *may this principal call this
service* per epoch ([future § billing](future/billing.md)). Until it is turned
on the same pair is a count and a dashboard row; after it, a cap. Splitting
generation from embeddings **by name** rather than by verb is what makes that
work with no new mechanism — price is declared per service, so two prices need
two services, and the split we wanted for access turns out to be the one
billing wanted too.

## What the catalogue is for

Two things beyond the tools. It is the **proof that the design carries real
work** — every entry that needs no new bus feature is evidence, and the first
one that does is worth more than the tool it was going to be. And it is the
**shape of the ACL a company actually wants**: reading the box and acting on
it are separate groups here because they are separate grants there, and the
ladder is written once, in names rather than in flags.
