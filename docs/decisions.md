# Decisions

An **index**, not a source of truth: each row names a decision and links to the
section that states it. If you can learn the rule from a row here, the row is
too long.

Adding a decision is still two edits — the substance into the doc it belongs
to, one row here. Revising one means changing the doc, adding a new row, and
moving the old row to [Superseded](#superseded).

## Settled

All 2026-09-09 unless noted.

| Decision | Where |
|---|---|
| Names are `user@realm`; the name is the identity, provider ids are only a check | [identity § names](01-identity.md#names) |
| Registration is a stated record; a provider is an alternative to typing it and is not needed after enrolment | [identity § registration](01-identity.md#registration) |
| MVP is manual registration + GitHub; LDAP/AD deferred | [identity § registration](01-identity.md#registration) · [future](future/ldap-ad.md) |
| Self-service enrolment: open (auto, minimal role) or closed (approval queue) | [identity § registration](01-identity.md#registration) |
| More identity sources later: Google, LinkedIn, Facebook — not designed | [identity § registration](01-identity.md#registration) |
| ACL is two layers: service config first, then master ACL; a service may refuse master access; `*:` covers the rest | [identity § acl](01-identity.md#acl) |
| `allow: *` means anyone who can authenticate | [identity § acl](01-identity.md#acl) |
| The setup user gets `agent-bus-admin` | [identity § acl](01-identity.md#acl) |
| Delegation: A authenticates, adds an on-behalf-of claim | [identity § delegation](01-identity.md#delegation) |
| Publish a service or topic: any authenticated principal; change: owner or owner group | [identity § ownership](01-identity.md#ownership) |
| Registry records are writer-signed where a key exists; static-token writes are unsigned | [identity § ownership](01-identity.md#ownership) |
| Sealed private config, opt-in, daemon cannot read it | [identity § sealed private config](01-identity.md#sealed-private-config) |
| A call carries exactly two parameters, `user@realm` + token | [access § two parameters](02-access.md#two-parameters) |
| Two ways to get a token: over SSH, or `register` on the box; both need machine access | [access § getting a token](02-access.md#getting-a-token) |
| Tokens are persisted and the previous one is kept; local default never expires | [access § token lifetime](02-access.md#token-lifetime) |
| The socket hides the two fields, it does not replace them; one host, many users | [access § local socket](02-access.md#local-socket) |
| Socket layout, ownership and the one capability it needs | [access § local socket](02-access.md#local-socket) |
| Three key modes: static, pairwise, derived | [access § key modes](02-access.md#key-modes) |
| No forward secrecy | [access § encrypted sessions](02-access.md#encrypted-sessions) |
| Bodies end-to-end encrypted; `encryption: off` per service for development | [access § encrypted sessions](02-access.md#encrypted-sessions) |
| Wire is JSON, msgpack optional | [access § encrypted sessions](02-access.md#encrypted-sessions) |
| Wrong key at handshake: re-query AUTH once, then alert loudly | [access § key confirmation](02-access.md#key-confirmation) |
| Instance id = address = `unique-name@host` | [services § service and instance](03-services-and-topics.md#service-and-instance) |
| MCP tool info stored raw, shape-checked | [services § service and instance](03-services-and-topics.md#service-and-instance) |
| Destructive methods are a hint in the description, enforced by nobody | [services § service and instance](03-services-and-topics.md#service-and-instance) |
| Topics are first-class records; kind, TTL, bound, overflow declared at creation | [services § topics](03-services-and-topics.md#topics) |
| Peer registry sync is git push/pull on start; newer record wins per entry | [services § registry sync](03-services-and-topics.md#registry-sync) |
| Chaining queries an upstream, never replicates it | [overview § chaining](00-overview.md#chaining) |
| Per-agent queue on start; the address outlives the process | [messaging § inbox queues](04-messaging.md#inbox-queues) |
| `send` to a known receiver, `publish` to a topic, `consume` your own queue | [messaging § verbs](04-messaging.md#verbs) |
| `message_id` unique per channel; topic + tag; reply-to | [messaging § message fields](04-messaging.md#message-fields) |
| Optional receipts: `ack` and `done` | [messaging § receipts](04-messaging.md#receipts) |
| Optional TTL per message; expired is dropped and counted | [messaging § message TTL](04-messaging.md#message-ttl) |
| Consumers pull by default, may register a push address | [messaging § push and pull](04-messaging.md#push-and-pull) |
| Two overflow modes per topic: `ring` (default) and `strict` | [messaging § overflow](04-messaging.md#overflow) |
| Graceful restart dumps queues and stats to Parquet; optional periodic dump | [messaging § durability](04-messaging.md#durability) |
| No message kinds and no receiver policy in the bus | [messaging § envelope](04-messaging.md#envelope) |
| Dashboard shows services, topics and call counts — envelopes only | [discovery § dashboard](05-discovery.md#dashboard) |
| Admin-only debug trace per service | [discovery § debug mode](05-discovery.md#debug-mode) |
| AUTH merged into `agent-busd` as an optional role; WEB child cgroup-limited | [overview § roles](00-overview.md#roles) |
| Bundle in git over SSH; gaps resolved newer-generation-wins; master/slave | [AUTH role § topology](06-auth-role.md#topology) |
| `master_secret` is an out-of-band file | [AUTH role § where it runs](06-auth-role.md#where-it-runs) |
| Admin keys live in the bundle; root on the box is the break-glass | [AUTH role § SSH admin](06-auth-role.md#ssh-admin) |
| Minimal billing as an optional role: RADIUS balance, flat or per-call, no balance = denied | [billing role](07-billing-role.md) |
| Paid public API platform; the payment gateway is an ordinary bus service | [billing role](07-billing-role.md) |
| One push adapter per agent runtime; ChatGPT pull-only | [runner § adapters](08-runner-role.md#adapters) |
| Sandboxing on by default, backend chosen by environment | [runner § sandboxing](08-runner-role.md#sandboxing) |
| Minimal setup: install, `agent-bus setup`, start the service | [setup § install](09-setup.md#install) |
| Go first, bun/NPM later; client libs Go, PHP, Rust, JS, Python | [setup § install](09-setup.md#install) |
| Thin glue to external systems: shell out to the standard client | [overview § principles](00-overview.md#principles) |
| Modular by layer: protocol, ports, core, adapters, faces; dependencies point inward | [modules § the rule](10-modules.md#the-rule) |
| Process layout follows systemd: a supervisor that holds nothing, plus small single-task children | [processes § the rule](11-processes.md#the-rule) |
| Each child gets the narrowest privilege its task needs, declared not acquired | [processes § the processes](11-processes.md#the-processes) |
| Nothing is shared implicitly — children talk over unix sockets with explicit contracts | [processes § what is shared](11-processes.md#what-is-shared) |
| The supervisor owns the listening sockets and passes fds down | [processes § the rule](11-processes.md#the-rule) |
| The runner is its own process: the only component that execs code it did not write | [processes § why the runner is its own process](11-processes.md#why-the-runner-is-its-own-process) |
| `CAP_CHOWN` is the supervisor's alone, so no long-running child holds a capability | [processes § why the supervisor holds CAP_CHOWN](11-processes.md#why-the-supervisor-holds-cap_chown) |
| Every external dependency sits behind a port, so it is replaced by writing one adapter | [modules § the rule](10-modules.md#the-rule) |
| Only adapters touch the outside world; thin glue is an adapter-layer rule | [modules § where thin glue lands](10-modules.md#where-thin-glue-lands) |
| `protocol` is the layer the client libraries reimplement, and depends on nothing | [modules § the rule](10-modules.md#the-rule) |
| No external broker; `agent-busd` is the broker | [overview § goal](00-overview.md#goal) |
| V1 leftovers (RAG, KV/DB gateways, writers) deferred, non-core — nothing to design | — |

## Open

| ❓ | Settled by | Where |
|---|---|---|
| Static sessions are not end-to-end against the daemon | owner | [access § encrypted sessions](02-access.md#encrypted-sessions) |
| Per-method pricing needs the method name in the envelope | owner | [billing role](07-billing-role.md) |
| A newcomer with no balance cannot reach `pay` | owner | [billing role](07-billing-role.md) |
| Direct talk bypasses billing | owner | [billing role](07-billing-role.md) |
| `authorized_keys` regeneration would drop the setup-installed token key | owner | [AUTH role § SSH admin](06-auth-role.md#ssh-admin) |
| Peer sync trusts unsigned records; no clock authority for "newer wins" | owner | [services § registry sync](03-services-and-topics.md#registry-sync) |
| With AUTH off, who filters the MCP catalog? | owner | [discovery § audience](05-discovery.md#audience) |
| What else lives in SQLite | owner | [setup § storage](09-setup.md#storage) |
| npm install vs Go-first for the first release | owner | [setup § install](09-setup.md#install) |
| OpenCode (Z.AI) push path | one spike | [runner § adapters](08-runner-role.md#adapters) |
| How `protocol` is specified for five client languages | owner, with data models | [modules](10-modules.md) |
| Which process owns the store handle | owner | [processes § what is shared](11-processes.md#what-is-shared) |

## Superseded

| Was | Now |
|---|---|
| The runner is part of the core | its own process — it is the one component that execs code it did not write — [processes](11-processes.md) |
| A token lives in daemon memory and dies with a restart | tokens are saved, and the previous one is kept — otherwise a reloaded queue is undecryptable ciphertext — [access § token lifetime](02-access.md#token-lifetime) |
| The token is the whole identity in "minimal mode" | a call always carries two parameters; the socket supplies them locally — [access § two parameters](02-access.md#two-parameters) |
| Local access needs no credential at all | the socket hides the credentials, it does not remove them — [access § local socket](02-access.md#local-socket) |
| Socket in each user's `/run/user/<uid>/` | the daemon's own directory, one socket per user — [access § local socket](02-access.md#local-socket) |
| Setup takes local username + gh-username | local account + bus username; the username carries its realm — [setup § local users](09-setup.md#local-users) |
| Principal id is GitHub's numeric id | the name is the identity; the id is only a re-check comparison — [identity § names](01-identity.md#names) |
| GitHub is *the* identity source, and the reason public services work | a provider is an alternative to typing the record — [identity § registration](01-identity.md#registration) |
| Overflow: drop oldest | two modes, `ring` and `strict` — [messaging § overflow](04-messaging.md#overflow) |
| Audience with AUTH off is a per-service `user: token` map | the two ACL layers — [identity § acl](01-identity.md#acl) |
| LDAP/AD in scope | deferred — [future](future/ldap-ad.md) |
| NATS · Redis Streams · AUTH-signed JWT keys | dropped; kept in `legacy/` for history |
