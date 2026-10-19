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

- ⏳ RT9 [Agent method metadata](method-metadata.md#method-metadata)
  - Format: settled with the owner on 2026-10-19 ([settled so far](method-metadata.md#settled-so-far)): typed fields in `agent.json`, commented `env.dist`, and the `help`, `help-config` and `help-secret` asks; MCP generation is [R1.2](../R1.2/TODO.md#dependencies)
  - RT9.1 The runner and `agent-bus run` read the new `agent.json` fields strictly and answer `help`, `help-config` and `help-secret` themselves for packaged agents, without running the script; the CLI and MCP face send the two new asks (`agent-bus call <to> --help-config`, `--help-secret`). Acceptance: a packaged fixture with no help code answers all three from its files; `help-secret` names keys and comments and never a value (sentinel); an unknown field is refused by name; an enum value outside the list is shown. Mutants: the script run for a help ask; a secret value in `help-secret`
  - RT9.3 The web face shows it all (owner, 2026-10-19: "once done, the web must show all this data"): an agent's record page gets an "About this agent" card (description, version, license, author, homepage), a Methods table (name, arguments with types and enums, returns, role, destructive), and the Configuration editor shows each documented key's type, help and example beside the JSON, warning about undocumented keys. The Secret field lists the expected keys from `env.dist` with their comments and which are required, never a value. The web gets it through the same asks, or from the runner for its packaged agents. Acceptance: a fixture agent's page shows every field; a secret sentinel never appears; an agent answering only `--help` shows that text and no tables. Mutants: the secret value rendered; an enum dropped
  - RT9.2 Every bundled agent's `agent.json` and `env.dist` carry the metadata (methods, args, Configuration keys, description, license, author, homepage) and their hand-written help code is removed where RT9.1 covers it; the guides link `--help-config` and `--help-secret`. Acceptance: each agent's `--help` matches its real methods (a parser check per agent); removing a method from the code without its entry fails the check

- ⚠️ RT10 [Runner](runner.md#runner)
  - Details: decided; [waves](#runner-waves) proposed
  - State: one-message serving built 0.8.123; full evidence remains in [runner waves](#runner-waves)
  - Depends: [method metadata](method-metadata.md#method-metadata); the pool hostname field, now R1 ([where a member says it is](runner-implementation.md#where-a-member-says-it-is))

- ⚠️ RT11 [Runners section in the web face](runner/web-face.md#goal) — status, per-agent controls, host files and instances are built; remaining acceptance is tracked in the checklist
  - State: the information page and per-agent management page are separate; denied management shows Access denied
  - Home: [Minimum and acceptance checklist](runner/web-face.md#minimum-checklist)

- ⚠️ RT12 [Agents for the runner](agents.md#what-moved-and-what-r1-adds): telegram, systemd, logwatch
  - Depends: Telegram and logwatch: the runner keeping an agent it did not write (№M.1 and №M.2, built; №M.11 for the install) and the kept process form for state between messages (№M.3, №M.9). Systemd needs none of them — it is a root unit of its own running `agent-bus run`
  - Settled by: owner, 2026-10-17
  - Home: [tasks](#bundled-agents)

- ⏳ RT13 [Backups](runner-implementation.md#backing-it-up)
  - Details: decided: the runner's `backup` verb

- ⛔ RT14 [Installable distribution](distribution.md#release-artifacts): P.1–P.3
  - Format: publication names and platforms
  - Editions: [full and client-only](distribution.md#two-editions), the latter the runner and the basic CLI (owner, 2026-10-19)

- ✅ RT15 Federation moved to R1.2
  - Moved: owner, 2026-10-17 (№Q137); [R1.2 task](../R1.2/TODO.md#federation)
  - Home: [chaining](../R1.2/federation.md#chaining), [№RD91](DECISIONS.md#recorded-decisions)

- ✅ RT18 Configuration is public
  - Built: configuration display/edit for Owner and Maintainers; next-start `AGENT_BUS_CONFIG`; [current contract](../../docs/03-records-agent.md#configuring-a-template).
  - Settled by: owner, 2026-10-19: "Configuration (JSON) is now public; should be passed to shell scripts as ENV; suggest to use jq to extract values; always show on web; use pretty print to show/edit; compact on save"
  - Details: mostly docs and the web face. "Public" means not treated as a secret: the record's Owner and Maintainers see and modify it, in every face including the web, and nobody else does (owner, 2026-10-19: "Owner and Maintainer - can see & modify"). Others still get neither the bytes nor `config_sha`. A Secret stays private and unchanged.
    - Scripts: the runner and `agent-bus run` pass the compact JSON to the script in one environment variable `AGENT_BUS_CONFIG` (owner, 2026-10-19); the docs show `jq` extracting a value, e.g. `jq -r .channel <<<"$AGENT_BUS_CONFIG"`. A record with no configuration leaves the variable unset, so a script can tell it from `{}`. A changed configuration reaches a script at its next start: a per-message script on its next run, a kept process after a restart or the runner's `reload` (owner, 2026-10-19).
    - Web: the record page always shows the configuration, pretty-printed; the edit form opens it pretty-printed and saves it compact. This reverses the configuration half of "web forms never display a stored Secret or Configuration".
  - Docs to change: [configuring a template](../../docs/03-records-agent.md#configuring-a-template), [private values](../../docs/constitution.md#-private-values), [web § private configuration](../../src/web/web-face/records.md#private-configuration), the runner's script environment, the API reference, MCP and CLI help; the decision row in [decisions](../../docs/decisions.md#settled), with the old one moved to superseded history
  - Acceptance:
    - the Owner and a Maintainer read the configuration; a caller who sees the record but is neither is still refused it and its write;
    - a script started by the runner and by `agent-bus run` sees the stored compact JSON in its environment, and a script with no configuration has no such variable; a kept process sees a changed configuration after `reload`, not before;
    - the web record page shows the configuration pretty-printed to its Owner and Maintainers, and not to anyone else; saving a pretty-printed edit stores compact JSON with the same `config_sha` as the same JSON set from the CLI; the Secret is still never displayed
  - Mutants: the web page hiding it from a Maintainer; the web page showing it to a non-manager; the variable not set; the variable set to `{}` when unconfigured; the web saving the pretty-printed bytes (different `config_sha`); the Secret displayed with the configuration

> **Serg Parf:** Review prompts AI's send to each other, we can make them MUCH shorter; 
> 1st - ask to read proto, then just send minimal data w/o needed

Plan for discussion of the note above: [shorter prompts](short-prompts.md#proposal), choices №Q182–№Q184.

> **Serg Parf:** Define "official" method description syntax for agents; AI prompts to generate one; try to use official standard

> **Serg Parf:** root level runner?

- 💡 RT19 A root runner: one system runner that runs each agent under the Linux account it names, or as root with optional privilege drop
  - Owner, 2026-10-19: "root runner; allow to run services under specific user; or under root (optional drop permissions)"; extends the note above
  - Idea, not designed: today each person runs their own runner as `<person>-ab-runner`, and agents that need host access (systemd, logwatch, git) are standalone units on the shared starter. A root runner could keep all of them in one place, per agent `user: <account>` or `user: root` with `drop` of capabilities (e.g. keep only CAP_DAC_READ_SEARCH for logwatch)
  - Settled, owner 2026-10-19: an agent's account or root is set by whoever has access to the host ("тот кто имеет доступ к host"): it lives in a root-owned host file the runner reads, never in a bus record or call, so no bus caller, Owner included, can raise an agent's privilege ([process boundary](../../docs/08-processes.md#the-rule))
  - To decide before any design: whether it replaces the standalone starter or sits beside it; the sandbox under each account; how a person's agents stay isolated from another's

> **Serg Parf:** git agent; ~ autopull / pull on demand,; status, more - come with ideas

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
  - Built in 0.8.210: reading and writing the host's files, instance create and delete ([the host's files](../../docs/runner/operations.md#the-hosts-files-over-the-bus)); the override of `agent.json`, read by the runner ([overriding](../../docs/runner/configuration.md#overriding-the-authors-options))
  - Builds: overrides of an agent's other shipped JSON files (JSON Merge Patch, mounted); `depends` ordering; the `backup` verb; a configuration change applied at `reload` ([№Q19](runner.md#stop-and-reload))
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
  - Acceptance: each verb leaves the host in its stated state and a second run changes nothing; uninstall leaves no account, unit or link behind and always keeps shared `agent.d`, including with `--purge`; disable stops and disables the unit and keeps the files; a verb for a person who is not a bus User is refused
  - Mutant: Not recorded in the source.

- ⏳ M.12 Agent code from a list of directories (owner, 2026-10-17)
  - Builds: the runner reads agent code from an ordered list of directories: first the shared `agent.d` (`/var/lib/agent-bus/agent.d`), then the release's bundled agents (`/usr/local/lib/agent-bus/current/agents`, or the checkout's `src/agents` on a node running from git), so every bundled agent is always there and a new one arrives with an upgrade; each bundled agent is installed in its packaged form (the Slack agent through its `package.sh`) by the release build
  - Rules: a name in an earlier directory wins over the same name in a later one, so the host's shared copy overrides a bundled one; `status` says which directory an agent came from; a bundled agent is opt-in, listed in `status` as available and served only once `agents.json` enables it, while one in the shared `agent.d` stays enabled by default ([№Q175](DECISIONS.md#recorded-decisions))
  - Acceptance: a bundled agent appears in `status` as available, not served, with no copy in `agent.d`, and is served once `agents.json` enables it; the host's shared copy of the same name is the one served; an upgrade that adds a bundled agent shows it at the next `reload`; removing the shared copy falls back to the bundled one
  - Mutant: read only the first directory, let a later directory win a name, or serve a bundled agent nobody enabled, and a check must fail

- ✅ M.13 An agent says it is not configured (owner, 2026-10-18: "i agree - accept your idea")
  - Builds: an agent of `agent-bus-runner` that exits with code **78** (`EX_CONFIG`, sysexits.h) says it is not configured; the last line it wrote to stderr, if any, is the reason. The runner does not restart it and does not read its inbox any more: `status` shows `state: "unconfigured"` and the reason, and messages wait in its queue as for any agent not being served. A script agent's triggering message is answered `error: <agent> is not configured: <reason>`. Only `reload` tries it again: no timer, no retry. The bundled Slack agent exits 78 when its tokens are missing
  - Scope: `agent-bus-runner` only; the dedicated runner (`agent-bus run`) keeps its behaviour for now
  - Docs: the code and the state in [runner operations](../../docs/runner/operations.md) and [configuration](../../docs/runner/configuration.md)
  - Acceptance: a kept agent exiting 78 is not restarted and shows `unconfigured` with its reason; a script agent exiting 78 answers its caller with the error, and the next message waits unread; `reload` after a fix serves it again; any other exit code keeps today's restart and failure behaviour
  - Mutant: restart on 78, keep reading the inbox, drop the reason, or answer nothing, and a check must fail
  - Built: de0a205 (0.8.193) by Codex; OpenCode reviewed it with no P1/P2, the stderr-capture hardening followed in b514fc7 and was independently reviewed

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

- ✅ B.1 telegram — [plan and tasks](telegram/README.md#telegram-agent), TG.1–TG.7: built as [the Telegram agent](../../src/agents/telegram/README.md#the-telegram-agent) (src/agents/telegram), the tests in smoke's agent_telegram shard
  - Builds: `#telegram/<account>@<realm>`: calls out, and a subscription forwarding a chat to an agent, 📮 queue or 📣 pubsub; the bot token read from the record's bus secret and nowhere else; `#group` and `@user` resolved to a chat id; a default channel declared in the instance's configuration ([telegram](agents.md#telegram))
  - Format: settled 2026-10-18 as the Slack agent's methods where Telegram allows ([methods](telegram/README.md#methods)); the configuration's fields are proposed in TG.1
  - Acceptance: a post reaches its chat and answers what identifies it; a call with no destination goes to the instance's declared default, and the same call against an instance that declares none is refused; an unknown alias is refused rather than resolved to anything; an upstream refusal comes back as an answer; a subscription's match reaches its target and a non-match does not. Mutants: the token read from the process env or from `telegram.toml` at run time; the declared default replaced by the script's first-configured-chat fallback, which must fail the no-default refusal; the alias miss falling back instead of refusing
  - Mutant: the token read from the process env or from `telegram.toml` at run time; the declared default replaced by the script's first-configured-chat fallback, which must fail the no-default refusal; the alias miss falling back instead of refusing

- ✅ B.2 systemd — built: [agent and verification](../../src/agents/standalone/systemd/README.md#verification)
  - Builds: the [standalone systemd agent](agents.md#systemd) on the shared starter, including `logs`, `failed`, `timers` and `list [type]` for every host unit. Owner, 2026-10-19: "add journalctl methods: check last logs (all or specific service), return N lines; support date filtering (from 1h ago)".
  - Format: [B.2 calls and delivered tiers](../../src/agents/standalone/systemd/README.md#calls); no installed unit-set file; `agent.json` supplies the `systemd` discovery tag.
  - Acceptance: no-role list/timers/failed/status, operate start/restart/reload/logs, admin stop/enable/disable; an unconfigured service or timer is listed, statused and started by operate; every tier refuses invalid names; install twice leaves one unit and one name, removal leaves no unit, token or record. Running as a non-root account leaves list/status working and every acting verb returns systemctl's refusal as an error, never silent success.
  - Mutants: remove role enforcement; trust a role claimed in the body; suppress errors from the non-root unit's acting verbs. NoNewPrivileges is not a privilege-loss mutant: it does not drop existing root credentials.

- ✅ B.9 systemd dedicated instances
  - Builds: a `units` entry in the instance's Configuration, a comma-separated list of exact unit names, which limits that instance to those units ([dedicated instances](../../src/agents/standalone/systemd/README.md#dedicated-instances)). Owner, 2026-10-19: "support "config" entry; with comma separated list of units (timers, ... whatever systemd manages); this way we can create dedicated instances of specific service status/restarters for ordinary users"
  - Depends: №RT18, the Configuration in the script's environment
  - Acceptance: with `units` set, list/timers/failed show only the listed units; status, an acting verb and `logs <unit>` on an unlisted unit are refused by name with systemctl never run; `logs` with no unit reads only the listed units; without `units` every unit works as today; a malformed entry answers every call with a named Configuration units error without executing; two instances on one host with different lists each see only their own; tiers unchanged
  - Mutants: the list ignored for an acting verb (an unlisted restart reaches systemctl); `logs` with no unit reading the whole journal while a list is set; a malformed entry treated as "all units"

- ✅ B.3 logwatch — [guide](../../docs/agents/logwatch.md#what-it-does), [verification](../../src/agents/standalone/logwatch/README.md#verification)
  - Builds: `#logwatch@<host>`, standalone on the shared starter (a long-lived process: the ring lives in memory), ported from `/rd/service/tail-ring-buffer` (owner, 2026-10-19: "implement logs agent; check my tail-log … extract ideas"). One instance per host; a dedicated instance `#logwatch/<name>@<host>` with its own source list, like systemd's.
  - Methods: `tail [N]` (default 20, verbatim, oldest first, each line with its source); `grep <text|/re/flags> [N]`; `since <age> [N]`; `mark [label]`, then `after <label>`, for "mark, act, see what appeared"; `sources` (effective files, excluded ones, line count and last line per source); `stats` (version, uptime, ring size and fill, lines seen, dropped); `sub add|list|delete` forwarding new matching lines to an agent, 📮 queue or 📣 pubsub, repeats folded ("×37 in 5 min"); `help`.
  - Configuration: source globs (tail-all defaults), platform syslog, exclude globs, optional journald units, ring size (default 2000), rescan interval (default 600 s). Lines are cleaned exactly as tail-all and the server do (ANSI, `#033`, SGR fragments, control runs → `...`); zero-byte files are skipped; a new file starts at its end.
  - Access, settled by the owner on 2026-10-19: every method but subscriptions is read-only, so **no roles**; the ACL alone decides who may call (default `@owner`). A subscription is changed by whoever manages its destination, as the Slack agent's are.
  - Acceptance: the last N come back unfiltered by default and oldest first; a filter applies only when asked; `after <mark>` returns exactly the lines appended after the mark; a file created after start is picked up by the next rescan; an excluded glob never appears; cleaned output contains no escape bytes; writing far beyond the ring keeps it at its bound; a subscribed line reaches its target and a non-match does not; a caller outside the ACL is refused
  - Mutants: the ring's bound removed, which must fail the bounded check rather than hang it; a default filter applied when none was asked for; a new file never rescanned; the cleaning removed; `after` returning lines from before the mark

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

- ✅ B.7 server-info — built: [agent and verification](../../src/agents/standalone/server-info/README.md#verification), shared standalone starter for №B.2
  - Builds: `#server-info@<host>`, **standalone like systemd** (owner, 2026-10-19: "we need separate starter same as for systemd one"). Its own install script writes a unit running `agent-bus run` with the sandbox off, reaching the bus over IP with a token from the unit's `EnvironmentFile`. It answers the [catalogue's sections](../R1.2/services.md#server-info): `uname -a`, `ip a`, `ip r`, `free`, `lsblk` with models and serials plus `df`, `uptime`, load average, and the accepted additions: OS release, CPU, inodes, pressure, failed units, time sync, DNS, listening ports, users, virtualization, and root-only disk health and sensors
  - Depends: none in the runner; the systemd agent's install script (№B.2) is the pattern, shared rather than copied
  - Format: [section names, text and JSON calls](../../src/agents/standalone/server-info/README.md#calls)
  - Acceptance: each section answers the host's own facts (not a sandbox's: `ip a` shows more than `lo`); a call carrying arguments is refused, so nothing a caller sends reaches a shell; the install script run twice leaves one unit and one name, and its removal leaves no unit, no token file and no orphaned record; root-only sections are absent in an unprivileged instance, not failing
  - Mutant: run inside the sandbox, which must fail the host-facts check; a caller's argument passed to a command, which must fail the no-arguments refusal

- ✅ B.8 git — remote git sync
  - Built: [standalone git agent](../../src/agents/standalone/git/README.md#git-agent), configured repositories and delivered tiers (current roots/caps in №B.10 below).
  - Depends: №RT18, for the configured key
  - Builds: `#git@<host>`, standalone on the shared starter like systemd. Owner, 2026-10-19: "allow remote git sync methods calls; main set — git pull; git reset --hard; git clean -fd (typical deploy); pull revision; pull file specific revision"; earlier: "autopull / pull on demand; status, more".
  - Methods, owner's set: `pull`; `reset` (`reset --hard` to the upstream); `clean` (`clean -fd`); `deploy`, the three in that order; `pull <rev>`, fetch then a detached checkout of that revision, a pinned deploy or rollback, which `deploy` leaves for the branch head again; `pull <rev> <file>`, one file at a revision.
  - Methods, also accepted: `status`, `log [N]`, `rev`, the HEAD commit and branch; `fetch` with an ahead/behind count; `diff`, a stat of local changes, which is what `reset` and `clean` would discard.
  - Historical B.8 design (roots and tiers replaced by №B.10 below), owner 2026-10-19: it acts only on repositories declared at install, by name and path (`--repo ab=/usr/local/src/ai-agent-bus`), and a call names one; listing them needs no role. Tiers as systemd's: no role for `status`, `log`, `rev`, `fetch`, `diff`; `operate` for `pull`, `pull <rev>`, `pull <rev> <file>` and `deploy`; `admin` for `reset` and `clean` on their own. Only delivered roles count. Autopull is a later row, not B.8.
  - Second set, owner 2026-10-19 ("second set - add/commits/push (reqs a role)"): `add <path…>`, `commit <message>` and `push`, each now needing `commit` under №B.10; who commits and pushes is the server's own: the git identity configured there and the ssh key the push uses, never the bus caller, which git cannot carry (owner, 2026-10-19: "author is what configured on this server"; "not a caller … author defined by ssh key on this server"). The agent sets no author. Which key it uses is the agent's configuration (owner, 2026-10-19: "use agent config - there it can specify key to use (when not set - users keys)"): a key file path read from `AGENT_BUS_CONFIG` ([№RT18](#objective)) and used for git's ssh with only that identity; with none set, the running account's own keys. The configuration holds the path, never the key. The account must be able to read that file, so it is installed with `--user` naming an account that holds the key or the git configuration, as the starter's default `ab-git` has neither. A path outside the repository and a message carrying an option are refused.
  - Format: the text and JSON calls, proposed in the commit and README as systemd's were
  - Acceptance: against a fixture repository and its fixture upstream, `deploy` leaves the work tree equal to the upstream head, a local edit and an untracked file gone; `pull <rev> <file>` changes only that file; a revision or path that is not one is refused, naming it, with the tree unchanged; a caller's argument never reaches a shell or git as an option (a `--upload-pack=` revision is refused); a refused tier changes nothing; an undeclared repository name or path is refused; `pull <rev>` leaves HEAD detached at that revision and a later `deploy` returns to the branch head; `add` then `commit` by `commit` makes one commit with the server's configured identity and `push` reaches the fixture upstream, while the same calls with no role change nothing; with a key configured, the push offers only that key (a fixture ssh upstream accepts it and refuses the account's default key), and with none it uses the account's
  - Mutants: the option-injection refusal removed; `clean` run without the tier check; a role claimed in the body trusted; a path outside the declared set accepted; an identity taken from the call; the configured key ignored, so the push uses the account's default key; `pull <rev> <file>` touching another file

- ✅ B.8a Git autopull (0.9.19)
  - [Install-time timers and status](../../src/agents/standalone/git/README.md#autopull): an optional `--autopull ab=5m` for each configured repository with deploy or commit access, ff-only under the same lock; dirty or diverged trees are never forced, errors are logged and returned by `autopull ab`. No timer by default and no service restart. Owner revised the initial Configuration proposal to install-time systemd timers on 2026-10-19.
  - Acceptance: a real private timer brings a fixture upstream commit within a bounded interval; a dirty tree remains intact and its failure is returned; removal removes the timer. Mutants: timer not enabled; dirty tree reset/cleaned.

- ✅ B.10 Git roots and access levels in Configuration
  - Builds: [repositories and access levels](../../src/agents/standalone/git/README.md#repositories-and-access-levels): `repos` with path and `access` (`read`, `deploy`, `commit`, cumulative) in the Configuration, replacing `--repo`; roles `deploy` and `commit` replace `operate` and `admin`; the effective level is the lower of the repository cap and the caller's role. Owner, 2026-10-19: "in config - list git roots and allowed access level … even with best role user can not go over repo cap"
  - Acceptance: a `commit` caller on a `deploy` repository is refused push and reset-free pull still works; a `deploy` caller on a `commit` repository is refused commit; a `read` repository refuses pull from any role; a path not in `repos` is refused; a changed Configuration applies to the next call; autopull runs only on a repository whose cap is at least `deploy`; a malformed `repos` answers an error naming it. Live: `ab` capped at `read`
  - Mutants: the cap ignored (role alone decides); the role ignored (cap alone decides); a repository outside `repos` accepted

- 💡 B.11 AI proxy: `#ai@<host>` makes model calls for bus callers, so the provider key stays secret
  - Owner, 2026-10-19: "agent — AI/OpenRouter proxy — proxy AI/openrouter calls; main idea — keep KEY secret; support Claude, OpenAI, Z.AI, OpenRouter (as #1)"
  - Idea, not designed: callers send a prompt (or a chat request) on the bus and get the answer; the API keys live only in the runner's host files ([two places for secrets](../../docs/runner/security.md#secrets)), so no caller, script or session ever holds one. Providers in order: OpenRouter first, then Anthropic (Claude), OpenAI, Z.AI
  - To decide before design: the call shape (plain text prompt vs a provider-neutral chat JSON); model naming (`openrouter:<model>` or an alias map in Configuration); streaming or whole answers; who may call and per-caller limits (requests, tokens or cost per day) with a `usage` report; whether answers are logged; how a provider error or rate limit is answered

- 💡 B.12 Session starter: an agent that starts a named set of bus-connected AI sessions (Codex CLI, Claude Code, OpenCode) in tmux, each with our integrations built in, as `ab-claude`, `ab-codex` and `ab-opencode` do by hand today
  - Owner, 2026-10-19: "agent — start set of named AB connected sessions — codex-cli, claude-cli, opencode sessions in tmux; with our integrations built-in, as we do now with ab-claude, ..."
  - Idea, not designed: a call such as `start team-dev` brings up the sessions a Configuration names (e.g. `#claude/main`, `#codex/ab-dvp`, `#codex/ab-dvp-3`), each in its own tmux window, through the existing [smart launchers](../../docs/06-launchers.md#smart-launchers), so every session is registered under its fixed name with MCP and push wired; `list`, `stop <name>`, `restart <name>` and `attach` help (the tmux command a person runs)
  - Settled, owner 2026-10-19:
    - it runs as the person (their account), since the sessions act as them;
    - the set lives in the agent's Configuration: name, runtime, working directory, launcher arguments;
    - built: [launcher crash restart](../../docs/06-launchers.md#restart-after-a-crash), `--restart[=<timeout>]`; only a failed foreground TUI restarts, not a clean exit or a background server. The starter passes the flag through;
    - a name already live fails, as everywhere ([fixed names](../../docs/06-launchers.md#session-names)): the starter reports it and starts the rest.

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

## Web scale follow-up from MVP

- ⏳ F.13.6 follow-up — growing pages and directory reads
  - Origin: the released MVP's [measured findings](../done/R0.8-MVP/done/web-acceptance.md#findings), carried forward when R0.8 was archived; the original acceptance is complete.
  - Work: decide pagination for Groups, Account, the Owner's User page and Activity, and bound directory bytes fetched for a listing. These were deferred page-design choices, not failed MVP acceptance; remeasure the TypeScript face before choosing changes.
  - Acceptance: record HTML and bus response sizes as directory size grows; any chosen bound gets a check that fails when the bound is removed. No pagination or API design is settled by this carry-forward.
