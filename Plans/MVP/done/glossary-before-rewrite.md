# Historical glossary

Snapshot before scope separation; not current behavior.

# Glossary

Every name and term, one line each, with the section that defines it. This file
is **normative for naming**: if a name here disagrees with prose elsewhere, this
wins.

## Names that are enforced

| Name | Is | Never |
|---|---|---|
| `agent-busd` | *the* daemon (`sshd`/`dockerd` convention) | not "the server", not `agentbusd` |
| `agent-bus` | the CLI any user runs | — |
| `agent-bus-setup` | the root-only installer | not `agent-bus setup` |
| `agent-bus-token` | what hands a user a credential, locally or as their forced command over SSH | not an admin program |
| `agent-bus` | the keyword everywhere else: `/etc/agent-bus/`, `~/.config/agent-bus/`, `/var/lib/agent-bus/`, `agent-busd.service` | — |
| `agent-bus-runner` | the program that keeps a set of services, and the system account it runs as | not "the runner daemon"; it is not a child of `agent-busd` |
| `runner@<host>` | the name it registers under on the bus, where it is reached like any other service | never `agent-bus-run` — nothing in this design says `run` |
| `ab_` | **MCP tool prefix only** (`ab_list_services`, `ab_call`) | never in CLI, config or prose. Underscore, not `ab:` — a client exposes a tool as `mcp__<server>__<tool>` (and a plugin's as `mcp__plugin_<plugin>_<server>__<tool>`), and model tool names must match `[a-zA-Z0-9_-]{1,64}`, so a colon does not survive the trip; `:` also already means a capability (`publish:<glob>`) |
| `ab:` | the **plugin** namespace, if we ship a Claude Code plugin named `ab` | slash commands, skills and agents only — `/ab:send`, never a tool name |

## Vocabulary

| Say | Not | Why |
|---|---|---|
| service discovery | registration | registration is one operation on discovery |
| personal / shared | user-scoped/system, bound/unbound, single-/multi-tenant | shared is the default |
| realm | domain, provider, tenant | the right-hand side of `user@realm` |
| service template | service, service type | a **service** is always configured; the template is the unconfigured capability. *Record* kind (generic · agent · topic) is a different axis and keeps its name |
| service | instance, service instance | a configured thing is a **service**; *instance* survives only as the second half of a name and as the runner's per-instance directory ([runner § what an instance is](../../R1/runner.md#what-an-instance-is)) |

## Terms

| Term | One line | Defined in |
|---|---|---|
| **principal** | any user or service with an identity | [identity § principals](../../../docs/01-identity-and-roles.md#identities) |
| **`user@realm`** | how every principal is written; realm = a name a bus answers for · provider · team. A service may prefix a template | [identity § names](../../../docs/01-identity-and-roles.md#names) |
| **realm** | who vouches for a name | [identity § names](../../../docs/01-identity-and-roles.md#names) |
| **canonical name** | the one spelling everything compares and routes on: lower-case, ASCII, trimmed per component, within the bound | [identity § names](../../../docs/01-identity-and-roles.md#names) |
| **token** | the one thing every call carries, and the whole identity; persisted, previous one kept | [access § token lifetime](../../../docs/02-access.md#token-lifetime) |
| **service ACL / master ACL** | the two access layers, service asked first | [identity § acl](../../../docs/02-access.md#acl) |
| **`*`** | the term for anyone who can authenticate; `allow: *` opens a service to the world | [identity § sigils](../../R1/identity.md#sigils) |
| **`agent-bus-admin`** | the program that edits what the `agent-busd` account owns — *not* a role; the setup user simply holds master | [setup § the programs](../../../docs/09-setup.md#the-programs) |
| **role** | service-defined string saying what a principal may do, written in parentheses after the term and handed over as written | [identity § sigils](../../R1/identity.md#sigils) |
| **`@`** · **`#`** (ACL) | a leading `@` is a group, a leading `#` a service, and anything else is a user | [identity § sigils](../../R1/identity.md#sigils) |
| **`term(roles)`** | an ACL entry: the term says who, the parentheses what the service is told; omitted when there are no roles | [identity § sigils](../../R1/identity.md#sigils) |
| **`GithubUser`** | the GitHub login on a record, equal to the username when the record came from GitHub; unique across records like every other identifier | [identity § registration](../../../docs/01-identity-and-roles.md#registration) |
| **maintainer** (of the daemon) | who may write user records — everything below their own level and nothing at it; the owner is always one | [identity § who may write a record](../../../docs/01-identity-and-roles.md#users-and-profiles) |
| **delegation / on-behalf-of** | A calls B for U, carrying a claim | [identity § delegation](../../R1/identity.md#delegation) |
| **service-to-service token** | one service's credential, narrowed to the service it is calling, asked for over the bus and as itself | [access § service to service](../../R1.1/access.md#service-to-service) |
| **generic · agent · consumer · publisher** | the service kinds | [services § service kinds](../../../docs/03-services-and-topics.md#service-kinds) |
| **`lock` · `try-lock` · `release`** | a named lock the daemon grants to one holder for a ttl; the first waits, the second does not | [messaging § shared locks](../../R1/locks.md#shared-locks) |
| **lock set** | a named group of locks, one per resource a shared service owns; take any free one and be told which | [messaging § a set of locks](../../R1/locks.md#a-set-of-locks) |
| **service template** | the *unconfigured* capability; does not run, has no address | [services § service and template](../../../docs/03-services-and-topics.md#service-and-template) |
| **service** | **always configured**: a template + its config + where it runs. Never say "service" for an unconfigured template | [services § service and template](../../../docs/03-services-and-topics.md#service-and-template) |
| **`template/instance-name@realm`** | a service configured from a template; `service@realm` when there is no separate template | [identity § names](../../../docs/01-identity-and-roles.md#names) |
| **`service-template`** | the verb that configures a template into a service, and reads that configuration back | [services § configuring a template](../../../docs/03-services-and-topics.md#configuring-a-template) |
| **`protocol`** | on a record: how to call it. Unset = an ordinary bus service, send to the name; `/etc/services` names suggested, never checked | [services § how to call it](../../../docs/03-services-and-topics.md#how-to-call-it) |
| **`reading` · `queued` · `in` · `out`** | live state on an answer, never stored: is anything serving this name, how much is waiting, and how much has arrived and been taken since the daemon started | [discovery § what a listing answers](../../../docs/05-discovery.md#what-a-listing-answers) |
| **`dropped` · `expired` · `oldest`** | live state on an answer too: what this inbox lost to overflow and to TTL, and how long the head of its queue has waited | [discovery § what a listing answers](../../../docs/05-discovery.md#what-a-listing-answers) |
| **topic** | a registered record; kind `queue` or `pub/sub` | [services § topics](../../../docs/03-services-and-topics.md#topics) |
| **inbox** | the implicit queue topic every agent owns | [messaging § inbox queues](../../../docs/04-messaging.md#inbox-queues) |
| **`message_id` · topic · tag** | the three fields on every message | [messaging § message fields](../../../docs/04-messaging.md#message-fields) |
| **ack / done** | the two optional receipts, both emitted by the receiver: `ack` = got it, `done` = finished processing | [messaging § receipts](../../../docs/04-messaging.md#receipts) |
| **ring / strict** | the two overflow modes | [messaging § overflow](../../../docs/04-messaging.md#overflow) |
| **envelope** | everything the bus can see; bodies are not in it | [messaging § envelope](../../../docs/04-messaging.md#envelope) |
| **static · pairwise · derived** | the three key modes | [access § key modes](../../R1/access.md#key-modes) |
| **epoch** | one hour; the lifetime of a derived key | [access § key modes](../../R1/access.md#key-modes) |
| **bundle · generation (`gen`)** | signed AUTH data in git, newer wins | [AUTH role § bundle](../../R1/auth.md#bundle) |
| **`master_secret`** | out-of-band file, AUTH replicas only | [AUTH role § where it runs](../../R1/auth.md#where-it-runs) |
| **chaining / upstream** | query up, never replicate | [overview § chaining](../../R1/federation.md#chaining) |
| **audience** | who may see and use a service or topic | [discovery § audience](../../../docs/05-discovery.md#audience) |
| **debug mode** | admin-only message trace on one service | [discovery § debug mode](../../Future/debug.md#debug-mode) |
| **adapter** (runtime) | per-runtime push path into a live agent session | [runner § adapters](../../../docs/08-runner-role.md#adapters) |
| **thin glue** | built-in first, then the system's tool, then a library — never our own | [modules § external tools](../../../docs/10-modules.md#external-tools) |
| **supervisor** | the `agent-busd` process that spawns the rest and holds nothing else | [processes](../../../docs/11-processes.md#processes-and-privileges) |
| **`agent-busd`** · **`agent-bus-runner`** (accounts) | the two system users, one per secret domain: credentials and configurations, neither readable by the other | [setup § the two accounts](../../../docs/09-setup.md#the-two-accounts) |
| **configuration** (registry) · **environment** (runner) | two things one word names: what a template was configured with, which the daemon holds and a service fetches for itself; and the env files the runner injects, which it holds and nobody reads back | [services § configuring a template](../../../docs/03-services-and-topics.md#configuring-a-template) · [runner § the three env layers](../../R1/runner.md#the-three-env-layers) |
| **`--algo`** (`args` · `std` · `json` · `jsonl` · `msgpack`) | how a message reaches a script and what that implies about the process: argv, raw bytes on stdin, the envelope as JSON, or — into a child that is kept — that JSON per line, or `uint32`-framed msgpack | [runner § script services](../../../docs/08-runner-role.md#script-services) |
| **pool** (`--share`) | one name served by several processes, on one host or many, all reporting to one bus; the word is the same on `consume` and on `start` | [runner § one name on many hosts](../../R1/runner.md#one-name-on-many-hosts) |
| **`on`** (listing field) | where a name's members are running, one entry each, stated by them and never checked | [discovery § where a member says it is](../../R1/discovery.md#where-a-member-says-it-is) |
| **pool realm** | a realm a daemon holds that is not its hostname, so a pool's name claims membership rather than a location — `image-scaler@pool1` | [identity § names](../../../docs/01-identity-and-roles.md#names) |
| **long-lived service** | a child started once and fed message after message, so state survives between them — the stream forms `jsonl` and `msgpack`, and the only shapes `reload` means anything to | [runner § long-lived services](../../R1/runner.md#long-lived-services) |
| **template** · **instance** | on disk: `service.d/<name>` is what a service is, `runner/<name>/<instance>` is what one is configured with | [runner § what an instance is](../../R1/runner.md#what-an-instance-is) |
| **`services.json`** | the runner's list of everything **installed** here: per service the host's options (`autostart`, `-N`, confinement, `depends`) and what the runner records (version, origin, `first-started`, `last-started`) | [runner § the list of what is installed](../../R1/runner.md#the-list-of-what-is-installed) |
| **`autostart`** (`on` · `off` · `on-demand`) | one field in that row — at boot, never, or when first addressed; `off` is installed and configured, started by hand | [runner § the list of what is installed](../../R1/runner.md#the-list-of-what-is-installed) |
| **`depends`** | services this one comes after: the author's in `config.json`, added to or turned off by the host in `services.json`; ordering only, never a readiness wait | [runner § what it comes after](../../R1/runner.md#what-it-comes-after) |
| **`kept` · `ephemeral`** | whether the registry holds a record once nobody is using it; a different axis from kind, and nothing being served ever expires | [services § how long a record lives](../../R1.1/records.md#how-long-a-record-lives) |
| **owner · maintainer** | one **user** who holds the record; a **group** that may change everything about it but ownership | [identity § ownership](../../../docs/01-identity-and-roles.md#ownership) |
| **bus** | the child that is the core: registry, queues, sessions, delivery | [processes](../../../docs/11-processes.md#processes-and-privileges) |
| **port** | an interface core depends on; the seam a dependency is swapped at | [modules § the rule](../../../docs/10-modules.md#the-rule) |
| **adapter** (layer) | the one implementation of a port; the only layer allowed outside I/O | [modules § the rule](../../../docs/10-modules.md#the-rule) |
| **face** | an entry point — API, MCP, WEB, CLI — with no domain logic | [modules § the rule](../../../docs/10-modules.md#the-rule) |

## CLI verbs

| Group | Verbs |
|---|---|
| identity | `keygen`, `register`, `enrol` |
| registry | `register`, `topic create`, `ls`, `service-template`, `status` |
| messaging | `send`, `call`, `publish`, `subscribe`, `unsubscribe`, `consume`, `ack`, `done`, `reply` |
| runner | `start`, `stop`, `restart`, `reload`, `enable`, `disable`, `logs` — systemd's semantics; **`start`, never `run`**, and `reload` only where a child is kept |
| AUTH admin | `auth sign`, `auth admin` |
| over SSH | `token`, the admin grammar (`user add\|list\|remove`), `bundle show\|history`, `replica-sync` |
