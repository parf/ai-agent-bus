# What you grant the runner

📌 **TL;DR:** Every managed agent runs sandboxed unless the host turns that
off, in namespaces of its own, writing nowhere that lasts but its own work
directory, in a cgroup with its resource limits and under a system-call filter.
A script is handed no bus credential, and who may call an agent is the record's
[ACL](../02-access.md#acl), set the ordinary way, not a runner setting.

## The short version

- **Sandboxed by default.** Own namespaces, the system read-only, its work
  directory and a private temporary directory the only writable paths, and no
  network unless the agent asked.
- **A host that cannot sandbox refuses the agent** rather than running it
  unconfined.
- **Bounded:** memory, CPU, processes, open files and disk, by its cgroup and
  limits ([isolation today](#isolation-today)); keys not yet in force are listed
  in [what is accepted but inert](#what-is-accepted-but-inert).
- **`"sandbox": "off"` removes all of it.** The agent then has the runner
  account's whole reach; use it only for code you would run as that account.
- **A script is handed no credential.** `AGENT_BUS_TOKEN` is removed from the
  child's environment, so it cannot leak from there. An agent that
  [serves itself](configuration.md#agents-that-serve-themselves) is the exception:
  it is handed its own token, and only its own, since reading its inbox is its job.
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
| A soft disk limit | after each message run and before a self-serving agent restarts, the work directory is measured; an agent over its `limits.disk` (default 1G) is stopped and refused by name until it is emptied and reloaded. A running self-serving process is not measured on a timer |

Work directories must not be symlinks. Keys whose work paths would contain one
another cannot be served together. A sandboxed agent that serves itself is
refused on an account socket: mounting it would also give the agent that
account's tokenless identity. Use the shared socket with the agent's token.

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
agent-bus-token '#hello@host' | sudo -u you-ab-runner tee /var/lib/agent-bus/you-ab-runner/tokens/%23hello@host >/dev/null
agent-bus call '#runner/you@host' reload
```

Author settings (`agent.json` and `env.dist`), including their packaged help,
are read only from regular files; a final symlink is refused by filename.

## What the code is told, and what it may trust

The child gets the message as its argument (`form: "args"`) or as the envelope
on stdin, plus these variables:

| | |
|---|---|
| `AGENT_BUS_MESSAGE_ID`, `AGENT_BUS_FROM`, `AGENT_BUS_TO`, `AGENT_BUS_TOPIC`, `AGENT_BUS_TAG` | who and what this message is |
| `AGENT_BUS_WORK` | this agent's work directory, writable, where the child starts |
| `AGENT_BUS_CONFIG` | compact stored JSON at this start; unset when unconfigured ([configuration](../03-records-agent.md#configuring-a-template)) |
| `AGENT_BUS_ASK` | Empty for a generic packaged script; with `help: native`, `mcp`, `help`, `help-config`, `help-secret` or `help-json` ([package help](configuration.md#package-help)) |
| `AB_ROLE_<NAME>=1` | one per [delivered role](../03-records-agent.md#roles), including `owner` and `maintainer`, the sender holds toward this record |

Two rules about them:

- **The roles are a decision; the rest are labels.** The daemon works
  `AB_ROLE_*` out from the record's own allow list and overwrites anything the
  sender claimed, and the runner clears inherited ones before each run. Branch
  on a role. Do not branch on a string comparison against `AGENT_BUS_FROM`.
- **No credential comes with them.** `AGENT_BUS_TOKEN` is taken out, so a
  token cannot be read out of the environment, written to a log or passed to a
  child by accident. The one exception is an agent that
  [serves itself](configuration.md#agents-that-serve-themselves): it is handed
  its own token, since talking to the bus is its job, and still never yours. What keeps the agent away from the bus itself is the
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
token, and only you manage the record. At start and `reload`, it checks that
the token answers as that agent and that the agent is still yours. A transferred
agent or rotated-out token is then stopped and refused by name; its messages
wait for whoever serves it now. An admitted operation does not re-ask identity
or authority midway ([authority at the door](../constitution.md#authority-at-the-door)).

## Who may manage the runner

`#runner/<you>@<realm>` is yours, under the ACL you give it, and answers the
calls of [operations](operations.md#controlling-one-agent): status, reload, the
controls, and reading and writing the host's files. Anything else is refused by
name. `status` is for whoever that ACL admits; **reload, controls and
the host's files are only for the runner's Owner and Maintainers**, as the daemon
works the sender's roles out toward the runner itself
([roles](../03-records-agent.md#roles)). A Maintainer can replace host env files but reads only their digest and variable names through the runner. Grant Maintainer only to someone trusted to change how its agents run. The calls never name the kept tokens or
an agent's work. Installing an
agent's code over the bus is planned and not built.

## Secrets

Keep secrets in Secret and config in Configuration; see [configuration](../03-records-agent.md#configuring-a-template).

There are **two places to keep a secret**, and they never mix:

| Store | Where it lives | Who can read it | Leaves the host? |
|---|---|---|---|
| The record's [bus Secret](../03-records-service.md#secrets) | the daemon's database | the record's Owner and Maintainers, and the agent itself, over the bus | yes: shared through the bus with whoever may read it |
| The runner's host files (`runner/<agent>/env`, `runner/<agent>/<instance>/env`) | this host's `runner/` tree, mode `0700`; the daemon never stores them | the runner's account, to start agents; viewing values requires host access | **never leave the host through the runner API**: only existence, SHA-256, size and variable names are returned; the daemon never stores the files |

Use the bus Secret for a credential several hosts or people must share. Use the
runner's host files for one that must stay on this machine. The runner merges
both into the agent's environment at start; **the runner's own secrets take
over** a bus Secret key of the same name.

The web editor starts empty and saving replaces the entire host env file. To
inspect values, use host access, for example `sudo cat /var/lib/agent-bus/parf-ab-runner/runner/slack/env`;
the runner’s status reports its actual `runner_dir`. Debug `env` masks all
values except the delivered `AGENT_BUS_*` and `AB_ROLE_*` namespaces.

Secrets are injected into the process before it starts, never as a path the
code could open:

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
| `needs.writable` | `agent.json` | writable paths beyond the work directory |
| `depends` | `agent.json` | the host's controls |
| `share` | `agents.json` | pools and edge hosts |
| `idle`, `in_flight` | `agent.json` | kept processes |
| `version` | `agents.json` | host-side package version pin |
| `origin`, `first_started`, `last_started` | `agents.json` | recording where an agent came from and when it ran |

The package's `agent.json` version is shown by generated help (the shared bus
version when omitted). The version in runner `status` is the **runner's**.

One key is **half** in force, and it is the one worth knowing about:
`needs.network`. A non-empty list switches the agent's network on; the host
names in it are not applied, so the agent reaches everything, not those hosts.
`"sandbox"` is fully in force.

`processes` is half in force for a script: its maximum bounds how many copies
run at once, its minimum does nothing, so `0..1` and `1` behave identically. An
agent that serves itself is kept at its maximum, restarted when one exits.

<details>
<summary>What is refused by name instead</summary>

Unsupported *forms* behave better: `jsonl`, `msgpack` and `std` are refused by
name, each naming the work that will serve it. The
table above is the set that fails silently.

What each option is for is in
[agent configuration](../../Plans/R1.0-Release/runner-config.md#where-an-option-lives);
the order the features are planned in is the
[roadmap](../../Plans/R1.0-Release/TODO.md#runner-waves).

</details>

The runner replaces values of at least four bytes from the bus Secret and private host env files, plus a self-serving agent’s own token, with `***` in child stderr copied to its journal, unconfigured reason and `last_exit.reason`. Configuration and package defaults remain readable. Shorter nonempty private values cannot be reliably masked as substrings; each load warns by key name without logging its value. An agent’s own replies are its code’s responsibility.

The digest covers the whole file: compare the hash, not a secret store. It confirms what was written, but does not hide a tiny guessable value.
