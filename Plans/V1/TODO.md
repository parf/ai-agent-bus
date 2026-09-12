# TODO — V1

**Not started.** The active plan is [Plans/MVP/TODO.md](../MVP/TODO.md); this
file exists so that work displaced from the MVP lands somewhere named rather
than in a comment. Stable knowledge for this stage is [README.md](README.md),
the design is [docs/](../../docs/00-overview.md), and open questions and
settled decisions live in [decisions](../../docs/decisions.md).

**Objective**: V1 as scoped in
[stages § release 1](../../docs/12-stages.md#release-1) — a team or a company
can run it, and it can be exposed.

**Next step**: the owner's. Nothing here is planned into waves yet, because
waves are the thing this repo plans *last* — the scope above is still
*proposed*, and the MVP is not finished.

## Carried in from the MVP

Each was cut deliberately, with a reason that is on the record. They are the
first candidates for this stage's waves because their shape is already
settled.

| What | Why it was cut | Where it is designed |
|---|---|---|
| **Bodies end to end** — wave D in full | the MVP's key mode cannot carry the claim: the daemon issues the token a session key would derive from | [Plans/MVP/TODO.md](../MVP/TODO.md#d--the-bus-stops-reading-payloads), [access § encrypted sessions](../../docs/02-access.md#encrypted-sessions) |
| **Many names addressed together** — several services sharing a template, asked at once, and the scatter-gather that needs. Not a **pool**, which is many processes behind one name | the owner's word; the PoC and MVP address one service at a time | [services § service and template](../../docs/03-services-and-topics.md#service-and-template) |
| **Groups with `& \| !`, and roles** | groups exist only with the AUTH role on, and a role is service-defined and never interpreted here | [identity § groups and roles](../../docs/01-identity.md#groups-and-roles) |
| **The dashboard's groups, health, load graphs, child liveness and record origin** | each waits on the thing that would make it true — AUTH, the health child, the ring buffers, the supervisor reporting in, chained registries — not on the page | [discovery § what it shows](../../docs/05-discovery.md#what-it-shows) |
| **A second sandbox backend** | one backend plus off is what the MVP builds; `bwrap` is one adapter behind the same port the day a host has no systemd | [runner § sandboxing](../../docs/08-runner-role.md#sandboxing) |
| **Phone numbers and IM handles on a person** | contact routes nothing on the bus uses | [identity § registration](../../docs/01-identity.md#registration) |
| **A one-time sign-in code, and a dashboard-scoped credential** | the MVP signs in with the token a person already has; narrowing what the browser holds is the next move, not the first | [discovery § signing in](../../docs/05-discovery.md#signing-in) |

## The stage's own scope

Straight from [stages § release 1](../../docs/12-stages.md#release-1), which
owns it — listed here so that a wave has something to be cut from, not
restated in any more detail than that.

| | |
|---|---|
| AUTH role | bundle, signed generations, git over SSH, 2+ replicas |
| policy | groups, service-defined roles, delegation |
| admin | SSH forced commands, audit log |
| federation | chaining upstream; peer registry sync through git |
| calls | a call reaches an **upstream** service the way a local one is reached, carrying on-behalf-of; long answers stream |
| observability | health checker, stats, Prometheus export |
| **locks** | named locks in the daemon: `lock` (ttl, waits), `try-lock` (does not), `release`; a grant number that only goes up, and nothing kept across a restart |
| secrets | sealed private config |
| credentials | per-principal-per-service tokens, replacing the MVP's master token |
| **the runner** | `agent-bus-runner`: its own account, installed instances under `runner/`, write-only configuration, autostart and restart policy, on-demand start, start order |
| script forms | `std` bytes on stdin; `jsonl` and `msgpack`, a child kept across messages with a deadline and `reload` |
| what is installed | `services.json`: every installed service, `autostart` on/off/on-demand, run options, `depends` (author's, overridden per host), and the version, origin and start dates the runner keeps |
| backup and restore | one encrypted archive of `runner/`; `service.d` is re-fetched from the recorded origins, not backed up |
| service pools | `start --share`: one name over many hosts, its members given a complete name in a realm the daemon holds; a bare name still completed with the local host |
| where a member is | a hostname stated as its own field at registration, answered in a listing as `on` |
| a service's version | the record says what was registered, a `version` call what is running; a simple service declares it in `config.json` and the runner answers |
| **the runner's page** | the dashboard drives a runner: everything it manages, separated into installed · enabled · running, the verbs on each, and installing a new instance from the browser — the first page with controls on it, and they post as the person ([discovery § what it shows](../../docs/05-discovery.md#what-it-shows)) |
| encryption | AEAD sessions, bodies end to end |
| clients | Go, PHP, Rust, JS, Python — gated on how `protocol` is specified |
| operations | zero-downtime reload, packaging |

## Blockers

Every one is already an indexed open decision
([decisions § open](../../docs/decisions.md#open)); they are here because this
stage is where each of them bites.

| Gates | ❓ | Where it is settled |
|---|---|---|
| the whole stage | Release 1 contents | [stages](../../docs/12-stages.md) |
| encryption | how a queued body is decrypted by a receiver that was not present when it was sent | [access § encrypted sessions](../../docs/02-access.md#encrypted-sessions) |
| clients | how `protocol` is specified for five client languages | [modules](../../docs/10-modules.md) |
| federation | peer sync trusts unsigned records, and there is no clock authority for "newer wins" | [services § registry sync](../../docs/03-services-and-topics.md#registry-sync) |
| chaining, and service templates | a chaining namespace and a service template both want the `/` | [overview § chaining](../../docs/00-overview.md#chaining) |
| admin | `authorized_keys` regeneration would drop the key `agent-bus-setup` installed | [AUTH role § SSH admin](../../docs/06-auth-role.md#ssh-admin) |
| the runner | whether one kept child may have several messages in flight | [runner § long-lived services](../../docs/08-runner-role.md#long-lived-services) |
| a service's version | `version` is a **standard method**, and nothing on a record carries method information yet | [services § service and template](../../docs/03-services-and-topics.md#service-and-template) |
| backup | whether a backup is a runner verb, a bundled service, or neither — the archive and the manifest are the same either way | [runner § backing it up](../../docs/08-runner-role.md#backing-it-up) |
