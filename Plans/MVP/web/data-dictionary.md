# Data dictionary

Draft for peer review. What each value **means**, what we **call** it, and when
it is **not shown**.

This is the layer where the audited defects actually live: almost every finding
in [W01–W17](../done/web-review.md#findings) and
[C01–C16](review/codex.md#junk-and-misleading-content) is a value that was true
being presented as something it is not. The complete list of what is rendered
today is codex's [inventory](review/current-state.md#visible-fields-by-page);
this file is the vocabulary we replace it with.

## The rule

> Declared state and observed state are never merged, and neither is health.

A record *declares* it is enabled. The daemon *observes* whether a read is
outstanding. Neither says a process is alive, and nothing on this dashboard is
allowed to imply it does.

Two corollaries the first draft left implicit, both from home-parf:

**Provenance is the same class of distinction.** A figure the daemon reported
and a figure the web child computed are not interchangeable, and a table that
shows them side by side without saying which is which has merged them exactly
the way declared and observed must not be merged.

**An observation is worth what its read path actually did.** The daemon reports
queue state without pruning expiry first, so *held* is not *waiting* and *at
capacity* is not *refusing* ([glyphs](glyphs.md#what-an-observation-is-worth)).
Where a value's meaning depends on when it was computed, this file says so.

## Fields

### Identity

| Field | Means | We call it | Not shown when |
|---|---|---|---|
| `Name` | full routing address, `user@realm` | the name, in monospace | never hidden — it disambiguates sessions |
| `Descr` | caller-supplied display text | the description, shown first | absent: the name stands alone, no empty label |
| `Kind` | generic, agent or topic | Service, Agent, Channel | — |
| `Mode` | queue or pub/sub, topics only | delivery: one at a time, or a copy each | not a topic |
| `Owner` | the principal who owns the record | owner | — |
| `Maintainers` | list of named users, groups, agents and services with management authority | maintainers | **empty: the label goes too** |

### State

| Field | Means | We call it | Never |
|---|---|---|---|
| `Disabled` | **delivery is off**. Not a declaration, and not availability either: `visible` answers the stored bit OR the name having stopped being active, and a suspended *owner* is checked separately at `Send` — so a record reads Enabled while every send to it is refused. The pages give it its own heading, call it the delivery *setting*, and say it does not establish that a send would be accepted. The bit does not say why: `visible` (manage.go:305) returns `r.Disabled` OR `!b.active(r.Name)`, merging the owner's decision with the name having stopped being active | **Disabled** / **Enabled** | "Inactive", which reads as broken — and equally *"the owner turned delivery off"*, which this answer cannot establish ([S04](review/codex.md#specification-review-round-one)) |
| `Readers` | the live count of all outstanding filtered and unfiltered reads; omitted means unavailable, while explicit `0` is measured | the numeric **Readers** value; registry filters say **Reading now**, **No reader now** and **Unavailable** | "Serving"/"Offline". A busy process between pulls is not offline, and no count is health |
| `Reading` | compatibility-only indication that an unfiltered read is outstanding | do not render; new consumers use [`Readers`](../../../docs/05-discovery.md#readers) | presenting an incomplete compatibility bit beside the complete count |
| `Proto` | a caller-supplied hint that the thing is reached another way | **external** | treating it as proof of anything, or as a healthy state |
| user `State` | active, paused or banned | active / paused / banned | conflating a user's state with a record's |

**Queue counters persist; activity history does not.** The two are easy to
conflate and we did: accepted, dequeued, dropped and expired are restored from
the snapshot, while sampled traffic history starts empty after a restart. So a
counter spanning a restart is a real cumulative total, and a graph before the
restart boundary is `¿`.

### Queue

| Field | Means | We call it | Not shown when |
|---|---|---|---|
| `Queued` | messages **held** now — the observation path does not prune, so some may already have outlived their TTL | queued | — |
| `Oldest` | age of the head of the queue, which may be a message the daemon already considers expired but has not swept | oldest held | empty: `—`. The queue is empty **now**; the daemon does not say whether it ever held anything |
| `In` | messages accepted, **cumulative across restarts** | **accepted** | — not "since start": [`Restore`](../../../src/internal/core/snapshot.go) puts these back from the snapshot. codex's correction |
| `Out` | messages handed to a reader, cumulative across restarts | **dequeued** | — never "completed". Handing a message over is not doing the work |
| `Dropped` | discarded by the overflow policy, cumulative across restarts | dropped | zero: `0`, which is a measurement |
| `Expired` | passed their TTL undelivered, cumulative across restarts | expired | zero: `0` |
| `AtBound` | at capacity **at the moment of the read** | **at capacity when observed**, red | predicting the next send at all. Not "dropped or refused now", and not "refused or drops oldest" either: the send path prunes before deciding, may hand a waiting reader the message directly, and races a concurrent consume ([glyphs](glyphs.md#what-an-observation-is-worth)) |
| `Bound` | queue capacity | unset: *uses the daemon default* — `boundOf` really does resolve one | — never copy the resolved value into a template as though the record carried it |
| `TTL` | this record's retention | unset: **no queue-imposed expiry — a message may still specify its own** | — **never** *uses the daemon default*. There is no retention default to inherit. But the record's TTL is only half the answer: `life()` returns zero, and the envelope a zero `Expires` that `prune` skips, **only when the sender named no TTL either**; `life("", "1m")` returns the sender's minute (bus.go:738). Saying "no expiry" flat is a one-sided read of the same function — codex's correction of my correction |
| `Full` | overflow policy | **refuse** or **drop the oldest** — always the record's own | — never *unset*. `Register` normalises an empty one to `strict` (bus.go:168), so a face never meets an unset overflow on a registered record. It is a normalisation rather than a daemon resolution, and the difference from `Bound` is the point: one is written onto the record, the other is resolved on each use and is not readable here |

### Node and scope

| Field | Means | We call it |
|---|---|---|
| `Status.Up` | time since this daemon started | uptime |
| `Status.Services` | **node-wide** record count | records on this node |
| `Status.Queued`, `Waiting`, `Dropped`, `Expired` | node-wide totals | labelled node-wide, always |
| `Status.Unclean` | the last stop did not write memory down | a fact, with no acknowledgement state — rendered **only when true**. It is `omitempty`, so absent covers both a clean stop and a first start with no previous stop, and we may not claim either |
| `Status.Refused` | lifetime counts by reason, a `map[string]int` | refusals since start — **never** "climbing" *here*, which needs a window this value does not carry. A supported reason absent from a status that returned successfully is **`0`, a measured zero**: `refused` starts empty (bus.go:126), `Refuse` increments per reason, and `Status` copies the map whole. Sparse serialisation is not missing observation, and `¿` is for evidence that is unavailable or unsupported — codex's correction of my correction. The reason set is closed and [named](../../../docs/05-discovery.md#refusals), so the face knows which zeros to draw |

**Node totals and any list never have to agree**, and every page showing both
says so: one is the node, the other is what this caller may see
([C16](review/codex.md#junk-and-misleading-content)).

### Credentials

| Field | Means | We call it |
|---|---|---|
| `Fingerprint` | names a credential without being one | fingerprint |
| `Issued` | when it was minted | issued |
| `Used` | last seen this run | last used, or *not this run* — not an expiry signal |
| `Kind: unregistered` | a credential whose name the daemon holds nothing else for | a leftover, with what it means — and **legacy-only** since 0.5.29: such a name is refused `401` on every call and is not issued a credential at all, so the daemon no longer creates these. It describes the cohort that predates the change, and must not read as a live category |

A credential is never rendered. Age is not expiry; nothing here retires for
being old.

### Directory

| Field | Means | We call it | Never |
|---|---|---|---|
| identity kind | user, record, or credential-only | a word: registered user / registered name / credential with no registered name | a colour, or a guess from a slash or prefix in the name |
| `Services` | records this identity owns | owned records | — |
| `Groups` | memberships visible to the caller | memberships, or *no memberships* | — |
| group members, withheld | the caller may not see them | **not visible to you** | an empty array shown as zero members |
| `PeopleCount`, `OtherCount` | counts over the caller-visible directory before search | labelled with that scope, **and marked as the face's own** — they are fields of `peopleView` in the web child, not daemon answers, and sit beside `Services` and `Groups`, which are | presenting a face-computed figure and a daemon-reported one as the same kind of fact. Provenance is the same class of distinction as declared-versus-observed, and this file's rule covers it |

### GitHub profile

**Built in 0.5.71.** The GitHub directory adapter retains the public profile
response with the User and returns it through the existing caller-visible
directory answer. A template never fetches GitHub itself.

| GitHub field | Profile meaning | Display and absence |
|---|---|---|
| `login` | GitHub login; already stored as `github_user` | link label for the GitHub profile |
| `name` | trusted person name; already fills a blank `person_name` without overwriting an explicit value | ordinary Person name; omit when blank |
| `avatar_url` | provider photo location | primary source for a locally imported profile photo; never emitted as a browser-hotlinked image |
| `gravatar_id` | legacy provider avatar identifier | fallback source when the GitHub photo cannot be imported; never fetched by the browser |
| `company` | provider-published organization or employer | Company; omit when blank |
| `location` | provider-published location text | Location; omit when blank |
| `email` | provider-published public email | fills the existing AgentBus `email` only when it is empty; there is no separate GitHub-email field |
| `twitter_username` | provider-published Twitter/X handle | linked handle; omit when blank |

Provider metadata is read-only in AgentBus. A successful GitHub lookup replaces
the retained provider fields, including clearing values the provider no longer
publishes. `name` and `email` are imports into the existing Person name and Email
fields: each fills a blank and never overwrites a value already stored. Email
also uses the existing cross-profile uniqueness rule. If another profile already
owns the normalized public email, enrolment still succeeds and the blank stays
blank; optional provider metadata does not become an authentication gate. Once
imported, Email remains the ordinary AgentBus field under its existing edit
rules; GitHub later hiding or changing the public email does not clear it.
Visibility follows the existing user-directory answer; these fields do not
create a public profile endpoint.

The fetch is tied to the GitHub-login field: creating or changing
`github_user` attempts to fetch and validate the public GitHub profile. A
provider transport, status or decode failure does not block the login field;
provider facts remain unobserved until a later successful fetch. Saving an
unrelated profile field does not contact GitHub. **Refresh GitHub profile** is
the explicit way to fetch the current login again and reports failure without
mutation. Clearing `github_user` clears GitHub metadata and the imported photo,
but keeps Person name and Email: after fill-blank import they are ordinary
AgentBus fields, not mirrors.

### User photo

**Built in 0.5.71.**

The user photo source order is GitHub `avatar_url`, then `gravatar_id`, then
locally generated initials. The trusted provider adapter fetches remote image
bytes, bounds their size and dimensions, validates and decodes the image, and
re-encodes a small local thumbnail so metadata and provider-controlled formats
do not reach the browser. Redirects outside the approved GitHub-avatar or
Gravatar hosts are refused.

Photo import is optional metadata. A failed photo request never refuses key
enrolment or clears a previously imported thumbnail, and a partial or failed
decode commits no image bytes. The page falls back to the previous local photo
or initials. Concrete byte and dimension limits are adapter constants tested at
both boundaries. A complete provider response explicitly
removing both photo sources clears the imported thumbnail. The retained photo
records its fetch time; the page never claims it is current beyond that read.
The same local thumbnail may accompany a caller-visible User owner on a resource
detail; it reveals no provider URL and no hidden profile fact.

## Absence

Four facts, four markers, and not a licence to mark every blank
([glyphs](glyphs.md#absence-which-is-four-different-facts)):

| | |
|---|---|
| `0` | measured zero |
| `∅` | measured, non-zero, below display precision — **not** for integer counts |
| `—` | not applicable here |
| `¿` | not measured or not observable |

Where an absence does not change a decision, the label goes with the value
instead. Stating every absence is its own wall of noise.

## Time

One explicit display timezone, full timestamps available, and each of these
labelled as what it is rather than as "date":

registration update · observation time · credential issued · credential last
used · history interval · sample time.

Service lists compact a registration update as `now`, then whole minutes,
hours or days while it is under 30 days old. Older values use `Jan 1` in the
current display year or `Jan 12, 2025` across years. A future value caused by
clock skew reads `now`; detail retains the full timestamp.

## Not available, and not invented

Named so nobody fills them with a green badge, a zero, or a guess:

| | |
|---|---|
| process health, liveness, execution results | not supplied by MVP data |
| latency distributions, audit history | [R1](../../R1/discovery.md#dashboard-extensions) |
| PID, runtime session id, working directory, process start | launcher-local; the web child must not read launcher homes or `/proc` |
| node label and running build | plausible narrow read addition, justified by the Overview node section — and the web executable's version is **not** the bus version |
| effective resolved queue settings | would need a narrow daemon read; until then inheritance is stated honestly |
| historical credential provenance | if no profile, record or independent evidence remains, it cannot be reconstructed. Keep it unclassified rather than guessing ([provenance](../done/web-review.md#credential-provenance)) |
