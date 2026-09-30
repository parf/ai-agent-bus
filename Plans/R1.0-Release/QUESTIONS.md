# R1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q20 | A chaining namespace and an agent template both want the `/` | owner, with chaining | [overview § chaining](federation.md#chaining) |
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

## Federation context

❓ **A namespace and an agent template both want the `/`.** A name holds at
most one, and it already means *template* / *instance*
([identity § names](../../docs/01-identity-and-roles.md#names)), so `team/ci@realm` parses as
template `team`. Either a chaining namespace *is* the template part, or
chaining needs a separator of its own. *Settled by:* the owner, when chaining
is designed.

## Runner context

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
answer. *Settled by:* owner, with the runner.

## Modules context

❓ **How `protocol` is specified for five languages** — a document, a shared
schema, or a generator? Nothing can be reimplemented consistently until this is
answered, and it is the gate on the client libraries. *Settled by:* owner, when
data models are taken up.
