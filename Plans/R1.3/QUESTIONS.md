# R1.3 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q22 | Whether an agent holds a key of its own, and where shared secrets and a locking KV live | owner, after R1.2 | [exploration](exploration.md#shared-secrets-and-a-kv-with-locks) |
| Q23 | Whether the daemon's own parts — dashboard, health, stats — become bundled agents rather than supervisor children | owner, once the catalogue is real | [components](exploration.md#whether-the-daemons-own-parts-become-agents) |
| Q27 | How two principals are linked as one person, given that both sides have to state it | owner | [bundled agents § people and the world outside](../R1.2/services.md#people-and-the-world-outside) |

## Exploration context

❓ **Both wants above.** *Settled by:* owner, after R1.2.

❓ **Whether they move, and which of them.** *Settled by:* owner, once the
catalogue is real.

## Identity linkage

❓ **How two principals are linked into one person.** Both sides stating it is
the rule; where that statement lives and what checks it is not settled.
*Settled by:* owner, after R1.2.

## Peer registry

<a id="Q5"></a>

- ❓ Q5 Peer sync trusts unsigned records and has no clock authority
  - Context: a peer can push an unsigned record for any name, and "newer wins" compares clocks that are not synchronised ([registry § registry sync](../R1.1/registry.md#registry-sync))
  - Settled by: the owner
  > 💬 👤parf elaborate
  >
  > 💬👾 The peer registry (R1.1) would let two of your nodes share records, so an agent registered on one is known on the other. Two holes today: (1) a node accepts any record a peer sends, unsigned, so a compromised or misconfigured peer can publish a record under any name, yours included; (2) when both nodes changed a record, "newer wins" compares the two machines' clocks, which drift, so the wrong version can win silently. It only matters once you run more than one node.
  - ( ) Defer the peer registry with the auth work moved to R2.0 (T-37): signing records needs that work's node keys 🌟
  - ( ) Peers are only nodes you own and trust: accept their records unsigned, document the risk, use a per-record version number instead of clocks ⭐
  - ( ) Build signed records now: each node signs what it publishes and a node accepts a record only from its home node
  > 💬 move to R1.3

<a id="Q367"></a>

- ❓ Q367 Does the whole peer registry move to R1.3 with its trust question (№Q5)
  - Context: №Q5 moved here on 2026-11-06 (owner). The registry plan ([registry sync](../R1.1/registry.md#registry-sync)) and its row in the R1.1 [dependencies](../R1.1/TODO.md#dependencies) still sit in R1.1 and wait on №Q5 for record authenticity and clock authority, so the plan and its blocking question are in two stages
  - Settled by: the owner
  - ( ) Move the whole peer registry to R1.3: `registry.md`, its TODO row and its scope row follow №Q5, and R1.1 keeps none of it 🌟
  - ( ) Only №Q5 moves: the registry plan stays in R1.1 as work blocked on a question in R1.3 ⭐
  > 💡👾 Move it all: the registry cannot be built until №Q5 is answered, and R1.1 has no open scope until the owner opens it ([№Q136](../R1.1/QUESTIONS.md#scope)), so one stage should hold the plan and its question.
