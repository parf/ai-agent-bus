# Processes and privileges

📌 **TL;DR:**
- **Problem:** a bus that serves requests, holds listeners and shows a web page must keep their powers apart, or one flaw reaches everything.
- **Outcome:** the supervisor owns listeners and child lifetime, the bus owns state and serves requests, and the web page, under its own account and unit, asks the bus for the visitor's own view. Passing a listener is a runtime boundary, separate from the module rules. The daemon never runs a program a user supplied.
- **Start:** on a Linux node, inspect the daemon processes: `ps -ww -u agent-busd -o args` ([Try this first](#try-this-first)).

## Try this first

On an installed Linux node, inspect the daemon's processes:

```sh
ps -ww -u agent-busd -o args
```

Their titles distinguish the supervisor from the request-serving bus process. Client-only installations have no local daemon account.

## The rule

The supervisor owns listeners and child lifetime. The bus owns state and
serves requests. The web face requests the caller's view from the bus.
Passing a listener is a runtime boundary; the [module rules](../src/MODULES.md#the-rule)
remain separate.

## The processes

| Process | Owns | Runtime |
|---|---|---|
| Supervisor | Listening sockets, socket ownership and child restart | `agent-busd` |
| Bus | Registry, queues, tokens and browser sessions | Same executable, bus role |
| Web face | HTTP rendering, no credential store | `agent-bus-web.service`: the compiled `agent-bus-ts`, or bun on a checkout, as the `agent-bus-web` account ([the web face](#the-web-face)) |
| Dedicated runner | Its agent's inbox reader and script children | User-launched CLI; outside the daemon |

The bus is the supervisor's only child.

## Process titles

- Use `--version` for the program version and build information; `--help` also names the program and version. MCP handshakes and the Codex adapter report the version too.

| Process | Title in `ps -ww -o args` | Calls |
|---|---|---|
| Supervisor | `agent-busd <version> ; supervisor` | No counter; it serves no requests |
| Bus child | `agent-busd <version> ; Calls: <count> ; bus` | HTTP requests received across every listener, including refusals and requests still in progress |
| Dedicated runner | `agent-bus run <version> ; Calls: <count> ; <name>` | Inbox messages taken, including work that later fails or is skipped |
| Web face | `agent-bus-web <version> ; Calls: <count>` | HTTP requests received, refusals, assets and `/healthz` included; written over bun's own argv, so a title longer than the unit's command line is cut |

- Counts are cumulative for the process lifetime, start at zero, and reset on restart.
- Titles are set at start and refreshed once a second; the request path only increments an atomic counter.
- Idle polling does not count as runner work.
- Scripts, arguments, credentials and message bodies are never added to titles.
- The executable's short `comm` name stays unchanged.

## Nothing the daemon runs may exec

The bus never executes user-supplied programs. Scripts run in a separate
[dedicated runner](06-dedicated-runner.md) or [runner](runner/README.md).

The supervisor starts its bus child. The bus may invoke the fixed
`ssh-keygen` verifier for enrolment signatures, with arguments it constructs.
The web unit permits execution only of its configured runtime and libraries.

## Why the supervisor holds CAP_CHOWN

- Per-account socket ownership needs this capability.
- The installed unit grants it only to the supervisor; the supervisor clears ambient capabilities before starting its children.

The supervisor reports a failed ownership change. That does not certify that
another account can use the intended socket.

## The TCP listener

- The supervisor binds the TCP address; the bus child serves it.
- With a TLS directory (`--tls-dir`, [setup § TLS](administration.md#tls)), every connection on that port is sniffed: a first byte of `0x16`, a TLS handshake, is answered over TLS with HTTP/2 or HTTP/1.1, and anything else as plain HTTP.
- Without one, the port is plain HTTP alone.
- The unix sockets never answer TLS.

<details>
<summary>Limits</summary>

| | |
|---|---|
| First byte | within 5 s, or the connection is closed; each is sniffed on its own, so a silent client holds up nobody |
| Handshake | bounded by the bus server's header timeout |
| A pair that does not load | refuses the start before anything is bound, with the reason |
| A key readable by everyone | refused the same way |
| Fingerprint | logged at start, by the supervisor and again by the bus child for the certificate it serves |

</details>

## What is shared

Inherited listener descriptors and explicit API calls. The bus owns the store; the supervisor does not keep a store handle. The web face, a
separate unit, gets the shared socket and no principal token. It forwards each
visitor's credentials and then uses their browser session.

## The web face

- The web face is TypeScript in `src/web`: in a release, part of the compiled `agent-bus-ts`; on a checkout, run from source by the system bun.
- Either way it runs under its own system account and its own systemd unit.
- It is not a daemon child: the supervisor starts nothing for it.
- It reaches the daemon only over the shared socket, with each visitor's session, and holds no bus credential or state; with TLS on, it holds its own certificate copy and a socket in its `RuntimeDirectory`.

| | |
|---|---|
| Account | `agent-bus-web`: system account, `nologin`, home its code directory, no SSH keys, not in the account map; owns nothing on disk ([setup § the accounts](administration.md#the-two-accounts)) |
| Code | the real [web code directory](07-setup.md#install), directly; no state-directory link |
| Runtime | a release: `<release>/agent-bus-ts web`, with the browser code and assets read from `web/` beside it; a checkout: `/usr/bin/bun run <web-directory>/server.ts`, no build step |
| Unit | `agent-bus-web.service`, shipped as `src/web/agent-bus-web.service`; setup writes it with the host's exec paths |
| Daemon link | `/run/agent-bus/bus.sock` only; mapped account sockets are closed to it by their mode |
| Listen | [discovery § where it listens](05-discovery.md#where-it-listens) |
| With TLS on | a small front on the port sniffs each connection: TLS is piped to the face's own server, which holds the certificate on a unix socket in its `RuntimeDirectory` (`/run/agent-bus-web/https.sock`); plain HTTP is answered at the front with a `301` to `https://` and never reaches the app, except `/healthz`, answered for the app. Without TLS the face binds the port itself, as before ([setup § TLS](administration.md#tls)) |

**The face acts on the visitor's session.** Every call a signed-in person causes is made with that person's
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
| Filesystem | `ProtectSystem=strict`, `ProtectHome`, private `/tmp` and devices; the daemon, runner and `agent.d` state, `/root` and `/etc/ssh` are inaccessible; `RuntimeDirectory` `/run/agent-bus-web` (`0700`) is the one writable path |
| Exec | `NoExecPaths=/`; `ExecPaths` is the runtime and the libraries it links, computed per host by setup from `ldd` of that runtime |
| Network | `IPAddressDeny=any` with `IPAddressAllow=localhost`: it answers on loopback and connects out to nothing |
| System calls and kernel | `@system-service` minus privileged, resource, mount and debug sets; kernel tunables, modules, logs, cgroups, clock and hostname protected; `/proc` shows only its own processes |
| Environment | only the shared API address, the listen address, `HOME` and bun's no-cache setting, plus `AGENT_BUS_WEB_TLS_DIR` from setup's drop-in with TLS on; nothing inherited |
| Resources | `MemoryMax=256M`, `MemorySwapMax=0`, `TasksMax=64`, `CPUQuota=100%`, `LimitNOFILE=1024` |

Reaching a limit may kill the face; systemd restarts it, and bus calls continue
while it is absent.

</details>

## How a child is started

One binary. The supervisor opens every listener before any child exists, so a
child cannot make one and does not need the capability to.

| | |
|---|---|
| `AGENT_BUS_ROLE=bus` | which role this process is. Absent means supervisor |
| `AGENT_BUS_FDS` | what arrives at fd 3 upwards, in order: `tcp`, `shared`, `user:<principal>`, and `owner:` for the daemon account's own socket, the daemon Owner's — the whole contract between the two |
| `AGENT_BUS_ACCOUNTS` | the supervisor's active editable account map; the bus compares it with durable desired state to report whether a full restart is required |
| `AGENT_BUS_SUPERVISOR_PID` | the spawning supervisor's PID, replacing any inherited value; the non-Linux child checks this exact parent before starting and while running |
| `--owner` | required first-run Owner seed; the daemon account's socket answers as the durable daemon Owner, which a transfer moves ([daemon ownership](01-identity-and-authority.md#daemon-owner)) |
| `--web` | accepted and ignored for compatibility with older units; the [web face](#the-web-face) is its own unit |

The bus child inherits the supervisor's environment.

A child that dies is restarted with backoff, and the listeners are passed to
the replacement — the socket a client holds is the same file across a restart.
A supervisor that is killed outright takes its children with it, so nothing
orphaned keeps a port.

On Linux the kernel sends the death signal (`PR_SET_PDEATHSIG`). Elsewhere,
the bus child refuses startup if its expected supervisor is already gone;
after startup it stops itself as on SIGTERM once that supervisor is gone
([macOS](platforms/macos.md#the-daemon)).
