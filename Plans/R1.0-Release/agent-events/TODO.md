# Agent events TODO

📌 **TL;DR:** the owner's R1 cut is per-agent logs, subscriptions and a small
web preview with expand. The [design and estimate](README.md#build-split-and-estimate)
own the scope; the checks below decide when it is built. Existing producer
catalogues remain follow-up work, not an extra requirement hidden in P1.

## R1-min

Contract: [verbs and authority](README.md#r1-min-surface-and-authority),
[shared model](../../../docs/agents/eventlog.md#log-model),
[retention](README.md#flood-and-persistence). Persistence is
[decided on disk, Q321](../DECISIONS.md#recorded-decisions). P1 exposes the
[small storage ports](README.md#p1-storage-seam-for-p5); P5 supplies durable
alerts, while notices/warnings remain volatile. Id reservations and
subscription definitions are durable.
Do not mark RT34 complete on the smaller no-subscription comparison estimate.

| Part | Owner/order | Falsifiable acceptance | Status |
|---|---|---|---|
| P1 engine, verbs, ACL, ids, vectors | Landed | [Completed checks](done/r1-landed-parts.md#p1) | ✅ `3b87ce4c` · 0.9.289 |
| P2 runner observations | Landed | [Completed checks](done/r1-landed-parts.md#p2) | ✅ `d8228153` · 0.9.290 |
| P3 subscriptions | W2B after P1 | Shared semantics plus new daemon storage/ACL/send proof below | ⏳ |
| P4 web | Landed | [Completed checks](done/r1-landed-parts.md#p4) | ✅ `bedc2a2f` · 0.9.291 |
| P5 alerts on disk | W2B, build parallel with P3, land after it | Real store/migration/restart checks below; plugs into the P1 seam | ⏳ |
| Read, normal gate and release-run touched set | Two readers on ⚠️ changes, then gate | Compiling mutants fail their owning assertions; restored touched checks and normal gate green | ⏳ |

### P3 checks

- `sub add` starts now; wrong source authority or target admission refuses it.
  The creator or current source managers manage it; other admitted callers
  cannot list/delete it. Restart keeps definitions without replaying entries.
- Apply `src/testdata/subscriptions.json` to the Go adapter, including the
  portable regex rules. No Go-only regex extension silently changes matching.
- A matching new occurrence is sent once before pull can remove it, including
  an accepted fold. Immediate pull cannot steal the copy. Two subscriptions
  each get their copy; unrelated logs/levels do not match.
- A refused/full/unavailable target drops and counts that copy; the next event
  still attempts delivery. A revoked creator/source/target sends nothing.
  No outbox, retry, pause, or credential stored in definitions; no recursive
  event storm when delivery reporting itself fails.
- While a real selected subscriber's injected send is held, an unrelated
  read and status answer before it is released. Pull/clean of that source
  cannot erase its entry yet. Per-target and whole-write budgets drop/count
  remaining copies; a cancelled attempt cannot later send. Test through the
  real dispatch and subscription path, then break the lane/barrier to see the
  named assertion fail. See [dispatch bounds](README.md#bounded-dispatch-and-fairness).
- Mutants remove each ACL check, retry a refused copy, skip subscription
  persistence, or let pull win; each compiles and fails a named assertion.

### P5 checks, required by Q321

- A real table/migration backend attaches through the P1 seam after P3.
  The test restarts the actual daemon and alerts survive within the shared
  size/TTL limits, while notices and warnings disappear. Run the existing
  durable-alert shared cases. The second P3/P5 lander renumbers schema10/11
  after rebasing; both migrations must remain, starting from schema9.
- Failed alert append or pull/clean removal cannot acknowledge partial state;
  crash/reopen cannot replay a delivered occurrence or reuse an id. Corrupt
  state fails closed, and rollback leaves the old store usable.
- Compiling storage mutants drop persistence, omit removal commits or reset
  ids; each fails its own assertion. No live logs are migrated in this part.
- Until P5 is present, no deployment or reply may claim durable alert
  acceptance. Both SQLite and memory ports are tested, and damaged load fails
  closed without resetting ids or adopting old-name records.

## Post-1.0 parts

| Part | Falsifiable acceptance | Status |
|---|---|---|
| Eventlog front/migration | Existing alert ids, high-waters, configuration and subscriptions survive isolated import; no replay/double writer; original files survive failed import/rollback | ⏳ |
| Daemon's own producers | Fixed safe templates at each selected report site; planted tokens/bodies/raw refusal internals never enter daemon-authored lines | ⏳ |
| Runner provenance B | Manager-controlled daemon binding, foreign-runner/rebinding race mutants, no tag/shared-owner shortcut | ⏳ |

The catalogue below is retained design work for later producer integrations.
It is not all included in the four-part R1 cut.

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

The built named-log contract is the [eventlog guide](../../../docs/agents/eventlog.md#log-model).
The old implementation/configuration/wire proposal is [dated history](done/b28-plan-history.md).
Only the later producer tasks above remain active here.
