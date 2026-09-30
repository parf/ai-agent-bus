# R1 decisions

## Recorded decisions

| Decision topic | Substance | Earlier rows |
|---|---|---|
| R1's scope is settled: the release's own extensions, federation, the managed runner, method metadata, client libraries and record-defined roles; distributed identity and encryption moved to [R1.1](../R1.1/README.md#scope) (Q18) | [R1 scope](README.md#scope) | 2026-09-30 owner split; the old R1 was divided into R1 and R1.1, and the tools and exploration stages renumbered to R1.2 and R1.3 |
| Record-defined roles are R1's, and its first topic after 0.7 (Q98) | [roles](roles.md) | 2026-09-22 owner decision; moved out of MVP |
| Locks are held in memory and never stored; a set of locks is one grant over several | [shared locks](locks.md#shared-locks) | 2026-09-22 owner decision; a `sync.Map`, nothing in the database, no dump and no grant numbers, so a restart releases everything | D64, D65, D66 |
| A per-name key-value store assigned to R1 | [key-value store](kv.md#per-name-storage) | 2026-09-22 owner instruction; database-backed with SQLite first, per User and per registry record, with atomic operations |
| MCP Resource and Resource Template become registry record kinds | [Resource records](resources.md#resource-records) | 2026-09-23 owner instruction; the same common fields and the same ACL as every other kind |
| A Resource is an Agent in disguise; the bus switches the read (Q112) | [Resource records](resources.md#resource-records) | 2026-09-23 owner framing; the bus is a connector, so something answers behind the name and the daemon does not become a content store |
| On demand | [definition](runner.md#on-demand) | |
| Message routing | [definition](runner.md#many-names-into-one-inbox) | |
| Additional script forms | [definition](runner.md#additional-script-forms) | D61, D118, D119, D121 |
