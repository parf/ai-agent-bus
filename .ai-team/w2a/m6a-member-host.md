# M.6a: where a pool member says it is

📌 **TL;DR:** A pool is one name read by several runners (№Q159), so the name no
longer says which host serves it. This proposes where a member's hostname lives,
what sets it and how a listing shows it. Recommended: **A**, the hostname rides on
each read, and the daemon keeps a short-lived set per name.

For the owner. Proposal only: nothing here is built or decided.

## What is already settled

| Settled | Source |
|---|---|
| Members of a pool each read the pool name's own inbox, as one of several readers; nothing forwards | №Q159, [one name on many hosts](../../Plans/R1.0-Release/runner-implementation.md#one-name-on-many-hosts) |
| The hostname is stated by the member, never observed; a label, never an input to routing or authority | [where a member says it is](../../Plans/R1.0-Release/runner-implementation.md#where-a-member-says-it-is) |
| One entry per member, refreshed while it serves and gone when it is; not part of the identity | same section |
| `readers` counts outstanding reads now and is not durable | [discovery § readers](../../docs/05-discovery.md#readers) |

## The choice

| | A. On each read 🌟 | B. A registration field | C. A presence call |
|---|---|---|---|
| Where it lives | in the daemon's memory, beside `readers`: per name, a set of hostnames with when each last read | in the record, durable | in the daemon's memory, per name |
| What sets it | every consume carries the reader's hostname; the runner fills it in, as `share` already rides there | `register` carries it; a pool registers one name many times, so the record would hold a list | the runner calls it on a timer |
| When an entry goes | its last read is older than a window (2 minutes proposed); nothing to clean up after a crash | never by itself: a dead member stays listed until someone edits the record | when its timer stops, after a window |
| New endpoints | none; one optional parameter on an existing one | none; one record field | one |
| Fits "refreshed while it serves, gone when it is" | yes, reads are the heartbeat | no: durable, and register is not repeated while serving | yes, but a second heartbeat beside the reads |
| Cost | memory per (name, host), bounded; nothing written to the database | a write per registration; stale data in backups | an extra call per member per interval |

**Recommendation: A.** The read is already the member's proof of life, so the
hostname costs one parameter and no timer. A busy member whose places are all
full has no read outstanding for a while; the window covers that, so a working
member never disappears from the listing.

## A, in detail

| Question | Proposal |
|---|---|
| The value | the host's own name as the member states it, 1 to 64 characters of letters, digits, `.` and `-`; anything else refused by name (400), never silently dropped |
| Who sends it | the runner, from the host's name, overridable in its unit for hosts whose name says nothing; the dedicated runner and client libraries may send it too; a read without it is still a read and simply adds no host |
| The window | 2 minutes since that host's last read began or ended, so the runner's 55-second read and a full pool both stay listed |
| Bound | at most 32 hosts per name; beyond that the oldest is forgotten, and the listing says it is cut |
| Not | an identity, a route, an ACL input or a durable fact: it is never written to the database or a backup |

How it shows:

| Face | Shows |
|---|---|
| `agent-bus ls <name>` (one record) | a `Hosts` line: each hostname with how long ago it last read, e.g. `srv1 (now), srv2 (40s)` |
| `ls` table | unchanged; the `Readers` count stays the one column |
| Web record page | the same list under Readers |
| Lookup JSON | a list of hosts, each with its name and last-read time, present only when non-empty; visible to whoever may see the record's `readers` today |

## Questions for the owner

1. A, B or C? (recommended A)
2. Is a 2-minute window right, or should it follow the runner's read wait?
3. Should a host's version ride with it now ([agent version reporting](../../Plans/R1.0-Release/runner-implementation.md#agent-version-reporting)), or wait?
