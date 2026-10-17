# What you grant the runner

📌 **TL;DR:** Every managed agent runs sandboxed unless the host turns that
off, in namespaces of its own with its work directory the only place it may
write. Resource limits are not enforced yet. The bus credential never reaches
the code, and who may call an agent is the record's
[ACL](../02-access.md#acl), set the ordinary way, not a runner setting.

## The short version

- **Sandboxed by default.** Own namespaces, the system read-only, the work
  directory the only writable path, and no network unless the agent asked.
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
| Its own filesystem view | the system's programs read-only, its own checkout read-only, its work directory the only writable place, a private `/tmp`. No `/home`, no `/etc`, and nothing else of the host — so no other agent's `env` file, no other agent's code, and no path to the daemon's socket |
| No network | unless its `agent.json` asked for one |
| No capabilities, its own session | every capability dropped, and a terminal it cannot type into |

What it does **not** do yet:

| | |
|---|---|
| Resource limits | memory, CPU, processes, open files and disk I/O are not bounded; one agent can still starve the host |
| System-call filter | not applied |
| Named hosts | asking for the network gives the **whole** network: the host list in `needs.network` is not yet applied |
| A separate identity | whatever it can still reach, it reaches as the runner's account |

**With `"sandbox": "off"` none of this applies.** That agent has the runner
account's full reach: every other agent's `env` file under `runner/`, every
checkout under `agent.d/`, and the account socket that owns every managed
agent. It is the right setting only for code you would run as that account
anyway.

Sandboxing arrived in 0.8.126. An older runner confines nothing, whatever
`agents.json` says — `status` reports the version it is, and the journal line
for each agent names the backend it got.

## What runs as what

| | Runs as | Owned on the bus by |
|---|---|---|
| `#runner@<host>` | the `agent-bus-runner` system account, its own unit | the keyless User `agent-bus-runner` |
| every agent it serves | the same account | the same User |
| one message's child process | the same account, in that agent's work directory | — |

The runner registers each agent, takes that agent's own token, and
[forwards](../04-messaging.md#deliver-to) it to `#runner`, so the answer
leaves under the name that was addressed. The token stays in the runner.

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
no ACL when it registers an agent, and an empty one
[admits only the Owner and Maintainers](../02-access.md#acl) — which, here,
means the runner's own account. An agent appears in `status` as `idle` and
still refuses everybody until somebody widens it:

```sh
agent-bus manage '#hello@host' --add-allow alice@host --add-maintainers you
```

**Name the callers.** `--add-allow '*'` is broad access, and an agent reached
by anybody is one you have to trust against anybody.

| Who can run that | |
|---|---|
| the record's Owner | the `agent-bus-runner` account, through its own socket |
| the daemon Owner | node-wide record authority |
| an Administrator | **no** — administering users grants no record authority ([authority](../01-identity-and-authority.md#record-authority)) |

Adding yourself as a Maintainer at the same time is what lets you edit the
record afterwards without going back through the runner's account. Having the
runner register agents its deploying user keeps is
[Q164](../../Plans/R1.0-Release/QUESTIONS.md#open-questions), still open.

## Who may manage the runner

`#runner@<host>` is registered with `@owner, @administrators`, and answers two
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
[roadmap](../../Plans/R1.0-Release/TODO.md#managed-runner-waves).

</details>
