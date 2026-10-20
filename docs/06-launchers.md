# Session launchers

📌 **TL;DR:** `ab-claude`, `ab-codex` and `ab-opencode` put a live AI session
behind a bus name with the bus tools loaded, and push each message into the
running session through that runtime's adapter. A script behind a name is the
[dedicated runner](06-dedicated-runner.md)'s instead.

This page owns the runtime adapters and the session launchers.

## Adapters

**Agent runtimes get one push adapter each**: each runtime takes a message
differently, so each gets its own adapter that reads the session's queue and
pushes into the *running* session. Claude Code, Codex and opencode are proven here — `src/mcp`, run by `smoke.sh`.

| Runtime | Push path | Status |
|---|---|---|
| Claude Code | Channels — `claude --channel`, `notifications/claude/channel`, reply tool | built and smoke-tested here |
| Codex | App Server — `turn/steer` if busy, `turn/start` if idle, `thread/resume` after restart. **Two transports**: told where a shared app-server is, it attaches over a loopback WebSocket; told nothing, it spawns `codex app-server` and speaks newline-delimited JSON-RPC on stdio. What is ruled out is a WebSocket over a **unix socket**, which bun cannot open | both proven in the live run |
| opencode | HTTP. The launcher starts `opencode serve`, selects an existing session or creates one, and starts the TUI and pusher on that explicit session ID. `POST /api/session/{id}/prompt` with `delivery: "queue"` delivers into that session, waiting for an idle turn rather than steering one. Initial TUI startup need not emit a selection event. `ab-opencode1` speaks the 1.x API and its `attach` TUI for hosts still on it | built and smoke-tested here |
| ChatGPT | none — cannot be pushed; pull through the MCP inbox only | pull only |

**Push stops rather than repeats a refusal about who is asking.** A credential
the daemon does not know and an inbox somebody else already holds are answers
that asking again cannot change, so the loop says so once and leaves the
session running without a channel. A suspended principal is different: it can
be reactivated, so push says so once and asks again every 30
minutes, and resumes by itself after reactivation (owner, 2026-09-23). It is
told by the daemon's own sentence for it, "user access is suspended"; any other
403 stops push as before. A
transient failure is retried after two seconds; a session that outlives its
principal otherwise refuses its way through the daemon for as long as it lives.

