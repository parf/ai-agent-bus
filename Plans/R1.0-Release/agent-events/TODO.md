# Agent events TODO

📌 **TL;DR:** what each bundled agent must write into its own
[event rings](README.md#levels): the runner's common events once, then each
agent's own alerts and warnings, as its guide states them. Blocked on the API
shape (see [RT34](../TODO.md#rt34-agent-events)) and on stderr noise: many
agents log every call on stderr today, which would make each call a warning.

## Tasks

### Common runner events

Written by the runner for every agent it runs; [levels](README.md#levels) owns
their meaning. Agents with their own Linux service (git, logwatch, server-info,
systemd, tmux-sessions) and `#bus` get none of these.

| Level | Event | Status |
|---|---|---|
| notice | the runner started or stopped the agent | ⏳ |
| warning | a line on the agent's stderr | ⏳ |
| alert | a non-zero exit nobody asked for, a kill by signal, a memory or process limit, a failed start, a restart loop, a sandbox refusal | ⏳ |

### Per-agent events

Short names; the guide row holds the wording and what to do. A standalone
agent writes its own rows; it has no runner to write the common ones.

| Agent | Alerts | Warnings | Guide | Status |
|---|---|---|---|---|
| ai | key or account refused; provider unreachable | 429/5xx; timeout; no model | [ai](../../../docs/agents/ai.md#warnings-and-alerts) | ⏳ |
| at | invalid `tz` | paused; refused after 24 h; creator check unavailable; deferred; invalid entry set aside | [at](../../../docs/agents/at.md#warnings-and-alerts) | ⏳ |
| bus | conceptual error; store write failed; listener cannot bind; TLS expired; unclean previous stop ([design](README.md#the-buss-own-events)) | dropped copies and ring drops; low disk; TLS expiring; failed-auth burst; clock step | [bus](../../../docs/agents/bus.md#warnings-and-alerts) | ⏳ |
| cron | invalid `tz` | paused; refused after TTL; slot missed; deferred; invalid entry set aside | [cron](../../../docs/agents/cron.md#warnings-and-alerts) | ⏳ |
| db | connection or login refused; invalid Configuration or missing driver | statement timeout; tier without account | [db](../../../docs/agents/db.md#warnings-and-alerts) | ⏳ |
| debug | selftest step failed; `crash` called | `fail` called; doctor could not ask a helper | [debug](../../../docs/agents/debug.md#warnings-and-alerts) | ⏳ |
| fetch | invalid `allow`; helper cannot start | unknown outcome | [fetch](../../../docs/agents/fetch.md#warnings-and-alerts) | ⏳ |
| git | invalid Configuration; path not a repository; autopull failed | autopull left a copy alone | [git](../../../docs/agents/git.md#warnings-and-alerts) | ⏳ |
| imap | sign-in or OAuth refused; server unreachable or TLS failed; forwarding state invalid; Configuration or key missing | poll failed then recovered; subscription paused; skipped at outbox capacity; UID skipped | [imap](../../../docs/agents/imap.md#warnings-and-alerts) | ⏳ |
| logwatch | source unreadable; journalctl failed | lines dropped; subscription paused; send failed or check unavailable | [logwatch](../../../docs/agents/logwatch.md#warnings-and-alerts) | ⏳ |
| qdrant | key missing or refused; store unreachable | plain HTTP to a remote store; unknown write outcome | [qdrant](../../../docs/agents/qdrant.md#warnings-and-alerts) | ⏳ |
| rag | collection missing or mismatched; block refuses grant; Configuration incomplete | collection optimizing; unknown outcome or incomplete delete | [rag](../../../docs/agents/rag.md#warnings-and-alerts) | ⏳ |
| server-info | none (reports, never judges) | tool missing; section failed; certificate unreadable | [server-info](../../../docs/agents/server-info.md#warnings-and-alerts) | ⏳ |
| slack | token missing or rejected; live events keep failing | reconnected; events dropped; routed message failed; subscription disabled or channel unresolved | [slack](../../../docs/agents/slack.md#warnings-and-alerts) | ⏳ |
| sms | key missing or rejected; `from` invalid | unknown outcome; one message refused | [sms](../../../docs/agents/sms.md#warnings-and-alerts) | ⏳ |
| smtp | sign-in or OAuth refused; server unreachable or TLS failed; `sender` or key missing | unknown outcome; partial or non-auth refusal | [smtp](../../../docs/agents/smtp.md#warnings-and-alerts) | ⏳ |
| systemd | permission denied; invalid `units`; systemctl missing or manager silent | systemctl timeout | [systemd](../../../docs/agents/systemd.md#warnings-and-alerts) | ⏳ |
| telegram | token missing or rejected; another receiver (409); malformed Configuration | poll failed then recovered; routed message failed; subscription disabled | [telegram](../../../docs/agents/telegram.md#warnings-and-alerts) | ⏳ |
| tmux-sessions | window exits at start; invalid Configuration; tmux missing | window would not stop; degraded | [tmux-sessions](../../../docs/agents/tmux-sessions.md#warnings-and-alerts) | ⏳ |
| voyage | key missing or rejected; Voyage unreachable | 429/5xx; timeout | [voyage](../../../docs/agents/voyage.md#warnings-and-alerts) | ⏳ |
| webhook | listener closed (Configuration, Secret, publish grant); address taken; target refuses | unknown outcome (503); rate or admission limit; repeated failed authentication; Basic over plain HTTP | [webhook](../../../docs/agents/webhook.md#warnings-and-alerts) | ⏳ |

### Dependencies

| Blocks | What |
|---|---|
| every agent | The API shape for writing an entry (RT34) |
| the stderr warning | ⚠️ ai, db, fetch, qdrant, rag, sms, smtp, voyage, imap and webhook write one status line per call on stderr; cron and `#at` one outcome line per firing; Slack logs each socket connect. Under the stderr rule each becomes a warning, contradicting the acceptance below. Settled ([RD159](../DECISIONS.md#recorded-decisions)): those agents stop writing routine lines to stderr; what is worth keeping becomes an explicit notice; stderr stays a warning |
| standalone agents | Their start, stop and crash have no runner to write them; today they are only in the host's service log |

### Failures the code hides today

Each needs its entry where it happens; found while writing the guides.

| Where | Today |
|---|---|
| `src/agents/rag/rag.py:33` | invalid Configuration is swallowed (`except RagError: pass`); only a status ask shows it |
| `src/agents/rag/rag.py:38` | collection verification runs only on a status ask; a failure sets health and logs nothing |
| `src/agents/standalone/logwatch/watcher.py:180`, `:245` | an unreadable source only changes its `sources` row; nothing is logged |
| `src/agents/standalone/logwatch/watcher.py:224` | a journalctl start failure is kept in `journal_error`, not logged |
| `src/agents/sms-telnyx/sms.py:114` | every 4xx is `failed`: a rejected key looks like one refused message |
| `src/agents/telegram/receive.ts:103` | a rejected token is logged and retried every five seconds without end |
| `src/agents/standalone/git/autopull.py:36` | a dirty copy, a warning, exits 1 like a real failure |
| `src/agents/shared/scheduler.py:248` | a lock or KV failure skips the pass silently; slots are missed unlogged |

## Acceptance

- Each alert in the per-agent table is raised by a fixture that provokes it: an unreachable database, a refused IMAP or SMTP login, a missing Secret or host key, an invalid Configuration, a closed webhook listener.
- A transient failure that recovers (one failed poll, one reconnect) produces a warning and no alert; replace the warning with an alert and its check fails.
- A successful normal run of each agent writes no warning and no alert; only start and stop notices.
- The runner's common alerts fire for an unrequested exit, a signal kill, a memory or process limit, a failed start and a restart loop; a requested stop writes a notice only.
- `#debug` `crash` yields the runner's kill alert; `#debug` `fail` yields one warning.
- No entry contains a secret, a token, a password, a message body, a prompt, an SQL statement or a mail; a fixture planting each in the failure path finds none in the rings.
- Every guide in `docs/agents/` has a `## Warnings and alerts` section whose rows match this table.

## B.28 Eventlog agent

⏳ Owner, 2026-10-29: [design](eventlog.md#what-it-does). Assigned: agent xAB,
web page AB (after RT22's page), review `#claude/ab2-review`.

| Step | Work |
|---|---|
| 1 | Shared ring module: levels, size, TTL, repeat folding, alerts on disk, used by this agent and later by RT34 |
| 2 | `#eventlog` runner agent: `write`, `read`, `pull`, `clean`, `list`, Configuration defaults and per-log overrides, `read`/`write` roles, help and help-config |
| 3 | Guide `docs/agents/eventlog.md` (end-user), index row, smoke shard, package, deploy enabled |
| 4 | Web: instance page with logs, alert marks and the shared rings view |

Acceptance, each failing on a missing implementation or by a named mutant:

- 201 notices into a log with size 200 keep the newest 200; a notice older than its TTL is gone on the next read.
- 300 warnings never remove an alert; alerts survive an agent restart, notices do not.
- The same line twice within a minute is one entry with count 2; after a minute it is a new entry.
- A caller with only `read` is refused on `write` and the log is unchanged; a caller with neither sees nothing; a 201-character line is stored cut at 200.
- A log name with `/`, a space or over the length bound is refused; the log count bound refuses a new log and keeps the old ones.
- `pull <log> notice 2` on 5 new notices returns the oldest 2, the next `pull` the following 2.
- Two `pull`s by one caller return each entry once, a second caller's first `pull` still gets them all; an entry expired or pushed out between pulls is counted as missed, not returned.
- `clean <log> warning` empties only warnings; `clean` by a `read`-only caller is refused and changes nothing.
- The web shows an alert mark only for logs with an alert within its window, and a reader without the role gets no page.

