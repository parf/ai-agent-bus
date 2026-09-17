# MVP decisions

📌 **TL;DR:** Decision index; linked contracts define the rule and implementation status.

## Settled

Migrated 2026-09-13. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| Web filesystem and process confinement | [web authority boundary](11-processes.md#web-authority-boundary) | 2026-09-17 G.1.3 implementation; explicit input mounts and environment, no unconfined fallback |
| SSH forced-command execution and entitlement | [SSH administration](09-setup.md#ssh-admin), [account shell](09-setup.md#the-two-accounts) | 2026-09-17 H.5.2 implementation; repairs the previously blocked token delegation and forced-command execution |
| Metadata refresh preserves omitted ACL settings | [registration](01-identity-and-roles.md#registration) | 2026-09-17 implementation review; explicit replacement and management clearing retained in 0.5.44 |
| Personal ACLs exclude groups | [Personal assignment limits](03-services-and-topics.md#personal-and-shared) | 2026-09-17 owner clarification; resolves Q71, including groups composed only of services; implementation pending |
| ACL governs access by other principals | [access rules](02-access.md#acl) | 2026-09-17 owner confirmation; own-inbox right retained in the 0.5.44 default change |
| Resource owners choose groups; Administrators control membership | [group authority](01-identity-and-roles.md#groups) | 2026-09-17 owner decision; confirms existing indirect acquisition of resource authority |
| Broader transfer-recipient eligibility deferred | [ownership](01-identity-and-roles.md#ownership) | 2026-09-17 owner direction; rare case, existing conditions retained |
| Startup revocation failure hardening deferred | [existing failure behavior](02-access.md#ownerless-credentials) | 2026-09-17 owner direction; Q69 removed from open questions, implementation unchanged |
| One reader count includes filtered waits | [reader observation](05-discovery.md#readers) | 2026-09-17 owner decision; resolves Q70, implementation pending |
| Explicit inbox selection, independent message filters | [consume syntax](04-messaging.md#inbox-selection-and-filters) | 2026-09-17 owner decision; resolves Q21, implementation pending |
| Draining an inactive identity's inbox | [user-state access](01-identity-and-roles.md#user-states) | 2026-09-17 owner decision; resolves Q63, confirming existing behavior |
| Explicit wildcard for broad user access | [ACL grants](02-access.md#acl) | 2026-09-16 owner clarification |
| Empty ACL retains resource-management access | [ACL default](02-access.md#acl) | 2026-09-16 owner correction; implemented in 0.5.44 |
| Owner-tagged Personal services | [web grouping and assignment limits](03-services-and-topics.md#personal-and-shared) | 2026-09-16 instruction, clarified 2026-09-17; ordinary access policy retained, implementation pending |
| Plain-text ACL editing | [ACL editing](05-discovery.md#acl-editing) | 2026-09-16 owner instruction; display glyphs are not input syntax |
| Entity labels in web and CLI | [display labels](05-discovery.md#identity-labels-in-web-and-cli) | 2026-09-16 owner-selected glyphs for web and human-readable CLI output |
| Protected identity fields in self-service profile editing | [profile permissions](01-identity-and-roles.md#users-and-profiles) | 2026-09-16 owner clarification; accepted, implementation pending |
| Separate administrative and resource-maintenance roles | [role names](01-identity-and-roles.md#role-names-and-scopes), [shared management](01-identity-and-roles.md#groups) | 2026-09-16 owner clarification; replaces the shared daemon/service “Maintainer” terminology. Earlier dated rows retain historical wording |
| Retained exchange correlation preserves references and qualifies inferred responses | [retained exchanges](05-discovery.md#retained-exchanges) | 2026-09-15 authorized web implementation; F.13.5 partial |
| A full queue answers `429`, not `503` | [overflow](04-messaging.md#overflow) | 2026-09-15 owner instruction; `503` is left to a service that is briefly unavailable |
| Dashboard is a loopback address, not a borrowed hostname | [where it listens](05-discovery.md#where-it-listens) | 2026-09-15 owner instruction; the name's certificate had expired |
| The API root's redirect to the dashboard is permanent | [where it listens](05-discovery.md#where-it-listens) | 2026-09-15 owner instruction |
| Service method information is the description | [service and template](03-services-and-topics.md#service-and-template) | 2026-09-15 owner instruction; resolves Q7 |
| Coordinated runtime and bus rename | [explicit session rename](08-runner-role.md#explicit-session-rename) | 2026-09-13 owner clarification |
| Launcher terminal titles, palettes and restoration | [terminal appearance](08-runner-role.md#terminal-appearance) | 2026-09-13 owner instruction |
| OpenCode explicit session binding | [adapters](08-runner-role.md#adapters) | 2026-09-13 installed launcher verification |
| OpenCode user-wrapper precedence | [running the launchers](08-runner-role.md#running-the-launchers) | 2026-09-13 fish launcher verification |
| MVP web credential and state boundary | [web authority](11-processes.md#web-authority-boundary) | 2026-09-13 owner-approved release review |
| MVP runtime sidecar isolation and recovery | [runtime acceptance](08-runner-role.md#runtime-isolation-and-recovery) | 2026-09-13 owner-approved release review; endpoint authentication built 2026-09-17, interactive acceptance remains open |
| MVP upgrade, recovery and real SSH acceptance | [installation acceptance](09-setup.md#installation-acceptance) | 2026-09-13 owner-approved release review |
| MVP installed browser acceptance | [browser acceptance](05-discovery.md#browser-acceptance) | 2026-09-13 owner-approved release review |
| Administrative crash recovery | [policy status](04-messaging.md#administrative-crash-recovery) | 2026-09-13 owner-approved review; guarantee settled 2026-09-15, implemented 2026-09-17 in 0.5.47 |
| Project license | [terms](../LICENSE.md#polyform-noncommercial-license-100) | 2026-09-13 owner instruction |
| Human-readable CLI listings | [CLI listing](05-discovery.md#cli-listing) | 2026-09-13 owner instruction |
| Manual removal of idle registry addresses | [unregistering](01-identity-and-roles.md#unregistering) | 2026-09-13 owner request |
| A removed name keeps nothing; a name is protected by retiring it | [unregistering](01-identity-and-roles.md#unregistering), [R1.1 retirement](../Plans/R1.1/records.md#down-and-retired) | 2026-09-15 owner revision |
| Launcher address follows renamed session on restart | [session names](08-runner-role.md#session-names) | 2026-09-13 owner revision |
| Launcher addresses use template and instance naming | [session names](08-runner-role.md#session-names) | 2026-09-13 owner instruction |
| Credential lifetime across releases | [token lifetime](02-access.md#token-lifetime) | 2026-09-13 owner instruction |
| Launcher local socket discovery | [smart launchers](08-runner-role.md#smart-launchers) | 2026-09-13 owner instruction |
| CLI address precedence and local socket discovery | [local socket](02-access.md#local-socket) | 2026-09-13 owner instruction |
| Token helper local socket discovery | [local socket](02-access.md#local-socket) | 2026-09-13 installed token-command fix |
| Cross-account session access through the authenticated shared socket | [local socket](02-access.md#local-socket) | 2026-09-13 installed-launcher fix |
| Versioning | [definition](../CLAUDE.md#versioning) | D1 |
| Build information | [definition](09-setup.md#build-information) | D2 |
| Process titles | [definition](11-processes.md#process-titles) | D3 |
| Names | [definition](01-identity-and-roles.md#names) | D4, D5, D67, D68, D116, D126, D127 |
| The two accounts | [definition](09-setup.md#the-two-accounts) | D8, D14, D195 |
| The programs | [definition](09-setup.md#the-programs) | D9, D10, D13 |
| Getting a token | [definition](02-access.md#getting-a-token) | D11, D43, D44, D47 |
| Proving possession | [definition](02-access.md#proving-possession) | D12, D33, D34, D35 |
| Acl | [definition](02-access.md#acl) | D15, D25, D36 |
| Registration | [definition](01-identity-and-roles.md#registration) | D16, D17 |
| Conditional new-name registration | [definition](01-identity-and-roles.md#registration) | 2026-09-13; enforces the owner's concurrent-session uniqueness requirement |
| Profile fields | [definition](01-identity-and-roles.md#users-and-profiles) | D19, D248 |
| Who may write a record | [definition](01-identity-and-roles.md#users-and-profiles) | D20, D21 |
| Every identifying field is unique | [definition](01-identity-and-roles.md#users-and-profiles) | D22 |
| Audience | [definition](05-discovery.md#audience) | D31 |
| Encrypted sessions | [definition](02-access.md#trust-boundary) | D32 |
| Ownership | [definition](01-identity-and-roles.md#ownership) | D39, D52 |
| What a call carries | [definition](02-access.md#what-a-call-carries) | D42 |
| Token scope | [definition](02-access.md#what-a-call-carries) | D45 |
| Local socket | [definition](02-access.md#local-socket) | D48, D56, D57 |
| Token lifetime | [definition](02-access.md#token-lifetime) | D49, D50, D51 |
| Storage | [definition](09-setup.md#storage) | D53 |
| Durability | [definition](04-messaging.md#durability) | D54, D55 |
| Service and template | [definition](03-services-and-topics.md#service-and-template) | D70, D71, D76, D77 |
| Configuring a template | [definition](03-services-and-topics.md#configuring-a-template) | D72, D73, D74, D184 |
| Why a digest at all | [definition](03-services-and-topics.md#why-a-digest-at-all) | D75 |
| Topics | [definition](03-services-and-topics.md#topics) | D78 |
| Inbox queues | [definition](04-messaging.md#inbox-queues) | D81, D185, D231 |
| Verbs | [definition](04-messaging.md#verbs) | D82, D237 |
| Message fields | [definition](04-messaging.md#message-fields) | D83 |
| Receipts | [definition](04-messaging.md#receipts) | D84, D229, D230, D253 |
| Message ttl | [definition](04-messaging.md#message-ttl) | D85, D255 |
| Push and pull | [definition](04-messaging.md#push-and-pull) | D86, D176 |
| Overflow | [definition](04-messaging.md#overflow) | D87, D235 |
| Envelope | [definition](04-messaging.md#envelope) | D89 |
| What a listing answers | [definition](05-discovery.md#what-a-listing-answers) | D90, D91, D92, D93, D188 |
| What it shows | [definition](05-discovery.md#what-it-shows) | D94, D247 |
| Refusals | [definition](05-discovery.md#refusals) | D95 |
| Dashboard | [definition](05-discovery.md#dashboard) | D96, D97, D98 |
| Where it listens | [definition](05-discovery.md#where-it-listens) | D99 |
| What is shared | [definition](11-processes.md#what-is-shared) | D101, D192 |
| How a child is started | [definition](11-processes.md#how-a-child-is-started) | D102 |
| Adapters | [definition](08-runner-role.md#adapters) | D109, D177 |
| MVP runtime integration delivery | [definition](08-runner-role.md#runtime-integration-delivery) | 2026-09-13 owner instruction |
| Codex delivery form | [definition](08-runner-role.md#runtime-integration-delivery) | 2026-09-13 owner clarification; resolves Q36 |
| Smart runtime launchers | [definition](08-runner-role.md#smart-launchers) | 2026-09-13 owner instruction |
| Assigned session names before directory fallback | [definition](08-runner-role.md#session-names) | 2026-09-13 owner instruction |
| Numbered duplicate session names | [definition](08-runner-role.md#session-names) | 2026-09-13 owner instruction |
| Launcher automatic execution and continuation | [definition](08-runner-role.md#smart-launchers) | 2026-09-13 owner instruction; replaces preserving caller-selected execution modes |
| Runtime MCP minimum | [definition](05-discovery.md#mcp-minimum) | 2026-09-13 owner instruction |
| 12-stages | [definition](12-stages.md#stages) | D110 |
| Request and reply | [definition](04-messaging.md#request-and-reply) | D112, D227, D236, D238, D239, D240 |
| Script services | [definition](08-runner-role.md#script-services) | D117, D250, D251, D252, D254 |
| One reader per inbox | [definition](04-messaging.md#one-reader-per-inbox) | D169, D172, D220, D221, D222 |
| Reply routing | [definition](04-messaging.md#reply-routing) | D170 |
| Languages | [definition](10-modules.md#languages) | D179 |
| External tools | [definition](10-modules.md#external-tools) | D180, D182, D216 |
| Http is built in | [definition](10-modules.md#external-tools) | D181 |
| Our own small module | [definition](10-modules.md#external-tools) | D183 |
| How to call it | [definition](03-services-and-topics.md#how-to-call-it) | D186, D187 |
| The rule | [definition](10-modules.md#the-rule) | D189, D215, D217 |
| The rule | [definition](11-processes.md#the-rule) | D190, D193 |
| The processes | [definition](11-processes.md#the-processes) | D191 |
| Nothing the daemon runs may exec | [definition](11-processes.md#nothing-the-daemon-runs-may-exec) | D194 |
| The three doors | [definition](02-access.md#what-a-call-carries) | D196 |
| Why the supervisor holds cap_chown | [definition](11-processes.md#why-the-supervisor-holds-cap_chown) | D214 |
| Goal | [definition](00-overview.md#goal) | D218 |
| Sandboxing | [definition](08-runner-role.md#sandboxing) | D223, D224 |
| Stopping it and reading what it said | [definition](08-runner-role.md#stopping-it-and-reading-what-it-said) | D225, D226 |
| Several readers may wait when they say so | [definition](04-messaging.md#several-readers-may-wait-when-they-say-so) | D228 |
| Subscribers | [definition](04-messaging.md#subscribers) | D232, D233, D234 |
| Rules it is built to | [definition](05-discovery.md#rules-it-is-built-to) | D241, D242, D244 |
| Signing in | [definition](05-discovery.md#signing-in) | D245, D246 |
| Current and future documentation ownership | [working rules](../CLAUDE.md#working-rules) | 2026-09-13 owner instruction |

## Dashboard scope revision

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Required administrative dashboard | Owner confirms missing requirements and the MVP/R1 split | [required tabs](05-discovery.md#required-tabs) |
| 2026-09-13 | Owner and maintainers in MVP | Owner explicitly retains the authority model in required scope | [groups and maintainers](01-identity-and-roles.md#groups) |

## Service owner authority

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Full control of owned services | Owner clarifies that ordinary users need no daemon administration role to manage their services | [owner control](01-identity-and-roles.md#services) |

## Dashboard implementation defaults

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Local user administration before AUTH | Implement the accepted owner/maintainer hierarchy with a protected flat group; resolves Q29 | [groups and maintainers](01-identity-and-roles.md#groups) |
| 2026-09-13 | Profile identifier normalization | Initial implementation default without provider alias merging; resolves Q28 | [identifier uniqueness](01-identity-and-roles.md#users-and-profiles) |
| 2026-09-13 | User pause and ban behavior | Initial implementation default preserves credentials and queued work; resolves Q37 | [user lifecycle](01-identity-and-roles.md#user-states) |
| 2026-09-13 | Bounded dashboard history | Initial implementation default keeps collection independent of page visits; resolves Q38 | [activity history](05-discovery.md#activity-history) |
| 2026-09-15 | A person's credential lasts as long as they are a registered user | It is how they call at all, so nothing underneath it ends it; registration is what does | [token lifetime](02-access.md#token-lifetime) |
| 2026-09-15 | A credential nobody owns is dropped at daemon start; resolves Q51 | A sweep at a known moment, not expiry — tokens still never retire for being old or idle; this clears what the old outlives-the-address rule left behind | [ownerless credentials](02-access.md#ownerless-credentials) |
| 2026-09-15 | Node-wide counters are visible to anyone who may ask; resolves Q50 | They describe the daemon, not the caller, so there is no narrower audience; no access is the whole restriction, and a page must label node scope against caller scope | [dashboard](05-discovery.md#dashboard) |
| 2026-09-15 | A certificate asked for and missing refuses the start; resolves Q52 | Somebody who wanted HTTPS would otherwise get a log line they do not read and a page that is not encrypted; refusing is the only answer they cannot miss | [where it listens](05-discovery.md#where-it-listens) |
| 2026-09-15 | An acknowledged ban, group removal or ACL restriction holds until explicitly lifted; resolves Q39 | Acknowledging is a promise, and a crash is not a way out of one; if the durability cannot be promised the acknowledgement must not be given | [administrative crash recovery](04-messaging.md#administrative-crash-recovery) |
| 2026-09-15 | A service whose owner is not a user is deleted at once, with its credential and all its queues; resolves Q40, and Q53 with it | Every owner is a user and a user is never deleted, so this is wreckage rather than a state to run in — nothing can be told to drain a queue addressed to something no principal answers for, and a name nothing holds is free | [when the owner is gone](01-identity-and-roles.md#orphaned-records) |
| 2026-09-15 | A service owned by a paused or banned user refuses calls, in the daemon | The state is about what that person's names may do, and a page that labelled the owner while the service still answered would describe a rule nobody applied; it deletes nothing and kills nothing, or a ban could not be lifted | [services of a user who is paused or banned](01-identity-and-roles.md#user-states) |
| 2026-09-15 | The daemon may run `ssh-keygen`; the boundary is no user services, not no exec; resolves Q30 | A fixed verifier the daemon ships and calls with its own arguments is not a stranger's program, and what a user supplied is what the rule is about | [nothing the daemon runs may exec](11-processes.md#nothing-the-daemon-runs-may-exec) |
| 2026-09-15 | MVP installs by its own script only, with no npm path; resolves Q10 | One supported way in is what installation acceptance can be run against; npm publication stays R1 | [install](09-setup.md#install) |
| 2026-09-15 | The account map is administered by operator SSH key or user token, and nothing new; resolves Q9 | The entitlement is already the principal in the key's entry, so who may edit the map is answered by what answers everything else | [administering the account map](09-setup.md#administering-the-account-map) |
| 2026-09-16 | The dashboard's “advanced” is information design, not partial updates: no enhancement layer, and the no-script rule stands unchanged; resolves Q58 | Not one of the seventeen audit findings or sixteen inventory findings is fixed by a partial DOM update — they are architecture, form design, state language and data shape. Inline handler attributes are script under another name on the one page that carries the node's whole envelope feed, and adoption is a one-way door for the templates. The declarative platform also closed the gap the rule used to cost: menus, disclosures and dialogs are authorable without script | [rules it is built to](05-discovery.md#rules-it-is-built-to), [interaction without script](../Plans/MVP/web/components.md#interaction-without-script) |
| 2026-09-16 | The dashboard ships a house layer — tokens, layout primitives and hand-authored components — rather than adopting a design system; resolves Q59 | The component surface is nine page types against thirty-nine lines of existing CSS, and no framework we examined supplies the one component that matters — a table driven by real URLs rather than client-side state. The systems that would survive the no-script rule supply a styling baseline with no operations patterns in it. Patterns are copied and credited; packages are not depended on. A like-for-like component comparison was owed and not run ([dissent](../Plans/MVP/web/README.md#dissent)) | [visual design](../Plans/MVP/web/visual-design.md#what-ships) |
| 2026-09-16 | The dashboard is authored in `templ`; resolves Q60 | A component whose mandatory arguments are declared makes the structural obligations — a table's caption and scoped headers, a field's label and error slot — compile errors rather than discipline, which is the class of defect the audit kept finding. The guarantee comes from designing the component API that way, not from the tool: `templ` compiles a component to a Go function whose parameters its author chooses. Decided apart from the script question on purpose: `templ` renders server-side and buys no JavaScript, so accepting modern Go could not smuggle in a relaxation of the rule | [rendering](../Plans/MVP/web/technology.md#rendering) |
| 2026-09-16 | The dashboard ships **one design and no themes**: a single light palette, no scheme control, no density control and no stored preference; supersedes Q61 and settles Q62's control half | Owner instruction: *one good design for admin panels, no themes*. Two schemes are two designs to keep in step, and the second was never the thing that made the interface good. One palette means one contrast obligation instead of a doubled one, no preferences route, no cookie, no persistence scope, and no page whose appearance depends on state the daemon does not hold. Light, because an operations console sits beside other light tooling and every system this plan borrows patterns from defaults that way. The container query stays: it answers the width a table has, which is a fact about the layout rather than a preference about the person | [colour](../Plans/MVP/web/tokens.md#colour), [theme and density controls](../Plans/MVP/web/visual-design.md#theme-and-density-controls) |
| 2026-09-16 | The daemon owner owns the dashboard token file; resolves Q62 | The recorded risk is *amateur if nobody owns typography and density*, and it is conditional on the ownership rather than on the tokens. The five acceptance checks make a lapse visible; they cannot create the ownership. An existing role rather than a new appointment means no post to leave vacant, and one design rather than two makes the post small enough to actually hold | [risk, recorded](../Plans/MVP/web/visual-design.md#risk-recorded) |
| 2026-09-16 | The web panel acts on the visitor's token and on nothing else | Owner instruction. What the dashboard can do is what the signed-in person could do from the CLI: no privileged fallback when a call is refused, no authority held between requests, and permissions rendered from what the daemon answered rather than recomputed in the face — two implementations of the access rules disagree, and the disagreement that matters is a page offering an action the daemon will refuse | [web authority](11-processes.md#web-authority-boundary) |
| 2026-09-15 | No registration, no access | The record is what access hangs on; a credential whose record is gone has nothing left to hold, and the page said the opposite | [unregistering](01-identity-and-roles.md#unregistering) |
| 2026-09-15 | A group is neither deleted nor given states; a group is retired by emptying its membership; resolves Q54 | Removing a name other records point at silently changes what they mean, and a group is not a principal to pause — *inactive* could only mean its members stop counting, which an empty list already says, with no second concept | [groups and maintainers](01-identity-and-roles.md#groups) |
| 2026-09-15 | A suspended owner's service answers `403 suspended`, the caller's own reason widened rather than a second one; resolves Q55 | It is one suspension seen from two sides, and the caller can act on neither: no credential and no grant makes a suspended name answer, which is what `403` already tells them | [refusals](05-discovery.md#refusals) |
| 2026-09-15 | An ownerless credential may be removed by hand by the daemon owner or a maintainer, and the start-of-day sweep stays; resolves Q56 | Maintainers already administer users and this is the user directory; the access order does not settle it, because a credential nobody owns is not a level below anybody | [ownerless credentials](02-access.md#ownerless-credentials) |
| 2026-09-15 | Who the caller is, and what they may do to what they are touching, is settled under the hold the operation writes under — not at the gate, and not before the write | The gate released the registry before core took it, so every verb was a check-then-act on a stale answer: a caller whose record was removed mid-request registered itself back into existence, a caller paused after the gate was served, and `/token` established ownership before minting, so a transfer in between handed the former owner the current owner’s credential. Closing it for one verb would have left the class, so it is closed at the predicates every verb goes through, and the two operations that write in two places — issuing, and removing an address with its credential — became single held operations | [what a call carries](02-access.md#what-a-call-carries), [getting a token](02-access.md#getting-a-token), [unregistering](01-identity-and-roles.md#unregistering) |
| 2026-09-15 | A credential alone is not a principal: an unregistered name is refused `401` on everything, and is not issued a credential in the first place; resolves Q57 | Unknown was reading as active, so a name the daemon held nothing for could call and could leave a service owned by nobody — the wreckage the deletion rule exists to clean up, created by an ordinary call. Owner instruction: an unregistered name can do nothing at all, self-registration included, so the bootstrap is closed at the source rather than left as an exception: a name is created by somebody already here, and only then holds a credential | [what a call carries](02-access.md#what-a-call-carries), [getting a token](02-access.md#getting-a-token) |
| 2026-09-15 | A record is not removed while its name still owns others, unless that name is a registered user | Removing a name's only standing while it owns services strands each of them under an owner nothing answers for; refusing names what is in the way, and a user survives losing a record because the user is still somebody | [unregistering](01-identity-and-roles.md#unregistering) |
| 2026-09-15 | Keep non-user identities visible and distinguish them from registered users | Cleanup needs visible evidence rather than a directory that hides debris | [person records](01-identity-and-roles.md#users-and-profiles) |
| 2026-09-16 | An owner is known by having a profile or a record of its own; the question is asked one step and never walked to a person | A self-owned record with no user behind it is a principal the daemon supports — it authenticates, may be handed a record by transfer and may register records of its own — so reachability to a user would delete names the daemon had just accepted | [when the owner is gone](01-identity-and-roles.md#orphaned-records) |
| 2026-09-16 | A deleted name's group membership goes with it | A freed name is reclaimable by anybody, so a membership left behind is inherited rather than stale: whoever registers the name next arrives in every group the old holder was in, and reaches every record those groups allow | [unregistering](01-identity-and-roles.md#unregistering) |
| 2026-09-16 | A node publishes a closed list — release, build, host name, daemon owner, uptime and calls served — to anybody, signed in or not, in the shell every page shares and on the sign-in page | Owner-asked. Somebody who reaches a bus they hold no credential for should be able to tell what it is and whose it is without asking anybody, which is precisely the person a sign-in page leaves with nothing. Each cost was put to the owner and accepted, the traffic figures explicitly: an unauthenticated visitor learns the host's name, who runs the node, how long it has been up and how much it carries. The list was cut back at 0.5.39, when the owner removed the host load average and the message counters in favour of the daemon's own served-call counts. Nothing else becomes readable — no record, no principal, no refusal count — and the face gains no privilege, since the daemon answers this list to any caller | [what a node says about itself](05-discovery.md#what-a-node-says-about-itself), [web authority boundary](11-processes.md#web-authority-boundary) |
| 2026-09-16 | The header prints a window's label without its measured span | Owner decision at 0.5.40, taken with the cost stated: readings are a minute apart, so `minute:` usually covers more than a minute and the header no longer says how much. `uptime:` beside it shows how young the node is and was judged enough for a glance. The measurement is unchanged and `observed` still travels on `GET /identity`, so this narrows what the page shows, not what the daemon knows | [what a node says about itself](05-discovery.md#what-a-node-says-about-itself) |

| 2026-09-16 | Use an inline bus mark derived from the project artwork in the shared page header | Owner-requested; no external asset or additional public route | [what a node says about itself](05-discovery.md#what-a-node-says-about-itself) |

| 2026-09-16 | Implement the Administrator naming distinction | Preserve existing grants during the terminology change | [upgrade migration](09-setup.md#administrator-name-migration) |

## Superseded

| Earlier design | Replacement |
|---|---|
| Owner-controlled effective Maintainer membership (owner answer 2026-09-16; never implemented) | [group authority](01-identity-and-roles.md#groups) — supersedes that owner answer by the owner's revised decision on 2026-09-17 |
| Empty ACL admits only the record owner (2026-09-16; never implemented) | [ACL default](02-access.md#acl) — corrected by the owner the same day |
| The shared owner/maintainer/user vocabulary for daemon and resource authority (2026-09-15) | [Scoped role names](01-identity-and-roles.md#role-names-and-scopes) — replaced by owner clarification on 2026-09-16 |
| The dashboard is `https://agent-bus.localhost.direct`, with a certificate under that name, port 443 and an 8443 fallback | [where it listens](05-discovery.md#where-it-listens) |
| MVP dashboard permits only sign-in/out; basic groups and activity charts wait for R1 | [required tabs](05-discovery.md#required-tabs) |
| The dashboard ships both colour schemes, carried by the token layer from the start (2026-09-16, resolving Q61) | [one design, no themes](../Plans/MVP/web/tokens.md#colour) — owner instruction the same day |
| Maintainers require AUTH and are absent from MVP | [groups and maintainers](01-identity-and-roles.md#groups) |
| An unregistered name stays reserved to its owner and keeps a valid credential | [unregistering](01-identity-and-roles.md#unregistering); protecting a name is now [R1.1 retirement](../Plans/R1.1/records.md#down-and-retired) |
| A record whose owner nobody answers to stays alive and waits for the daemon owner to adopt it | [when the owner is gone](01-identity-and-roles.md#orphaned-records) |
| A change to a user's state leaves the services they own answering | [services of a user who is paused or banned](01-identity-and-roles.md#user-states) |
| A group is retired by an *inactive* or *banned* state, as a person is | [groups and maintainers](01-identity-and-roles.md#groups) |

## Open

Unresolved choices live in [MVP questions](../Plans/MVP/QUESTIONS.md#open-questions).

## History

Original wording and superseded choices are preserved in [decision history](../Plans/MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../Plans/MVP/done/document-migration.md#decision-mapping).
