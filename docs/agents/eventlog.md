# Eventlog

📌 **TL;DR:** keep named operational logs with separate notice, warning and
alert rings. Subscribe a queue to new matching entries, or poll entries away
with `pull`; alerts and increasing entry IDs survive an agent restart.

## Why you'd want it

Keep deploy results, CI failures or checks together under a name. Read the
recent entries without asking the service that produced them, and send new
alerts to a queue your team already watches.

## What it does

| Call | Result |
|---|---|
| `write <log> notice\|warning\|alert <text>` | creates a log when needed; stores an entry, then forwards matching subscriptions before answering |
| `read <log> [level]` | live entries newest first, grouped by severity |
| `list` | active log names, live counts and the latest alert time |
| `pull <log> [level] [count]` | removes and returns the oldest entries; defaults to 100 |
| `clean <log> [level] [id]` | removes entries up to the inclusive id, or all entries when no id is given |
| `sub add <target> [log] [level] [/regex/]` | sends new matching entries once to an agent or queue |
| `sub list`, `sub delete <id>` | inspects or deletes subscriptions |

An unbounded all-level `clean` retires the log and frees its active-log slot.
Writing that name later continues its ID sequence. Partial clean and pull keep
the log active, even when empty. Pull and clean never remove subscriber inbox
copies. Missing logs return a fixed error; only write creates them.

<details><summary>Entry and reply fields</summary>

Each entry has `id`, `level`, `text`, `at`, `last_at` and `count`. Times are UTC
RFC3339 with milliseconds and `Z`. IDs order entries even when the clock moves
backwards. A repeated level/text within a fixed minute from its first entry
folds into that live entry, preserving its ID and increasing count; an accepted
folded occurrence still forwards once. Controls and ANSI sequences are removed
before the text is cut to 200 Unicode characters. Text remains untrusted data.

Write returns `{entry,deliveries:{matched,sent,dropped}}`. Read returns
`{log,rings,counts,latest_alert_at,now}`: rings appear in alert, warning, notice
order, or contain only the requested level; counts cover all levels. Latest
alert time is null when no live alert remains. List returns `{logs,now}`, with
rows `{log,counts,latest_alert_at}` sorted by name. Pull returns
`{log,entries,remaining,now}` in oldest-ID order. Clean returns
`{log,removed,counts}`. Errors are always `{error:{code,text}}`, with fixed
content-free descriptions. Metadata asks retain their standard text/JSON form.

</details>

## You'll need

A running bus and runner, a writable instance work directory, and a grant with
`read` or `write` on this instance. A subscription destination must admit both
the subscription creator and the eventlog instance that sends the copy.

## Try this first

```sh
agent-bus call '#eventlog@node.example' 'write deploys notice deployed 691ae2da'
agent-bus call '#eventlog@node.example' 'read deploys'
```

## Use examples

```sh
agent-bus call '#eventlog@node.example' 'sub add ops@node.example deploys alert /failed/i'
agent-bus call '#eventlog@node.example' 'write deploys alert tests failed'
agent-bus call '#eventlog@node.example' 'pull deploys alert 1'
agent-bus call '#eventlog@node.example' 'clean deploys warning 7'
```

JSON uses `method`: `write`, `read`, `list`, `pull`, `clean`, `sub.add`,
`sub.list` or `sub.delete`; arguments use the names in the table. `count` and
an entry `id` are integers; a subscription `id` is a string. For example,
`{"method":"sub.add","target":"ops@node.example","level":"alert"}`
subscribes to alerts across logs. In text, a single selector word always names
a log, even one named `alert`; choose a level alone with JSON.

Requests are strict UTF-8 and at most 16 KiB. Unknown/duplicate fields, NUL,
nonfinite numbers and booleans used as counts are refused. Pull count is
1–500; entry IDs are positive integers. Log names are case-sensitive ASCII
letters/digits followed by letters/digits, `.`, `_` or `-`, up to 64 characters.

## Installing it

Package `src/agents/eventlog/package.sh <destination>` for a runner instance.
The package includes its Python client and shared ring/subscription helpers;
no source checkout is needed. Register it with `agent.json`, grant callers the
appropriate roles, then enable it through your runner inventory. Keep each
instance's work directory separate; `AGENT_BUS_WORK` selects it, otherwise the
runner's working directory is used. State is never written into the package.
The agent is bus-only, kept and sandboxed.

## Configuration

| Key | Meaning |
|---|---|
| `rings` | level → optional `size` and `ttl` duration overrides |
| `logs` | log name → per-level overrides on top of `rings` |
| `max_logs` | active log bound, default 100, range 1–100 |

Default rings:

| Level | Entries | TTL |
|---|---|---|
| notice | 200 | 1 day |
| warning | 200 | 7 days |
| alert | 50 | 30 days |

The combined ring capacity of a log is at most 500. Each level's size is at
least one. TTL is a positive duration with one `s`, `m`, `h` or `d` unit, at
most 366 days. `config.json.dist` demonstrates an unconfigured-safe instance.

Alert snapshots and ID reservations are private files, atomically replaced and
fsynced before success. Notice and warning entries live only in memory. IDs
reserve blocks of 64; restart can skip unused IDs and never reuses them. Full
clean retains a name's tombstone. The lifetime name cap is 10,000: refuse new
names at the cap, retain old names and allow cleanup. Startup refuses corrupt
or oversized state and an overlapping generation. Storage uncertainty poisons
further operations until state is reopened and validated; do not replay an
uncertain write blindly.

## Secret keys

None. The runner supplies this instance's bus credentials.

## Who may use it

Direct Owner/Maintainers and `write` may write, pull, clean and read. `read`
may read/list and subscribe. Bare ACL admission grants no log
data, including status. Forwarded channel roles and ambient `AB_ROLE_*` values
never authorize data access. Subscription behavior, management and invalid stored state follow the
[shared rule](subscriptions.md#the-shared-behaviour).

Role assignment at subscription creation authorizes that stored subscription
until deletion; later application-role edits alone do not remove it. Delivery
and management follow the linked shared rule. Eventlog checks the writer's
remaining call budget before storage: selector candidates require at least
three seconds; no-candidate writes have no artificial floor. Source-KV calls
share the remaining deadline, and every forwarding helper ends before the
write answer.

## Warnings and alerts

| Level | Condition |
|---|---|
| alert | invalid Configuration or persistent ring state; storage mutation failed or has an unknown outcome |
| warning | a subscription copy is dropped, its filter is invalid/slow, or delivery counters could not be stored |

Diagnostics contain fixed codes, never log text, bodies or credentials.
They currently appear on stderr and the runner records them as warnings; the
separate view of agent warnings and alerts is in progress. Log entries supplied
to `write` are the caller's content, not automatic diagnostics.

## Troubleshooting

A reader cannot pull: grant `write` only if destructive access is intended.
An empty log may have expired entries or entries taken by another poller.
Check subscription counts and creator/destination grants for missing copies.
For `storage_locked`, stop the previous generation normally. For
`storage_invalid` or `storage_unknown`, preserve the files for inspection and
reopen validated state; never reset the ID counter or delete files as repair.
