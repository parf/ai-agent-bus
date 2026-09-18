# Information architecture

Draft for peer review. Nothing here is settled.

## What is wrong with the shape today

Fifteen routes render eleven distinct screens, and the homepage is not a
homepage: `GET /` is a seven-section diagnostics wall — node counts, stuck
inboxes, retained exchanges, the **entire registry**, loss by name, the
caller's credentials, and three paragraphs of explanation — served in one
response whether or not anything is wrong. Everything else is a form.

Three structural faults follow from that, and no amount of styling fixes them:

| Fault | Where it shows |
|---|---|
| No page answers "is anything wrong?" | The operator reads seven sections and decides for themselves |
| Detail and administration are the same page | `/service` is a read view followed by six stacked forms ([W04](../done/web-review.md#findings)) |
| Things that are not alike share a page | Services and channels share one list and one detail template, so delivery mode is invisible |

## Journeys

The page set is derived from these, not from the data model.

| Question | Journey | Lands on |
|---|---|---|
| Is anything wrong right now? | Overview | attention items, each linking to the thing itself |
| Why is work not arriving? | Overview → the backlog → that service's queue | Service → Queue |
| What can I use, and who owns it? | Services → search | Service overview |
| Is this my named AI session? | Services → agents → description and full address | Service overview |
| How does this channel deliver? | Channels → mode and subscribers | Channel overview |
| Who may use my service? | Service → Access → one focused edit | back to Access, with the result |
| Who is this person and what may they administer? | Users → person | User detail |
| Why can I not do this? | any refusal → explanation and the corrective step | in place, or a problem page |
| Did **this** message arrive, and was it consumed? | Diagnostics → retained envelopes, scoped to what you may see | **not queue counters.** Totals cannot identify an individual message: accepted and dequeued are aggregate flow, and the [exchange contract](../../../docs/05-discovery.md#retained-exchanges) keeps envelope evidence, receipts and responses as separate things. Where an individual dequeue is not recorded, the page says it cannot be established rather than inferring it — codex's [S09](review/codex.md#specification-review-round-one) |
| Is work flowing through this queue at all? | Services → the service → Queue | queue counters, as aggregate flow, which is the question they can answer |
| What credentials do I hold? | Account | Account |
| What happened to this exchange? | Diagnostics → the envelope feed | Diagnostics |

## Pages

The planned page map follows. `⚠` marks a page that does not exist today.

| Page | URL | Purpose | State today |
|---|---|---|---|
| Overview | `/` | What needs attention, and nothing else | ⚠ new; `/` is diagnostics |
| Services | `/services` | Find a caller-visible non-Personal service | shared with channels |
| My services | `/services?scope=my` | Find a non-Personal service owned by the caller | a filter on Services |
| Personal services | `/personal` | Find Personal services in the existing owner-scoped view | exists as a separate top-level tab |
| Service | `/service?name=` | One service: overview, queue, activity, access, configuration | one long form page |
| Register service | `/services/new` | Create one | built in 0.5.64 |
| Channels | `/channels` | Find a channel, by delivery mode | shared with services |
| Channel | `/channel?name=` | One channel: mode, subscribers, queue, activity, access | rendered by the service template |
| Register channel | `/channels/new` | Create one | built in 0.5.64 |
| Activity | `/activity` | Observed traffic over a stated window | exists |
| Diagnostics | `/diagnostics` | Retained envelopes, losses, refusals | is the homepage today |
| Users | `/users` | Find a person or an identity | exists |
| User | `/user?name=` | Profile, authority, memberships, owned records | exists |
| Register user | `/users/new` | Create one | built in 0.5.64; the legacy empty `/user` route remains |
| Groups | `/groups` | Groups and caller-visible membership | linked compact table |
| Group | `/group?name=` | One group: members and authority-scoped editor | built |
| Register group | `/groups/new` | Create one | built; success opens Group detail |
| Account | `/account` | Own identity, own credentials, how to rotate | built in 0.5.79; the signed-in name opens it |
| Sign in | `/signin` | Token, and how to get one | exists |
| Problem | — | Refusal, expired session, unavailable bus, not found | one page for all four |

**Names stay query parameters.** I proposed path segments and both peers showed
the reasoning was wrong. `@` costs nothing — RFC 3986 permits it unencoded in a
path segment. The real obstacle is the **slash**: launcher-registered names
contain one (`codex/review-web@realm`), and opencode counted 206 of 232
directory names with a slash — the majority case, not an edge. In a segment that
is `%2F`, and correctness then depends on escaped-segment matching and
unescaping behaving identically through the mux, every hand-built href and every
return-URL round trip.

One rule for every resource: `?name=`. Not segments for some and queries for
others. Both forms bookmark equally well, so the ambition bought nothing and
would have touched every form and every legacy link.

## Navigation

Six destinations. Diagnostics is deliberately last, and Account is not in the
row — it sits with the principal's name at the top right, where an account
control is looked for.

`Overview · Services · Channels · Activity · Users · Groups · Diagnostics` —
then, separately, the signed-in name → Account, and Sign out.

The shared `shell()` gives every signed-in page its navigation and sign-out
([inventory](review/current-state.md#routes-and-templates)). Since 0.5.78 the
Channels collection also carries its own document title, heading, active
navigation entry and canonical Channel detail links; the historical C06 defect
is closed. Since 0.5.79 the signed-in name links to Account and credential facts
no longer occupy the Diagnostics page.

The current location is marked. Every page carries the same shell; the shell is
one thing in one place.

**Resource sections have a second navigation row.** Creation is a destination,
not a form appended to a list and not an unrelated heading action:

| Section | Second-level navigation |
|---|---|
| Services | All (`#`) · My (`#`) · Personal (`#`) · Register service |
| Channels | All channels · Register channel |
| Users | All identities (`#`) · Register user |
| Groups | All groups · Register group |

The current entry is marked. Service counts cover the records in each
caller-visible category before search, state and kind filters: All and My omit
Personal services; Personal uses the established owner-scoped Personal view.
All and My overlap deliberately: My is the caller-owned subset of All. Personal
is separated from both.

A register entry is shown only when the caller may perform that action; hiding
the link is a convenience and the daemon still authorizes submission. Detail
pages keep the same section navigation, so a person can return to the list or
start another registration without climbing through the global menu.

## What moves, and what stops being shown

The homepage's seven sections, resolved:

| Section today | Goes to |
|---|---|
| node counts | Overview, with the scope stated. The node's totals and a caller-filtered list are different numbers ([W15](../done/web-review.md#findings)) |
| unclean-stop warning | Overview attention item |
| refusal counts | Overview if non-zero; Diagnostics always, by reason |
| stuck inboxes | Overview attention items, linking to the service. "Stuck" is not a fact the daemon has — it is a backlog, with or without a reader, so the heading says *inboxes holding messages* |
| retained exchanges | Diagnostics |
| **the whole registry table** | **Removed — but only after Services and Channels carry what it uniquely showed.** codex's precondition ([C08](review/codex.md#junk-and-misleading-content)) and it is right: the lists today omit kind, description and accepted/dequeued, so removing the table first would lose real data. No usage research says nobody reads it; the argument for removal is that it answers no question, not that nobody looked |
| loss by name | Diagnostics, and per-record on the service page where it belongs |
| my names / fingerprints | Account |
| three explanatory paragraphs | help beside the control they explain, not a wall at the bottom |

Candidates for removal or demotion. Codex's junk list is the evidence; this is
the shape it takes here.

| Shown today | Proposal |
|---|---|
| configuration digest, on the list and the detail | Demote. It answers "did this change" for us, not for an operator. Keep it on the configuration section, not in a table |
| `Controls: Manage / View` column | Remove. It is text that looks like a control and is not ([W02](../done/web-review.md#findings)). Authority belongs on the row's link, or nowhere |
| `Maximum: 0` repeated down the activity page | Remove ([W09](../done/web-review.md#findings)) |
| blank labels for unset values | Replace with the fact **only where the absence changes a decision** — uses the daemon default, not observed, none queued. Where it does not, drop the label with the value. codex's correction: stating every absence is its own wall of noise |
| `/avatar` endpoint | Reachable and authenticated; simply referenced by no current template ([inventory](review/current-state.md#routes-and-templates)). That is what the source establishes — my earlier "loaded gun" was an overclaim, no defect is shown. Decide deliberately: use it on Users, or remove it |
| every service POST returning to `/services?scope=my` | **Built in 0.5.78:** registration and ordinary edits return to the affected Service or Channel; removal returns to the matching collection ([W13](../done/web-review.md#findings)) |
| group membership visible only inside the editor | A non-administrator sees a group name with no members and no explanation of why. Say which it is: empty, or hidden from you |
| registry iteration order | Replace with stable ordering ([W16](../done/web-review.md#findings)) |
| in / out | Rename to accepted / dequeued. Dequeued is not completed |
| Serving / Offline | Replace with the numeric **Readers** observation. Explicit zero, a positive count and unavailable stay distinct; none claims process health ([W03](../done/web-review.md#findings)) |
| Active / Inactive | Enabled / Disabled — an administrative state, not liveness |

## Overview, and the two guards on it

An attention-only homepage fails in two ways if it is not specified precisely.

**Its sources are enumerated, not judged.** "Needs attention" must be a list the
face can compute, or it becomes vibes. The candidates, each with an observed
basis and a link to the thing itself: an unclean last stop, refusals by reason,
backlogs, queues at their bound, and losses. Each row states what was observed
and when. codex's constraint applies throughout: a severity we cannot derive
from an observation is not ours to invent — a queue worker between pulls is a
backlog with no reader and no incident.

**Empty reads positively.** "Nothing needs attention, as of 14:22" with the
observation scope stated — never "healthy", which is a claim about the system,
and never a bare empty page, which reads as breakage. That is the zero-versus-
absent mistake one level up.

The page also carries conspicuous navigation into Services and Channels, because
an attention list is not a place to start a search from, and attention links
route by kind so an incident is never fragmented by the Services/Channels split.

## Permission and visibility

Stated carefully, because my first draft was not. **The daemon's visibility
rules decide what a caller may read — always, and they are the only thing that
does.** The face adds nothing and relaxes nothing.

What the face must stop doing is *narrowing that further by edit permission*.
Access policy, queue settings, address and protocol are already in the answer
the daemon returned to this caller; today they sit inside `.CanManage` blocks,
so a visitor who may not edit loses the summary along with the form
([C04](review/codex.md#junk-and-misleading-content),
[W04](../done/web-review.md#findings)). Controls are conditional; already-
returned metadata is not.

A record the caller may not see does not exist, and says so the same way a
missing one does — hiding and refusing are different answers and only one of
them is safe to give.

## Open

- Whether Diagnostics and Activity are two pages or two sections of one.
