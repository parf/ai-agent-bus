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
- **Upstreams are not replicated.** A mounted upstream bus is queried live
  as one of its users, never copied
  ([upstream mount](../R1.0-Release/federation.md#upstream-mount)).

Unresolved details: [questions](../R1.3/QUESTIONS.md#peer-registry), where №Q5 moved on 2026-11-06.
