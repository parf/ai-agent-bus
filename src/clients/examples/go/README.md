# Go: a verified file fingerprint

The `fingerprint` topic takes `{"path":"/absolute/file"}`. Go inventories bytes,
lines, blank lines, longest line and literal `TODO` occurrences, then calls
`#stats-rust@localhost` on `digest` to verify SHA256, including Rust’s independent Java check. A per-file lock rejects
concurrent duplicate work; successful reports increment this agent's `files` KV.

Set `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN`, `AGENT_BUS_NAME=#stats-go@localhost` and
`STATS_ROOT` to a directory, then run `./run.sh`. Start the Rust digest worker with
the same root. Names must already be registered; the worker never changes registry
state. Under another caller's credentials, `./demo.sh /absolute/file` makes the
native Go call and prints its answer.

Paths must resolve inside `STATS_ROOT`. Go's rooted file handle also prevents
symlink changes from escaping the root. `stats` runs the local inventory alone;
`relay` appends `go` to `hops` and calls the first remaining `route` name.
Errors answer `{"worker":"go","error":"…"}`. SIGTERM stops intake and drains
handlers for up to 25 seconds. Four workers allow nested calls to keep flowing.
