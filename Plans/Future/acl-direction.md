# Directional access

Status: proposed, unassigned. No release owns this yet.

## Where direction is needed

**Only a queue or a pub/sub channel needs two lists.** Everywhere else one side
of the exchange is fixed by what the record is, so a single list is the whole
answer. Owner decision of 2026-09-18.

| Record | Who reads it | What the list decides |
|---|---|---|
| Registered queue | anyone admitted | **read and write, separately** |
| Pub/sub channel | every admitted subscriber | **read and write, separately** |
| A person's own queue | that person, and nobody else | who may write |
| An agent's queue | that agent's processor, and nobody else | who may write |
| An external-service record | nothing is delivered to it at all ([service](../../docs/03-records.md#record-kinds)) | who may view and edit the entry |

A queue and a channel are the only records where both ends are open, because
they exist to be shared: somebody publishes, somebody else takes. For the other
three the reader is a consequence of the record, not a grant — which is why
naming it in a list would be a way to get it wrong.

## What that changes

<details>
<summary>The reader is not fixed today</summary>

MVP asks one undirected question for every use of a record. `may(caller, record)`
is the check for sending (`core/bus.go:579`) and the same check for consuming
(`core/bus.go:858`), and `ConsumeAs` takes a caller and a name that need not
match — `agent-bus consume --inbox <name>` is a supported call. So a person's
queue or an agent's queue carrying `--allow '*'` can be **drained** by any
admitted principal, not merely written to.

Those records read as reader-fixed today only because they are registered with
an empty allow list, which means owner and maintainers alone
([ACL](../../docs/02-access.md#acl)). That is a default, not a rule.

So this work has two halves, and the second is the larger one: give queues and
channels a second list, **and** stop treating read as an ACL question for the
records whose reader is fixed. Without the second half the first changes nothing
about the case that motivated it.

</details>

**Management authority is not affected.** An owner and an assigned maintainer
reach a record through `manages`, which is a separate path from the allow list
and stays that way; direction is about admitted principals, not about who
administers the record.

Open: [Q74](QUESTIONS.md#open-questions).
