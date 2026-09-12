# Glossary

Every name and term, one line each, with the section that defines it. This file
is **normative for naming**: if a name here disagrees with prose elsewhere, this
wins.

## Names that are enforced

| Name | Is | Never |
|---|---|---|
| `agent-busd` | *the* daemon (`sshd`/`dockerd` convention) | not "the server", not `agentbusd` |
| `agent-bus` | the CLI any user runs | — |
| `agent-bus-setup` | the root-only installer | not `agent-bus setup`, which was a verb |
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
| service | instance, service instance | "instance" was the old word for the configured thing — that is now just a service |

## Terms

| Term | One line | Defined in |
|---|---|---|
| **principal** | any user or service with an identity | [identity § principals](01-identity.md#principals) |
| **`user@realm`** | how every principal is written; realm = host · provider · team. A service may prefix a template | [identity § names](01-identity.md#names) |
| **realm** | who vouches for a name | [identity § names](01-identity.md#names) |
| **canonical name** | the one spelling everything compares and routes on: lower-case, ASCII, trimmed per component, within the bound | [identity § names](01-identity.md#names) |
| **token** | the one thing every call carries, and the whole identity; persisted, previous one kept | [access § token lifetime](02-access.md#token-lifetime) |
| **service ACL / master ACL** | the two access layers, service asked first | [identity § acl](01-identity.md#acl) |
| **`allow: *`** | anyone who can authenticate | [identity § acl](01-identity.md#acl) |
| **`agent-bus-admin`** | the program that edits what the `agent-busd` account owns — *not* a role; the setup user simply holds master | [setup § the programs](09-setup.md#the-programs) |
| **role** | service-defined string saying what a principal may do; `#admin` here, `admin` to the service | [identity § sigils](01-identity.md#sigils) |
| **`@`** · **`#`** (ACL) | a leading `@` is a group, a leading `#` a role, and anything else is a user | [identity § sigils](01-identity.md#sigils) |
| **delegation / on-behalf-of** | A calls B for U, carrying a claim | [identity § delegation](01-identity.md#delegation) |
| **generic · agent · consumer · publisher** | the service kinds | [services § service kinds](03-services-and-topics.md#service-kinds) |
| **service template** | the *unconfigured* capability; does not run, has no address | [services § service and template](03-services-and-topics.md#service-and-template) |
| **service** | **always configured**: a template + its config + where it runs. Never say "service" for an unconfigured template | [services § service and template](03-services-and-topics.md#service-and-template) |
| **`template/instance-name@host`** | a service configured from a template; `service@host` when there is no separate template | [identity § names](01-identity.md#names) |
| **`service-template`** | the verb that configures a template into a service, and reads that configuration back | [services § configuring a template](03-services-and-topics.md#configuring-a-template) |
| **`protocol`** | on a record: how to call it. Unset = an ordinary bus service, send to the name; `/etc/services` names suggested, never checked | [services § how to call it](03-services-and-topics.md#how-to-call-it) |
| **`reading` · `queued` · `in` · `out`** | live state on an answer, never stored: is anything serving this name, how much is waiting, and how much has arrived and been taken since the daemon started | [discovery § what a listing answers](05-discovery.md#what-a-listing-answers) |
| **`dropped` · `expired` · `oldest`** | live state on an answer too: what this inbox lost to overflow and to TTL, and how long the head of its queue has waited | [discovery § what a listing answers](05-discovery.md#what-a-listing-answers) |
| **topic** | a registered record; kind `queue` or `pub/sub` | [services § topics](03-services-and-topics.md#topics) |
| **inbox** | the implicit queue topic every agent owns | [messaging § inbox queues](04-messaging.md#inbox-queues) |
| **`message_id` · topic · tag** | the three fields on every message | [messaging § message fields](04-messaging.md#message-fields) |
| **ack / done** | the two optional receipts, both emitted by the receiver: `ack` = got it, `done` = finished processing | [messaging § receipts](04-messaging.md#receipts) |
| **ring / strict** | the two overflow modes | [messaging § overflow](04-messaging.md#overflow) |
| **envelope** | everything the bus can see; bodies are not in it | [messaging § envelope](04-messaging.md#envelope) |
| **static · pairwise · derived** | the three key modes | [access § key modes](02-access.md#key-modes) |
| **epoch** | one hour; the lifetime of a derived key | [access § key modes](02-access.md#key-modes) |
| **bundle · generation (`gen`)** | signed AUTH data in git, newer wins | [AUTH role § bundle](06-auth-role.md#bundle) |
| **`master_secret`** | out-of-band file, AUTH replicas only | [AUTH role § where it runs](06-auth-role.md#where-it-runs) |
| **chaining / upstream** | query up, never replicate | [overview § chaining](00-overview.md#chaining) |
| **audience** | who may see and use a service or topic | [discovery § audience](05-discovery.md#audience) |
| **debug mode** | admin-only message trace on one service | [discovery § debug mode](05-discovery.md#debug-mode) |
| **adapter** (runtime) | per-runtime push path into a live agent session | [runner § adapters](08-runner-role.md#adapters) |
| **thin glue** | built-in first, then the system's tool, then a library — never our own | [modules § external tools](10-modules.md#external-tools) |
| **supervisor** | the `agent-busd` process that spawns the rest and holds nothing else | [processes](11-processes.md) |
| **`agent-busd`** · **`agent-bus-runner`** (accounts) | the two system users, one per secret domain: credentials and configurations, neither readable by the other | [setup § the two accounts](09-setup.md#the-two-accounts) |
| **configuration** (registry) · **environment** (runner) | two things one word names: what a template was configured with, which the daemon holds and a service fetches for itself; and the env files the runner injects, which it holds and nobody reads back | [services § configuring a template](03-services-and-topics.md#configuring-a-template) · [runner § the three env layers](08-runner-role.md#the-three-env-layers) |
| **`--algo`** (`args` · `std` · `json` · `jsonl` · `msgpack`) | how a message reaches a script and what that implies about the process: argv, raw bytes on stdin, the envelope as JSON, or — into a child that is kept — that JSON per line, or `uint32`-framed msgpack | [runner § script services](08-runner-role.md#script-services) |
| **pool** (`--share`) | one name served by several processes, on one host or many; the word is the same on `consume` and on `start` | [runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts) |
| **long-lived service** | a child started once and fed message after message, so state survives between them — the stream forms `jsonl` and `msgpack`, and the only shapes `reload` means anything to | [runner § long-lived services](08-runner-role.md#long-lived-services) |
| **template** · **instance** | on disk: `service.d/<name>` is what a service is, `runner/<name>/<instance>` is what one is configured with | [runner § what an instance is](08-runner-role.md#what-an-instance-is) |
| **bus** | the child that is the core: registry, queues, sessions, delivery | [processes](11-processes.md) |
| **port** | an interface core depends on; the seam a dependency is swapped at | [modules § the rule](10-modules.md#the-rule) |
| **adapter** (layer) | the one implementation of a port; the only layer allowed outside I/O | [modules § the rule](10-modules.md#the-rule) |
| **face** | an entry point — API, MCP, WEB, CLI — with no domain logic | [modules § the rule](10-modules.md#the-rule) |

## CLI verbs

| Group | Verbs |
|---|---|
| identity | `keygen`, `register` |
| registry | `register`, `topic create`, `ls`, `service-template`, `status` |
| messaging | `send`, `call`, `publish`, `consume`, `ack`, `reply` |
| runner | `start`, `stop`, `restart`, `reload`, `enable`, `disable`, `logs` — systemd's semantics; **`start`, never `run`**, and `reload` only where a child is kept |
| AUTH admin | `auth sign`, `auth admin` |
| over SSH | `static-token`, `bundle show\|history`, `user list`, `service list`, `replica-sync` |
