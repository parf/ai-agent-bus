# Future questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q1 | Per-method pricing needs the method name in the envelope | owner | [future/billing.md](billing.md#billing-role--future) |
| Q2 | A newcomer with no balance cannot reach `pay` | owner | [future/billing.md](billing.md#billing-role--future) |
| Q3 | Direct talk bypasses billing | owner | [future/billing.md](billing.md#billing-role--future) |
| Q8 | What additional runner data and encrypted/replicated storage belong beyond the selected 0.7 persistence scope | owner | [storage alternatives](storage.md#storage) |
| Q74 | Which release carries directional access, and whether an existing single list becomes the read list, the write list or both when it is introduced | owner | [directional access](acl-direction.md#where-direction-is-needed) |
| Q142 | For lock sets: is the set's membership a stored record field or runtime state; what happens to a holder whose member is removed; the order *any free one* follows; may a plain lock share a member's name | owner, when taken up | [lock sets](lock-sets.md#a-set-of-locks) |
| Q11 | OpenCode (Z.AI) push path | one spike | [runner § adapters](../../docs/06-launchers.md#adapters) |
| Q24 | Whether the daemon publicly exports its people and their keys, unauthenticated and on by default | owner | [public directory](public-directory.md#a-public-directory-of-people-and-their-keys) |
| Q34 | Which proposed storage engine actually supplies each required encryption and replication property | an engine evaluation and owner decision | [context](storage.md#storage) |
| Q193 | The threshold (and any top-k cap) that turns Jev's per-option probabilities into a record's list of classification ids | owner | [RAG](rag.md#rules) |
| Q194 | How the keyword half of hybrid search is computed for the namespace's languages (Russian, English or both): Qdrant's own BM25 or sparse vectors computed by the agent | owner, after a trial on real text | [RAG](rag.md#rules) |
| Q195 | Expected volume per namespace (records, data size), which sets chunk size and whether vectors are quantized | owner | [RAG](rag.md#rules) |
| Q196 | What `search` returns (ids and snippets, or the stored data too) and who calls it (the realty project, AI agents, both) | owner | [RAG](rag.md#rules) |
| Q197 | Whether Jev is reached through `#ai` (extended to OpenRouter's Decisions API) or by the RAG agent with its own OpenRouter key | owner | [RAG](rag.md#rules) |
| Q198 | How a KV write made outside the RAG agent (web, CLI) invalidates its read cache: prompts edited only through the agent's own methods, which invalidate as they write (no daemon change; recommended first); a daemon notice of KV changes to the record's agent (a general feature, any agent could cache); or a revision counter checked per use | owner | [RAG](rag.md#rules) |

## Constitution forwarding

Settled in the [channel contract](../../docs/constitution.md#-channels).

## Public-directory context

❓ **The exception, and its shape.** *Settled by:* owner.

| | |
|---|---|
| **a listing is not the same question as a lookup** | the fleet pattern needs **only** `<keyserver>/<user>`, and the one running today refuses to list at all — per-user answers, no index. Exporting *who exists* is the larger claim of the two and can be decided separately, and differently |
| **what the status code has to say** | the calling script has no error handling: it curls, and hands whatever came back to `sshd`. So an unknown person must be an empty answer and a refusal, never a page — and the daemon being unreachable has to look different from the person having no keys. `authorized_keys` on the box stays as the way back in when it is |
| **what counts as public** | keys and the name are the point. The person's name and avatar are already description anybody may render. **How to reach a person is not** — it is a phone number, and who may read somebody else's is open where it is defined ([identity § how to reach a person](../R1.2/people.md#how-to-reach-a-person)). Email is where GitHub itself hesitates and makes it opt-in |
| **what default-on means for a company bus** | *on* is right for a bus that is a directory; a private one wants it off, and the same daemon is both. So the decision is which way the switch points when nobody touched it, and that is the part the owner has stated: **on** |
| **whose list it is** | a name is `user@realm` and a realm may be a pool ([identity § names](../../docs/01-identity-and-authority.md#names)), so the answer is per realm and every member has to give the same one |

## Billing context

❓ **Per-method pricing** — may a per-call price vary by method? Bodies are
encrypted, so that needs the method name in the envelope. Less urgent than it
looks: a price difference worth charging for is usually a **grant** difference
too, and splitting it into two names prices it with the mechanism that already
exists — generation and embeddings are two gateways for exactly that reason
([bundled agents § for the agents themselves](../R1.2/services.md#for-the-agents-themselves)).
What that does not cover is a price varying *within* one grant.
*Settled by:* owner.

❓ **A newcomer with no balance cannot reach `pay`** — "no balance = call
denied" plus "register, pay, use through the bus" needs the sign-up and payment
agents to be free. *Settled by:* owner.

❓ **Direct talk bypasses billing** — parties that already know an address may
skip the bus ([overview § goal](../../docs/00-overview.md#goal)), and the bus only counts
what routes through it. *Settled by:* owner.

## Storage context

❓ **Which persistent data belongs in a replacement store?** The original question assumed unbuilt database and snapshot choices. [Current storage](../../docs/07-setup.md#storage) is the baseline; selecting a replacement remains the owner’s decision.
