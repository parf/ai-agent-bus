# Codex web redesign review

## Position

Design review, 2026-09-16, against `471f550`. No application code changed.
The [source inventory](current-state.md#evidence-boundary) and
[current-run browser captures](current-state.md#browser-evidence) are the
evidence. Recommendations here need reconciliation with the owner's decisions;
they do not replace current contracts.

Keep server-rendered Go. Keep the current no-script baseline for this redesign:
none of the reproduced usability failures requires relaxing it. Decide the Go
template authoring tool separately. Consider `templ` seriously now that the
whole presentation is being replaced; do not preserve embedded string templates
merely to avoid a dependency. A typed renderer still cannot prove that an
operational claim is true.

The design should make routine discovery fast and exceptional conditions
findable. An exception-only homepage is insufficient if finding a service or
recognising a session becomes a secondary task. Hiding repetitive scaffolding
is useful; hiding unresolved operational debris or authorized facts is not.

## Junk and misleading content

Source links and capture names refer to the [inventory](current-state.md#visible-fields-by-page).
“Hide” below means remove from the default presentation or put behind deliberate
disclosure, not silently remove records, authority checks or required functions.

| ID | Evidence and consequence | Proposed treatment |
|---|---|---|
| C01 | [Service list, admin:329](../../../../src/cmd/agent-bus-web/admin.go#L329), `services.jpg`: address-only identification; description/kind absent, while timestamp and inert Manage/View text consume columns | Show recognisable description plus full routing name; remove the pseudo-control column. Put update time in detail unless the task needs it. Keep owner visible |
| C02 | [Detail, admin:336](../../../../src/cmd/agent-bus-web/admin.go#L336), `service.jpg`: operational summary followed immediately by digest and an always-open wall of editors | Read overview first. Focused edits for metadata, queue policy, access, config, transfer and removal; digest inside config evidence, not the lead |
| C03 | [Channel detail, admin:337](../../../../src/cmd/agent-bus-web/admin.go#L337), `channel.jpg`: pub/sub says Offline, oldest is blank, config digest is blank; delivery mode itself is not stated | Mode-first channel presentation. Suppress irrelevant direct-reader state and empty digest. Queue and subscriber information must explain where work actually waits |
| C04 | [Editable/read-only branches, admin:343](../../../../src/cmd/agent-bus-web/admin.go#L343), `readonly-service.jpg`: external service shows “external” but no address or protocol value | Authorized read-only summaries independent of edit controls. Hidden editing authority must not hide already-returned metadata |
| C05 | [Groups, admin:360](../../../../src/cmd/agent-bus-web/admin.go#L360), `readonly-groups.jpg`: headings with no members or explanation. Backend deliberately hides membership from ordinary callers | Say membership is not visible to this caller. Never turn an empty returned array into a zero-member count without authority to know it |
| C06 | [List shell, admin:324](../../../../src/cmd/agent-bus-web/admin.go#L324) and [redirect, admin:303](../../../../src/cmd/agent-bus-web/admin.go#L303), `channels.jpg`: Channels highlights Services; every channel mutation leaves its context | Correct page identity and return to affected resource with retained list filters and a specific result |
| C07 | [Activity, activity:75](../../../../src/cmd/agent-bus-web/activity.go#L75), `activity.jpg`: five tall, independently scaled graphs with unlabelled axes; zero series use as much space as traffic | Compact shared time range, explicit units/scales, actual interval and partial bucket; summarize zero series without losing the value table. Absence is not a measured zero |
| C08 | [Diagnostics registry, main:450](../../../../src/cmd/agent-bus-web/main.go#L450), `diagnostics.jpg`: digest competes with identity/description, full registry repeats discovery, names are not links | Move unique useful registry data into discovery/detail before removing the duplicate. Link queue/loss rows to their names. Do not assume nobody uses it: no usage research established that |
| C09 | [Exchange rows, views:252](../../../../src/cmd/agent-bus-web/views.go#L252), `diagnostics.jpg`: repeated “No completion receipt observed in retained history” and a mostly-one Envelopes column occupy substantial space | Preserve receipt evidence and its limits, but demote repetitive absence/count scaffolding. Compact exceptional evidence and disclose IDs/routes when useful; no change to correlation semantics |
| C10 | [Directory, users:192](../../../../src/cmd/agent-bus-web/users.go#L192), `users.jpg`: an empty cleanup table and explanation occupy the quiet view | Collapse the empty section into a compact count/link. When candidates exist, keep a visible attention count, reason and route to review/removal. Do not hide by runtime prefix, age or spelling |
| C11 | [Detail labels, admin:336](../../../../src/cmd/agent-bus-web/admin.go#L336), `channel.jpg`: empty Maintainers, oldest and digest labels are unfinished sentences | Omit nonessential label with absent value. Where absence changes a decision, state it precisely: inherited policy, no queued messages, not observed, membership hidden |
| C12 | [User access form, users:233](../../../../src/cmd/agent-bus-web/users.go#L233), `person.jpg`: already-active user offers Activate beside Pause and Ban; all look alike | Separate state summary from consequential actions; hide redundant transitions, identify target/consequence before submit. No delete-user feature |
| C13 | [Request errors, main:302](../../../../src/cmd/agent-bus-web/main.go#L302), [fail, admin:87](../../../../src/cmd/agent-bus-web/admin.go#L87), `form-error.jpg`: submitted invalid name produces raw JSON; descriptive input disappears | Human error summary, field association, nonsensitive value preservation, specific next step. Never echo token/private config. Transport failure must not promise “Nothing was changed” |
| C14 | [Sign-in, main:419](../../../../src/cmd/agent-bus-web/main.go#L419): token acquisition help is developer-shaped; SSH onboarding also has a documented pending gap | Keep supported acquisition instructions discoverable and concise. Do not offer an onboarding journey that backend provisioning cannot complete; link the [setup limitation](../../../../docs/09-setup.md#ssh-admin) in design dependencies |
| C15 | [CSS, main:327](../../../../src/cmd/agent-bus-web/main.go#L327), `mobile-services.jpg`: desktop list becomes a scrolling table; narrow viewport retains name/owner but hides queue observation to the side | Choose essential narrow-screen columns or record summaries deliberately. Current capture had no document-wide horizontal overflow; do not report a whole-page overflow defect |
| C16 | [Node summary, main:435](../../../../src/cmd/agent-bus-web/main.go#L435), `diagnostics.jpg`: node-wide totals sit above caller-visible queue/registry rows without an explicit scope contrast | Label node totals and visible-resource counts separately, including retained-history scope. Never imply their counts must agree; follow [dashboard audience](../../../../docs/05-discovery.md#dashboard) |

These are content/interaction findings, not a vote for hiding every zero or
identifier. A measured zero can answer a question. A full routing name
disambiguates sessions. A message ID connects evidence. A digest can prove a
configuration write landed. Put each where that question is being asked.

## What the earlier plan gets wrong or leaves weak

| Earlier position | Correction for the new design |
|---|---|
| [State language](../../web-interfaces.md#state-and-language): replace blank labels with explicit absence text | Too mechanical. C11: do that only where absence changes the next decision; otherwise remove the label. A page of “Not supplied” is still junk |
| [Architecture](../../web-interfaces.md#architecture-recommendation): keep `html/template` as baseline | Sound minimum, not a final tool choice for a complete rewrite. Existing templates are dense concatenated strings ([admin:324](../../../../src/cmd/agent-bus-web/admin.go#L324)). Compare component authoring and build cost, not library novelty. `templ` is a credible alternative without browser JS |
| [Navigation](../../web-interfaces.md#navigation-and-pages): Overview, Diagnostics and Activity | Risks splitting one investigation into three places. Specify links/filter retention and unique purpose before adding destinations. Overview must retain a direct service-finding path |
| [Data coverage](../../web-interfaces.md#data-coverage): provenance read addition | Classification/cleanup are now built. Historical origin still cannot be reconstructed. Design against returned Kind/capabilities; do not reopen solved classification as a new schema project |
| [Visual direction](../../web-interfaces.md#visual-direction): generic compact/neutral/native prescriptions | Insufficient for the owner's design request. Require a concrete chosen visual reference and reviewed populated/quiet/error/narrow examples before coding; prose adjectives do not establish design quality |
| [Retention language](../../web-interfaces.md#state-and-language): absent completion gets a per-row sentence | Truthful semantics, excessive repetition in the implemented page (C09). Keep the guarantee while changing its presentation |
| Historical W01/W10/W11/W12 used as if still wholly open | Current shell already has sign-out, viewport, focus treatment and distinct recovery branches. Re-audit remaining paths; root `/ls` failures and local form errors still bypass that recovery ([inventory](current-state.md#errors-and-recovery)) |

The required-tabs paragraph still names group deletion, while the owning
[group decision](../../../../docs/01-identity.md#groups-and-maintainers) forbids
it. This is a current-doc reconciliation item, not a reason to restore a button.
The inventory marks the residual handler separately from visible controls.

## Go and browser recommendation

Primary documentation rechecked 2026-09-16. These are capability assessments,
not benchmarks or claims that a new library improves the current layout.

| Layer | Recommendation and reason | Primary evidence |
|---|---|---|
| HTTP/API boundary | Keep standard Go routing and visitor-authenticated daemon calls; no redesign-driven router/framework replacement | [Go routing](https://go.dev/blog/routing-enhancements); current [web authority boundary](../../../../docs/11-processes.md#web-authority-boundary) |
| HTML authoring | Shortlist `templ` for typed, composable page components. Accept it only with reproducible generation and reviewed migration/build cost. `html/template` with real partials remains a valid fallback | [templ introduction](https://templ.guide/), [Go contextual escaping](https://pkg.go.dev/html/template#hdr-Security_Model) |
| Browser behavior | Retain ordinary URLs, forms and native disclosures as the complete MVP workflow. No relaxation of the no-JS rule has been proven necessary by this audit | [Forms inventory](current-state.md#forms-and-actual-effects), [current rule](../../../../docs/05-discovery.md#rules-it-is-built-to) |
| Optional later enhancement | If the owner identifies a concrete need for partial refresh/search, htmx can enhance HTML responses. Local hosting does not remove its script privilege. Disallow eval/script processing and private history snapshots; preserve non-script operation and deliberate focus/error behavior | [htmx security](https://htmx.org/docs/#security), [progressive enhancement](https://htmx.org/docs/#progressive_enhancement) |
| Form quality | Error summary plus field-level guidance, preserved nonsensitive input and explicit success state are design requirements regardless of renderer | [WAI form notifications](https://www.w3.org/WAI/tutorials/forms/notifications/) |
| Admin design choice | Compare a selected static admin-template subset, a CSS framework and a small custom system on the same workflows. Do not declare “no template survives” from dependency labels | [Tabler separates CSS from interaction scripts and permits self-hosting](https://docs.tabler.io/ui/getting-started/installation) |

`templ` provides compile-time Go structure; it cannot reject a well-typed but
false claim such as calling a credential-only identity a user. Behavior and
permission checks remain necessary. Nor is modern Go synonymous with htmx:
the renderer and browser enhancement are independent decisions.

The proposal to select a house system is not yet supported by a measured
comparison. Tabler's static layout/forms are not automatically disqualified
because its optional dropdowns require script. Conversely Bulma's lack of
bundled JavaScript does not make every documented interaction work without
it: its [modal documentation](https://bulma.io/documentation/components/modal/)
supplies a JavaScript implementation. Compare the actual selected components,
keyboard behavior, narrow layouts, asset/license obligations and adaptation
effort before choosing. No new frontend prototype was built for this review.

## Review of the new direction

These points were sent to Claude before target specs were expanded. They are
review constraints, not a second information architecture.

| Choice under discussion | Position |
|---|---|
| Separate Channels destination | Keep it: delivery mode and subscriber journeys differ. Share components, not misleading labels. Separate destinations do not require duplicated implementations |
| Change names into URL path segments | Keep current query URLs for MVP unless a concrete benefit justifies migration. They already bookmark. The actual escaping issue is slash-bearing agent names, not `@`; existing [session naming](../../../../docs/08-runner-role.md#session-names) makes that common |
| Remove homepage registry | Yes after migrating unique useful fields into Services/Channels/detail. No claim about present user habits: captures are synthetic. Keep direct navigation/search for routine discovery |
| Attention-only overview | State observation time and visibility; use “no observed issues in this view,” not a health verdict. Lifetime refusals and all queued work must not become perpetual high-severity alerts |
| Read versus edit permissions | The daemon constrains everything read. Separately, lack of edit capability must not suppress public metadata already authorized by the daemon |
| Unused avatar endpoint | No current HTML references it. Direct authenticated access still exists; “unused by UI” is established, “unreachable” or “vulnerability” is not |
| Glyph vocabulary | Use the owner's [meaning and restraint](https://parf.dev/ai-skills/Glyphs.md), with visible text. CSS equivalents versus actual glyphs remain a design choice, not a settled replacement |
| Severity from observed facts | Do not invent acknowledgment of an unclean stop, rate trends from lifetime totals, or service health from protocol hints. Missing current oldest age is not evidence that an inbox never held work |
| Absence markers | Keep hidden-membership explanation in words. A nonzero value below numeric display precision differs from no observation; an integer series drawn below a pixel does not need a new state |

## Handoff and limits

Opencode independently checked the inventory against route declarations,
action dispatch and template branches, and accepted this review after source
spot-checks. Its ordering/template-name notes and explicit scope-label finding
are incorporated. That review does not expand the stated browser coverage.

Claude owns target specs and owner questions. Opencode reviews completeness
and researches design candidates. This inventory is the comparison baseline:
each required field/action must receive a target home, an explicit justified
demotion, or a recorded removal decision. Presenting an action is not proof
that its backend effect is built or safe.

No app code, version bump or deployment is part of this slice. Separate access
review remains relevant: the [operation-time changes](../../done/gate-window.md#scope)
do not themselves certify every session-issuance/read-side interleaving or the
web child's mapped-socket configuration. These were handed back to Claude;
the design must not label the release security-complete.
