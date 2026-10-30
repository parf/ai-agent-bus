# Eventlog: keep named logs your team can read

<a id="eventlog"></a>

📌 **TL;DR:**
- **Problem:** deploy results, CI failures and check outcomes are scattered across tools, and the only way to read them is to ask the service that produced them.
- **Outcome:** write each one to a named log with a severity; read the recent entries, poll them away, or send new alerts to a queue your team already watches.
- **Start:** write one entry and read it back: `agent-bus call '#eventlog@node.example' 'write deploys notice deployed 691ae2da'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🗂️ **Results live in many places.** Now each log has a name, such as `deploys`, and keeps its own entries.
- 🔎 **You must ask the producer to see its history.** Now you read the recent entries from the log itself.
- 🔔 **Nobody sees a failure until they look.** Now new alerts go to a queue or agent your team watches, once each.
- 🧹 **Old noise piles up.** Now each severity has its own size and lifetime, and entries expire by themselves.
- 🔁 **A restart loses the alerts.** Now alerts and entry numbers survive an agent restart.

## What it does

- ✅ **Named logs with three severities:** notice, warning and alert, each kept in its own ring with its own size and lifetime.
- ✅ **Write and read:** an entry is stored with a number and a time; reading shows the newest first, alerts before warnings before notices.
- ✅ **Alerts to a queue:** a subscription sends new matching entries once to an agent or queue, before the write is answered.
- ✅ **Poll entries away:** `pull` removes and returns the oldest entries, so one reader can work through a log.
- ✅ **Repeats fold:** the same text at the same level within a minute becomes one entry with a count.
- ✅ **Survives restarts:** alerts and entry numbers are kept; a number is never reused.
- ✅ **Control:** readers can read and subscribe; only writers can write, pull and clean.
- 🚫 **Not for:** long-term storage (entries expire), or text you do not control as instructions: an entry is the writer's content and stays untrusted data.

The calls, last:

| Call | Result |
|---|---|
| `write <log> notice\|warning\|alert <text>` | Creates a log when needed; stores an entry, then forwards matching subscriptions before answering |
| `read <log> [level]` | Live entries newest first, grouped by severity |
| `list` | Active log names, live counts and the latest alert time |
| `pull <log> [level] [count]` | Removes and returns the oldest entries; defaults to 100 |
| `clean <log> [level] [id]` | Removes entries up to the inclusive id, or all entries when no id is given |
| `sub add <target> [log] [level] [/regex/]` | Sends new matching entries once to an agent or queue |
| `sub list`, `sub delete <id>` | Inspects or deletes subscriptions |

- An unbounded all-level `clean` retires the log and frees its active-log slot. Writing that name later continues its number sequence.
- Partial clean and pull keep the log active, even when empty. Pull and clean never remove subscribers' inbox copies.
- Missing logs return a fixed error; only `write` creates them.

## You'll need

- A running bus and runner, and a writable work directory for the instance.
- A grant with `read` or `write` on this instance.
- A subscription destination that admits both the subscription's creator and the eventlog instance that sends the copy.

## Try this first

- Write one entry, then read it back:

```sh
agent-bus call '#eventlog@node.example' 'write deploys notice deployed 691ae2da'
agent-bus call '#eventlog@node.example' 'read deploys'
```

<details>
<summary>See it on a real node (ran today)</summary>

Check the instance without writing a log entry:

```sh
agent-bus call '#eventlog@parf.us' --wait 15s --status | jq -r .body
```

```text
eventlog: 0 active logs
```

- Status is read-only; here no logs are active.
- This ran on the development node on 2026-10-30.

</details>

## Use examples

| I want to… | Command |
|---|---|
| Send new failures to my team's queue | `agent-bus call '#eventlog@node.example' 'sub add ops@node.example deploys alert /failed/i'` |
| Record a failure | `agent-bus call '#eventlog@node.example' 'write deploys alert tests failed'` |
| Take the oldest alert away | `agent-bus call '#eventlog@node.example' 'pull deploys alert 1'` |
| Clear old warnings | `agent-bus call '#eventlog@node.example' 'clean deploys warning 7'` |
| See which logs exist | `agent-bus call '#eventlog@node.example' list` |

<details>
<summary>JSON forms and limits</summary>

- JSON uses `method`: `write`, `read`, `list`, `pull`, `clean`, `sub.add`, `sub.list` or `sub.delete`; arguments use the names in the table.
- `count` and an entry `id` are integers; a subscription `id` is a string.
- Example: `{"method":"sub.add","target":"ops@node.example","level":"alert"}` subscribes to alerts across logs.
- In text, a single selector word always names a log, even one named `alert`; choose a level alone with JSON.
- Requests are strict UTF-8 and at most 16 KiB. Unknown or duplicate fields, NUL, nonfinite numbers and booleans used as counts are refused.
- Pull count is 1–500; entry ids are positive integers.
- Log names are case-sensitive ASCII letters or digits followed by letters, digits, `.`, `_` or `-`, up to 64 characters.

</details>

## Installing it

- Package `src/agents/eventlog/package.sh <destination>` for a runner instance; the package includes its Python client and shared helpers, so no source checkout is needed.
- Eventlog [serves itself](../runner/configuration.md#agents-that-serve-themselves), so the runner never registers it. The owner registers it, grants callers their roles, and hands the runner its token once; without the token the runner refuses to enable it.
- Then enable it through your runner inventory and reload the runner.
- Keep each instance's work directory separate: `AGENT_BUS_WORK` selects it, otherwise the runner's working directory is used. State is never written into the package.
- The agent is bus-only, kept and sandboxed.
- On termination it finishes the current bounded bus operation, closes its resources and exits cleanly.

## Configuration

- An absent or null Configuration uses the defaults below.
- `config.json.dist` shows an unconfigured instance.

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

- The combined ring capacity of a log is at most 500. Each level's size is at least one.
- TTL is a positive duration with one `s`, `m`, `h` or `d` unit, at most 366 days.

## Secret keys

None. The runner supplies this instance's bus credentials.

## Who may use it

- Direct Owner and Maintainers, and `write`, may write, pull, clean and read.
- `read` may read, list and subscribe.
- Bare access-list admission grants no log data, including status. Forwarded channel roles and ambient `AB_ROLE_*` values never authorize data access.
- Subscription behavior, management and invalid stored state follow the [shared rule](subscriptions.md#the-shared-behaviour).
- Role assignment at subscription creation authorizes that stored subscription until deletion; later application-role edits alone do not remove it.

## Warnings and alerts

| Level | Condition |
|---|---|
| alert | Invalid Configuration or persistent ring state; a storage change failed or has an unknown outcome |
| warning | A subscription copy is dropped, its filter is invalid or slow, or delivery counters could not be stored |

- Diagnostics contain fixed codes, never log text, bodies or credentials.
- They appear on stderr and the runner records them as warnings.
- Log entries supplied to `write` are the caller's content, not automatic diagnostics.

## Troubleshooting

| If you see | Do this |
|---|---|
| A reader cannot pull | Grant `write` only if destructive access is intended |
| An empty log | Entries may have expired, or another poller took them |
| Missing subscription copies | Check subscription counts and the creator's and destination's grants |
| `storage_locked` | Stop the previous generation normally |
| `storage_invalid` or `storage_unknown` | Preserve the files for inspection and reopen validated state; never reset the id counter or delete files as repair |

## Technical details

<details>
<summary>Entries and replies</summary>

- Each entry has `id`, `level`, `text`, `at`, `last_at` and `count`. Times are UTC RFC3339 with milliseconds and `Z`. Ids order entries even when the clock moves backwards.
- A repeated level and text within a fixed minute from its first entry folds into that live entry, preserving its id and increasing count; an accepted folded occurrence still forwards once.
- Controls and ANSI sequences are removed before the text is cut to 200 Unicode characters. Text remains untrusted data.
- `write` returns `{entry,deliveries:{matched,sent,dropped}}`.
- `read` returns `{log,rings,counts,latest_alert_at,now}`: rings appear in alert, warning, notice order, or contain only the requested level; counts cover all levels. Latest alert time is null when no live alert remains.
- `list` returns `{logs,now}`, with rows `{log,counts,latest_alert_at}` sorted by name.
- `pull` returns `{log,entries,remaining,now}` in oldest-id order. `clean` returns `{log,removed,counts}`.
- Errors are always `{error:{code,text}}`, with fixed content-free descriptions. Metadata asks retain their standard text and JSON form.

</details>

<details>
<summary>Persistence and storage</summary>

- Alert snapshots and id reservations are private files, atomically replaced and fsynced before success. Notice and warning entries live only in memory.
- Ids reserve blocks of 64; a restart can skip unused ids and never reuses them. Full clean retains a name's tombstone.
- The lifetime name cap is 10,000: new names are refused at the cap, old names are retained and cleanup is allowed.
- Startup refuses corrupt or oversized state and an overlapping generation.
- Storage uncertainty poisons further operations until state is reopened and validated; do not replay an uncertain write blindly.

</details>

<details>
<summary>Write budget and forwarding</summary>

- Eventlog checks the writer's remaining call budget before storage: selector candidates require at least three seconds; writes with no candidates have no artificial floor.
- Source-KV calls share the remaining deadline, and every forwarding helper ends before the write answer.
- Delivery and management follow the [shared subscription rule](subscriptions.md#the-shared-behaviour).

</details>
