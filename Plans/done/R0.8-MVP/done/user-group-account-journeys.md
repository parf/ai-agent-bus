# User, Group and Account journeys

📌 **TL;DR:** 0.5.79 completes caller-scoped identity, lifecycle, group-impact
and credential journeys and makes GitHub-populated User details editable.

## Result

The signed-in identity now opens Account. It distinguishes an optional person
profile, the caller's own record, records owned by the caller and credentials
the caller holds. Credentials moved off Diagnostics; only fingerprints and
usage dates render, never credential values. A service principal gets the same
journey without a fabricated user profile.

User detail links memberships and caller-visible owned records to their correct
Group, Service or Channel destination. Administrative lifecycle controls appear
only from daemon-returned authority and only for transitions that apply. The
current state remains visible by word and colour; actions sit behind the red
**Change** disclosure. Ban and unused-credential removal stop on server-rendered
consequence confirmations.

Company, Location and Twitter/X are ordinary editable AgentBus User fields.
GitHub may populate them, and explicit refresh replaces them, but the page no
longer presents a second GitHub profile or provider source/fetch bookkeeping.
Older API writers preserve the fields; the web's presence marker makes a
deliberate complete replacement, including clearing, unambiguous.

Group detail shows caller-visible records whose ACL or Maintainers terms refer
to that group. Cycle-safe traversal also labels nested paths such as
`ACL via @outer`; hidden records never enter the face-side list. The Groups
table keeps members inline beside linked names, and the protected
`@administrators` detail explains its purpose and resource-maintenance limit.

## Checks

Focused integration checks cover profile-backed and service-principal Account,
credential ownership and rotation help, applicable lifecycle transitions,
ordinary-user denial, Ban and credential-removal confirmation, linked groups,
Service/Channel routing, hidden-record intersection and direct/nested Group
impact. Existing directory, provider-optional refresh, local-photo, protected
group and filter-return checks remain green. Core/API checks separately pin
editable-field normalization, trusted image/provenance rejection, old-writer
preservation and explicit clearing.

A skeptical pass over the side-review report reproduced one narrower issue:
snapshot restore retained caller-specific and computed User fields even though
profile writes already stripped them and outward views recomputed them. Restore
now uses the same derived-field scrubber. Three focused tests and **3/3**
mutations separately pin restore stripping and the next snapshot, preservation
of durable profile/lifecycle/provider fields, and the existing write-path
defence. The report's generic-queue premise does not match the built model —
generic services have inboxes — and no channel address/protocol restriction was
inferred from the remaining open question.

Real Chromium inspected the previous production pages and the current-source
Users, User, Group, Account and confirmation pages at desktop and 375 px. Every
page had zero horizontal overflow. The first Account layout used a tall stack of
credential cards; browser review rejected it and the final layout uses one
compact table below the two summary cards. A final authenticated pass covered
the live-shaped `parf@parf` and `chief@srv1` details: linked membership,
editable profile fields and access disclosure all rendered without overflow.

The final mutation set catches **21/21** named breaks across Account identity
and credentials, caller-visible owned records, Service/Channel routing,
lifecycle authority and confirmation, current-state rendering, the red Change
disclosure, hidden-record intersection, direct/nested group impact, inline
members, Administrator help, credential confirmation/ownership and editable
profile-field presence semantics. The first current-state mutant survived
because its assertion saw the class name in shared CSS; that run is retained
without credit. Scoping the assertion to rendered `<main>` catches the same
mutant.

Fast smoke passes **489/0**. Documentation validation checks **177 Markdown
files** and **2,906 local links** with zero errors. The first **610/0** frozen
slow run was superseded when the side-review invariant and two documentation
corrections joined the candidate; only the corrected frozen run below receives
final credit. The corrected frozen slow smoke passes **610/0**, including vet
and race, and all **406** frozen files match after the run;
the foreign `.gitignore` edit and this measured evidence file were deliberately
outside the manifest. The tracked smoke script retains SHA-256
`b9bbb438002621453f0b8568401b757fdf20d55e0f399018527e13aa7f502e5f`.

## Scope

This completes F.13.4. It adds one backward-compatible API request-presence
flag for replacing the three editable profile details; response fields and
provider/image trust boundaries stay unchanged. Installed multi-role browser
acceptance remains under F.12; whole-redesign
accessibility/performance acceptance remains F.13.6.
