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

- ✅ RT9 [Agent method metadata](method-metadata.md#method-metadata)
  - Format: settled with the owner on 2026-10-19 ([settled so far](method-metadata.md#settled-so-far)): typed fields in `agent.json`, commented `env.dist`, and the `help`, `help-config`, `help-secret` and `help-json` asks; MCP generation is [R1.2](../R1.2/TODO.md#dependencies)
  - ✅ RT9.1 The runner and `agent-bus run` read the new `agent.json` fields strictly and answer `help`, `help-config`, `help-secret` and `help-json` themselves for packaged agents, without running the script; the CLI and MCP face send the three new asks (`agent-bus call <to> --help-config`, `--help-secret`, `--help-json`). Acceptance: a packaged fixture with no help code answers all four from its files; `help-secret` names keys and comments and never a value (sentinel); an unknown field is refused by name; an enum value outside the list is shown. Mutants: the script run for a help ask; a secret value in `help-secret`
  - ✅ RT9.3 The web face shows it all (owner, 2026-10-19: "once done, the web must show all this data"): an agent's record page gets an "About this agent" card (description, version, license, author, homepage), a Methods table (name, arguments with types and enums, returns, role, destructive), and the Configuration editor shows each documented key's type, help and example beside the JSON, warning about undocumented keys. The Secret field lists the expected keys from `env.dist` with their comments and which are required, never a value. The web gets it through the same asks, or from the runner for its packaged agents. Acceptance: a fixture agent's page shows every field; a secret sentinel never appears; an agent answering only `--help` shows that text and no tables. Mutants: the secret value rendered; an enum dropped
  - ✅ RT9.2 Every bundled agent's `agent.json` and `env.dist` carry the metadata (methods, args, Configuration keys, description, license, author, homepage) and their hand-written help code is removed where RT9.1 covers it; the guides link `--help-config`, `--help-secret` and `--help-json`. Acceptance: each agent's `--help` matches its real methods (a parser check per agent); removing a method from the code without its entry fails the check
  - ✅ RT9.4 [Usable Methods cards](../../src/web/web-face/records.md#method-rows), built in 0.9.44: native disclosure rows, signatures, typed args/enums, CLI/JSON examples from declared `body_formats`, GET filter, meaningful fields and compact settings. RT9.4b reports only role-hidden counts/needed roles. Phone-width/no-JS browser check passed; help-listed and hidden-count mutants compiled and failed assertions under `--slow`; restored full gate 1164/0. A call-execution form remains later.

- ⚠️ RT10 [Runner](runner.md#runner)
  - Details: decided; [waves](#runner-waves) proposed
  - State: one-message serving built 0.8.123; full evidence remains in [runner waves](#runner-waves)
  - Depends: [method metadata](method-metadata.md#method-metadata); the pool hostname field, now R1 ([where a member says it is](runner-implementation.md#where-a-member-says-it-is))

- ⚠️ RT11 [Runners section in the web face](runner/web-face.md#goal) — status, per-agent controls, host files and instances are built; remaining acceptance is tracked in the checklist
  - State: the information page and per-agent management page are separate; denied management shows Access denied
  - Home: [Minimum and acceptance checklist](runner/web-face.md#minimum-checklist)

- ✅ RT12 [Agents for the runner](agents.md#what-moved-and-what-r1-adds): telegram, systemd, logwatch
  - Depends: Telegram and logwatch: the runner keeping an agent it did not write (№M.1 and №M.2, built; №M.11 for the install) and the kept process form for state between messages (№M.3, №M.9). Systemd needs none of them — it is a root unit of its own running `agent-bus run`
  - Settled by: owner, 2026-10-17
  - Home: [tasks](#bundled-agents)

- ⏳ RT13 [Backups](runner-implementation.md#backing-it-up)
  - Details: decided: the runner's `backup` verb

- ✅ RT29 Record export and import (0.9.108)
  - Built: Owner-only encrypted permanent records, private KV and credentials, explicit selection/conflict/realm choices, atomic import, public matching backup-info, and independent bounded backup storage; [current contract](../../docs/07-setup.md#encrypted-record-backups).
  - Accepted on disposable daemons: real age/CLI/SQLite rebuild, old runner and standalone token clients after restart, wrong key and sidecar refusal, whole-transaction rollback, Owner doors and explicit conflicts; sixteen owning mutants and normal1187/0. No live export/import run.

- ⏳ RT14 [Installable distribution](distribution.md#release-artifacts): P.1–P.3
  - Built part: bundled agent runtime packages, Python client and shared helpers ship in build/package output and its verified manifest; fresh-install acceptance exercises the installed tree outside the checkout (0.9.35).
  - Format: publication names and platforms
  - Editions: [full and client-only](distribution.md#two-editions), the latter the runner and the basic CLI (owner, 2026-10-19)
  - Platforms (owner, 2026-10-19: "setup PC/Mac builds"): builds for macOS (Apple Silicon and Intel) besides Linux, at least the client-only edition; no Windows (owner, 2026-10-19: "no windows, mac/linux"). To decide for macOS: which parts run there (CLI, runner, launchers; the daemon and sandbox are Linux-only today), the sandbox and process-boundary equivalent, the install method (Homebrew or an archive) and code signing

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

Plan for discussion of the note above: [shorter prompts](short-prompts.md#proposal), open choices №Q183–№Q184; №Q182 is settled in the [receipt rules](../../docs/04-messaging.md#receipts).

> **Serg Parf:** Define "official" method description syntax for agents; AI prompts to generate one; try to use official standard

> **Serg Parf:** root level runner?

- ✅ RT20 [A may-send dry run](../../docs/09-daemon-api.md#checking-permission-to-send)
  - State: built in 0.9.41; CLI `agent-bus may-send <name>`, shared target-admission function with Send; no queue, refusal counter or activity changes. The caller asks only for itself; Send keeps its ACL `403`, including invisible targets.
  - Verification: API parity for active/allowed, ACL/Personal refused, invisible, inactive, absent, external and User-inbox cases; CLI success/refusal; no on-behalf input. Enqueue and divergent-answer mutants compiled and failed assertions under `--slow go_checks`; restored full gate 1160/0.

- ⏳ RT21 A client calling itself takes its own request as the answer
  - Found by Codex #3 in B.13, 2026-10-19: the Python client's `Bus.call` to its own name consumes the request it just sent (the same inbox, topic and tag) as the correlated answer, before the handler runs. Check every client (RT8 conformance), not only Python
  - Acceptance: an agent calling itself gets its handler's answer, never its own request echoed back; a conformance case per client. Mutant: correlation that ignores the reply marker

- 💡 RT19 A root runner: one system runner that runs each agent under the Linux account it names, or as root with optional privilege drop
  - Owner, 2026-10-19: "root runner; allow to run services under specific user; or under root (optional drop permissions)"; extends the note above
  - Idea, not designed: today each person runs their own runner as `<person>-ab-runner`, and agents that need host access (systemd, logwatch, git) are standalone units on the shared starter. A root runner could keep all of them in one place, per agent `user: <account>` or `user: root` with `drop` of capabilities (e.g. keep only CAP_DAC_READ_SEARCH for logwatch)
  - Settled, owner 2026-10-19: an agent's account or root is set by whoever has access to the host ("тот кто имеет доступ к host"): it lives in a root-owned host file the runner reads, never in a bus record or call, so no bus caller, Owner included, can raise an agent's privilege ([process boundary](../../docs/08-processes.md#the-rule))
  - To decide before any design: whether it replaces the standalone starter or sits beside it; the sandbox under each account; how a person's agents stay isolated from another's

> **Serg Parf:** git agent; ~ autopull / pull on demand,; status, more - come with ideas

## Next step

Assigned roles are built ([verification](done/assigned-roles.md#verification)).
Method metadata, bundled agents and distribution packaging are built on 0.9;
continue the remaining runner waves, distribution acceptance and the accepted
[protocol conformance task](#objective). Federation is
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
version bump, normal gate (`src/smoke.sh --normal`) and peer review. A wave that needs a format first
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

- ✅ B.4 Choose the next set — the owner chose mail, then webhook, cron and fetch, 2026-10-27 ([RD125](DECISIONS.md#recorded-decisions)); rows B.5, B.6, B.16–B.18

- ✅ B.5 IMAP client — [guide](../../docs/agents/imap.md#what-it-does), [verification and disabled deployment](done/mail.md#b5-imap)
  - Plain agent and three owner-requested instances installed disabled, sandboxed and owner-only; publication target unset. Manager-controlled live enable/small sync remains separate from fixture proof.

- ✅ B.6 SMTP client — [guide](../../docs/agents/smtp.md#what-it-does), [app-password and XOAUTH2 evidence](done/mail.md#b6-xoauth2)
  - Installed disabled, without an account. The owner-approved real Gmail read/send check remains pending; fixture proof does not claim real-provider verification.

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

- ✅ B.11 AI proxy
  - Built as the [AI agent](../../docs/agents/ai.md): runner-hosted, sandbox and network on; one text prompt or an OpenAI-compatible chat-completions JSON request, no streaming or history. Configuration is only `model` and optional `base_url`; the provider key lives only in the runner host env file. Owner-approved minimal design, 2026-10-20.
  - Verification: fake provider covers pass-through/default model/text/errors, deadline and size bounds, runner required-key report and direct missing-key exit 78, redirect refusal and key-free answers/logs (including provider echoes); package runs outside the checkout. The secrecy check kills the no-redaction mutant. Owner changed the live ACL to `*`; every request logs its caller, model, byte sizes and status, never prompt or key. No per-caller limits beyond the ACL.

- ✅ Telnyx SMS V2 provider agent and Legacy skill migration
  - [SMS guide](../../docs/agents/sms.md): stdlib runner agent, Telnyx host only, public sender/aliases in Configuration, private host key; owner-approved `*` ACL and caller audit.
  - [Verification](done/sms-telnyx.md): fake-only tests, required-key and sandbox regressions, recipient privacy guard; exactly two owner-approved real sends accepted, direct and final thin `/rd` wrapper. No retries; examples use only reserved fictional recipients.

- ✅ B.12 Session starter (0.9.74)
  - Built: [sessions](../../docs/agents/tmux-sessions.md#what-it-does), `start` only; a configured set runs as the person in detached tmux sessions, using fixed bus names, launcher restart and set/session tags. Existing tmux sessions or live names fail that entry; others continue. Children inherit no starter bus credentials or Configuration.
  - Follow-ups: list/status, stop/restart, and attach guidance as methods; no such controls in this version. Known real-use blocker: detached ab-claude waits at Claude Code’s “Loading development channels” confirmation for server:agent-bus; owner decision pending, no workaround in this starter.

- ✅ B.13 debug as the bus doctor (0.9.36) (owner, 2026-10-19: "make debug more interesting … yes, let's do this")
  - Builds, on `#debug@<host>`: `echo` (the envelope as the agent sees it), `roles` (the caller's roles and where they come from, in words), `ping` (receive and answer times), `sleep <s>` (bounded by the deadline), `fail [code]`, `crash` (SIGKILL), `sandbox` (limits, network, writable paths, as seen inside); then `selftest` (ping, echo, a slow answer, an error, KV and a lock, each ✅ or ❌), `doctor` (component versions agree, the TLS pin and certificate expiry, failed agents, a Telegram 409)
  - Identity: debug serves itself with its own agent credential. Doctor calls other agents as debug; refused access is never bypassed. There is no debug `can`: callers use RT20 `agent-bus may-send <name>` for their own permission, without impersonation.
  - Bounds: every wait within the deadline, every size and count capped; ACL `@owner`; `env` stays masked (RD105). Only existing daemon calls are used, so a check that would need a new daemon call or host access debug's sandbox lacks is reported as unavailable with the reason, and proposed separately
  - Used by our tests (owner, 2026-10-19: "make it useful and start using it in our tests"): smoke checks that today write their own throwaway fixture script use debug's methods instead where one fits (`sleep` for the delayed ack and deadlines, `fail` for error answers and stderr redaction, `crash` for runner restart, `echo` and `roles` for delivered roles, `unicode` for escaping); and `selftest` runs against the live node after every deploy, its ❌ failing the deploy check
  - TLS follow-up: server-info exposes public certificate facts and the token-helper pin; doctor checks expiry and pin agreement over the bus, without host access.
  - Built extras: `big` and `unicode`; `count`, `lock`, `burst`, `who`, `trace`, `explain`, `chaos` and `replay` remain follow-ups.
  - Later set, after the first lands (owner: "add more ideas too"): `big <KB>` (a capped answer of that size, for message limits); `count` (a KV counter that survives restarts); `lock <name> <s>` (holds a bus lock); `burst <N>` (sends the caller N messages, at most 50, for inbox, push and ordering); `unicode` (emoji, Cyrillic, `\n` and control bytes, for escaping in the CLI, web and Telegram); `who` (who is online now and last active); `trace <name>` (where a message to that name would go: queue, subscribers, readers, forwarding); `explain <error text>` (a bus error in plain words, with what to do and a docs link); `chaos <ms>` (the next few calls answer with a random bounded delay or error); `replay <id>` (resends one of its own stored messages)
  - Acceptance: each method answers its documented fields; `selftest` reports ❌ for a broken check (fixture) and ✅ otherwise; nothing exceeds its cap. Mutants: a cap removed; `selftest` reporting ✅ for a failed step

- ✅ B.14 Unattended Claude channel plugin (0.9.82): [setup and migration](../../docs/06-launchers.md#claude-channel-plugin). After the Team org allowlist was set, fresh sessions registered the plugin channel without a channel dialog and answered real push calls; startup refreshed the cached org policy. B.12 still requires an already-trusted working directory.

- ⏳ RT22 Bus setup: the Configuration of `#bus` on the ordinary edit page, daemon Owner only: the bus realm (default the host name, short when host names are long) and agent-event settings ([plan](daemon-configuration.md#approved-plan), [RD108](DECISIONS.md#recorded-decisions), [RD158](DECISIONS.md#recorded-decisions)). Assigned 2026-10-29: daemon side (keys declared by `#bus`, validation, Owner-only write, apply without restart, backup) AB2; edit page AB; review `#claude/ab2-review`
  - [One-page proposal, pending approval](daemon-configuration.md#approved-plan): realm first, other settings read-only or kept in their owning interface; changing the default never renames existing records
  - Acceptance: only the daemon Owner sees and saves the page; a new record defaults to the configured realm; an existing name never changes silently

- ✅ B.19 voyage — built as RAG R.1, 0.9.123; [result, gate and review](rag/DONE.md#r1-voyage-proxy). The manager/owner makes the first live provider call.

- ✅ B.20 Database proxy, built in 0.9.129 (commit 50f7df42; gate --normal 1208/0; deployed to `agent.d/db` and disabled in the runner inventory until the owner gives a database; [guide](../../docs/agents/db.md#what-it-does); fixtures MariaDB, PostgreSQL, ClickHouse; 14 mutants each failing on an assertion; MySQL itself unproven, MariaDB tested) (owner, 2026-10-28: "add DB proxy with special role (read) - allow selects only")
  - Builds: a database agent per database instance (**MySQL, PostgreSQL and ClickHouse**, all required: owner, 2026-10-28: "support mysql & postgre", "and clickhouse"; MariaDB through the MySQL protocol), the credentials in the host runner env, with the same [application-role](../../docs/03-records-agent.md#roles) gating as `#qdrant` ([RD133](DECISIONS.md#recorded-decisions))
  - Role `read`: SELECT only. Enforced by the database, not by parsing SQL: a read-only transaction (`START TRANSACTION READ ONLY`) in MySQL and PostgreSQL, the `readonly=1` query setting in ClickHouse, and, where configured, a separate read-only database account; a statement that writes fails in the database and the error is the answer
  - Roles ([RD134](DECISIONS.md#recorded-decisions)): `read` SELECT only; `write` also INSERT, UPDATE and DELETE; `admin` DDL and everything else; Owner and Maintainers count as `admin`; a caller the ACL admits with no role gets `read`
  - Docs: the guide leads with the preferred setup — real SQL users with the right grants, one proxy instance per account (`#mysql/billing-ro`, `#mysql/billing-rw`), so the database enforces permissions; roles on one instance are the fallback, **only when you cannot create normal SQL users with correct permissions** (owner, 2026-10-28)
  - Acceptance: against a fixture MySQL, PostgreSQL and ClickHouse, each, a `read` caller's SELECT returns rows; its INSERT, UPDATE, DELETE, DDL, a writing function called from a SELECT, and a multi-statement smuggle all fail and change nothing; credentials appear in no reply or log
  - Mutant: run a `read` call outside the read-only transaction, without ClickHouse's `readonly=1`, or with the write account; a check must fail
  - 💬👾 Plan (AB, 2026-10-28), for review before any code:
    - Shape: one Python agent `src/agents/db/` beside `voyage`, same envelope, `metadata` dispatch, deadline and size caps, `env.dist`, `package.sh`; one instance serves one database, its engine and address in Configuration, its account and password in the runner env only
    - Role: the agent reads the `roles` the daemon put on the envelope. `owner`, `maintainer` and `admin` → admin; `write` → write; `read` or no role → read; the highest held wins; a role the agent does not know grants nothing beyond `read`
    - Enforcement of `read` (never by reading the SQL): MySQL/MariaDB `START TRANSACTION READ ONLY`, always rolled back; PostgreSQL a read-only transaction plus `default_transaction_read_only` on the session; ClickHouse `readonly=1` sent with every request. `write` runs in a normal transaction that commits; `admin` runs as the account is allowed
    - One statement per call, enforced by the protocol: MySQL multi-statements stay off; PostgreSQL needs the extended query protocol (psycopg 2 sends a whole string in the simple protocol, so `SELECT 1; COMMIT; DROP …` would leave the read-only transaction — hence **psycopg 3**, not installed here; pulling it is a download, so the manager or owner says yes first); ClickHouse takes one statement over HTTP
    - Drivers: `pymysql` (installed) and `psycopg` 3 as established libraries ([MODULES § external tools](../../src/MODULES.md#external-tools)); ClickHouse over the standard-library HTTP client. A missing driver is an error naming it, never a fallback
    - Secrets: password from the env only; every error and the stderr line pass through the same redaction as `voyage`; the stderr line carries role, engine, byte counts and status, never SQL text or values
    - Bounds: row and byte cap on an answer (rows past it are cut and the answer says so), the call deadline, one connection per call
    - Fixtures: MariaDB from the installed `mariadbd` in a private `tmp/` directory with a unix socket; PostgreSQL and ClickHouse in podman, digest-pinned, bound to 127.0.0.1 on a free port, removed on exit. Images are not on this node: pulling them (about 0.4 GB) needs the manager's yes
    - Checks (each of the three engines): `read` SELECT returns rows; INSERT, UPDATE, DELETE, DDL, a writing function inside a SELECT and a multi-statement smuggle each fail with the database's own error and the table is unchanged afterwards; `write` changes rows but its DDL fails; `admin` runs DDL; no role equals `read`; owner and maintainer equal `admin`; the password is in no answer and no stderr line
    - Mutants (each runs only its own check): `read` outside the read-only transaction; ClickHouse without `readonly=1`; PostgreSQL on the simple protocol; the write account for a `read` call; no role mapped to `write`; owner not mapped to `admin`; redaction removed
    - Docs: `docs/agents/db.md` leads with one proxy instance per real SQL account, roles only as the fallback; the agent list and the smoke map gain the new path; version bump with a changelog line; deploy only this agent, then `check-deploy`
    - 💬👾 After review by #claude/ab2-review (these supersede the lines above where they differ):
      - Accounts (H1): the write/admin boundary has a mechanism only as accounts. The runner env may hold one account (`DB_USER`, `DB_PASSWORD`) or per-tier accounts (`DB_READ_*`, `DB_WRITE_*`, `DB_ADMIN_*`). With per-tier accounts a tier whose account is missing is refused, never served by a stronger one. With one account only `read` is database-enforced; `write` and `admin` run with that account's authority and the guide says so plainly
      - PostgreSQL (H2, H3): force the extended protocol and pin it by a check that the multi-statement string fails with the server's "multiple commands" error; one statement per call is load-bearing, stated in the code and docs. The read-only transaction is not a boundary against a privileged account (`dblink_exec`, `COPY … TO`, file functions): the guide says a `read` caller needs a read account without them; a superuser fixture row shows the limit
      - MySQL (M1, M2): session `transaction_read_only=ON` as well as the transaction; multi-statements off, pinned; checks for DDL, `CREATE TEMPORARY TABLE`, `LOCK TABLES`, `SET autocommit`, `COMMIT`. The fixture is MariaDB; a MySQL image is added if the owner wants MySQL itself proven, else the docs say MySQL is unproven
      - ClickHouse (M3): `readonly=1` cannot be influenced by the caller; checks for `SETTINGS readonly=0`, `SET readonly=0` and a second statement; credentials in headers, never the URL; the guide recommends a read profile with `readonly=1` as a constraint
      - Limits (M4, L2): server-side `statement_timeout`, `max_execution_time`; rows streamed to the cap, a cell and column cap, the answer says when it cut; deterministic text for bytes, Decimal, datetime
      - Secrets (M5): keyword arguments never a DSN; every driver exception wrapped; a sentinel password checked in a wrong-password error, a refused connection, a timeout and an echoing query
      - Roles (L1): `Admin` in another case is unknown, so `read`; a non-list `roles` reads; the guide notes no role is `read` here, where `#qdrant` gives write
      - Every refusal check has a positive control (the same string succeeds under write, admin or the mutant). Added mutants: MySQL multi-statements on, MySQL without READ ONLY, PostgreSQL without the read-only transaction and without the session default, ClickHouse without `readonly=1` in one request path, a missing tier falling up, no server-side timeout, exception text unredacted, unknown role as `write`, highest-wins inverted
      - psycopg 3.3.6 (LGPL-3.0-only, <https://psycopg.org>) is added to the external-tools row and the distribution notice

- ✅ B.21 IMAP subscriptions — built and deployed in **0.9.150**, `6449f674`; normal **1214/0**, **67** assertion-red mutants; reviewer verified landed source; manager check-deploy **6/6**, no subscriptions. [Current guide](../../docs/agents/imap.md#what-it-does), [evidence and plan history](done/imap-subscriptions.md#result). Live automatic forwarding waits for the owner to add a subscription.

- ✅ B.24 IMAP live read status — **0.9.154**, `cedba648`; normal **1214/0**, **26** assertion-red mutants, reviewer verified landed runtime; three configured instances enabled, manager read-only list checks and check-deploy **6/6**. [Current guide](../../docs/agents/imap.md#what-it-does), [evidence and plan history](done/imap-read-status.md#result). Live Seen writes remain untested.

- ✅ B.16 webhook — built in **0.9.143**, `61ae2338`; normal **1217/0**, **45** assertion-red mutants; deployed disabled, check-deploy **6/6**. [Current guide](../../docs/agents/webhook.md#what-it-does), [completion and plan history](done/webhook.md#result). No public listener or real sender check; owner approval required before exposure.

- ✅ B.17 cron, built in 0.9.139 (commit 3305d54e; gate --normal 1215/0; 48 tests on the fixture daemon; 51 mutants each failing its owning test on an assertion; deployed to `agent.d/cron`, disabled in the runner inventory; check-deploy 6/6; [guide](../../docs/agents/cron.md#what-it-does)) — [defaults](agents.md#next-set); AB2
  - Acceptance: `sub add/list/delete` behave as in Slack and Telegram; delivery is admitted by the target's own ACL for cron (RD140), an ACL refusal pauses the entry; a schedule fires at its minute exactly once; a one-shot survives an agent restart and fires once; a missed recurring slot is not replayed
  - Mutant: fire twice, lose a one-shot on restart, or replay a missed slot; a check must fail
  - 💬👾 Plan (AB2, 2026-10-28), after review by #claude/ab2-review (H1–H2, M1–M6, L1–L4 folded in), before any code:
    - Shape: a Python stdlib agent `src/agents/cron/` like `imap`: `serves` itself, one kept main-thread loop (`consume` bounded by the next due time), shared metadata dispatch; files `cron.py`, `cronexpr.py`, `agent.json`, `package.sh`, tests. No daemon, runner or web change
    - Calls: `sub add <m h dom mon dow> <target> <message>`, `sub list`, `sub delete <id>` with logwatch's JSON forms and words; `later <+90s|+15m|+2h|+3d|RFC 3339 with offset> <target> <message>`, `later list`, `later cancel <id>`; `help`
    - Permission ([RD140](DECISIONS.md#recorded-decisions), replaces `/manages` and may-send): cron sends under its own name and the target's ordinary ACL decides (`*` counts); `sub add` and `later` need admission to cron; `sub delete` and `later cancel` need the entry's creator or cron's Owner/Maintainers. An ACL refusal at firing pauses the entry visibly. Each firing names the entry's creator in the message. The target is never cron itself.
    - Authority at firing: the send itself is the check — a 403 from the target's ACL pauses the entry as `denied`, not retried; the tag `by <creator>` names who scheduled it
    - Lists: `sub list` and `later list` show the caller's own entries with bodies; cron's Owner and Maintainers also see the others' id, target, next slot and outcome, never their bodies
    - Text form: five fields, target, then the rest of the line verbatim as the body (no shell-style rejoining); fewer than seven tokens or a macro (`@daily`) refused; JSON delivers the body byte for byte
    - Loader: a stored entry that fails to parse is skipped with a log line and shown as invalid; orphan slot claims are swept at start; `sub delete` removes the subscription first, its claim second; an id collision on add retries
    - Single scheduler (H1): firing needs the record lock `scheduler`, taken with a TTL and extended each pass; a second process stays idle until it gets it. A recurring slot is also claimed atomically with KV `add` on `fired.<id>.<slot>` (the previous slot's key is deleted afterwards); a one-shot is claimed by deleting `later.<id>` (deleted=true). Claim, then send, then record success; a firing is never sent twice
    - Delivery ([RD138](DECISIONS.md#recorded-decisions)): the slot claim holds `sending` until the send is confirmed, then `sent`. An unconfirmed send (transport error or timeout after the request left, or a claim still `sending` found at start after a stop) pauses that subscription with a visible error in `sub list`, and that one-shot is kept as `unconfirmed` in `later list`; neither is sent again. A definite refusal retries with backoff (10 s doubling to 5 min) until the send TTL ends, then records `refused`. A paused subscription is restarted by `sub delete` and `sub add`; nothing resends by itself
    - Claim states (review pins, 2026-10-28): `sending` only while a request is in flight; `retrying <attempt> <until>` after a definite refusal (a restart resumes it); `sent`; `refused`; `unconfirmed`. Definite refusal = a daemon 4xx; a transport failure after the request left, a timeout, a 5xx or a malformed answer is unconfirmed. A paused subscription claims no later slot and counts it as `skipped`; cleanup removes only `sent` and `refused` claims
    - Missed slots: only the current local minute is eligible; nothing is back-filled after a restart, a stall or a slow pass. A pass has a time budget; KV trouble skips with a `missed` count shown by `sub list`; a bus error backs off and never ends the process (M4)
    - One-shots: KV json `later.<id>` {at, target, body, by}; an overdue one at start fires once. A new `at` more than 30 s in the past is refused; horizon 366 days; ids from `secrets`. Outcomes (sent, refused, unknown) of the last 100 one-shots are kept in KV and shown by `later list` (M1, L1)
    - Subscriptions: KV json `sub.<id>` {id, cron, target, body, by}, created with mode add, loaded at start; `sub delete` removes the slot claim too. `sub list` shows expression, target, next slot, last outcome, never bodies to non-managers (L2)
    - Bounds: 100 subscriptions (20 per creator), 1000 pending one-shots (100 per creator); body 16 KiB so the escaped JSON value stays under the KV value cap (M5). Every send has a TTL: a recurring one its period capped at 24 h, a one-shot 24 h, so a target that was down is not flooded (M6). The body is delivered unchanged; topic `cron` or `later`
    - Cron expressions (⚠️ parser, M2): ASCII digits by regex, never `int()` on the token; `*`, `n`, `a-b`, `*/n`, `a-b/n`, lists; one space between fields; dow 0–7 with 7 folded to Sunday; a>b, step 0, empty list items and out-of-range refused with the field's position; Vixie's day rule (a field starting with `*` counts as star). An expression that can never fire under that rule is refused; Feb 29 is valid, with an 8-year next-slot horizon
    - Time zone (M3): Configuration `tz`, default UTC; a zone missing from the sandbox's tzdata makes Configuration invalid at start. Slots are local wall-clock minutes: a skipped minute does not fire, a repeated one fires once (differs from Vixie; the guide says so). Invalid Configuration makes the agent answer with the error and fire nothing, touching no stored entry (L3)
    - Tests on the fixture daemon (`src/clients/fixture`) with an injected clock: the parser table and a brute-force minute-by-minute comparison over a year; a subscription fires once in its minute across two passes and across two processes on the same daemon; a one-shot survives a restart and fires once; a stop between send and success record pauses the subscription and nothing is resent; a refused send is retried and then delivered once; overdue at start fires once; a missed slot is not replayed; refusal without management stores nothing, with a maintainer of the target as positive control; a maintainer of cron who does not manage the target refused; the creator losing management stops the next firing (`revoked`); lists hide others' bodies; a body with double spaces, quotes, a leading `{` and (JSON) a newline arrives unchanged; one bad stored entry leaves the others firing; self target, per-creator caps, cancel by another caller refused; bodies absent from stderr
    - Mutants, each on its owning test: no lock and no slot claim (two sends); one-shot claimed after the send; an unconfirmed send resent instead of paused; a refusal not retried; `>=` for "already fired"; subscriptions or one-shots in memory only; back-fill a missed slot; an ACL refusal retried instead of paused; delete by someone other than the creator or cron's staff; a message without its creator; self target allowed; per-creator caps removed; list showing others' bodies; shell-style rejoined body; loader stopping on one bad entry; `int()` accepting `1_0` or Unicode digits; dow 7 ≠ Sunday; day rule AND instead of OR; no send TTL; stored entries touched while Configuration is invalid
    - Docs and rollout: `docs/agents/cron.md` with the ten headings (at-most-once wording, DST difference, authority sentence, tzdata); `docs` key, index row and generated list; `agent_cron` in the smoke map and shard; version bump and changelog; normal gate under the lock in a `tmp/` worktree; package deployed to `agent.d/cron` disabled; `check-deploy`

- ✅ B.22 at: a deferred call, its own agent, built in 0.9.141 (commit e6cdf8fc; cron's engine moved to `src/agents/shared/scheduler.py`; gate --normal 1216/0; 22 tests plus cron's 49; 67 mutants red; deployed to `agent.d/at` disabled, cron's package refreshed; check-deploy 6/6; [guide](../../docs/agents/at.md#what-it-does)); RT33's may-send check follows ([defaults](agents.md#next-set)); AB2
  - Acceptance: `in 30m` and `at <time>` reach the target exactly once; a pending call survives an agent restart; an overdue one fires once on start; `cancel` stops it; delivery is admitted by the target's own ACL for `#at` (RD140), an ACL refusal pauses the call
  - Mutant: fire twice, lose a pending call on restart, or resend after an ACL refusal; a check must fail
  - 💬👾 Plan (AB2, 2026-10-28), for review before code:
    - Shared engine: cron's `Agent` minus its call grammar moves to `src/agents/shared/scheduler.py` (`Scheduler`: KV entries and claims, the record lock `scheduler`, sweep, RD138 delivery, RD140 pauses, one-shot and subscription firing, lists, delete rules); `cronexpr.py` moves to `shared/`. `cron.py` keeps its grammar, `sub.*`/`later.*` methods and `main`; behaviour unchanged, its 48 tests and 51 mutants rerun against the moved code
    - `#at` (`src/agents/at/at.py`): a thin subclass with only one-shots: `in <duration> <target> <message>` (`90s`, `30m`, `2h`, `3d`, `+` optional), `at <time> <target> <message>` (RFC 3339 with an offset, or `HH:MM` meaning its next occurrence in Configuration `tz`), `list`, `cancel <id>`, `help`; JSON forms `{"method":"in"|"at",…}`, `list`, `cancel`. Topic `at`, tag `by <creator>`, body unchanged
    - Same rules as cron's `later` (RD140): admission to `#at` is enough to schedule; the target must exist and be a queue or agent; its ACL decides at firing; 403 pauses as `denied`, 404 as `unknown destination` (stored distinctly, so a restart keeps the text — the review's cosmetic note), neither retried; other 4xx retry with backoff until the 24 h TTL; unconfirmed pauses; cancel by the creator or `#at`'s Owner/Maintainers; 1000 pending, 100 per creator, 16 KiB, 366 days
    - Package: `src/agents/at/{agent.json,at.py,package.sh,test_at.py}` copying `shared/scheduler.py` and `shared/cronexpr.py`; cron's `package.sh` copies them too
    - Tests (fixture daemon, injected clock): grammar (`in 30m`, `+30m`, `at` with offset, `HH:MM` today vs tomorrow and across a DST change, naive or past refused); fires once; survives a restart; overdue fires once at start; cancel stops it; an ACL refusal pauses and is not retried; the creator tag; the packaged process. Cron's tests stay as the engine's main suite
    - Mutants: fire twice (claim by set, no lock); one-shots in memory only; ACL refusal retried; cancel by anyone; message without its creator; `HH:MM` taken as today when already past; `in` accepting zero or a unitless number; plus cron's set rerun on the shared engine
    - Docs and rollout: `docs/agents/at.md` (ten headings), index row and generated list, smoke-map `agent_at` (and `agent_cron` paths gain the shared files), shard, version bump and changelog, gate, deploy disabled, check-deploy

- ✅ B.18 fetch — built in **0.9.148**, `ada8061c`; normal **1218/0**, **40** assertion-red mutants; deployed explicitly disabled, independently verified, check-deploy **6/6**. [Guide](../../docs/agents/fetch.md#what-it-does), [completion and plan history](done/fetch.md#result). No real public URL call; owner-approved live verification remains separate.

- ✅ B.23 scoped curl and per-instance fetch allowlists — **0.9.153**, `3b6e6669`; final normal **1218/0, 47.513 s**, **59** assertion-red mutants, **79** focused checks; manager deployed disabled and implementer independently verified package/state plus check-deploy **6/6**. [Current guide](../../docs/agents/fetch.md#what-it-does), [result/review/plan history](done/fetch-curl.md#result). Real private scopes remain the owner's choice.

- ⏳ B.25 `#git` autopull from bus channels: `autopull-cmd` events, start check and operator force, `ff`/`mirror` modes, revision deploy, JSON results on `autopull-log`, success/fail scripts; the timer is removed — [design](git-autopull/README.md#two-channels), [tasks](git-autopull/TODO.md#b25-channel-driven-autopull-in-git) (owner, 2026-10-29)

- ⏳ B.26 `#webhook` publishes a hosting service's push to `autopull-cmd` — [tasks](git-autopull/TODO.md#b26-webhook-publishes-to-a-pubsub-channel)

- ⏳ B.28 `#eventlog`: named logs with notice/warning/alert rings, the agent-events model for anything; web page — [design](agent-events/eventlog.md#what-it-does), [tasks](agent-events/TODO.md#b28-eventlog-agent) (owner, 2026-10-29)

- ⏳ B.27 Messaging agents, the [Slack and Telegram shape](../R1.2/services.md#people-and-the-world-outside): post and read, replies routed to the asking session, `sub` forwarding a conversation; one instance per account, the provider token as the record's Secret (owner, 2026-10-29). Unassigned; each needs a plan of its own, with methods and Configuration proposed for approval, before code
  - B.27a Discord
  - B.27b Google Chat
  - B.27c Microsoft Teams
  - B.27d WhatsApp
  - B.27e Facebook Messenger

- ⏳ B.15 Node cleanup with a lasting bound ([RD110](DECISIONS.md#recorded-decisions))
  - Remove stale session records from a list the owner sees first; back up the daemon before.
  - Clean and bound: the Go build cache, the shared Rust target, abrt crash dumps, old release versions, logs and backups, `/tmp` leftovers from our tests.
  - Acceptance: after one week of normal work, none of them has grown past its stated bound, and the disk report shows it.

- ❓ RT23 Agent capabilities: every agent answers `help` (minimum); callers learn which further asks an agent answers before sending them (owner, 2026-10-26)
  - Plus an optional `status` an agent may declare; the web shows it and may offer actions based on it (owner, 2026-10-26)
  - Exposed to bus MCP users too: `status` as an `ab_send` ask, and the capabilities line read before other asks (owner, 2026-10-26)
  - Open choice: [Q191](QUESTIONS.md#agent-capabilities)
  - Acceptance: the web record page and doctor send no ask an agent did not declare; an agent that answers only `help` costs one round trip and never a timeout
  - Status: needs thinking through first (owner, 2026-10-26: "this question needs to be thought through"); decide Q191, then document it (constitution § agent self-description, authoring checklist, create-agent prompt) and update every bundled and standalone agent and the MCP face to the chosen form

- ⏳ RT24 The bus on the bus: the daemon answers as an agent of its own, so a person manages it from an AI CLI with ordinary bus messages (owner, 2026-10-26: "bus itself should be on bus; provide list of methods: help, status, …; so I can manage it from my AI CLIs")
  - Answers the [RT23](#bundled-agents) asks like any agent: `help` with its capabilities line, `status` (`ok: 82 records, 13 readers`, `degraded: …`), `help-json`
  - Methods map one-to-one onto existing daemon calls, never new powers: for example `status`, `version`, `ls`, `manage`, `debug-log on|off`, `realm` (RT22). Each runs with the caller's own authority, checked at the door as today; a method the caller may not use is refused, and help leaves it out
  - Settled (owner, 2026-10-26: "name it #bus; start with help, status, version, ls"): the record is `#bus@<realm>`; the first set is `help`, `status`, `version` and `ls`, all read-only. Management methods come later, one by one
  - To decide in the plan: how answers are shaped for an AI reader, and what `status` counts
  - Acceptance: from an AI CLI session, `ab_send` with ask `help` to the bus record lists its methods; `status` answers in the RT23 form; a method beyond the caller's authority is refused exactly as the CLI refuses it

- ⏳ RT25 One ask design for every kind of agent (owner, 2026-10-26: "want to have uniform design for all kinds of agents"): an ask is checked first, before any work; a known ask is answered; any other answers `unsupported: <ask>`; only a message with no ask reaches the normal flow
  - Built 0.9.101: shared ask-first dispatch, status routing and observed state, no-asks policies, every existing agent entry point, and owning regression mutants. Reader-less status and Slack ask-only initialization remain the follow-up below.
  - Owner's shape: `if ask { switch ask { …; default: return "unsupported" } }; normal_flow()`
  - Applies to: runner and `agent-bus run` scripts ([RD116](DECISIONS.md#recorded-decisions)), standalone Python and TypeScript agents, `#bus`, the MCP face, and the shell template for unpackaged scripts
  - Gap today: each standalone agent has its own `if` chain. Telegram and Slack send an unknown ask into their normal handler, which is safe only because the daemon refuses asks outside the closed set (`src/internal/core/bus.go:1289`)
  - One shared helper per language (Go `agentmeta`, Python and TypeScript `shared/metadata`) holds the switch; an agent passes only its own extra handlers (`status`, native help)
  - `unsupported: <ask>` is a normal reply, never an error or a timeout. The daemon keeps the closed ask set and adds `status`
  - No compatibility layer: every bundled and standalone agent is rewritten to the shape ([RD117](DECISIONS.md#recorded-decisions))
  - `--no-asks` on `agent-bus run`, and `"asks": "none"` in a package, mark a script that knows nothing of asks: the runner answers from metadata what it can and `unsupported: <ask>` for the rest, never running the script for an ask ([RD117](DECISIONS.md#recorded-decisions))
  - The runner answers `status` itself for `starting`, `stopping` and `failed`; a running agent's `status` goes to the script ([RD118](DECISIONS.md#recorded-decisions))
  - Long start-up (owner, 2026-10-26: "some of our existing agents need minutes to load/initialize; most simple agents do not need this"): such an agent takes its reader first and answers `help` and `status` (`starting: <what it is loading>`) while it initializes; ordinary messages stay queued until it is ready. A simple agent has no `starting` of its own
  - Deferred reader-less status: refused/unconfigured entries and a keeper before its reader attaches cannot answer without ask-only consumption. No fallback may take ordinary work. A follow-up would add an ask-only core/API/client read and authority/queue-retention tests, then use it for Slack's pre-serve workspace user/channel pagination (the actual potentially long initialization). Telegram's filter/KV warm-up and the other local startup paths do not need a separate slow-start phase.
  - Then: the constitution § agent self-description, authoring checklist, create-agent prompt and the shell template
  - Acceptance: every agent kind, sent each known ask and one ask it does not handle, answers the known ones and `unsupported: <ask>` for the other, with its normal handler never called; with `--no-asks`, a script that writes a marker file on every run leaves none after each ask; removing one agent's default branch fails a check

- ✅ RT26 One CLI: `agent-bus token` replaces the `agent-bus-token` program (owner, 2026-10-27: "B - use wrapper for token command; so no escalation; if needed we can make agent-bus-token as bash script") ([RD119](DECISIONS.md#recorded-decisions))
  - `agent-bus token [<name>] [--key=<file>] [--fingerprint] [--rotate]` holds the whole logic, moved from `src/cmd/agent-bus-token`
  - `agent-bus-token` becomes a bash wrapper, `exec agent-bus token "$@"`, kept at the same path so existing `authorized_keys` forced commands keep working
  - No escalation over SSH: the forced command reaches only `token`; `token` reads `SSH_ORIGINAL_COMMAND` for its own arguments and never dispatches another verb
  - Docs show `agent-bus token` everywhere; `agent-bus-token` is named once, in the SSH section
  - Acceptance: over the SSH forced command, `token`, `token --fingerprint` and an own name work; another verb in `SSH_ORIGINAL_COMMAND` (`manage …`, `kv …`) is refused with nothing run; another identity's name is refused as today. Mutant: let the wrapper or `token` pass a verb through, and the check fails
  - Done 0.9.102–0.9.103 (`724169c1`…`7b9071b7`): normal 1191/0, seven mutants red, real SSH onboarding, fresh install and the .100→.103 upgrade green; live token and SSH fingerprint checked

- ✅ RT27 `agent-bus help` in man style, main commands first (owner, 2026-10-27: "do deep logical review of agent-bus help; place main commands first; group commands; use commonly accepted linux man-style presentation … add github link") ([RD120](DECISIONS.md#recorded-decisions))
  - General help: USAGE, MAIN COMMANDS (status, ls, send, call, consume, reply, run), then MESSAGES, AGENTS AND RECORDS, STORAGE AND LOCKS, CREDENTIALS, NAMES, ENVIRONMENT, EXIT STATUS, SEE ALSO; one line per command; the GitHub link in the header and SEE ALSO; every doc reference in CLI help is a full `https://github.com/parf/ai-agent-bus/blob/main/…` URL, never a repository path (owner, 2026-10-27: "use URLs instead")
  - `agent-bus help <command>` and `<command> --help` give one command's synopsis and options, one per line, in one column, spelled `--name=value`; `help '#agent'` stays an agent's help, told apart by the `#`
  - `send` and `call` get their own usage through `help send` and `help call`; `call <to> --help` stays an ask
  - Renames, the old spelling kept working but hidden from help: `agent-template` → `config`; `ls -h` → `ls --table`; `register --addr` → `--service-addr`; `channel create` → `register --kind=queue|pubsub`
  - Fix the review findings: the broken `run` block, stray description lines, missing descriptions (`ack`, `done`, `stop`, `logs`, `-N`), ENVIRONMENT (`AGENT_BUS_TLS_FINGERPRINT`, `NO_COLOR`), EXIT STATUS checked against the code
  - The same layout for `agent-bus-admin` and `agent-bus-runner` help; docs/user/cli.md follows the new names
  - Acceptance: `agent-bus help` opens with the main commands and fits one screen per group; every verb has `help <verb>` output that lists every option its parser accepts (a check compares the two); each old spelling still works; the EXIT STATUS codes match a check that triggers each
  - Done 0.9.104 (`77ea51d2`): normal 1187/0, 15 mutants red, parser/help parity and alias checks green; CLI, admin and runner binaries deployed
  - Reopened (owner, 2026-10-27: "agent-bus ls -h is missing (check for other missing options) … and helps; agent-bus help ls is also incomplete"): short aliases show beside their long form (`-h, --table`); every command page is complete — what each argument means, every option's values and default, an example — and loses no fact the 0.9.103 usage stated
  - Done 0.9.105 (`1b405b6e`): short aliases shown, every page has DESCRIPTION, OPTIONS with values and defaults, EXAMPLES; 0.9.103 facts restored; normal 1187/0, nine mutants red

- ✅ RT28 The session-set agent becomes `#tmux-sessions` and names each tmux session (owner, 2026-10-27) ([RD121](DECISIONS.md#recorded-decisions))
  - Rename `#sessions` to `#tmux-sessions`: record, package directory, docs, installer
  - A session entry may carry `tmux`, the exact tmux session name (`tmux attach -t AB`); default derived from the bus name as today
  - Fix what we met: `tag()` still strips digits and `-` (RD113 allows `a-z 0-9 - _`)
  - A node's own set is local data, not release content; this node's lives in `local/dev-setup.md` (owner: "this is our new dev setup; do not make it part of the release")
  - Acceptance: a set with `tmux` names starts sessions under exactly those names; a tag with digits and `-` reaches each record unchanged; a second `start` reports each as already running
  - Done 0.9.107 (`fff602f0`): normal 1191/0, seven mutants red; `#tmux-sessions` live, `#sessions` retired; this node's set waits for the owner's switchover
  - Reopened (owner, 2026-10-27): one tmux `session` per set, each entry a `window` in it (keys `session`, `windows`, `window`); methods `start`, `stop`, `restart` for all or named windows (start adds only missing windows), `list` of the configured windows with their real state; the `status` ask checks the real tmux windows and bus readers (`ok` all up, `degraded` some missing, `failed` none or no session); generic, not AI-only (owner: "this is a generic tmux session starter"): a window is `name` (the tmux window name, required, used to check it runs), `cwd`, a `command` string run by `sh -c` as tmux does, and optional `env`, an env-file string (`KEY=VALUE` lines) or an object, never credentials; nothing runtime- or bus-specific, every method gated by the record ACL alone
  - Done 0.9.110 (`b4317179`): one session of named command windows, start/stop/restart/list, observed status, shared Python env-file parser on the Go vectors, `config.json.dist`; normal 1187/0, 13 mutants red

- 🚫 RT30 `#tmux-sessions` runs under the person's runner, not as a standalone service (owner, 2026-10-27: "any reason why this is not a part of Runner?" → "yes")
  - A kept agent of the personal runner with `"sandbox": "off"`, declared and documented as such: it needs the person's tmux server, `systemd-run --user --scope`, their HOME logins and network, and windows that outlive any call
  - Windows already leave the agent's cgroup through `systemd-run --user --scope`; reload, stop or a crash of the agent never touches them
  - The main and the `ab-dvp` instance move with their Configuration, ACL and owner; their standalone units are removed; a second instance is a second runner record
  - Acceptance: both instances answer help and status through the runner; `reload` and `stop` of the agent leave every tmux window and its pid unchanged; no standalone tmux-sessions unit remains
  - Cancelled 2026-10-27: the person's runner runs as its own account (`parf-ab-runner`), not as the person, so it cannot reach their tmux server, user manager or logins without crossing the process boundary. `#tmux-sessions` stays a standalone service run as the person

- ⏳ RT31 (code ✅ 0.9.149, commit c952c241: setup verb, tmux-sessions package, refresh on git-install/upgrade; gate --normal 1218/0; 15 Go mutants red; real-systemd container acceptance 10/10 on a disposable user, run on a test-only archive copy with agents/db stripped (issue G3), scope mutant red; this node's #tmux-sessions switchover is the manager's) A person's user-account runner, `#runner/<person>-user`, for agents that must act as the person (owner, 2026-10-27: "simpler and more correct to bring up a separate parf-runner") ([RD124](DECISIONS.md#recorded-decisions))
  - Runs as the person under their systemd user manager (`systemctl --user`), not under a `<person>-ab-runner` account; installed only on request, like the system runner
  - `agent-bus-setup --runner install|uninstall|enable|disable --user-runner` installs it (named `--user-runner` because setup's `--user` already takes `account=user`) (built first), with `--dry-run`, idempotent; upgrade and git-install refresh it (owner: "make sure our agent-bus-setup can bring it up")
  - Holds only person-context agents, starting with `#tmux-sessions`; every other agent stays in the person's system runner; sandbox stays on by default per agent, and `#tmux-sessions` declares it off
  - This node's `#runner/parf-user` is created and started through that setup verb, never by hand
  - Docs open with why: some agents must act as the person (tmux server, user manager, CLI logins, files), which the system runner's own account deliberately cannot; a table of when to use which
  - Its agents' tokens and secrets live in the person's home, readable by the person's own processes (their AI sessions included); docs say so where it is installed
  - `#tmux-sessions` (main and the `ab-dvp` instance) moves into it with Configuration, ACL, owner and tags; the standalone units go
  - ⏳ Follow-up, before this node's switchover (manager, 2026-10-28; code ✅ 0.9.151, commit 9905f3ff: `state_home` + legacy default; gate --normal 1214/0; 5 unit mutants red; container acceptance 11/11, no-default mutant red there; switchover is the manager's): **state handover**. Each standalone unit runs with `XDG_STATE_HOME=/var/lib/agent-bus-tmux-sessions[-<instance>]`, and sessions.py passes it into every window, so the programs in the windows (the AI launchers) keep their live state there: `sessions/*.json`, `*.lock`/`pid.*` per running window, `sessions/runs/{claude,codex,opencode}-*`, `claude/`, `opencode/`. Under the user-account runner that variable would differ, so the windows' programs would see an empty state: duplicate starts, orphaned locks, lost session bindings
    - To decide in the plan (AB2, for review): keep each instance's `XDG_STATE_HOME` stable across the move — a per-record setting the agent passes into its windows, defaulting to the old path when it exists — or a checked, documented move of that directory into the person's state tree with the old units stopped first; either way root never writes into the home except as the person, and nothing running is restarted
    - Acceptance: a set started under the old standalone unit is reported running (`status` ok, `start` answers `already running`, no new window, the same pids and lock files) after the switch to the user-account runner, in the real-systemd container
    - 💬👾 Plan (AB2, 2026-10-29; manager's pick, option 1): keep each instance's `XDG_STATE_HOME`; nothing moves, nothing restarts
      - A new optional Configuration key `state_home` (an existing absolute directory) is passed into every window as `XDG_STATE_HOME`; a window's own `env` still overrides it
      - Absent: the legacy `/var/lib/agent-bus-tmux-sessions[-<instance>]` (instance from the agent name, not the session) when it is a real directory (lstat, no symlink) owned by the running uid; else the inherited value, as today. Under the old unit both give the same path, so the switch changes no window's environment
      - Windows already running keep their environment (they live in the user manager's `ab-session-*` scope, not the agent's cgroup); the rule matters for every later start and restart
      - Sandbox: the user-runner unit has no sandbox directives and `#tmux-sessions` declares sandbox off; the old dirs are `parf:dvp` 0755, so parf reaches them; the tmux socket stays in `/tmp/tmux-<uid>` (neither unit has `PrivateTmp`)
      - Acceptance in the real-systemd container (`user-runner.sh`): the old standalone unit starts a set whose window writes a lock and pid under its `XDG_STATE_HOME`; the old unit stops, the user runner takes the agent; `status` ok, `start` answers `already running`, the same pids and lock files; a `restart` of one window gets the legacy path in `/proc/<pid>/environ`. Mutants: drop the legacy default, drop the uid check, let `state_home` lose to the inherited value; each fails its unit test on an assertion
  - Acceptance: a fresh `agent-bus-setup` install brings it up on a disposable user; `#runner/parf-user` runs as parf in the user manager; both tmux instances answer help and status through it; reload, stop and a crash of the agent leave every tmux window and pid unchanged; no standalone tmux-sessions unit remains
  - 💬👾 Plan (AB2, 2026-10-28), for review before code:
    - Verb: `sudo agent-bus-setup --runner install|uninstall|enable|disable|refresh --user-runner [--for <person>] [--dry-run]`, `--for` defaulting to the invoker (SUDO_USER). Same observe → plan → apply shape as the system runner (`runner_install.go`): a second run plans nothing; `--dry-run` prints the plan. The person must be an active bus User and a Linux login account of the same name, else refused before any change
    - Layout, per person: unit template `/etc/systemd/user/agent-bus-runner.service` (root-owned, shared; `ConditionPathExists=%h/.local/state/agent-bus/runner/runner.env`, `ExecStart=/usr/local/bin/agent-bus-runner --agents %h/.local/share/agent-bus/agent.d --host %h/.local/state/agent-bus/runner`, `Delegate=yes`, `KillMode=mixed`, `Restart=on-failure`); state `~/.local/state/agent-bus/runner` (0700: runner.env, inventory, tokens) and code `~/.local/share/agent-bus/agent.d` (person-only packages; starts empty, so no shared agent.d package ever runs as the person)
    - Root never writes through the person's tree: every directory and file under the home is made by a child running as the person (`setpriv --reuid/--regid/--init-groups`), the token passed on stdin, so a planted symlink can only redirect into what the person can already write
    - Bus: `#runner/<person>-user@<host>` registered as the person (allow `@owner`), its token in runner.env (0600, the person's); linger enabled (`loginctl enable-linger`) so it starts without a login; `systemctl --user -M <person>@ enable --now agent-bus-runner.service`
    - uninstall: disable, remove the record, remove the state dir (`--purge` also the person's agent.d); linger is left as found (it may serve other things) and the plan says so; the template goes with the last user runner. enable/disable act on the unit only
    - Refresh: `git-install.sh` and `upgrade` rewrite the template when it differs and `try-restart` each installed user runner; tmux sessions are untouched because each lives in its own `ab-session-<name>.scope` under the user manager (`systemd-run --user --scope`), outside the runner's cgroup
    - `#tmux-sessions` as a runner package: `src/agents/standalone/tmux-sessions/package.sh` → `agent.d/tmux-sessions`; inventory entries keep the existing records by `name` (`#tmux-sessions@…`, `#tmux-sessions/ab-dvp@…`) with `sandbox: off`, tokens handed over as for any existing agent; Configuration, ACL, owner and tags stay on the records. This node's switchover (stop the two standalone units, hand tokens, reload) is the manager's, from the documented steps
    - Docs: `docs/07-setup.md` § a person's user-account runner (why, a when-to-use-which table, the secrets-readable-by-your-processes warning), runner configuration, the tmux-sessions guide's install section
    - Tests: Go plan/observe tests as for the system runner (refusals: unknown person, no Linux account, a non-user record under the name, a record owned by someone else); a smoke dry-run case; a slow container acceptance on the real-systemd image (plus tmux): fresh install, disposable user, `--runner install --user-runner`, the runner process runs as that user under `user@<uid>`, a tmux-sessions package answers help and status through it and starts a session; tmux server and window pids unchanged across runner reload, `systemctl --user stop`/start and `kill -9` of the runner; second install plans nothing; uninstall leaves no unit, record or state
    - After review by #claude/ab2-review (these supersede the lines above where they differ):
      - Home access (H1, M3): root neither reads nor writes under the person's home. One helper runs every home step as the person (setpriv with the person's uid, gid and groups, no_new_privs, minimal env, umask 077, no inherited fds, stdin for the token): observation (returns only booleans and a token hash), directory creation, runner.env (O_EXCL|O_NOFOLLOW, 0600), packaging into the person's agent.d, uninstall/purge removal (non-following). Root touches only `/etc/systemd/user`, the registry, `loginctl` and `systemctl --user -M`. Test: a planted symlink to a root-only file is neither read nor changed by observe, install or uninstall
      - Person (H2): `--for` or SUDO_USER, never empty; refused: uid 0, uid below UID_MIN, a nologin/false shell, names outside the bus and Linux rules, a bus User without that Linux account. The bus keeps no uid binding: the same name is the whole binding, acceptable because root runs the install. runner.env holds a token for the agent record `#runner/<person>-user` only, never the person's own token
      - Refresh (M4, L10): installed user runners are enumerated from the registry's `#runner/*-user` records, never by scanning /home; each person is restarted one by one with a timeout and its own result; the template goes when no person has a record or an enabled unit
      - User manager (M5): needs `systemctl --user -M` (systemd version checked); after enabling linger, wait bounded for `/run/user/<uid>/bus`; a failure leaves nothing half-applied and a second run resumes; dry-run runs no helper and starts nothing, printing actions only and no secret (L8)
      - Unit and survival (M6, M7): the unit carries no PrivateTmp/ProtectHome/RuntimeDirectory (the agent must act in the person's full context; the per-agent sandbox stays the control), and has `RestartSec`, a StartLimit and `TimeoutStopSec`. The container test also asserts the tmux server's cgroup is an `ab-session-*.scope` outside the runner unit and that the scopes stay active. A command-builder guard: setup never emits a stop or restart of `user@<uid>.service`, `loginctl terminate-user|kill-user`, a stop of `ab-session-*`, `daemon-reexec` or a kill outside the runner unit
      - Token rotation is an explicit action; a second install compares by hash inside the helper (L8). Uninstall leaves the hosted tmux records in place, unhosted, and warns (L9)
      - Added mutants: root reads through a planted link; `--for root` accepted; package.sh run as root into the home; removal as root; a user-manager restart emitted; `-M` failure leaves partial state; dry-run runs the helper; template without RestartSec/StartLimit; enumeration by scanning /home; user-manager `daemon-reload` killing scopes (proven harmless in the container)
    - Mutants: tmux started without `--scope` (pids change on stop); the template without `Delegate=yes` or with `KillMode=control-group` reaching the scopes (pids change); root writing into the home (a planted symlink to a root-owned file is followed); install not idempotent; a shared agent.d package visible to the user runner

- ☐ RT32 Every bundled agent has its own doc page in one structure, listed on one index page, and returns the page's link (owner, 2026-10-28: "make sure to have good doc page with a list of bundled agents; every agent must have its own doc page; return this page from agent (use github link); check how we do these pages (structure)")
  - Index: [docs/agents/README.md](../../docs/agents/README.md#choose-an-agent) lists every bundled agent, one row each, linking its page
  - One structure, the one most pages already use (git, slack, telegram, systemd, server-info): Why you'd want it · What it does · You'll need · Try this first · Use examples · Installing it · Configuration · Secret keys · Who may use it (roles) · Troubleshooting; a section an agent has nothing for says so in one line rather than vanishing
  - Brought up to it: `voyage`, `tmux-sessions`, `debug`, `bus`, `qdrant`; every new agent (`#qdrant` proxy, `#rag`, the database proxy, mail, webhook, cron, fetch) lands with its page
  - Link from the agent: each `agent.json` carries its page as a full GitHub URL (built from the Source URL, like the CLI's SEE ALSO), and `help` prints it; `#bus` too
  - Acceptance: one check fails when a bundled agent lacks a page, an index row, the structure's headings, or a `docs` URL that resolves to an existing file in the checkout; every agent's live `help` shows its URL
  - Mutant: drop one agent's page, index row or `docs` URL; the check must fail
  - Status 2026-10-28 (AB2): landed 0.9.131 `ae64582b` (loader `38b87a09`, renames `42b6bd64`, pages `791312cc` `e4407467` `52588f0b` `e0d99eb8`); normal 1211/0; 13 check mutants and 3 renderer mutants red; runner, packages and web deployed, check-deploy 6/6; live help shows `Docs:` for every agent reachable. rag complied in 0.9.132 `2960c44b`, `PENDING` is empty. Open: `#bus` Docs needs a daemon restart (owner)
  - Plan (AB2, 2026-10-28; review by `#claude/ab2-review` before code; review findings of 2026-10-28 folded in):

    | Step | Change |
    |---|---|
    | 1 Loader | Own commit and bump: `docs` in Go `agentmeta/metadata.go` (struct, unknown-key check) and `help.go` (help-json struct, footer), Python `metadata.py` (key list, footer), TS `metadata.ts` (type, payload, footer). `Docs: <url>` is the footer's last line, before `More:`, for every agent with or without a version. Golden help fixtures updated in the same commit. No `docs` → no `Docs:` line (tested), and unknown-key refusal still holds for other keys. Deploy daemon, runner and CLI before any manifest carries `docs`; the sha goes to xAB2 |
    | 2 Renames | Own commit: git `Roles and boundaries`, systemd `Calls and permissions`, logwatch `Install it` / `Configuration and Secret` (split), ai and sms (`Configuration and Secret keys` split, `Roles and access` / `Access and results` → `Who may use it` keeping results text, `If something goes wrong` → `Troubleshooting`, sms `First call` → `Try this first`, sms sections reordered; ai gains `Try this first`). Inbound links fixed in `docs/decisions.md` (link targets only, row text unchanged), `docs/agents/README.md`, `logwatch.md`, `src/agents/standalone/{systemd,git}/README.md`; linkcheck over `docs/` and those READMEs. Links from outside the repo to old anchors break; accepted unless the owner objects |
    | 3 Pages | One commit per page: voyage, tmux-sessions, debug, bus rewritten into the structure; each commit message lists old facts → new section so `#claude/ab2-review` can diff them. Empty section = one line. qdrant: xAB2 in R.4; rag: xAB; db proxy: AB |
    | 4 Index | Generated block atop `docs/agents/README.md`, one marker pair exactly once: one line per agent, linked name + `agent.json` `description`; bus description becomes `display.BusDescription` (Go), read by the generator from that one Go line, with a Go test that `#bus` uses it. `src/agents/agent_docs.py --write` is idempotent and refuses without markers. The table below keeps only How it runs / What it needs / Who (its home); the list owns name and description |
    | 5 `docs` keys + check | `docs` added to every committed manifest in the same commit as the check; rag, db and the qdrant proxy owners add their own key after the loader sha reaches them, excepted until then. `src/agents/agent_docs_test.py` (L, fast list), anchored on the checkout of the test file: globs `src/agents/**/agent.json`, asserts the manifest count equals the list's line count (the both-way match guards a dropped agent); the `BusDescription` parse fails loudly when the const is missing; both-way match manifests ↔ list ↔ table rows ↔ pages (exact `<page>.md` target, no fragment); real `##` headings outside fences, exact text, ten as an ordered subsequence; `docs` = `display.SourceURL` (read from `urls.go`) + `/docs/agents/<page>.md`, existing, and the same file as the index target; generated block equals a fresh render and one line is checked against an independent literal. Exceptions are one constant with a reason each (rag, qdrant, db until their pages land) and the check fails when an excepted agent becomes compliant. `metadata_contract.py` and the TS/Go tests assert `Docs: <url>` placement and the help-json key, with a fixture carrying `docs` |
    | Mutants | delete a page; drop an index row; drop a heading; a heading only inside a fence; drop a `docs` key; `docs` pointing at another agent's page; at a missing file; edit one list line; delete the markers; rename `BusDescription`; empty the exception list; a stale exception; drop the `Docs:` line from each renderer — each fails on an assertion |
    | 6 Web (last, optional) | Docs row in the agent page's About |
    | Land | normal gate under `smoke-gate` per code commit; version slots from `#codex/xab` (R.4 keeps .128); deploy changed packages after the loader is live; `check-deploy`; live check `for a in $(agent-bus ls --kind=agent …); do agent-bus help "$a" \| grep '^Docs: '; done`. No push |

## RT34 Agent events

⏳ Owner, 2026-10-29, under discussion: per-agent notice/warning/alert rings in
the daemon, written by the runner, the daemon, the agent and diagnostics, shown
by the API and the web to Owner and Maintainers ([design](agent-events/README.md#levels)).
Settings in the bus setup ([RT22](#bundled-agents)). Unassigned until the owner
asks for the API shape; then runner, daemon and web parts are split.

## RT33 May-send for another name

- ⏳ RT33 `GET /may-send?who=<name>&name=<target>` and its use by every agent that sends for someone else (daemon and clients ✅ 0.9.141 `2889a06b`/`f5442b9a`, deployed, 14 mutants, reviewed; Slack, Telegram and logwatch `sub` ✅ 0.9.145 `a54afe01`/`ba3b9c82`, 20 mutants, gate 233/0, reviewed, not yet deployed; IMAP `sub` (xAB2) open) ([RD141](DECISIONS.md#recorded-decisions))
  - Daemon (AB): `who` is optional; without it the answer is for the caller, as today. With it, the daemon answers by Send's own admission rule for `who`, only for a target the caller can see, exactly as `/manages?who=` limits disclosure; a hidden target answers as not found. No refusal counter or activity is recorded. Docs: [daemon API § checking permission to send](../../docs/09-daemon-api.md#checking-permission-to-send). Clients: Python and TypeScript `may_send(who, name)`. Daemon restart with the owner's OK
  - Agents, after the daemon lands: cron and `#at` (AB2, ✅ 0.9.144, commit 7114ba9a: one creator verdict before every firing and retry, per-pass circuit breaker, 85 mutants, gate green, deployed disabled), IMAP `sub` (xAB2), Slack, Telegram and logwatch `sub` (whoever is free). Add and each send check the author; a refusal at add stores nothing, a refusal at send pauses the entry visibly (as an ACL refusal under RD140); an unavailable check refuses rather than sends
  - Acceptance: a caller who may not send to X cannot schedule, defer or subscribe to X through any of these agents; revoking the author's access stops the next send; the caller-only form is unchanged
  - Mutants: answer for the caller instead of `who`; disclose a hidden target; skip the check at add; skip it before a send; send when the check is unavailable — each fails a check
  - Plan for the daemon and clients (AB, 2026-10-28), for review before code
    - Contract: `who` present once and nonempty selects the form; an empty or repeated `who`, or any other key, is 400; absent keeps today's caller-only form exactly. The form with `who`: a target the caller cannot see (`canSee`: manages or may), absent or inactive is 404 `no such name`, the same text for each, so hidden and absent cannot be told apart. For a visible target the answer is 200 `{"may_send":true|false}`: false when `who` is unknown or not acting (inactive), or when Send's admission refuses `who` for any reason (ACL, Personal, User-inbox rule, external kind). A bad name in `who` or `name` is 400. The reason of a refusal is not returned, as `/manages` returns none
    - Core: `Bus.MaySendAs(caller, who, to) (bool, error)` beside `Manages`: canon both, one lock hold, `entity`+`canSee` else `ErrUnknown`, then `acting(who)` and `sendTarget(who, to)`; the true/false form never touches the refusal counter, the queues or activity. The caller-only `MaySend` is untouched. `api.maySend` branches on `who`
    - Clients: Python `Bus.may_send(who, name) -> bool` and TypeScript `bus.maySend(who, name): Promise<boolean>` (`maySend`, as `manages` is `manages`) call `GET /may-send` with both; true only for `{"may_send":true}`; a 404, a refusal, a timeout or any error raises, never returns false, so an unavailable check is never read as a no
    - Docs: `docs/09-daemon-api.md` § checking permission to send: the `who` form, its disclosure limit, and the client calls; the "no `who` option" sentence is replaced
    - After ab2-review (2026-10-28): the answer echoes `who` in canonical form, `{"may_send":b,"who":w}`, and a client raises on a missing or different `who` (an older daemon ignores `who` and answers for the caller). Disclosure: the allow list is already public on a visible record (`Record.Public` keeps `Allow`), so the answer adds no new read: accepted as `/manages?who=` precedent and documented, with the rule that an agent passes only the authenticated `from`. Unknown, inactive and refused `who` answer the same bytes but the echo. A refusal counter is not moved (the `who` form answers through `dryRefusal`, not `reply`). Clients raise the daemon's 404 as its refusal and everything else as an error. More mutants: unknown who as 404; repeated `who` accepted; echo unchecked in a client; `name` used as `who`; Manages semantics; caller-only form changed
    - Tests: API: for each of allowed, ACL-refused, Personal-refused, User-inbox, external, inactive `who`, unknown `who`, the answer equals what `/send` would do for `who`; a hidden target and an absent one answer identically 404; counters, queues, recent and activity are unchanged; `who` empty or repeated is 400; the caller-only form is unchanged. Clients: against the fixture daemon, true, false, 404 raises. Mutants (⚠️ authority): answer for the caller instead of `who`; disclose a hidden target (skip `canSee`); skip `acting(who)`; record a refusal; treat an error as false in a client

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
