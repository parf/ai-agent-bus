# Later exploration

Status: unscheduled. These topics were explicitly deferred until after the tools stage; no mechanism is approved by moving them here.

## Shared secrets, and a KV with locks

### What this is for

| | Wants |
|---|---|
| **secrets** | reliable, secure, **network-shared** storage for a service's secrets |
| **state** | reliable, fast, **network-shared** per-service KV — the **values**; the locks half is no longer part of this question |

The state example, which is the one that sets the bar: several **batcher**
processes share a list of batches, `id → todo · processing · done`, and exactly
one of them may move a batch to `processing`.

**The claiming half is settled and the storing half is not.** R1 gives
the daemon named locks ([messaging § shared locks](../R1.0-Release/locks.md#shared-locks)),
so *exactly one of them* is answered by the authority a pool already shares —
and what is left here is where the `id → status` map itself lives. That is a
smaller question than the one this page opened with: a store that holds values
need not also order two writers.

**And both wants now point at the same place.** `kv` is where shared service
state *and configuration* would live ([bundled services § data](../R1.1/services.md#data)),
which is the state want and the secrets want arriving at one service. What
keeps this page open is the part that does not follow from that: whether the
store has to be **blind** to what it holds, which is the table below — a
`kvrocks` underneath reads everything it is given.

**Network-shared** is the word that matters in both. A service may run
anywhere, including somewhere nobody here administers, and the members of a
pool sit on different machines ([runner § one name on many
hosts](../R1.0-Release/runner.md#one-name-on-many-hosts)).

### Why a service would want a key of its own

**A service can be anywhere — a cloud box, somebody else's host — and it must
not depend on its owner's key to run.** The owner's key is a person's, held by
that person; a service that needed it at every start would need a human
present, or a copy of a human's private key on a rented machine. Neither is
acceptable, so a service that holds secrets wants a key that is *its own*.

**What 1.1 answers, and what it leaves.** A service exchanges the credential it
holds for one scoped to whatever it is calling
([access § service to service](../R1.1/access.md#service-to-service)) — so every
hop *after the first* needs no key. What stays open is the first one, on a box
with no runner and nobody to ask, which is the only place this want still bites.

This is already assumed elsewhere: private config is sealed **to the service's
own key** ([identity § sealed private config](../R1.0-Release/identity.md#sealed-private-config)),
and nothing yet says where that key comes from. Deliberately nothing in the
main documents does — the owner key would appear only in a **backup** of a
service key, as a recovery path rather than a run-time dependency, and that
whole arrangement waits on this page.

### The open choice

**Tokens or keys — and in the daemon or beside it.** These are two questions
and they are not independent.

| | Tokens | Ed25519 keys |
|---|---|---|
| Who can read the stored bytes | whoever holds the token, and the daemon mints tokens | only the holder of the key; the daemon cannot |
| What it costs | nothing new — they exist | key generation, backup, rotation, recovery |
| What it buys | access control | the store, and the bus, are **blind** |

So the question is not which is better but **whether the store has to be blind
to the thing it holds**. Secrets say yes. Batch statuses say no — `todo` is not
a secret, and the batchers want ordering, not secrecy.

| | In `agent-busd` | A separate service |
|---|---|---|
| for | one hop, no bootstrap problem, one authority already exists — a pool is one bus | the daemon stays small; *a cache is a service with its own name* ([bundled services § rules they all obey](../R1.1/services.md#rules-they-all-obey)) |
| against | grows the daemon, and contended durable state pushes it toward being the database *no external broker* was written to avoid | two hops. It **can** now be a pool, which it could not before — the ordering comes from the daemon's locks rather than from agreement between its own members |

### Three things already in the design that may answer this

Worth checking before anything is built, because a want that is already served
is not a feature:

| | |
|---|---|
| **sealed private config** | the proposed daemon would hold opaque bytes it cannot read, sealed to the service's key, versioned ([identity § sealed private config](../R1.0-Release/identity.md#sealed-private-config)). The secrets want may be this plus *the service may write it*, rather than a new mechanism |
| **`kv` in the catalogue** | already listed, and its first version is an access wrapper around `kvrocks` ([bundled services § data](../R1.1/services.md#data)) — values with a ttl, `cas`, hashes, lists with blocking forms and pull-push, none of it ours to build. `cas` and pull-push each claim work without a lock, which is most of the state want |
| **redis · kvrocks gateways** | also already listed — and fast shared KV with locks is exactly what they do. If a gateway serves this, *ours* has to justify existing |

### Why it is not decided here

The two wants pull apart. Secrets are **rarely written, never contended, and
must be unreadable**. Batch statuses are **often written, contended, and not
secret at all**. One mechanism for both is the assumption worth testing rather
than the design worth committing to — and the answer likely differs per want,
which is a thing to find out with a real service in front of us.

## Whether the daemon's own parts become services

The earlier target placed dashboard, health and stats in **supervisor children**.
Only the dashboard exists as a child today; the other roles are proposed.
That target uses passed descriptors and declared privileges rather than service names
([processes](../../docs/11-processes.md#processes-and-privileges)). The question is whether they stop being
that and become entries in the catalogue like everything else, leaving
`agent-busd` as the bus and nothing more.

| For | Against |
|---|---|
| each gets a **name and an ACL**, so who may see the dashboard is granted the way everything else is granted, instead of being a second mechanism | **what you open when the bus is sick must not be something the bus delivers.** A dashboard that needs a working bus to tell you the bus is broken is no dashboard |
| each can then run on **another host**, which a passed fd cannot ([runner § one name on many hosts](../R1.0-Release/runner.md#one-name-on-many-hosts)) | a child that never had a token cannot leak one, and this hands three of them a credential |
| the daemon shrinks, which is the direction [modules](../../docs/10-modules.md#layers-and-modules) already points | health and stats are *about the daemon*, and a service asking the daemon about itself is a round trip to answer what was already in memory |

**It is the opposite question to the one R1.1 asks.** That stage's
acceptance criterion is *no entry in the catalogue needed a change to
`agent-busd`* ([stages § R1.1](../R1.1/README.md#scope)) — a test of
whether the design carries work **inward**. This asks what should move
**outward**, and the honest order is to finish the first before answering the
second: a catalogue that is real is the evidence, and the same argument then
either holds for the daemon's own parts or visibly does not.



Unresolved details: [questions](QUESTIONS.md#open-questions).
