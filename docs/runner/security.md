# What you grant the runner

📌 **TL;DR:** Every managed agent runs sandboxed unless the host turns that
off, in namespaces of its own, writing nowhere that lasts but its own work
directory. Resource limits are not enforced yet. The bus credential never reaches
the code, and who may call an agent is the record's
[ACL](../02-access.md#acl), set the ordinary way, not a runner setting.

## The short version

- **Sandboxed by default.** Own namespaces, the system read-only, its work
  directory and a private temporary directory the only writable paths, and no
  network unless the agent asked.
- **A host that cannot sandbox refuses the agent** rather than running it
  unconfined.
- **What it does not do yet:** bound memory, CPU or processes. `"limits"` is
  accepted and enforces nothing — see
  [what is accepted but inert](#what-is-accepted-but-inert).
- **`"sandbox": "off"` removes all of it.** The agent then has the runner
  account's whole reach; use it only for code you would run as that account.
- **The code is handed no credential.** `AGENT_BUS_TOKEN` is removed from the
  child's environment, so it cannot leak from there.
- **The first call from outside is refused** until the agent's ACL is widened,
  because a fresh registration admits only its Owner and Maintainers.

## Isolation today

**Every agent is sandboxed unless the host turns it off for that agent.** The
backend is bubblewrap, each agent in namespaces of its own. There is no
quiet fallback: a host without bubblewrap, or with unprivileged user
namespaces turned off, **refuses to start the agent** and says so by name.

What the sandbox gives a running script:

| | |
|---|---|
| Its own process view | it cannot see, signal or trace any other process, the runner's included |
| Its own filesystem view | the system's programs read-only and its own checkout read-only; it may write only to its work directory and to a private temporary directory that is gone when the run ends. No `/home`, no `/etc`, and nothing else of the host — so no other agent's `env` file, no other agent's code, and no path to the daemon's socket |
| No network | unless its `agent.json` asked for one |
| No capabilities, its own session | every capability dropped, and a terminal it cannot type into |
| Its own cgroup and limits | every process of the agent is started inside a cgroup of its own, under the runner's delegated subtree, never outside it even for a moment. Memory (no swap), CPU, processes and I/O weight are the kernel's to enforce, open files are each process's limit, and `/tmp` is a tmpfs no larger than the memory limit. Defaults: 512M of memory and 64 processes and 1024 open files; the host sets them per agent ([limits](configuration.md#limits)) |
| A need above its limits | refused by name: an agent whose `needs.memory` is over its memory limit is not started |
| A system-call filter | the calls an agent never needs and an escape often does are answered `EPERM`: kernel modules, rebooting, mounting, new namespaces (by `unshare`, or by `clone` with a namespace flag), another process's memory, keys, clocks, BPF and perf, io_uring, swap and quotas. `clone3` is answered "not implemented", so threads are made with the filtered `clone` instead. A call of another architecture is refused whole |
| A soft disk limit | after each run the work directory is measured, and an agent over its `limits.disk` (default 1G) is stopped and refused by name until it is emptied and reloaded |

The filter has a cost to know: an agent cannot `strace` or `gdb` its own
children (`ptrace`), and a program using the kernel keyring (`keyctl`, as some
Kerberos setups do) fails. Turn the sandbox off for such an agent only, and
read below what that gives up.

A runner that cannot get a cgroup subtree of its own (its unit's
`Delegate=yes`) **refuses every sandboxed agent**, by name, rather than run one
unbounded.

What it does **not** do yet:

| | |
|---|---|
| Named hosts | asking for the network gives the **whole** network: the host list in `needs.network` is not yet applied |
| A separate identity per agent | whatever it can still reach, it reaches as its runner's account, which is its person's alone |

**With `"sandbox": "off"` none of this applies.** That agent has the runner
account's full reach: every other agent's `env` file under `runner/`, every
checkout under `agent.d/`, and the account socket that owns every managed
agent. It is the right setting only for code you would run as that account
anyway.

Sandboxing arrived in 0.8.126, cgroups and limits in 0.8.150, the system-call
filter and the disk limit in 0.8.151. An older runner confines nothing, whatever
`agents.json` says — `status` reports the version it is, and the journal line
for each agent names the backend it got.

## What runs as what

| | Runs as | Owned on the bus by |
|---|---|---|
| `#runner/<you>@<host>` | `<you>-ab-runner`, a system account of your runner's alone, unit `agent-bus-runner@<you>`, with your runner agent's token in `runner/runner.env` ([a person's runner](../07-setup.md#a-persons-runner)) | you |
| every agent it serves | the same account | you |
| one message's child process | the same account, in that agent's work directory | — |

The runner is your agent and never you: it refuses to start with a person's
token. It creates each agent it serves for you; that `register` answers the new
agent's token ([getting a token](../02-access.md#getting-a-token)), which the
runner keeps in `runner/tokens/` and reads, secret and inbox alike, as that
agent, so the answer leaves under the name that was addressed. An agent that
already exists is refused by name until you hand its token over once:

```sh
agent-bus-token '#hello@host' | sudo -u you-ab-runner tee /var/lib/agent-bus/runners/you/runner/tokens/%23hello@host >/dev/null
agent-bus call '#runner/you@host' reload
```

## What the code is told, and what it may trust

The child gets the message as its argument (`form: "args"`) or as the envelope
on stdin, plus these variables:

| | |
|---|---|
| `AGENT_BUS_MESSAGE_ID`, `AGENT_BUS_FROM`, `AGENT_BUS_TO`, `AGENT_BUS_TOPIC`, `AGENT_BUS_TAG` | who and what this message is |
| `AGENT_BUS_WORK` | this agent's work directory, writable, where the child starts |
| `AB_ROLE_<NAME>=1` | one per [application role](../03-records-agent.md#roles) the sender holds toward this record |

Two rules about them:

- **The roles are a decision; the rest are labels.** The daemon works
  `AB_ROLE_*` out from the record's own allow list and overwrites anything the
  sender claimed, and the runner clears inherited ones before each run. Branch
  on a role. Do not branch on a string comparison against `AGENT_BUS_FROM`.
- **No credential comes with them.** `AGENT_BUS_TOKEN` is taken out, so a
  token cannot be read out of the environment, written to a log or passed to a
  child by accident. What keeps the agent away from the bus itself is the
  [sandbox](#isolation-today), not this.

An env file cannot forge any of this: the runner's own stamp is applied last,
so an `env` line setting `AGENT_BUS_FROM` loses to the real sender.

## Who may call an agent

**The record's ACL decides, exactly as for any other record.** The runner sets
no ACL when it creates an agent, and an empty one
[admits only the Owner and Maintainers](../02-access.md#acl) — which, here,
means you. An agent appears in `status` as `idle` and still refuses everybody
else until you widen it:

```sh
agent-bus manage '#hello@host' --add-allow alice@host
```

**Name the callers.** `--add-allow '*'` is broad access, and an agent reached
by anybody is one you have to trust against anybody.

| Who can run that | |
|---|---|
| the record's Owner | you |
| the daemon Owner | node-wide record authority |
| an Administrator | **no** — administering users grants no record authority ([authority](../01-identity-and-authority.md#record-authority)) |

The runner gains no authority over what it creates: it holds each agent's
token, and only you manage the record. It serves an agent only while that
token still answers as it and the agent is still yours: at every `reload`, an
agent you transferred to somebody else, or whose token was rotated out, is
stopped and refused by name, and its messages wait for whoever serves it now.

## Who may manage the runner

`#runner/<you>@<realm>` is yours, under the ACL you give it, and answers two
calls, `status` and `reload`. Anything else is refused by name. There is no
call that installs, enables or removes an agent — deploying over the bus is
planned and not built, so today every change is a change to files on the host.

## Secrets

Secrets live in the runner's own files and are injected into the process
before it starts, never as a path the code could open:

| Layer, lowest first | Where | Who writes it |
|---|---|---|
| declared variables and non-secret defaults | `agent.d/<agent>/env.dist` | the agent's author |
| the record's [bus secret](../03-records-service.md#secrets) | the daemon | the Owner, on the bus |
| what every instance of the agent shares | `runner/<agent>/env` | this host |
| this instance's own | `runner/<agent>/<instance>/env` | this host |

**Local wins over remote**, and an instance wins over its agent. A variable
that `env.dist` declares with no value, and that no layer supplies, refuses
the agent at `reload` by name rather than starting it half-configured.

Two file-mode facts worth knowing: `runner/` is `0700` and holds the secrets;
`agent.d/` is `0755`, a checkout anyone on the host may read. **Never put a
secret in `agent.d/`.** A sandboxed agent cannot reach either tree at all; one
with the sandbox off reaches both as the runner's account.

## What is accepted but inert

`agent.json` and `agents.json` are read strictly — an unknown field is refused
by name, so a typo cannot pass. But a *real* key for a feature that has not
shipped is accepted in silence and changes nothing:

| Key | Where | Belongs to |
|---|---|---|
| `limits` (`memory`, `cpu`, `processes`, `files`, `io_weight`) | `agents.json` | resource limits |
| `needs.memory`, `needs.writable` | `agent.json` | resource limits |
| `depends` | `agent.json` | the host's controls |
| `share` | `agents.json` | pools and edge hosts |
| `idle`, `in_flight` | `agent.json` | kept processes |
| `version` | `agent.json`, `agents.json` | reporting an agent's own version |
| `origin`, `first_started`, `last_started` | `agents.json` | recording where an agent came from and when it ran |

`version` is the one most likely to mislead: an agent may declare one, and
nothing reports it. The version in the `status` answer is the **runner's**.

One key is **half** in force, and it is the one worth knowing about:
`needs.network`. A non-empty list switches the agent's network on; the host
names in it are not applied, so the agent reaches everything, not those hosts.
`"sandbox"` is fully in force.

`processes` is half in force: its maximum bounds how many copies run at once,
its minimum does nothing, so `0..1` and `1` behave identically today.

<details>
<summary>What is refused by name instead</summary>

Unsupported *forms* behave better: `jsonl`, `msgpack` and `std` are refused by
name, as is `serves: "itself"`, each naming the work that will serve it. The
table above is the set that fails silently.

What each option is for is in
[agent configuration](../../Plans/R1.0-Release/runner-config.md#where-an-option-lives);
the order the features are planned in is the
[roadmap](../../Plans/R1.0-Release/TODO.md#runner-waves).

</details>
