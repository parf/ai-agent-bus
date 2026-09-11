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
| A service name is its address and its inbox | [identity § names](01-identity.md#names) |
| Where the host is split off, and how wide an instance name may be | [identity § names](01-identity.md#names) |
| One service on many hosts, and scatter-gather over them, is Release 1 | [stages § release 1](12-stages.md#release-1) |
| A service is always configured; the unconfigured capability is a service template | [services § service and template](03-services-and-topics.md#service-and-template) |
| Config is arbitrary and separate from the name; nothing is parsed out of an address | [services § service and template](03-services-and-topics.md#service-and-template) |
| One verb configures a template and reads that configuration back | [services § configuring a template](03-services-and-topics.md#configuring-a-template) |
| What a configuration is, who may write it, and where it never appears | [services § configuring a template](03-services-and-topics.md#configuring-a-template) |
| A configuration is private to its service; a query gets a digest of it | [services § configuring a template](03-services-and-topics.md#configuring-a-template) |
| Why the digest exists: detecting a changed setup without seeing it | [services § why a digest at all](03-services-and-topics.md#why-a-digest-at-all) |
| MCP tool info stored raw, shape-checked | [services § service and template](03-services-and-topics.md#service-and-template) |
| Destructive methods are a hint in the description, enforced by nobody | [services § service and template](03-services-and-topics.md#service-and-template) |
| Topics are first-class records; kind, TTL, bound, overflow declared at creation | [services § topics](03-services-and-topics.md#topics) |
| Peer registry sync is git push/pull on start; newer record wins per entry | [services § registry sync](03-services-and-topics.md#registry-sync) |
| Chaining queries an upstream, never replicates it | [overview § chaining](00-overview.md#chaining) |
| Per-agent queue on start; the address outlives the process | [messaging § inbox queues](04-messaging.md#inbox-queues) |
| `send` to a known receiver, `publish` to a topic, `consume` your own queue | [messaging § verbs](04-messaging.md#verbs) |
| `message_id` unique per channel; topic + tag; reply-to | [messaging § message fields](04-messaging.md#message-fields) |
| Optional receipts: `ack` and `done` | [messaging § receipts](04-messaging.md#receipts) |
| Optional TTL per message; expired is dropped and counted | [messaging § message TTL](04-messaging.md#message-ttl) |
| Consumers pull by default, may register a push address | [messaging § push and pull](04-messaging.md#push-and-pull) |
| Two overflow modes, declared per record | [messaging § overflow](04-messaging.md#overflow) |
| Graceful restart dumps queues and stats to Parquet; optional periodic dump | [messaging § durability](04-messaging.md#durability) |
| No message kinds and no receiver policy in the bus | [messaging § envelope](04-messaging.md#envelope) |
| Dashboard shows services, topics and call counts — envelopes only | [discovery § dashboard](05-discovery.md#dashboard) |
| Admin-only debug trace per service | [discovery § debug mode](05-discovery.md#debug-mode) |
| AUTH merged into `agent-busd` as an optional role; WEB child cgroup-limited | [overview § roles](00-overview.md#roles) |
| Bundle in git over SSH; gaps resolved newer-generation-wins; master/slave | [AUTH role § topology](06-auth-role.md#topology) |
| `master_secret` is an out-of-band file | [AUTH role § where it runs](06-auth-role.md#where-it-runs) |
| Admin keys live in the bundle; root on the box is the break-glass | [AUTH role § SSH admin](06-auth-role.md#ssh-admin) |
| Minimal billing as an optional role: RADIUS balance, flat or per-call, no balance = denied | [future/billing.md](future/billing.md) |
| Paid public API platform; the payment gateway is an ordinary bus service | [future/billing.md](future/billing.md) |
| One push adapter per agent runtime; ChatGPT pull-only | [runner § adapters](08-runner-role.md#adapters) |
| Sandboxing on by default, backend chosen by environment | [runner § sandboxing](08-runner-role.md#sandboxing) |
| Minimal setup: install, `agent-bus setup`, start the service | [setup § install](09-setup.md#install) |
| Development goes PoC → MVP → Release 1, each ending in something that works end to end | [stages](12-stages.md) |
| PoC: sockets + HTTP, one master token issued over SSH, a small set of CLI verbs, a basic MCP face, no npm | [stages § PoC](12-stages.md#poc) |
| A service call is a `send` whose reply comes back on the same topic and tag; the bus adds no call machinery | [messaging § request and reply](04-messaging.md#request-and-reply) |
| PoC includes basic service support: consume, `ack`, reply, and a caller that waits | [stages § PoC](12-stages.md#poc) |
| PoC has no encrypted sessions at all — bodies plaintext; SSH-issued tokens stay because they cost nothing | [stages § PoC](12-stages.md#poc) |
| Write the simple version first, compare with V1, take its solution where it is better; simplicity breaks the tie | [stages § PoC](12-stages.md#poc) |
| Names are canonical, bounded, and one spelling each | [identity § names](01-identity.md#names) |
| A shell script is a service: `start --algo=std\|args [-N]`, stdout is the reply, no bus code in the script | [runner § script services](08-runner-role.md#script-services) |
| V2 code lives in this repo, in `src/` beside `docs/` | [stages § PoC](12-stages.md#poc) |
| `consume` is at-most-once: handed over and gone, with the loss on a crash documented | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |
| The daemon keeps no reply state; a client replies from what it consumed, and `reply` is sugar over the routing fields | [messaging § reply routing](04-messaging.md#reply-routing) |
| Billing is deferred out of every stage, design intact | [future/billing.md](future/billing.md) |
| One outstanding unfiltered read per inbox, with filtered waiters served ahead of it; process ownership is convention, not enforcement | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |
| A PoC daemon binds loopback or an SSH tunnel, never a public interface | [stages § PoC](12-stages.md#poc) |
| A queue topic is an inbox with a name, read by `consume --topic` | [stages § PoC](12-stages.md#poc) |
| Pub/sub waits for MVP: fan-out is cheap, but a subscription is an ACL capability and PoC has no ACL | [stages § PoC](12-stages.md#poc) |
| Both listeners speak HTTP and JSON; `consume` long-polls | [messaging § push and pull](04-messaging.md#push-and-pull) |
| The TypeScript packages run on bun; the Codex App Server is reached over loopback when shared, and spawned on stdio when not | [runner § adapters](08-runner-role.md#adapters) |
| Go for protocol, core and the CLI; TypeScript for the MCP face and the push adapters; client libs Go, PHP, Rust, JS, Python | [modules § languages](10-modules.md#languages) |
| No verb exists only in a face: every CLI and MCP operation is first a core API | [modules § languages](10-modules.md#languages) |
| Do not reinvent the wheel: built-in first, then the system's tool, then a well-known library, never our own | [modules § external tools](10-modules.md#external-tools) |
| HTTP is a first-class citizen and always the built-in client, never a subprocess — unless the caller is a script | [modules § HTTP is built in](10-modules.md#http-is-built-in) |
| The per-message hot path stays in-process: a well-known library, never our own primitives | [modules § the hot path](10-modules.md#the-hot-path) |
| Plumbing written a third time becomes one small internal module, preferred over a dependency | [modules § our own small module](10-modules.md#our-own-small-module) |
| A registration never carries a configuration or its digest; the digest is derived | [services § configuring a template](03-services-and-topics.md#configuring-a-template) |
| An inbox belongs to a registered name: consuming as an unregistered one is refused | [messaging § inbox queues](04-messaging.md#inbox-queues) |
| A registration says how to call it; no protocol means an ordinary bus service | [services § how to call it](03-services-and-topics.md#how-to-call-it) |
| `/etc/services` is the suggested protocol vocabulary and is never enforced | [services § how to call it](03-services-and-topics.md#how-to-call-it) |
| A query says whether anything is serving a name, as live state that is never stored | [discovery § what a listing answers](05-discovery.md#what-a-listing-answers) |
| Modular by layer: protocol, ports, core, adapters, faces; dependencies point inward | [modules § the rule](10-modules.md#the-rule) |
| Process layout follows systemd: a supervisor that holds nothing, plus small single-task children | [processes § the rule](11-processes.md#the-rule) |
| Each child gets the narrowest privilege its task needs, declared not acquired | [processes § the processes](11-processes.md#the-processes) |
| Nothing is shared implicitly — children talk over unix sockets with explicit contracts | [processes § what is shared](11-processes.md#what-is-shared) |
| The supervisor owns the listening sockets and passes fds down | [processes § the rule](11-processes.md#the-rule) |
| The runner is its own process: the only component that execs code it did not write | [processes § why the runner is its own process](11-processes.md#why-the-runner-is-its-own-process) |
| `CAP_CHOWN` is the supervisor's alone, so no long-running child holds a capability | [processes § why the supervisor holds CAP_CHOWN](11-processes.md#why-the-supervisor-holds-cap_chown) |
| Every external dependency sits behind a port, so it is replaced by writing one adapter | [modules § the rule](10-modules.md#the-rule) |
| Only adapters touch the outside world; calling an external tool is an adapter-layer rule | [modules § external tools](10-modules.md#external-tools) |
| `protocol` is the layer the client libraries reimplement, and depends on nothing | [modules § the rule](10-modules.md#the-rule) |
| No external broker; `agent-busd` is the broker | [overview § goal](00-overview.md#goal) |
| V1 leftovers (RAG, KV/DB gateways, writers) deferred, non-core — nothing to design | — |
| A registered topic named alone is an inbox to read; with a tag it is a filter — one rule for every face | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |
| A name-shaped topic that is registered nowhere is refused, not read as a filter | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |
| The filter is a priority, not a lease: it holds only while its wait is outstanding | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |
| A caller states its own record before it calls, and only if it has none | [messaging § request and reply](04-messaging.md#request-and-reply) |
| A receipt is a closed set of two words | [messaging § receipts](04-messaging.md#receipts) |
| The receipts answer "picked up, or lost?", so the bus keeps no delivery journal | [messaging § receipts](04-messaging.md#receipts) |
| A queue belongs to a name, never to a connection or session | [messaging § inbox queues](04-messaging.md#inbox-queues) |
| A full queue refuses by default, and a drop is counted | [messaging § overflow](04-messaging.md#overflow) |
| A request that expects a reply must name a registered reply address; registered is not live | [messaging § request and reply](04-messaging.md#request-and-reply) |
| A send to a name with no record is refused, never accepted and dropped later | [messaging § verbs](04-messaging.md#verbs) |
| `--wait` is the caller's deadline and bounds the HTTP exchange, not only the daemon's wait | [messaging § request and reply](04-messaging.md#request-and-reply) |
| A script service is the name it registered — it reads and answers as that name | [runner § script services](08-runner-role.md#script-services) |
| A script service takes a message only when a worker is free; stopping waits for the running ones | [runner § script services](08-runner-role.md#script-services) |

## Open

| ❓ | Settled by | Where |
|---|---|---|
| Static sessions are not end-to-end against the daemon | owner | [access § encrypted sessions](02-access.md#encrypted-sessions) |
| Per-method pricing needs the method name in the envelope | owner | [future/billing.md](future/billing.md) |
| A newcomer with no balance cannot reach `pay` | owner | [future/billing.md](future/billing.md) |
| Direct talk bypasses billing | owner | [future/billing.md](future/billing.md) |
| `authorized_keys` regeneration would drop the setup-installed token key | owner | [AUTH role § SSH admin](06-auth-role.md#ssh-admin) |
| Peer sync trusts unsigned records; no clock authority for "newer wins" | owner | [services § registry sync](03-services-and-topics.md#registry-sync) |
| With AUTH off, who filters the MCP catalog? | owner | [discovery § audience](05-discovery.md#audience) |
| What else lives in SQLite | owner | [setup § storage](09-setup.md#storage) |
| npm install vs Go-first for the first release | owner | [setup § install](09-setup.md#install) |
| OpenCode (Z.AI) push path | one spike | [runner § adapters](08-runner-role.md#adapters) |
| How `protocol` is specified for five client languages | owner, with data models | [modules](10-modules.md) |
| Which process owns the store handle | owner | [processes § what is shared](11-processes.md#what-is-shared) |
| MVP and Release 1 contents | owner | [stages](12-stages.md) |
| What happens to a running service when its configuration changes | owner | [services § configuring a template](03-services-and-topics.md#configuring-a-template) |
| A chaining namespace and a service template both want the `/` | owner, with chaining | [overview § chaining](00-overview.md#chaining) |
| Whether reading an inbox and filtering one become separate options | owner, with the MVP CLI | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |

## Superseded

| Was | Now |
|---|---|
| Go first, a bun/NPM build later | Go inside, TypeScript for the MCP face and adapters — built by bun, run on Node — [modules § languages](10-modules.md#languages) |
| TypeScript runs on Node because bun's WebSocket fails on a unix socket | bun, reaching the App Server over stdio instead — the WebSocket was the only thing that needed Node — [modules § languages](10-modules.md#languages) |
| Only stdio reaches the Codex App Server; the WebSocket is not needed | both are used — a loopback WebSocket to a shared app-server, stdio to a spawned one. Only a WebSocket over a *unix socket* is out — [runner § adapters](08-runner-role.md#adapters) |
| The runner is part of the core | its own process — it is the one component that execs code it did not write — [processes](11-processes.md) |
| A token lives in daemon memory and dies with a restart | tokens are saved, and the previous one is kept — otherwise a reloaded queue is undecryptable ciphertext — [access § token lifetime](02-access.md#token-lifetime) |
| The token is the whole identity in "minimal mode" | a call always carries two parameters; the socket supplies them locally — [access § two parameters](02-access.md#two-parameters) |
| A name never holds a slash: a name is not a path | one slash is allowed, between service template and instance name — the realm still never holds one — [identity § names](01-identity.md#names) |
| "Service" = the kind, "instance" = a running copy of it | "service" means the **configured** thing; the kind is a **service template** — [services § service and template](03-services-and-topics.md#service-and-template) |
| Local access needs no credential at all | the socket hides the credentials, it does not remove them — [access § local socket](02-access.md#local-socket) |
| Socket in each user's `/run/user/<uid>/` | the daemon's own directory, one socket per user — [access § local socket](02-access.md#local-socket) |
| Setup takes local username + gh-username | local account + bus username; the username carries its realm — [setup § local users](09-setup.md#local-users) |
| Principal id is GitHub's numeric id | the name is the identity; the id is only a re-check comparison — [identity § names](01-identity.md#names) |
| GitHub is *the* identity source, and the reason public services work | a provider is an alternative to typing the record — [identity § registration](01-identity.md#registration) |
| Overflow: drop oldest | two modes, `ring` and `strict` — [messaging § overflow](04-messaging.md#overflow) |
| `ring` is the default mode | `strict` is: a queue that loses work silently is worse than one that fails visibly — [messaging § overflow](04-messaging.md#overflow) |
| Audience with AUTH off is a per-service `user: token` map | the two ACL layers — [identity § acl](01-identity.md#acl) |
| LDAP/AD in scope | deferred — [future](future/ldap-ad.md) |
| NATS · Redis Streams · AUTH-signed JWT keys | dropped; kept in `legacy/` for history |
