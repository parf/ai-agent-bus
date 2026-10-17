# R1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan. Q13 and Q107 were settled on 2026-09-30, Q12 and Q150–Q158 on 2026-10-16 ([decisions](DECISIONS.md#recorded-decisions)); Q140 was withdrawn the same day, since locks no longer use a Group's membership.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q20 | A chaining namespace and an agent template both want the `/` | owner, with chaining | [federation § chaining](federation.md#chaining) |
| Q15 | Who vouches for a runner's name on a host that runs no daemon | owner, with the runner | [runner § where it runs](runner-implementation.md#where-it-runs) |
| Q19 | What happens to a running agent when its configuration changes | owner | [agents § configuring a template](../../docs/03-records-agent.md#configuring-a-template) |
| Q33 | Whether backup is a runner verb, a bundled agent, or neither | owner | [context](runner-implementation.md#backing-it-up) |
| Q17 | How `protocol` is specified across client languages | owner, with data models | [client libraries](done/client-libraries.md#scope) |
| Q137 | Whether R1's chaining is a lookup-only fallthrough without AUTH, signed generations and peer sync, or waits for R1.1 | owner, when federation comes off hold | [federation](federation.md#chaining) |
| Q139 | Whether the member hostname field moves to R1 with pools, or pools ship without it | owner, with the runner | [runner § one name on many hosts](runner-implementation.md#one-name-on-many-hosts) |
| Q146 | Confirm the implemented assigned-role representation, duplicate actor grants and applicable record kinds | owner | [assigned roles](#assigned-roles) |
| Q147 | Confirm retaining current nested-group behavior for roles instead of the older flat-groups wording | owner | [assigned roles](#assigned-roles) |

## Assigned roles

Q146: the implementation uses existing `allow` strings, preserves separate
same-actor grants, and supports assignments on records with ACLs, excluding
User records and Group membership. Owner confirmation of this representation
remains open; the implemented behavior is documented in
[roles](../../docs/03-records-agent.md#roles).

Q147: the implementation preserves current nested-group membership, including
the protected Administrator boundary. Confirm the older plan's flat-groups
wording should be retired; group expressions remain R1.3 scope.

## Federation context


❓ **A namespace and an agent template both want the `/`.** A name holds at
most one, and it already means *template* / *instance*
([identity § names](../../docs/01-identity-and-authority.md#names)), so `team/ci@realm` parses as
template `team`. Either a chaining namespace *is* the template part, or
chaining needs a separator of its own. *Settled by:* the owner, when chaining
is designed.

## Split context

❓ **R1 topics written on R1.1 machinery.** The 2026-09-30 split kept these
topics in R1, but each was written against something now in R1.1, which ships
later. Chaining pins upstream keys and takes signed AUTH generations (Q137).
Pools name a member's host in a field written up in R1.1's discovery plan
(Q139). For each, either the R1 version is cut down to stand alone, or the
machinery moves to R1, or the topic waits for R1.1. *Settled by:* owner.

## Runner context

❓ **What a backup is driven by** — a runner verb, a bundled agent, or
neither. It is one encrypted archive either way, which is why the shape is
settled here and the trigger is not. *Settled by:* owner, with R1.

❓ **Who vouches for `#runner@<edge>` when that host runs no daemon.** A
`user@host` realm is vouched for by that host's `agent-busd`
([identity § names](../../docs/01-identity-and-authority.md#names)), and an edge box has none — so the
name it registers under is the one case the realm rule does not already
answer, and the vouching may need R1.1's identity work. *Settled by:* owner,
with the runner.

## Modules context

❓ **How `protocol` is specified across client languages** — a document, a shared
schema, or a generator? Nothing can be reimplemented consistently until this is
answered, and it is the gate on the client libraries. *Settled by:* owner, when
data models are taken up.
