# Storage

## Backends

Owner-assigned to R1, not started: configurable MySQL and PostgreSQL adapters,
with SQLite remaining the default. Reuse the persistence ports and shared
authority, write-through and statistics rules established in 0.7.

Every backend must enforce one active daemon per database, including competing
hosts for server databases. Invalid configuration or an unavailable selected
backend must fail explicitly without fallback or exposing connection secrets.

Implementation and checks: [K.1.2](TODO.md#storage-acceptance), carried from 0.7.
