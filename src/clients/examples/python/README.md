# Python file audit

📌 **TL;DR:**
- **Problem:** a TODO audit of a file is more useful when an independent worker confirms it read the same bytes.
- **Outcome:** `#stats-python@localhost` finds TODO lines and asks the PHP worker for file statistics, checking that both saw the same SHA-256.
- **Start:** [run it](#try-this-first) with the other examples on a throwaway bus.

## Why you'd want it

- 🔍 **Findings you can trust.** A mismatch between two languages' reads shows at once.
- 🧮 **A counter for free.** Each processed file increments the agent's `files` key-value entry.

## What you get

| Topic | Request | Answer |
|---|---|---|
| `audit` | JSON object with absolute `path` | Line-numbered findings and PHP statistics |
| `stats` | Same | Byte, line and TODO counts, SHA-256 |
| `relay` | `route` names and `hops` strings | Adds Python, calls the next agent, returns its answer |

## Try this first

Run every example, Python among them:

```sh
src/clients/examples/run.sh
```

```text
== python
{"worker": "python", "path": "…/samples/notes.txt", "stats": {"worker": "php", "path": "…/samples/notes.txt", "bytes": 84, "lines": 5, "todos": 2, "sha256": "2e67bc128207c26f09318749c1bd07b086368e77f265ec6f8b0523db55f66ac2", "served": 24}, "findings": [{"line": 2, "text": "// TODO: first thing"}, {"line": 4, "text": "# TODO second thing"}], "served": 24}
```

- The demo uses the harness notes file (or `DEMO_FILE`), verifies the exact findings and the PHP response, and checks that an outside-root path is refused.

## Technical details

- **Identity:** `run.sh` always selects `#stats-python@localhost` and its token from `EXAMPLES_TOKENS`; `demo.sh` keeps the separate caller credentials. Run it through the shared [harness](../README.md).
- **Paths:** they must resolve beneath `STATS_ROOT`; symlinks escaping it are refused.
- **Serving:** the worker serves two handlers so a nested call can receive its answer. SIGTERM stops intake and drains work.
