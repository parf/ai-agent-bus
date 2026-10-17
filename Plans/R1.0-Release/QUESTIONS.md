# R1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan. Q13 and Q107 were settled on 2026-09-30, Q12, Q15, Q19, Q33, Q139, Q150–Q158, Q162 and Q163 on 2026-10-16 ([decisions](DECISIONS.md#recorded-decisions)); Q140 was withdrawn the same day, since locks no longer use a Group's membership.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q145 | Tag normalization, literal `no-tags`, multiple-tag matching and counts with active filters | owner, with R1 | [record tags](#record-tags) |
| Q20 | A chaining namespace and an agent template both want the `/` | owner, with chaining | [federation § chaining](federation.md#chaining) |
| Q17 | How `protocol` is specified across client languages | owner, with data models | [client libraries](done/client-libraries.md#scope) |
| Q137 | Whether R1's chaining is a lookup-only fallthrough without AUTH, signed generations and peer sync, or waits for R1.1 | owner, when federation comes off hold | [federation](federation.md#chaining) |
| Q146 | Confirm the implemented assigned-role representation, duplicate actor grants and applicable record kinds | owner | [assigned roles](#assigned-roles) |
| Q159 | A shared (pool) agent cannot forward to several runners: one record has one `deliver_to` slot. Does each member runner read the pool name's own inbox, as one of several readers, instead of forwarding? | owner, before M.6 | [one name on many hosts](runner-implementation.md#one-name-on-many-hosts) |
| Q160 | With `in_flight` above 1, answers cannot be matched by topic and tag, which are the caller's labels and need not be unique. Does each frame carry the message ID, echoed on its answer, amending Q13? | owner, before M.3 | [long-lived agents](runner-implementation.md#long-lived-agents) |
| Q161 | An env file change cannot reach a running process by `SIGHUP`: env is set only at exec. Does a changed env (or secret) restart the agent's kept processes after their work drains, while override files and bus `config` keep `SIGHUP`, amending Q19? | owner, before M.5 | [stop and reload](runner.md#stop-and-reload) |
| Q164 | A serve-only grant: the Owner names a User who may fetch a record's token to serve it, shown on the record and revocable, so a runner can serve a name its Owner keeps; today the name is transferred instead | owner, to discuss | [who it runs as](runner-implementation.md#who-it-runs-as) |
| Q147 | Confirm retaining current nested-group behavior for roles instead of the older flat-groups wording | owner | [assigned roles](#assigned-roles) |

## Record tags

Q145: decide case normalization and allowed tag characters; whether literal
`no-tags` is reserved; whether multiple query tags mean any or all; and whether
web counts use the whole visible list or the currently filtered list.
*Settled by:* owner, with R1. The proposed behavior lives in
[record tags](tags.md#classification-and-filtering).

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
(Pools' host field moved to R1, Q139.) For chaining, either the R1 version is cut down to stand alone, or the
machinery moves to R1, or the topic waits for R1.1. *Settled by:* owner.

## Runner context

## Modules context

❓ **How `protocol` is specified across client languages** — a document, a shared
schema, or a generator? Nothing can be reimplemented consistently until this is
answered, and it is the gate on the client libraries. *Settled by:* owner, when
data models are taken up.
