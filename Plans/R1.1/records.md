# Record lifetime

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

Manual removal of idle addresses is now [built in MVP](../../docs/01-identity.md#unregistering).
Automatic lifetime and removal while served remain future scope below.

## Down and retired

**A record says what its owner means it to be, and what separates the words is
what a caller is told to do about it.** One verb sets it; the state is
**declared**, never inferred — the same distinction the hostname field is built
on ([where a member says it is](../R1/discovery.md#where-a-member-says-it-is)).

| Declared | Means | What a caller should do | Answer |
|---|---|---|---|
| **down** | out of service, back eventually — this is [today's disabled record](../../docs/01-identity.md#owner-control), not a state beside it | back off hard; roughly one retry an hour, not a loop | `409` |
| **retired** | gone for good | stop, and fix the code that still sends here | `410` |

*"Not today"* and *"not ever"* are different, and a caller can act on the
difference: back off, or stop. Collapsing them throws away the only thing the
owner knew and the caller did not.

**Both are the record's owner or its assigned maintainers, in both
directions** — the [record management
authority](../../docs/01-identity.md#groups-and-maintainers) that already
disables and deletes. Declaring and undoing are the same authority: whoever may
retire a name may bring it back. That adds no new authority, needs no new
group, and leaves nobody able to make a change they cannot reverse. The daemon
owner is not named here and does not need to be — it reaches everything
already, and listing it per operation would invite the reading that somewhere
it does not ([authority](../../docs/01-identity.md#groups-and-maintainers)).

**The daemon does not retry on anybody's behalf.** These say what a caller
should do; nothing in the bus holds a refused send and tries again later, and
nothing here proposes that it should. The states already settled what happens
to traffic meanwhile: what is queued stays, what is new is refused.

**Neither is a `500`, because neither is a fault.** `500` keeps the one meaning
it has: something actually went wrong, and it was ours. A service that is down
is down *on purpose* — somebody decided it — so the two are opposite claims.
The distinction is also counted: a 500 is deliberately **left out of the
refusal totals**, so that
*"how often am I refusing callers?"* is not answered with a number that
includes our bugs ([refusals](../../docs/05-discovery.md#refusals)). An owner's
decision answered as a server fault would be invisible there and
indistinguishable from a daemon bug in the log.

`409` is what a disabled record answers **today**, so `down` changes nothing
for a caller that already handles it, and *conflicts with the state this
resource is in* is what an administrative state is.

**Nothing new accumulates is not the same as nothing is there.** A backlog
already in the inbox is kept; what is refused is anything new. That is exactly
what disabling does today ([owner control](../../docs/01-identity.md#owner-control)),
and it holds for retirement too: work somebody already accepted is not thrown
away because the name it was for has been given up. Nothing is kept forever by
it either — what is in a queue is subject to the TTL it already had
([message ttl](../../docs/04-messaging.md#message-ttl)), so a backlog nobody
will ever read empties itself rather than becoming permanent.

**410 is chosen for what it means, not because it was free.** *Gone* is the
one code that says a name was real, is not coming back, and that this is
intentional — and a caller may cache that answer, which is the correct thing to
do about something permanent.

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

So **undoing retirement is a person's act, not the service's.** The service
cannot ask — it has nothing left to ask with. Whoever manages the record can,
as themselves: a record's owner and maintainers are on the record, and a
person's own credential is the one thing removing a record never takes.
A name brought back needs a credential again the ordinary way, by the owner
asking for one for a name they own
([getting a token](../../docs/02-access.md#getting-a-token)), or by the service
re-registering when it starts.

**The health checker ignores both**, because both are declared: a service that
was turned off on purpose is not a service that failed
([health checker](../R1/discovery.md#health-checker)). What the checker reports
is observed, what this verb writes is stated, and a listing that cannot tell
them apart cannot answer *did this break, or did you mean it*.

### Coming back in a moment is not one of them

**A service that is restarting says so itself, and nothing writes it down.**
*Briefly unavailable* is a `503`, and it comes from the service over the
protocol — the thing knows it is warming up, and it is the only thing that
does. It is **not settable**: no verb declares it, the registry has no such
state, and an owner reaching for one is reaching for `down`.

That keeps the axis clean. A **declared** state is a decision somebody made and
the registry holds; **observed** is what the health checker concluded
([health checker](../R1/discovery.md#health-checker)); and this is neither —
it is the service answering for itself, in the moment, and gone the moment it
is true again. A record that could be marked briefly-down would be a third
thing to keep in step with the other two, and stale the second nobody updated
it.

**`503` says one thing here, and a full inbox no longer says it.** A full
inbox answers `429` — the sender is outrunning the reader, which is what that
code is for — leaving `503` to the service alone. That is
[built in MVP](../../docs/04-messaging.md#overflow); without it a caller cannot
tell *the service is restarting* from *the reader is behind*, two problems with
different owners and different fixes.

Telling them apart also matters less often than it looks, and that is not a
reason to skip it. A daemon refusal arrives from the bus; a service's own
answer arrives from the service, where [`protocol`](../../docs/03-services-and-topics.md#how-to-call-it)
says the caller speaks to it directly — different endpoints, usually a caller
that knows which it asked. The codes make it true on every path, including one
where something answers on a service's behalf.

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

Nothing here is open. Scheduling the `429` change to a built behaviour is
[TODO](TODO.md#objective) work.

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
