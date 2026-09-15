# R1.2

## Scope

Exploratory and unscheduled. These ideas were explicitly deferred until after the tools stage; moving them here approves no mechanism.

| Topic | Canonical knowledge |
|---|---|
| Service identity and shared state | [Service identity and shared state](exploration.md#shared-secrets-and-a-kv-with-locks) |
| Daemon components as services | [Daemon components as services](exploration.md#whether-the-daemons-own-parts-become-services) |
| Removed names protection | [Removed names](#removed-names) |

Open choices are in [questions](QUESTIONS.md#open-questions); recorded choices are in [decisions](DECISIONS.md#recorded-decisions).
Execution prerequisites are in [TODO](TODO.md#objective).

## Removed names

Deferred from MVP by owner revision, 2026-09-15. Nothing is designed here;
moving it here approves no mechanism.

MVP [unregistering](../../docs/01-identity.md#unregistering) removes an address
and its credential and keeps nothing else: a removed name is reserved for
nobody, and whoever asks for it next gets it.

MVP originally held two things back, and both were removed for costing more
than they were worth at this stage:

| Held back | What it bought | Why it went |
|---|---|---|
| The name, reserved to its last owner across restarts | Its owner could take it back, and a stranger could not take it | A permanent reservation per address, including every throwaway one |
| The credential, still valid after the address went | The owner could still call, and the name could be reclaimed | A permanent credential per address; one launcher-smoke run left roughly 250 of them in one person's name list |

What a later release would have to answer before restoring either:

- who a removed name belongs to, and for how long — forever is what MVP had, and what filled the list;
- whether a stranger asking for a removed name is refused or told nothing;
- whether a name a person deliberately gave up can be taken by anyone, and how they give it up if not;
- whether reclaiming is a distinct request rather than an ordinary registration that happens to succeed.

A squatting or impersonation risk is the case that would justify it: a name a
peer has learned and still sends to, taken by somebody else. MVP accepts that
risk because every name on the bus is one person's own.
