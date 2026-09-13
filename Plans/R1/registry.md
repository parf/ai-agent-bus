# Peer registry

Status: proposed, not built.

## Registry sync

Between **peer nodes at the same level**, the registry syncs through git — no
live replication protocol.

- Each node holds its registry live and writes snapshots to the shared git
  repo. **On start a node pushes its snapshot and pulls from the other known
  nodes.**
- **Newer record wins per entry, provided its writer had access.** A
  key-holding writer's record carries its signature, which a peer checks before
  taking it; a static-token record is trusted on the strength of the node that
  accepted it.
- **Upstreams are not replicated.** An upstream `agent-busd` has its own
  registry and we usually lack full access to it; chaining queries it and
  caches answers, nothing more
  ([overview § chaining](federation.md#chaining)).

Unresolved details: [questions](QUESTIONS.md#open-questions).
