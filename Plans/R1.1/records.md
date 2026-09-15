# Record lifetime

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

Manual removal of idle addresses is now [built in MVP](../../docs/01-identity.md#unregistering).
Automatic lifetime and removal while served remain future scope below.

## Down and retired

**A record says what its owner means it to be, and the difference between the
two words is what a caller is told.** One verb sets it; the state is
**declared**, never inferred — the same distinction the hostname field is built
on ([where a member says it is](../R1/discovery.md#where-a-member-says-it-is)).

| Declared | Means | A caller gets | The name |
|---|---|---|---|
| **down** | turned off for a reason, and nothing new accumulates while it is — this is [today's disabled record](../../docs/01-identity.md#owner-control), not a second state beside it | an error it may be worth retrying: the thing may be back | stays its owner's, as it does now |
| **retired** | gone for good | an error that is not worth retrying — a caller still sending to it has a bug to fix | **held**, so nothing else can appear under it |

The two errors are the point. *"Not now"* and *"not ever"* are different
answers, and a caller can act on the difference: wait, or stop. Collapsing them
into one refusal throws away the only thing the owner knew and the caller did
not.

**Nothing new accumulates is not the same as nothing is there.** A backlog
already in the inbox is kept; what is refused is anything new. That is exactly
what disabling does today ([owner control](../../docs/01-identity.md#owner-control)),
and it holds for retirement too: work somebody already accepted is not thrown
away because the name it was for has been given up. Nothing is kept forever by
it either — what is in a queue is subject to the TTL it already had
([message ttl](../../docs/04-messaging.md#message-ttl)), so a backlog nobody
will ever read empties itself rather than becoming permanent.

**A retired name must never answer *no such name*.** That is the one refusal it
must not borrow: a caller cannot tell it from a typo, and telling a caller its
own name is wrong when the name is right is the answer that sends somebody
looking in the wrong place. Each declared state answers with its own code and
its own words, and a caller acts on them. The daemon already works this way —
one table turns a refusal into a status and a counted reason, so the reason and
the code cannot drift apart ([refusals](../../docs/05-discovery.md#refusals)) —
and `down` already has an answer of its own there. Retirement needs one too;
which code it is belongs to whoever writes it.

**A retired name keeps no credential.** It is dropped with the address, exactly
as [unregistering](../../docs/01-identity.md#unregistering) drops it, and for
the reason MVP found: a credential per given-up name is what filled a person's
list. Holding the name costs the record that is already there; holding a
working credential is the expensive half, and this does not.

So **undoing retirement is the owner's own act, not the service's.** The
service cannot ask — it has nothing left to ask with. Its **owner** can, as
themselves, and so can the **daemon owner**; a record's owner is on the record,
and a person's own credential is the one thing removing a record never takes.
A name brought back needs a credential again the ordinary way, by the owner
asking for one for a name they own
([getting a token](../../docs/02-access.md#getting-a-token)), or by the service
re-registering when it starts.

**The health checker ignores both**, because both are declared: a service that
was turned off on purpose is not a service that failed
([health checker](../R1/discovery.md#health-checker)). What the checker reports
is observed, what this verb writes is stated, and a listing that cannot tell
them apart cannot answer *did this break, or did you mean it*.

### Retired is not the reservation that was removed

[MVP removed name reservation](../../docs/01-identity.md#unregistering) because
every `unregister` bought one, including for every throwaway launcher address.
Retirement is the opposite shape: somebody decides, on a record that already
exists, and nothing accumulates on its own. Unregistering still keeps nothing
and still frees the name; retiring is the other way out, for a name worth not
reusing. **Protecting a name is this, and nowhere else**: the topic R1.2 was
holding for it is [retired](../R1.2/README.md#removed-names).

A retired record also survives what a deletion cannot: **newer record wins per
entry** has no representation for a record that is gone
([registry sync](../R1/registry.md#registry-sync)), so a deleted name returns
from whichever peer still holds it. A retired one is a record, and syncs like
any other.

What is left open is who may put a record into these states at all, given that
today's authority over a record is wider than the two people who may undo a
retirement ([Q46](QUESTIONS.md#open-questions)).

## How long a record lives

**A record is `kept` or `ephemeral`, and that is a different axis from its
kind.** Kind says what the thing is ([service kinds](../../docs/03-services-and-topics.md#service-kinds));
this says whether the registry is meant to hold it after nobody is using it.

| | Registered by | Expires |
|---|---|---|
| **`kept`** | the runner, and anything that asks for it | never on its own |
| **`ephemeral`** | `agent-bus start` and the dashboard, by default | after long inactivity — weeks, not hours |

The default falls where the registrations do: the runner keeps a list of what
is installed and means every entry to persist
([runner § the list of what is installed](../R1/runner.md#the-list-of-what-is-installed)),
while a name that appeared because somebody ran a command is incidental until
somebody says otherwise. A hand-started service that is meant to stay says so;
nothing is derived from who registered it, because a derived answer and the
runner's list would be two truths about one thing.

**Nothing that is being served ever expires.** `reading` already says whether a
read is outstanding on an inbox right now
([discovery § what a listing answers](../../docs/05-discovery.md#what-a-listing-answers)),
so the clock only ever considers records nobody is serving, and a rarely-called
tool that is sitting there connected is safe. Inactivity is measured from the
last thing that happened on the name — registered, read, or delivered to — all
of which the daemon already counts.

**A person may always delete, served or not.** The clock is restrained;
authority is not. But deleting a served record is narrower than it looks:

- The daemon **cannot stop the process**. No process the daemon starts may exec
  at all, and the runner is a separate program under its own account, not a
  child ([processes](../../docs/11-processes.md#processes-and-privileges)). Deleting forgets the record; the
  process keeps running, blind, and sends to it refuse as *no such name*.
- **It comes back if that process restarts**, because a service re-registers on
  every start ([identity § ownership](../../docs/01-identity.md#ownership)).

So for something running, the honest order is **stop it, then delete it**, and
delete-while-served is the escape hatch rather than the path.

⚠️ **Expiry is single-node until peer sync has a clock.** Deletion has no
representation in *newer record wins per entry* ([registry sync](../R1/registry.md#registry-sync)):
a deleted record returns from whichever peer still holds it, and a wrong clock
stops meaning *a stale record won* and starts meaning *a live service was
deleted somewhere else*. The open question there gates this one.
