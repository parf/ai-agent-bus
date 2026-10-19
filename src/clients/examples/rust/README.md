# Rust digest worker

Computes a file's SHA256, takes a record-scoped lock and caches the verified digest
in its JSON KV store. On a cache miss it calls `#stats-java@localhost` on `stats`
and requires Java's SHA256 to match. Same-content Rust tasks share a local mutex;
that avoids the daemon's immediate same-principal lock reacquisition refusal.

The shared harness registers and starts this worker and Java together. `run.sh`
reads its credential through `../tokens.sh`, using the harness transport and
`STATS_ROOT`. Build output uses the [shared Cargo target](../../README.md#checks).
`demo.sh [path]` makes a digest call as the harness owner, defaulting to `DEMO_FILE`.

`digest` accepts `{"path":"file"}`. Relative paths resolve under `STATS_ROOT`;
canonical paths must stay inside that root. The JSON answer contains `worker`,
`path`, `bytes`, `sha256`, `cached`, `java_verified` and Java's stats answer.
Changed bytes get a new digest key. Errors return `worker` and `error`.

The worker also supports `stats` and `relay` for the shared example harness.
Stats counts bytes, LF lines plus a final unterminated line, literal TODO markers
and SHA256. Relay pops the next route name, appends `rust` to hops and delegates.
Two handlers run concurrently. SIGTERM stops intake and drains accepted work.
