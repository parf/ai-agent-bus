# Foreground runner

📌 **TL;DR:** Run scripts per message, and connect agent sessions through
launchers. `agent-bus start` registers an 👾, reads its queue and runs the
script for each message; `ab-claude`, `ab-codex` and `ab-opencode` put a live
session behind a name with bus tools loaded.

## Status

| MVP | Scope |
|---|---|
| Built | Script agents, bounded parallel execution, graceful stop, logs, optional systemd sandbox, runtime adapters and smart launchers with MCP tools. |
| Pending | Full live-runtime and fresh-host acceptance of [runtime integration delivery](#runtime-integration-delivery), including [sidecar isolation and recovery](#runtime-isolation-and-recovery). |

## What the runner does

`agent-bus start` stays in the foreground, registers an 👾
[agent](03-records.md#record-kinds) — the kind that has a
queue here — obtains its credential and reads that queue. It runs the script once per message. It does
not manage installed instances, autostart, reload or restart policies.

## Script agents

A script agent is a **script behind an 👾 agent record**: the name is an
agent because that is the kind with a queue to read
([records § five record kinds](03-records.md#record-kinds)). A 📡
`service` is the external case, and nothing runs behind one here.

| Form | Input | Output |
|---|---|---|
| `--algo=args` | Message body as the script's argument | Stdout at exit |
| `--algo=json` | Envelope JSON on stdin; the default form | Stdout at exit |

```sh
agent-bus start hello@srv1 --algo=args ./hello-world.sh --descr "greets you"
cat service.json | agent-bus start -5
```

| Rule | Built behavior |
|---|---|
| Script argument | One shell command line; quote it if it contains arguments |
| `-N` | At most N script processes; default 1. One runner reads one inbox |
| Success with output | Send the output as the answer |
| Silent success | Send `done` so the caller can stop waiting |
| Nonzero exit | Log failure and send no answer; no retry |
| Expired caller deadline | Skip the script; message TTL is enforced separately in the daemon |
| Stop | Stop taking work and wait for scripts already running; leave the registry record and queue intact |
| Work directory | One per agent; the script starts there |
| Credentials | The launcher must be allowed to obtain that agent's credential; it cannot become somebody else's agent |
| Sharing | State `--allow name,...`, `--allow '@owner'` or `--allow '*'`; JSON uses `allow`. `@owner` admits the records the direct Owner owns. Fresh registrations use the [restricted default](02-access.md#acl); omitted settings on restart follow [registration rules](01-identity-and-roles.md#registration). Reply inboxes need their own grants |

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
| starting a name that is already running here | is refused; the foreground `start` command has no shared-start mode |

Because a message is taken from the daemon only when a script process is free
to run it, an agent that dies loses only the work already in flight; the rest
is still queued for whatever reads that inbox next.

Stopping leaves the registration in place; remove an idle address with
[unregister](01-identity-and-roles.md#unregistering).

## What the child is told

The script receives message id, sender, receiver, topic, tag and its work path
through environment variables. Their exact names are owned by
[the runner source](../src/cmd/agent-bus/start.go). These values describe the
work and authenticate nothing. Managed configuration layers are not built.

## Who it runs as

The foreground runner executes as its launching user. It is not a child of the
daemon. The separate account prepared by setup does not imply a managed runner
program or unit is installed; see [setup](09-setup.md#the-two-accounts).

## Adapters

**Agent runtimes get one push adapter each**: each runtime takes a message
differently, so each gets its own adapter that reads the session's queue and
pushes into the *running* session. Claude Code, Codex and opencode are proven here — `src/mcp`, run by `smoke.sh`.

| Runtime | Push path | Status |
|---|---|---|
| Claude Code | Channels — `claude --channel`, `notifications/claude/channel`, reply tool | built and smoke-tested here |
| Codex | App Server — `turn/steer` if busy, `turn/start` if idle, `thread/resume` after restart. **Two transports**: told where a shared app-server is, it attaches over a loopback WebSocket; told nothing, it spawns `codex app-server` and speaks newline-delimited JSON-RPC on stdio. What is ruled out is a WebSocket over a **unix socket**, which bun cannot open | both proven in the live run |
| opencode | HTTP. The launcher starts `opencode serve`, selects an existing session or creates one, and attaches the TUI and pusher to that explicit session ID. `POST /session/{id}/prompt_async` delivers into that session. Initial TUI startup need not emit a selection event | built and smoke-tested here |
| ChatGPT | none — cannot be pushed; pull through the MCP inbox only | pull only |

The adapter acknowledges to the bus only after the runtime has *accepted* the
message. **Acceptance is not processing** — Claude Channels expose no later
"the model handled it" signal — so anything that needs to know the work was
done waits for a correlated reply, never for the ack. It never lets an incoming message change the session's permissions or
mode — that is the adapter's policy as a receiver
([messaging § envelope](04-messaging.md#envelope)), not a rule of the bus.


## Runtime integration delivery

**Implementation built; installed live-runtime acceptance pending.** Ship usable Claude channels and Codex App Server integration with
the [installation](09-setup.md#install), building on the existing
[adapters](#adapters). Adapter smoke tests alone do not establish that a new
user can install and launch either integration.

Both integrations include the [MCP minimum](05-discovery.md#mcp-minimum).

| Deliverable | Required behavior |
|---|---|
| Claude channel | Installed channel configuration, incoming messages in the running session, and replies through the bus; document required runtime support and activation |
| Codex App Server | Ship the App Server integration with the [smart launcher](#smart-launchers), bus tools and live-session messaging. This is the owner's intended meaning of "Codex apps"; a separate app/plugin is outside this MVP requirement |

Any vendor approval needed for an official listing is external; a future
listing cannot substitute for the working integration required in MVP.

## Smart launchers

**Built.** `ab-claude`, `ab-codex` and `ab-opencode` are ordinary-user
scripts. References: `/rd/bin/ai-claude`, `/rd/bin/ai-codex` and their shared
`/rd/bin/.ai-common.sh`; these are behavioral examples, not
runtime dependencies of the installed V2 scripts.

| Concern | Required behavior |
|---|---|
| Startup | Find the installed runtime and integration assets; explain missing prerequisites; work without `/rd` or a repository checkout |
| Configuration | Use V2 [access](02-access.md#getting-a-token) and [face configuration](../src/mcp/README.md#environment); an explicit address wins. Without one, discover the account's socket in its login runtime directory, then the installed [local socket](02-access.md#local-socket). With a token, select the shared listener instead |
| Bus tools | Load the [MCP minimum](05-discovery.md#mcp-minimum) into the launched session alongside message delivery; authorize the agent-bus MCP namespace in Claude and set Codex's server-specific `default_tools_approval_mode="approve"` so those calls need no initial tool prompt |
| Session | Resume the current directory's conversation when available; otherwise start fresh. Preserve caller arguments and route messages to the intended live session |
| Automatic execution | Always enable Claude's `--enable-auto-mode`; configure Codex's App Server with `approval_policy="never"` and `sandbox_mode="danger-full-access"`, overriding contrary launch options |
| Continuation | Always continue the last available session in the launch directory. Claude uses `--continue`; Codex selects the latest thread and pins its ID when attaching the TUI. With no saved conversation, let the runtime create one; the Codex pusher attaches after the TUI creates its thread |
| Accounts | `ab-claude` accepts `-2`, `-3` and `-4`: the Claude configuration home becomes `~/.claude2`, `~/.claude3` or `~/.claude4`, or a set `CLAUDE_CONFIG_DIR` with that digit appended, so several accounts run side by side with their own login, sessions and servers. The launcher consumes the flag; the runtime never receives it |
| Session naming | Prefer an assigned session name under the [session naming contract](#session-names); use the launch directory as the fallback |
| Claude wiring | Activate the installed channel and tools using the [loading contract](../src/mcp/README.md#loading-it) |
| opencode wiring | Start the server the session attaches to, and give it the face and the launcher's enforced permission through `OPENCODE_CONFIG_CONTENT`. The server is loopback with a password passed by environment, never as an argument: anything that reaches it can drive the session |
| Codex wiring | Connect the terminal and pusher to the same App Server, with the tools and inbox reader arranged as [documented](../src/mcp/README.md#one-app-server-two-clients) |
| Readiness | Report whether bus integration is active. With no bus configuration, allow a plain runtime session with an explicit notice; failed integration must never be reported as connected |
| Lifecycle | Wait for readiness before attaching; propagate the runtime's exit status and stop only the helper processes created by this launch on exit or startup failure |
| Permissions | The launcher's enforced mode is chosen at startup; incoming messages cannot change it or the [adapter policy](#adapters) |
| Terminal helpers | Apply and restore [terminal appearance](#terminal-appearance); optional styling tools must not prevent startup |

## Terminal appearance

**Built.** The shared helper lives under `src/launchers/` and is bundled into
its launcher; it is not a command on `PATH` and has no `/rd` dependency.

| Concern | Behavior |
|---|---|
| Title | `Claude(...)`, `Codex(...)` or `OpenCode(...)`; prefer the assigned session name, otherwise the last two directory components, with home abbreviated as `~`. Refresh when the session name changes |
| Ownership | Disable the runtime's own title updates while the launcher controls an interactive terminal |
| Exit | Restore the saved terminal title on normal exit, startup failure and handled termination; reset launcher tab colors to terminal defaults. Konsole restores its saved local and remote title formats |
| Claude palette | Rotate blue/cyan shades, following the reference launcher |
| Codex palette | Rotate green shades, following the reference launcher |
| OpenCode palette | Rotate a distinct violet palette; red stays reserved for production |
| Styling | Kitty tab backgrounds with dimmed inactive tabs and contrasting text; Konsole tab backgrounds. Address the caller's tab/session, even when another tab has focus |
| Compatibility | Standard terminal title sequences elsewhere; unsupported restoration falls back to clearing the title for the shell. Missing or denied optional terminal controls are harmless; pipes and dumb terminals receive no styling |

## Session names

**Built.** Prefer the name assigned to the actual Claude or
Codex session when the runtime exposes it. Otherwise identify it by runtime
and launch directory, as the reference scripts do. The launch directory is
captured at startup; a later working-directory change does not rename the session.

| Source | Use |
|---|---|
| Explicit bus name | Honor the caller's configured identity under the existing [face configuration](../src/mcp/README.md#environment) and [credential rules](02-access.md#what-a-call-carries) |
| Runtime-assigned session name | Preferred human-readable label and basis for a derived bus name when no explicit bus name was supplied; obtain it from the session being launched or resumed |
| No assigned name available | Fall back to runtime and launch directory, in a human-readable form such as `claude(/rd/)` |

Human-readable labels and [canonical bus names](01-identity-and-roles.md#names) serve
different purposes: preserve the label for discovery, and derive a valid bus
name for routing. A title is not a credential or a unique session identifier.
The `ab-*` launchers derive `#runtime/instance@host` from 0.7, with `claude`, `codex`
or `opencode` as the template, the normalized session title (or directory) as
the instance and the host name as the realm, while the launching User stays
realm-less. For example, a Codex session titled `home` on host `parf.us`
becomes `#codex/home@parf.us`, owned by `parf`. They keep deriving a realm although
[0.7 makes it optional](01-identity-and-roles.md#names): a bare `codex` or
`runner` would be one name for every node and every user. From 0.7 they also
register those agents as [Personal](03-records.md#personal-and-shared): a
session record belongs to the launching user, not on the shared web pages. Saved dot-form addresses migrate on restart through the same address-change
behavior below. Explicit bus names and other clients' naming remain unchanged.
Duplicate human-readable labels gain `#2`, `#3` and so on. Canonical addresses
use `.2`, `.3` before the realm, because `#` inside a name is not part of the
grammar — from 0.7 it is the [agent prefix](01-identity-and-roles.md#names) and
appears only at the front.
When changing addresses, the session's own previous registration does not
compete for its label.
Allocation checks the visible registry and holds local locks; normalization
and truncation collisions are checked on the resulting canonical name.
The daemon's [conditional registration](01-identity-and-roles.md#registration) arbitrates
simultaneous new claims, including launchers with separate local state directories.

Bind the selected identity to the runtime's actual session, consistently across
the tools and pusher. A runtime UI title change updates only the label until
an explicit bus rename or restart.
On restart, `ab-claude` and `ab-codex` derive a new address if the session title
has changed its normalized base; an explicit bus name stays fixed. Otherwise
resume reuses the saved address, including any collision suffix. This restart
behavior belongs only to these launchers, not other bus clients.
After claiming the new address, the launcher [unregisters](01-identity-and-roles.md#unregistering)
the old one if idle. If removal is refused, it reports the retained address;
old queued messages stay there and are not moved to the new inbox.
Claude's explicit name option and saved title metadata supply its label;
Codex supplies it through App Server thread metadata. When unavailable, the
fallback still works. Runtime session IDs key the saved bus-name binding;
concurrent launches do not attach to the same session. The launcher state
and credential files are private to the launching OS account.

## Explicit session rename

**Built.** With an `ab-*` launcher, `ab_rename(name)` changes the runtime
session title and bus address together. Codex and OpenCode use their own
session APIs; Claude appends its existing title metadata. Without a name,
the launcher reads the runtime's current title immediately. Without a
launcher, a bare MCP face holding only its agent's credential refuses a
named rename and changes nothing, because the new address is its User's
record ([Q105](../Plans/MVP/QUESTIONS.md#open-questions)).

The launcher owns the transition: claim a unique address as the launching
account, acquire its credential, update the runtime title, stop the old inbox
reader, persist the new session binding and private environment, then read
the new inbox. All MCP clients read that shared identity before tool calls.
Concurrent rename and metadata refresh operations are serialized; repeated
renames reuse the selected address. A now-unneeded collision suffix may be
removed when the base address becomes available.

The old address is removed only when idle; queued messages stay in a retained
inbox and the tool reports it. A failed runtime rename releases the new claim
and keeps the previous identity. An explicit `AGENT_BUS_NAME` pins the address
and refuses this operation. Older running launchers must be restarted; their
MCP face must not change addresses independently.

The internal control listener binds loopback and requires a random credential
kept in the private session file. It can rename only its own launcher; it does
not accept an arbitrary principal or expose the owner's bus credential.

## Running the launchers

OpenCode executable selection honors `OPENCODE_BIN`, then tries
`~/.local/bin/opencode`, then searches `PATH`. This preserves user-wrapper
settings even when an interactive shell function hides a system executable.
An explicit override that cannot run is an error, not a fallback.

Use the [Claude entry point](../src/launchers/ab-claude),
[Codex entry point](../src/launchers/ab-codex) or
[opencode entry point](../src/launchers/ab-opencode), or the relocatable output of
[the build script](../src/README.md#build-and-check). Bun and the selected
runtime must be installed. Local socket discovery needs no environment setup;
explicit addresses and tokens use the [face environment](../src/mcp/README.md#environment).

The launcher registers the session as its owner, obtains the session's own
token and passes only that credential to its tools. On a mapped user socket,
it switches to the shared listener for session calls and verifies the resulting
identity. Runtime configuration is temporary; global Claude/Codex configuration
is not edited. Per-runtime executable overrides and state paths are described
by each launcher's help output.

The launcher owns its local App Server; attaching to another server and worktree
creation are unsupported. After an uncatchable launcher kill, stale lock directories
may need removal from its state directory after checking the recorded PID is dead.
Resuming the same saved session binding from copied state on different hosts is
not coordinated by the local session locks; default state stays on its launching host.

## Runtime isolation and recovery

Every shipped runtime must keep session control private to its OS account.
**Built:** launcher-owned Codex and OpenCode servers require per-run credentials;
launcher rename control requires its own capability. Loopback alone is not a
boundary. **H.9.5 acceptance remains open:** the [interactive evidence](../Plans/MVP/done/runtime-interactive.md#scope) covers Codex and OpenCode; Claude still needs the same co-exercise.

<details>
<summary>Credentials and measured scope</summary>

Codex uses native capability-token authentication before WebSocket upgrade.
Its token file is private inside the launcher's private run directory; the
pusher supplies an Authorization header and the TUI reads its token from an
environment variable. It requires a runtime supporting `--ws-auth capability-token`
and `--remote-auth-token-env`; unsupported versions fail startup, with no
unauthenticated fallback. OpenCode uses its native server password. Secrets
never appear in launcher-supplied command-line arguments. Root and processes
already running as the same OS account are outside this account boundary.

The [endpoint evidence](../Plans/MVP/done/runtime-endpoint-auth.md#checks) includes
actual second-account reads and renames, positive controls with authentication
removed, and separate launcher/MCP/pusher fixture checks. [H.9.5](../Plans/MVP/TODO.md#remaining-work)
retains the outstanding co-exercise; the [Claude prerequisite](../Plans/MVP/done/runtime-interactive.md#claude-prerequisite) names the unavailable input path and the acceptance it prevents.

</details>

After a daemon restart or sidecar failure, an open interactive session must
resume bus delivery or clearly report that integration is inactive and how
to recover. Recovery instructions must lead to a successful correlated
exchange in that session. This adds no replay or exactly-once guarantee:
[consumption](04-messaging.md#one-reader-per-inbox) and
[snapshots](04-messaging.md#durability) retain their loss boundaries.
[H.9.6](../Plans/MVP/TODO.md#remaining-work) owns live acceptance.

## Sandboxing

Off by default; `--sandbox on|off` selects it explicitly. The built backend is
`systemd-run --user` behind the sandbox port. Asking for confinement without a
working user manager fails rather than silently running unconfined. Startup
reports the selected backend.

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
