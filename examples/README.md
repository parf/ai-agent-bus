# Examples

📌 **TL;DR:** Run `kv.sh` against a running daemon to exercise every KV
operation. It creates or reuses an Agent and holds its named lock across the
reset, commands and result comparisons.

## KV

[`kv.sh`](kv.sh) is a runnable Bash example and test of the
[key-value store](../docs/01-identity-and-roles.md#key-value-store).
It creates `#kv-example` if absent, resets its `example/*` test names, issues
commands and compares both their answers and the stored values. A mismatch or
unexpected refusal exits nonzero. A named lock covers reset and every section;
it is renewed before each section and released on normal exit, failure or
termination. The TTL is a fallback if the process is killed without cleanup.

```bash
./examples/kv.sh
./examples/kv.sh '#my-kv-test'
AGENT_BUS_ADDR=/path/to/user-your-account.sock ./examples/kv.sh
```

Requires Bash, `agent-bus`, `od` and `tr`, and a running daemon. Normal CLI
credentials and address discovery apply. Run as the Agent's Owner, Maintainer
or own principal; creating a missing Agent requires registration permission.
`AGENT_BUS_CLI=/path/to/agent-bus` selects another CLI binary.

| Coverage | Checks |
|---|---|
| String, int, JSON | get, set, delete; default, add and replace modes; refused writes preserve the value; deleting an absent name |
| Strings | empty values, stdin, NUL and non-UTF-8 bytes |
| Namespaces | the same name stores independent string, int and JSON values |
| Integers | absent starts at zero; default, positive, negative and zero increments |
| JSON | set, unset, inc, push, unshift, shift, pop, add_to_set, remove_from_set; answers and stored state |
| JSON edge cases | empty and absent keys, duplicate removal, object/number equality, malformed operations, wrong types and complete rollback |

The Agent and final test values remain available to inspect. Reruns reset the
same five names in every kind; other names and records are untouched. An
overlapping run on the same Agent is refused before resetting any values; use different Agents
for simultaneous runs.
