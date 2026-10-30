# Eventlog agent

📌 **TL;DR:** `#eventlog` keeps **named logs**, each with the same three rings
as [agent events](README.md#levels): notice, warning and alert. Scripts, CI,
agents and sessions write one line to a log by name; Owner, Maintainers and
callers with the read role read it; the web shows logs with alerts loud. Same
model and the same web view as agent events, for anything (owner, 2026-10-29:
"the same code, for something else"). Tasks: [B.28](TODO.md#b28-eventlog-agent).

## What it does

| Call | Does |
|---|---|
| `write <log> notice\|warning\|alert <text>` | adds one line; a log is created by its first write |
| `read <log> [level]` | the log's entries, newest first, alerts first when no level is named |
| `pull <log> [level] [count]` | takes the oldest `count` entries (default 100) and **removes** them from the log, like a queue; the rest wait for the next `pull`. Owner, Maintainers and the `write` role (owner, 2026-10-29: "pull should remove items") |
| `clean <log> [level]` | empties the log, or one level of it; Owner, Maintainers and the `write` role (owner, 2026-10-29) |
| `list` | logs with their counts per level and the time of the newest alert |
| `sub add <target> [log] [level] [/regex/]`, `sub list`, `sub delete <id>` | the usual subscription, as [`#imap`](../../../docs/agents/imap.md#what-it-does) and [`#logwatch`](../../../docs/agents/logwatch.md#what-it-does): each new matching entry is sent to a queue or agent; no log or level means all; starts from now (owner, 2026-10-29) |

Example: an autopull success script writes `write realty-prod notice deployed
691ae2da`; when its tests fail, `write realty-prod alert tests failed`.

## Rules

| | |
|---|---|
| Rings | per log, one per level, fixed size and TTL; defaults as [agent events](README.md#levels), set in the agent's Configuration, optionally per log |
| Ids | every entry gets the next number of its log, one sequence across the three levels, never reused, not reset by `clean` or a restart; a gap between ids a reader saw is what expired, was pushed out or was pulled by someone else |
| Lines | one line, cut at 200 characters; a line repeated within a minute is kept once with a count |
| Kept | alerts on disk in the agent's work directory, surviving a restart; notices and warnings in memory |
| Subscriptions | the author must be allowed to send to the target, checked at `sub add` and before each send, as every `sub`; a subscriber gets the entry with its log, level and id; `pull` and `clean` never take entries from subscribers that already got them |
| Access | the agent's ACL with roles `read` and `write`; Owner and Maintainers do both. Many instances, each with its own Configuration and ACL |
| Names | a log name is a short word (letters, digits, `.`, `_`, `-`); the number of logs per instance is bounded in Configuration |
| Web | a page per instance: logs with an alert mark, and a log's rings shown like an agent's events, alerts loud, warnings quieter, notices folded |

## Same code

The ring rules (size, TTL, repeat folding, levels) are written once as a shared
module that RT34 and this agent both follow, and the web uses one component for
an agent's events and for a log. v1 is a runner agent in Python with no daemon
change.
