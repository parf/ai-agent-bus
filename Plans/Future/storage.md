# Storage alternatives

SQLite is selected for [0.7 persistence](../../docs/constitution.md#persistence-and-loading).
The encryption, replication and runner-storage proposals below remain future
work; they are not prerequisites for that release.

## Storage

An earlier proposal preferred SQLite (single file, zero ops), with MySQL/PostgreSQL alternatives behind
the `store` port — swapping one for another is a single adapter
([modules](../../docs/10-modules.md#layers-and-modules)).

Built storage is defined in [setup § storage](../../docs/09-setup.md#storage).
The [0.7 plan](../MVP/0.7.0-TODO.md#storage-and-identity) replaces that backend.

**Earlier proposal, superseded for 0.7 by SQLite.** RocksDB was the preferred candidate for a replacement holding the daemon's data *and* the runner's, **encrypted at rest** — one
store instead of a token file, a dump file, a snapshot and a directory of env
files, each protected only by its mode. The catalogue proposes a `kvrocks` wrapper; its RocksDB engine and its server-level features must be distinguished before treating them as the same solution ([bundled services § data](../R1.1/services.md#data)),
Reusing infrastructure is the motivation, not evidence that the library provides the server’s properties.

What it has to not break:

| | |
|---|---|
| **the daemon reaches it directly, never over the bus** | a daemon that fetched its own tokens by calling a service would need a token to read its tokens. It opens the store behind the `store` port, the way it opens a file today ([modules](../../docs/10-modules.md#layers-and-modules)); `kv` is the **bus-facing face of the same engine**, not the path the daemon uses |
| **two accounts stay two secret domains** | the daemon and the runner are separate accounts precisely so neither reads the other's ([the two accounts](../../docs/09-setup.md#the-two-accounts)). One store holding both is only allowed if it is **separate namespaces under separate keys** — otherwise this quietly merges the two things the split exists to keep apart |
| **it must not become a process the daemon has to start** | no process the daemon starts may exec at all ([processes](../../docs/11-processes.md#processes-and-privileges)). So either the engine is **linked in as a library** — RocksDB is one — or it is a **unit and an account of its own**, started by systemd like the daemon is. That fork is the thing to settle, and the library side costs no third account |
| **backup follows the data** | the runner's backup is an encrypted archive of `runner/` ([runner § backing it up](../R1/runner.md#backing-it-up)); env files moving into the store moves that too, and a store is backed up by snapshotting it rather than by tar |

**Replication is a desired property, not a verified property of the selected engine.**
The [engine evaluation](QUESTIONS.md#open-questions) must identify which layer
provides it and how failover works. Required boundaries:

| | |
|---|---|
| **it is a copy of one node, not peer sync** | peers are separate daemons that exchange **registry records through git**, newer wins per entry ([services § registry sync](../R1/registry.md#registry-sync)). This is the same node's data on a second box, for taking over — two different problems that would otherwise both be called replication |
| **it is not AUTH's replicas either** | those are **signed generations**, and a replica is trusted because the signature is, not because it was copied ([AUTH role § bundle](../R1/auth.md#bundle)). Copying cannot produce authority |
| **and locks stay out of it** | they are live state, deliberately not persisted, and a lock that survived onto a standby would be a claim about processes that are not there ([messaging § shared locks](../R1/locks.md#shared-locks)). Replication carries what is durable, which is what makes *durable* worth stating |


Unresolved details: [questions](QUESTIONS.md#open-questions).
