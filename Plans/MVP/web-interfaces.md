# MVP web interfaces

**Superseded in part.** The owner reopened this as a full redesign on
2026-09-16, and the current specifications live in
[the redesign directory](web/README.md#what-each-document-owns): architecture,
pages, forms, components, the data dictionary, glyphs, visual design and
technology, with peer review beside them. This file remains the earlier
proposal and the plan's topic entry; where the two disagree the redesign is
newer. Its [architecture recommendation](#architecture-recommendation) in
particular is superseded by the owner's decisions on
[script, design system, rendering and colour scheme](../../docs/decisions.md#settled).

## Proposal

Status: individually requested slices are built; the remaining redesign depends on F.13.0, the owner’s specification review. [Completed slices](DONE.md#done--mvp) do not close the remaining [acceptance work](TODO.md#web-redesign). Replace the collection of administrative forms with one coherent interface for finding services, understanding delivery, and managing access. Keep Go rendering, make existing information discoverable, and fix misleading states before adding more telemetry. The owner should be able to recognise a session, identify a stalled inbox, and understand the consequence of an administrative action without interpreting API terminology.

Read the [audit](done/web-review.md#findings) for evidence, [data coverage](#data-coverage) for what is actually available, and [execution plan](TODO.md#web-redesign) for dependencies and falsifiable acceptance. Recommendations here are not settled product contracts; acceptance promotes them through the [document lifecycle](../../CLAUDE.md#working-rules).

## Requirements recovered

The owner already requested an administrative application. The gap is not a missing list of tabs: much of the required information exists but is poorly connected, partially presented, or mislabeled.

| Requirement source | Consequence for this redesign |
|---|---|
| [Required dashboard](../../docs/05-discovery.md#required-tabs) | Preserve service, channel, user, group and activity workflows; a visual overview cannot replace their controls |
| [Existing diagnostics](../../docs/05-discovery.md#what-it-shows) | Keep backlog, exchange, loss, refusal, node and credential views reachable, even when the homepage becomes shorter |
| [Owner control](../../docs/01-identity-and-roles.md#services) and [maintainer scopes](../../docs/01-identity-and-roles.md#groups) | Show the relevant actions and their scope; daemon administration is not ownership of every service |
| [Discovery observations](../../docs/05-discovery.md#what-a-listing-answers) | Separate registration, permission to deliver, reader presence and queue condition |
| [Session names](../../docs/08-runner-role.md#session-names) | Show the familiar session description prominently beside its complete routing address |
| [Service method information](../../docs/03-services-and-topics.md#service-and-template) | Show the description before asking someone to use a service; do not promise a generated method browser |
| [Dashboard boundary](../../docs/05-discovery.md#dashboard) and [sign-in](../../docs/05-discovery.md#signing-in) | Preserve visitor-scoped data and administration throughout navigation and error recovery |
| [Current rendering rules](../../docs/05-discovery.md#rules-it-is-built-to) | The baseline design must work within the existing browser and asset restrictions |
| [Release split](../R1/discovery.md#dashboard-extensions) | Do not quietly bring managed processes, health checks or long-term analytics into MVP |

## Primary journeys

| Person's question | Shortest useful journey |
|---|---|
| What can I use, and who owns it? | Services → search by description or address → overview → owner/access and usage guidance |
| Is this my named AI session? | Services → Agents filter → session description and full address → observed reader state |
| Why is work not arriving? | Overview → queued-work warning → service queue → activity and retained exchanges, with visibility/window limits |
| Who may use or maintain my service? | Service → Access → review current policy → focused edit → same detail page with result |
| How does this channel deliver? | Channels → delivery mode and subscribers → recipient inbox links; queue mode gets queue information instead |
| What may this person administer? | Users → person detail → groups and owned services; distinguish daemon authority from record maintenance |
| Why can I not complete this action? | Inline explanation or error page → corrective step/back to preserved context; no raw backend response |

## Navigation and pages

Use a persistent application header with page identity, signed-in principal, an account link and sign out. A desktop side navigation becomes a wrapping or native disclosure navigation on narrow screens. Highlight the current location. Keep browser Back, direct links and ordinary form submission meaningful.

| Destination | First screen | Detail and actions |
|---|---|---|
| Overview | Observation time, clearly scoped counts, attention items ordered by consequence, and recently observed traffic | Link each backlog/loss/refusal item to its relevant filtered view. Empty sections become short statements rather than empty tables. Node facts remain a distinct section |
| Services | Searchable, sortable list with description/name, full address, kind, owner, administrative state, reader observation and queued work | Overview first; then Queue, Access and Configuration sections. Editing is a deliberate action; registration gets a dedicated form |
| Channels | List with description/address, delivery mode, owner, administrative state and mode-appropriate queue/subscriber summary | Mode explanation, queue policy, visible subscribers and access. Link each visible subscriber to its inbox. Keep subscriber opt-in distinct from owner removal |
| Activity | Scope selector, actual observed time range, recent traffic and exceptions, with labelled graphs and value table | Link from service/channel context with that filter retained. Explain the current interval and observation gaps next to the graph |
| Diagnostics | Backlogs, losses, refusals and the existing bounded exchange timeline, as focused sections | Search/filter within retained data; ordinary envelope details only. Node section explains restart history available today. This is not the R1 operations console |
| Users | Search by identity or supplied profile, authority/state filters, bounded result pages | Person/profile, authority, memberships and owned resources; separate editing and lifecycle actions. Keep uncertain credential-only identities visible with an honest category, not a count of people |
| Groups | Names, visible membership counts, protected state and visible resource references | A group detail page with member links and assignments; edit membership separately. Explain hidden membership instead of displaying an empty group |
| My account | Own identity and the existing credential-fingerprint view | Separate person identity from owned service identities, with lifecycle help and supported commands. No central credential administration expansion |
| Sign-in and recovery | A clear token label, local/SSH acquisition help, and an explicit sign-in action | Auth-required deep links return here with a validated local return destination. Distinguish expired session, permission refusal and unavailable bus without revealing hidden resources |

The overview is a summary of existing MVP views. It must not acquire the R1 operations tab's supervisor reports or alerts. All existing useful deep links should continue to resolve or redirect to their equivalent page; bookmarks must not silently land on unrelated diagnostics.

## Lists and detail pages

| Pattern | Proposed treatment |
|---|---|
| Find a named session | Description/display text first, full selectable address immediately below; do not discard the identity when the description is empty or duplicated |
| List controls | Search, scope, state and sort in one toolbar; filters and paging in the URL. Separate administrative state from reader observation. Include result count, active filters and clear-filters action |
| Large lists | Bounded pages with stable ordering and address tie-break. Preserve filters on paging, detail links and return. Count/filter only the daemon-authorized result set |
| Empty data | Distinguish no records, no matches, insufficient authority, no retained observations, and failed load. Never report a failed request as an empty healthy bus |
| Relationships | Link owners, groups, resources and subscribers where the caller can inspect them. When visibility is limited, explain the limit without probing forbidden identities |
| Read-only details | Show description, protocol/address, access summary and queue settings even when edit forms are unavailable; permission to edit must not determine whether visible metadata is readable |
| Changes | Edit one concern at a time. Return to the affected detail section with a specific success message and updated state |
| Dangerous changes | Separate availability, ownership transfer, credential/configuration consequences and removal. A server-rendered confirmation names the target and consequence; recheck authorization and current conditions at submission |
| Validation | Keep nonsensitive input, show an error summary and field errors, and preserve the original task. Do not echo a submitted token or private configuration back into HTML |

## State and language

| Current problem | Proposed language or behavior |
|---|---|
| “Active” can be mistaken for a live process | “Enabled” / “Disabled” describes administrative delivery state; explanatory text follows [owner control](../../docs/01-identity-and-roles.md#services) |
| “Serving” / “Offline” claims more than an instantaneous read observation proves | “Reader attached” / “No reader waiting”; explain that the latter does not establish process death. External protocol and pub/sub presentation must not imply a direct inbox reader is required |
| Every queued inbox is called stuck | Present “Queued work”; distinguish backlog with a reader from backlog without one. Show observed age and saturation rather than inventing a health threshold |
| In/out and waiting are ambiguous | Use accepted/dequeued and waiting reads; never equate dequeued with completed work |
| Omitted values print as blank labels | Use “Not supplied”, “Uses daemon default”, “No queued messages”, or “Not observed”, according to the actual fact |
| “Users” includes credentials without person provenance | Distinguish trusted profiles from unclassified identities using daemon evidence; do not classify a person from a slash or runtime prefix in a name |
| “Credentials remain valid” on removal is stale | Derive action help from [unregistering](../../docs/01-identity-and-roles.md#unregistering); do not reuse transfer's credential wording |
| A missing receipt looks like unfinished execution | “No completion observed in retained history”; absence from a bounded feed is not failure or pending work |
| Dates have no timezone and several meanings | Label registration update, observation time, credential issue/use and history interval separately; use one explicit display timezone, with full timestamps available |

The same declared-versus-observed distinction is used in [R1.1 record states](../R1.1/records.md#down-and-retired); this link does not promote those states into MVP.

## Data coverage

This is an inventory of existing information and missing observations, not a proposed schema. The snapshot followed reported test-record cleanup and is not evidence of the installation's normal workload. The [audit snapshot](done/web-review.md#live-data) records the inspected installation separately from these recommendations.

| Information | Availability and source | MVP presentation or gap |
|---|---|---|
| Registry identity and description | Built: [record source](../../src/internal/protocol/envelope.go), [listing](../../src/internal/core/bus.go), [launcher registration](../../src/launchers/identity.ts) | Already enough to make sessions recognisable. Description is user-supplied display text; runtime classification inferred from names is not authoritative |
| Owner, maintainers, ACL and allowed controls | Built: [management and visibility](../../src/internal/core/manage.go) | Expose in read-only summaries as well as forms; access scope stays with the daemon |
| Queue condition and totals | Built: [listing observations](../../docs/05-discovery.md#what-a-listing-answers) | Put oldest/full/loss information beside the relevant name; separate instantaneous observations from accumulated totals |
| Effective inherited queue settings | The daemon resolves defaults internally; [public records](../../src/internal/protocol/envelope.go) retain unset/inherited settings | Show inheritance honestly now. A resolved value requires a narrow daemon read addition; never copy daemon defaults into templates |
| Channels and subscriptions | Built: [topics](../../docs/03-services-and-topics.md#topics), [subscribers](../../docs/04-messaging.md#subscribers) | Display delivery mode and visible relationships; shared service layout currently conceals this distinction |
| Profiles, authority and owned services | Built: [user view construction](../../src/internal/core/users.go) | The answer merges profiles, self-owned records and unregistered credential holders. Exposing known origin helps current entries; a new field cannot reconstruct lost historical provenance. See the [credential investigation](done/web-review.md#credential-provenance) |
| Group membership and references | Membership is role-filtered; references can be joined from visible records: [groups](../../src/internal/core/manage.go) | Do not show membership-hidden as zero. A complete hidden-resource reference count would need an authorized daemon answer |
| Traffic history | Built: [sampling and aggregation](../../src/internal/core/activity.go) | Use actual timestamps, visible scope, restart boundary and partial-interval marking. Fan-out and read refusals make these unsuitable for a generic “successful calls” metric |
| Exchanges and receipts | Built [retained exchange view](../../docs/05-discovery.md#retained-exchanges) | [Correlation checks and browser fixture](done/exchange-evidence.md#checks) cover the implemented slice; installed acceptance remains pending |
| Node totals and refusal reasons | Built: [status response](../../src/internal/api/server.go), [global totals](../../src/internal/core/bus.go) | Global status and caller-filtered lists are different scopes. Visibility is settled — anybody who may ask sees the node's numbers ([dashboard](../../docs/05-discovery.md#dashboard)) — so what is left is labelling which scope a figure is |
| Credential fingerprints | Built: [held credentials](../../src/internal/auth/tokens.go), [names response](../../src/internal/api/server.go) | Move from the long diagnostics page to the account area; credential age is not automatic expiry |
| Node label and running build | Version/build exist in [program version output](../../docs/09-setup.md#build-information); the inspected status answer does not supply them or a node name | Useful narrow read addition for the node area. Do not label the web executable's version as the bus version or derive a node hostname from a principal realm |
| Process PID, runtime session ID, cwd, process start, OS uptime | Some session bookkeeping is local to [launchers](../../src/launchers/sessions.ts); the bus record does not expose this set | The requested familiar session view can use descriptions now. A real process inventory requires producer support and explicit scope; the web child must not scrape launcher homes or `/proc` |
| Health, execution results, latency distributions, audit history | Not supplied by the inspected MVP dashboard data | Keep [R1 extensions](../R1/discovery.md#dashboard-extensions) separate. Do not fill absent data with green badges, zeroes or fabricated history |

The built directory distinguishes current identity evidence and keeps cleanup candidates visible; startup collection and explicit removal follow [ownerless credentials](../../docs/02-access.md#ownerless-credentials), including the interim dependency on orphan-service handling. Support for the category is permanent, its occupancy is not. Presentation must remain correct when empty or populated; absent historical provenance cannot be reconstructed automatically. See the [completed directory slice](done/identity-cleanup.md#scope).

## Visual direction

A compact operations application: neutral surfaces, strong text hierarchy, a restrained accent for navigation/actions, and color reserved for meaning. Lists use available width; forms use a readable measure. The same header, navigation, spacing, table, form and feedback components appear on every page.

| Element | Direction |
|---|---|
| Hierarchy | Page title and purpose, task toolbar, primary data, then secondary detail. Put operational problems ahead of implementation digests |
| Typography | Readable system UI text; monospace only for addresses, fingerprints and commands. Numeric columns align consistently |
| Status | Text plus color, never color alone. Distinguish warning, refusal, disabled and unknown; no “healthy” state derived from reader presence |
| Layout | Comfortable default table density; wrap descriptions and long names. Narrow layouts retain essential identity and actions; only genuinely wide tables scroll horizontally |
| Forms | Labels above controls, grouped by purpose, with nearby help and one primary submission. Native controls and visible focus; no custom widget framework |
| Charts | Traffic comparison and exception metrics arranged compactly; labelled time/value axes, common time range, explicit independent scales where used, partial interval differentiated, accessible values beside them |
| Quiet state | A brief factual statement with a next action; do not give empty tables or flat charts most of the screen |
| Refresh | A visible observation time and Refresh action. Optional full-page refresh must be explicitly enabled and pausable; never reload a form or unexpectedly move focus |

Validate the proposed design with populated, empty, unavailable, denied and long-name examples before implementation. The current production data is too quiet to validate incident presentation. Produce reviewable page designs in F.13.0; this document does not pretend a new visual design has already been tested.

## Architecture recommendation

Keep `net/http` and `html/template`, split templates from handler source, embed local presentation assets, and build a small shared web presentation module. Route → visitor-authenticated API calls → presentation preparation → template is sufficient. The web child remains stateless between requests under the [process boundary](../../docs/11-processes.md#web-authority-boundary).

Prepare display data before rendering; templates perform no I/O or authorization. Keep shared navigation, list controls, status text, forms and error presentation in one place. Fetch only the data needed by the current page. Build avatar initials from the already-authorized directory response instead of fetching the entire directory once per image. Pass cancellation and bounded request timeouts through the web/API boundary.

Server-side search/sort/paging over the current authorized listing is the first implementation. It bounds HTML and browser work, not daemon response size. Measure both separately; add daemon-side querying only when evidence warrants it. Do not add a database, cross-user cache, general component framework or new API wire design to solve presentation problems.

### Modern Go options researched

Primary documentation checked 2026-09-15. The verdicts are recommendations for this repository, not benchmark claims.

| Option | What it buys | Cost or boundary | Recommendation |
|---|---|---|---|
| [Go HTTP routing](https://go.dev/blog/routing-enhancements), [html/template](https://pkg.go.dev/html/template), [embed](https://pkg.go.dev/embed) | Method/path routing, contextual escaping and packaged templates/assets | Template field errors remain runtime concerns; parse/render tests must exercise pages | Baseline: least migration work, existing stack and sufficient for these workflows |
| [templ](https://templ.guide/) and [presentation preparation](https://templ.guide/core-concepts/view-models/) | Composable templates compiled into Go, Go tooling, no required browser JavaScript | Adds a generator, dependency and template migration | Best alternative if component authoring becomes the bottleneck; trial one list and form before choosing. Not required for a good design |
| [htmx](https://htmx.org/docs/) | HTML fragment updates, progressive enhancement of ordinary links/forms | It is browser JavaScript; local hosting does not satisfy the current no-script rule. History snapshots and fragment responses require particular care on private pages | Smallest candidate if the owner later permits partial updates. Keep real URLs/forms; avoid history caching of private pages, script evaluation and third-party assets |
| [Datastar](https://data-star.dev/guide/getting_started), [Go SDK](https://data-star.dev/reference/sdks) | Reactive HTML and server-sent updates from Go | Browser runtime plus stream/reconnect lifecycle; changes the no-script boundary | Not justified by a bounded registry/administration UI; reconsider for an approved live-monitoring requirement |
| [chi](https://github.com/go-chi/chi) | Composable standard-HTTP routing and middleware | Another dependency; present route complexity is modest | No router migration for this redesign |
| [Tailwind CLI](https://tailwindcss.com/docs/installation/tailwind-cli) | Generates static CSS, including through a standalone CLI; no browser runtime required | Build tooling and utility-heavy templates; not a design system by itself | Valid with current browser restrictions, but start with a small shared stylesheet |
| [Pico CSS](https://picocss.com/docs) | Semantic HTML styling, themes and CSS variables | Primarily a styling baseline; task hierarchy and dense administration patterns still need design | Useful prototype baseline, not a complete answer to the audited problems |

No-script and no-CDN are different decisions. Self-hosting a library removes external fetching, not the script's access to page content. The current contract therefore remains the default; adopting an enhancement would first require an explicit change to its owning section and decision index. Do not write our own htmx substitute.

### Research applied

| Primary source | Principle used here |
|---|---|
| [Carbon data tables](https://carbondesignsystem.com/components/data-table/usage/) | Give dense data space, a task toolbar, search, sorting and paging; move extensive detail off the list |
| [GOV.UK error summaries](https://design-system.service.gov.uk/components/error-summary/) | Keep validation in the task with matching summary and field errors; adopt the pattern without importing its JavaScript-dependent behavior |
| [GOV.UK check answers](https://design-system.service.gov.uk/patterns/check-answers/) | Let a person review consequential changes before submission and return to the relevant edit |
| [WAI page structure](https://www.w3.org/WAI/tutorials/page-structure/) and [tables](https://www.w3.org/WAI/tutorials/tables/) | Page landmarks, meaningful headings and programmatically associated table headings |
| [WCAG reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html), [contrast](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html), [updating content](https://www.w3.org/WAI/WCAG22/Understanding/pause-stop-hide.html) | Test narrow/zoomed layouts and contrast; let the reader control automatic updates |

These references supply interaction and accessibility patterns, not a request to clone their branding or import their frontend packages.

## Boundaries and handoff

Missing-certificate startup policy is settled in [where it listens](../../docs/05-discovery.md#where-it-listens); this redesign follows that contract.

The [execution plan](TODO.md#web-redesign) handles visual design, data semantics, shared presentation, feature migration and installed acceptance separately. Existing isolation and crash-recovery gates remain release blockers; a redesigned page does not close them.

Keep managed runtime controls, process health, advanced exchanges, long histories and central settings in their [assigned release](../R1/discovery.md#dashboard-extensions). Later declared record states remain [R1.1](../R1.1/records.md#down-and-retired). A browser message composer is not added: it needs a separately accepted body-handling workflow, whereas the present dashboard is for discovery, administration and envelope diagnostics.

The main missing MVP work is trustworthy presentation and complete journeys. Narrow read additions may be needed for identity origin, node/build identity and effective inherited values. Those additions must be justified by a specific screen; they do not authorize schemas, new storage or broader telemetry collection.
