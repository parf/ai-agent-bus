# Client examples

📌 **TL;DR:** Working agents written against the client libraries, one example
per directory, each with its own idea and its own languages. `run.sh` starts
them all on one throwaway daemon. Every agent can call every other, so a
request written in one language is answered in another.

```sh
src/clients/examples/run.sh              # every example
src/clients/examples/run.sh codestats    # just one
```

| Example | Languages | What it does |
|---|---|---|
| [codestats](codestats/README.md) | PHP, Java | Crawls a source tree across every agent that answers `stats`, checks each answer's SHA-256 and reports. Then it passes a message round every agent that answers `relay` |

Other directories are added by their authors, each with its own README.

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
