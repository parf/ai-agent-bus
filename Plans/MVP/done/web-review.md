# Web review evidence

## Scope

Historical review, 2026-09-15, source revision `f48f793`; installed programs reported `0.5.19`. This is evidence for the [proposed redesign](../web-interfaces.md#proposal), not acceptance of that design. Reviewed all dashboard page families, current requirements, relevant API/core data and the release boundary. The API root is a redirect, not another application; the repository search found no separate product website implementation.

Used the connected Chrome browser against the installed dashboard, with a fresh isolated owner session. Desktop captures used 1440 × 1000; the mobile service capture used 390 × 844. A first connection failed; retry succeeded. No local Playwright fallback was used. Inspected current screenshots, DOM, authenticated API answers and source. Created and ended audit-only sessions; did not submit administrative changes, send messages, rotate tokens or restart services.

The [local screenshot gallery](../../../tmp/web-audit/screenshots.md) contains the captures below. Raw captures, HTML and the API sample remain in ignored `tmp/web-audit/`, not repository history. They contain live identities; the tracked report retains findings and aggregate observations only.

## Captured journey

| Step | Capture | Observed health |
|---|---|---|
| 01 | Sign-in | Working token sign-in; weak task explanation and low-contrast help |
| 02 | Diagnostics after sign-in | Working, but long and repetitive; empty tables and credentials share the incident page |
| 03 | Services | Working list; descriptions absent, no search, register form competes with discovery |
| 04 | Service detail | Working owner view; all editing and consequential actions stacked together |
| 05 | Channels | Empty list and create form captured; no live channel available for a populated detail capture |
| 06 | Users | Populated but unusably long; credential identities presented as people |
| 07 | Owner user detail | Relationships and profile form present; empty fields and plain membership text |
| 08 | Groups | Membership form present; offers deletion of the protected group |
| 09 | Activity | Graphs render, but have no axis labels; quiet data consumes a long page |
| 10 | Mobile services | Desktop page shrunk to fit instead of a usable narrow layout |
| 11 | Unknown service | Bare backend JSON error; no application navigation or recovery |
| 12 | Anonymous services deep link | Bare “sign in required”; no link to sign in or return path |

Sign-out returned to the anonymous page. This was a read/navigation audit: no full owner/maintainer/ordinary-user action matrix, assistive-technology audit, restart acceptance or adverse-load exercise was performed. Empty channels/feed and a quiet activity window prevent claims about populated incident flows. Their missing cases are implementation acceptance work, not assumed passes.

## Live data

| Observation at collection | Implication |
|---|---|
| Four registered records, all agents; four waiting reads, no queued messages | This snapshot exercises session identification, not generic services, channels or backlog diagnosis; it does not establish the normal workload |
| 232 user-directory entries; none returned populated person name, email or GitHub profile | Directory size is not evidence of 232 people. Existing answers do not expose the origin of each entry |
| 206 directory names contained a slash; 120 contained “smoke” | Strong clues of historical service/test identities, not proof permitting automatic deletion or person classification |
| One group and five own credential entries | Group management and personal credentials are different inventories from the user directory |
| No retained envelopes; three activity points immediately after a recent start | Empty recent history cannot establish that the bus has never carried traffic; record counters were already nonzero |
| Browser Users view: 232 image elements and 11,073 CSS pixels in height | Unbounded rendering already hurts with today's data, without an artificial scale test |

Sampling and browser captures occurred at different instants. Counts and uptime are observations, not configuration defaults or a claim about the bus now. No stored credential values were read for the directory investigation; a token was obtained through the authorized local socket only for browser sign-in.

## Findings

| ID | Evidence | Finding and consequence |
|---|---|---|
| W01 | Captures 02–09; [main.go:304](../../../src/cmd/agent-bus-web/main.go#L304), [admin.go:233](../../../src/cmd/agent-bus-web/admin.go#L233) | Navigation is duplicated, has no current-page indication, and sign-out exists only on diagnostics. Users must navigate by memory; account actions disappear between pages |
| W02 | Capture 03; [admin.go:239](../../../src/cmd/agent-bus-web/admin.go#L239) | The list omits kind and description; “Controls” contains non-clickable “Manage”/“View”. The description exists in every sampled agent record, but people must decode addresses to recognise sessions |
| W03 | Captures 03–04; [admin.go:240](../../../src/cmd/agent-bus-web/admin.go#L240), [bus.go:372](../../../src/internal/core/bus.go#L372) | “Offline” is inferred from no unfiltered waiter. A busy process need not be blocked in a read, and external protocol display replaces the actual reader observation. This is not health telemetry |
| W04 | Capture 04; [admin.go:246](../../../src/cmd/agent-bus-web/admin.go#L246) | Queue, access, configuration, assignment, transfer and removal are one long edit page. Blank oldest/maintainer/digest labels look incomplete. Read-only visitors lose access/queue-policy summaries because those values appear primarily inside `.CanManage` forms |
| W05 | Capture 04; [admin.go:268](../../../src/cmd/agent-bus-web/admin.go#L268) | Removal says “Credentials remain valid.” The current [unregistering contract](../../../docs/01-identity-and-roles.md#unregistering) says the service credential is removed. Transfer's consequence has leaked into removal help; this is an incorrect promise |
| W06 | Capture 06 and API sample; [users.go:185](../../../src/internal/core/users.go#L185), [web users.go:114](../../../src/cmd/agent-bus-web/users.go#L114) | `Users` unions profiles, self-owned records and unknown credential holders; the page labels each ordinary entry “User”. The combined response lacks origin information. Some historical origin may be absent from storage entirely; neither that conclusion nor a cleanup-failure diagnosis applies automatically to all entries. See [credential provenance](#credential-provenance) |
| W07 | Capture 06, DOM count; [web users.go:20](../../../src/cmd/agent-bus-web/users.go#L20), [web users.go:59](../../../src/cmd/agent-bus-web/users.go#L59) | Each avatar invokes the loader, which calls status and the complete user directory, then scans it. Repeated directory work grows with every row, before adding any useful interactivity. The browser showed one image per entry; backend call counts were not instrumented |
| W08 | Capture 08; [admin.go:270](../../../src/cmd/agent-bus-web/admin.go#L270), [manage.go:88](../../../src/internal/core/manage.go#L88) | “Delete unused group” is offered for the protected maintainers group although core unconditionally refuses its removal. No group detail or resource-reference view explains why other groups cannot be deleted |
| W09 | Capture 09; [activity.go:38](../../../src/cmd/agent-bus-web/activity.go#L38), [activity.go:71](../../../src/cmd/agent-bus-web/activity.go#L71) | Graphs use evenly spaced sample indices, unlabeled axes and separate implicit vertical scales. “Maximum: 0” repeated down a long page adds little. Actual timestamps exist; irregular/partial intervals should not be rendered as equal durations |
| W10 | Capture 10 and DOM; [main.go:257](../../../src/cmd/agent-bus-web/main.go#L257) | No viewport metadata; the emulated 390-pixel device laid out at 980 CSS pixels. Screenshot shows tiny desktop text and controls. Shared fixed input measures and table layout have no narrow-screen treatment |
| W11 | Captures 01–02; [main.go:266](../../../src/cmd/agent-bus-web/main.go#L266), [main.go:300](../../../src/cmd/agent-bus-web/main.go#L300) | Muted text `#888` on white calculates to 3.54:1; ordinary small text misses the WCAG AA minimum. Whole-page refresh repeatedly replaces diagnostics with no pause control. DOM also lacked a document language and main landmark; this is not a complete WCAG audit |
| W12 | Captures 11–12; [admin.go:24](../../../src/cmd/agent-bus-web/admin.go#L24), [admin.go:44](../../../src/cmd/agent-bus-web/admin.go#L44), [main.go:91](../../../src/cmd/agent-bus-web/main.go#L91) | Error output discards the shell and task context. Anonymous deep links do not enter sign-in. On root, any failed status request becomes the anonymous page, so the source also conflates an unavailable bus with missing authentication |
| W13 | [admin.go:212](../../../src/cmd/agent-bus-web/admin.go#L212), [users.go:108](../../../src/cmd/agent-bus-web/users.go#L108) | Source: every service/channel action returns to the user's service list, and profile changes return to the user list, with no specific success result. A channel edit loses its context. Not submitted against live records in this audit |
| W14 | [views.go:157](../../../src/cmd/agent-bus-web/views.go#L157), [envelope references](../../../src/internal/protocol/envelope.go) | Source: exchange grouping keys only on topic/tag, except when both are empty, and drops receipt references from its output. Different participants reusing the pair can merge; reply inference assumes a reversed sender. Redirected replies and truncated history need explicit acceptance before being presented as completed exchanges |
| W15 | [server.go:317](../../../src/internal/api/server.go#L317), [bus.go:846](../../../src/internal/core/bus.go#L846), [activity.go:66](../../../src/internal/core/activity.go#L66) | Source: status returns node-global totals to authenticated callers, while activity/listing have caller-specific filtering. Unqualified overview totals would mix scopes. Q50 owns the unresolved visibility choice |
| W16 | [main.go:328](../../../src/cmd/agent-bus-web/main.go#L328), [core listing](../../../src/internal/core/bus.go) | Diagnostics renders registry iteration order directly. Successive captured snapshots reordered the same records. A refreshed table should not move unchanged rows arbitrarily |
| W17 | [main.go:53](../../../src/cmd/agent-bus-web/main.go#L53), [main.go:144](../../../src/cmd/agent-bus-web/main.go#L144), [listener contract](../../../docs/05-discovery.md#where-it-listens), [session contract](../../../docs/05-discovery.md#signing-in) | Source/docs ambiguity relevant to access: missing certificate files log and continue with HTTP. “It says so rather than quietly serving plain HTTP” does not establish an unambiguous refusal requirement; its author reports intending refusal, which the code never implemented. Q52 asks the owner to choose. Separately, the cookie is Secure only with TLS while session prose states it unconditionally. No transport settings changed |

The main UX/accessibility references are linked with their application in the [research section](../web-interfaces.md#research-applied). None of the screenshots establishes authorization correctness, application completion, or process health.

## Review corrections

Follow-up from `claude/ab-dvp@parf.us`, topic `web-redesign-review`, message `42f914d7b5280259`, reported deletion of 18 test-debris records shortly before sampling, including a previously present generic service. This is reviewer-supplied context, not an independently observed deletion. It explains why the snapshot must not be used to characterize the normal workload.

Question history confirms earlier R1.1 use of Q41/Q42 (`c8bfa09`, resolved in `cbb3081`) and Q49 (`f48f793`, resolved in `46b6a0a`). The redesign questions were corrected to Q50/Q51; historical R1.1 identities were not changed. The quoted migration-identity instruction is in R1.1 QUESTIONS, not its current DECISIONS file; the latter records the decisions without those question IDs.

## Credential provenance

The current [unregister handler](../../../src/internal/api/server.go#L342) forgets a successfully removed non-person credential. That does not prove an existing credential-only identity was never unregistered: in `d2f4c55^`, the same handler explicitly says “The credential deliberately outlives the address”, and the earlier core retained an owner reservation. A process exit alone also does not explain a missing record: something must have removed or never created the record.

The [current credential representation](../../../src/internal/auth/tokens.go#L27) retains credential material and issuance/use observations, not the former service owner or person/service origin. The [record lookup](../../../src/internal/core/unregister.go#L53) has no reservation fallback. If no profile, record or independent historical evidence remains for a name, adding a future field cannot recover that name's past automatically. Keep it unclassified unless evidence is supplied.

The aggregate directory sample does not establish how many entries have that condition. It includes the known daemon owner; empty profile fields alone do not make an identity an orphan. Do not label all 232 entries permanent leaks. Q51 owns manual retention review for the unresolved subset; [record expiry](../../R1.1/records.md#how-long-a-record-lives) does not collect credentials with no record and does not authorize automatic token expiry.
