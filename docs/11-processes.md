# Processes and privileges

📌 **TL;DR:** The supervisor owns listeners and child lifetime, the bus owns
state and serves requests, and the web child asks the bus for the visitor's
own view. Passing a listener is a runtime boundary, separate from the module
rules. Nothing the daemon runs may exec.

## Status

| MVP | Scope |
|---|---|
| Built | Supervisor, bus child, optional confined web child, inherited listeners and versioned process titles. Installed account, socket and capability placement is accepted on a package-only real-systemd host. |

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
| Foreground script runner | Its agent's inbox reader and script children | User-launched CLI; outside the daemon |

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
| Foreground script runner | `agent-bus-runner <version> ; Calls: <count> ; <name>` | Inbox messages taken, including work that later fails or is skipped |

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
user programs. The supervisor necessarily executes its children. User scripts
run in the separate foreground runner.

**`ssh-keygen` is allowed, and the boundary is about user programs.** The bus
runs it to check a signature at enrolment, and that stays. What the rule forbids
is executing **what a user supplied** — a script behind an agent, anything whose
contents somebody else chose. A fixed verifier the daemon ships and invokes with
arguments it built is a different thing from running a stranger's program, and
collapsing the two would have bought nothing but a literal claim.

So the boundary is not *no exec*, it is **no user programs**, and the check is
what decides the argument rather than how the code is reached. An unavailable
future AUTH child is not what settles this; the distinction is.

## Why the supervisor holds CAP_CHOWN

