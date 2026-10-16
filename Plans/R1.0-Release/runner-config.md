# Managed runner: agent configuration options

Status: **proposal, for owner review** ([Q155](QUESTIONS.md#open-questions)).
Option names and meanings only; no file format yet. The main points are on
the [runner page](runner.md#what-an-agent-ships).

📌 **TL;DR:** Every requirement in the runner's plans lands on one of four
homes: the author's `agent.json`, the host's `agents.json`, an instance's
`env`, or the bus record. Each option has one home, or an author default the
host may override. Three older names collide with newer decisions and are
proposed for removal.

## Where an option lives

| Home | Owned by | Holds |
|---|---|---|
| `agent.d/<agent>/agent.json` | the author, with the code | what the code is and can do |
| `agent.d/<agent>/env.dist` | the author | the variables it reads ([secrets](runner.md#secrets)) |
| `runner/agents.json` | the host | what runs here, how many, how confined |
| `runner/<agent>[/<instance>]/env` | the host | variables and secrets |
| the runner's unit | the host | which bus the runner serves, the account it runs as ([runner unit](runner-implementation.md#runner-unit)) |
| the bus record | the Owner, on the bus | ACL, record secret, `config`, `deliver_to`: never runner configuration |

**Rule:** a property of the code is the author's; a decision about this host
is the host's. Where both have a say, the author gives the default and the
host overrides it, as `depends` already does
([what it comes after](runner-implementation.md#what-it-comes-after)).

## The author's options

| Option | Says | Requirement it comes from |
|---|---|---|
| `description` | one line for the record | registration |
| `version` | what is installed, for a script that cannot answer a `version` call | [version reporting](runner-implementation.md#agent-version-reporting) |
| `command` | how to start it; the only place it is written | [what an instance is](runner-implementation.md#what-an-instance-is) |
| `form` | `args`, `json`, `std`, `jsonl`, `msgpack`; decides one-message or kept | [forms](runner-implementation.md#additional-script-forms) |
| `serves` | `runner` (the default): registered with `deliver_to` and fed by the runner. `itself`: links a client library, registers and reads its own inbox; the runner only starts it and restarts it when it dies | Q151, settled 2026-10-16 |
| `processes` | the default count or range | [kinds of agents](runner.md#kinds-of-agents) |
| `in_flight` | how many messages one kept process may hold, echoing topic and tag; default 1 | Q13, [long-lived agents](runner-implementation.md#long-lived-agents) |
| `deadline` | how long one message may take before the process is killed | [long-lived agents](runner-implementation.md#long-lived-agents) |
| `idle` | how long a process above `min` waits before `SIGTERM`; default 5 minutes | Q154, settled 2026-10-16 |
| `roles` | the [roles](../../docs/03-records-agent.md#roles) it understands | roles |
| `depends` | what it comes after | [what it comes after](runner-implementation.md#what-it-comes-after) |

Not an option, because it is derived: whether the agent is a template (an
`env.dist` variable with no default), and its lifetime (its `form`).

## The host's options

One row per installed agent or instance in `agents.json`:

| Option | Says | Overrides |
|---|---|---|
| `enabled` | started by the runner, or installed and left alone | — |
| `processes` | this host's count or range | the author's |
| `idle`, `deadline` | this host's times | the author's |
| `name` | a complete name, taken whole; default the directory and this host | — |
| `share` | this copy joins a pool of one name across hosts | — |
| `sandbox` | the confinement | — |
| `depends` | entries added, or turned off as remote | the author's |
| `version`, `origin`, `first-started`, `last-started` | written by the runner, never by hand | — |

## What `serves: itself` changes

| | `serves: runner` | `serves: itself` |
|---|---|---|
| registers the name | the runner, with `deliver_to` to itself | the agent, with its own token |
| holds a token | no | yes: a variable in its instance `env` |
| `form`, `in_flight`, `deadline` | used | ignored: the runner sees no messages |
| `processes` | count or range | count only: the runner cannot see waiting messages, so it cannot grow a pool |
| `SIGTERM`, `SIGHUP` | sent to an idle process | the same signals; `SIGTERM` only when the runner stops ([stop and reload](runner.md#stop-and-reload)) |
| when it dies | restarted, with backoff | the same |

## Collisions with older names

| Older name | Collides with | Proposal |
|---|---|---|
| `config.json` ([what an instance is](runner-implementation.md#what-an-instance-is)) | `agent.json` on the runner page | one file, `agent.json` |
| `autostart: on · off · on-demand` ([the list](runner-implementation.md#the-list-of-what-is-installed)) | `processes` with `min` 0 is on demand | `enabled` on or off; on demand is `processes: 0..N` |
| `-N` on a managed agent | `processes` | `processes`; `-N` stays the foreground `start` option |
| the author naming the agent (runner page) | the implementation's rule that only the directory and the host name it | the host's `name` only |
