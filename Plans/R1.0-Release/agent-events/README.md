# Agent events

📌 **TL;DR:** the daemon keeps a syslog-like event log for each agent, in three
rings: notice, warning and alert. Starts and stops are notices, stderr lines are
warnings, unexpected failures are alerts. The API and the web show them to the
agent's Owner and Maintainers, and the web makes alerts loud and warnings
quieter. Owner, 2026-10-29; settings live in [the bus setup](../daemon-configuration.md#approved-plan).

Tasks: [RT34](../TODO.md#rt34-agent-events), [per-agent events](TODO.md#per-agent-events).

## Levels

| Level | Written when | Ring | Kept |
|---|---|---|---|
| notice | usual information: the runner starts or stops the agent; the agent or a diagnostic reports something worth keeping | its own | daemon memory |
| warning | usual warnings, never a failure: a line on the agent's stderr; a warning the agent reports; a daemon warning about the record (a dropped copy, a refused forward) | its own | daemon memory |
| alert | the agent's script failed: a non-zero exit nobody asked for, a kill by signal, memory or process limit, a start that fails, a restart loop, a sandbox refusal; or the script reports an alert itself | its own | disk, survives a daemon restart |

stderr is for problems only: an agent never writes routine lines there; what
is worth keeping it writes as an explicit notice (owner, 2026-10-29: "just
writing to stderr is bad design").

One ring per level, so a stream of stderr warnings never pushes an alert out
(owner, 2026-10-29). Each ring is fixed-size; when full, the oldest entry is
lost. Every entry also expires after its level's TTL, so nobody sees that an
agent failed 100 days ago (owner, 2026-10-29). Defaults, changed in the bus
setup:

| Level | Entries per agent | TTL |
|---|---|---|
| notice | 200 | 1 day |
| warning | 200 | 7 days |
| alert | 50 | 30 days |

## Who writes

| Writer | Writes |
|---|---|
| the runner | start, stop, stderr lines and unexpected failures of the agents it runs |
| the daemon | its own warnings about the record, the ones `error.log` holds today; its own life into [`#bus`](#the-buss-own-events) |
| the agent itself | its own record's rings only |
| diagnostics (`#debug`, doctor, `check-deploy`) | what they find about an agent, and they read the rings in their reports (owner: "diagnostic and alike should also do") |

## Who reads

The agent's Owner and Maintainers, and the daemon Owner, through the API and
the web (owner, 2026-10-29). Callers on the allow list see none, not even
entries their own messages caused: stderr is not tied to one message reliably
and may carry other callers' data, and a caller already gets an error answer.

Every entry has an increasing id per agent, one sequence across the three
levels, never reused, so a reader can resume after the last id it saw.

An entry is one line, cut at 200 characters as `error.log` lines are; a line
repeated within a minute is kept once with a count of those held back. stderr
may carry anything an agent prints, which is why readers are Owner and
Maintainers only. Bodies and secrets are never written by the runner or daemon.

## The bus's own events

`#bus` is the daemon's own record, so the daemon writes its own life into
`#bus`'s rings (owner, 2026-10-29: "#bus itself must write there too"). Readers
are the daemon Owner. `error.log` and syslog keep getting what they get today;
the rings are an extra, readable view.

| Level | When |
|---|---|
| notice | the daemon started (version, realm, listeners) or stopped cleanly; the bus setup changed (who, which key); `debug.log` turned on or off; a store migration ran at start; a backup or restore finished |
| warning | what `error.log` holds as a warning today: a pubsub copy dropped for an inactive or full recipient, a ring queue dropping its oldest, a forward refused for crossing owners with roles; free space on the store's disk under 10 %; a burst of failed authentications from one address; a listener's TLS certificate expiring within 14 days; the clock stepped back; an unreadable stored activity day |
| alert | a [conceptual error](../../../docs/constitution.md#errors-and-alerts): a violated invariant, corrupt stored state, a failure to answer with authority; a store write that failed (disk full, I/O error); a listener that cannot bind at start; an expired TLS certificate; the previous run ended without a clean stop, found at start |

- The daemon cannot write while it is down: a crash shows as the start-time
  alert of the next run, with the time of the last clean write.
- Agents' own events stay in their own rings; `#bus` never copies them. The web
  home sums alerts across `#bus` and the agents the viewer manages.
- Thresholds (10 %, 14 days, the burst size) are bus setup keys with these
  defaults.

## Web

| Where | Shows |
|---|---|
| agent list | an alert mark on agents with alerts in the last day; warnings as a quieter count |
| agent page | the three rings, alerts first and prominent, warnings below, notices folded |
| home | recent alerts across the agents the viewer manages |

Red for alerts, orange for warnings, nothing loud for notices
(red/orange are reserved for errors and warnings). Alerts need no
acknowledgement: they stand out by recency (owner, 2026-10-29).

## Not in this

- No push: alerts are not published to a channel; a watcher reads the API
  (owner, 2026-10-29).
- No change to `error.log`, `audit.log` or syslog.
- Queues and other records: agents first; a record's daemon warnings may join later.
