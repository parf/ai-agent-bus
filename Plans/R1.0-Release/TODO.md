# TODO R1

## Objective

Migration labels: `RT` identifies the original unnumbered work rows; `RBK`
identifies the backup acceptance paragraph. Existing M/B/P wave IDs are unchanged.

Prepare the [proposed scope](README.md#scope). Started ahead of the release on
the 0.8 line; no release wave is committed yet. Item glyphs follow
[plan markup](../MARKUP.md#glyphs); built results are in [DONE](DONE.md).

- ✅ RT1 [Shared locks](done/locks.md#shared-locks)
  - State: built in 0.8.66

- ✅ RT2 [Resource records](done/resources.md#resource-records)
  - State: built in 0.8.70

- ✅ RT3 [Key-value store](done/kv.md#per-record-storage)
  - State: built in 0.8.77

- ✅ RT4 [Roles reach Agents](roles.md#roles-are-for-agents): transport, generated `owner` and `maintainer`, supported list
  - State: built in 0.8.97–0.8.99

- ✅ RT5 [Client libraries](done/client-libraries.md#scope): Python, TypeScript, Go, Rust, Java and PHP, with native examples
  - State: built in 0.8.100
  - Evidence: Built in 0.8.100, all six with examples and slow-gate checks; conformance-first protocol specification is settled as [№RD90](DECISIONS.md#recorded-decisions), implementation in №RT8

- ✅ RT6 [Assigned roles](roles.md#role-syntax) in allow lists
  - State: built in 0.8.110; [checks](done/assigned-roles.md#verification)

- ✅ RT7 [Record tags](tags.md#classification-and-filtering)
  - State: built 0.8.143; [verification](done/record-tags.md#verification); multiple-tag matching is settled and not built ([№RTA1](#record-tags-acceptance))

- ⏳ RT8 A shared executable protocol conformance suite for the clients
  - Details: start with the calls the six clients already support; explanatory prose may accompany executable examples, with no new schema or generator
  - Acceptance: each client passes the same request/answer cases; a changed request or answer in one client fails the corresponding case independently
  - Mutant: change one client's emitted request or interpretation of an answer and the common case must fail
  - Home: [protocol conformance](modules.md#protocol-conformance), [№RD90](DECISIONS.md#recorded-decisions)

- ⛔ RT9 [Agent method metadata](method-metadata.md#method-metadata)
  - Depends: The MVP description-only behavior it replaces ([decision](../../docs/decisions.md#settled)); an owner-approved representation
  - Format: needs an owner-approved representation

- ⚠️ RT10 [Runner](runner.md#runner)
  - Details: decided; [waves](#runner-waves) proposed
  - State: one-message serving built 0.8.123; full evidence remains in [runner waves](#runner-waves)
  - Depends: [method metadata](method-metadata.md#method-metadata); the pool hostname field, now R1 ([where a member says it is](runner-implementation.md#where-a-member-says-it-is))

- ⚠️ RT11 [Runners section in the web face](runner/web-face.md#goal) — the status-and-reload slice built 0.8.198; start/stop waits on runner methods
  - Depends: lifecycle actions depend on runner capabilities
  - Home: [Minimum and acceptance checklist](runner/web-face.md#minimum-checklist)

- ⛔ RT12 [Agents for the runner](agents.md#what-moved-and-what-r1-adds): telegram, systemd, logwatch
  - Depends: Telegram and logwatch: the runner keeping an agent it did not write (№M.1 and №M.2, built; №M.11 for the install) and the kept process form for state between messages (№M.3, №M.9). Systemd needs none of them — it is a root unit of its own running `agent-bus run`
  - Settled by: owner, 2026-10-17
  - Home: [tasks](#bundled-agents)

- ⏳ RT13 [Backups](runner-implementation.md#backing-it-up)
  - Details: decided: the runner's `backup` verb

- ⛔ RT14 [Installable distribution](distribution.md#release-artifacts): P.1–P.3
  - Format: publication names and platforms

- ✅ RT15 Federation moved to R1.2
  - Moved: owner, 2026-10-17 (№Q137); [R1.2 task](../R1.2/TODO.md#federation)
  - Home: [chaining](../R1.2/federation.md#chaining), [№RD91](DECISIONS.md#recorded-decisions)

## Next step

Assigned roles are built ([verification](done/assigned-roles.md#verification)).
Resolve method metadata and distribution choices while continuing the runner
waves and the accepted [protocol conformance task](#objective). Federation is
[R1.2](../R1.2/federation.md#chaining); its R1.1 prerequisites no longer block
this release. Existing decisions describe targets, not completed code.

## Dependencies

Repeated work has one editable item under Objective, with its dependency
cells retained there. Distinctly named work keeps its own item here:

- ✅ RT16 [Record-defined roles](roles.md#record-defined-roles)
  - Evidence: Built using the existing allow list and nested membership; [current contract](../../docs/03-records-agent.md#roles)

- [№RT15](#objective) — Federation moved to [R1.2](../R1.2/TODO.md#federation)
- [№RT10](#objective) — Runner
- [№RT12](#objective) — [Agents for the runner](agents.md#dependencies)
- [№RT9](#objective) — [Agent method metadata](method-metadata.md#method-metadata)
- [№RT5](#objective) — [Client libraries](done/client-libraries.md#scope)

- ⛔ RT17 Release distributions
  - Depends: Release builds and runnable daemon/runner roles; choose publication names and supported platforms before packaging

Name implementation waves and falsifiable acceptance after those choices.
Federation's distinct-node acceptance moved with [its task](../R1.2/TODO.md#federation).

## Record tags acceptance

Built 0.8.143. [Current contract](../../docs/03-records.md#tags),
[verification](done/record-tags.md#verification). Multiple-tag matching is
settled — [№RD85](DECISIONS.md#recorded-decisions) — and not built; the
implemented UI selects one label.

`RTA…` labels this section's rows.

- ⏳ RTA1 A multiple-tag query selects the records carrying every tag listed
  - Details: comma-delimited, as `tag=a,b`. A single tag is the request that
    exists today, so no page or client changes; an unknown tag gives an empty
    result rather than an error, and `no-tags` cannot combine with a real tag,
    since a record either has no tags or has that one
  - Acceptance: two tags return only the records holding both, and one tag
    returns what it returns today
  - Mutant: match any of the listed tags instead of all, and the two-tag check
    must fail
  - Home: [record tags](../../docs/03-records.md#tags)

## Group walk acceptance

Settled as [№RD87](DECISIONS.md#recorded-decisions). `RGW…` labels this
section's rows.

- ⏳ RGW1 Every group-membership walk is iterative, as role assignment's already is
  - State: role assignment built (`assigned_roles.go:62-91`); the ACL check,
    directory index and publication expansion recurse
  - Details: each keeps its processed-group marking and the direct-only
    Administrator boundary; no membership result changes
  - Acceptance: a chain of nested groups deep enough to exceed a deliberately
    small stack limit resolves through all four walks, and the existing
    nested-group and cycle checks stay green
  - Mutant: restore recursion in any one walk and the deep-chain check must fail
  - Home: [groups](../../docs/01-identity-and-authority.md#groups)

## Runner waves

Proposed 2026-10-16, for owner agreement before any code. Each wave is its own
version bump, `--slow` gate and peer review. A wave that needs a format first
proposes it for approval ([working rules](../../CLAUDE.md#working-rules)).
Rows run in the order listed; №M.9 and №M.8 are placed by what they need.

Until №M.8, an agent is installed the way the runner's
[use case](runner.md#how-it-works) does it: `git clone` into `agent.d/`, its
configuration written under `runner/` on the host, then a `reload` call.

- ✅ M.1 The runner is an agent
  - State: done: the runner's User, 0.8.114; the program with `status` and `reload`, 0.8.117; its unit, 0.8.118; deployed 2026-10-16. Its identity was replaced by №M.10 (the person's agent `#runner/<instance>@<host>`) and its install by №M.11 (an account per person)
  - Builds: a new `agent-bus-runner` program and unit (`Delegate=yes`); registers `#runner@<host>`; the `agent-bus-runner` account given a principal so its socket is served; reads `agent.d/` and `runner/agents.json`; `reload` and a `status` call; its own heartbeat; each child's output captured to its log
  - Format: `agent.json`, `agents.json` fields; the `status` answer
  - Acceptance: the runner registers, stays alive in listings by its heartbeat, and answers `status` listing what is installed; drop the principal and the socket check fails; a malformed `agent.json` is reported by name, and skipping it silently fails
  - Mutant: drop the principal and the socket check fails; a malformed `agent.json` is reported by name, and skipping it silently fails

- ✅ M.2 It serves agents
  - State: deployed 2026-10-17 at 0.8.124, serving `#debug`; built 0.8.123: the read filter, forwarding, `args` and `json`, env layers, templates and instances, bounded answers. Reading by forwarding and the deployer as Maintainer were replaced by №M.10: each agent is its person's and is read on its own inbox
  - Builds: each enabled agent registered with `deliver_to` to the runner; dispatch on `original_to`; the `args` and `json` forms (the dedicated runner's serve loop, moved into a shared module); the reply comes from the addressed name; an unknown `original_to` refused to its sender; env layers, templates and instances; stats reported per agent
  - Format: the env files' syntax; the read filter's name on the consume call ([№Q162](runner-implementation.md#many-names-into-one-inbox))
  - Acceptance: a call to `#hello@h` is answered by `#hello@h` through the runner; a message for an unserved name gets an error back, holding it fails; an instance missing a required variable is refused at `reload`; a script's output beyond a bound is cut and said, never buffered whole in the runner; each agent registered with its deploying User as a Maintainer ([who it runs as](runner-implementation.md#who-it-runs-as)); answers are sent off the read loop, so a slow send never leaves the runner without a reader; a template shows in `status` as one, not as a stopped agent
  - Mutant: Not recorded in the source.

- ⏳ M.3 Processes
  - Builds: `processes` counts and ranges; the `jsonl` kept form with `in_flight`; `SIGTERM`/`SIGHUP`, the 5-minute idle, the deadline, restart with backoff
  - Format: the `jsonl` frame, carrying each message's bus ID ([№Q160](DECISIONS.md#recorded-decisions))
  - Acceptance: a pool grows under load and shrinks after idle; a killed child is restarted and reported; an unhandled `SIGHUP` restarts it; removing the idle stop fails a bounded check; `in_flight` 2 holds two messages at once and answers both in any order
  - Mutant: removing the idle stop fails a bounded check;

- ✅ M.9 Agents that serve themselves
  - State: built 0.8.155: kept at a fixed count with its own token and the bus socket, restarted with backoff, restarted at `reload` when its env or command changed, stopped by `SIGTERM` then its cgroup killed; its Owner registers it and hands the token over; the Slack agent packaged by `src/agents/slack/package.sh`
  - Builds: `serves: itself`: started and restarted, never registered by the runner, a fixed count only
  - Acceptance: such an agent registers itself and reads its own inbox; the runner registering it fails the check; a range on it is refused at `reload`
  - Mutant: no restart, no restart on an env change, a stopping runner leaving it, a range accepted, no token handed, the runner registering it: each fails a smoke check

- ✅ M.4 Sandboxing
  - State: built: bubblewrap namespaces, the filesystem and network rules and a clean environment, 0.8.126; a cgroup per agent with `CLONE_INTO_CGROUP`, `limits`, `needs.memory`, sized `/tmp` and failing closed without a delegated subtree, 0.8.150; the system-call filter and the soft disk limit, 0.8.151
  - Builds: one cgroup per agent in the delegated subtree, `CLONE_INTO_CGROUP`; bubblewrap with a PID and network namespace; the system-call filter; `limits` and `needs`; sized `/tmp`; the soft work-directory limit; fail closed
  - Format: the filter's library ([modules](../../src/MODULES.md#external-tools))
  - Acceptance: an agent cannot see a sibling's processes, read a sibling's files or env, connect to the runner's or another account's socket, write outside its work directory, or exceed its memory; on a host without `bwrap` it is refused; each removed limit fails its check
  - Mutant: each removed limit fails its check

- ⏳ DR.1 The dedicated runner applies the agent's bus secret
  - Builds: `agent-bus run` reads its agent's own bus secret with the agent's token and puts it in the script's environment, as the runner does for a script agent, parsed by the shared [env-file parser](../../docs/03-records-service.md#env-file-syntax); stated in the sandbox job's environment, since a `--sandbox on` child starts from the user manager's, not the dedicated runner's; read per message, so `agent-bus secret` takes effect without a restart; never in `agent-bus logs`, the process title or an error
  - Precedence: the secret wins over the shell's value of the same name ([№Q174](DECISIONS.md#recorded-decisions)); the message stamps still win over both
  - Also: the bundled debug agent's `env` answer leaves out the record's secret keys — today, under the runner, it hands the secret to any caller its ACL admits
  - Acceptance: a script started by `agent-bus run` sees `KEY=value` from its record's secret, sandboxed and not; a changed secret reaches the next message; no secret value appears in its log or title; the debug agent's `env` names no secret value
  - Mutant: skip the secret layer, read it only at start, or leave it out of the sandboxed job's env, and a check must fail

- ⏳ M.5 The host's controls
  - Built in 0.8.207: `enable`/`disable`/`start`/`stop`/`restart` calls ([controlling one agent](../../docs/runner/operations.md#controlling-one-agent), [№Q181](DECISIONS.md#recorded-decisions))
  - Builds: override files (JSON Merge Patch, mounted); `depends` ordering; the `backup` verb; a configuration change applied at `reload` ([№Q19](runner.md#stop-and-reload))
  - Format: how an archive larger than one answer travels
  - Acceptance: an override changes the agent's view of its file and a refused key is named; a cycle in `depends` is refused; `backup` restores with the intended key only; at `reload` a changed agent's kept processes get `SIGHUP` and an untouched sibling gets none; a changed env file or bus secret restarts its kept processes after their work drains ([№Q161](DECISIONS.md#recorded-decisions))
  - Mutant: Not recorded in the source.

- ⏳ M.6 Pools and edges
  - Builds: `share`; the member hostname field (daemon change); edge runners under the bus's realm
  - Format: the hostname field; pool members read the pool's own inbox ([№Q159](DECISIONS.md#recorded-decisions))
  - Acceptance: two runners serve one name taking turns and a listing shows both hosts; an edge runner registers as `#runner/<edge>@<realm>`
  - Mutant: Not recorded in the source.

- ⏳ M.7 Named hosts
  - Builds: the filtering proxy for `needs` hosts
  - Acceptance: a named host is reached and another refused; bypassing the proxy has no route
  - Mutant: Not recorded in the source.

- ✅ M.10 The runner acts as its person
  - State: owner decisions 2026-10-17, moved from R1.1; built: `register` answers the token of an agent an Agent created, 0.8.141; the runner as `#runner/<person>`, creating its agents create-only, keeping their tokens and reading each one's own inbox, 0.8.142; the Linux account `<person>-ab-runner`, with №M.11, 0.8.149; a transferred or rotated agent dropped at reload, 0.8.153
  - Builds: the runner runs as a Linux account of its own, likely `$user-ab-runner`, so its secrets stay apart, but has **no bus identity of its own**: it is the person's agent `#runner/<person>`, and the person owns every agent it serves. **`register` answers with the new agent's token when, and only when, that call created the record**, so the runner creates an agent, keeps its token in its private directory and makes itself the agent's Maintainer. An agent that already exists is handed over once by its Owner. No new flag. Replaces the keyless User `agent-bus-runner`, and makes the serve-only grant ([№Q164](DECISIONS.md#recorded-decisions)) unneeded
  - Format: the `token` field on the register answer
  - Acceptance: a re-registration of an existing record answers no token; a register that created the record answers its token, which works; the runner never holds its person's token and serves no name it has no token for; removing the created-only guard lets a re-registration collect an existing agent's token, and the check fails
  - Mutant: removing the created-only guard lets a re-registration collect an existing agent's token, and the check fails

- ⚠️ M.11 Runner install through setup
  - State: owner, 2026-10-17; built 0.8.149: `--runner install|uninstall|enable|disable --for <person> [--purge]`, the `<person>-ab-runner` account and `agent-bus-runner@.service` template, nothing installed by default; [a person's runner](../../docs/07-setup.md#a-persons-runner)
  - Builds: `agent-bus-setup` options that install, uninstall, enable and disable a person's runner: the Linux account, its home, the unit, and with №M.10 the person's `#runner/<person>`; nothing installed by default
  - Format: the option names
  - Acceptance: each verb leaves the host in its stated state and a second run changes nothing; uninstall leaves no account, unit or link behind and keeps the person's `agent.d` unless asked; disable stops and disables the unit and keeps the files; a verb for a person who is not a bus User is refused
  - Mutant: Not recorded in the source.

- ⏳ M.12 Agent code from a list of directories (owner, 2026-10-17)
  - Builds: the runner reads agent code from an ordered list of directories: first the person's own `agent.d` (`/var/lib/agent-bus/runners/<person>/agent.d`), then the release's bundled agents (`/usr/local/lib/agent-bus/current/agents`, or the checkout's `src/agents` on a node running from git), so every bundled agent is always there and a new one arrives with an upgrade; each bundled agent is installed in its packaged form (the Slack agent through its `package.sh`) by the release build
  - Rules: a name in an earlier directory wins over the same name in a later one, so a person's copy overrides a bundled one; `status` says which directory an agent came from; a bundled agent is opt-in, listed in `status` as available and served only once `agents.json` enables it, while one in the person's own `agent.d` stays enabled by default ([№Q175](DECISIONS.md#recorded-decisions))
  - Acceptance: a bundled agent appears in `status` as available, not served, with no copy in `agent.d`, and is served once `agents.json` enables it; a person's own copy of the same name is the one served; an upgrade that adds a bundled agent shows it at the next `reload`; removing the person's copy falls back to the bundled one
  - Mutant: read only the first directory, let a later directory win a name, or serve a bundled agent nobody enabled, and a check must fail

- ✅ M.13 An agent says it is not configured (owner, 2026-10-18: "i agree - accept your idea")
  - Builds: an agent of `agent-bus-runner` that exits with code **78** (`EX_CONFIG`, sysexits.h) says it is not configured; the last line it wrote to stderr, if any, is the reason. The runner does not restart it and does not read its inbox any more: `status` shows `state: "unconfigured"` and the reason, and messages wait in its queue as for any agent not being served. A script agent's triggering message is answered `error: <agent> is not configured: <reason>`. Only `reload` tries it again: no timer, no retry. The bundled Slack agent exits 78 when its tokens are missing
  - Scope: `agent-bus-runner` only; the dedicated runner (`agent-bus run`) keeps its behaviour for now
  - Docs: the code and the state in [runner operations](../../docs/runner/operations.md) and [configuration](../../docs/runner/configuration.md)
  - Acceptance: a kept agent exiting 78 is not restarted and shows `unconfigured` with its reason; a script agent exiting 78 answers its caller with the error, and the next message waits unread; `reload` after a fix serves it again; any other exit code keeps today's restart and failure behaviour
  - Mutant: restart on 78, keep reading the inbox, drop the reason, or answer nothing, and a check must fail
  - Built: de0a205 (0.8.193) by Codex; OpenCode reviewed it with no P1/P2, and one P3 (a comma-ok on the stderr writer's type assertion) is Codex's

- ⏳ M.8 Deploy over the bus
  - Builds: install, configure and remove as calls to the runner ([reaching the runner](runner-implementation.md#reaching-the-runner)); configuration write-only
  - Format: the calls' bodies; how code arrives
  - Acceptance: an agent is installed and served with no shell on the host; reading a configuration back is refused; a caller outside the runner's ACL is refused
  - Mutant: Not recorded in the source.

## Bundled agents

Asked for by the owner on 2026-10-17. Each is an entry of the
[R1.2 catalogue](../R1.2/services.md#the-catalogue) brought to R1 with the
runner that keeps it, except systemd, which the runner cannot keep and which
stands alone ([agents for the runner](agents.md#what-moved-and-what-r1-adds)).
The catalogue's own rules hold — the daemon applying the ACL before delivery,
and no daemon change. A row that needs a format first proposes it for approval.

- ⏳ B.1 telegram — [plan and tasks](telegram/README.md#telegram-agent), TG.1–TG.7
  - Builds: `#telegram/<account>@<realm>`: calls out, and a subscription forwarding a chat to an agent, 📮 queue or 📣 pubsub; the bot token read from the record's bus secret and nowhere else; `#group` and `@user` resolved to a chat id; a default channel declared in the instance's configuration ([telegram](agents.md#telegram))
  - Format: settled 2026-10-18 as the Slack agent's methods where Telegram allows ([methods](telegram/README.md#methods)); the configuration's fields are proposed in TG.1
  - Acceptance: a post reaches its chat and answers what identifies it; a call with no destination goes to the instance's declared default, and the same call against an instance that declares none is refused; an unknown alias is refused rather than resolved to anything; an upstream refusal comes back as an answer; a subscription's match reaches its target and a non-match does not. Mutants: the token read from the process env or from `telegram.toml` at run time; the declared default replaced by the script's first-configured-chat fallback, which must fail the no-default refusal; the alias miss falling back instead of refusing
  - Mutant: the token read from the process env or from `telegram.toml` at run time; the declared default replaced by the script's first-configured-chat fallback, which must fail the no-default refusal; the alias miss falling back instead of refusing

- ⛔ B.2 systemd
  - Builds: `#systemd/<set>@<host>`, **standalone**: its own install script writes a root unit running `agent-bus run` with the sandbox off, reaching the bus over IP with a token from the unit's `EnvironmentFile`. `list` and `status` at no role, `start`/`restart`/`reload` at the operate tier, `stop`/`enable`/`disable` at admin, over the unit set its instance declares ([systemd](agents.md#systemd))
  - Depends: the owner-approved tier names in Format below
  - Format: the tier names ([open choices](agents.md#open-choices))
  - Acceptance: a caller with no role lists and reads status and is refused `start`; an operate caller starts, restarts and reloads and is refused `enable`; an admin caller reaches all of them; a unit outside the instance's declared set is refused at every tier including admin; the install script run twice leaves one unit and one registered name, and its removal leaves no unit, no token file and no orphaned record. Mutants: the role check removed, so a no-role caller starts a unit; the unit-set check removed, so admin reaches an undeclared unit; the tier read from the call body instead of the delivered roles, which must fail for a caller claiming a role it was not granted; the unit given `NoNewPrivileges=yes`, which must fail every acting verb and is what says the agent is standalone for a reason
  - Mutant: the role check removed, so a no-role caller starts a unit; the unit-set check removed, so admin reaches an undeclared unit; the tier read from the call body instead of the delivered roles, which must fail for a caller claiming a role it was not granted; the unit given `NoNewPrivileges=yes`, which must fail every acting verb and is what says the agent is standalone for a reason

- ⛔ B.3 logwatch
  - Builds: `#logwatch/<sources>@<host>`: a bounded ring over file globs, journald units and syslog; last N verbatim; an asked-for filter; subscriptions forwarding matching lines to an agent, 📮 queue or 📣 pubsub ([logwatch](agents.md#logwatch))
  - Depends: the syslog input and source-instance choice in Format below
  - Format: which syslog input it takes; whether a source set is one instance or several
  - Acceptance: the last N come back unfiltered by default and oldest first; a filter is applied only when asked for; a line arriving after a subscription was added reaches its target and a non-matching one does not; a source that appears after start is picked up; writing far beyond the ring's size leaves the ring at its bound and the process at its memory limit. Mutants: the ring's bound removed, which must fail the bounded check rather than hang it; a default filter applied when none was asked for; a new file never rescanned
  - Mutant: the ring's bound removed, which must fail the bounded check rather than hang it; a default filter applied when none was asked for; a new file never rescanned

- ⛔ B.4 Choose the next set
  - Depends: the owner picking the next set in Acceptance below
  - Acceptance: the owner picks from [still to approve](agents.md#still-to-approve), or names others; each chosen entry gets its own row here before any code
  - Mutant: Not recorded in the source.

- ⏳ B.5 IMAP client
  - Builds: the R1 [IMAP client](agents.md#mail-clients), kept by the runner
  - Acceptance: against a fixture IMAP server, read the configured mailbox and publish the expected message to its configured bus channel; a server refusal is reported without exposing credentials
  - Mutant: omit the fetched message or report a refused read as successful; each must fail its corresponding check

- ⏳ B.6 SMTP client
  - Builds: the R1 [SMTP client](agents.md#mail-clients), kept by the runner
  - Acceptance: against a recording SMTP server, send the requested message to its intended recipient exactly once; a server refusal is reported without exposing credentials
  - Mutant: change the recipient, submit twice, or report a refused submission as successful; each must fail its corresponding check

## Backup acceptance

- ⏳ RBK1 Backup acceptance
  - Acceptance: The [backup contract](runner-implementation.md#backing-it-up) is settled, invoked as the runner's `backup` verb. Implementation must restore an archive with the intended user's key and refuse an unrelated key. Replace encryption with plaintext output and the format/decryption check must fail; encrypt to the wrong recipient and the intended-user restore must fail. Confirm backup creation works with only the public key available.
  - Mutant: Replace encryption with plaintext output and the format/decryption check must fail; encrypt to the wrong recipient and the intended-user restore must fail.

## Distribution acceptance

Implement the [release artifacts](distribution.md#release-artifacts) and [container contract](distribution.md#container-runtime).

- ⏳ P.1 npm package
  - Acceptance: On a clean supported host without the checkout or Go toolchain, install the published package, start the bus, complete an agent request/reply and an MCP tool call. Omit a required executable or MCP runtime asset from the package and the corresponding exercise fails
  - Mutant: Omit a required executable or MCP runtime asset from the package and the corresponding exercise fails

- ⏳ P.2 Container image
  - Acceptance: Pull the published image on a clean host, start each role from the supplied instructions and complete an agent request/reply. Break either role's entry point and delivery fails. Recreate the daemon container with its volume and verify the registered service and issued credential still work; move storage outside the volume and recovery fails. Request unavailable sandboxing and require refusal; silently downgrading must fail the check
  - Mutant: Break either role's entry point and delivery fails. silently downgrading must fail the check

- ⏳ P.3 Release evidence
  - Acceptance: Query every shipped program's version in both distributions and compare with the release source; require stamped Go build information and packaged license. Substitute an unstamped binary, alter one version or omit the license: each fails its corresponding check
  - Mutant: Substitute an unstamped binary, alter one version or omit the license: each fails its corresponding check
