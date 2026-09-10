# Glossary

Every name and term, one line each, with the section that defines it. This file
is **normative for naming**: if a name here disagrees with prose elsewhere, this
wins.

## Names that are enforced

| Name | Is | Never |
|---|---|---|
| `agent-busd` | *the* daemon (`sshd`/`dockerd` convention) | not "the server", not `agentbusd` |
| `agent-bus` | the CLI | — |
| `agent-bus` | the keyword everywhere else: `/etc/agent-bus/`, `~/.config/agent-bus/`, the `agent-bus` system user, `agent-busd.service` | — |
| `ab_` | **MCP tool prefix only** (`ab_list_services`, `ab_call`) | never in CLI, config or prose |

## Vocabulary

| Say | Not | Why |
|---|---|---|
| service discovery | registration | registration is one operation on discovery |
| personal / shared | user-scoped/system, bound/unbound, single-/multi-tenant | shared is the default |
| realm | domain, provider, tenant | the right-hand side of `user@realm` |

## Terms

| Term | One line | Defined in |
|---|---|---|
| **principal** | any user, service or instance with an identity | [identity § principals](01-identity.md#principals) |
| **`user@realm`** | how every principal is written; realm = host · provider · team | [identity § names](01-identity.md#names) |
| **realm** | who vouches for a name | [identity § names](01-identity.md#names) |
| **token** | the second of the two parameters every call carries; persisted, previous one kept | [access § token lifetime](02-access.md#token-lifetime) |
| **service ACL / master ACL** | the two access layers, service asked first | [identity § acl](01-identity.md#acl) |
| **`allow: *`** | anyone who can authenticate | [identity § acl](01-identity.md#acl) |
| **`agent-bus-admin`** | the master-ACL role the setup user gets | [identity § acl](01-identity.md#acl) |
| **role** | service-defined string saying what a principal may do | [identity § groups and roles](01-identity.md#groups-and-roles) |
| **delegation / on-behalf-of** | A calls B for U, carrying a claim | [identity § delegation](01-identity.md#delegation) |
| **generic · agent · consumer · publisher** | the service kinds | [services § service kinds](03-services-and-topics.md#service-kinds) |
| **service vs instance** | the kind vs a running copy, `unique-name@host` | [services § service and instance](03-services-and-topics.md#service-and-instance) |
| **topic** | a registered record; kind `queue` or `pub/sub` | [services § topics](03-services-and-topics.md#topics) |
| **inbox** | the implicit queue topic every agent owns | [messaging § inbox queues](04-messaging.md#inbox-queues) |
| **`message_id` · topic · tag** | the three fields on every message | [messaging § message fields](04-messaging.md#message-fields) |
| **ack / done** | the two optional receipts | [messaging § receipts](04-messaging.md#receipts) |
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
| **bus** | the child that is the core: registry, queues, sessions, delivery | [processes](11-processes.md) |
| **port** | an interface core depends on; the seam a dependency is swapped at | [modules § the rule](10-modules.md#the-rule) |
| **adapter** (layer) | the one implementation of a port; the only layer allowed outside I/O | [modules § the rule](10-modules.md#the-rule) |
| **face** | an entry point — API, MCP, WEB, CLI — with no domain logic | [modules § the rule](10-modules.md#the-rule) |

## CLI verbs

| Group | Verbs |
|---|---|
| setup | `setup` |
| identity | `keygen`, `register` |
| registry | `register`, `topic create` |
| messaging | `send`, `publish`, `consume` |
| runner | `start`, `stop`, `ls`, `logs` |
| AUTH admin | `auth sign`, `auth admin` |
| over SSH | `static-token`, `bundle show\|history`, `user list`, `service list`, `status`, `replica-sync` |
