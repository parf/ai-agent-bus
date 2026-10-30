# Running and checking the runner

**See what your agents are doing, and fix the one that is not, from one place.**

📌 **TL;DR:**
- **Problem:** a script that quietly stops answering looks fine until someone needs it, and finding out why means logging in and digging.
- **Outcome:** one call tells you which agents run and why any of them does not; one more starts, stops or reloads them.
- **Start:** ask your runner for its health line: `agent-bus call '#runner/<you>@<realm>' --status` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔍 **An agent stops and nobody notices.** Now `status` lists every agent, and one it cannot serve shows with the reason in words.
- 🔁 **Restarting means a server login.** Now you start, stop, restart, enable or disable one agent with a call.
- 📝 **Changing a setting means editing files by hand.** Now you read and write the runner's settings files with a call, and the change applies at once.
- 🧾 **Nobody knows what the script said.** Now everything an agent writes to its error output is in one journal, line by line, under its name.

## What you get

- ✅ **One health line:** running counts, on-demand agents, and any agent that is down.
- ✅ **Full status:** one row per installed agent, with its state, sandbox, limits and last exit.
- ✅ **Per-agent controls:** `start`, `stop`, `restart`, `enable`, `disable`.
- ✅ **Settings over the bus:** `read`, `write`, `create`, `delete` on the runner's own settings files.
- ✅ **Safe reloads:** `reload` rereads the files; an unchanged agent keeps running.
- ⚠️ **A new agent refuses outsiders** until you widen its ACL, however healthy `status` looks.
- ⚠️ **Secret values never come back:** reading an env file returns its variable names and digest only.
- 🚫 **No code install over the bus:** you put an agent's code on the host, then call `reload`.

## Try this first

Ask your runner for its one-line health verdict:

```sh
agent-bus call '#runner/parf@parf.us' --status | jq -r .body
```

```text
degraded: 11/12 running, 10 on demand (ai, fetch, qdrant, sms-telnyx, smtp/difive, smtp/eprofit, smtp/gmail, smtp/realmo, smtp/scienteco, voyage); eventlog (unconfigured)
```

- `degraded` names the agents that are down; here one exited as not configured.
- `on demand` agents start a process per message, so they are not counted as down.
- A disabled agent does not count as a failure.

<details>
<summary>The full status, for one or more agents (ran today)</summary>

Show three rows of the full status, without the empty fields:

```sh
agent-bus call '#runner/parf@parf.us' --wait 15s status | jq -r .body \
  | jq -c '.agents[] | select(.key=="debug" or .key=="eventlog" or .key=="db") | {name,state,reason,sandbox} | with_entries(select(.value != null))'
```

```text
{"name":"#db@parf.us","state":"disabled"}
{"name":"#debug@parf.us","state":"running","sandbox":"on"}
{"name":"#eventlog@parf.us","state":"unconfigured","reason":"eventlog: startup refused"}
```

The output is from the development node on 2026-10-30; names differ on yours.

</details>

## Where to look

| To find out | Use |
|---|---|
| whether your runner is up | `systemctl status agent-bus-runner@<you>` |
| what it serves, and why something was refused | `agent-bus call '#runner/<you>@<realm>' status` |
| what it and its agents said | `journalctl -u agent-bus-runner@<you>` |

