# Client examples

📌 **TL;DR:**
- **Problem:** to trust a client library you want to see agents in several languages work together, not read an API.
- **Outcome:** working agents in all six client languages (Python, TypeScript, Go, Rust, Java and PHP), one example per directory; a request written in one language is answered in another.
- **Start:** run [one example](#try-this-first) on a throwaway bus.

## Why you'd want it

- 👀 **See it work.** Every agent can call every other, across languages.
- 🧹 **Isolated records.** By default the daemon, its agents and their records are gone when the run ends.
- 📚 **Patterns to copy.** Locks, key-value, calls between agents and a relay ring, each in one small directory.

## What you get

| Example | Language | Agent | What it does | Calls |
|---|---|---|---|---|
| [codestats](codestats/README.md) | PHP, Java | `#stats-php`, `#stats-java` | A crawler that spreads a source tree across every agent answering `stats`, checks each answer's SHA-256 and reports. It then sends one message round every agent answering `relay` | every `[stats]` and `[relay]` agent |
| [python](python/README.md) | Python | `#stats-python` | Audits a file's TODO lines, cross-checked by PHP's statistics | PHP |
| [ts](ts/README.md) | TypeScript | `#stats-ts` | Validates a JSON file, then adds Python's TODO audit | Python, which calls PHP |
| [go](go/README.md) | Go | `#stats-go` | Fingerprints a file (lines, blank lines, longest line, TODOs). A per-file lock refuses duplicate work | Rust, for the digest |
| [rust](rust/README.md) | Rust | `#stats-rust` | Computes a digest and caches Java's cross-check in KV under a record lock | Java, to verify it |

- Every agent also answers `stats` and `relay`, so all six languages take part in the crawl and in the relay ring.
- One request can cross several languages: TypeScript, Python, PHP; or Go, Rust, Java.
- Agent names are `@localhost`.

## Try this first

These are contributor fixtures run from a checkout. They need Bash, curl,
OpenSSL, `setsid`, Python 3.11+, PHP 8.1+ with curl/json/mbstring, and a Java 21
JDK. The full set also needs Go 1.24+, a Rust edition-2024 toolchain and Bun
1.3+. Java, Cargo and Bun dependencies may be downloaded and cached.

The fixture expects built `src/agent-busd` and `src/agent-bus` (use the
[contributor build](../../README.md#build-and-check)), or `CLIENT_BIN` pointing
to a directory containing those release binaries. Put that same directory on
`PATH`, because the demo scripts invoke `agent-bus` by name.

Run one example on a throwaway daemon:

```sh
src/clients/examples/run.sh codestats
```

- Run them all with `src/clients/examples/run.sh`.
- In default isolated mode the agents never appear on the installed bus. The fixture removes its records and processes; dependency downloads and build caches remain.

## Technical details

- `--live` registers the agents on the node itself, as the account running it, through its own socket, so none of the account's credentials is minted or rotated: `src/clients/examples/run.sh --live`.
- It runs the demos, then keeps the workers serving until Ctrl-C. Their records stay afterwards; `--live --clean` unregisters them on the way out.

### What an example provides, and what the harness gives it

| File | Holds |
|---|---|
| `agents.txt` | one agent per line: its name, then a description. `[stats]` or `[relay]` in the description says it answers that topic (see [codestats](codestats/README.md#protocol)). A line starting `//` is a comment |
| `run.sh` | starts the agents, in the foreground until SIGTERM |
| `demo.sh` | optional: drives the example and prints the result |

- The harness registers every listed agent, open to every caller, and mints each one a token.
- It starts each `run.sh` and waits until every agent is reading (`READY_SECONDS`, default 90), failing at once if a `run.sh` exits.
- It starts each `demo.sh` with `#demo@localhost` credentials; the Rust digest demo explicitly selects the fixture Owner. Default runs then stop the workers.
- Each script gets these variables:

| Variable | Is |
|---|---|
| `AGENT_BUS_ADDR` | the daemon's socket |
| `EXAMPLES_TOKENS`, `tokens.sh NAME` | the minted tokens |
| `EXAMPLES_ROOT` | the checkout |
| `STATS_ROOT` | where workers may read files: the checkout |
| `SAMPLES`, `DEMO_FILE` | a directory of sample files, and its `sample.json` |
| `EXAMPLES_OWNER`, `EXAMPLES_OWNER_TOKEN` | the daemon's owner, used for setup and the Rust digest demo |

- In default mode the daemon is the client tests' fixture: its own socket, port and database under `tmp/`. `--live` uses and modifies the installed bus as described above.
