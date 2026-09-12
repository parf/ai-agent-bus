# Bundled services

What ships in the box beyond the daemon itself: a catalogue, and the rules
every entry in it obeys. Nothing here is a feature of `agent-busd` — each one
is an ordinary service with a name, an inbox and an ACL, written the way this
design tells anyone else to write one.

Proposed for **Release 1.1** ([stages § release 1.1](12-stages.md#release-11)), after the
runner exists to keep them.

## Rules they all obey

The rules matter more than the list, because the list will grow and they
will not.

| | |
|---|---|
| **no daemon change, ever** | if a bundled service needs the bus to learn something, that is the signal to stop and design rather than to build. The catalogue is a test of the design as much as a set of tools |
| **danger is per name, because access is** | the ACL is attached to a name ([identity § acl](01-identity.md#acl)), so anything you would grant to different people must **be** a different name. `shell@h` and `root-shell@h`; `billing-ro@db1` and `billing-rw@db1`. Never one service with a privilege flag on the call — a flag cannot be granted, only a name can. **Spend divides the same way**: generation and embeddings are two names because a team may be allowed one and not the other |
| **inbound is a publisher, outbound is a call** | every *send/read* pair is two shapes, not one service with two verbs: reading Slack **publishes into a topic** and has no caller ([messaging § push and pull](04-messaging.md#push-and-pull)); sending is request/reply. One pattern for Slack, Telegram, SMS, mail and webhooks |
| **secrets are the instance's** | a bot token or a database password is an `env` file under `runner/`, never in the checkout ([runner § the three env layers](08-runner-role.md#the-three-env-layers)). `service.d` is world-readable by construction |
| **one template, many instances** | `slack/team-a@pool1`, `mysql/billing@db1` — configured copies of one thing, which the naming already carries ([identity § names](01-identity.md#names)) |
| **the form is part of the design** | a picture service is `--algo=std`, a database gateway is `--algo=msgpack` (binary values, the envelope in-band, and a kept process is what holds the connection), a tail is `--algo=jsonl`, a notification is `--algo=args` ([runner § script services](08-runner-role.md#script-services)) |
| **confinement is not what makes the risky ones safe** | a shell service's whole job is to run what it is told, so sandboxing it confines the thing you asked for ([runner § sandboxing](08-runner-role.md#sandboxing)). What contains it is the ACL and the account it runs as — nothing else pretends otherwise |
| **stateless ones are the pool cases** | scaling a picture or fetching a page is work anybody can do, so it pools across hosts ([runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts)). Anything about *this box* is per host by definition: `shell@srv1` means that machine and nothing else |

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
| **logs** | proposed | `journalctl`, scoped to named units. Pairs with **systemd** below, and is the half you can give to people you would not give the other half to |

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
| **object storage** | proposed | S3 and what speaks it. **files** for a disk, this for a bucket |

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
| **openrouter** | owner | generation, one name in front of many models. The router already picks the provider, so this is **one** gateway rather than one per vendor — the same reasoning that makes `apt` an adapter inside **dnf** |
| **bedrock** | owner | generation on AWS — **beside** the router, not inside it. A different account, a different bill, and so by the rule above a different name. It also marks the line: a **backend** is chosen by the host and invisible to a caller, the way `apt` sits inside **dnf**; a **gateway** is chosen by the caller, so it is a name. One router pointed at another is a hop nobody asked for |
| **voyage** | owner | embeddings and reranking — and a separate name from the one above, not a verb on it. **Spend is granted per name** like everything else here: embedding a corpus is cheap and answering with a frontier model is not, and a team may well be allowed the first and not the second |
| **fetch** | proposed | a URL in, readable text out. What an agent reaches for more than anything else, and a pool case |

Each of these holds an API key that **nothing calling it ever sees**, which is
the argument this design already makes about its own tokens
([runner § what the child is told](08-runner-role.md#what-the-child-is-told)).
The account is one instance's `env` and nothing else's
([runner § the three env layers](08-runner-role.md#the-three-env-layers)): one
place the key lives, one ACL over who may spend it, and — because **every
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
