# Dedicated runner

📌 **TL;DR:** The dedicated runner, `agent-bus run`, puts one script behind
one 👾 name: it stays in the foreground, reads that agent's queue and runs the
script for each message. It is dedicated to that one agent; installed agents,
pools, reload and restarts are the [runner](runner/README.md)'s.

This page owns `agent-bus run`. [Session launchers](06-launchers.md) put a live
AI session behind a name instead of a script.

## What it does

The dedicated runner, `agent-bus run`, stays in the foreground, registers an 👾
[agent](03-records.md#record-kinds) — the kind that has a
queue here — obtains its credential and reads that queue. It runs the script once per message. It serves one agent, the one it was
started for, and does not manage installed instances, autostart, reload or
restart policies: those are the [runner](runner/README.md)'s.

## Script agents

A script agent is a **script behind an 👾 agent record**: the name is an
agent because that is the kind with a queue to read
([records § record kinds](03-records.md#record-kinds)). A 📡
`service` is the external case, and nothing runs behind one here.

| Form | Input | Output |
|---|---|---|
| Default (`args`) | Message body as the script's argument | Stdout at exit |
| `--algo=json` | Envelope JSON on stdin; explicit alternative | Stdout at exit |

```sh
agent-bus run '#hello@srv1' "$PWD/hello-world.sh" --descr "greets you"
agent-bus run -5 - < agent.json
```

The agent itself may be JSON on stdin, read only when `run` is given `-`; its
fields are [the agent JSON](dedicated-runner/agent-json.md#fields).
With no arguments, `run` prints its usage and a link to the
[dedicated runner guide](user/dedicated-runner.md#-the-one-liner) on GitHub and exits non-zero; JSON
piped without `-` is refused.

| Rule | Built behavior |
|---|---|
| Script argument | One shell command line; quote it if it contains arguments. Use an absolute script path: the child starts in its work directory, so `run` refuses any relative path; a bare command on `PATH` is allowed. Within the command line, the executable path must be unquoted and contain no whitespace; its arguments may use shell quoting |
| The record's `script` | `run` writes the command line it serves into the Agent's `script` field each time it starts, so `agent-bus ls`, the MCP catalogue (`runs …`) and the Agent's web page say what is behind the name. It is informational: the daemon runs nothing and routes on nothing there, and only an 👾 carries one. A runner that takes the record over clears it ([hand-over](runner/security.md#what-runs-as-what)) |
| `-N` | At most N script processes; default 1. One dedicated runner reads one inbox |
| Success with output | Send the output as the answer |
| Silent success | Send `done` so the caller can stop waiting |
| Nonzero exit | Log failure and send no answer; no retry |
| Expired caller deadline | Skip the script; message TTL is enforced separately in the daemon |
| A receipt | Taken off the inbox and never runs the script: it is news about a message the agent sent, not work |
| Stop | Stop taking work and wait for scripts already running; unregister after a graceful stop when its inbox is idle, unless `--stay-registered` is set |
| Daemon away | A restart, a missing socket, a dropped forward or link, or a 502/503/504: the dedicated runner logs it once, shows `; away` in its process title and reads again with backoff, 1 s doubling to 30 s with jitter, until the daemon is back. A long poll with no answer 15 s past its wait counts as away. Just after an absence, a second-reader refusal is its own last poll and is waited out for one poll. Ctrl-c or `stop` ends the wait at once |
| Answer while away | A fast script sends only its answer (or done for quiet success). Work still outstanding after 2 s gets an ack, tried once for at most 2 s and never past the caller's deadline; an in-flight ack finishes before the answer. The answer or done is retried until the caller's deadline, or 2 min without one |
| Refusal | Any other refusal, such as a revoked token or a second reader, ends the dedicated runner with an error; the name stays. Restarting a dead script is the [runner](../Plans/R1.0-Release/runner.md#runner)'s |
| Work directory | One per agent; the script starts there |
| One run per name | A second `run` of a name already being started or run from this account is refused, even when both begin at once |
| Credentials | The launcher must be allowed to obtain that agent's credential; it cannot become somebody else's agent. The script never holds it: `AGENT_BUS_TOKEN` is taken out of the script's environment. On an account socket the dedicated runner or a [launcher](06-launchers.md#smart-launchers) stays there as its agent; a daemon before 0.8.63 moves it to the shared socket ([local socket](02-access.md#local-socket)) |
| Sharing | State `--allow name,...`, `--allow '@owner'` or `--allow '*'`; JSON uses `allow`. `@owner` admits the records the direct Owner owns. Fresh registrations use the [restricted default](02-access.md#acl); omitted settings on restart follow [registration rules](01-identity-and-authority.md#registration). Reply inboxes need their own grants |
| Tags | `--tags "work notes,claude"`, or the `tags` array in JSON passed to `agent-bus run -`, supplies explicit [record labels](03-records.md#tags). The dedicated runner adds no automatic tag |

### Keeping a supervised registration

`agent-bus run --stay-registered` leaves the agent record and credential in
place on a graceful stop. The default still unregisters an idle agent.
An external supervisor can therefore restart the agent with its own token,
without retaining a person's credential. The [standalone starter](../src/agents/standalone/starter.md#lifecycle)
uses this mode and requires the remover's current credential for explicit
unregistration. This flag controls lifecycle only; normal registration and
authorization checks still apply on start.

### Stopping it and reading what it said

The daemon does not know **where** a script agent runs: the registry holds a
description of it, not a handle to it. So a running agent leaves a note of
itself in **its owner's own state directory**, and the two verbs read that.
Which is also what makes *only whoever started it may stop it* true with no
check in it — nobody else can see the file.

| | |
|---|---|
| `agent-bus stop <name>` | `SIGTERM`, and then it **waits**. Stopping is graceful, so the verb does not report a stop that has not finished, and it ends one agent without touching its siblings |
| `agent-bus logs <name> [--lines N] [--follow]` | what that agent and its scripts wrote. The log outlives the run on purpose: what a run said is most wanted once the run has ended |
| a note with no process behind it | is cleared, not reported as running — an agent killed outright leaves its note behind |
| starting a name that is already running here | is refused; the dedicated runner has no shared-start mode |

`run`, `stop` and `logs` accept a script agent name with or without its
leading `#`; all three refer to the same local run.

Because a message is taken from the daemon only when a script process is free
to run it, an agent that dies loses only the work already in flight; the rest
is still queued for whatever reads that inbox next.

A forced kill or an error exit leaves the registration in place; remove an idle address with
[unregister](01-identity-and-authority.md#unregistering).

## What the child is told

The script receives message id, sender, receiver, topic, tag and its work path
through environment variables. Their exact names are owned by
[its source](../src/cmd/agent-bus/run.go). These labels describe the work;
`AB_ROLE_<NAME>=1` separately carries each role the bus granted toward this
agent. Use those grants to guard methods, never a sender-name comparison
([roles](03-records-agent.md#roles)). The [runner](runner/README.md)'s agents get
[configuration layers](../Plans/R1.0-Release/runner-implementation.md#the-three-env-layers) as well.

Keep secrets in Secret and config in Configuration ([rule](03-records-agent.md#configuring-a-template)).
Configuration reaches each script as `AGENT_BUS_CONFIG`: compact JSON, unset
when unconfigured. It is refreshed at the next invocation; see
[configuration and jq](03-records-agent.md#configuring-a-template).

The record's Secret (`KEY=value` lines, [env-file syntax](03-records-service.md#env-file-syntax))
reaches each script as environment variables too, read again for every message,
so `agent-bus secret` takes effect without a restart, sandboxed or not. It wins
over the shell's value of the same name; the message's own `AGENT_BUS_*` fields
win over it, and the credential is never set by it. A value never appears on
any command line, and `agent-bus logs` masks a script line holding a value, or one
line of a multi-line value, of four bytes or more. A value or line under four
bytes (`PIN=123`, a trailing `w==`) is not masked, so keep such a secret out of
what a script prints.

Without `--package-dir`, asks reach the script as `AGENT_BUS_ASK`:
`help`, `help-config`, `help-secret`, `help-json` or `status`, with an empty
body. Packaged metadata answers what it can; `help: native` delegates help to
the script. Every remaining ask follows the same path. An unpackaged script has no MCP tools; the runner answers `mcp` with `{"tools":[]}`. To offer tools, use a package with `help: native`. See
[package help](runner/configuration.md#package-help).

Use `--no-asks` (JSON `no_asks: true`), or `"asks":"none"` in the package,
for an unchanged shell script: metadata still answers, and everything else
replies `unsupported: <ask>` without starting it. A script that handles asks
checks them before any ordinary work:

```sh
case "$AGENT_BUS_ASK" in
  "") ;;
  help) echo "Answer a ping: send ping"; exit 0 ;;
  *) echo "unsupported: $AGENT_BUS_ASK"; exit 0 ;;
esac
# Only ordinary input reaches the program below.
```

Known shutdown/failure status is answered by the runner while its reader is
attached; otherwise a running script receives status normally.

## Who it runs as

The dedicated runner executes as the user who started it. It is not a child of
the daemon. Setup installs a person's separate [runner](runner/README.md) and its
unit only on request ([a person's runner](07-setup.md#a-persons-runner)).

## Sandboxing

The dedicated runner's sandbox is off by default; `--sandbox on|off` selects it explicitly. The built backend is
`systemd-run --user` behind the sandbox port. Asking for a sandbox without a
working user manager and support for `--expand-environment=no` fails rather
than silently running unsandboxed. Startup reports the selected backend or the
probe's failure diagnostic.

| Profile | Behavior |
|---|---|
| Filesystem | `ProtectSystem=strict`, home read-only, private temporary directory |
| Work | Work directory bound in and writable; script directory bound read-only |
| Network | Private network unless explicitly requested |
| Privilege and resources | No new privileges; process and memory limits |

The exact invocation is in [sandbox source](../src/internal/sandbox/sandbox.go).
The built backend is tested with positive and negative controls: a script can
write inside its work directory and cannot write outside it or use an ungranted
network.

## Status

| MVP | Scope |
|---|---|
| Built | Script agents, bounded parallel execution, graceful stop, logs, optional systemd sandbox. |
