# Runner security: what you grant the runner

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
- ✅ **Strict configuration:** unknown keys and unsupported forms are refused ([configuration controls](#configuration-controls)).
- 🚫 **`"sandbox": "off"`:** read [what it gives up](#isolation-today) before enabling it.

## Try this first

Use your runner's complete name from `agent-bus ls --kind=agent`:

```sh
agent-bus call '#runner/<you>@<realm>' --wait 15s status
```

Inspect each agent's `sandbox` and `limits`. `sandbox: on` applies the Linux isolation below; it does not make the agent code trustworthy.

## Isolation today

**On Linux, `agent-bus-runner` uses bubblewrap by default for each managed agent.** The host can set `"sandbox": "off"` for an entry. This is not the optional `agent-bus run` sandbox described at the end of this section; the [macOS runner](../platforms/macos.md#client-and-runner) has no Linux sandbox or cgroup limits.

### Network reach

| Setting or form | What the agent can reach |
|---|---|
| No `needs.network`, or an empty list | It cannot make IP connections to the host's loopback services, LAN or internet: it has a private network namespace. This does not prevent communication through an explicitly mounted Unix socket |
| Nonempty `needs.network` | The host's network namespace, including its loopback services and any LAN or internet destination the account and host network policy permit; it can also listen on permitted ports. The listed hosts are **not** an allowlist |
| Per-message scripts and `jsonl` pools | The runner adds no bus socket or bus token for them; it reads and answers messages for them |
| `needs.dir` | The Configuration key `dir` names one absolute directory, checked again at every start and reload on the Configuration just read, and bound read-only; the agent reads it as the runner's per-agent user and cannot write it. **The node owner decides what may be watched:** the agent's `roots` in `runner/agents.json` (default empty, so nothing is watched until a root is listed, e.g. `"filescan": {"enabled": true, "roots": ["/srv/vault"]}`); `dir` must resolve inside one, symlinks followed, so a link cannot lead out. Refused by name, whatever the roots: `/`, the account's home and the directory holding homes, `/proc`, `/sys`, `/dev`, `/run`, `/var/run` and `$XDG_RUNTIME_DIR` (session environments and account sockets), `/var/lib/agent-bus` and `/etc/agent-bus` (the tokens and environment files of every agent), anything that is, contains or lies inside the runner's own state (runner directory, shared agent directory, bundled agents), and a tree holding a unix socket (checked to 100000 entries). A relative or missing path is refused too. **A root must not contain directories that untrusted users can write:** they could swap a path for a link between the check and the sandbox start, and the bind would then follow it (read-only, as far as the runner's account can read). The agent uses `AGENT_BUS_DIR`, since only that real path exists in its sandbox; the runner re-resolves it at each launch rather than binding by file descriptor, so a swap between that check and the sandbox starting needs write access to a parent directory. `agent-bus run` does not bind it |
| An agent that serves its own inbox | Its own bus token is supplied. A configured Unix bus socket is mounted as one socket, never its parent directory, so it can use the bus even without IP networking. The listener must require authentication: an account socket or unverified listener is refused. A TCP bus address needs network access. The bus's ordinary ACL still applies |

Network isolation does not restrict what the agent writes in an answer or sends through an allowed connection. Give it only the credentials and network access its job needs.

### Visible files and writes

| View | What the agent can see or write |
|---|---|
| System files | All of `/usr` is mounted read-only, with `/bin`, `/sbin`, `/lib` and `/lib64` pointing into it. This includes readable files under `/usr/local`: other agents' public code installed there is visible too |
| Its code | Its selected package is mounted read-only, with host file overrides mounted read-only over their packaged paths. Git-checkout bundled agents also get the public `src/agents/shared`, Python and TypeScript client roots, and `src/internal/version`, read-only |
| Persistent host writes | Its own work directory is bound writable and is the working directory; `HOME` points there. The work bind is the only writable host directory added by the sandbox; system and package mounts are read-only. |
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
| `#runner/<you>@<realm>` | `<you>-ab-runner`, a system account of your runner's alone, unit `agent-bus-runner@<you>`, with your runner agent's token in `runner/runner.env` ([a person's runner](setup.md#a-persons-runner)) | you |
| every agent it serves | the same account | you |
| one message's child process | the same account, in that agent's work directory | — |

- **The runner is your agent and never you:** it refuses to start with a person's token.
- **It creates each agent it serves for you.** That `register` answers the new agent's token ([getting a token](../02-access.md#getting-a-token)), which the runner keeps in `runner/tokens/`.
- **It reads as that agent,** secret and inbox alike, so the answer leaves under the name that was addressed.
- **An agent that already exists is refused by name** until you hand its token over once.
- **Taking it over,** the runner clears the `script` an earlier `agent-bus run` left on the record, so listings no longer name a program that does not serve it.
- **Author settings** (`agent.json` and `env.dist`), including their packaged help, are read only from regular files; a final symlink is refused by filename. The package directory itself may be a link to a checkout; its target's writers then control the agent's code ([linked checkout](operations.md#install-a-custom-package)).
- **A linked checkout is the host owner's.** Only the host owner places the link; the runner never writes through it: `install --replace` swaps the link for a real directory, `remove` drops the link, and the checkout stays untouched in both. `reload` after a `git pull` there runs the new code. A link whose target is inside the runner's own directory (secrets, settings) or inside `agent.d` is refused by name and the package is listed as refused; so is a link when either of those directories cannot be resolved to check it.

- The Owner provisions the token in the runner’s private `tokens/` directory, named for the percent-encoded full agent name.
- Use mode `0600`, owned by the runner account, then reload. Keep tokens out of command arguments and transcripts.
- Follow the [credential handoff procedure](../agents/create-agent-prompt.md#self-serving-credential-handoff).

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
| its Maintainers | assigned record management |
| the daemon Owner | node-wide record authority |
| an Administrator | **no** — administering users grants no record authority ([authority](../01-identity-and-authority.md#record-authority)) |

- The runner holds each served agent’s token and can act as that agent, including its self-management rights. Record Owners and Maintainers retain their separate management and private-value rights ([authority](../01-identity-and-authority.md#record-authority)). Trust the runner account with those credentials.
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
| the record's [bus secret](../03-records-service.md#secrets) | the daemon | its Owner and Maintainers |
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
- Inspect values only through authorized host access at the `runner_dir` reported by status. Do not paste them into messages or logs.
- **Debug `env`** masks all values except the delivered `AGENT_BUS_*` and `AB_ROLE_*` namespaces.

What the logs mask:

- **Masked with `***`:** values of at least four bytes from the bus Secret and private host env files, plus a self-serving agent's own token.
- **Where:** child stderr copied to the journal, the unconfigured reason and `last_exit.reason`.
- **Who else can read stderr:** masked lines are also forwarded to the agent’s [private `#bus` log](../agents/bus.md#agent-logs). The agent’s Owner and Maintainers, and the daemon Owner, can read that log.
- **Not masked:** Configuration and package defaults stay readable.
- **Shorter private values** cannot be reliably masked as substrings; each load warns by key name without logging the value.
- **An agent's own replies** are its code's responsibility.
- **The digest covers the whole file:** compare the hash, not a secret store. It confirms what was written, but does not hide a tiny guessable value.

## Configuration controls

`agent.json` and `agents.json` refuse unknown fields by name. Only the keys in [package metadata](configuration.md#package-options) and [host entries](configuration.md#inventory) are accepted.

- **Network:** `needs.network` has the [effect described above](#isolation-today); host names do not restrict destinations.
- **Per-message scripts:** `processes` sets the maximum parallel copies; a minimum cannot keep an `args` or `json` script running between messages.
- **JSONL agents:** both ends of the `processes` range control the pool.
- **Self-serving agents:** the runner keeps the configured number of processes alive.
- **Versions:** generated help shows the package's `agent.json` version, or the shared bus version when omitted. Runner `status` reports the runner's version.

## Technical details

<details>
<summary>The cgroup and its limits</summary>

- **Kernel-enforced:** memory (no swap), CPU, processes and I/O weight.
- **Per process:** open files.
- **`/tmp`** is a tmpfs no larger than the memory limit.
- Choose values in [limits](configuration.md#limits); disk usage is a soft check, not a hard quota.

</details>

<details>
<summary>The system-call filter</summary>

- **Answered `EPERM`:** kernel modules, rebooting, mounting, new namespaces (by `unshare`, or by `clone` with a namespace flag), another process's memory, keys, clocks, BPF and perf, io_uring, swap and quotas.
- **`clone3`** is answered "not implemented", so threads are made with the filtered `clone` instead.
- **A call of another architecture** is refused whole.
- **The cost:** no `ptrace` and no kernel keyring inside the sandbox ([where the sandbox stops](#isolation-today)).

</details>
