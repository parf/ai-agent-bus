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
| 1a | Shared Python subscriptions and vectors complete on `task/eventlog`, awaiting QA/landing; [evidence](DONE.md#shared-subscriptions). B.29 TypeScript twin and consumer migrations remain pending |
| 1 | Shared ring module: levels, size, TTL, repeat folding, alerts on disk, used by this agent and later by RT34 |
| 2 | `#eventlog` runner agent: `write`, `read`, `pull`, `clean`, `list`, `sub add/list/delete`, Configuration defaults and per-log overrides, `read`/`write` roles, help and help-config |
| 3 | Guide `docs/agents/eventlog.md` (end-user), index row, smoke shard, package, deploy enabled |
| 4 | Web: instance page with logs, alert marks and the shared rings view |


### Agent implementation plan

- Shared `src/agents/shared/event_rings.py` owns independent notice/warning/alert rings, expiry, fixed 60-second folding windows, 200-character single-line text, one monotonically increasing sequence per log, id-bounded clean/pull and the persistent alert/counter snapshot. Language-neutral `src/testdata/event-rings.json` vectors are the contract for Python now and RT34's Go port later (owner approved); no daemon/API/subprocess bridge. Notice/warning entries are memory-only; ids reserve blocks of64 with a durable high-water mark (restart may skip unused ids, never reuse). Alerts and counter reservations use mode0600, fsync and atomic replace before a write returns; failure sends no subscriptions and reports an unconfirmed storage outcome if replacement may have happened. A storage error poisons mutation until state is revalidated, never rolls a counter back. A lifetime file lock waits at most2seconds for an overlapping generation, then fails closed. Corrupt/oversize state fails closed, never resets ids; Full clean retires the active log/frees max_logs while retaining its id tombstone (max10000 lifetime names, never evicted; at cap refuse new names, allow old names/cleanup); partial/level/id-bounded clean keeps it active. Alert pull/clean commits removal before answering, never restoring it after a crash.
- Configuration proposal: `rings` maps each level to optional `size` and `ttl` (duration); `logs` maps names to level overrides; `max_logs` defaults 100 (1..100). Defaults 200/1d, 200/7d, 50/30d. Each log's combined capacity is at most 500, and read/pull replies remain below 1 MiB; pull count defaults 100, range 1..500. Names are case-sensitive ASCII `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`. Requests <=16 KiB, strict UTF-8 JSON objects (duplicate/unknown keys, NaN/bools-as-counts/NUL refused); text grammar exactly the design, including optional level/count, no aliases or `--sub`.
- Proposed wire contract for AB review: entries `{id,level,text,at,last_at,count}` (ids/count integers; timestamps UTC RFC3339); `read` returns `{log,rings:{alert:[],warning:[],notice:[]},counts:{alert:n,warning:n,notice:n},latest_alert_at,now}` with newest ids first in each selected ring; an omitted level orders alert/warning/notice. `list` returns `{logs:[{log,counts,latest_alert_at}],now}` sorted by name. Null latest_alert_at means none; counts are live entries, folding occurrences are entry.count; the web derives age and its one-day alert mark from returned timestamps/now. `pull` returns `{log,entries:[...],remaining,now}` oldest ids first, atomically removing only the returned entries. `clean` returns `{log,removed,counts}`; optional integer `id` removes only entries <=that id, across all levels when level is absent. Missing logs are a safe error; no implicit creation except write. These types and examples go into agent.json/docs and are reviewed before parser code.
- Review pins: fold key is(level,text); A,B,A folds the second A into the newest live A whose first at is within60seconds, updates last_at/count, and keeps its id; pull/clean closes that fold, backwards time never folds. TTL uses last_at/wall clock; every call sweeps expiry, persisting alert removals. Times are UTC RFC3339 with fixed milliseconds/Z; newest/oldest order uses id, not a clock that can step. Strip ANSI/C0/C1/bidi/format controls before the200-character cut; text remains untrusted data. Text sub add with one selector word always names a log (including a log named alert); level-only selection uses the explicit JSON selector, avoiding reserved-name aliases. Errors are always `{error:{code,text}}` JSON (fixed codes/text, no bodies/credentials), including JSON-call failures; asks remain their standard text/metadata forms. Status uses message.roles only when original_to is absent/empty/self, never ambient AB_ROLE_* or forwarded channel roles.
- `src/agents/eventlog/` is a kept, bus-only Python runner agent, sandbox on, one serialized main-thread reader. Owner/Maintainers and write may write/pull/clean and read; read may only read/list. Bare ACL without either role gets no log data, including status; forwarded roles are never trusted for this record. Help is normal metadata, status is a truthful one-line ask. Separate instances isolate work state, Configuration and ACL; the host work directory is `AGENT_BUS_WORK`/cwd, never the package directory. No Secret or direct network access.
- Shared B.29 module `src/agents/shared/subscriptions.py` owns add/list/delete, creator/manager visibility, selector callbacks, bounded slash-regex matching, daemon may-send at add and EVERY send, destination queue/agent validation, one send/match, and sent/dropped counts. Agent-specific code supplies only selector validation/match and the message body. Language-neutral `src/testdata/subscriptions.json` vectors cover calls, matching, lifecycle, authorization and drop counters for AB2's TS twin. Eventlog uses it first; xAB2 owns later migrations of imap/logwatch/cron/at, so this landing changes none of them. Reuse the existing IMAP common JS/Python regex subset; batch matching runs in a bounded token-free helper process, not an unbounded regex on the inbox reader. Metadata/counters use the agent's own KV, max100 subscriptions/20 per creator; ids are12 lower-case hex with injected factory and clock; stored fields are id,target,selector,filter,created_by,created_at,sent,dropped. Consumers migrate valid old sub.<id> metadata to this shape rather than dropping it; obsolete disabled/paused/outbox state is not part of the new engine. Unsupported stored filters stay visible but inert with reason invalid_filter and a fixed warning, never become match-all. Agents enforce app method roles before engine calls; engine handles creator/staff visibility. creator-only list/delete except this record's Owner/Maintainers may manage all. No old-data replay. Shared vectors use stable error/warning codes, not prose, and pure state/input→ordered send/counter/list outputs. Checks run creator→target then creator→source, short-circuit first failure into exactly one dropped count/warning. Unknown delivery may have reached the inbox: dropped includes uncertain copies and is documented as such. Regex accepts bare patterns or /pattern/flags (as Slack/IMAP), flags remain imsu, <=200 Unicode scalar characters; g/y, lookarounds, named groups, backreferences, inline flags, \Z, non-ASCII classes and divergent set operations are refused. Python/TS batch matching has a1second helper/worker budget, separate group termination on Python; no claim that backreference-free regex is linear. Only direct authenticated roles on this record may grant read/write (write includes read). Before each forward also check creator still has send access to this eventlog instance; document that app-role assignment at creation authorizes the stored subscription until deletion, while daemon send-grant loss prevents forwarding.
- `sub.add/sub.list/sub.delete` JSON forms have target/log/level/filter or id fields, matching the design's text forms. Each accepted write (including a folded occurrence) synchronously attempts every matching subscription once with `{log,level,id,text,count,at,last_at}` JSON under #eventlog's own identity before pull is possible. A denied/full/unreachable/uncertain send is dropped and counted, emits one fixed content-free warning, and leaves the subscription ACTIVE for the next entry. No pause, retry, signal, outbox, backlog, cursor or pull --sub. Pull/clean never affect recipient inbox copies. Matching and forwarding have separate bounded budgets within a30-second call. Forwarding has at most8 independent workers, <=1second per subscriber including authority/delivery, and15seconds total for100 subs; a slow target cannot monopolize the whole budget. Results/counters are reduced in sub-id order, while network completions across different subscriptions may reorder. Refuse before storing when selector candidates exist and less than3seconds remain; no floor when none exist. Shorter admitted deadlines shorten the batch; no task survives the write answer. Per-pattern matching gets50ms and one pathological filter drops only its own copy. Failures/remaining unsent copies are counted without a later retry. The entry remains pullable even after a forward failure. Writes that stored an entry return `{entry,deliveries:{matched,sent,dropped}}`; storage uncertainty is reported honestly and never retried. `sub.list` returns `{subscriptions:[{id,target,selector,filter,created_by,created_at,sent,dropped}]}` for the permitted scope. Reviewer must pin this common Python/TS contract before code; AB2 gets the frozen vectors/API.
- Work order: first land shared subscriptions module/vectors/owning checks and runtime inventory (fresh patch), then eventlog rings/agent/package (fresh patch), so AB2 and xAB2 can build twins/migrations on the actual shared contract. No peer migrates runtime before the API/vectors are source-reviewed and landed.
- Files/tests: eventlog metadata/env.dist/package/config example, shared module+vectors, deterministic-clock state/ring tests, real isolated daemon role/subscription fixture, guide/index, all shared Python/release/setup executable+required-file inventories, acceptance helper, normal shard and fast essential map. Include the shared ring/subscription modules and each support copy in Python runtime/release inventories; vectors stay in the shared test-data contract. Package fixtures load the real .dist and install only declared artifacts. Runtime remains unconfigured-safe; enabled deploy by manager after green. Coordinate additive shared wiring with AB2/xAB2; AB owns web only, and receives the reviewed read/list contract before implementation.
- Named assertion-red mutants own size/TTL/severity independence, fold window/count, truncation, shared sequence/clean/restart, corrupt-state fail-closed, atomic counter persistence, roles/direct versus forwarded roles, name/log cap, destructive pull count/order/removal, partial clean id boundary, full-clean tombstone slot release, block reservation restart gaps, no duplicate alert after crash, read-only pull/clean refusal, creator-only sub management, may-send at add+each send, subscriber-copy-before-pull order, drop/no-pause/no-retry, selector isolation and JSON-vector parity, regex bounds, and package omission. Full affected reuse checks plus final normal under slow, one fresh patch/changelog, finished-block review, manager deployment/check-deploy report; no push.

Acceptance, each failing on a missing implementation or by a named mutant:

- 201 notices into a log with size 200 keep the newest 200; a notice older than its TTL is gone on the next read.
- 300 warnings never remove an alert; alerts survive an agent restart, notices do not.
- The same line twice within a minute is one entry with count 2; after a minute it is a new entry.
- A caller with only `read` is refused on `write` and the log is unchanged; a caller with neither sees nothing; a 201-character line is stored cut at 200.
- A log name with `/`, a space or over the length bound is refused; the log count bound refuses a new log and keeps the old ones.
- Ids rise across levels, survive a restart and a `clean`, and are never reused; a restart may skip numbers, never repeat one.
- A full `clean` of a log frees a place under the log bound; writing that name again gives ids above its old last id.
- `pull <log> notice 2` on 5 notices returns the oldest 2 and leaves 3; a `read` after it shows only those 3; a second caller's `pull` never gets the 2 again.
- `pull` by a `read`-only caller is refused and removes nothing.
- A subscription whose target refuses one message drops it, counts it, raises one warning and still delivers the next entry once the target accepts again; it never pauses.
- Subscriptions first: an entry written and at once taken by `pull deploys` has still reached every matching subscription's inbox; two subscriptions each get all their entries; a full subscriber inbox follows its own overflow and blocks neither the log nor other subscribers.
- `sub add <queue> deploys alert` gets exactly one message per new alert in `deploys`, carrying log, level, id and line, and none for its notices or other logs; a subscriber without may-send to the target is refused at add; a subscription's target losing the grant later stops sends with nothing delivered.
- `clean <log> alert 7` removes alerts with ids up to 7 and keeps alert 9 and every warning;
- `clean <log> warning` empties only warnings; `clean` by a `read`-only caller is refused and changes nothing.
- The web shows an alert mark only for logs with an alert within its window, and a reader without the role gets no page.

