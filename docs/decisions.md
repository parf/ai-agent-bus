# Decisions

An **index**, not a source of truth: each row names a decision and links to the
section that states it. If you can learn the rule from a row here, the row is
too long.

Adding a decision is still two edits — the substance into the doc it belongs
to, one row here. Revising one means changing the doc, adding a new row, and
moving the old row to [Superseded](#superseded).

## Settled

Dated where it matters; the runner split and what it touched is 2026-09-11.

| Decision | Where |
|---|---|
| Names are `user@realm`; the name is the identity, provider ids are only a check | [identity § names](01-identity.md#names) |
| What characters a name may hold, and that it starts alphanumeric | [identity § names](01-identity.md#names) |
| Setup installs the separate-user arrangement, and where the accounts live | [setup § the two accounts](09-setup.md#the-two-accounts) |
| One program per privilege, split no finer | [setup § the programs](09-setup.md#the-programs) |
| Over SSH a key reaches one forced command; the admin's is a superset, and the token verb is the same either way | [setup § the programs](09-setup.md#the-programs) |
| A token can be had by signing a challenge, because not every host runs sshd | [access § getting a token](02-access.md#getting-a-token) |
| Enrolment is the one route with no credential on it, because it is where one comes from | [identity § proving possession](01-identity.md#proving-possession) |
| A user is a line in `authorized_keys`, written by one program, never a format of ours | [setup § the programs](09-setup.md#the-programs) |
| The unit is what makes the arrangement true: the account, its home, one capability, restart | [setup § the two accounts](09-setup.md#the-two-accounts) |
| Writing is subject to the ACL, like reading | [identity § acl](01-identity.md#acl) |
| Registration is a stated record; a provider is an alternative to typing it and is not needed after enrolment | [identity § registration](01-identity.md#registration) |
| MVP is manual registration + GitHub; LDAP/AD deferred | [identity § registration](01-identity.md#registration) · [future](future/ldap-ad.md) |
| Self-service enrolment: open (auto, minimal role) or closed (approval queue) | [identity § registration](01-identity.md#registration) |
| More identity sources later: Google, LinkedIn, Facebook — not designed | [identity § registration](01-identity.md#registration) |
| ACL is two layers: the service's record first, then master ACL; a service may refuse master access; `*:` covers the rest | [identity § acl](01-identity.md#acl) |
| A sigil says what an ACL entry is: bare is a user, `@` a group, `#` a role | [identity § sigils](01-identity.md#sigils) |
| A user starts alphanumeric, which every name does — so a sigil is free to lead | [identity § sigils](01-identity.md#sigils) |
| A group is only ever an ACL subject; a role only ever reaches a service, `#`-stripped | [identity § sigils](01-identity.md#sigils) |
| A group is local to one daemon, because every ACL a daemon enforces is its own | [identity § sigils](01-identity.md#sigils) |
| A group never travels: an upstream decides with its own list, so two daemons may both have `@dev` | [identity § sigils](01-identity.md#sigils) |
| The daemon filters, because it holds the record — not a face | [discovery § audience](05-discovery.md#audience) |
| The MVP does not claim bodies are end to end; the bus is trusted on its own host | [access § encrypted sessions](02-access.md#encrypted-sessions) |
| Possession is proved in a step of its own; the `directory` port only fetches | [identity § proving possession](01-identity.md#proving-possession) |
| A realm with a directory behind it is enrolled into, never registered into | [identity § proving possession](01-identity.md#proving-possession) |
| An enrolled record is owned by the name itself, and the proof hands out its credential | [identity § proving possession](01-identity.md#proving-possession) |
| `allow: *` means anyone who can authenticate | [identity § acl](01-identity.md#acl) |
| Delegation: A authenticates, adds an on-behalf-of claim | [identity § delegation](01-identity.md#delegation) |
| Publish a service or topic: any authenticated principal; change: owner or owner group | [identity § ownership](01-identity.md#ownership) |
| Changing a record is the owner's, and the record's own; publishing a new name stays open | [identity § ownership](01-identity.md#ownership) |
| Registry records are writer-signed where a key exists; static-token writes are unsigned | [identity § ownership](01-identity.md#ownership) |
| Sealed private config, opt-in, daemon cannot read it | [identity § sealed private config](01-identity.md#sealed-private-config) |
| A call carries a token and no name — the token is the principal | [access § what a call carries](02-access.md#what-a-call-carries) |
| Two ways to get a token: over SSH, or `token` on the box; both need machine access | [access § getting a token](02-access.md#getting-a-token) |
| `token` is the credential verb, `register` the registry one | [access § getting a token](02-access.md#getting-a-token) |
| Over SSH the key names you, so a caller never states their own principal | [access § token scope](02-access.md#token-scope) |
| MVP tokens are master, one per principal; Release 1 scopes them per service so one cannot be replayed at another | [access § token scope](02-access.md#token-scope) |
| The daemon's owner may get a credential for any name; anyone else only for one they own | [access § getting a token](02-access.md#getting-a-token) |
| One socket per local account is a credential of its own; `status` says which name it used | [access § local socket](02-access.md#local-socket) |
| Tokens are persisted and the previous one is kept; local default never expires | [access § token lifetime](02-access.md#token-lifetime) |
| When a credential was issued is durable; when it was last used is this run's | [access § token lifetime](02-access.md#token-lifetime) |
| A caller may ask what credentials they hold and never anybody else's; owning a name is not holding one | [access § token lifetime](02-access.md#token-lifetime) |
| A record that owns itself is a person, so nothing carries a separate flag saying so | [identity § ownership](01-identity.md#ownership) |
| Credentials persist behind the store port, in a text file until the database is chosen | [setup § storage](09-setup.md#storage) |
| The restart snapshot carries the registry too, and is JSON until Parquet is written | [messaging § durability](04-messaging.md#durability) |
| A start that follows an unclean stop says so, and from when it is missing traffic | [messaging § durability](04-messaging.md#durability) |
| The socket is a credential, not an exemption from having one; one host, many users | [access § local socket](02-access.md#local-socket) |
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
| Call counts are per service, live state like `reading` and `queued` | [discovery § what a listing answers](05-discovery.md#what-a-listing-answers) |
| Loss is counted per inbox; the node's total is the sum of them | [discovery § what a listing answers](05-discovery.md#what-a-listing-answers) |
| A backlog reports the age of its oldest waiting message | [discovery § what a listing answers](05-discovery.md#what-a-listing-answers) |
| An unclean restart is a fact on `status`, not only a line in the log | [discovery § what it shows](05-discovery.md#what-it-shows) |
| Refusals are counted by reason, and a daemon fault is not one of them | [discovery § refusals](05-discovery.md#refusals) |
| Dashboard shows services, topics and call counts — envelopes only | [discovery § dashboard](05-discovery.md#dashboard) |
| The bus keeps a bounded feed of routed envelopes, body struck out where it is written | [discovery § dashboard](05-discovery.md#dashboard) |
| The envelope feed is filtered per caller — what you were party to, and the node's for master | [discovery § dashboard](05-discovery.md#dashboard) |
| The dashboard has a real hostname and a real certificate, from a public source | [discovery § where it listens](05-discovery.md#where-it-listens) |
| Admin-only debug trace per service | [discovery § debug mode](05-discovery.md#debug-mode) |
| The bus holds the store, and the supervisor holds nothing durable | [processes § what is shared](11-processes.md#what-is-shared) |
| One binary, the role from the environment; the supervisor opens every listener and hands it down | [processes § how a child is started](11-processes.md#how-a-child-is-started) |
| AUTH merged into `agent-busd` as an optional role; WEB child cgroup-limited | [overview § roles](00-overview.md#roles) |
| Bundle in git over SSH; gaps resolved newer-generation-wins; master/slave | [AUTH role § topology](06-auth-role.md#topology) |
| `master_secret` is an out-of-band file | [AUTH role § where it runs](06-auth-role.md#where-it-runs) |
| Admin keys live in the bundle; root on the box is the break-glass | [AUTH role § SSH admin](06-auth-role.md#ssh-admin) |
| Minimal billing as an optional role: RADIUS balance, flat or per-call, no balance = denied | [future/billing.md](future/billing.md) |
| Paid public API platform; the payment gateway is an ordinary bus service | [future/billing.md](future/billing.md) |
| One push adapter per agent runtime; ChatGPT pull-only | [runner § adapters](08-runner-role.md#adapters) |
| Development goes PoC → MVP → Release 1, each ending in something that works end to end | [stages](12-stages.md) |
| PoC: sockets + HTTP, one master token issued over SSH, a small set of CLI verbs, a basic MCP face, no npm | [stages § PoC](12-stages.md#poc) |
| A service call is a `send` whose reply comes back on the same topic and tag; the bus adds no call machinery | [messaging § request and reply](04-messaging.md#request-and-reply) |
| PoC includes basic service support: consume, `ack`, reply, and a caller that waits | [stages § PoC](12-stages.md#poc) |
| PoC has no encrypted sessions at all — bodies plaintext; SSH-issued tokens stay because they cost nothing | [stages § PoC](12-stages.md#poc) |
| Write the simple version first, compare with V1, take its solution where it is better; simplicity breaks the tie | [stages § PoC](12-stages.md#poc) |
| Names are canonical, bounded, and one spelling each | [identity § names](01-identity.md#names) |
| A shell script is a service: `start --algo=args\|std\|json\|jsonl\|msgpack [-N]`, stdout is the reply, no bus code in the script | [runner § script services](08-runner-role.md#script-services) |
| A form names a channel (`args`, `std`), a channel and its payload (`json`), or that payload repeated (`jsonl`) | [runner § script services](08-runner-role.md#script-services) |
| `std` is the body as bytes on stdin, so a binary service costs no base64 pass | [runner § script services](08-runner-role.md#script-services) |
| A long-lived child is a framing, not a flag: a stream form reads frame after frame, so the process is kept | [runner § long-lived services](08-runner-role.md#long-lived-services) |
| `msgpack` is `uint32` length + msgpack both ways — the envelope in-band, a binary body as bytes, and the process kept | [runner § script services](08-runner-role.md#script-services) |
| A kept child takes one message at a time, and needs the per-message deadline the others do not | [runner § long-lived services](08-runner-role.md#long-lived-services) |
| `start --share` puts a service in a pool spread over any number of hosts, passing the word `consume` already has | [runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts) |
| A name is up while any pool member is, and which member answered is nobody's business | [runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts) |
| That pool members are interchangeable is the operator's promise, not something the bus checks | [runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts) |
| A realm is the name a daemon answers for; a hostname is only its default, so a pool may have a realm of its own | [identity § names](01-identity.md#names) |
| A bare name is completed with the local host as a convenience that asserts nothing; a complete name is taken whole | [identity § names](01-identity.md#names) |
| A pool is one bus — members that report to different daemons are two queues, not one service | [runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts) |
| The directory name is the default name to register, and `autostart.json` may state a complete one instead | [runner § what an instance is](08-runner-role.md#what-an-instance-is) |
| A member states its hostname at registration: stated never observed, a label never an input, one entry per member | [discovery § where a member says it is](05-discovery.md#where-a-member-says-it-is) |
| Bundled services ship as ordinary services, and none of them may need a daemon change | [bundled services § rules they all obey](13-bundled-services.md#rules-they-all-obey) |
| No bundled service enforces access: the daemon refused the call before it arrived, which is why the tools stay small | [bundled services § rules they all obey](13-bundled-services.md#rules-they-all-obey) |
| Danger is a name, never a flag: read-only and read-write, shell and root-shell, are separate names because access is granted per name | [bundled services § rules they all obey](13-bundled-services.md#rules-they-all-obey) |
| Spend divides by name too: generation and embeddings are two gateways, not one with a verb | [bundled services § for the agents themselves](13-bundled-services.md#for-the-agents-themselves) |
| A backend is chosen by the host and is invisible; a gateway is chosen by the caller and is a name | [bundled services § for the agents themselves](13-bundled-services.md#for-the-agents-themselves) |
| An API gateway is minimal: it holds the key, counts, and never normalises one provider's API into another's — the access control was the daemon's before the call arrived | [bundled services § for the agents themselves](13-bundled-services.md#for-the-agents-themselves) |
| One contract across the bundled tools, and it is the envelope — addressing, ACL, declared surface, counters, version, failure. The body is the tool's own | [bundled services § one contract for the set](13-bundled-services.md#one-contract-for-the-set) |
| An upstream refusing is an answer, not a failure; no reply means the tool itself is broken | [bundled services § one contract for the set](13-bundled-services.md#one-contract-for-the-set) |
| Extending the set is an instance or a template, never a change to the contract | [bundled services § one contract for the set](13-bundled-services.md#one-contract-for-the-set) |
| No layer of ours between a caller and a tool: a retry or a cache is a service with its own name | [bundled services § rules they all obey](13-bundled-services.md#rules-they-all-obey) |
| The image is one image and two containers, because the two accounts are two secret domains | [stages § the image](12-stages.md#the-image) |
| The image ships the catalogue installed and enables only the reading half, using installed-not-enabled for what it is for | [stages § the image](12-stages.md#the-image) |
| `logwatch` keeps a bounded ring of cleaned lines per glob set, answers for the past and publishes the future, and makes the globs the grant | [bundled services § reading the box](13-bundled-services.md#reading-the-box) |
| Reading a channel publishes into a topic; sending is a call — one shape for Slack, SMS, mail and webhooks | [bundled services § people and the world outside](13-bundled-services.md#people-and-the-world-outside) |
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
| The runner is not part of `agent-busd`: a separate program under its own account, and no process the daemon starts may exec | [processes § nothing the daemon runs may exec](11-processes.md#nothing-the-daemon-runs-may-exec) |
| Two system accounts, one per secret domain, under one `/var/lib/agent-bus` | [setup § the two accounts](09-setup.md#the-two-accounts) |
| ssh with a forced command is a third way a caller is named, and how a remote daemon is reached | [access § the three doors](02-access.md#the-three-doors) |
| The runner is a service on the bus; deploying on a host is its service ACL, not a second door | [runner § reaching the runner](08-runner-role.md#reaching-the-runner) |
| The runner registers as `runner@<host>`, a name like any other | [runner § reaching the runner](08-runner-role.md#reaching-the-runner) |
| Two accounts, two units, started and stopped independently | [setup § the two units](09-setup.md#the-two-units) |
| Which bus the runner serves is a setting, defaulting to the local one | [setup § the two units](09-setup.md#the-two-units) |
| A local runner wants the local daemon and starts after it, but is not stopped with it | [setup § the two units](09-setup.md#the-two-units) |
| `service.d` is externally controlled — usually a checkout — so no local state lives in it | [runner § what an instance is](08-runner-role.md#what-an-instance-is) |
| Configuration is three env layers overlaid, and the more secret one wins | [runner § the three env layers](08-runner-role.md#the-three-env-layers) |
| `env.dist` declares the surface; a service needs an instance exactly when something is declared without a default | [runner § the three env layers](08-runner-role.md#the-three-env-layers) |
| A script under the runner holds no credential; a linked service holds one, as a variable in its env | [runner § what the child is told](08-runner-role.md#what-the-child-is-told) |
| The runner may never mint a credential; it asks for one for a name it owns, like anyone else | [runner § what the child is told](08-runner-role.md#what-the-child-is-told) |
| `config.json` owns the command line and travels with the code; `autostart.json` owns whether, how many and how confined | [runner § what an instance is](08-runner-role.md#what-an-instance-is) |
| Configuration is write-only: it is never handed back | [runner § reaching the runner](08-runner-role.md#reaching-the-runner) |
| A bus that is away is not a service that failed: the client reconnects, the runner restarts nothing | [runner § where it runs](08-runner-role.md#where-it-runs) |
| A directory is the installed state; installed, enabled and running are three states with one home each | [runner § what an instance is](08-runner-role.md#what-an-instance-is) |
| `reload` is `SIGHUP` to a kept child and refused on every other shape — the one place the runner cares what kind of child it has | [runner § what the runner does](08-runner-role.md#what-the-runner-does) |
| The verb is `start`, never `run` | [runner § what the runner does](08-runner-role.md#what-the-runner-does) |
| The runner is `agent-bus-runner` as a program and an account, and `runner` on the bus | [glossary § names that are enforced](glossary.md#names-that-are-enforced) |
| `CAP_CHOWN` is the supervisor's alone, so no long-running child holds a capability | [processes § why the supervisor holds CAP_CHOWN](11-processes.md#why-the-supervisor-holds-cap_chown) |
| Every external dependency sits behind a port, so it is replaced by writing one adapter | [modules § the rule](10-modules.md#the-rule) |
| Only adapters touch the outside world; calling an external tool is an adapter-layer rule | [modules § external tools](10-modules.md#external-tools) |
| `protocol` is the layer the client libraries reimplement, and depends on nothing | [modules § the rule](10-modules.md#the-rule) |
| No external broker; `agent-busd` is the broker | [overview § goal](00-overview.md#goal) |
| V1 leftovers (RAG, KV/DB gateways, writers) deferred, non-core — nothing to design | — |
| A registered topic named alone is an inbox to read; with a tag it is a filter — one rule for every face | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |
| A name-shaped topic that is registered nowhere is refused, not read as a filter | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |
| The filter is a priority, not a lease: it holds only while its wait is outstanding | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |
| Sandboxing is off by default and opted into per service; one backend and off, the rest an adapter when a host needs one | [runner § sandboxing](08-runner-role.md#sandboxing) |
| Off is a setting and the default, and asking for confinement a host cannot give is an error rather than a quiet downgrade | [runner § sandboxing](08-runner-role.md#sandboxing) |
| A running script service leaves a note in its owner's state directory, which is what `stop` and `logs` read | [runner § stopping it and reading what it said](08-runner-role.md#stopping-it-and-reading-what-it-said) |
| `stop` does not unregister: the name keeps its queue, and nothing is reading it | [runner § stopping it and reading what it said](08-runner-role.md#stopping-it-and-reading-what-it-said) |
| A caller states its own record before it calls, and only if it has none | [messaging § request and reply](04-messaging.md#request-and-reply) |
| Several readers may wait on one empty inbox when each asks to share it | [messaging § several readers may wait when they say so](04-messaging.md#several-readers-may-wait-when-they-say-so) |
| A receipt is a closed set of two words | [messaging § receipts](04-messaging.md#receipts) |
| The receipts answer "picked up, or lost?", so the bus keeps no delivery journal | [messaging § receipts](04-messaging.md#receipts) |
| A queue belongs to a name, never to a connection or session | [messaging § inbox queues](04-messaging.md#inbox-queues) |
| A subscriber is a registered name, and a fan-out copy lands in its own inbox | [messaging § subscribers](04-messaging.md#subscribers) |
| A subscription is a record on the topic, so it outlives a restart | [messaging § subscribers](04-messaging.md#subscribers) |
| One subscriber that will not read cannot stop a topic; its lost copy is a drop | [messaging § subscribers](04-messaging.md#subscribers) |
| A full queue refuses by default, and a drop is counted | [messaging § overflow](04-messaging.md#overflow) |
| A request that expects a reply must name a registered reply address; registered is not live | [messaging § request and reply](04-messaging.md#request-and-reply) |
| A send to a name with no record is refused, never accepted and dropped later | [messaging § verbs](04-messaging.md#verbs) |
| `--wait` is the caller's deadline and bounds the HTTP exchange, not only the daemon's wait | [messaging § request and reply](04-messaging.md#request-and-reply) |
| The caller's deadline travels to the service, which is what lets it give up early | [messaging § request and reply](04-messaging.md#request-and-reply) |
| The deadline is the caller's and the TTL the receiver's, so the queue bounds one and not the other | [messaging § request and reply](04-messaging.md#request-and-reply) |
| The dashboard's anonymous page shows what the bus would answer a caller it cannot name | [discovery § rules it is built to](05-discovery.md#rules-it-is-built-to) |
| No page renders a credential; a fingerprint and the rotate command stand in for one | [discovery § rules it is built to](05-discovery.md#rules-it-is-built-to) |
| The dashboard may drive a runner, because a control posts as the person and the child holds no authority of its own | [discovery § what it shows](05-discovery.md#what-it-shows) |
| The dashboard runs with no JavaScript, no CDN and no external asset | [discovery § rules it is built to](05-discovery.md#rules-it-is-built-to) |
| A dashboard session lives in the bus, and the browser holds only its id | [discovery § signing in](05-discovery.md#signing-in) |
| The web child stops reaching the bus as the owner once people sign in | [discovery § signing in](05-discovery.md#signing-in) |
| Which dashboard views are MVP, and what each one costs the daemon | [discovery § what it shows](05-discovery.md#what-it-shows) |
| A person record carries three description fields; status and role are not among them | [identity § registration](01-identity.md#registration) |
| A group before AUTH is a flat named set expanded where `allow` is checked | [identity § groups and roles](01-identity.md#groups-and-roles) |
| A script service is started by its owner: becoming a name needs that name's credential | [runner § script services](08-runner-role.md#script-services) |
| A script service is the name it registered — it reads and answers as that name | [runner § script services](08-runner-role.md#script-services) |
| A script service takes a message only when a worker is free; stopping waits for the running ones | [runner § script services](08-runner-role.md#script-services) |
| What a caller does when a receipt arrives | [messaging § receipts](04-messaging.md#receipts) |
| What a script that prints nothing sends back | [runner § script services](08-runner-role.md#script-services) |
| Expiry is counted apart from overflow | [messaging § message TTL](04-messaging.md#message-ttl) |

## Open

| ❓ | Settled by | Where |
|---|---|---|
| Per-method pricing needs the method name in the envelope | owner | [future/billing.md](future/billing.md) |
| A newcomer with no balance cannot reach `pay` | owner | [future/billing.md](future/billing.md) |
| Direct talk bypasses billing | owner | [future/billing.md](future/billing.md) |
| `authorized_keys` regeneration would drop the setup-installed token key | owner | [AUTH role § SSH admin](06-auth-role.md#ssh-admin) |
| Peer sync trusts unsigned records; no clock authority for "newer wins" | owner | [services § registry sync](03-services-and-topics.md#registry-sync) |
| How a queued body is decrypted by a receiver that was not present when it was sent | owner, with the MVP | [access § encrypted sessions](02-access.md#encrypted-sessions) |
| What carries a service's method information | owner, with the MVP faces | [services § service and template](03-services-and-topics.md#service-and-template) |
| What else lives in SQLite | owner | [setup § storage](09-setup.md#storage) |
| Where the ACL and the user-to-account map are edited | owner | [setup § the programs](09-setup.md#the-programs) |
| npm install vs Go-first for the first release | owner | [setup § install](09-setup.md#install) |
| OpenCode (Z.AI) push path | one spike | [runner § adapters](08-runner-role.md#adapters) |
| How a dormant name is woken, and what the daemon has to learn to do it | owner, in Release 1 | [runner § what an instance is](08-runner-role.md#what-an-instance-is) |
| Whether one kept child may have several messages in flight | owner, when a service asks | [runner § long-lived services](08-runner-role.md#long-lived-services) |
| Who vouches for a runner's name on a host that runs no daemon | owner, with the runner | [runner § where it runs](08-runner-role.md#where-it-runs) |
| How a per-service token argument is told apart from asking for a name you own | owner, with Release 1 | [access § token scope](02-access.md#token-scope) |
| How `protocol` is specified for five client languages | owner, with data models | [modules](10-modules.md) |
| MVP and Release 1 contents | owner | [stages](12-stages.md) |
| What happens to a running service when its configuration changes | owner | [services § configuring a template](03-services-and-topics.md#configuring-a-template) |
| A chaining namespace and a service template both want the `/` | owner, with chaining | [overview § chaining](00-overview.md#chaining) |
| Whether reading an inbox and filtering one become separate options | owner, with the MVP CLI | [messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox) |

## Superseded

| Was | Now |
|---|---|
| The runner has no `reload`; there is no long-lived child to signal | long-lived services arrive in Release 1, and a kept child is exactly something to signal — [runner § what the runner does](08-runner-role.md#what-the-runner-does) |
| `--algo=std` is the envelope as one JSON line on stdin | `std` names the channel and claims nothing about the payload; the envelope form is `json`, and `std` is the raw body in bytes — [runner § script services](08-runner-role.md#script-services) |
| A service's credential is handed to the runner at install | a script needs none at all and the runner asks for its own at start; a linked service carries one in its env — [runner § what the child is told](08-runner-role.md#what-the-child-is-told) |
| The runner has an ssh door of its own, and reaching it is the right to install | it is a service on the bus, reached by a call like anything else; the service ACL decides who may deploy — [runner § reaching the runner](08-runner-role.md#reaching-the-runner) |
| An instance directory holds the description, the config, the code and the credential | code and its declared surface are `service.d`, which is a checkout; only env files are the host's, under `runner/` — [runner § what an instance is](08-runner-role.md#what-an-instance-is) |
| Sandboxing is on by default, with a profile per child | off by default and opted into per service: secrets are injected as environment and never sit on a path the child can open, so confinement is hardening rather than what makes the layout correct — [runner § sandboxing](08-runner-role.md#sandboxing) |
| The directory being there is the desired state, and there is no catalogue | it is the *installed* state; what should be **up**, and how many, is a list the runner reads at start — [runner § what an instance is](08-runner-role.md#what-an-instance-is) |
| The runner is one of the supervisor's children, and the only one that may exec | it is outside the daemon entirely, under its own account, so nothing the daemon starts execs at all — [processes § nothing the daemon runs may exec](11-processes.md#nothing-the-daemon-runs-may-exec) |
| The daemon's system account is `agent-bus` | `agent-busd`, so the account and the CLI are not the same word — [setup § the two accounts](09-setup.md#the-two-accounts) |
| The setup user gets the `agent-bus-admin` role | they hold master, and the name is the operator's program instead — [setup § the programs](09-setup.md#the-programs) |
| Minimal setup: install, `agent-bus setup`, start the service | `sudo agent-bus-setup` does all three, and is its own program — [setup § install](09-setup.md#install) |
| Go first, a bun/NPM build later | Go inside, TypeScript for the MCP face and adapters — built by bun, run on Node — [modules § languages](10-modules.md#languages) |
| TypeScript runs on Node because bun's WebSocket fails on a unix socket | bun, reaching the App Server over stdio instead — the WebSocket was the only thing that needed Node — [modules § languages](10-modules.md#languages) |
| Only stdio reaches the Codex App Server; the WebSocket is not needed | both are used — a loopback WebSocket to a shared app-server, stdio to a spawned one. Only a WebSocket over a *unix socket* is out — [runner § adapters](08-runner-role.md#adapters) |
| The runner is part of the core | its own process — it is the one component that execs code it did not write — [processes](11-processes.md) |
| A token lives in daemon memory and dies with a restart | tokens are saved, and the previous one is kept — otherwise a reloaded queue is undecryptable ciphertext — [access § token lifetime](02-access.md#token-lifetime) |
| A call carries exactly two parameters, `user@realm` + token | the token alone: it already backs exactly one principal, so a name beside it is redundancy, not information — [access § what a call carries](02-access.md#what-a-call-carries) |
| A name is checked against the credential it arrived with, and a mismatch told apart from a bad token | no name arrives to check. What that check caught was a typo in a config, not somebody trying to be somebody else — [access § what a call carries](02-access.md#what-a-call-carries) |
| A name never holds a slash: a name is not a path | one slash is allowed, between service template and instance name — the realm still never holds one — [identity § names](01-identity.md#names) |
| "Service" = the kind, "instance" = a running copy of it | "service" means the **configured** thing; the kind is a **service template** — [services § service and template](03-services-and-topics.md#service-and-template) |
| Local access needs no credential at all | the socket hides the credentials, it does not remove them — [access § local socket](02-access.md#local-socket) |
| Socket in each user's `/run/user/<uid>/` | the daemon's own directory, one socket per user — [access § local socket](02-access.md#local-socket) |
| Setup takes local username + gh-username | local account + bus username; the username carries its realm — [setup § local users](09-setup.md#local-users) |
| Principal id is GitHub's numeric id | the name is the identity; the id is only a re-check comparison — [identity § names](01-identity.md#names) |
| GitHub is *the* identity source, and the reason public services work | a provider is an alternative to typing the record — [identity § registration](01-identity.md#registration) |
| Overflow: drop oldest | two modes, `ring` and `strict` — [messaging § overflow](04-messaging.md#overflow) |
| The sandbox backend is chosen by environment: systemd-run, else bwrap, else unshare | one backend and off; the others are an adapter each, written when a host needs one — [runner § sandboxing](08-runner-role.md#sandboxing) |
| `ring` is the default mode | `strict` is: a queue that loses work silently is worse than one that fails visibly — [messaging § overflow](04-messaging.md#overflow) |
| Audience with AUTH off is a per-service `user: token` map | the two ACL layers — [identity § acl](01-identity.md#acl) |
| The service ACL lives in the service's own configuration | the record: the daemon will not read a private configuration, so a layer it enforces cannot live there — [identity § acl](01-identity.md#acl) |
| The MVP encrypts bodies end to end | struck: the daemon issues the key they would derive from — Release 1, on pairwise or derived keys — [access § encrypted sessions](02-access.md#encrypted-sessions) |
| `register` both issues a credential and states a registry record | `token` issues the credential; `register` only states a record — [access § getting a token](02-access.md#getting-a-token) |
| One token reaches every name, and the face overwriting `from` is the only guard | a token backs exactly one principal, and it is the only thing the daemon reads a caller out of — [access § what a call carries](02-access.md#what-a-call-carries) |
| LDAP/AD in scope | deferred — [future](future/ldap-ad.md) |
| NATS · Redis Streams · AUTH-signed JWT keys | dropped; kept in `legacy/` for history |
