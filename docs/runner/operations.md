# Running and checking the runner

📌 **TL;DR:** The runner is a systemd unit with two calls. `status` says what
it serves and why anything was refused, `reload` rereads the host's files, and
the unit's journal holds what the agents said. Change configuration on the host
and apply it with `reload`; script-content updates affect the next message.

- **Two calls, `status` and `reload`.** There is no call that starts, stops or
  installs an agent yet.
- **`status` leaves nothing out.** An agent it cannot serve is listed as
  `refused`, with the reason in words.
- **The journal is the log.** `agent-bus logs` belongs to the dedicated
  command and cannot see any of this.
- **Widen the new agent's ACL** or your first call is refused, however healthy
  `status` looks.
- **The host needs bubblewrap.** Agents are sandboxed by default, and a host
  that cannot make one refuses the agent instead of running it unconfined.

## The three things you have

| To find out | Use |
|---|---|
| whether the runner is up | `systemctl status agent-bus-runner` |
| what it serves, and why something was refused | `agent-bus call '#runner/<you>@<realm>' status` |
| what it and its agents said | `journalctl -u agent-bus-runner` |

Your runner is your agent, `#runner/<you>@<realm>`; it answers `status` and
`reload`, and nothing else. Who may call it is its ACL, yours to set
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
| `stopped` | enabled and supported, but not being served — an agent whose registration failed |

`processes` reports `running`, `min` and `max`. Only `max` is in force today;
see [what is accepted but inert](security.md#what-is-accepted-but-inert).

A row is never left out. An agent the runner cannot serve appears as
`refused` with its reason, because an agent silently skipped is an agent that
looks fine and never answers.

## Installing, changing and removing an agent

Deploying over the bus is planned and not built, so installation and
configuration are file changes followed by a `reload` call.

| | |
|---|---|
| 1. install | `git clone <agent> /var/lib/agent-bus/agent.d/<agent>` — its code, its `agent.json`, its `env.dist` |
| 2. configure | if it is a template, one instance per use: `runner/<agent>/<instance>/env`. Host decisions go in `runner/agents.json` |
| 3. apply | `agent-bus call '#runner/<you>@<realm>' reload` |
| 4. open it up | widen the new record's ACL, or nobody can call it: [who may call an agent](security.md#who-may-call-an-agent) |

`runner/agents.json` is optional. An agent directory under `agent.d/` with no
entry is served under its own name; entries exist to disable an agent, name it
explicitly, or declare the instances of a template.

Changing the contents of an existing script takes effect on the next message:
each message starts a fresh process. Changing `agent.json`, an env file or the
inventory needs `reload`. A code update that changes these files needs a reload
too; a process already handling a message is not replaced by editing its file.

**New directories and configuration changes wait for `reload`**, including a
directory added while the runner was running.

**Removing takes care**, because an agent with no entry is served by default:

| To | Do |
|---|---|
| stop serving it, keep it installed | set `"enabled": false` on its entry in `runner/agents.json` |
| remove it | delete its directory under `agent.d/` **and** every `agents.json` entry naming it |

Deleting only the entry does not remove the agent: the runner falls back to the
directory and serves it under its own name again. Deleting only the directory
does stop it, but each orphaned entry then shows as `refused`.

Either way `reload` applies it. The runner stops reading that agent's inbox;
what is sent to it waits there, as for any agent nobody is serving, until it
is served again or its record is removed.

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
| `agents.json: sandbox "maybe" is on or off` | those are the two settings |
| `the sandbox is on and this host cannot give one: bubblewrap is not installed` | install bubblewrap, or set `"sandbox": "off"` for that agent and read [what that gives up](security.md#isolation-today) |
| `the sandbox is on and this host cannot give one: bubblewrap cannot make a sandbox here` | usually unprivileged user namespaces turned off on the host |

A `agents.json` that cannot be read at all refuses the **whole** load: `status`
carries `error`, the agent list is empty, and the journal says
`nothing loaded`. Every agent's host decisions are in that one file, so the
runner will not guess at half of them.

## Reading the logs

**The runner's log is its journal**, not `agent-bus logs`:

```sh
journalctl -u agent-bus-runner -f
```

`agent-bus logs <name>` and `agent-bus stop <name>` belong to the dedicated
`agent-bus run`, which
[leaves a note](../06-dedicated-runner.md#stopping-it-and-reading-what-it-said) in the launching user's own state directory. The runner leaves
no such note, so those two verbs cannot see it or its agents.

What you will find there: `serving #hello@host, 3 at a time, sandbox bwrap`
when an agent is taken up — which is also how you confirm what confined it,
`sandbox off` being said just as plainly — `no longer serving …` when it is
dropped, each refusal by name, and everything the agents wrote on **stderr**,
each line prefixed with the agent it came from. An agent's **stdout** is not
logged; it is the answer.

## When a call does not come back

| Symptom | Cause |
|---|---|
| the caller is refused | the agent's ACL still admits only you ([ACL](security.md#who-may-call-an-agent)) |
| `READERS` is `0` in `agent-bus ls` | the agent is not served: `status` says why, often `exists and its token is not kept here` ([who owns what](security.md#what-runs-as-what)) |
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
registered, their messages wait in their own inboxes, and the runner keeps
their tokens in `runner/tokens/`, so a restart resumes them.

A daemon that goes away is not a failure: the runner logs it and reads again.

## Not available yet

Today the runner starts a process for each message and nothing else. All of
this is planned and none of it works yet, so do not write a procedure around
any of it:

- kept processes and pools, `SIGTERM` and `SIGHUP`, restart with backoff
- resource limits: memory, CPU, processes, open files and disk I/O
- confining an agent's network to the hosts it named, rather than all or none
- `enable`, `disable`, `start`, `stop`, `restart` and `backup` calls
- override files
- installing and configuring over the bus, with no shell on the host

The order they are planned in is the
[roadmap](../../Plans/R1.0-Release/TODO.md#runner-waves). A refusal
that names one of its stages, such as `(M.3)` above, points at the same place.