Per-account socket ownership needs this capability. The installed unit grants
it only to the supervisor; the supervisor clears ambient capabilities before
starting its children. A developer run without the capability cannot prove
that boundary. The [installed checks](../Plans/MVP/done/installed-shared-host.md#checks)
exercise two actual account sockets and mutations that remove the grant or let
the bus inherit it.

The supervisor reports a failed ownership change. That does not certify that
another account can use the intended socket.

## What is shared

Inherited listener descriptors and explicit API calls. The bus owns the store
and snapshot; the supervisor does not keep a store handle. The web child gets
the shared socket and no principal token. It forwards each visitor's credentials
and then uses their browser session.

## Web authority boundary

The supervised web child cannot read or modify
daemon credentials, snapshots or SSH authorization, or use mapped account
sockets to acquire authority independently of its visitor. Normal calls use
the shared listener and the visitor's credential or browser session.

**The panel acts on the visitor's token, and on nothing else.** Owner-settled,
2026-09-16. Every call a signed-in person causes is made with that person's
credential, so what the dashboard can do is exactly what that person could do
from the CLI. It is a face, not an authority.

| | |
|---|---|
| **The node's own identity is not authority** | release, build, host name, daemon owner, uptime and calls served are published to anybody, signed in or not ([what a node says about itself](05-discovery.md#what-a-node-says-about-itself)). The daemon answers that closed list without a credential, so the face reads it as anybody does — it is not the face acquiring privilege, and nothing else is readable that way |
| **No privileged fallback** | a call refused for the visitor is refused. The panel does not retry as anybody else, because it is started without a credential of its own ([how a child is started](#how-a-child-is-started)) and there is nobody else for it to be |
| **Nothing outlives the session** | authority arrives with the request and leaves with it. The session lives in the bus, not the child ([signing in](05-discovery.md#signing-in)), so a panel that is not serving a signed-in request is holding no authority at all |
| **Authority is rendered, not computed** | the daemon already answers per caller — a record comes back saying whether *this* caller may manage or transfer it ([ACL](02-access.md#acl)). The panel shows what it was told rather than working it out. A face that derives permissions itself is a second implementation of the access rules, and two implementations disagree; the disagreement that matters is the one where the page offers an action the daemon will refuse |

The supervisor uses bubblewrap to expose individual inputs in an otherwise
empty filesystem. Sandbox setup failure never falls back to an unconfined web
child. A standalone invocation of `agent-bus-web` does not establish this boundary.

<details>
<summary>Filesystem and process confinement</summary>

| Input or boundary | Supervised web |
|---|---|
| Executable | Static web binary, read-only; no host library directories |
| API | Shared socket only; mapped account sockets are absent |
| TLS | Only explicitly configured certificate/key files, read-only; a missing requested file prevents startup |
| Name resolution | Read-only `/etc/hosts` and `/etc/resolv.conf`, when present |
| Processes | Private PID namespace and its own `/proc`; host process roots and descriptors are not exposed |
| Other files | Private temporary filesystem and minimal `/dev`; no daemon state, home or SSH directory |
| Environment | Only the shared API address and explicit web listen/TLS settings; inherited credentials and unrelated variables are discarded |
| Privilege | New user/IPC/UTS namespaces, new session, no capabilities; host networking retained for the HTTP listener |

The earlier [same-account exposure](../Plans/MVP/done/release-gap-review.md#findings)
is why application wiring alone was insufficient. This does not provide a
network firewall. It also does
not hide credentials a visitor intentionally supplies to the web process.
Read the [installed acceptance](../Plans/MVP/done/web-isolation.md#checks) for
the exercised host policy and mutation limits.

</details>

## Web resource limits

The supervised dashboard and every process inside its wrapper share a cgroup
limited to 256 MiB memory, no swap, 64 tasks and one CPU. The supervisor and
bus remain outside that cgroup. Reaching a limit may kill the web child; the
supervisor logs the failure, applies its normal restart backoff and starts a
new renderer under the same limits. Bus calls continue while web is absent.

<details>
<summary>Delegation, startup and recovery</summary>

The generated system unit delegates only the CPU, memory and task controllers
and starts the supervisor in its own subgroup. The supervisor verifies that
exact construction, enables the controllers and uses `clone3` cgroup placement
to put bubblewrap inside the limited group before it executes. There is no
unlimited interval before later migration.

Missing delegation keeps web down and logs the required
`Delegate=cpu memory pids` and `DelegateSubgroup=supervisor` setup; it does not
stop the bus or search parent cgroups for another writable directory. An
ordinary development run needs an explicit delegated transient user unit and
`AGENT_BUS_WEB_USER_DELEGATION=1`; the installed system unit uses systemd's
delegation marker instead. That environment value is trusted launcher input
only for the xattr distinction: subgroup name, ownership, controllers and
writability are still checked, so a false claim leaves web down rather than
running it without limits.

The same cgroup is emptied and reused after a renderer failure. Cleanup waits
at most two seconds; a non-empty group fails closed instead of allocating
another directory. Supervisor shutdown kills remaining descendants and removes
the group. [Installed pressure evidence](../Plans/MVP/done/web-resources.md#checks)
records CPU throttling, task refusal, group OOM and renderer recovery while the
bus keeps answering.

</details>

## How a child is started

One binary. The supervisor opens every listener before any child exists, so a
child cannot make one and does not need the capability to.

| | |
|---|---|
| `AGENT_BUS_ROLE=bus` | which role this process is. Absent means supervisor |
| `AGENT_BUS_FDS` | what arrives at fd 3 upwards, in order: `tcp`, `shared`, `user:<principal>` — the whole contract between the two |
| `AGENT_BUS_ACCOUNTS` | the supervisor's active editable account map; the bus compares it with durable desired state to report whether a full restart is required |
| `-owner` | required first-run Owner seed; the daemon account's socket answers as the durable daemon Owner, which a transfer moves ([setup upgrade](09-setup.md#daemon-ownership-upgrade)) |
| `-web` | the dashboard runs as a child too ([discovery § where it listens](05-discovery.md#where-it-listens)), reaching the bus over the shared socket and forwarding the visitor's credentials |

The bus child inherits the supervisor's environment. The web wrapper starts
with an empty environment and receives only the explicit inputs listed in
the [web boundary](#web-authority-boundary), never a principal token.

A child that dies is restarted with backoff, and the listeners are passed to
the replacement — the socket a client holds is the same file across a restart.
A supervisor that is killed outright takes its children with it
(`PR_SET_PDEATHSIG`), so nothing orphaned keeps a port.
