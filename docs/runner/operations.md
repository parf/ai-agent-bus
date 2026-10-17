# Running and checking the runner

📌 **TL;DR:** The runner is a systemd unit with two calls. `status` says what
it serves and why anything was refused, `reload` rereads the host's files, and
the unit's journal holds what the agents said. Every change to what is served
is a change to files on the host, followed by `reload`.

## The three things you have

| To find out | Use |
|---|---|
| whether the runner is up | `systemctl status agent-bus-runner` |
| what it serves, and why something was refused | `agent-bus call '#runner@<host>' status` |
| what it and its agents said | `journalctl -u agent-bus-runner` |

`#runner@<host>` answers `status` and `reload`, and nothing else; the host is
the daemon's realm. Only its Owner and the Administrators may call it
([who may manage the runner](security.md#who-may-manage-the-runner)).

## Reading `status`

The answer is JSON: the runner's name and version, one row per installed
agent, and `error` if the load failed as a whole.

```sh
agent-bus call '#runner@parf.us' --wait 15s status
```

| `state` | Means |
|---|---|
| `idle` | served, nothing running |
| `running` | one or more processes working; `processes.running` counts them |
| `refused` | not served; `reason` says why, in words |
| `disabled` | `"enabled": false` in `runner/agents.json` |
| `stopped` | enabled and serveable, but not being served — an agent whose registration failed |

`processes` reports `running`, `min` and `max`. Only `max` is in force today;
see [what is accepted but inert](security.md#what-is-accepted-but-inert).

A row is never left out. An agent the runner cannot serve appears as
`refused` with its reason, because an agent silently skipped is an agent that
looks fine and never answers.

## Installing, changing and removing an agent

Until deploying over the bus lands (wave M.8), all three are file changes
followed by one call.

| | |
|---|---|
| 1. install | `git clone <agent> /var/lib/agent-bus/agent.d/<agent>` — its code, its `agent.json`, its `env.dist` |
| 2. configure | if it is a template, one instance per use: `runner/<agent>/<instance>/env`. Host decisions go in `runner/agents.json` |
| 3. apply | `agent-bus call '#runner@<host>' reload` |
| 4. open it up | widen the new record's ACL, or nobody can call it: [who may call an agent](security.md#who-may-call-an-agent) |

`runner/agents.json` is optional. An agent directory under `agent.d/` with no
entry is served under its own name; entries exist to disable an agent, name it
explicitly, or declare the instances of a template.

**Nothing is picked up before `reload`**, including a directory added while the
runner was running. **Removing** is the same in reverse: take the entry or the
directory away and `reload`. The runner stops forwarding that name, and
anything already forwarded for it is answered `error: <name> is not served
here any more` rather than left in front of every other agent's work.

## Why an agent was refused

Every reason is a sentence in the `status` row and in the journal. The common
ones, as they read:

| Reason | What to do |
|---|---|
| `agent.json: missing` | the directory has no `agent.json` |
| `agent.json: json: unknown field "comand"` | a misspelled key; the files are read strictly, by name |
| `agent.json: command is required` | the one required field |
| `agent.json: form "x" is not args, json, std, jsonl or msgpack` | a form that does not exist |
| `the jsonl form is not served yet (M.3)` | a real form, not yet built — use `args` or `json` |
| `an agent that serves itself is not run yet (M.9)` | same, for `serves: "itself"` |
| `a template: configure an instance under runner/mail/<instance>/env (it needs IMAP_USER)` | `env.dist` requires a variable, so the agent runs only as a configured instance |
| `needs IMAP_USER` | that instance is still missing one |
| `agents.json: no agent mail is installed in agent.d` | an entry naming nothing installed — usually a typo left to look harmless |
| `#hello@host is also the name of hello2` | two installed agents resolve to one name; both are refused |

A `agents.json` that cannot be read at all refuses the **whole** load: `status`
carries `error`, the agent list is empty, and the journal says
`nothing loaded`. Every agent's host decisions are in that one file, so the
runner will not guess at half of them.

## Reading the logs

**The managed runner's log is its journal**, not `agent-bus logs`:

```sh
journalctl -u agent-bus-runner -f
```

`agent-bus logs <name>` and `agent-bus stop <name>` belong to the foreground
`agent-bus start`, which
[leaves a note](../06-runner-role.md#stopping-it-and-reading-what-it-said) in the launching user's own state directory. The managed runner leaves
no such note, so those two verbs cannot see it or its agents.

What you will find there: `serving #hello@host, 3 at a time` when an agent is
taken up, `no longer serving …` when it is dropped, each refusal by name, and
everything the agents wrote on **stderr**, each line prefixed with the agent
it came from. An agent's **stdout** is not logged — it is the answer.

## When a call does not come back

| Symptom | Cause |
|---|---|
| the caller is refused | the agent's ACL still admits only the runner's account ([ACL](security.md#who-may-call-an-agent)) |
| `READERS` is `0` in `agent-bus ls` | **normal for a managed agent.** The runner reads one inbox for all of them, so the readers show on `#runner@<host>` |
| the caller waits, then gives up | the script exited nonzero: no answer is sent, the failure is logged, and there is no retry |
| nothing ran at all | the caller's deadline had already passed when the message came up |
| the answer ends in `[agent-bus-runner: the answer was cut at 1048576 bytes]` | one answer is bounded at 1 MiB; the rest was dropped |
| `#runner` answers `error: … answers status and reload, not "x"` | the runner is not the agent you meant to call |

A message is taken off the bus only when a process slot is free, so a busy
agent holds nothing back from an idle one, and work not yet taken stays queued
if the runner stops.

## Stopping and restarting

`systemctl stop agent-bus-runner` ends the unit. Messages already taken are
lost in the ordinary way ([durability](../04-messaging.md#durability));
everything still queued waits for the runner to come back. The agents stay
registered and keep forwarding to `#runner`, so a restart resumes them.

A daemon that goes away is not a failure: the runner logs it and reads again.

## Not available yet

Today the runner starts processes for messages and nothing else. These are
planned, with their [waves](../../Plans/R1.0-Release/TODO.md#managed-runner-waves);
until each lands, do not write a procedure around it.

| | Wave |
|---|---|
| kept processes, pools, `SIGTERM`/`SIGHUP`, restart with backoff | M.3 |
| sandboxing and resource limits | M.4 |
| `enable`, `disable`, `start`, `stop`, `restart` and `backup` calls; override files | M.5 |
| installing and configuring over the bus, with no shell on the host | M.8 |
