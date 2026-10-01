# Python file audit

`#stats-python@localhost` finds TODO lines and asks the PHP worker for independent
file statistics. It checks that both read the same SHA-256 before returning the
findings. Each processed file increments the agent's `files` KV counter.

| Topic | Request | Answer |
|---|---|---|
| `audit` | JSON object with absolute `path` | Line-numbered findings and PHP statistics |
| `stats` | Same | Byte, line and TODO counts, SHA-256 |
| `relay` | `route` names and `hops` strings | Adds Python, calls the next agent, returns its answer |

Paths must resolve beneath `STATS_ROOT`; symlinks escaping it are refused. The
worker serves two handlers so a nested call can receive its answer. SIGTERM stops
intake and drains work. Run it through the [shared harness](../README.md); the demo
uses the harness notes file (or `DEMO_FILE`), verifies exact findings and the
PHP response, and checks that an outside-root path is refused.
