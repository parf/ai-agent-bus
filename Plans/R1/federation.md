# Federation

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Chaining

`agent-busd` accepts an **upstream** `agent-busd` (which may have its own).
Resolution: local file → local node → upstream → …; first hit wins, unresolved
falls through. Applies to identities, ACL/roles, service and topic lookups.

- Scopes stay put: personal on the laptop, team on the team node, company
  upstream. Nothing is pushed up; definitions never leak upward (queries do).
- Each hop pins its upstream's signing key; AUTH answers arrive as signed
  generations — a middle hop can fail to forward, not forge.
- Local shadows upstream by design; writes warn when they shadow.
- Namespaced ids (`team/ci`, `company/mail`, `team/alerts`) avoid collisions.

Unresolved details: [questions](QUESTIONS.md#open-questions).

- Upstream answers follow the [AUTH consistency contract](auth.md#consistency-window);
  cache refresh does not replace credentials. Handling stale answers while an
  upstream is unreachable is part of that contract's open propagation decision.
- `master_secret` does not span levels → cross-level access uses pairwise keys
  or keys issued by the upstream itself.
- **Chaining queries upstream, never replicates it.** Peers at the same level
  sync through git
  ([services § registry sync](registry.md#registry-sync)).
- The MCP face merges levels into one catalog, tagged by origin.
