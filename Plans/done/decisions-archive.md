# Decision archive

History only; these rows are never a current contract. See [current decisions](../../docs/decisions.md#settled).

## Identity and names

| Decision topic | Substance | Why |
|---|---|---|
| One daemon parser enforces the documented name syntax on every kind | [names](../../docs/01-identity-and-authority.md#names), [K.19](R0.8-MVP/0.7.0-TODO.md#storage-and-identity) | 2026-09-22 owner instruction; the table is unchanged, so a component still starts alphanumeric |
| A Group name may carry a realm (Q100) | [common record fields](../../docs/constitution.md#common-record-fields) | 2026-09-22 owner answer; the ordinary name rules apply after the `@` |
| Every User has exactly one 👤 record, never removable while the User exists | [constitution § User](../../docs/constitution.md#-user) | 2026-09-22 owner answer; the record's `owner_id` is the `user_id`, and a User found without one at startup is a conceptual error |
| The daemon Owner is a bare account name; session Agents keep the host as realm | [identity § names](../../docs/01-identity-and-authority.md#names), [session names](../../docs/06-launchers.md#session-names) | 2026-09-22 owner answer; `parf` owns `#claude/…@parf.us` |
| A Group is a registry record with `kind` and `status` | [constitution § record kind](../../docs/constitution.md#-record-kind), [what `inactive` means](../../docs/constitution.md#common-record-fields) | 2026-09-21 owner restatement of the original design, which the docs had lost; the closed enum gains `group`, and an inactive Group does not exist while its name stays reserved |
| Reactivation under the single inactive User state (Q79) | [user states](../../docs/01-identity-and-authority.md#user-states) | 2026-09-20 owner decision; no formerly-banned authority branch remains |
| Protected Administrator Group ownership follows daemon ownership (Q80 revised) | [constitution § Group](../../docs/constitution.md#-group) | 2026-09-21 owner correction; replaces the ownerless Group rule while retaining the protected authority boundary |
| Agent/record removal clears every stored reference (Q82) | [constitution persistence](../../docs/constitution.md#persistence-and-loading) | 2026-09-20 owner decision; name reuse inherits no former authority or delivery |
| Editable GitHub-populated profile details | [profiles](../../docs/01-identity-and-authority.md#users-and-profiles), [profile fields](../../src/web/web-face/people.md#profile-fields) | 2026-09-18 owner clarification; Company, Location and Twitter/X are AgentBus User fields, editable by existing profile authority, with GitHub as an import/refresh source rather than a second profile |
| GitHub profile imports and local photos | [profiles](../../docs/01-identity-and-authority.md#users-and-profiles), [profile fields](../../src/web/web-face/people.md#profile-fields) | 2026-09-17 owner instruction; public provider fields fill blanks where specified, locally imported photo is preferred over initials, no browser hotlink; implemented in 0.5.71 |
| Self-email editing | [profile permissions](../../docs/01-identity-and-authority.md#users-and-profiles) | 2026-09-17 implementation; users control only their own email |
| Protected identity fields in self-service profile editing | [profile permissions](../../docs/01-identity-and-authority.md#users-and-profiles) | 2026-09-16 owner clarification; self-service protection implemented in 0.5.56 and trusted person-name imports in 0.5.58 |
| Manual removal of idle registry addresses | [unregistering](../../docs/01-identity-and-authority.md#unregistering) | 2026-09-13 owner request |
| A removed name keeps nothing; a name is protected by retiring it | [unregistering](../../docs/01-identity-and-authority.md#unregistering), [R1.2 retirement](../R1.2/records.md#down-and-retired) | 2026-09-15 owner revision |
| Full control of owned records | [record authority](../../docs/01-identity-and-authority.md#record-authority) | 2026-09-13 — Owner clarifies that ordinary users need no daemon administration position to manage what they own |
| Local user administration before AUTH | [groups and maintainers](../../docs/01-identity-and-authority.md#groups) | 2026-09-13 — Implement the accepted owner/maintainer hierarchy with a protected flat group |
| Profile identifier normalization | [identifier uniqueness](../../docs/01-identity-and-authority.md#users-and-profiles) | 2026-09-13 — Initial implementation default without provider alias merging |
| No registration, no access | [unregistering](../../docs/01-identity-and-authority.md#unregistering) | 2026-09-15 — The record is what access hangs on; a credential whose record is gone has nothing left to hold, and the page said the opposite |
| A record is not removed while its name still owns others, unless that name is a registered user | [unregistering](../../docs/01-identity-and-authority.md#unregistering) | 2026-09-15 — Removing a name's only standing while it owns records strands each of them under an owner nothing answers for; refusing names what is in the way, and a user survives losing a record because the user is still somebody |
| Keep non-user identities visible and distinguish them from registered users | [person records](../../docs/01-identity-and-authority.md#users-and-profiles) | 2026-09-15 — Cleanup needs visible evidence rather than a directory that hides debris |

## Access and credentials

| Decision topic | Substance | Why |
|---|---|---|
| Whoever may edit a User pastes that User's `ssh-ed25519` key in the web face; the daemon writes the forced `agent-bus-token` line, and operator keys and other types stay host-side (0.8.78) | [SSH admin](../../docs/07-setup.md#ssh-admin) | 2026-09-30 owner request; for owner review: ssh-ed25519 only, and a User may not set their own key |
| For owner review: an issue over a credential ignored as corrupt is allowed and reported at alert, as an authorized repair, rather than refused until an operator edits the store (0.8.85, K.29) | [token](../../docs/constitution.md#-token) | 2026-09-30: refusing it would lock the name out; the daemon still never repairs a row by itself |
| Personal checks only `allow` and `maintainers`, on every save (Q101) | [records § personal](../../docs/03-records.md#personal-and-shared) | 2026-09-22 owner answer; a term outside the Owner's cohort is an error at creation and after it, while `deliver_to`, forwarding and delivery are unaffected |
| Issuing or rotating a token is not logged in the audit log | [registry record](../../docs/constitution.md#-registry-record) | 2026-09-22 owner answer; an ordinary, frequent operation there would only be noise |
| `@agent` is a reserved term valid only on an Agent's record (Q102) | [actor terms](../../docs/constitution.md#actors-and-ascii-textarea-syntax), [ACL](../../docs/02-access.md#acl) | 2026-09-22 owner answer; like `@owner` it can never be a Group's name, and on any other kind it rejects the update |
| `*` admits every active User and every active Agent (Q97) | [actor terms](../../docs/constitution.md#actors-and-ascii-textarea-syntax), [ACL](../../docs/02-access.md#acl) | 2026-09-22 owner answer |
| An inactive target answers `unknown`; `disabled` goes (D6) | [refusals](../../docs/05-discovery.md#refusals), [K.11](R0.8-MVP/DONE.md#done--mvp) | 2026-09-22 owner answer; an inactive caller keeps `suspended` |
| An inactive record is no such entity, apart from one read-only web call | [common record fields](../../docs/constitution.md#common-record-fields), [user states](../../docs/01-identity-and-authority.md#user-states) | 2026-09-22 owner answer; the call admits the record's `allow` and always the daemon Owner, the Owner or a Maintainer reactivates, and an inactive User's records are inactive too, hidden and unknown rather than suspended |
| Personal is on every kind, and a User is always Personal | [constitution § private values](../../docs/constitution.md#-private-values), [records § personal](../../docs/03-records.md#personal-and-shared) | 2026-09-21 owner decision; Personal is valid on every kind but 👤, while `config` and `secret` are valid on 👾, 📡 and 👥 under one private-value rule |
| A token always names a User and optionally an Agent | [constitution § Token](../../docs/constitution.md#-token) | 2026-09-20 owner decision; replaces the single-principal token, keeps the Agent as the acting principal, and requires the named User to be that Agent's current Owner, a mismatch being corrupt state reported to syslog at `alert` severity and to the daemon log rather than a refusal path |
| Personal ACLs exclude groups | [Personal assignment limits](../../docs/03-records.md#personal-and-shared) | 2026-09-17 owner clarification; including groups composed only of agents; enforced in 0.5.50 |
| ACL governs access by other principals | [access rules](../../docs/02-access.md#acl) | 2026-09-17 owner confirmation; own-inbox right retained in the 0.5.44 default change |
| Explicit wildcard for broad user access | [ACL grants](../../docs/02-access.md#acl) | 2026-09-16 owner clarification |
| Empty ACL retains resource-management access | [ACL default](../../docs/02-access.md#acl) | 2026-09-16 owner correction; implemented in 0.5.44 |
| Owner-tagged Personal agents | [web grouping and assignment limits](../../docs/03-records.md#personal-and-shared) | 2026-09-16 instruction, clarified 2026-09-17; stored classification and assignment limits built in 0.5.50, web grouping built in 0.5.51; ordinary access policy retained |
| Credential lifetime across releases | [token lifetime](../../docs/02-access.md#token-lifetime) | 2026-09-13 owner instruction |
| A credential nobody owns is dropped at daemon start | [ownerless credentials](../../docs/02-access.md#ownerless-credentials) | 2026-09-15 — A sweep at a known moment, not expiry — tokens still never retire for being old or idle; this clears what the old outlives-the-address rule left behind |
| The account map is administered by operator SSH key or user token, and nothing new | [administering the account map](../../docs/07-setup.md#administering-the-account-map) | 2026-09-15 — The entitlement is already the principal in the key's entry, so who may edit the map is answered by what answers everything else |
| A suspended owner's record answers `403 suspended`, the caller's own reason widened rather than a second one | [refusals](../../docs/05-discovery.md#refusals) | 2026-09-15 — It is one suspension seen from two sides, and the caller can act on neither: no credential and no grant makes a suspended name answer, which is what `403` already tells them |
| An ownerless credential may be removed by hand by the daemon owner or a maintainer, and the start-of-day sweep stays | [ownerless credentials](../../docs/02-access.md#ownerless-credentials) | 2026-09-15 — Maintainers already administer users and this is the user directory; the access order does not settle it, because a credential nobody owns is not a level below anybody |
| Master access is removed; there is no second ACL layer | [ACL](../../docs/02-access.md#acl), [0.5.74 evidence](R0.8-MVP/done/owner-acl-runtime-group.md#checks) | 2026-09-18 — Owner decision. It was never asked for and no decision record was ever made for it — it entered the documents on 2026-09-10 as part of a restructure that consolidated earlier design notes, and was implemented on 2026-09-11. Access now comes from record authority and the allow list alone. The owner keeps [node-wide management](../../docs/01-identity-and-authority.md#daemon-owner), which is a separate and recorded authority |

## Records and configuration

| Decision topic | Substance | Why |
|---|---|---|
| Configuration is visible and editable to Owner and Maintainers in every face; Secret stays hidden | [configuration](../../docs/03-records-agent.md#configuring-a-template), [web presentation](../../src/web/web-face/records.md#private-configuration) | 2026-10-19 owner decision, RT18; readers unchanged |
| An Agent's `script` field says what `agent-bus run` serves it with: written by the runner, read-only, informational, on an 👾 alone (Q124, 0.8.76) | [script agents](../../docs/06-dedicated-runner.md#script-agents) | 2026-09-30 owner decision; the runner's note leaves `addr`, a 📡's field |
| For owner review: a Resource read through an MCP Service uses the Service's secret, so it answers only those who may read it, its Owner and Maintainers; others are refused rather than sent on without it (0.8.74) | [Resource records](../../docs/03-records-resource.md#what-a-resource-is) | 2026-09-30, following the private-values rule; the daemon forwarding the read itself would break the rule that it serves no content |
| A holder extends its lock with a separate `extend` call; re-taking stays refused | [shared locks](../../docs/01-identity-and-authority.md#shared-locks) | 2026-09-30 owner decision; a re-take that renews mixes two meanings into one call |
| For owner review: KV name and operation limits, JSON object values, integer increments, absent deletes and kind-selected MCP tools | [key-value store](../../docs/03-records-kv.md#key-value-store) | 2026-09-30 implementation choices; value sizing is replaced by the owner-requested review correction below |
| KV values leave room for wire encoding within the API request limit | [key-value store](../../docs/03-records-kv.md#key-value-store) | 2026-09-30 owner request after review P2; the API request bound now covers escaping; the accepted smaller value cap remains pending |
| KV integers use the owner-selected signed range | [key-value store](../../docs/03-records-kv.md#key-value-store) | 2026-09-30 owner clarification after review P1; the narrower signed range remains pending; MCP preserves exact large integers |
| Initial Maintainers are validated and stored with record creation; re-registration keeps existing grants | [registration](../../docs/01-identity-and-authority.md#registration) | 2026-09-30 owner request for Maintainers on registration forms |
| Service secrets use validated env-file syntax in 0.7 | [constitution § Service](../../docs/constitution.md#-service) | 2026-09-19 owner replacement of Q77; basic env-file syntax is validated while application-specific values remain opaque |
| An external service has no queue here (Q78) | [services § it has no queue here](../../docs/03-records-service.md#it-has-no-queue-here) | 2026-09-19 owner decision; a 📡 is an information card its [ACL](../../docs/02-access.md#acl) admits people and agents to read — address, protocol, description and secret — so send, subscribe and consume refuse it, no snapshot may restore a queue under one, and no face reports a reader count or backlog it never measured |
| A record's own user state is asked only of kinds that can have one | [record kinds](../../docs/03-records.md#record-kinds) | 2026-09-18 owner decision; a queue, a pub/sub channel and an external service have no user state, and the enum is what makes that answerable |
| Record kinds are a closed set | [record kinds](../../docs/03-records.md#record-kinds) | 2026-09-18 owner decision; a page was inferring what a record is because nothing stored it |
| A service holds secrets, separately from its configuration | [service secrets](../../docs/03-records-service.md#secrets) | 2026-09-18 owner decision; brought forward from R1.2 because reaching an external thing needs a credential and configuration was built for a different purpose |
| Agent method information is the description | [agent templates](../../docs/03-records-agent.md#agent-templates) | 2026-09-15 owner instruction |

## Messaging and delivery

| Decision topic | Substance | Why |
|---|---|---|
| An Agent may send to a User its own ACL admits; the daemon does not recognize replies (Q94, Q104) | [constitution § Channels](../../docs/constitution.md#-channels) | 2026-09-22 owner answers; whoever may reach an Agent may be answered by it, so an Agent open to `*` may send to every User, matching stays the sender's topic + tag, a User has no list of its own, and no User sends to a User |
| One send passes through at most ten 📣 channels; the eleventh fails the send loudly as a Deliver-To loop | [forwarded envelope](../../docs/constitution.md#-channels) | 2026-10-17 owner instruction on issue A3, replacing 0.8.169's once-per-name delivery: a loop is a configuration error and must fail |
| A 📣 Deliver-To term is added only by a caller who manages its destination, record or group; asked when added, never again | [what deliver_to accepts](../../docs/constitution.md#-channels) | 2026-10-17 owner decision on issue A2: a channel listing a queue or 📣 bypassed that destination's ACL |
| A message holding roles passes from A to B only when A's Owner owns or maintains B; otherwise the forward fails loudly — refused and logged (0.8.98) | [agents § roles](../../docs/03-records-agent.md#roles) | 2026-09-30 owner decision, chosen over dropping the roles: an Owner's own publication reaches only recipients that Owner answers for |
| A person does not send to a person | [request and reply](../../docs/04-messaging.md#request-and-reply) | 2026-09-22 owner confirmation of the reply-only User inbox |
| PubSub counters measure accepted publications and delivered copies | [PubSub routing](../../docs/constitution.md#pubsub-routing) | 2026-09-21 owner answer; settles routing counter units separately from reader consumption |
| A forward counter bounds chains at ten steps | [constitution § Channels](../../docs/constitution.md#-channels) | 2026-09-20 owner decision; replaces the one-hop limit now that a 📣 may fan out into channels, and the counter is what terminates a cycle |
| A second-hop destination refuses the original send (Q85) | [constitution § Channels](../../docs/constitution.md#-channels) | 2026-09-20 owner decision; a direct delivery refuses before storage; PubSub branch failures follow the later aggregate routing rule |
| Forwarding adds no TTL or deadline policy | [constitution § Channels](../../docs/constitution.md#-channels) | 2026-09-20 owner decision; the destination queue applies the ordinary message rules |
| Forwarded envelopes identify their prior destination (Q90) | [constitution § Channels](../../docs/constitution.md#-channels) | 2026-09-20 owner decision; required single-valued `original_to` makes a one-hop forward visible to its recipient |
| Forwarding applies destination rules as if directly targeted (Q88) | [constitution § Channels](../../docs/constitution.md#-channels) | 2026-09-20 owner replacement; direct queue refusal changes no queue state; ring eviction retains its destination rule; PubSub outcomes follow the later routing clarification |
| One reader count includes filtered waits | [reader observation](../../docs/05-discovery.md#readers) | 2026-09-17 owner decision; implemented in 0.5.53 |
| Explicit inbox selection, independent message filters | [consume syntax](../../docs/04-messaging.md#inbox-selection-and-filters) | 2026-09-17 owner decision; implemented in 0.5.52 |
| A full queue answers `429`, not `503` | [overflow](../../docs/04-messaging.md#overflow) | 2026-09-15 owner instruction; `503` is left to a service that is briefly unavailable |
| Deliver-To | [definition](../../docs/04-messaging.md#deliver-to) | D232, D233, D234 |

## Daemon and storage

| Decision topic | Substance | Why |
|---|---|---|
| Every calendar day of activity is kept 400 days as one zstd row per name and yymmdd date, holding only non-empty slots, written once per ten-minute boundary and at stop; Day, Week and Month with Prev and Next on Activity and every record page | [activity history](../../docs/05-discovery.md#activity-history) | 2026-09-24 owner request: date key yymmdd, name → daily blob, zstd, non-empty only, saved right after each ten-minute block, today loaded at start; no per-minute save. Built in 0.8.41 |
| Activity is a per-record day ring of ten-minute slots on the node's local clock, saved in SQLite; a slot carries only counts | [activity history](../../docs/05-discovery.md#activity-history) | 2026-09-23 owner answers: persisted with the regular save batch, node local time, a clock-aligned ring; then the owner's simplification that down time is zero and the open slot carries no partial mark. Built in 0.8.12 |
| Daemon logs live under `/var/log/agent-bus/` with ordinary logrotate, readable by `adm` (Q96) | [errors and alerts](../../docs/constitution.md#errors-and-alerts) | 2026-09-22 owner answer |
| Three daemon logs: `debug.log`, `audit.log` and `error.log` | [logs](../../docs/constitution.md#logs) | 2026-09-22 owner decision; debug is an access log of every request, off by default and written only on demand, audit holds administrative actions and entity edits, error holds warnings and errors and is copied to syslog |
| Incorrect stored records are always ignored (Q99) | [persistence and loading](../../docs/constitution.md#persistence-and-loading) | 2026-09-22 owner answer; reported as a conceptual error, never loaded or repaired, while the rest of the node starts |
| SQLite through `modernc.org/sqlite` | [constitution persistence](../../docs/constitution.md#persistence-and-loading) | 2026-09-22 owner choice |
| Conceptual errors and alerts go to syslog and the error log | [constitution § errors and alerts](../../docs/constitution.md#errors-and-alerts) | 2026-09-20 owner decision; impossible states are reported to both destinations, while an ordinary refusal is answered to its caller and reported nowhere |
| A status change is an entity edit and is logged | [constitution § Registry record](../../docs/constitution.md#-registry-record) | 2026-09-20 owner decision; `status` is an ordinary field, and suspending somebody is the one administrative act that must not be silent |
| Statistics cadence and immediate data changes | [statistics persistence](../../src/MODULES.md#statistics-persistence) | 2026-09-20 owner clarification; replaces unspecified batching and the earlier token timestamp budget |
| Identity cleanup through write-through changes | [constitution persistence](../../docs/constitution.md#persistence-and-loading) | 2026-09-19 owner clarification after review; durable and in-memory grants must follow identity lifecycle changes |
| SQLite first; additional backends in R1.1 | [constitution persistence](../../docs/constitution.md#persistence-and-loading); [R1.1 storage](../R1.1/storage.md#backends) | 2026-09-20 owner instruction; replaces the earlier 0.7 adapter scope |
| Direct entity edits go only to daemon log files | [constitution § Registry record](../../docs/constitution.md#-registry-record) | 2026-09-20 owner correction; the authenticated User-or-Agent actor, operation, target, result, and available client IP are logged without sensitive information; credential, state, read, and traffic operations are outside scope |

## Runner and execution

| Decision topic | Substance | Why |
|---|---|---|
| "Dedicated runner" is `agent-bus run`, and plain "runner" is `agent-bus-runner` alone; its page is split from the launchers' | [dedicated runner](../../docs/06-dedicated-runner.md#what-it-does), [names](../../docs/glossary.md#names) | 2026-10-17 owner decision: two runners, two names |
| A dedicated runner unregisters on a deliberate exit only, takes a plain name as its `#` agent, and needs an absolute script path | [script agents](../../docs/06-dedicated-runner.md#script-agents) | 2026-09-29; a transient fault keeps the name, and the script starts in its work directory |

## Bundled agents

| Decision topic | Substance | Why |
|---|---|---|
| RD132 `#qdrant` passes the whole API with no finer grant than the ACL | [R.4](../R1.0-Release/rag/TODO.md#steps) | 2026-10-28 owner choice, replaced the same day by RD133: the same pass-through, gated by `read`, `write` and `admin` roles |
| RD130 Qdrant runs setup-managed with host networking bound to loopback | [R.3 history](../R1.0-Release/rag/TODO.md#steps) | 2026-10-28 owner choice, replaced the same day by RD131: Qdrant is plain podman anywhere, outside the bus |
| Server-info uses fixed host commands and a shared standalone starter | [server-info](../../src/agents/standalone/server-info/README.md#server-info-agent), [starter](../../src/agents/standalone/starter.md#standalone-starter) | 2026-10-19 owner-accepted sections and standalone deployment; section names and unit naming chosen in implementation |
| Git autopull uses opt-in install-time timers and ff-only pulls | [original core plan history](../R1.0-Release/git-autopull/DONE.md#reviewed-plan-history) | 2026-10-19 owner decision; superseded by channel-driven modes and explicit post-attempt scripts on 2026-10-29 |
| RD149 `#git` runs no deploy hooks; services act on `autopull-log` | [git autopull](../R1.0-Release/git-autopull/README.md#after-an-attempt) | 2026-10-29, replaced the same day by RD150 and RD151: the owner wants each server's own after-deploy scripts; acting on the log stays optional |

## Web, CLI, MCP and launchers

| Decision topic | Substance | Why |
|---|---|---|
| A web-face edit carries no client IP in MVP; forwarding the visitor's IP is R1.1 work (Q128) | [constitution § logs](../../docs/constitution.md#logs) | 2026-09-30 owner agreement |
| The web face gets TLS through its own copy of the node's certificate and a setup drop-in (0.8.62) | [setup § TLS](../../docs/07-setup.md#tls), [the web face](../../docs/08-processes.md#the-web-face) | 2026-09-29; neither account reads the other's key |
| Every program's help opens with its name and version (0.8.56) | [build information](../../docs/07-setup.md#build-information) | 2026-09-29; the version is visible wherever usage is |
| The web face is the TypeScript face under its own account and systemd unit; the Go dashboard, its supervised child, bubblewrap and web cgroup are removed (0.8.50) | [the web face](../../docs/08-processes.md#the-web-face) | 2026-09-24 owner-accepted cutover ([Web plan](R0.8-MVP/web/README.md#cutover)); the unit's walls replace the supervisor's sandbox and delegated cgroup |
| The daemon counts records inactive through their owner in `/status`, for Administrators; the Overview shows one item from it | [overview and diagnostics](../../docs/05-discovery.md#overview-and-diagnostics) | 2026-09-23 owner decision on F.13.7's transport; one number from the daemon, no face-side per-record join |
| A Codex session's TUI runs the enforced mode too: no approval prompts, full access | [runner § smart launchers](../../docs/06-launchers.md#smart-launchers) | 2026-09-23 owner answer (option a); the TUI starts its thread with its own settings, so an App-Server-only mode never reached the session |
| Push retries a suspended principal every 30 minutes; only an unknown credential and a second reader stop it | [runner § adapters](../../docs/06-launchers.md#adapters) | 2026-09-23 owner answer; suspension is reversible, so push resumes after reactivation |
| The anonymous page carries the project's description, links and picture | [discovery § rules it is built to](../../docs/05-discovery.md#rules-it-is-built-to) | 2026-09-23 owner instruction; the picture is a downscaled copy this node serves, like every other asset |
| The CLI names an Agent with `--agent`, adding a missing `#` | [actor terms](../../docs/constitution.md#actors-and-ascii-textarea-syntax) | 2026-09-22 owner answer; an unquoted `#` is a shell comment, so the flag spares quoting and never doubles the prefix |
| Every entity is added and edited by one form, each on its own page | [register](../../src/web/web-face/records.md#register), [compact administration pages](../../docs/05-discovery.md#compact-administration-pages) | 2026-09-19 owner instruction; the two were separate markup and had drifted field by field — the settings panel offered a queue policy to kinds that hold no queue and no credential to the one kind that has one, while the registration page asked for neither Maintainers nor an agent's inbox policy. One field set per kind, rendered by both, makes a field added for a kind appear on both. The editor leaves the detail page for an address of its own, so a refused field returns to the form rather than to a panel inside a page. The same holds for a 👤 user and a 👥 group, whose two copies had drifted in what they said about a GitHub login and how much of a membership list they showed |
| The daemon owner and a record's Maintainers carry a glyph | [identity labels](../../docs/05-discovery.md#identity-labels-in-web-and-cli), [glyphs](../../docs/glossary.md#glyphs) | 2026-09-18 owner instruction; the first authorities to be marked. Neither can appear on enough rows to violate the no-glyph default, which a daemon administrator, a record Owner and a Member would |
| An agent's record is labelled `👾` | [identity labels](../../docs/05-discovery.md#identity-labels-in-web-and-cli) | 2026-09-18 owner decision; supersedes the `📥 Inbox` label of 0.5.84 while the channels placement that decision also made stands. Built in 0.6.1 |
| Plain-text ACL editing | [ACL editing](../../docs/05-discovery.md#acl-editing) | 2026-09-16 owner instruction; display glyphs are not input syntax |
| Entity labels in web and CLI | [display labels](../../docs/05-discovery.md#identity-labels-in-web-and-cli) | 2026-09-17 owner-selected `👾` for Agent, superseded 2026-09-18 by `📥 Inbox`; implemented from the shared daemon-kind display mapping in 0.5.61. Compact WEB name prefixes and Group landed in 0.5.60; plain machine values retained |
| Dashboard is a loopback address, not a borrowed hostname | [where it listens](../../docs/05-discovery.md#where-it-listens) | 2026-09-15 owner instruction; the name's certificate had expired |
| The API root's redirect to the dashboard is permanent | [where it listens](../../docs/05-discovery.md#where-it-listens) | 2026-09-15 owner instruction |
| Coordinated runtime and bus rename | [explicit session rename](../../docs/06-launchers.md#explicit-session-rename) | 2026-09-13 owner clarification |
| Human-readable CLI listings | [CLI listing](../../docs/05-discovery.md#cli-listing) | 2026-09-13 owner instruction |
| Launcher address follows renamed session on restart | [session names](../../docs/06-launchers.md#session-names) | 2026-09-13 owner revision |
| Launcher addresses use template and instance naming | [session names](../../docs/06-launchers.md#session-names) | 2026-09-13 owner instruction |
| Launcher local socket discovery | [smart launchers](../../docs/06-launchers.md#smart-launchers) | 2026-09-13 owner instruction |
| CLI address precedence and local socket discovery | [local socket](../../docs/02-access.md#local-socket) | 2026-09-13 owner instruction |
| Codex delivery form | [definition](../../docs/06-launchers.md#runtime-integration-delivery) | 2026-09-13 owner clarification |
| Smart runtime launchers | [definition](../../docs/06-launchers.md#smart-launchers) | 2026-09-13 owner instruction |
| Assigned session names before directory fallback | [definition](../../docs/06-launchers.md#session-names) | 2026-09-13 owner instruction |
| Numbered duplicate session names | [definition](../../docs/06-launchers.md#session-names) | 2026-09-13 owner instruction |
| Rules it is built to | [definition](../../docs/05-discovery.md#rules-it-is-built-to) | D241, D242, D244 |
| Signing in | [definition](../../docs/05-discovery.md#signing-in) | D245, D246 |
| Node-wide counters are visible to anyone who may ask | [dashboard](../../docs/05-discovery.md#dashboard) | 2026-09-15 — They describe the daemon, not the caller, so there is no narrower audience; no access is the whole restriction, and a page must label node scope against caller scope |
| A certificate asked for and missing refuses the start | [where it listens](../../docs/05-discovery.md#where-it-listens) | 2026-09-15 — Somebody who wanted HTTPS would otherwise get a log line they do not read and a page that is not encrypted; refusing is the only answer they cannot miss |
| Registration is a page per record kind: the two channel kinds get one form each, and a service form carries its secret | [compact administration pages](../../docs/05-discovery.md#compact-administration-pages), [secrets](../../docs/03-records-service.md#secrets), [forms](../../src/web/web-face/records.md#register) | 2026-09-19 — Owner instruction. The kinds do not ask the same questions — a queue declares the policy of the queue it will hold and a pub/sub topic has no queue to bound — so one form with a kind control had to change shape underneath the answer. The secret is a second call to its own verb, because a registration carries neither the bytes nor their digest; the field is never repopulated, not even after a refusal, and a registration whose secret call fails says exactly that rather than claiming either outcome |
| A multi-line form value is normalised to the bytes that were typed | [secrets](../../docs/03-records-service.md#secrets) | 2026-09-19 — A browser submits a textarea with CRLF whatever the page was served with, and a service secret is stored exactly as sent, so a two-line credential entered on the page would otherwise be stored with a carriage return nobody typed and would not match the same credential set from the CLI |
| Record detail has one editor: settings, access, classification and Maintainers together, with owner-only fields disabled rather than hidden; supersedes the separate Classification and sharing section | [compact administration pages](../../docs/05-discovery.md#compact-administration-pages), [forms](../../src/web/web-face/records.md#settings) | 2026-09-19 — Owner instruction. The split put the same allow list behind two doors and made the page's shape depend on who was reading it. A disabled control submits nothing, so the form states in a hidden flag whether it carried the owner-only fields at all; without that a Maintainer saving a description would clear the Maintainers list it was never offered. The daemon still decides: a forged flag is refused |
| Record Maintainers use the same line-list editor pattern as ACLs and become a list of named principals and groups | [record authority](../../docs/01-identity-and-authority.md#record-authority) | 2026-09-17 — Owner instruction; replaces the planned single-group selector while preserving owner-only assignment |
| Dangerous resource controls leave ordinary detail | [resource Danger Zone](../../docs/05-discovery.md#resource-danger-zone) | 2026-09-17 — Owner instruction; the red link keeps configuration replacement, transfer and removal on their own server-rendered path |
| Deactivation starts in the Danger Zone | [resource Danger Zone](../../docs/05-discovery.md#resource-danger-zone) | 2026-09-29 — Owner instruction; active registry items with a deactivation action lead through the Danger Zone to confirmation |

## Versioning and project process

| Decision topic | Substance | Why |
|---|---|---|
| No compatibility obligation before R1.2 | [record kinds](../../docs/03-records.md#record-kinds) | 2026-09-18 owner decision; no migration feature is written. The upgrade still needs a stated restore rule and a cutover, which are [J.11 and J.12](R0.8-MVP/0.6.0-TODO.md#remaining-work) |
| Current and future documentation ownership | [working rules](../../CLAUDE.md#working-rules) | 2026-09-13 owner instruction |
| MVP installs by its own script only, with no npm path | [install](../../docs/07-setup.md#install) | 2026-09-15 — One supported way in is what installation acceptance can be run against; npm publication stays R1 |

## Subscriptions and scheduled sends

| Decision topic | Substance | Why |
|---|---|---|
| A subscription forward whose delivery cannot be confirmed pauses that subscription with a visible error; a definite refusal retries with backoff (RD138, Q207) | [the shared behaviour](../../docs/agents/subscriptions.md#the-shared-behaviour), [cron](../../docs/agents/cron.md#what-it-does) | 2026-10-28 owner decision, replaced by RD161 (2026-10-29): a refused copy, or one whose acceptance is unknown, is dropped with one warning and the subscription stays active; a one-shot is dropped and removed; nothing is held, retried or paused. A confirmed delivery is still never resent |

