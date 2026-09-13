# Processes and privileges

## Status

| MVP | Scope |
|---|---|
| Built | Supervisor, bus child, optional web child, inherited listeners and versioned process titles. |
| Pending | Installed capability checks and dashboard resource confinement; see [MVP gates](../Plans/MVP/TODO.md#installed-stage-gate). |

## The rule

The supervisor owns listeners and child lifetime. The bus owns state and
serves requests. The web child requests the caller's view from the bus.
Passing a listener is a runtime boundary; the [module rules](10-modules.md#the-rule)
remain separate.

## The processes

| Built process | Owns | Runtime |
|---|---|---|
| Supervisor | Listening sockets, socket ownership and child restart | `agent-busd` |
| Bus | Registry, queues, tokens and browser sessions | Same executable, bus role |
| Web | HTTP rendering, no credential store | `agent-bus-web`, enabled with `-web` |
| Foreground script runner | Its service's inbox reader and script children | User-launched CLI; outside the daemon |

The web child is optional and not started by default. AUTH and health are not
implemented children.

## Process titles

All programs use the shared [working rules § versioning](../CLAUDE.md#versioning).
Every Go program accepts `--version` (also `-version`) and prints the version
and [setup § build information](09-setup.md#build-information) before doing
any work. The MCP entry point accepts the same flag via
`bun run server.ts`; its handshake and the Codex adapter report that version too.

| Process | Title in `ps -ww -o args` | Calls |
|---|---|---|
| Supervisor | `agent-busd <version> ; supervisor` | No counter; it serves no requests |
| Bus child | `agent-busd <version> ; Calls: <count> ; bus` | HTTP requests received across every listener, including refusals and requests still in progress |
| Foreground script runner | `agent-bus-runner <version> ; Calls: <count> ; <service>` | Inbox messages taken, including work that later fails or is skipped |

Counts are cumulative for the process lifetime, start at zero, and reset on
restart. Titles are set at start and refreshed once a second; the request path
only increments an atomic counter. Idle polling does not count as runner work.
Scripts, arguments, credentials and message bodies are never added to titles.
The executable's short `comm` name stays unchanged.

The Go helper wraps `gspt`, as in the reference Go API: it rewrites the OS
argument area while preserving Go's arguments and environment for child exec;
build requirements live in [setup § build information](09-setup.md#build-information).

## Nothing the daemon runs may exec

This is the target privilege boundary for bus and web: they must not execute
user services. The supervisor necessarily executes its children. User scripts
run in the separate foreground runner.

**Limitation:** the current bus invokes the signature adapter's `ssh-keygen`
for enrolment. A literal no-exec confinement claim is therefore not met. The
resolution belongs to the [MVP privilege gate](../Plans/MVP/QUESTIONS.md#open-questions),
not to an assertion that an unavailable future AUTH child already does it.

## Why the supervisor holds CAP_CHOWN

Per-account socket ownership needs this capability. The installed unit grants
it only to the supervisor; the supervisor clears ambient capabilities before
starting its children. A developer run without the capability cannot prove
that boundary. Read the actual process capabilities under the installed unit.

The supervisor reports a failed ownership change. That does not certify that
another account can use the intended socket.

## What is shared

Inherited listener descriptors and explicit API calls. The bus owns the store
and snapshot; the supervisor does not keep a store handle. The web child gets
the shared socket and no principal token. It forwards each visitor's credentials
and then uses their browser session.

## How a child is started

One binary. The supervisor opens every listener before any child exists, so a
child cannot make one and does not need the capability to.

| | |
|---|---|
| `AGENT_BUS_ROLE=bus` | which role this process is. Absent means supervisor |
| `AGENT_BUS_FDS` | what arrives at fd 3 upwards, in order: `tcp`, `shared`, `user:<principal>` — the whole contract between the two |
| `-web` | the dashboard runs as a child too ([discovery § where it listens](05-discovery.md#where-it-listens)), reaching the bus over the shared socket and forwarding the visitor's credentials |

A child that dies is restarted with backoff, and the listeners are passed to
the replacement — the socket a client holds is the same file across a restart.
A supervisor that is killed outright takes its children with it
(`PR_SET_PDEATHSIG`), so nothing orphaned keeps a port.
