# Logwatch: recent server logs, on request

📌 **TL;DR:**
- **Problem:** when something breaks, the clues are in log files on a server, and reading them means logging in, knowing where each file lives and scrolling through noise.
- **Outcome:** you or your AI ask "what went wrong in the last five minutes?" over the bus and get the matching lines back, or have new errors sent to one inbox.
- **Start:** ask it what it can do: `agent-bus help '#logwatch@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **Troubleshooting needs a login on the server.** Now you ask the helper; nobody logs in.
- 🔍 **The clue is one line in thousands.** Now you ask for lines containing "error", or for the last five minutes.
- 🧪 **After a deploy you cannot tell what it changed.** Now you set a mark, restart, and read only what came after.
- 🔔 **You must watch a screen to catch errors.** Now new errors arrive in an inbox, repeats counted.
- 🛡️ **Logs hold private data.** Now only the people you allow can ask, and it only reads.

## What it does

- ✅ **Recent lines on request:** the latest lines, or only those matching a word or pattern.
- ✅ **"What happened since…":** lines from the last minutes or hours.
- ✅ **Before and after:** mark a point, change something, then see only what the change logged.
- ✅ **Alerts:** new matching lines go to an inbox; the first is sent at once and repeats are counted in one summary.
- ✅ **Honest memory:** it keeps the latest lines in memory; after a restart or a full history it says so rather than show an incomplete story.
- ✅ **Read only:** it never changes a program or its logs.
- 🚫 **Not for:** long-term storage or search over weeks, or editing and rotating logs.

The calls, last:

| Call | Does |
|---|---|
| `tail` | Latest lines, oldest first |
| `grep` | Latest lines matching a word or pattern |
| `since` | Lines received in the last minutes or hours |
| `mark`, `after` | Remember a point; show what came after it |
| `sources`, `stats` | What it watches; how full its memory is |
| `sub add`, `sub list`, `sub delete` | Send new matching lines to an inbox |

- Subscriptions survive restarts; a failed alert is reported by `sub list` and never resent.
- How its subscriptions differ from the shared rules is in [what each helper does today](subscriptions.md#what-each-helper-does-today).

## You'll need

- The bus and Python 3 on the server, and an administrator to install it.
- A Linux account allowed to read the chosen logs.
- A place on its access list.

## Try this first

- Ask it what it can do (read-only):

```sh
agent-bus help '#logwatch@node.example'
```

- See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

<details>
<summary>See it on a real node (ran today)</summary>

```sh
agent-bus call '#logwatch@parf.us' --wait 15s stats | jq -r .body | jq 'del(.version)'
```

```text
{
  "uptime": 23011.278787851334,
  "ring_size": 2000,
  "fill": 2000,
  "ring_bytes": 363406,
  "ring_byte_capacity": 8388608,
  "lines_seen": 124881,
  "dropped": 122881,
  "forward_dropped": 0
}
```

- `fill` against `ring_size` says how full the memory is; `dropped` counts lines pushed out by newer ones, so a long answer can be older than you think.
- The version is left out of the output here. The output is from the development node on 2026-10-30.

</details>

## Use examples

| I want to… | Command |
|---|---|
| See the latest lines | `agent-bus call '#logwatch@node.example' 'tail 5'` |
| Find recent errors | `agent-bus call '#logwatch@node.example' 'grep /error/i 20'` |
| See the last five minutes | `agent-bus call '#logwatch@node.example' 'since 5m'` |
| Mark before a repair | `agent-bus call '#logwatch@node.example' 'mark before'` |
| See what the repair logged | `agent-bus call '#logwatch@node.example' 'after before'` |
| Get new errors in my inbox | `agent-bus call '#logwatch@node.example' 'sub add #claude/oncall@node.example ERROR'` |
| Stop that | `agent-bus call '#logwatch@node.example' 'sub delete <id from sub list>'` |
| Check what is watched | `agent-bus call '#logwatch@node.example' sources` |

<details>
<summary>More use cases and rules for repairs</summary>

| Use case | What you do | What you get |
|---|---|---|
| Site shows errors | Ask for recent lines containing "error" | The matching lines, with their file or service |
| A deploy just failed | Ask for the last five minutes | Everything logged since |
| Check a repair | Set a mark, restart, ask what came after | Only the lines the restart produced |
| On-call alerts | Subscribe an AI session to `ERROR` | New errors arrive in its inbox, repeats counted |
| A log seems ignored | Ask which sources it watches | Watched, skipped and unreadable files |

- The repair itself is another helper's call, such as a [systemd](systemd.md#who-may-use-it) restart, which needs its own permission.
- Give concurrent repairs different mark names.
- `{"method":"since","age":"5m","n":50}` returns the same lines as structured JSON.

</details>

## Installing it

- An administrator installs it once as a Linux service on the server, chooses the account that reads the logs, and starts with owner-only access.

<details>
<summary>Step by step</summary>

From the checkout root, with [installation credentials](README.md#standalone-helpers):

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  ./src/agents/standalone/logwatch/install.py install \
  --name '#logwatch@node.example' --user root --allow '@owner' \
  --bus-binary /usr/local/bin/agent-bus
agent-bus call '#logwatch@node.example' sources
unset AGENT_BUS_TOKEN
```

