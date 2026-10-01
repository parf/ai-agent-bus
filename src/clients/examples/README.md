# Client examples

📌 **TL;DR:** Working agents in all six client languages (Python, TypeScript,
Go, Rust, Java and PHP), one example per directory, each with its own idea. `run.sh` starts
them all on one throwaway daemon. Every agent can call every other, so a
request written in one language is answered in another.

```sh
src/clients/examples/run.sh              # every example, on a throwaway daemon
src/clients/examples/run.sh codestats    # just one
src/clients/examples/run.sh --live       # on this node's daemon, serving until Ctrl-C
```

By default nothing reaches the node: the daemon, its agents and their records
are gone when the run ends, so they never show in `agent-bus ls --all` or the
web face. `--live` registers the agents on the node itself, as the account
running it, through its own socket, so none of the account's credentials is
minted or rotated. It runs the demos, then keeps the workers serving until
Ctrl-C. Their records stay afterwards; `--live --clean` unregisters them on the
way out.

| Example | Language | Agent | What it does | Calls |
|---|---|---|---|---|
| [codestats](codestats/README.md) | PHP, Java | `#stats-php`, `#stats-java` | A crawler that spreads a source tree across every agent answering `stats`, checks each answer's SHA-256 and reports. It then sends one message round every agent answering `relay` | every `[stats]` and `[relay]` agent |
| [python](python/README.md) | Python | `#stats-python` | Audits a file's TODO lines, cross-checked by PHP's statistics | PHP |
| [ts](ts/README.md) | TypeScript | `#stats-ts` | Validates a JSON file, then adds Python's TODO audit | Python, which calls PHP |
| [go](go/README.md) | Go | `#stats-go` | Fingerprints a file (lines, blank lines, longest line, TODOs). A per-file lock refuses duplicate work | Rust, for the digest |
| [rust](rust/README.md) | Rust | `#stats-rust` | Computes a content digest and caches it in KV under a record lock | Java, to verify it |

Every agent also answers `stats` and `relay`, so all six languages take part
in the crawl and in the relay ring. One request can cross several languages:
TypeScript, Python, PHP; or Go, Rust, Java. Agent names are `@localhost`.

<details>
<summary>What an example provides, and what the harness gives it</summary>

| File | Holds |
|---|---|
| `agents.txt` | one agent per line: its name, then a description. `[stats]` or `[relay]` in the description says it answers that topic (see [codestats](codestats/README.md#protocol)). A line starting `//` is a comment |
| `run.sh` | starts the agents, in the foreground until SIGTERM |
| `demo.sh` | optional: drives the example and prints the result |

The harness registers every listed agent, open to every caller, and mints each
one a token. It starts each `run.sh` and waits until every agent is reading
(`READY_SECONDS`, default 90), failing at once if a `run.sh` exits. Then it runs
each `demo.sh` as `#demo@localhost` and stops everything. Each script gets:

| Variable | Is |
|---|---|
| `AGENT_BUS_ADDR` | the daemon's socket |
| `EXAMPLES_TOKENS`, `tokens.sh NAME` | the minted tokens |
| `EXAMPLES_ROOT` | the checkout |
| `STATS_ROOT` | where workers may read files: the checkout |
| `SAMPLES`, `DEMO_FILE` | a directory of sample files, and its `sample.json` |
| `EXAMPLES_OWNER`, `EXAMPLES_OWNER_TOKEN` | the daemon's owner, for setup only |

The daemon is the client tests' fixture: its own socket, port and database
under `tmp/`, never the node's.

</details>
