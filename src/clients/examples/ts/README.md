# TypeScript JSON validator

`#stats-ts@localhost` validates a JSON file, reports its top-level type and object
keys, then calls Python's `#stats-python@localhost` on topic `audit` for TODO
findings and the PHP file statistics. It combines local validation and a peer
review; an unavailable auditor is reported separately from JSON validity.

Run `run.sh` with `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN`, `AGENT_BUS_NAME` and
`STATS_ROOT` after registering the name in `agents.txt`. Bun runs the native
TypeScript client. A second caller runs `demo.sh /absolute/path/file.json` with
its own bus credentials. The shared examples harness supplies both principals.

`validate-json` takes `{"path":"/absolute/path/file.json"}`. The real file must
be inside `STATS_ROOT`; opened-descriptor checks prevent symlink replacement
from reading outside it. The response contains `worker`, `path`, `valid`,
`type`/`keys` or `error`, and `audit`/`audit_error`. Each inspected file increments
the worker's own `files` KV integer. Invalid requests return JSON errors.

`stats` provides bytes, newline-based lines, literal TODO count and SHA256 for
interop checks. `relay` takes `route` and `hops` arrays, appends `ts`, and calls
the next peer until the route is empty. Two handlers allow peer calls while
serving. SIGTERM stops intake and drains active work for up to 25 seconds.
