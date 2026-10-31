# Runner security: what you grant the runner

**Run other people's scripts as team tools without handing them your server or your passwords.**

📌 **TL;DR:**
- **Problem:** a shared script can read every file and password its account can, and anyone who can reach it may be able to call it.
- **Outcome:** on Linux, each managed agent gets the [network and filesystem isolation described below](#isolation-today); a script run per message holds no bus password; only the callers you name may use it.
- **Start:** check that your agents are confined: `agent-bus call '#runner/<you>@<realm>' --wait 15s status` ([Try this first](#try-this-first)).

## Why you'd want it

- 🧱 **A script can touch everything.** Now the runner applies the [isolation boundaries below](#isolation-today), unless the host disables them for that agent.
- 🔑 **A script can leak its password.** Now a script run per message is handed no bus credential, and a secret value is masked in what it logs.
- 🙋 **Anyone could call it.** Now a new agent admits only you, until you name who else may.
- 💥 **One runaway script can take the host down.** Now the runner applies [resource limits](#isolation-today), including a soft work-directory disk check.

## What you get

- ✅ **Confined by default on Linux:** [isolation today](#isolation-today) defines the network, visible files, writable paths and limits, including refusals when the host cannot provide them.
- ✅ **No credential handed to a per-message script:** `AGENT_BUS_TOKEN` is removed from its environment, so it cannot leak from there.
- ✅ **Callers you choose:** who may call an agent is its record's [ACL](../02-access.md#acl), set the ordinary way; a fresh one admits only its Owner and Maintainers.
- ⚠️ **An agent that [serves itself](configuration.md#agents-that-serve-themselves)** is handed its own token, and only its own, since reading its inbox is its job.
- ⚠️ **Network access is broader than a list of hosts:** see [isolation today](#isolation-today).
- ⚠️ **Some keys are accepted and do nothing:** see [what is accepted but inert](#what-is-accepted-but-inert).
- 🚫 **`"sandbox": "off"`:** read [what it gives up](#isolation-today) before enabling it.

## Try this first

See whether each agent is confined, and its limits:

```sh
agent-bus call '#runner/parf@parf.us' --wait 15s status | jq -r .body \
  | jq -c '.agents[] | select(.key=="debug" or .key=="webhook") | {name,sandbox,limits}'
```

```text
{"name":"#debug@parf.us","sandbox":"on","limits":{"memory_bytes":536870912,"processes":64,"disk_bytes":1073741824}}
{"name":"#webhook@parf.us","sandbox":"on","limits":{"memory_bytes":536870912,"processes":64,"disk_bytes":1073741824}}
```

- `sandbox: on`: the [isolation below](#isolation-today) applies.
- `limits`: the default 512 MiB of memory, 64 processes and 1 GiB of disk.
- The output is from the development node on 2026-10-30; names differ on yours.

## Isolation today

**On Linux, `agent-bus-runner` uses bubblewrap by default for each managed agent.** The host can set `"sandbox": "off"` for an entry. This is not the optional `agent-bus run` sandbox described at the end of this section; the [macOS runner](../07-setup.md#the-client-and-runner) has no Linux sandbox or cgroup limits.

### Network reach

| Setting or form | What the agent can reach |
|---|---|
| No `needs.network`, or an empty list | It cannot make IP connections to the host's loopback services, LAN or internet: it has a private network namespace. This does not prevent communication through an explicitly mounted Unix socket |
| Nonempty `needs.network` | The host's network namespace, including its loopback services and any LAN or internet destination the account and host network policy permit; it can also listen on permitted ports. The listed hosts are **not** an allowlist |
| Per-message scripts and `jsonl` pools | The runner adds no bus socket or bus token for them; it reads and answers messages for them |
| An agent that serves its own inbox | Its own bus token is supplied. A configured Unix bus socket is mounted as one socket, never its parent directory, so it can use the bus even without IP networking. The listener must require authentication: an account socket or unverified listener is refused. A TCP bus address needs network access. The bus's ordinary ACL still applies |

Network isolation does not restrict what the agent writes in an answer or sends through an allowed connection. Give it only the credentials and network access its job needs.

### Visible files and writes

| View | What the agent can see or write |
|---|---|
| System files | All of `/usr` is mounted read-only, with `/bin`, `/sbin`, `/lib` and `/lib64` pointing into it. This includes readable files under `/usr/local`: other agents' public code installed there is visible too |
| Its code | Its selected package is mounted read-only, with host file overrides mounted read-only over their packaged paths. Git-checkout bundled agents also get the public `src/agents/shared`, Python and TypeScript client roots, and `src/internal/version`, read-only |
| Persistent host writes | Its own work directory is bound writable and is the working directory; `HOME` points there. The work bind is the only writable host directory added by the sandbox; system and package mounts are read-only. `needs.writable` adds no paths |
| Temporary files | A private `/tmp` tmpfs, sized from the memory limit, lasts for that child process; a kept process retains it across messages. It disappears when that sandbox ends |
| Other host files | Host home directories, runner settings, credentials and other work directories are not mounted as whole trees. Files outside the mounts above are unavailable by path; this is not a promise to hide a file placed inside `/usr`, the selected package, a mounted support root or the work directory. Ordinary account permissions still apply to visible files |
| Local interfaces | A private `/proc` shows the sandbox's processes and `/dev` supplies basic devices. With network enabled, existing resolver and certificate paths under `/etc` are mounted read-only; the rest of the host's `/etc` is not mounted. The self-serving socket exception is described above |

- **Environment values:** the runner reads host env files and injects their merged values; hiding file paths does **not** hide values supplied to that agent.
- **Secret files:** keep them outside public code roots.
- **Work paths:** a path containing a symlink is refused, and overlapping keys cannot be served together.

### Processes and limits

- **Separate process view and session:** other host processes are outside its PID namespace; all capabilities are dropped. A system-call filter refuses operations including `ptrace`, kernel keyrings, mounting and new namespaces.
- **Limits from process start:** every agent process starts in its agent's cgroup under the runner's delegated subtree. Memory and process limits, and configured CPU quota and I/O weight, apply there; open-file limits are applied with `prlimit`. Default CPU quota is unlimited, not a fixed share ([limits](configuration.md#limits)).
- **Disk is a soft check:** load/reload checks the work directory, as do exits of per-message children and self-serving processes. An over-limit agent is stopped/refused until it is emptied and reloaded. A `jsonl` pool does not run the post-message check, and there is no periodic running-process check or hard filesystem quota.
- **No silent fallback:** missing bubblewrap/user-namespace support, a delegated cgroup subtree or a usable system-call filter refuses the sandboxed agent by name. Startup logs name the backend ([reading the logs](operations.md#reading-the-logs)).

### With the managed sandbox off

- **Isolation removed:** `"sandbox": "off"` uses neither bubblewrap mounts/network namespace nor the managed cgroup, file, tmpfs and disk limits above.
- **Account reach:** the child can read other env files and code, write files, open account sockets and use the network wherever the runner account's permissions and host policy allow.
- **Still controlled:** message deadlines and runner control apply; use this setting only for code you would run as that account.

### The separate `agent-bus run` sandbox

The [dedicated runner](../06-dedicated-runner.md#sandboxing) is off by default. On Linux, `--sandbox=on` uses `systemd-run --user`, not the managed runner's bubblewrap view:

- **Files:** it retains the account's readable filesystem view, including a read-only home; it does not hide other readable files. `ProtectSystem=strict` makes the system read-only, with the work directory bound writable and the script directory bound read-only; private `/tmp` is available. Kernel interfaces such as `/dev`, `/proc` and `/sys` are not a blanket read-only filesystem guarantee.
- **Network:** private unless `--network` is supplied; then it shares the host's permitted network reach, not a hostname allowlist.
- **Limits:** no new privileges, a process limit and a memory limit. An unavailable user manager or required systemd option refuses the requested sandbox. With it off, the script has its starting account's reach.

## What runs as what

| | Runs as | Owned on the bus by |
|---|---|---|
| `#runner/<you>@<host>` | `<you>-ab-runner`, a system account of your runner's alone, unit `agent-bus-runner@<you>`, with your runner agent's token in `runner/runner.env` ([a person's runner](../07-setup.md#a-persons-runner)) | you |
| every agent it serves | the same account | you |
| one message's child process | the same account, in that agent's work directory | — |

- **The runner is your agent and never you:** it refuses to start with a person's token.
- **It creates each agent it serves for you.** That `register` answers the new agent's token ([getting a token](../02-access.md#getting-a-token)), which the runner keeps in `runner/tokens/`.
- **It reads as that agent,** secret and inbox alike, so the answer leaves under the name that was addressed.
- **An agent that already exists is refused by name** until you hand its token over once.
- **Taking it over,** the runner clears the `script` an earlier `agent-bus run` left on the record, so listings no longer name a program that does not serve it.
- **Author settings** (`agent.json` and `env.dist`), including their packaged help, are read only from regular files; a final symlink is refused by filename.

Hand an existing agent's token to your runner, then reload:

```sh
agent-bus token '#hello@host' | sudo -u you-ab-runner tee /var/lib/agent-bus/you-ab-runner/tokens/%23hello@host >/dev/null
agent-bus call '#runner/you@host' reload
```

## What the code is told, and what it may trust

The child gets the message as its argument (`form: "args"`) or as the envelope on stdin, plus these variables:

| | |
|---|---|
| `AGENT_BUS_MESSAGE_ID`, `AGENT_BUS_FROM`, `AGENT_BUS_TO`, `AGENT_BUS_TOPIC`, `AGENT_BUS_TAG` | who and what this message is |
| `AGENT_BUS_WORK` | this agent's work directory, writable, where the child starts |
| `AGENT_BUS_CONFIG` | compact stored JSON at this start; unset when unconfigured ([configuration](../03-records-agent.md#configuring-a-template)) |
| `AGENT_BUS_ASK` | Empty for ordinary work; every ask not answered by metadata reaches the script here with an empty body, unless `asks: none` suppresses execution ([package help](configuration.md#package-help)) |
| `AB_ROLE_<NAME>=1` | one per [delivered role](../03-records-agent.md#roles), including `owner` and `maintainer`, the sender holds toward this record |

- **The roles are a decision; the rest are labels.** The daemon works `AB_ROLE_*` out from the record's own allow list and overwrites anything the sender claimed, and the runner clears inherited ones before each run.
- **Branch on a role,** never on a string comparison against `AGENT_BUS_FROM`.
- **No credential comes with them.** `AGENT_BUS_TOKEN` is taken out, so a token cannot be read out of the environment, written to a log or passed to a child by accident.
- **The one exception** is an agent that [serves itself](configuration.md#agents-that-serve-themselves): it is handed its own token, since talking to the bus is its job, and still never yours.
- **What keeps the agent away from the bus itself** is the [sandbox](#isolation-today), not this.
- **An env file cannot forge any of this:** the runner's own stamp is applied last, so an `env` line setting `AGENT_BUS_FROM` loses to the real sender.

## Who may call an agent

**The record's ACL decides, exactly as for any other record.**

- **The runner sets no ACL** when it creates an agent, and an empty one [admits only the Owner and Maintainers](../02-access.md#acl) — here, you.
- **An agent shows in `status` as `idle`** and still refuses everybody else until you widen it.

Let one more person call the agent:

```sh
agent-bus manage '#hello@host' --add-allow alice@host
```

**Name the callers.** `--add-allow '*'` is broad access, and an agent reached by anybody is one you have to trust against anybody.

| Who can run that | |
|---|---|
| the record's Owner | you |
| the daemon Owner | node-wide record authority |
| an Administrator | **no** — administering users grants no record authority ([authority](../01-identity-and-authority.md#record-authority)) |

- **The runner gains no authority over what it creates:** it holds each agent's token, and only you manage the record.
- **At start and `reload`,** it checks that the token answers as that agent and that the agent is still yours.
- **A transferred agent or rotated-out token** is then stopped and refused by name; its messages wait for whoever serves it now.
- **An admitted operation** does not re-ask identity or authority midway ([authority at the door](../constitution.md#authority-at-the-door)).

## Who may manage the runner

`#runner/<you>@<realm>` is yours, under the ACL you give it.

- **It answers** the calls of [operations](operations.md#controlling-one-agent): status, reload, the controls, deployment, and reading and writing the host's files. Anything else is refused by name.
- **`status` is for whoever that ACL admits.**
- **Reload, controls, deployment and the host's files are only for the runner's Owner and Maintainers,** as the daemon works the sender's roles out toward the runner itself ([roles](../03-records-agent.md#roles)).
- **A Maintainer can replace host env files** but reads only their digest and variable names through the runner. Grant Maintainer only to someone trusted to change how its agents run.
- **The calls never name** the kept tokens or an agent's work.
- **Deployment calls** select bundled code or a fully pinned repository and write only a host-provisioned writable package directory ([deploying an agent](operations.md#deploying-an-agent)). They grant no host privilege or bus identity.

## Secrets

Keep secrets in Secret and settings in Configuration ([configuration](../03-records-agent.md#configuring-a-template)). There are **two distinct places to keep a secret**:

| Store | Where it lives | Who can read it | Leaves the host? |
|---|---|---|---|
| The record's [bus Secret](../03-records-service.md#secrets) | the daemon's database | the record's Owner and Maintainers, and the agent itself, over the bus | yes: shared through the bus with whoever may read it |
| The runner's host files (`runner/<agent>/env`, `runner/<agent>/<instance>/env`) | this host's `runner/` tree, mode `0700` | the runner's account, to start agents; viewing values requires host access | the runner returns only existence, SHA-256, size and variable names; writing through the bus carries the values in a message |

- **Use the bus Secret** for a credential several hosts or people must share.
- **Use the runner's host files** for one used only by this runner.
- **The runner merges both** into the agent's environment at start; **the runner's own secrets take over** a bus Secret key of the same name.

Secrets are injected into the process before it starts, never as a path the code could open:

| Layer, lowest first | Where | Who writes it |
|---|---|---|
| declared variables and non-secret defaults | `agent.d/<agent>/env.dist` | the agent's author |
| the record's [bus secret](../03-records-service.md#secrets) | the daemon | the Owner, on the bus |
| what every instance of the agent shares | `runner/<agent>/env` | this host |
| this instance's own | `runner/<agent>/<instance>/env` | this host |

- **Local wins over remote,** and an instance wins over its agent.
- **A required variable nobody supplies** (marked `# required` in `env.dist`, with a value in no layer) refuses the agent at `reload` by name, rather than starting it half-configured; an unmarked empty key is simply empty.
- **`runner/` is `0700`** and holds the secrets; **`agent.d/` is `0755`**, a checkout anyone on the host may read. **Never put a secret in `agent.d/`.**
- **File visibility follows [isolation today](#isolation-today);** package code is readable, and private host values are supplied through the environment.

Writing secrets over the bus or the web:

- **The daemon keeps no separate copy** of these host files, but a bus or web `write` carries their values in an ordinary durable message.
- **An unconsumed write** may be stored in the runner's inbox; its durable copy is removed at the checkpoint after consumption. SQLite free pages may retain the bytes until reused.
- **If the values must never enter daemon storage,** edit the file through host access instead.
- **The web editor starts empty,** and saving replaces the entire host env file.
- **To inspect values,** use host access, for example `sudo cat /var/lib/agent-bus/parf-ab-runner/slack/env`; `status` reports the actual `runner_dir`.
- **Debug `env`** masks all values except the delivered `AGENT_BUS_*` and `AB_ROLE_*` namespaces.

What the logs mask:

- **Masked with `***`:** values of at least four bytes from the bus Secret and private host env files, plus a self-serving agent's own token.
- **Where:** child stderr copied to the journal, the unconfigured reason and `last_exit.reason`.
- **Not masked:** Configuration and package defaults stay readable.
- **Shorter private values** cannot be reliably masked as substrings; each load warns by key name without logging the value.
- **An agent's own replies** are its code's responsibility.
- **The digest covers the whole file:** compare the hash, not a secret store. It confirms what was written, but does not hide a tiny guessable value.

## What is accepted but inert

`agent.json` and `agents.json` are read strictly: an unknown field is refused by name, so a typo cannot pass. These real keys are accepted in silence and change nothing:

| Key | Where | Would control |
|---|---|---|
| `needs.writable` | `agent.json` | writable paths beyond the work directory |
| `version` | `agents.json` | host-side package version pin |
| `origin`, `first_started`, `last_started` | `agents.json` | recording where an agent came from and when it ran |

- **`needs.network` has the [network effect described above](#isolation-today);** host names do not restrict destinations.
- **`processes` is half in force for a one-message script:** its maximum bounds how many copies run at once, its minimum does nothing, so `0..1` and `1` behave identically. For a `jsonl` agent both ends are in force.
- **An agent that serves itself** is kept at its `processes` count, restarted when one exits.
- **`"sandbox"` is fully in force.**
- **Unsupported forms fail loudly instead:** `msgpack` and `std` are refused by name. The table above is the set that fails silently.
- **Versions:** generated help shows the package's `agent.json` version (the shared bus version when omitted); the version in runner `status` is the **runner's**.

## Technical details

<details>
<summary>The cgroup and its limits</summary>

- **Kernel-enforced:** memory (no swap), CPU, processes and I/O weight.
- **Per process:** open files.
- **`/tmp`** is a tmpfs no larger than the memory limit.
- **Defaults:** 512M of memory, 64 processes, 1024 open files and a 1G soft disk limit ([limits](configuration.md#limits)).

</details>

<details>
<summary>The system-call filter</summary>

- **Answered `EPERM`:** kernel modules, rebooting, mounting, new namespaces (by `unshare`, or by `clone` with a namespace flag), another process's memory, keys, clocks, BPF and perf, io_uring, swap and quotas.
- **`clone3`** is answered "not implemented", so threads are made with the filtered `clone` instead.
- **A call of another architecture** is refused whole.
- **The cost:** no `ptrace` and no kernel keyring inside the sandbox ([where the sandbox stops](#isolation-today)).

</details>
