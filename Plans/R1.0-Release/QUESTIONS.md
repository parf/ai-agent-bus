# R1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan. Q13 and Q107 were settled on 2026-09-30, Q12, Q15, Q19, Q33, Q139, Q150–Q158, Q162 and Q163 on 2026-10-16, Q159 and Q164 on 2026-10-17 ([decisions](DECISIONS.md#recorded-decisions)); Q140 was withdrawn the same day, since locks no longer use a Group's membership.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q145 | Multiple-tag query matching | owner, with R1 | [record tags](#record-tags) |
| Q20 | A chaining namespace and an agent template both want the `/` | owner, with chaining | [federation § chaining](federation.md#chaining) |
| Q17 | How `protocol` is specified across client languages | owner, with data models | [client libraries](done/client-libraries.md#scope) |
| Q137 | Whether R1's chaining is a lookup-only fallthrough without AUTH, signed generations and peer sync, or waits for R1.1 | owner, when federation comes off hold | [federation](federation.md#chaining) |
| Q146 | Confirm the implemented assigned-role representation, duplicate actor grants and applicable record kinds | owner | [assigned roles](#assigned-roles) |
| Q160 | With `in_flight` above 1, answers cannot be matched by topic and tag, which are the caller's labels and need not be unique. Does each frame carry the message ID, echoed on its answer, amending Q13? | owner, before M.3 | [long-lived agents](runner-implementation.md#long-lived-agents) |
| Q161 | An env file change cannot reach a running process by `SIGHUP`: env is set only at exec. Does a changed env (or secret) restart the agent's kept processes after their work drains, while override files and bus `config` keep `SIGHUP`, amending Q19? | owner, before M.5 | [stop and reload](runner.md#stop-and-reload) |
| Q165 | A person's runner is `#runner/<person>@<realm>` (M.10), and an edge runner `#runner/<edge>@<realm>` (Q15): one slot, two meanings. Which names which, or do they compose (for example `#runner/<person>-<edge>`)? | owner, before M.6 | [who it runs as](runner-implementation.md#who-it-runs-as), [where it runs](runner-implementation.md#where-it-runs) |
| Q166 | The system-call filter needs a compiler for bubblewrap's `--seccomp FD`. Recommended: `github.com/elastic/go-seccomp-bpf`, pure Go and established, compiling a deny list like systemd's `@system-service` complement; or `libseccomp-golang`, which needs cgo and so breaks the pure-Go build; or a small filter of our own. Which? | owner, to finish M.4 | [sandboxing](runner-implementation.md#sandboxing), [modules](../../src/MODULES.md#external-tools) |
| Q167 | The work directory's soft limit has no size in the approved `limits` format. Add `"disk": "1G"` to `limits` (default 1G, measured, the agent stopped and reported by name when over), or derive it from `needs.writable`? | owner, to finish M.4 | [sandboxing](runner-implementation.md#sandboxing), [file formats](runner-config.md#file-formats) |
| Q147 | Confirm retaining current nested-group behavior for roles instead of the older flat-groups wording | owner | [assigned roles](#assigned-roles) |

## Record tags

Q145: if a future query selects multiple tags, should they match any or all?
The current interface selects one tag. Normalization, the synthetic `no-tags`
label and filtered counts are settled in [record tags](../../docs/03-records.md#tags).
*Settled by:* owner, with R1.

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