- **Your runner is your agent,** `#runner/<you>@<realm>`.
- **It answers** `status`, `reload`, the [controls](#controlling-one-agent) and the [file calls](#the-hosts-files-over-the-bus).
- **`status` is for anyone its ACL admits.** Everything else is for its Owner and Maintainers only, since it can replace your host env files ([who may manage the runner](security.md#who-may-manage-the-runner)).
- **`--status`** gives the one-line verdict; plain `status` gives the full JSON the runner page and scripts use.

## Reading `status`

- **The answer is JSON:** the runner's name and version, one row per installed agent, and `error` if the load failed as a whole.
- **A row is never left out.** An agent that is silently skipped looks fine and never answers, so one the runner cannot serve shows as `refused` with its reason.

| `state` | Means |
|---|---|
| `idle` | served, nothing running |
| `running` | one or more processes working; `processes.running` counts them |
| `refused` | not served; `reason` says why, in words |
| `unconfigured` | the agent exited 78; fix its configuration and use `reload` ([not configured](#not-configured)) |
| `disabled` | `"enabled": false` in `runner/agents.json` |
| `stopped` | enabled and supported, but not being served: a `stop` call (its `reason` says so), or an agent whose registration failed |

- **`processes`** reports `running`, `min` and `max`. For a script only `max` is in force ([what is accepted but inert](security.md#what-is-accepted-but-inert)).
- **More fields** (start times, paths, sandbox, limits, last exit) are under [Technical details](#technical-details).

## Controlling one agent

- **Each control is `<verb> <agent>`,** naming the agent or its `agents.json` key.
- **It answers as `status` does,** or `error: …` naming what was wrong.
- **Only the runner's Owner and Maintainers** may make them.
- **Example:** `restart #mail/work@parf.us`, sent as the body of an `agent-bus call` to your runner, restarts that one agent.

| Call | Does | Lasts |
|---|---|---|
| `stop <agent>` | stops serving it: a kept process drains as a runner stop does, a message already being run finishes, the rest waits in its inbox | until `reload` or the runner's restart |
| `start <agent>` | serves it again | — |
| `restart <agent>` | `stop`, then `start` | — |
| `disable <agent>` | writes `"enabled": false` on its `runner/agents.json` entry, keeping every other entry and field | until changed |
| `enable <agent>` | writes `"enabled": true` there | until changed |

- **Each control rereads the host's files** as `reload` does, so its answer is what is on disk.
- **Only `reload` itself** forgets what `stop` calls took down.

## The host's files over the bus

- **Which files:** those of [configuration](configuration.md#where-settings-belong), named from the runner's home.
- **Who:** the runner's Owner and Maintainers only.
- **Nothing else can be named:** not the kept tokens, not an agent's work.
- **Modes:** a write keeps the file's mode; a new file is `600`.
- **Secret values never come back.** A write carries them in a durable message ([the two secret stores](security.md#secrets)).
- **The web editor starts empty,** including after an invalid submission. To see values, use host access and the `runner_dir` that `status` reports.

| Call | Does |
|---|---|
| `read <file>` | Host env files answer `file`, `exists`, `sha256` (of the original bytes), `size` and variable `names`, never values. Other files answer `file`, `exists`, `content`. An absent file says `exists: false` |
| `write <file>` + the content | the content is everything after the first line. It is checked as the runner will read it — env syntax, `agents.json` an object with valid limits, an override's keys — and refused with `error: …` and the file untouched, or replaces the file whole and applies it as `reload` does, answering as `status` |
| `create <agent>/<instance>` | adds its `agents.json` entry, enabled, and an empty `runner/<agent>/<instance>/env`; applied |
| `delete <agent>/<instance>` | removes the entry, its `env` and its override; its work directory, kept token and bus record stay |

| File | `read` | `write` |
|---|---|---|
| `agent.d/<agent>/agent.json`, `agent.d/<agent>/env.dist` | yes | no: the author's |
| `runner/agents.json` | yes | yes |
| `runner/<agent>/env`, `runner/<agent>/<instance>/env` | digest and variable names only | yes, whole replacement |
| `runner/<agent>/agent.json`, `runner/<agent>/<instance>/agent.json` | yes | yes ([overrides](configuration.md#overriding-the-authors-options)) |

- **Example:** `create mail/home` adds an instance; `write runner/mail/home/env` with `IMAP_USER=carol` on the next line sets its variable.
- **Each `status` row has a `key`,** the `<agent>` or `<agent>/<instance>` its files are named by.
- **A path through a symbolic link is refused.**
- **`tokens` and `work` under `runner/` are the runner's own,** so an agent of either name has its author's files read and no host files or instances.

## Installing, changing and removing an agent

- **Installing** is a file change on the host followed by a `reload` call.
- **Configuring** is a file change too, or a [call](#the-hosts-files-over-the-bus) that writes and applies it.

| Step | |
|---|---|
| 1. install | `sudo git clone <agent> /var/lib/agent-bus/agent.d/<agent>` — its code, its `agent.json`, its `env.dist` |
| 2. configure | if it is a template, one instance per use: `runner/<agent>/<instance>/env`. Host decisions go in `runner/agents.json` |
| 3. apply | `agent-bus call '#runner/<you>@<realm>' reload` |
| 4. open it up | widen the new record's ACL, or nobody can call it: [who may call an agent](security.md#who-may-call-an-agent) |

- **`runner/agents.json` is optional.** An agent directory under `agent.d/` with no entry is served under its own name.
- **Entries exist** to disable an agent, name it explicitly, or declare the instances of a template.

What takes effect when:

| Change | Takes effect |
|---|---|
| a script's contents | the next message: each message starts a fresh process |
| `agent.json`, an env file or the inventory | `reload` |
| a code update that changes those files | `reload`; a process already handling a message is not replaced by editing its file |
| a new directory, even one added while the runner runs | `reload` |
| a kept agent's installed package files | `reload` restarts it; identical files leave it running, and `.git` is not compared |
| per-message to self-serving, or back | `reload` stops the old mode and starts the new one |

**Removing takes care,** because an agent with no entry is served by default:

| To | Do |
|---|---|
| stop serving it, keep it installed | `disable <agent>`, or set `"enabled": false` on its entry in `runner/agents.json` |
| remove it | delete its directory under `agent.d/` **and** every `agents.json` entry naming it |

- **Deleting only the entry** does not remove the agent: the runner falls back to the directory and serves it under its own name again.
- **Deleting only the directory** stops it, but each orphaned entry then shows as `refused`.
- **Either way `reload` applies it.** The runner stops reading that agent's inbox; what is sent waits there until it is served again or its record is removed.

## Why an agent was refused

Every reason is a sentence in the `status` row and in the journal. The common ones, as they read:

| Reason | What to do |
|---|---|
| `agent.json: missing` | the directory has no `agent.json` |
| `agent.json: json: unknown field "comand"` | a misspelled key; the files are read strictly, by name |
| `agent.json: command is required` | a required field |
| `agent.json: description is required` | description is missing, empty or whitespace-only |
| `agent.json: form "x" is not args, json, std, jsonl or msgpack` | a form that does not exist |
| `the jsonl form is not served …` | `jsonl`, `std` and `msgpack` are refused by name; use `args` or `json` |
| `… serves itself, so the runner does not register it; its Owner registers it and hands it over once …` | register the agent yourself and put its token in the named file ([agents that serve themselves](configuration.md#agents-that-serve-themselves)) |
| `an agent that serves itself takes a fixed count of processes` | give `processes` a number, not a range |
| `a template: configure an instance under runner/mail/<instance>/env (it needs IMAP_USER)` | `env.dist` requires a variable, so the agent runs only as a configured instance |
| `needs IMAP_USER` | that instance is still missing one |
| `agents.json: no agent mail is installed in agent.d` | an entry naming nothing installed — usually a typo left to look harmless |
| `the sandbox is on and this host cannot give the agent a cgroup: …` | the runner's unit lacks a delegated cgroup subtree; reinstall it with setup, or turn the sandbox off for that agent ([isolation](security.md#isolation-today)) |
| `it needs 2G of memory and this host gives it … bytes (limits.memory)` | raise the host's `limits.memory` for it, or lower its `needs.memory` ([limits](configuration.md#limits)) |
| `its work directory holds … bytes, over its disk limit of … (limits.disk); empty … and reload` | empty the named directory, then `reload` |
| `#hello@host is also the name of hello2` | two installed agents resolve to one name; both are refused |
| `agents.json: sandbox "maybe" is on or off` | those are the two settings |
| `the sandbox is on and this host cannot give one: bubblewrap is not installed` | install bubblewrap, or set `"sandbox": "off"` for that agent and read [what that gives up](security.md#isolation-today) |
| `the sandbox is on and this host cannot give one: bubblewrap cannot make a sandbox here` | usually unprivileged user namespaces turned off on the host |

- **An `agents.json` that cannot be read refuses the whole load:** `status` carries `error`, the agent list is empty, and the journal says `nothing loaded`.
- **Why:** every agent's host decisions are in that one file, so the runner will not guess at half of them.
- **Secret and Configuration:** keep secrets in Secret and settings in Configuration ([rule](../03-records-agent.md#configuring-a-template)).
- **Record Configuration is not a host file.** Each new script process reads it as `AGENT_BUS_CONFIG`; a kept process sees changes after a restart or `reload` ([record configuration](../03-records-agent.md#configuring-a-template)).

## Not configured

An agent that exits **78** (`EX_CONFIG`) is `unconfigured`:

- **The runner stops serving it,** does not restart it, and takes no more messages from its inbox; queued messages wait there.
- **Its last stderr line is the `reason`** in `status`, even an unterminated one; no stderr means no reason. The line is bounded by the script-answer size limit.
- **A script agent's triggering message is answered,** as that agent: `error: <agent> is not configured: <reason>`.
- **Other exit codes behave as usual:** a failed script logs its failure without an answer, and a kept process restarts with backoff.
- **To try again,** fix the settings or code, then call `reload`. Editing files alone does not resume it, and there is no timer or automatic retry.
- **This is the runner's rule;** the dedicated `agent-bus run` command behaves as it always has.

## Reading the logs

The runner's log is its journal, not `agent-bus logs`. Follow it:

```sh
journalctl -u agent-bus-runner@<you> -f
```

- **`agent-bus logs` and `agent-bus stop`** belong to the dedicated `agent-bus run`, which [leaves a note](../06-dedicated-runner.md#stopping-it-and-reading-what-it-said) in the launching user's state directory. The runner leaves none, so they cannot see it.
- **`serving #hello@host, 3 at a time, sandbox bwrap`** when an agent is taken up: this is how you confirm what confined it; `sandbox off` is said just as plainly.
- **`no longer serving …`** when it is dropped, and each refusal by name.
- **Everything the agents write on stderr,** each line prefixed with the agent's name.
- **A per-message script's stdout is its answer** and is not logged; a self-serving agent's stdout goes to the journal too.
- **Both streams pass the [private-value filter](security.md#secrets).**

## When a call does not come back

| Symptom | Cause |
|---|---|
| the caller is refused | the agent's ACL still admits only you ([ACL](security.md#who-may-call-an-agent)) |
| `status` says `is <someone>'s now` or `the token kept … is not its any more` | the agent was transferred, or its token rotated: hand it over again ([what runs as what](security.md#what-runs-as-what)), or let its new person's runner serve it |
| `READERS` is `0` in `agent-bus ls` | the agent is not served: `status` says why, often `exists and its token is not kept here` ([who owns what](security.md#what-runs-as-what)) |
| the caller waits, then gives up | the script exited nonzero: no answer is sent, the failure is logged, and there is no retry |
| nothing ran at all | the caller's deadline had already passed when the message came up |
| the answer ends in `[agent-bus-runner: the answer was cut at 1048576 bytes]` | one answer is bounded at 1 MiB; the rest was dropped |
| `#runner` answers `error: … answers status, reload, start, …, not "x"` | the runner is not the agent you meant to call |
| `runner/mail/agent.json: form may not be overridden` | an override names what the code is; remove that key |

## Stopping and restarting

Stop and disable your runner, keeping every file ([a person's runner](../07-setup.md#a-persons-runner)):

```sh
sudo /usr/local/bin/agent-bus-setup --runner disable --for <you>
```

- **`systemctl stop agent-bus-runner@<you>`** only stops it.
- **Messages already taken are lost** in the ordinary way ([durability](../04-messaging.md#durability)); everything still queued waits for the runner.
- **A restart resumes every agent:** they stay registered, their messages wait in their inboxes, and the runner keeps their tokens in `runner/tokens/`.
- **A daemon that goes away is not a failure:** the runner logs it and reads again.

## Technical details

<details>
<summary>Extra status fields</summary>

Optional observations describe the runner and the admitted settings, without reading the host or keeping a history:

| Field | Meaning |
|---|---|
| Runner `started_at` | when this runner process began, RFC 3339 UTC; not the deployment's saved start date |
| Runner `agent_dir`, `runner_dir` | the installed code and host-configuration paths this runner uses |
| Agent `serves` | `runner`: a script per message; `itself`: processes that read their own inbox |
| Agent `sandbox` | `on` or `off` for an agent currently being served; absent when not served |
| Agent `limits` | applied `memory_bytes`, `processes` (OS process limit), and soft `disk_bytes`; absent with sandbox off or when not served. These are limits, not usage measurements |
| Agent `started_at` | earliest start among its currently live kept processes, RFC 3339 UTC; absent once none remain. Scripts report their current count in `processes.running`, without start times |
| Agent `last_exit` | latest kept-process exit or nonzero script exit observed since this load: `code` (`-1` for a signal), `at` after the wait completed (RFC 3339 UTC), and `reason` (last stderr line, empty if none) |

- **A fresh load clears `last_exit`;** the next real exit replaces it.
- **A failure to start a local process** does not invent an exit.
- **A successful script** does not replace its last failure.
- **Retiring processes** cannot overwrite their replacement's observation.
- **There is no** restart total, timer or exit history.

</details>

<details>
<summary>File names in the file calls</summary>

- **`agent.d/…` and `runner/…` are logical names.** The runner resolves them under `--agents` and `--host`, whatever their physical paths.
- **`status` still reports** the real `agent_dir` and `runner_dir`.
- **A message an agent had already taken as it was stopped** is run and answered, not dropped.

</details>

<details>
<summary>Process lifecycle and the unit</summary>

- **Per message:** the runner starts one process per message, up to the agent's `processes` maximum.
- **Self-serving agents** ([agents that serve themselves](configuration.md#agents-that-serve-themselves)) are kept running: restarted with backoff when they exit, sent `SIGTERM` when they stop.
- **A message leaves the bus only when a process slot is free,** so a busy agent holds nothing back from an idle one, and work not yet taken stays queued if the runner stops.
- **The unit's first stop signal goes only to the runner** (`KillMode=mixed`), so it can drain agents itself.
- **Systemd kills what is left after 30 seconds;** use a unit drop-in with a longer `TimeoutStopSec` when an agent needs a longer shutdown.

</details>
