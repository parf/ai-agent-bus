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

## Specification review round one

Reviewed 2026-09-16: all nine draft specification files, against the inventory
and source at `471f550`. Draft copies used for line references are in ignored
`tmp/specs-round-1-codex/`; the specifications were uncommitted and may change
while findings are addressed. This is a source/document review, not a new
browser run or an implementation test.

Q58–Q61 are owner choices, not reopened questions. The specifications are **not
ready for implementation**: they assign false meanings to existing data, lose
actions, and leave the concrete visual design and editing journey unspecified.

| ID | Finding and evidence | Required correction |
|---|---|---|
| S01 | **Counter windows are wrong.** `data-dictionary.md:49–50` calls `In`/`Out` “since start”; `pages.md:48` calls drops/expiry “Losses since start”. [Restore](../../../../src/internal/core/snapshot.go) lines 61–65 restores all four counters; uptime is explicitly not restored (line 44). A restart therefore does not start their counting window | Label these as retained queue counters, without inventing a reset timestamp. Keep current-run refusals and uptime distinct. Add a restart-shaped example to the design fixtures so the labels cannot silently regress |
| S02 | **Attention invents present losses and a TTL deadline.** `pages.md:46–47` and `glyphs.md:48–49` say `AtBound` means “messages are being refused or dropped now”, and compare `Oldest` against the current record TTL. [withLiveness](../../../../src/internal/core/bus.go) lines 418–423 reports occupancy and rounded age only. `Send` fixes each envelope's expiry at admission (lines 556–562), using both message and queue TTL (`life`, lines 724–744). Changing the queue TTL does not change those stored expiries | At-bound means capacity reached, not observed loss. Do not infer the head message's deadline from today's queue policy. Any urgency threshold needs an explicitly chosen meaning and adequate data; otherwise show occupancy/age as observations |
| S03 | **The backlog journey disappears, and absence becomes assurance.** `information-architecture.md:28` routes “Why is work not arriving?” through Overview backlog; `pages.md:51–54,64` excludes ordinary backlog and replaces its table with exceptional queues, then says “Nothing needs attention”. A non-full queue below the proposed age threshold has no attention item even when its reader has actually stopped | Keep a direct route to all caller-visible queued work, with age/reader facts and no invented severity. “No observed attention conditions in this view” is bounded; “nothing needs attention” is not. A between-pulls example disproves certainty of failure, not the possibility of failure |
| S04 | **Inheritance and declared-state claims do not match the daemon.** `data-dictionary.md:54` groups `TTL`, `Bound`, `Full` as unset daemon defaults; [registration](../../../../src/internal/core/bus.go) lines 162–163 writes strict overflow into `Full`, `boundOf` resolves unset capacity (434–440), and `life` leaves retention unbounded when neither side supplies TTL (724–744). `data-dictionary.md:38` says `Disabled` means “the owner turned delivery off”; [visible](../../../../src/internal/core/manage.go) line 305 merges stored disable with the name's inactive state | Define each setting separately. Do not label the returned `Disabled` bit as proof of an owner's action. The current response does not separate those causes; state the effective observation or explicitly plan the missing distinction |
| S05 | **The confirmation rationale is false.** `forms.md:25–30` says client-side confirmation “would quietly remove the recheck”; `README.md:61` repeats it. Both an HTML confirmation page and a dialog can become stale before final submission. The [operation-time fix](../../done/gate-window.md#scope) lives in the daemon, not the confirmation presentation | Keep server-rendered confirmation as the chosen interaction. Require operation-time authorization for its final POST. Remove the claim that choosing a dialog necessarily removes that check, or that a second page supplies atomicity |
| S06 | **An intentional authority limit is listed as a defect.** `forms.md:90`: “Maintainers assignment and transfer share a permission guard, so a maintainer who may manage cannot assign”. [Manage](../../../../src/internal/core/manage.go) lines 179–180 deliberately reserves both changes to the owner. `pages.md:136` specifies only `CanManage` for the whole Manage section | Preserve owner-only assignment/transfer controls, and name their capability separately from ordinary edits. Do not turn this redesign into an authority expansion. Self-subscription also has its own applicability: today's [template](../../../../src/cmd/agent-bus-web/admin.go) lines 340–343 places it outside `CanManage` |
| S07 | **The field/action migration is not complete.** `forms.md:58` gives Replace configuration to Service only, but the existing shared [detail template](../../../../src/cmd/agent-bus-web/admin.go) line 354 offers it on managed channels too. `pages.md:96` demotes update time to detail without placing it in the detail section table (129–136). That table omits overflow from the read-only Queue summary. `forms.md:67` preserves Create user, but Users' page specification (218–246) provides no entry or creation-form journey | Keep channel configuration unless its removal is separately decided. Assign update time and overflow explicit read-only homes. Specify where a permitted administrator starts Create user and where invalid input returns. Address/protocol already have a read-only requirement in prose; also put them in the section map so implementation cannot miss them |
| S08 | **Account's proposed sources are incomplete.** `pages.md:274–275` uses “Users detail, for oneself” and “my names” for identity and owned records. [Users](../../../../src/internal/core/users.go) lines 442–454 excludes a non-self-owned service record without a person profile; such a signed-in service can have no directory row. [`/names`](../../../../src/internal/api/server.go) line 486 uses `Holds(Owned(...))`, and [Holds](../../../../src/internal/auth/tokens.go) lines 196–207 omits owned names with no credential | Design Account for both a person and a service principal. Source owned records from the authorized record listing; describe `/names` as credential holdings, not a complete ownership inventory. Do not fabricate a person profile or treat an empty `/users` answer as no identity. The rotation “button” at page line 278 is also unassigned: current scope is command help |
| S09 | **The message journey still claims individual evidence from totals.** `information-architecture.md:35`: “Did my message arrive, and was it consumed?” → “Queue counters”. Totals cannot identify which message was dequeued. The [exchange contract](../../../../docs/05-discovery.md#retained-exchanges) expressly distinguishes retained envelope evidence, receipts and possible responses | Route message investigation to scoped retained history, with its limits. Queue counters answer aggregate flow. If an individual dequeue is not recorded, say it cannot be established; do not turn accepted/dequeued totals into a per-message answer |
| S10 | **The error specification loses current distinctions.** `pages.md:305–315` offers four states and defines Refused as “lack permission”. Existing [fail](../../../../src/cmd/agent-bus-web/admin.go) lines 56–93 distinguishes 500, 503 and request/state refusals as well. Canonical [refusals](../../../../docs/05-discovery.md#refusals) distinguish suspended, busy, name-taken, full and malformed; giving a service permission cannot cure all of these | Map code/reason to recovery, including invalid input, current-state conflicts, suspension, daemon fault, temporary unavailability and uncertain transport outcomes. Preserve hidden/missing equivalence. A single reusable template is not itself the defect; incorrect reasons and recovery are |
| S11 | **Promised data has no implementation dependency or fallback.** `pages.md:35,132` requires node name and effective TTL/capacity. `data-dictionary.md:124–125` correctly says these need a daemon read addition; `technology.md:63–64` then says “no new wire design” and presentation solves presentation | Pick the MVP fallback explicitly (omit unavailable node label; display declared queue policy with its actual semantics), or name an approved data work item and dependency. Do not let template work silently invent daemon values or new response fields |
| S12 | **The eight-form proposal does not yet specify a better editing experience.** `pages.md:152–157` enumerates separate forms but does not say how one is opened or which are initially visible. `forms.md:10,82` identifies today's always-open editors as the defect. `visual-design.md:34,36` additionally promises user-selectable theme and per-table density, neither present in the form inventory | Keep independent submissions; that count is not a defect. Specify read-first sections with a visible Edit entry and one focused editor/disclosure per concern, initial and invalid states, Cancel, and return/filter preservation. Choose and describe the no-script theme/density controls and persistence scope, or remove the unapproved extra controls |
| S13 | **The visual-design deliverable is still principles, not a reviewable design.** `visual-design.md:31–37` says “neutral surfaces”, “one scale”, and “fluid scale”; it supplies no selected palettes, type/spacing values, layout examples or narrow-screen arrangements. Lines 84–89 require pre-implementation review of representative states, but none is supplied in these nine specifications | Before code, supply the actual token choices and representative page/component layouts, in both schemes and narrow/wide forms, including populated/error/empty/long-name cases. Measure contrast on the chosen combinations. These values should have one home, not be independently chosen by whoever implements each page |
| S14 | **Several claimed corrections remain stale.** `pages.md:81–82` says Channels has a Services heading, but [admin.go](../../../../src/cmd/agent-bus-web/admin.go) lines 324–325 has the wrong document title/nav and the correct channel `h1`. Page lines 106–107 claim unstable list loads although line 189 sorts the web list by name. Page line 258 still motivates group references as explaining why removal is refused, immediately before prohibiting group deletion altogether | Correct the current-state assertions; keep the real title/nav defect, and restrict the unsorted-map finding to the diagnostics registry. Explain group references by understanding affected records and membership changes, not an unavailable deletion operation |
| S15 | **Disputed research is still promoted into decision rationale.** `technology.md:88,94` repeats “Their identity is JavaScript, charts and CDN icon fonts” and “Every off-the-shelf table assumes client-side search”; the first also appears in the new Q59 row of [decisions](../../../../docs/decisions.md). The README dissent and technology's unverified-comparison section already acknowledge that a matched-component comparison was not run | Keep the owner's house-layer choice. Remove unsupported universal exclusions from its rationale rather than append a caveat that contradicts them. Likewise Q60's type-safety rationale must be conditional on component APIs: adopting templ alone does not make omitted captions, labels or error slots compile errors |

### Round-one handoff

While this review was being written, Claude committed `ae59cc3`. Reading that
diff verifies S06's owner-only controls and S12's initially collapsed,
in-section editors are addressed. The theme/density part of S12 remains.
Other closures are partial: S01's dictionary is fixed but the Overview still
says “Losses since start”; S02's at-bound wording is fixed on the page but not
in the glyph vocabulary, and its TTL inference remains; S05's form rationale
is fixed but the README still says a dialog removes the recheck. S10 gained a
changed-conditions state, not the remaining code/reason recoveries.

Two changes in that commit need correction before they become new premises:
Account now drops owner because “on your own credentials it is always you”.
`Owned` includes the caller itself, whose record may be owned by somebody
else; service principals are the counterexample. Keep the differing owner.
The Services kind column also becomes “filter only” because it is allegedly
redundant with the selected filter. The unfiltered mixed-kind list has no such
filter; preserve per-row kind there if the homepage registry is removed.

For S15, the primary references rechecked in this round are
[Tabler installation](https://docs.tabler.io/ui/getting-started/installation/)
(separate CSS/interactive script and local hosting) and
[templ composition](https://templ.guide/syntax-and-usage/template-composition/)
(components compile to Go functions whose arguments are chosen by their
author). The latter supports the narrower conclusion: typed mandatory
component arguments are a design choice, not automatic accessibility validation.

The decisions index now contains Q58–Q61; this review does **not** claim they
are missing. No source, partner-owned specification or decision row was edited.
No runtime suite was run for this documentation-only review. The requested
answers are S12 (form fragmentation), S02–S03 (Overview), and S04/S11
(inheritance and available values). The remaining findings are the coverage
and truthfulness checks against the baseline, not a replacement architecture.

## Review of the round-one response

Checked `7b29451` and `c613217`, including the new token and layout documents,
on 2026-09-16. **Not fully closed.** The channel configuration, read-only field
homes, Create-user entry, Account sources, individual-message journey and
explicit data fallbacks address their original findings. The palette is now
concrete: independent calculation reproduces every published contrast ratio,
including both outline minima. That verifies the arithmetic, not rendered
contrast, focus visibility, layout or zoom behaviour.

| ID | Remaining finding | Evidence and correction |
|---|---|---|
| R2-1 | **The confirmation invents destructive queue deletion.** `layouts.md:205` promises “48 held messages go with it” and offers Remove | [UnregisterAnd](../../../../src/internal/core/unregister.go) lines 76–79 prunes expired entries, then returns `ErrBusy` if any queued messages or waiters remain. Ordinary removal cannot discard those live messages. Draw a blocked-removal example with a safe recovery, and a separate eligible-removal confirmation; retain the final daemon recheck. Do not import orphan cleanup's purge semantics into this verb |
| R2-2 | **The disabled-queue correction is still too strong and inconsistent across files.** `pages.md:50` says “nothing expires out of it”; `layouts.md:50,162–163` again promises no drainage, although `pages.md:70–73` chooses the weaker effective-state claim. `data-dictionary.md:50` still says “the owner turned delivery off” | There are three prune callers, not just send and consume: [unregister.go:77](../../../../src/internal/core/unregister.go) prunes without a stored-disabled guard. A scratch reproduction with one expired and one live message increments `Expired` on attempted unregister, retains the live message and refuses removal. The ordinary send/consume paths do not drain a stored-disabled queue; that does not prove no operation can expire it. Separately, the returned effective bit still merges causes ([visible](../../../../src/internal/core/manage.go), line 305). Use the weaker observation consistently in the dictionary and actual mockups, not only in their caveat |
| R2-3 | **The new restart diagnosis contradicts both source and an existing test.** `pages.md:281,452` says restart resets counters and is clamped into a zero activity bucket | [Restore](../../../../src/internal/core/snapshot.go) restores queue counters but not samples. [TestActivityIsBoundedAndFiltered](../../../../src/internal/core/activity_test.go) lines 50–53 explicitly verifies the restored bus has no activity history. [Activity](../../../../src/internal/core/activity.go) lines 82–108 differences samples from the current process; it cannot subtract a previous process's sample that was never restored. `Status.Up` may help explain restart time, but it does not repair the alleged cross-restart delta. Describe missing pre-restart history and the available sampled interval, without inventing a zero bucket or a window spanning two runs |
| R2-4 | **The recovery table assumes a machine-readable reason it does not receive, and still supplies incorrect recoveries.** `pages.md:416–432` promises reason-specific 403/409 bodies, calls busy “transient”, and recommends retry | The daemon selects a counter reason in [server.go:634–639](../../../../src/internal/api/server.go), but its error response at 659–662 carries only error text. The web [request helper](../../../../src/cmd/agent-bus-web/main.go), line 303, stores status plus the raw payload; it has no stable reason discriminator. Name this dependency or give a safe fallback, rather than infer reasons from a closed *counter* vocabulary. Busy includes a credential now backed by a person or record, so waiting need not help. Enrolment means failed proof, not necessarily an unenrolled name. For an uncertain mutation outcome, inspect current state before offering a blind retry |
| R2-5 | **Claimed closures still have contradictory live copies.** `pages.md:135` retains “Kind: filter only, not a column”, while `layouts.md` restores kind on mixed lists. The dedup row at `pages.md:80` replaces one unsupported co-occurrence claim with “a disabled record holding work is usually also at its bound”. The Q60 row at `docs/decisions.md:142` retains the unconditional compile-error promise, and `components.md:13` still says nothing off the shelf supplies the table | Apply the corrected rules at their owning sections and link from other documents. Remove the co-occurrence premise entirely: one item per record does not need it. Remove the stale universal/component-guarantee assertions instead of relying on a later caveat. The mixed-list kind requirement must govern both the field map and mockup |
| R2-6 | **The density acceptance cannot yet reject the mutations it names.** `visual-design.md:130` requires “a minimum row count” without choosing one. Its introduction promises rejection of a wrong scale ratio or tight leading; the count caps in check 1 do not establish either | Choose the minimum and precise viewport/fixture/header conditions in the value home. Six arbitrarily spaced font sizes still satisfy the six-size cap; tightening every line height also leaves that cap unchanged. Name which separate bound or rendered check rejects each promised mutation. Checking contrast over the declared pairs likewise does not prove components actually use those pairs: retain rendered verification as a separate requirement |
| R2-7 | **The new mockups contradict the chosen presentation in smaller but implementation-relevant ways.** Overview's node strip in `layouts.md:57` includes Refused although `pages.md:81` excludes it there. The Services toolbar shows Kind Any while its active summary says kind agent, and its holding-work results include a zero-depth row. Its wide owner labels use ellipses with no stated way to recover the full value | Make each representative fixture internally consistent with its query and the field map. Specify how a truncated owner can be inspected without script and without assuming the caller may read that owner's user page. These are design decisions the mockup should resolve, not contradictions to leave to its implementer |

### Verification and scope

Ran the existing `TestActivityIsBoundedAndFiltered` and a scratch overlay test,
`TestReviewDisabledUnregisterPrunesButDoesNotPurge`; both pass. The latter
proves the disabled queue's expiry count can change on refused unregister and
that the live message and record survive. Scratch files are confined to ignored
`tmp/specs-round-2/`; no application or permanent test files were edited.
Recomputed palette ratios from the token hex values using sRGB relative
luminance; all match the table to its displayed precision. No browser rendering
or full runtime suite was claimed for this document review.

Q62 and the owner's specification review remain separate gates. These findings
do not reopen no-script, the house layer, templ or dark mode. They prevent the
new mockups and acceptance from undoing the source-grounded corrections.
