# Processes and privileges

📌 **TL;DR:** The supervisor owns listeners and child lifetime, the bus owns
state and serves requests, and the web face, under its own account and unit,
asks the bus for the visitor's own view. Passing a listener is a runtime
boundary, separate from the module rules. Nothing the daemon runs may exec.

## Status

| MVP | Scope |
|---|---|
| Built | Supervisor, bus child, inherited listeners and versioned process titles; the [web face](#the-web-face) under its own account and unit from 0.8.50. Installed account, socket and capability placement is accepted on a package-only real-systemd host. |

## The rule

The supervisor owns listeners and child lifetime. The bus owns state and
serves requests. The web face requests the caller's view from the bus.
Passing a listener is a runtime boundary; the [module rules](10-modules.md#the-rule)
remain separate.

## The processes

| Built process | Owns | Runtime |
|---|---|---|
| Supervisor | Listening sockets, socket ownership and child restart | `agent-busd` |
| Bus | Registry, queues, tokens and browser sessions | Same executable, bus role |
| Web face | HTTP rendering, no credential store | `agent-bus-web.service`: bun, as the `agent-bus-web` account ([the web face](#the-web-face)) |
| Foreground script runner | Its agent's inbox reader and script children | User-launched CLI; outside the daemon |

Only the bus is a child of the supervisor. AUTH and health are not
implemented children.

## Process titles

All programs use the shared [working rules § versioning](../CLAUDE.md#versioning).
Every Go program accepts `--version` (also `-version`) and prints the version
and [setup § build information](09-setup.md#build-information) before doing
any work. The MCP and web entry points accept the same flag via
`bun run server.ts`; the MCP handshake and the Codex adapter report that version too.

| Process | Title in `ps -ww -o args` | Calls |
|---|---|---|
| Supervisor | `agent-busd <version> ; supervisor` | No counter; it serves no requests |
| Bus child | `agent-busd <version> ; Calls: <count> ; bus` | HTTP requests received across every listener, including refusals and requests still in progress |
| Foreground script runner | `agent-bus-runner <version> ; Calls: <count> ; <name>` | Inbox messages taken, including work that later fails or is skipped |
| Web face | `agent-bus-web <version> ; Calls: <count>` | HTTP requests received, refusals, assets and `/healthz` included; written over bun's own argv, so a title longer than the unit's command line is cut |

Counts are cumulative for the process lifetime, start at zero, and reset on
restart. Titles are set at start and refreshed once a second; the request path
only increments an atomic counter. Idle polling does not count as runner work.
Scripts, arguments, credentials and message bodies are never added to titles.
The executable's short `comm` name stays unchanged.

The Go helper wraps `gspt`, as in the reference Go API: it rewrites the OS
argument area while preserving Go's arguments and environment for child exec;
build requirements live in [setup § build information](09-setup.md#build-information).

## Nothing the daemon runs may exec

This is the target privilege boundary for the bus: it must not execute user
programs. The supervisor necessarily executes its children, and the web face's
unit can execute nothing but bun. User scripts run in the separate foreground
runner.

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
that boundary. The [installed checks](../Plans/R0.8/done/installed-shared-host.md#checks)
exercise two actual account sockets and mutations that remove the grant or let
the bus inherit it.

The supervisor reports a failed ownership change. That does not certify that
another account can use the intended socket.

## What is shared

Inherited listener descriptors and explicit API calls. The bus owns the store
and snapshot; the supervisor does not keep a store handle. The web face, a
separate unit, gets the shared socket and no principal token. It forwards each
visitor's credentials and then uses their browser session.

## The web face

The web face is TypeScript in `src/web`, run from source by the system bun
under its own system account and its own systemd unit. It is not a daemon
child: the supervisor starts nothing for it. It reaches the daemon only over
the shared socket, with each visitor's session, and holds no credential,
state or writable path of its own.

| | |
|---|---|
| Account | `agent-bus-web`: system account, `nologin`, home `/var/lib/agent-bus/web`, no SSH keys, not in the account map; owns nothing on disk ([setup § the two accounts](09-setup.md#the-two-accounts)) |
| Code | `/var/lib/agent-bus/web`, a link to the current release's `web/` directory, or to a checkout's `src/web` in development |
| Runtime | `/usr/bin/bun run /var/lib/agent-bus/web/server.ts`, no build step |
| Unit | `agent-bus-web.service`, shipped as `src/web/agent-bus-web.service`; setup writes it with the host's exec paths |
| Daemon link | `/run/agent-bus/bus.sock` only; mapped account sockets are closed to it by their mode |
| Listen | [discovery § where it listens](05-discovery.md#where-it-listens) |

**The face acts on the visitor's session, and on nothing else.** Owner-settled,
2026-09-16. Every call a signed-in person causes is made with that person's
credential, so what the face can do is exactly what that person could do from
the CLI. It is a face, not an authority.

| | |
|---|---|
| **The node's own identity is not authority** | release, build, host name, daemon owner, uptime and calls served are published to anybody, signed in or not ([what a node says about itself](05-discovery.md#what-a-node-says-about-itself)). The daemon answers that closed list without a credential, so the face reads it as anybody does — it is not the face acquiring privilege, and nothing else is readable that way |
| **No privileged fallback** | a call refused for the visitor is refused. The face does not retry as anybody else, because its unit gives it no credential of its own and there is nobody else for it to be |
| **Nothing outlives the session** | authority arrives with the request and leaves with it. The session lives in the bus, not the face ([signing in](05-discovery.md#signing-in)), so a face that is not serving a signed-in request holds no authority at all |
| **Authority is rendered, not computed** | the daemon already answers per caller — a record comes back saying whether *this* caller may manage or transfer it ([ACL](02-access.md#acl)). The face shows what it was told rather than working it out. A face that derives permissions itself is a second implementation of the access rules, and two implementations disagree; the disagreement that matters is the one where the page offers an action the daemon will refuse |

<details>
<summary>The unit's walls</summary>

| Area | What the unit allows |
|---|---|
| Privilege | `NoNewPrivileges`, no capabilities, `PrivateUsers`, restricted namespaces, SUID/SGID and realtime |
| Filesystem | `ProtectSystem=strict`, `ProtectHome`, private `/tmp` and devices; the daemon, runner and `service.d` state, `/root` and `/etc/ssh` are inaccessible |
| Exec | `NoExecPaths=/`; `ExecPaths` is bun and the libraries it links, computed per host by setup from `ldd /usr/bin/bun` |
| Network | `IPAddressDeny=any` with `IPAddressAllow=localhost`: it answers on loopback and connects out to nothing |
| System calls and kernel | `@system-service` minus privileged, resource, mount and debug sets; kernel tunables, modules, logs, cgroups, clock and hostname protected; `/proc` shows only its own processes |
| Environment | only the shared API address, the listen address and bun's no-cache setting; nothing inherited |
| Resources | `MemoryMax=256M`, `MemorySwapMax=0`, `TasksMax=64`, `CPUQuota=100%`, `LimitNOFILE=1024` |

Reaching a limit may kill the face; systemd restarts it, and bus calls continue
while it is absent. `src/web/probe-unit.sh` exercises these walls on an
installed unit. The design and its checks are in
[Web § process and account](../Plans/R0.8-Web/README.md#process-and-account).

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
| `-web` | accepted and ignored from 0.8.50, so an older unit still starts; the [web face](#the-web-face) is its own unit |

The bus child inherits the supervisor's environment.

A child that dies is restarted with backoff, and the listeners are passed to
the replacement — the socket a client holds is the same file across a restart.
A supervisor that is killed outright takes its children with it
(`PR_SET_PDEATHSIG`), so nothing orphaned keeps a port.