- This installs `agent-bus-logwatch.service`, which keeps running with its own bus credential; your token is used only to install.
- `sources` should list your logs; `tail 5` says `(no lines)` if they are empty.
- Narrower account: omit `--user root` to create `ab-logwatch`, give it read access to the files, and for the journal add it to `systemd-journal` (or `adm`) with `sudo usermod -aG systemd-journal ab-logwatch`, then `sudo systemctl restart agent-bus-logwatch`.
- Group membership does not make every application log readable; `sources` shows permission errors.
- Remove: same installer, `remove --name '#logwatch@node.example'`, with current credentials. Other log readers such as `rd-tail-log` are left alone.

</details>

## Configuration

- Which files and services to watch and how much to remember.
- No passwords or tokens belong here; restart the service after a change.

<details>
<summary>Keys</summary>

| Key | Meaning |
|---|---|
| `sources` | File patterns (`**` for subfolders) and/or `syslog`. Default `["/rd/var/log/**/*.log", "syslog", "/var/log/nginx/*error*log"]` |
| `excludes` | File patterns to skip. Default: the legacy tail-all exclusion list (AI query logs, one nginx error log), shown by `--help-config` |
| `journal` | Exact service names, or `"all"`. Default `[]`. `"all"` reads what the account may read; it grants no private journals |
| `ring_size` | Lines kept, default `2000` (1–100000) |
| `line_bytes` | Bytes per line including the truncation marker, default `16384` |
| `ring_bytes` | Bytes of history, default `8388608` |
| `startup_bytes` | Bytes read from each file at start, default `65536` |
| `rescan_interval` | Seconds between searches for new files, default `600` |

Bounds: `agent-bus help '#logwatch@node.example' config`. Example, watching only the web journal:

```sh
printf '%s\n' '{"sources":[],"excludes":[],"journal":["agent-bus-web.service"]}' \
  | agent-bus config '#logwatch@node.example' -
sudo systemctl restart agent-bus-logwatch
```

</details>

## Secret keys

None.

## Who may use it

- Only people on its access list; start at `@owner` and add sessions by name (`agent-bus manage '#logwatch@node.example' --add-allow '#claude/oncall@node.example'`).
- Changing a subscription also needs the right to manage its destination.
- The logwatch Owner and Maintainers may change subscriptions calling it directly.

## Warnings and alerts

- `#logwatch` has its own Linux service, so the [runner entries](README.md#warnings-and-alerts) do not apply; it reports these itself.

| Level | When | What to do |
|---|---|---|
| alert | A configured source cannot be read (file permission, or a journal the account may not read) | Give the service account read access; restart after group changes |
| alert | `journalctl` cannot start, or exits | Run `journalctl -u <unit>` as that account and fix what it reports |
| warning | Lines were dropped because a queue was full | Narrow `sources` or the subscription filters |
| warning | A subscription paused: its author may no longer send, or the target is gone | Delete it and add it again |
| warning | A subscription's send failed, or its send check was unavailable | Nothing if it clears; if it repeats, check the destination |

## Troubleshooting

| If you see | Do this |
|---|---|
| `(no lines)` | Check `sources`; wait for a new line in a readable source |
| A permission error | Give the service account read access; restart after group changes |
| A journal source error | Run `journalctl -u <unit>` as that account and fix what it reports |
| A new file missing | Wait for `rescan_interval`; only lines appended after discovery are read |
| Unknown or expired mark | Set a new mark; marks do not survive restart or a full history |
| Refused call | Ask the administrator to check the access list |
| Refused subscription | Check who manages the destination, that the author may send to it, and that it accepts logwatch |
| A subscription shows `paused` | Its author may no longer send to the target, or the target is gone; delete it and add it again |
| Filter error | Try plain text; patterns are `/pattern/i`; long-running ones are refused |

## Technical details

<details>
<summary>Calls and how lines are kept</summary>

- `tail [n]` (1–100000, default 20) never filters. `grep <query> [n]` takes plain case-insensitive text or `/regex/imsu`; long-running patterns are refused.
- `since <age> [n]` takes `30s`, `5m`, `1h`, `1d`. `mark [label]` generates a label when omitted; `after <label>` refuses an expired mark. `sub add <target> [filter]`, `sub delete <id>`.
- "Received" means when logwatch read the line, not a date in its text. Each line names its source, such as `[/var/log/syslog]` or `[journal:agent-bus-web.service]`.
- At start each nonempty file contributes its last ten complete lines from the final `startup_bytes`; a newly found file starts at its end, so old content is never sent as a new alert. Empty files wait for content. New files are found every `rescan_interval`.
- An oversized line keeps its head and ends with `…[truncated N bytes]`, counted in `line_bytes`. A poll reads at most 64 KiB per file.
- History stops at `ring_size` lines or `ring_bytes`, oldest evicted first. An oversized journal record keeps its raw head under `journal:unknown`.
- Colour codes are removed and control characters become `...`, using the tail-log cleaner.
- Alerts: the first occurrence is sent at once; identical lines from one source are counted, and a summary such as `×37 in 5 min` follows when the five-minute window closes. A queue or channel can also be a destination, under the same access checks.
- Implementation notes: [source README](../../src/agents/standalone/logwatch/README.md).

</details>
