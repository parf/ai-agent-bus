# Current decisions

📌 **TL;DR:** Current decisions, grouped by topic, explain the choices behind
current behaviour. Each row links to the section that owns the contract.
Historical and superseded rows live in the [decision archive](../Plans/done/decisions-archive.md#decision-archive).

## Settled

The linked substance wins. Routine implementation details belong in their
owning documentation, not in this index.

### Identity and names

| Decision topic | Substance | Why |
|---|---|---|
| Additional agent instances use numeric dot suffixes (RD107) | [identity § names](01-identity-and-authority.md#names) | 2026-10-20 owner decision; one convention for service instances and launcher collisions |
| A Personal Group is named `@<owner>/<name>[@realm]`; a prefixed Group is its Owner's and is never transferred (Q109) | [constitution § Group](constitution.md#-group) | 2026-09-23 owner answer; the prefix is the User's whole name, so Users never compete for a personal name and the realm stays the Owner's choice; moving one is an administrator's database edit and restart |
| An Agent's canonical name begins with `#` | [constitution § Registry record](constitution.md#-registry-record), [identity § names](01-identity-and-authority.md#names) | 2026-09-21 owner decision; the prefix is part of the name everywhere, like `@` on a Group, so one `name` column is unique across kinds and no term needs a lookup to say what it names |
| The realm is optional in every name | [identity § names](01-identity-and-authority.md#names) | 2026-09-20 owner decision; a realm-less name is a complete distinct identity, the last `@` still separates the realm, and a User defaults to the Unix account name |
| Nested group resolution | [group authority](01-identity-and-authority.md#groups), [upgrade behavior](07-setup.md#nested-group-upgrade) | 2026-09-17 implementation; graph reachability with cycle termination, direct-only protected Administrator membership |
| Durable daemon ownership and node-wide management | [daemon owner](01-identity-and-authority.md#daemon-owner), [setup upgrade](07-setup.md#daemon-ownership-upgrade) | 2026-09-17 implementation; required setup seed, durable transfer to an active registered User, Owner-only resource override without inherited Administrator authority |
| Record Owners choose groups; Administrators control membership | [group authority](01-identity-and-authority.md#groups) | 2026-09-17 owner decision; confirms existing indirect acquisition of record authority; 2026-10-18 owner clarification (A6): Administrator authority edits membership only, not other Group fields |
| Separate administrative and record-maintenance positions | [positions](01-identity-and-authority.md#positions-and-scopes), [shared management](01-identity-and-authority.md#groups) | 2026-09-16 owner clarification; replaces the shared daemon/service “Maintainer” terminology. Earlier dated rows retain historical wording |
| Conditional new-name registration | [definition](01-identity-and-authority.md#registration) | 2026-09-13; enforces the owner's concurrent-session uniqueness requirement |
| A deleted name's group membership goes with it | [unregistering](01-identity-and-authority.md#unregistering) | 2026-09-16 — A freed name is reclaimable by anybody, so a membership left behind is inherited rather than stale: whoever registers the name next arrives in every group the old holder was in, and reaches every record those groups allow |

### Access and credentials

| Decision topic | Substance | Why |
|---|---|---|
| An Administrator maps only their own User identity or an Agent they own; the daemon Owner may map any principal (A13) | [administering the account map](07-setup.md#administering-the-account-map) | 2026-10-19 owner clarification; the administration gate remains, and mapping a foreign User is not Agent ownership |
| Whoever manages a record also sees it while inactive, and an Agent reaches its Owner and Maintainer Users without its allow list naming them (Q125, inactive-record half) | [inactive records](constitution.md#common-record-fields), [channels](constitution.md#-channels) | 2026-09-30, the code's behaviour: whoever may rewrite a record gains nothing by its being hidden, and an Agent answering its manager is the least surprise |
| A send-permission dry run shares Send's target checks and queues nothing (RT20) | [daemon API § checking permission](09-daemon-api.md#checking-permission-to-send) | 2026-10-19 owner decision; caller only, preserving Send's existing refusals |
| The daemon answers `GET /manages`: whether a name may manage a record, by the `/manage` rule, to a caller who may see the record; an agent deciding for a sender asks it rather than copying the rule (Q180) | [daemon API § registry](09-daemon-api.md#registry) | 2026-10-17 owner decision; one authority, and nothing read from a body |
| A transfer keeps an Agent's token working; the old Owner's copy lives until someone rotates it (Q129) | [management boundaries](01-identity-and-authority.md#record-authority) | 2026-09-30 owner decision, against the recommendation to rotate |
| A record's private values are read and written by its Owner and Maintainers, and an Agent reads its own; the allow list, a Group's membership and the daemon Owner's office grant neither (Q125) | [private values](constitution.md#-private-values) | 2026-09-30 owner decision; one authority for everything private on a record |
| A message carries the roles its sender holds toward the record it addressed — `owner` and `maintainer`, worked out by the daemon, passed through forwards, refused from a sender — and a script is told them as `AB_ROLE_<NAME>=1`; an Agent lists the roles it understands (0.8.97) | [agents § roles](03-records-agent.md#roles) | 2026-09-30 owner decisions; assigned roles completed ahead of R1; sender-claimed roles are refused and counted in diagnostics, never logged as supplied role values |
| Assigned application roles use ACL terms; generated names remain reserved | [agents § roles](03-records-agent.md#roles) | 2026-10-16 implementation choice under the start instruction; representation and duplicate grants are owner-confirmed as [№RD92](../Plans/R1.0-Release/DECISIONS.md#recorded-decisions), nested membership as №RD87; applicability as [№RD95](../Plans/R1.0-Release/DECISIONS.md#recorded-decisions). Generated-name reservation is owner-confirmed |
| An account socket takes the token of an Agent its principal owns, making the call that Agent's (0.8.63) | [local socket](02-access.md#local-socket) | 2026-09-29; a runner serves on the one socket forwarded to another host |
| Creating a record grants an Agent no authority over it (Q95) | [registry record](constitution.md#-registry-record) | 2026-09-22 owner answer; the owner is always a User, and an Agent manages only as an explicitly named Maintainer, a rare exception |
| Personal means the Owner and that Owner's agents | [records § personal](03-records.md#personal-and-shared); [ACL terms](02-access.md#acl) | 2026-09-20 owner decision; `@owner` and the new `@agent` alias become valid in ACL and Maintainers, terms outside the cohort stay refused, and the `ab-*` launchers register session agents Personal |
| A person's credential lasts as long as they are a registered user | [token lifetime](02-access.md#token-lifetime) | 2026-09-15 — It is how they call at all, so nothing underneath it ends it; registration is what does |
| Identity and authority are checked once, when a request comes in, and at restart; never again while the operation runs | [authority at the door](constitution.md#authority-at-the-door) | 2026-10-18 — Owner rule: "do not overcomplicate code over removal of users, or an agent losing its rights, right during operations — check only incoming or on restart". A revocation takes effect on the next request; caller-authority re-checks after a wait, at delivery or between writes are removed. Replaces 2026-09-15 "settled under the hold the operation writes under" |
| A credential alone is not a principal: an unregistered name is refused `401` on everything, and is not issued a credential in the first place | [what a call carries](02-access.md#what-a-call-carries), [getting a token](02-access.md#getting-a-token) | 2026-09-15 — Unknown was reading as active, so a name the daemon held nothing for could call and could leave a service owned by nobody — the wreckage the deletion rule exists to clean up, created by an ordinary call. Owner instruction: an unregistered name can do nothing at all, self-registration included, so the bootstrap is closed at the source rather than left as an exception: a name is created by somebody already here, and only then holds a credential |
| `@owner` is runtime ACL syntax for one direct ownership step | [ACL](02-access.md#acl), [upgrade](07-setup.md#owner-acl-and-master-removal) | 2026-09-18 — Owner instruction; it grants access to the direct Owner and that Owner's directly owned Services and Agents, without becoming a stored group or management position |

### Records and configuration

| Decision topic | Substance | Why |
|---|---|---|
| A 📣 stored with `ttl`, `bound` or `overflow` loses them at load with a warning, and the 📣 keeps working (Q123) | [constitution § PubSub routing](constitution.md#pubsub-routing) | 2026-09-30 owner agreement to the 0.8.52 behaviour; the fields never had a meaning of their own |
| Record classification uses normalized tags, retaining Personal rules | [tags](03-records.md#tags) | 2026-10-17 owner instructions |
| An env file refuses any reference — `$NAME`, `${…}`, `$(…)` in an unquoted or double-quoted value — at once, by its line; single quotes keep a `$` | [env-file syntax](03-records-service.md#env-file-syntax) | 2026-10-18 owner rule: "as fast and as simple as possible; FAIL instantly on any unknown reference"; what a reference is follows dotenvx's interpolation rules, the official library's own pointer |
| One env-file syntax, the official dotenv format with nothing evaluated, and one parser, for every `KEY=value` list: secrets and the runner's env files (Q171–Q173) | [env-file syntax](03-records-service.md#env-file-syntax) | 2026-10-17 owner decisions; no legacy form until release, a non-conforming secret is fixed |
| Configuration is visible and editable to Owner and Maintainers in the CLI and web; Secret stays hidden | [configuration](03-records-agent.md#configuring-a-template), [web presentation](../src/web/web-face/records.md#private-configuration) | 2026-10-19 owner decision, RT18; readers unchanged; 2026-10-20 clarification: MCP offers no separate Configuration tool |
| The face follows a card's `https://` source to any address, private and loopback included, on a trusted bus (Q141) | [Resource records](03-records-resource.md#what-a-resource-is) | 2026-09-30 owner decision; the answer goes only to the reader, who could fetch it anyway. Revisit when users other than the Owner register cards |
| A Resource is a card like a Service; its source, an Agent or an MCP Service, answers a read through the face (0.8.70) | [Resource records](03-records-resource.md#what-a-resource-is) | 2026-09-30 owner decisions, built; the design history is in [R1 decisions](../Plans/R1.0-Release/DECISIONS.md#recorded-decisions) |
| Locks belong to a record: its Owner, Maintainers and own Agent use them (0.8.74) | [shared locks](01-identity-and-authority.md#shared-locks) | 2026-09-30 owner decision; the same authority as private values, and the key-value store follows it |
| Each record keeps a key-value store of string, int and JSON values in the daemon's database, used by its Owner, Maintainers and own Agent, each write committed before it is answered (0.8.77) | [key-value store](03-records-kv.md#key-value-store) | 2026-09-30 owner decisions: three tables and call families, JSON operations on top-level keys, `remove_from_set`, the ops in core rather than SQL |
| A service is external; what runs on this bus is an agent | [record kinds](03-records.md#record-kinds) | 2026-09-18 owner decision; the word was doing two jobs, and the one it keeps is the thing a registration only describes. Built in 0.6.3 |

### Messaging and delivery

| Decision topic | Substance | Why |
|---|---|---|
| A 📣 copy that fails through a forwarding 👾 or 📮 counts in the listed recipient's `dropped` (Q127) | [PubSub routing](constitution.md#pubsub-routing) | 2026-09-30 owner agreement; that branch is what failed |
| A User inside a Group on a 📣's `deliver_to` is skipped at publication with an error-log warning and not counted as a drop (Q126) | [constitution § Channels](constitution.md#-channels) | 2026-09-30 owner agreement; refusing the Group would break a working 📣 when membership changes |
| Receipt timing avoids duplicate answers (RD103) | [receipt rules](04-messaging.md#receipts) | 2026-10-19 owner decision |
| A 📣 Deliver-To term is added only by a caller who manages its destination, record or group; that caller authority is checked when added | [what deliver_to accepts](constitution.md#-channels) | 2026-10-17 owner decision on issue A2; group-expanded recipient consent at publication is a separate routing-state check (A1) |
| Group-expanded Deliver-To destinations receive copies only when their Owner manages the channel | [Deliver-To](04-messaging.md#deliver-to) | 2026-10-19 owner decision on A1; owning a group must not grant access to foreign inboxes; direct destinations keep their stored admission |
| Fan-out loop limits report an error without undoing earlier delivered copies | [forwarded envelope](constitution.md#-channels) | 2026-10-20 owner clarification: report recursive configuration without pre-counting every branch or rolling back deliveries |
| An Agent may send to a User its ACL admits, or who is the Agent’s Owner or a Maintainer; the daemon does not recognize replies (Q94, Q104, Q125) | [constitution § Channels](constitution.md#-channels) | 2026-09-22 owner answers, completed by Q125: a User who may use an Agent may be answered by it; matching stays the sender’s topic and tag, and no User sends to a User |
| Deliver-To has kind-dependent meanings and no second forwarding field (Q81) | [constitution § Channels](constitution.md#-channels) | 2026-09-20 owner decision; PubSub carries a recipient list, while Agent and Queue carry at most one destination |
| PubSub routes independently and reports aggregate delivery outcomes | [PubSub routing](constitution.md#pubsub-routing) | 2026-09-21 owner clarification; prevents one minor delivery failure from breaking otherwise working pipelines; routing counters and partial-failure warnings are accepted |
| Agent forwarding checks the receiving ACL against the sending Agent (Q87) | [hop ACL rule](constitution.md#-channels) | 2026-09-21 owner clarification; resolves the principal choice and corrects the earlier original-sender check |
| Forwarding preserves the original sender (Q89) | [constitution § Channels](constitution.md#-channels) | 2026-09-20 owner decision; envelope provenance and reply routing retain the original sender; the later Q87 clarification separately defines the ACL actor |
| Forwarding moves one message (Q91) | [constitution § Channels](constitution.md#-channels) | 2026-09-20 owner decision; the source keeps no copy, distinguishing redirection from PubSub fan-out |
| A 📣 channel carries two lists: the ACL is who may publish, Deliver-To is who receives | [Deliver-To](04-messaging.md#deliver-to), [ACL](02-access.md#acl) | 2026-09-19 owner decision; one list was doing both jobs, so the only way to let somebody publish was to hand them the traffic, and the only way to stop the traffic was to take publishing away. Deliver-To lives on the record beside the allow list, carries 👤 users and 👾 agents and `@group`, and is written by whoever manages the channel. Delivery no longer consults the ACL, which leaves self-service joining with no gate at all, so a name may take itself off the list and no longer put itself on |
| Several readers may wait when they say so | [definition](04-messaging.md#several-readers-may-wait-when-they-say-so) | D228 |

### Daemon and storage

| Decision topic | Substance | Why |
|---|---|---|
| Startup drops only the specified unusable fields while keeping an otherwise valid record (A5) | [what startup drops](constitution.md#what-startup-drops) | 2026-10-19 owner clarification; an oversized Configuration must not discard the record’s grants or queue |
| Startup refuses on a record body that does not decode, message rows without their queue, an out-of-range ID, a damaged `@administrators` and an account mapping to a missing OS account; everything else incorrect is ignored and reported (Q122) | [what refuses the start](constitution.md#what-refuses-the-start) | 2026-09-30 owner agreement; each breaks the store's shape or the node's authority |
| A crash may hand a message consumed since the last flush over again, with the same `message_id` (Q115) | [durability](04-messaging.md#durability) | 2026-09-23 owner answer: simplicity and performance first; the window is one flush and opens only on a crash; the `message_id` lets a receiver drop the repeat |
| A write is published by releasing the registry lock it staged under (Q106) | [persistence and loading](constitution.md#persistence-and-loading) | 2026-09-23 owner answer; the rule is what a reader observes, so staging with an undo log under the one lock meets it; copy-on-write views are a [future candidate](../Plans/R2.0-Future/FUTURE.md#candidates) |
| The daemon's port answers TLS beside plain HTTP when setup turns TLS on; a self-signed certificate setup generates is pinned by fingerprint, handed out by `token --fingerprint` | [remote access § over HTTPS](02-access-remote.md#over-https), [setup § TLS](07-setup.md#tls) | 2026-09-29 owner instruction; one dual listener, serving both, promoted from R1 (Q131, Q132) |
| The daemon binds any TCP address it is given, plain HTTP, and says so at start when it is off loopback | [trust boundary](02-access.md#trust-boundary) | 2026-09-25 owner decision; agents run on any number of hosts, and a loopback-only listener forced every one through an SSH tunnel |
| A configured flow that breaks goes to the error log; a send to an unknown name does not (Q108) | [flow outcomes](constitution.md#common-record-fields) | 2026-09-23 owner answer; publications and routes that stop working are written, an ordinary refusal is told only to its caller |
| One active daemon per database | [constitution persistence](constitution.md#persistence-and-loading); [work](../Plans/done/R0.8-MVP/0.7.0-TODO.md#storage-and-identity) | 2026-09-19 owner decision after review, narrowed 2026-09-22: take SQLite's exclusive lock before serving; detecting a lost lock is not required |
| An acknowledged ban, group removal or ACL restriction holds until explicitly lifted | [administrative crash recovery](04-messaging.md#administrative-crash-recovery) | 2026-09-15 — Acknowledging is a promise, and a crash is not a way out of one; if the durability cannot be promised the acknowledgement must not be given |
| The daemon may run `ssh-keygen`; the boundary is no user services, not no exec | [nothing the daemon runs may exec](08-processes.md#nothing-the-daemon-runs-may-exec) | 2026-09-15 — A fixed verifier the daemon ships and calls with its own arguments is not a stranger's program, and what a user supplied is what the rule is about |

### Runner and execution

| Decision topic | Substance | Why |
|---|---|---|
| Runner host env reads expose metadata only; values require host access (RD105) | [the two secret stores](runner/security.md#secrets) | Owner-approved RD105; bus and web writes still travel as ordinary durable messages, not a separate host-secret store |
| Runner controls distinguish temporary start/stop/restart from persisted enable/disable (RD100) | [controlling one agent](runner/operations.md#controlling-one-agent) | Owner-approved RD100; changing whether an agent runs now does not silently change its configured enablement |
| Concise authored descriptions and help | [authoring checklist](agents/authoring.md#checklist-writing-an-agents-description-and-help) | 2026-10-19 owner decision: listings should explain useful work without repeating names; examples and shared arguments make generated help usable |
| Standalone agents hold only their own token | [standalone lifecycle](../src/agents/standalone/starter.md#lifecycle), [supervised registration](06-dedicated-runner.md#keeping-a-supervised-registration) | 2026-10-19 owner instruction: never retain a person's credential for a read-only agent |
| An agent describes its methods, Configuration keys and secret names in its own package (`agent.json`, commented `env.dist`); the daemon stores and interprets none of it; packaged agents' help is answered by the runner | [agent self-description](constitution.md#agent-self-description) | 2026-10-19 owner decision: "I don't want to describe this in the daemon — on an agent update they can be added or changed"; MCP's schema too heavy |
| An agent is asked to describe itself by the envelope's `ask` field, never by body text; a script agent sees only `AGENT_BUS_ASK` and no input (Q179) | [agent self-description](constitution.md#agent-self-description) | 2026-10-17 owner decision: no "magic input params"; an agent that knows nothing of asks treats one as no input |
| Packaged help is generated from strict agent.json metadata and commented env.dist (RT9); role arrays explicitly name alternatives | [package help](runner/configuration.md#package-help) | 2026-10-19 owner: descriptions live with code, secret help never carries values, and self-serving agents use their own helpers |
| Agent metadata in the web face | [About an agent](03-records-agent.md#about-an-agent) | 2026-10-19 owner: show the agent’s own description and settings declarations without exposing Secret values |
| Every agent answers `--help` with a description for an AI reader, limited to what the caller may do; every bundled agent must, and MCP sessions ask it first | [agent self-description](constitution.md#agent-self-description) | 2026-10-17 owner request; a caller should learn an agent from the agent itself, and admin-only actions stay unadvertised |
| A dedicated runner waits out a daemon that is away and reconnects with backoff; only a refusal ends it (0.8.65) | [script agents](06-dedicated-runner.md#script-agents) | 2026-09-29 owner instruction; a restart of the bus must not take every runner down with it |

### Bundled agents

| Decision topic | Substance | Why |
|---|---|---|
| Session sets start through personal-account standalone tmux launchers, with fixed names and tags | [session starter](agents/sessions.md#configuration) | 2026-10-20 owner-approved minimal B.12; collisions fail instead of replacing sessions |
| The AI proxy uses one OpenAI-compatible endpoint and host-only provider credentials | [AI configuration](agents/ai.md#configuration-and-secret-keys) | Owner-approved minimal B.11 design, 2026-10-20 |
| Bundled systemd and Git Owners and Maintainers get the top application tier within the instance’s caps (RD104) | [systemd permissions](agents/systemd.md#calls-and-permissions), [Git roles](agents/git.md#roles-and-boundaries) | Owner-approved RD104; assigned application roles grant others access and never bypass unit or repository limits |
| Slack and Telegram bundle one session router; Slack subscriptions stay independent of addressed routing | [Slack reply routing](agents/slack.md#configuration) | 2026-10-19 owner request, S.10 |
| Per-chat drop handlers discard incoming Telegram and Slack messages while outgoing posts remain available | [Telegram per-chat routing](agents/telegram.md#per-chat-routing), [Slack configuration](agents/slack.md#configuration) | 2026-10-20 owner request; quiet selected chats without changing others |
| Telegram per-chat routing and channel announcements | [Per-chat routing](agents/telegram.md#per-chat-routing) | 2026-10-19 owner instructions |
| Slack subscriptions may select resolved user and bot authors | [subscription authors](agents/slack.md#3-forward-only-useful-errors) | 2026-10-19 owner decision, S.9 |
| Git autopull uses opt-in install-time timers and ff-only pulls | [autopull](agents/git.md#autopull) | 2026-10-19 owner decision; no forced cleanup or service restart |
| Git synchronization uses server identity | [git agent](agents/git.md#what-it-does), [roles and boundaries](agents/git.md#roles-and-boundaries) | 2026-10-19 owner decision, B.8 |
| Git repositories and their access caps live in the agent's Configuration; a caller's role never exceeds the cap | [repositories and access levels](agents/git.md#repositories-and-access-levels) | 2026-10-19 owner request |
| A systemd instance may be limited to the units its Configuration lists, so an ordinary user gets a dedicated status and restart agent | [dedicated instances](agents/systemd.md#dedicated-instances) | 2026-10-19 owner request; the default instance still sees every unit |
| Telegram session tags and listed-user reply routing | [Telegram reply routing](agents/telegram.md#reply-routing) | 2026-10-19 owner instructions |
| Logwatch keeps bounded host history in one long-lived standalone agent | [Logwatch](agents/logwatch.md#configuration-and-secret) | Owner-approved B.3 port of tail-ring-buffer; history stays between calls |
| Debug is the bounded bus doctor | [Diagnostics and access](agents/debug.md#check-the-bus-in-a-minute) | 2026-10-19 owner request, B.13 |

### Web, CLI, MCP and launchers

| Decision topic | Substance | Why |
|---|---|---|
| Long CLI options use two dashes; one-letter options use one | [option spelling](user/cli.md#option-spelling) | 2026-10-20 owner rule; consistent help, examples and generated units, with legacy input accepted |
| Runner and web installation paths (RD102) | [a person's runner](07-setup.md#a-persons-runner), [web installation](07-setup.md#install) | 2026-10-18 owner decision: simplify shared code and private account state; remove the web link |
| Agent method cards use disclosure rows, declared call-body formats and role-hidden counts (RT9.4) | [web methods](../src/web/web-face/records.md#method-rows), [package help](runner/configuration.md#package-help) | 2026-10-19 owner-approved UX; examples and filtering without executing calls |
| Fan-out calls select by tag with --tagged; --tag stays the correlation tag | [calling many agents](user/cli.md#calling-many-agents-at-once) | 2026-10-19 owner-approved CLI fan-out; preserve existing scripted exchanges |
| Claude channels are moving to an approved marketplace plugin (RD106; launcher switch pending) | [Claude channel plugin](06-launchers.md#claude-channel-plugin) | Owner-approved unattended startup; org policy remains authoritative |
| Launcher session tags survive resume and MCP re-registration; explicit --tag replaces the custom set | [session tags](06-launchers.md#session-tags) | 2026-10-20 owner request; group sessions for listing and tagged calls |
| Launcher identities remain reserved for their saved sessions | [session names](06-launchers.md#session-names) | 2026-10-19 owner priority: fixed names; automatic reclamation rejected, the session owner's rename remains explicit |
| The MCP face stays on 2025-11-25 as a dated exception until Claude Code delivers channels on 2026-07-28 and Codex and OpenCode speak it; then the v2 SDK, older revisions refused (Q143) | [external protocols](constitution.md#external-protocols) | 2026-09-30 owner decision; a latest-only face stops push and refuses two of the three clients |
| `ls` and `ab_ls` default to the agents being read now (0.8.55) | [CLI listing](05-discovery.md#cli-listing) | 2026-09-29; the everyday question is who can take a message |
| Renaming is for `ab-*` sessions only; a bare MCP face refuses it (Q105) | [explicit session rename](06-launchers.md#explicit-session-rename) | 2026-09-23 owner answer; an Agent is never issued another Agent's credential, so only a launcher, which acts for the User, moves an address |
| Launcher automatic execution and continuation | [definition](06-launchers.md#smart-launchers) | 2026-09-13 owner instruction; replaces preserving caller-selected execution modes |
| The web panel acts on the visitor's token and on nothing else | [web face](08-processes.md#the-web-face) | 2026-09-16 — Owner instruction. What the dashboard can do is what the signed-in person could do from the CLI: no privileged fallback when a call is refused, no authority held between requests, and permissions rendered from what the daemon answered rather than recomputed in the face — two implementations of the access rules disagree, and the disagreement that matters is a page offering an action the daemon will refuse |
| A node publishes a closed list — release, build, host name, daemon owner, uptime and calls served — to anybody, signed in or not, in the shell every page shares and on the sign-in page | [what a node says about itself](05-discovery.md#what-a-node-says-about-itself), [web face](08-processes.md#the-web-face) | 2026-09-16 — Owner-asked. Somebody who reaches a bus they hold no credential for should be able to tell what it is and whose it is without asking anybody, which is precisely the person a sign-in page leaves with nothing. Each cost was put to the owner and accepted, the traffic figures explicitly: an unauthenticated visitor learns the host's name, who runs the node, how long it has been up and how much it carries. The list was cut back at 0.5.39, when the owner removed the host load average and the message counters in favour of the daemon's own served-call counts. Nothing else becomes readable — no record, no principal, no refusal count — and the face gains no privilege, since the daemon answers this list to any caller |

### Versioning and project process

| Decision topic | Substance | Why |
|---|---|---|
| The development node runs from the checkout through `git-install.sh`; `release.sh` is for a node running a fixed release | [setup § install](07-setup.md#install) | 2026-09-29 owner instruction; the CLI and every tool are always the checkout's |
| Each subsystem has fast, normal and slow test sets; a simple change reaches the node in minutes | [testing speed](constitution.md#testing-speed) | 2026-10-20 owner instruction: a simple change taking ten minutes or more is unacceptable |
| Project license | [terms](../LICENSE.md#polyform-noncommercial-license-100) | 2026-09-13 owner instruction |
| Client libraries share the MIT license | [terms](../src/clients/LICENSE.md#mit-license) | 2026-10-01 owner instruction |

## Open

Unresolved choices live with their [release stages](../Plans/README.md#stages).
