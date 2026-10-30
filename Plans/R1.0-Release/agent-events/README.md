# Agent events

📌 **TL;DR:** the daemon keeps a syslog-like event log for each agent, in three
rings: notice, warning and alert. Starts and stops are notices, stderr lines are
warnings, unexpected failures are alerts. The API and the web show them to the
agent's Owner and Maintainers, and the web makes alerts loud and warnings
quieter. Owner, 2026-10-29; settings live in [the bus setup](../daemon-configuration.md#approved-plan).

Tasks: [RT34](../TODO.md#rt34-agent-events).

## Levels

| Level | Written when | Ring | Kept |
|---|---|---|---|
| notice | usual information: the runner starts or stops the agent; the agent or a diagnostic reports something worth keeping | its own | daemon memory |
| warning | usual warnings, never a failure: a line on the agent's stderr; a warning the agent reports; a daemon warning about the record (a dropped copy, a refused forward) | its own | daemon memory |
| alert | the agent's script failed: a non-zero exit nobody asked for, a kill by signal, memory or process limit, a start that fails, a restart loop, a sandbox refusal; or the script reports an alert itself | its own | disk, survives a daemon restart |

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
| the daemon | its own warnings about the record, the ones `error.log` holds today |
| the agent itself | its own record's rings only |
| diagnostics (`#debug`, doctor, `check-deploy`) | what they find about an agent, and they read the rings in their reports (owner: "diagnostic and alike should also do") |

## Who reads

The agent's Owner and Maintainers, and the daemon Owner, through the API and
the web (owner, 2026-10-29). Callers on the allow list see none, not even
entries their own messages caused: stderr is not tied to one message reliably
and may carry other callers' data, and a caller already gets an error answer.

An entry is one line, cut at 200 characters as `error.log` lines are; a line
repeated within a minute is kept once with a count of those held back. stderr
may carry anything an agent prints, which is why readers are Owner and
Maintainers only. Bodies and secrets are never written by the runner or daemon.

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