Follow the [receipt rules](04-messaging.md#receipts).

The adapter acknowledges to the bus only after the runtime has *accepted* the
message. **Acceptance is not processing** — Claude Channels expose no later
"the model handled it" signal — so anything that needs to know the work was
done waits for a correlated reply, never for the ack. It never lets an incoming message change the session's permissions or
mode — that is the adapter's policy as a receiver
([messaging § envelope](04-messaging.md#envelope)), not a rule of the bus.


## Runtime integration delivery

**Built; passed [live acceptance](../Plans/done/R0.8-MVP/done/runtime-delivery.md#checks) on the development host and on a [fresh host](../Plans/done/R0.8-MVP/done/fresh-host-runtime.md#checks).** Ship usable Claude channels and Codex App Server integration with
the [installation](07-setup.md#install), building on the existing
[adapters](#adapters). Adapter smoke tests alone do not establish that a new
user can install and launch either integration.

Both integrations include the [MCP minimum](05-discovery.md#mcp-minimum).

| Deliverable | Required behavior |
|---|---|
| Claude channel | Installed channel configuration, incoming messages in the running session, and replies through the bus; document required runtime support and activation |
| Codex App Server | Ship the App Server integration with the [smart launcher](#smart-launchers), bus tools and live-session messaging. This is the owner's intended meaning of "Codex apps"; a separate app/plugin is outside the release |

An official vendor listing is external and not part of the release.

## Smart launchers

**Built.** `ab-claude`, `ab-codex` and `ab-opencode` are ordinary-user
scripts, modelled on the Legacy-V1 launchers and independent of them;
`ab-opencode1` is the archived 1.x variant of the last, kept beside it.

| Concern | Required behavior |
|---|---|
| Startup | Find the installed runtime and integration assets; explain missing prerequisites; work without `/rd` or a repository checkout |
| Configuration | Use V2 [access](02-access.md#getting-a-token) and [face configuration](../src/mcp/README.md#environment); an explicit address wins. Without one, discover the socket of the account the launcher runs as, found by uid rather than `$USER`, in its login runtime directory, then the installed [local socket](02-access.md#local-socket). With a token, select the shared listener instead. An `https://` address needs `AGENT_BUS_TLS_FINGERPRINT` ([over HTTPS](02-access-remote.md#over-https)) |
| Bus tools | Load the [MCP minimum](05-discovery.md#mcp-minimum) into the launched session alongside message delivery; authorize the agent-bus MCP namespace in Claude and set Codex's server-specific `default_tools_approval_mode="approve"` so those calls need no initial tool prompt |
| Session | Resume the current directory's conversation when available; otherwise start fresh. Preserve caller arguments and route messages to the intended live session |
| Automatic execution | Always enable Claude's `--enable-auto-mode`; configure Codex's App Server and the TUI that runs its thread with `approval_policy="never"` and `sandbox_mode="danger-full-access"`, overriding contrary launch options |
| Continuation | Always continue the last available session in the launch directory. Claude uses `--continue`; Codex selects the latest thread and pins its ID when attaching the TUI. With no saved conversation, let the runtime create one; the Codex pusher attaches after the TUI creates its thread |
| Accounts | `ab-claude` accepts `-2`, `-3` and `-4`: the Claude configuration home becomes `~/.claude2`, `~/.claude3` or `~/.claude4`, or a set `CLAUDE_CONFIG_DIR` with that digit appended, so several accounts run side by side with their own login, sessions and servers. The launcher consumes the flag; the runtime never receives it |
| Session naming | Prefer an assigned session name under the [session naming contract](#session-names); use the launch directory as the fallback |
| Claude wiring | Activate the installed channel and tools using the [loading contract](../src/mcp/README.md#loading-it) |
| opencode wiring | Start the server the session runs on, and give it the face and the launcher's enforced permission through `OPENCODE_CONFIG_CONTENT`. The TUI starts on that server with `--server`; `ab-opencode1` attaches a 1.x TUI with `attach --dir`. The server is loopback with a password passed by environment, never as an argument: anything that reaches it can drive the session |
| Codex wiring | Connect the terminal and pusher to the same App Server, with the tools and inbox reader arranged as [documented](../src/mcp/README.md#one-app-server-two-clients) |
| Readiness | Report whether bus integration is active. With no bus configuration, allow a plain runtime session with an explicit notice; failed integration must never be reported as connected |
| Lifecycle | Wait for readiness before attaching; propagate the runtime's exit status and stop only the helper processes created by this launch on exit or startup failure. A helper that dies at startup is reported with the last lines of its own log, which cleanup then removes |
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

Human-readable labels and [canonical bus names](01-identity-and-authority.md#names) serve
different purposes: keep the title for the terminal, and derive a valid bus
name for routing. A new record description is empty by default;
`--descr "Review billing"` sets a short purpose. Without `--descr`, resuming
keeps the record’s current description, including web edits. Neither runtime
nor working directory is copied into it.
Resuming keeps the saved address; changing a purpose changes only its description. A title is not a credential or a unique session identifier.
The `ab-*` launchers derive `#runtime/instance@host` from 0.7, with `claude`, `codex`
or `opencode` as the template, the normalized session title (or directory) as
the instance and the host name as the realm, while the launching User stays
realm-less. For example, a Codex session titled `home` on host `parf.us`
becomes `#codex/home@parf.us`, owned by `parf`. They keep deriving a realm although
[0.7 makes it optional](01-identity-and-authority.md#names): a bare `codex` or
`runner` would be one name for every node and every user. From 0.7 they also
register those agents as [Personal](03-records.md#personal-and-shared): a
session record belongs to the launching user, not on the shared web pages. Saved dot-form addresses migrate on restart through the same address-change
behavior below. Explicit bus names and other clients' naming remain unchanged.
Canonical addresses use `.2`, `.3` before the realm, because `#` inside a name is not part of the
grammar — from 0.7 it is the [agent prefix](01-identity-and-authority.md#names) and
appears only at the front.
Allocation checks the visible registry and holds local locks; normalization
and truncation collisions are checked on the resulting canonical name.
The daemon's [conditional registration](01-identity-and-authority.md#registration) arbitrates
simultaneous new claims, including launchers with separate local state directories.

Launcher names are persistent session reservations. A new session never
reclaims an existing record, regardless of its age, readers or queued messages.
To inspect dormant reservations use `agent-bus ls --kind agent --all`; after
checking that a session is no longer needed, its owner can explicitly run
`agent-bus unregister <name>` (the daemon refuses a busy record).
It takes the first free address in order: the base, then `.2`, `.3` and so on
regardless of their descriptions. Concurrent new sessions keep distinct names; another
session cannot take a live session's name during startup or later.

An explicit `AGENT_BUS_NAME` is itself a reservation: a fresh session uses
that exact address, never a numbered alternative. A saved or explicit
reservation with another attached reader is refused by name without updating
the record; resume after that session has stopped.

Known startup limits: two starts of the same saved or explicit address from
different state directories can both register before either reader attaches;
a reader recheck cannot arbitrate that gap. Avoid simultaneous starts of the
same reserved session from different state directories. Local lock directories
are published atomically with their owner PID already inside; stale cleanup
cannot remove another claimant’s replacement lock.

A saved session with an unchanged title resumes exactly its saved address,
including its collision suffix: after one second, after a crash with no reader,
or after a long idle period. The exception is the person's own rename,
described below; age and another session never rename it.

Dormant identities therefore accumulate in the registry. Release an unwanted
reservation explicitly with [unregister](01-identity-and-authority.md#unregistering).
The existing [explicit rename](#explicit-session-rename) also releases only the
renamed session's own old record when idle. Releasing a name makes it available
to new sessions; keep reservations for sessions you intend to resume.

Bind the selected identity to the runtime's actual session, consistently across
the tools and pusher. A runtime UI title change updates only the terminal label, never the purpose, until
an explicit bus rename or restart.
On restart, `ab-claude` and `ab-codex` derive a new address if the session title
has changed its normalized base; an explicit bus name stays fixed. Otherwise
resume reuses the saved address, including any collision suffix. This restart
behavior belongs only to these launchers, not other bus clients.
After claiming the new address, the launcher [unregisters](01-identity-and-authority.md#unregistering)
the old one if idle. If removal is refused, it reports the retained address;
old queued messages stay there and are not moved to the new inbox.
Claude's explicit name option and saved title metadata supply its label;
Codex supplies it through App Server thread metadata. When unavailable, the
fallback still works. Runtime session IDs key the saved bus-name binding;
concurrent launches do not attach to the same session. The launcher state
and credential files are private to the launching OS account. Saved bindings,
lock PID files and palette counters are published by same-directory atomic
rename with mode 0600, so readers see complete snapshots. This does not add
power-loss durability or serialize simultaneous palette-counter increments.

## Session tags

All four launchers accept repeatable `--tag a,b` options to group session records
for `agent-bus ls --tag review` and `agent-bus call --tagged review 'check progress'`.
The [record tag grammar](03-records.md#tags) applies: labels are normalized,
deduplicated and sorted; an empty or reserved label is refused by name.

```sh
ab-codex --tag review,billing --tag 'release notes'
```

Without `--tag`, a resumed session keeps its stored tags, including web edits.
An explicit option replaces the custom set; `--tag ''` alone clears custom tags.
The runtime tag (`claude`, `codex` or `opencode`) and the record's Personal
classification stay. Rename, crash restart and MCP re-registration keep tags.
Options after `--` belong to the runtime and are not parsed by the launcher.

## Restart after a crash

**Built.** `ab-claude`, `ab-codex`, `ab-opencode` and `ab-opencode1` accept `--restart`
(default 30 seconds), `--restart=10` (seconds) or `--restart=2m` (minutes).
Use a positive whole number; other timeout forms are refused. The flag belongs
to the launcher and is not passed to the runtime. After `--`, it is a runtime
argument and is left untouched.

A nonzero exit or signal such as SIGSEGV restarts only the foreground TUI after
the wait. The launcher prints `<name> exited (<code or signal>); restarting in
30s (attempt 1/10) — Ctrl-C to stop` (with the current wait and attempt).
Each short-lived failure doubles the wait, capped at five minutes, including
the first requested delay. After ten consecutive runs that fail before staying
alive for 60 seconds, the launcher stops with a named crash-loop error. A run
lasting at least 60 seconds resets the count and backoff; waiting does not.
The launcher preserves the original user arguments and resumes the same session. The bus name, record and local binding
locks stay reserved during the wait; a conflicting live name is refused.
Ctrl-C during the wait ends the launcher and releases its locks normally.

Exit 0 means the user quit: it never restarts. Codex's App Server and OpenCode's
server are reused, not restarted; a server exit still ends the launcher with
its existing error, including during the wait. Without `--restart`, exit
behaviour is unchanged.

## Explicit session rename

**Built.** With an `ab-*` launcher, `ab_rename(name)` changes the runtime
session title and bus address together. Codex and OpenCode use their own
session APIs; Claude appends its existing title metadata. Without a name,
the launcher reads the runtime's current title immediately. Without a
launcher, a bare MCP face holding only its agent's credential refuses a
named rename and changes nothing, because the new address is its User's
record: renaming is for `ab-*` sessions only.

The launcher owns the transition: claim a unique address as the launching
account, acquire its credential, update the runtime title, stop the old inbox
reader, persist the new session binding and private environment, then read
the new inbox. All MCP clients read that shared identity before tool calls.
Concurrent rename and metadata refresh operations are serialized; repeated
renames reuse the selected address. A now-unneeded collision suffix may be
removed when the base address becomes available.

After a rename, the replacement pusher retries a still-held read with backoff
for at most ten seconds, because client cancellation can finish before the
daemon releases its waiter. If another reader still holds the address, push
stops with a loud `push inactive` diagnostic naming it; it never takes over
that reader. An ordinary startup still refuses a competing reader immediately.

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
[Codex entry point](../src/launchers/ab-codex), the
[opencode entry point](../src/launchers/ab-opencode) — or the archived
[1.x one](../src/launchers/ab-opencode1) on a host whose opencode is older —
or the relocatable output of
[the build script](../src/README.md#build-and-check). Bun and the selected
runtime must be installed. Local socket discovery needs no environment setup;
explicit addresses and tokens use the [face environment](../src/mcp/README.md#environment).

The launcher registers the session as its owner, obtains the session's own
token and passes only that credential to its tools. On a mapped user socket,
it switches to the shared listener for session calls and verifies the resulting
identity. Runtime configuration is temporary. Claude's channel uses the installed marketplace plugin; see [one-time setup](#claude-channel-plugin).
A plain Claude session without launcher identity has no bus face to serve.
Per-runtime executable overrides and state paths are described
by each launcher's help output.

The launcher owns its local App Server; attaching to another server and worktree
creation are unsupported. After an uncatchable launcher kill, stale lock directories
may need removal from its state directory after checking the recorded PID is dead.
Resuming the same saved session binding from copied state on different hosts is
not coordinated by the local session locks; default state stays on its launching host.

## Claude channel plugin

Run once per Claude account (`CLAUDE_CONFIG_DIR` selects an alternate account):

```sh
claude plugin marketplace add /usr/local/src/ai-agent-bus/src
claude plugin install agent-bus@agent-bus-local --scope user
```

For a packaged release, use its installation's `current` directory instead of
`/usr/local/src/ai-agent-bus/src`. Both contain `.claude-plugin/marketplace.json`.
After updating the package, refresh it with `claude plugin update agent-bus@agent-bus-local`.
If an older launcher registered a local MCP server named `agent-bus`, remove
that old definition once from each affected launch directory:
`claude mcp remove --scope local agent-bus`. Keep other servers and scopes.
The plugin now provides that server; loading both would create competing readers.

`ab-claude` always passes `--channels plugin:agent-bus@agent-bus-local`, with
its namespaced tools allowed. It never uses or falls back to the development
channel flag, and never answers a confirmation dialog. The plugin gets its
face path and private session environment from the launcher; persistent plugin
files contain no bus token, name or socket. Other runtimes are unchanged.

### Team and Enterprise administration

The org Owner enables **Organization settings → Claude Code → Channels**.
In the effective managed policy, merge these keys, retaining other settings:

```json
{
  "channelsEnabled": true,
  "allowedChannelPlugins": [{"marketplace":"agent-bus-local", "plugin":"agent-bus"}]
}
```

On a Linux host without a higher managed source, use root-owned
`/etc/claude-code/managed-settings.json` (0644); back up an existing file and
merge it. `allowedChannelPlugins` **replaces Anthropic's default channel list**;
organizations using official channel plugins must include those entries too.

Server-managed policy wins over the host file by default, even for keys it
leaves unset. An org Owner must then merge the allowlist in **Organization
settings → Claude Code → Managed settings** on claude.ai. Restart the session
after the policy refresh. Do not edit the local remote-settings cache or use
a development flag to bypass policy. See the official [channel controls](https://code.claude.com/docs/en/channels#enterprise-controls)
and [managed-source order](https://code.claude.com/docs/en/managed-settings#how-claude-code-combines-managed-sources).

A visible channel warning means push is unavailable even if MCP tools connect.
If the cached org policy is stale, start a fresh Claude session: startup
refetches `~/.claude/remote-settings.json`. Confirm that it contains the
allowlist; do not edit that cache. Enabling Channels alone is insufficient
when the private plugin is not allowlisted. First-use workspace trust is
separate from channel approval; trust the intended working directory before
starting it unattended.

## Runtime isolation and recovery

Every shipped runtime must keep session control private to its OS account.
**Built:** launcher-owned Codex and OpenCode servers require per-run credentials;
launcher rename control requires its own capability. Loopback alone is not a
boundary.

<details>
<summary>Credentials and measured scope</summary>

Codex uses native capability-token authentication before WebSocket upgrade.
Its token file is private inside the launcher's private run directory; the
pusher supplies an Authorization header and the TUI reads its token from an
environment variable. It requires a runtime supporting `--ws-auth capability-token`
and `--remote-auth-token-env`; unsupported versions fail startup, with no
unauthenticated fallback. OpenCode uses its native server password. Before sending
either credential, the launcher verifies that the listening socket belongs to
its spawned runtime or a descendant, using Linux `/proc` or `lsof` and `ps`
on other systems. An unknown or foreign owner receives no authenticated probe;
a collision gets one fresh-port restart, then a named startup error. Malformed
session listings are refused by name rather than treated as empty history. Secrets
never appear in launcher-supplied command-line arguments. Root and processes
already running as the same OS account are outside this account boundary. If the runtime exits, an in-flight or immediate next request can reach a process that rebinds its freed port before the exit notification ends the session.

The [endpoint evidence](../Plans/done/R0.8-MVP/done/runtime-endpoint-auth.md#checks) includes
actual second-account reads and renames, positive controls with authentication
removed, and separate launcher/MCP/pusher fixture checks. [H.9.5](../Plans/done/R0.8-MVP/DONE.md#done--mvp)
passed for all three runtimes; Claude's channel needs a signed-in claude.ai
login ([Claude channel checks](../Plans/done/R0.8-MVP/done/runtime-interactive.md#claude-channel-checks)).

</details>

After a daemon restart or sidecar failure, an open interactive session must
resume bus delivery or clearly report that integration is inactive and how
to recover. Recovery instructions must lead to a successful correlated
exchange in that session. It is accepted live for all three
runtimes ([recovery evidence](../Plans/done/R0.8-MVP/done/runtime-recovery.md#checks)):

| Failure | What the session does |
|---|---|
| Graceful daemon restart; bus-child crash | Stays open and resumes by itself, under the same name and runtime session: push keeps asking after a short backoff, each tool call is its own request, and names and credentials are durable |
| MCP face dies under Codex or OpenCode | The launcher reports the bus tools inactive and stops push, so new messages stay queued; it restarts the face through the runtime and reports delivery resumed |
| MCP face dies under Claude | The launcher reports tools and channel inactive and says to type `/mcp`, select agent-bus and choose Reconnect; Claude owns its MCP servers |
| Face still down after 20 s; push inactive | The launcher prints the resume command for the same session |
| Codex App Server or OpenCode server dies | The session ends; the launcher prints `ab-codex resume <id>` or `ab-opencode --session <id>`, which reopens the same session under the same name |

<details>
<summary>How a lost face is noticed</summary>

The face runs under the runtime, not the launcher, and no runtime reports its
death or restarts it unasked. Each face leaves a mark in the launcher's private
run directory and clears it when its runtime lets it go; a mark whose process
is gone is a face that died. Codex restarts it on `config/mcpServer/reload`,
OpenCode on an MCP disconnect and connect.

</details>

This adds no replay or exactly-once guarantee:
[consumption](04-messaging.md#one-reader-per-inbox) and
[queue checkpoints](04-messaging.md#durability) retain their loss boundaries. A message
consumed at the moment of a failure may be lost, and after a crash one consumed
since the last flush may arrive again under its own `message_id`.

## Status

| MVP | Scope |
|---|---|
| Built | Runtime adapters and smart launchers with MCP tools. |
| Accepted | [Runtime integration delivery](#runtime-integration-delivery), [smart launchers](#smart-launchers), [session names](#session-names) and [isolation and recovery](#runtime-isolation-and-recovery) passed live acceptance on the development host ([launchers](../Plans/done/R0.8-MVP/done/runtime-launch.md#checks)) and, installed from the package, on a [fresh host](../Plans/done/R0.8-MVP/done/fresh-host-runtime.md#checks). Codex's enforced mode is also given to the TUI that runs its thread ([gap closed](../Plans/done/R0.8-MVP/done/runtime-launch.md#defect-found)). |
