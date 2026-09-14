# Record lifetime

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

Manual removal of idle addresses is now [built in MVP](../../docs/01-identity.md#unregistering).
Automatic lifetime and removal while served remain future scope below.

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
