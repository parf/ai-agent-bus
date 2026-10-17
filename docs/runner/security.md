# What you grant the runner

📌 **TL;DR:** Installing an agent means running its code as the one
`agent-bus-runner` account, with nothing separating it from the other agents
on that host — managed sandboxing is not built yet. The bus credential never
reaches the code. Who may call an agent is the record's
[ACL](../02-access.md#acl), set the ordinary way, not a runner setting.

## The short version

- **An installed agent runs as the runner's account.** Read its code the way
  you would read anything you are about to run as that user.
- **Nothing separates one agent from another today.** Same account, same home,
  no cgroup and no namespace: an agent can read its siblings' secrets.
- **`"sandbox"` and `"limits"` are accepted and do nothing.** See
  [what is accepted but inert](#what-is-accepted-but-inert) before relying on
  either.
- **The code is handed no credential.** `AGENT_BUS_TOKEN` is removed from the
  child's environment. That stops a credential leaking by accident; it is not
  a boundary, because the account's own socket is still reachable.
- **The first call from outside is refused** until the agent's ACL is widened,
  because a fresh registration admits only its Owner and Maintainers.

## Isolation today

**The managed runner does not sandbox anything yet.**
[Sandboxing](../../Plans/R1.0-Release/runner.md#sandboxing) is planned and not
built; [setup](../07-setup.md#the-two-accounts) says the same in one line.
Until it lands, an agent's process has exactly the privileges of the
`agent-bus-runner` account:

| Not in place | What follows |
|---|---|
| No per-agent cgroup | One agent's memory or CPU use is not bounded away from its siblings |
| No namespace | An agent can see, signal and trace its siblings' processes |
| No filesystem confinement | It can read every other agent's `env` file under `runner/`, and its checkout under `agent.d/` |
| No network confinement | It reaches whatever the host reaches, `needs.network` notwithstanding |
| Shared account | It can reach the runner's own socket, and so act as the User that owns every managed agent |

The foreground `agent-bus start` is the
[opposite case](../06-runner-role.md#sandboxing): sandboxing is off unless
asked for, but `--sandbox on` really sandboxes. Do not read one page's
behaviour into the other.

**So:** install code you trust. A host that serves agents from several
authors has no boundary between them today.

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
  child by accident. It is not containment: see
  [isolation today](#isolation-today).

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
secret in `agent.d/`.** And note the first section: with no sandbox, `0700`
protects the secrets from other users, not from other agents.

## What is accepted but inert

`agent.json` and `agents.json` are read strictly — an unknown field is refused
by name, so a typo cannot pass. But a *real* key for a feature that has not
shipped is accepted in silence and changes nothing:

| Key | Where | Belongs to |
|---|---|---|
| `sandbox` | `agents.json` | sandboxing |
| `limits` (`memory`, `cpu`, `processes`, `files`, `io_weight`) | `agents.json` | sandboxing |
| `needs` (`network`, `memory`, `writable`) | `agent.json` | sandboxing |
| `depends` | `agent.json` | the host's controls |
| `share` | `agents.json` | pools and edge hosts |
| `idle`, `in_flight` | `agent.json` | kept processes |

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
