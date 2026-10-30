# codestats

📌 **TL;DR:**
- **Problem:** to see a client library work, you want a job that needs many agents: one crawler, several workers, in different languages.
- **Outcome:** a crawler spreads a whole source tree across every worker on the bus, checks every answer's SHA-256 and reports.
- **Start:** [run it](#try-this-first) on a throwaway bus.

## Why you'd want it

- 🌍 **Many languages, one job.** Workers in PHP and Java answer "what is in this file"; a Java crawler fans the work out.
- 🔎 **Wrong answers show.** The crawler recomputes every file's SHA-256 itself.

## What you get

- ✅ **Calls and a bounded `serve`**, per-record key-value counters, failover and agent-to-agent calls in one small example.
- ✅ **Any agent can join** by speaking the [protocol](#protocol) and saying `[stats]` or `[relay]` in its description.

| Agent | Language | Serves |
|---|---|---|
| `#stats-php@localhost` | PHP | `stats`, `relay`; one request at a time, as PHP is |
| `#stats-java@localhost` | Java | `stats`, `relay`; four at a time on virtual threads |
| the crawler (`demo.sh`) | Java | calls every `[stats]` agent, sixteen calls in flight |

## Try this first

Crawl the checkout on a throwaway daemon:

```sh
src/clients/examples/run.sh codestats
```

```text
every answer's SHA-256 checked here: all match

relay round 2 agents in 8 ms: {"route":[],"hops":["java","php"]}

saved as last_report on #demo@localhost (run 1)
```

- Crawl another tree: `CRAWL_DIR=$PWD/src MAX_FILES=3000 src/clients/examples/run.sh codestats`.

## What the crawler does

- **Discovery:** it finds its workers by asking the bus for live agents whose description says `[stats]`; any agent in any example that speaks the protocol joins the crawl.
- **Checking:** it recomputes every file's SHA-256 itself, so a wrong answer in any language shows.
- **Failover:** a file one worker cannot do is tried once on another.
- **Counters:** each worker counts what it served in its own record's key-value store, and the report shows that count.
- **Relay:** it sends one message round every `[relay]` agent; each adds itself and calls the next from inside its handler, and the answer comes back down the chain.
- **Report:** saved as `last_report` in the caller's own record.

## Technical details

### Protocol

| Topic | Request | Answer |
|---|---|---|
| `stats` | `{"path": "/abs/file"}`, which must be under `STATS_ROOT` | `{"worker", "path", "bytes", "lines", "todos", "sha256", "served"}`, or `{"worker", "error"}` |
| `relay` | `{"route": [names…], "hops": [langs…]}` | the same, with the agent's language added to `hops`. With names left in `route`, the agent calls the first of them and answers with what came back |

- `lines` counts newlines, plus one for a last line without one.
- `todos` counts the text `TODO`.
- A worker reads nothing outside `STATS_ROOT`, whatever a request names.
