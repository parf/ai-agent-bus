# R1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q20 | A chaining namespace and an agent template both want the `/` | owner, with chaining | [federation § chaining](federation.md#chaining) |
| Q110 | Whether Resource and Resource Template are two record kinds or one with a template flag, and which glyphs they take | owner, with R1 | [Resource records](resources.md#what-is-not-decided) |
| Q111 | What a Resource record stores to name its subject — the URI, and whether MIME type, size, icons and annotations are kept at all — and whether the name takes a sigil | owner, with R1 | [Resource records](resources.md#what-is-not-decided) |
| Q114 | Where a read of a Resource goes, since a Resource has no queue: the daemon forwards it to an Agent the record names, or a Resource is itself an Agent with a queue. "An Agent in disguise" and "Not a channel" disagree until this is answered | owner, with R1 | [Resource records](resources.md#resource-records) |
| Q113 | Which basic protocols `agent-busd` serves itself for a Resource, if any, and what that does to the process and trust boundaries | owner, with R1 | [Basic protocols](resources.md#basic-protocols-in-the-daemon) |
| Q107 | Who may read and write a registry record's store — the record's ACL, its Maintainers, or only the principal of that name | owner, with R1 | [key-value store](kv.md#per-name-storage) |
| Q12 | Whether start-on-demand is built beside the wrapped call, and what idle stops a service that was started that way | owner, in R1 | [runner § on demand](runner.md#on-demand) |
| Q13 | Whether one kept child may have several messages in flight | owner, when a service asks | [runner § long-lived services](runner.md#long-lived-services) |
| Q15 | Who vouches for a runner's name on a host that runs no daemon | owner, with the runner | [runner § where it runs](runner.md#where-it-runs) |
| Q19 | What happens to a running service when its configuration changes | owner | [services § configuring a template](../../docs/03-records.md#configuring-a-template) |
| Q33 | Whether backup is a runner verb, a bundled service, or neither | owner | [context](runner.md#backing-it-up) |
| Q17 | How `protocol` is specified for the five client languages | owner, with data models | [future clients](modules.md#modules) |
| Q137 | Whether R1's chaining is a lookup-only fallthrough without AUTH, signed generations and peer sync, or waits for R1.1 | owner | [federation](federation.md#chaining) |
| Q138 | Whether R1's roles ship as flat strings, or wait for the group expression engine that comes with AUTH in R1.1 | owner | [roles](roles.md#record-defined-roles) |
| Q139 | Whether the member hostname field moves to R1 with pools, or pools ship without it | owner, with the runner | [runner § one name on many hosts](runner.md#one-name-on-many-hosts) |

## Federation context

❓ **A namespace and an agent template both want the `/`.** A name holds at
most one, and it already means *template* / *instance*
([identity § names](../../docs/01-identity-and-roles.md#names)), so `team/ci@realm` parses as
template `team`. Either a chaining namespace *is* the template part, or
chaining needs a separator of its own. *Settled by:* the owner, when chaining
is designed.

## Split context

❓ **R1 topics written on R1.1 machinery.** The 2026-09-30 split kept these
topics in R1, but each was written against something now in R1.1, which ships
later. Chaining pins upstream keys and takes signed AUTH generations (Q137).
Roles compose with the group expression engine that "comes with AUTH" (Q138).
Pools name a member's host in a field written up in R1.1's discovery plan
(Q139). For each, either the R1 version is cut down to stand alone, or the
machinery moves to R1, or the topic waits for R1.1. *Settled by:* owner.

## Runner context

❓ **Several messages in flight inside one child.** One at a time needs no
correlation; letting a child work on several would, and the envelope already
carries what that costs — a reply matches on topic and tag
([messaging § request and reply](../../docs/04-messaging.md#request-and-reply)), so the
child would echo the tag and replies could come back in any order. Left open
because nothing needs it yet and adding it later breaks nothing.
*Settled by:* owner, when a service asks for it.

❓ **A second kind of on demand** ([runner § on demand](runner.md#on-demand)).
The wrapped call is settled: no consumer, and the daemon hands the call to the
record's fallback channel for the runner to execute. Open is whether the other
kind is built beside it — the call starting the **service**, which then reads
its own inbox the ordinary way — and, if so, what idle stops it again. They
answer different questions, how rare against how expensive to start, so the
second is not a replacement for the first. *Settled by:* owner, in R1.

❓ **What a backup is driven by** — a runner verb, a bundled service, or
neither. It is one encrypted archive either way, which is why the shape is
settled here and the trigger is not. *Settled by:* owner, with R1.

❓ **Who vouches for `runner@<edge>` when that host runs no daemon.** A
`user@host` realm is vouched for by that host's `agent-busd`
([identity § names](../../docs/01-identity-and-roles.md#names)), and an edge box has none — so the
name it registers under is the one case the realm rule does not already
answer, and the vouching may need R1.1's identity work. *Settled by:* owner,
with the runner.

## Modules context

❓ **How `protocol` is specified for five languages** — a document, a shared
schema, or a generator? Nothing can be reimplemented consistently until this is
answered, and it is the gate on the client libraries. *Settled by:* owner, when
data models are taken up.
