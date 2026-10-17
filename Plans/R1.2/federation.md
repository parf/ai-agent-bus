# Federation

Deferred to R1.2 by the owner on 2026-10-17 (№Q137), not built. The earlier
2026-09-30 hold is not an instruction to start implementation. Prerequisites
remain R1.1's signed AUTH and peer-registry contracts; remaining propagation
choices stay in [R1.1 questions](../R1.1/QUESTIONS.md#open-questions).
The accepted namespace decision is recorded with the full №Q20 thread in
[R1 decisions](../R1.0-Release/DECISIONS.md#recorded-decisions).

## Chaining

`agent-busd` accepts an **upstream** `agent-busd` (which may have its own).
Resolution: local file → local node → upstream → …; first hit wins, unresolved
falls through. Applies to identities, ACL/roles and record lookups.

- Scopes stay put: personal on the laptop, team on the team node, company
  upstream. Nothing is pushed up; definitions never leak upward (queries do).
- Each hop pins its upstream's signing key; AUTH answers arrive as signed
  generations — a middle hop can fail to forward, not forge.
- Local shadows upstream by design; writes warn when they shadow.
- The realm supplies the chaining namespace (`#ci@team`, `#mail@company.upstream`); `/` remains the template/instance separator (№Q20), with no new name grammar.

Unresolved AUTH propagation details stay in [R1.1 questions](../R1.1/QUESTIONS.md#open-questions).

- Upstream answers follow the [AUTH consistency contract](../R1.1/auth.md#consistency-window);
  cache refresh does not replace credentials. Handling stale answers while an
  upstream is unreachable is part of that contract's open propagation decision.
- `master_secret` does not span levels → cross-level access uses pairwise keys
  or keys issued by the upstream itself.
- **Chaining queries upstream, never replicates it.** Peers at the same level
  sync through git
  ([registry § registry sync](../R1.1/registry.md#registry-sync), R1.1).
- The MCP face merges levels into one catalog, tagged by origin.
