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
  - State: built 0.8.143; [verification](done/record-tags.md#verification); multiple-tag matching is built ([№RTA1](#record-tags-acceptance))

- ✅ RT8 A shared executable protocol conformance suite for the clients ([result](DONE.md#rt8-shared-client-conformance))
  - Home: [protocol conformance](modules.md#protocol-conformance), [№RD90](DECISIONS.md#recorded-decisions)

- ✅ RT9 [Agent method metadata](done/method-metadata.md#method-metadata)
  - Format: settled with the owner on 2026-10-19 ([settled so far](done/method-metadata.md#settled-so-far)): typed fields in `agent.json`, commented `env.dist`, and the `help`, `help-config`, `help-secret` and `help-json` asks; MCP generation is [R1.2](../R1.2/TODO.md#dependencies)
  - ✅ RT9.1 The runner and `agent-bus run` read the new `agent.json` fields strictly and answer `help`, `help-config`, `help-secret` and `help-json` themselves for packaged agents, without running the script; the CLI and MCP face send the three new asks (`agent-bus call <to> --help-config`, `--help-secret`, `--help-json`). Acceptance: a packaged fixture with no help code answers all four from its files; `help-secret` names keys and comments and never a value (sentinel); an unknown field is refused by name; an enum value outside the list is shown. Mutants: the script run for a help ask; a secret value in `help-secret`
  - ✅ RT9.3 The web face shows it all (owner, 2026-10-19: "once done, the web must show all this data"): an agent's record page gets an "About this agent" card (description, version, license, author, homepage), a Methods table (name, arguments with types and enums, returns, role, destructive), and the Configuration editor shows each documented key's type, help and example beside the JSON, warning about undocumented keys. The Secret field lists the expected keys from `env.dist` with their comments and which are required, never a value. The web gets it through the same asks, or from the runner for its packaged agents. Acceptance: a fixture agent's page shows every field; a secret sentinel never appears; an agent answering only `--help` shows that text and no tables. Mutants: the secret value rendered; an enum dropped
  - ✅ RT9.2 Every bundled agent's `agent.json` and `env.dist` carry the metadata (methods, args, Configuration keys, description, license, author, homepage) and their hand-written help code is removed where RT9.1 covers it; the guides link `--help-config`, `--help-secret` and `--help-json`. Acceptance: each agent's `--help` matches its real methods (a parser check per agent); removing a method from the code without its entry fails the check
  - ✅ RT9.4 [Usable Methods cards](../../src/web/web-face/records.md#method-rows), built in 0.9.44: native disclosure rows, signatures, typed args/enums, CLI/JSON examples from declared `body_formats`, GET filter, meaningful fields and compact settings. RT9.4b reports only role-hidden counts/needed roles. Phone-width/no-JS browser check passed; help-listed and hidden-count mutants compiled and failed assertions under `--slow`; restored full gate 1164/0. A call-execution form remains later.

- ⚠️ RT10 [Runner](runner.md#runner)
  - Details: decided; [waves](#runner-waves) proposed
  - State: one-message serving built 0.8.123; full evidence remains in [runner waves](#runner-waves)
  - Depends: [method metadata](done/method-metadata.md#method-metadata); the pool hostname field, now R1 ([where a member says it is](runner-implementation.md#where-a-member-says-it-is))

- ⚠️ RT11 [Runners section in the web face](runner/web-face.md#goal) — status, per-agent controls, host files and instances are built; remaining acceptance is tracked in the checklist
  - State: the information page and per-agent management page are separate; denied management shows Access denied
  - Home: [Minimum and acceptance checklist](runner/web-face.md#minimum-checklist)

- ✅ RT12 [Agents for the runner](agents.md#what-moved-and-what-r1-adds): telegram, systemd, logwatch
  - Depends: Telegram and logwatch: the runner keeping an agent it did not write (№M.1 and №M.2, built; №M.11 for the install) and the kept process form for state between messages (№M.3, №M.9). Systemd needs none of them — it is a root unit of its own running `agent-bus run`
  - Settled by: owner, 2026-10-17
  - Home: [tasks](#bundled-agents)

- ✅ RT29 Record export and import (0.9.108)
  - Built: Owner-only encrypted permanent records, private KV and credentials, explicit selection/conflict/realm choices, atomic import, public matching backup-info, and independent bounded backup storage; [current contract](../../docs/07-setup.md#encrypted-record-backups).
  - Accepted on disposable daemons: real age/CLI/SQLite rebuild, old runner and standalone token clients after restart, wrong key and sidecar refusal, whole-transaction rollback, Owner doors and explicit conflicts; sixteen owning mutants and normal1187/0. No live export/import run.

- ⏳ RT14 [Installable distribution](distribution.md#release-artifacts): P.1–P.3
  - Built: runtime packages and verified manifests ship in build/package output; the installed-tree checks run outside the checkout.
  - Install: [npm full and client-only packages](distribution.md#simple-install), per the owner’s replacement decision RD178. No release signing; the shell installer is deferred.
  - Platforms and native evidence: [distribution](distribution.md#simple-install) and the [Mac run list](mac-run-list.md#a-linux-aarch64).

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

- ✅ RT21 A client calling itself gets its handler's answer, never its own request
  - Found by Codex #3 in B.13, 2026-10-19: the Python client's `Bus.call` to its own name consumes the request it just sent (the same inbox, topic and tag) as the correlated answer, before the handler runs. All six clients had it
  - Built on `ab-w2a/RT21`, with #ab-w2c's RT8 Python, TypeScript and Go slices: an unmarked message carrying the call's own message ID is work for a free serving worker, drained like other taken work; PHP, serving one message at a time, runs its handler in the call and refuses outside `serve()`. Shared `self-call.json` runs in all six adapters, direct and through another reply inbox; red on the old code in each
  - Mutants, 2026-10-30: the own-request check off in each client compiled and failed its self-call checks on assertions (Python 5, TypeScript 4, Go 3, Rust 2, Java 4, PHP 5)

- ✅ J27 Lock waiters are served in arrival order ([issue](../issues/2026-10/2026-10-30.md#locks)) — **0.9.185**, `de98f55d`; deployed 2026-10-30 ([result](DONE.md#j27-lock-waiters-in-arrival-order))

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
- [№RT9](#objective) — [Agent method metadata](done/method-metadata.md#method-metadata)
- [№RT5](#objective) — [Client libraries](done/client-libraries.md#scope)

- ⛔ RT17 Release distributions
  - Depends: Release builds and runnable daemon/runner roles; choose publication names and supported platforms before packaging

Name implementation waves and falsifiable acceptance after those choices.
Federation's distinct-node acceptance moved with [its task](../R1.2/TODO.md#federation).

## Record tags acceptance

Built 0.8.143. [Current contract](../../docs/03-records.md#tags),
[verification](done/record-tags.md#verification). Multiple-tag matching is
settled — [№RD85](DECISIONS.md#recorded-decisions) — and built in the listing
query; the web UI selects one label.

`RTA…` labels this section's rows.

- ✅ RTA1 A multiple-tag query selects the records carrying every tag listed ([record tags](../../docs/03-records.md#tags)): `ls --tag=a,b` and `/ls?tag=a,b` keep only records holding every label; one tag is unchanged, an unknown tag gives an empty result, `no-tags` beside a real tag is refused. Check `TestListingFiltersByEveryListedTag`; mutants (match any label; accept `no-tags` with a tag) each fail it on an assertion
  - RTA1c: an empty `--tag`/`--tagged` value is refused by name on `ls`, `call --tagged` and `#bus ls`, and a refused call sends nothing. Check `TestAnEmptyTagValueIsRefusedNotDropped` (real CLI, real daemon handler); mutants (CLI guard off; `#bus` guard off) each fail it on an assertion

## Group walk acceptance

Settled as [№RD87](DECISIONS.md#recorded-decisions). `RGW…` labels this
section's rows.

- ✅ RGW1 Every group-membership walk is iterative
  - State: ACL membership, directory indexing and publication expansion use
    explicit work lists; role assignment keeps its existing iterative walk.
  - Checks: `TestGroupWalksUseBoundedStack` resolves a cyclic 1024-group chain
    through all four walks under a 128 KiB goroutine stack; positive/negative
    membership and publication order/deduplication are asserted.
  - Evidence: full core suite green; four compiling recursion mutants fail
    the owning stack assertion (ACL, directory, publication, roles), restored.
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

- ✅ M.3 Processes
  - M.3a kept `jsonl` with `in_flight` and M.3b growth/idle shrink are built on `ab-w2c/M3`, extracted from reviewed source `e80f5143`; M.3c SIGHUP/restart/report was already built. Settings-file overlays remain in the separate M.5 landing.
  - The owner approved answer option (a), 2026-10-30 15:25: `{"message_id":"<echoed>","body":"<answer>"}`; absent/empty body is done, malformed/unknown lines are logged and dropped, and an exit logs held messages and restarts with backoff ([long-lived agents](runner-implementation.md#long-lived-agents)). No parser seam change was needed.
  - Acceptance: a pool grows under load and shrinks after idle; a killed child is restarted and reported; an unhandled SIGHUP restarts it; `in_flight` 2 holds two messages and answers in either order. Deadlines, blocked stdin, idle TERM refusal, retiring pools and late one-shot fallback are checked.
  - Result: focused runner `-race` on the M.3-only split, 146/0, no skips (29.7 s), both changed guides pass docs-order; [evidence](DONE.md#m3-kept-jsonl-pools). The split-only hunks receive a non-author read before landing.

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

- ✅ DR.1 The dedicated runner applies the agent's bus secret — **0.9.174**, `70fcfa4a` ([result](DONE.md#dr1-the-dedicated-runner-applies-the-agents-bus-secret))

- ✅ M.5 The host's controls
  - M.5a dependency order/cycle refusal was already built. M.5b reload SIGHUP and drain use M.3; the sibling test registers a second agent and proves the untouched keeper gets no signal. M.5c shipped JSON overrides and their bus read/write/delete calls are built on `ab-w2c/M5`, stacked after M.3.
  - A settings patch is an object naming a file the agent ships; agent then instance JSON Merge Patches are mounted read-only by bubblewrap. Null patches, unshipped files and sandbox-off overrides are refused by file name. The digest includes the override so a changed kept agent restarts after drain; the author file stays intact.
  - Existing enable/disable/start/stop/restart and host-file controls remain; backup moved to R1.1. The current contract is [runner operations](../../docs/runner/operations.md#the-hosts-files-over-the-bus) and [settings overrides](../../docs/runner/configuration.md#overriding-a-settings-file).
  - Result: the combined runner/ports/sandbox runtime and tests are byte-identical to reviewed `e80f5143`; focused runner+sandbox `-race` 154/0, no skips, and guides3/0. [Evidence](DONE.md#m5-host-controls-and-settings-overrides). The inherited malformed/unknown answer-line test LOW is J50, a separate follow-up after landing.

- ⏳ M.6 Pools and edges (audit 2026-10-30: the daemon already lets two readers share an inbox; `share` in `agents.json` is parsed and read by nothing; no member hostname field)
  - M.6a wire `share` to the consume call and add the member hostname field with the listing (L, daemon change, owner restarts); M.6b edge runners as `#runner/<edge>@<realm>` (M, after Q165)
  - State M.6a 2026-10-30, branch `ab-w2b/M6a` (owner chose A in `local/w2a/m6a-member-host.md`): the daemon part is built: `host` on consume (1–64 of letters, digits, `.`, `-`; a 400 names it), a per-name map of host to last read, 2 minutes, at most 32 with a `hosts_cut` flag, in memory only; the lookup's `hosts` list, `ls --table <name>`'s Hosts line, the web record page. Wired on `ab-w2b/M6a-share`: the runner and `agent-bus run` state their host on every read (`AGENT_BUS_READER_HOST` when the unit sets it, else the machine's hostname; none is sent if it fails the 1–64 `[A-Za-z0-9.-]` check), `consume --host=`, and a served agent's `share` in `agents.json` rides its reads. Two runners serve one name, sharing its messages between them, and the listing shows both hosts, through the token handed over once (`TestTwoRunnersServeOneNameSharingMessagesAndTheListingShowsBothHosts`, no new code needed); `agent-bus run`'s host send is pinned by a real run. The deploy needs a daemon restart
  - Builds: `share`; the member hostname field (daemon change); edge runners under the bus's realm
  - Hostname settled by owner, 2026-10-30 15:25, option A: optional `host` on each consume, refreshed when the read begins or ends, held in memory for 2 minutes; bounded to 32 hosts per name, never routing/authority or durable state. Validation and listing substance: [where a member says it is](runner-implementation.md#where-a-member-says-it-is). Pool members still read the pool's own inbox ([№Q159](DECISIONS.md#recorded-decisions)).
  - Acceptance: two runners serve one name, shared between them, and a listing shows both hosts; an edge runner registers as `#runner/<edge>@<realm>`
  - Mutant: Not recorded in the source.

- ⏳ M.7 Named hosts (audit 2026-10-30: the network is on or off, nothing filters hosts)
  - M.7a the mechanism is chosen and its format approved (owner question); M.7b the proxy and the no-route bypass check (L)
  - Builds: the filtering proxy for `needs` hosts
  - Acceptance: a named host is reached and another refused; bypassing the proxy has no route
  - Mutant: Not recorded in the source.

- ✅ M.10 The runner acts as its person
  - State: owner decisions 2026-10-17, moved from R1.1; built: `register` answers the token of an agent an Agent created, 0.8.141; the runner as `#runner/<person>`, creating its agents create-only, keeping their tokens and reading each one's own inbox, 0.8.142; the Linux account `<person>-ab-runner`, with №M.11, 0.8.149; a transferred or rotated agent dropped at reload, 0.8.153
  - Builds: the runner runs as a Linux account of its own, likely `$user-ab-runner`, so its secrets stay apart, but has **no bus identity of its own**: it is the person's agent `#runner/<person>`, and the person owns every agent it serves. **`register` answers with the new agent's token when, and only when, that call created the record**, so the runner creates an agent, keeps its token in its private directory and makes itself the agent's Maintainer. An agent that already exists is handed over once by its Owner. No new flag. Replaces the keyless User `agent-bus-runner`, and makes the serve-only grant ([№Q164](DECISIONS.md#recorded-decisions)) unneeded
  - Format: the `token` field on the register answer
  - Acceptance: a re-registration of an existing record answers no token; a register that created the record answers its token, which works; the runner never holds its person's token and serves no name it has no token for; removing the created-only guard lets a re-registration collect an existing agent's token, and the check fails
  - Mutant: removing the created-only guard lets a re-registration collect an existing agent's token, and the check fails

- ✅ M.11 Runner install through setup (plan rules under test with six assertion-red mutants, branch ab-w2b/T-7-m11; real-host acceptance of the system runner's verbs is pending, the owner's)
  - State: owner, 2026-10-17; built 0.8.149: `--runner install|uninstall|enable|disable --for <person> [--purge]`, the `<person>-ab-runner` account and `agent-bus-runner@.service` template, nothing installed by default; [a person's runner](../../docs/07-setup.md#a-persons-runner)
  - Builds: `agent-bus-setup` options that install, uninstall, enable and disable a person's runner: the Linux account, its home, the unit, and with №M.10 the person's `#runner/<person>`; nothing installed by default
  - Format: the option names
  - Acceptance: each verb leaves the host in its stated state and a second run changes nothing; uninstall leaves no account, unit or link behind and always keeps shared `agent.d`, including with `--purge`; disable stops and disables the unit and keeps the files; a verb for a person who is not a bus User is refused
  - Mutant: purge removing agent.d, a second run planning again, a non-User accepted, a record of another owner or kind taken over, the shared template removed beside another runner, a disable that ignores an active unit: each fails an assertion in `src/cmd/agent-bus-setup/runner_install_test.go` or `runner_authority_test.go`

- ✅ M.12 Agent code from a list of directories (owner, 2026-10-17)
  - Built: shared-first selection, bundled opt-in, source paths and reload fallback; [agent directories](../../docs/runner/configuration.md#agent-directories) owns the accepted behaviour and spellings (#ab-dir, 2026-10-30). [Evidence](DONE.md#m12-agent-directories).

- ✅ M.13 An agent says it is not configured (owner, 2026-10-18: "i agree - accept your idea")
  - Builds: an agent of `agent-bus-runner` that exits with code **78** (`EX_CONFIG`, sysexits.h) says it is not configured; the last line it wrote to stderr, if any, is the reason. The runner does not restart it and does not read its inbox any more: `status` shows `state: "unconfigured"` and the reason, and messages wait in its queue as for any agent not being served. A script agent's triggering message is answered `error: <agent> is not configured: <reason>`. Only `reload` tries it again: no timer, no retry. The bundled Slack agent exits 78 when its tokens are missing
  - Scope: `agent-bus-runner` only; the dedicated runner (`agent-bus run`) keeps its behaviour for now
  - Docs: the code and the state in [runner operations](../../docs/runner/operations.md) and [configuration](../../docs/runner/configuration.md)
  - Acceptance: a kept agent exiting 78 is not restarted and shows `unconfigured` with its reason; a script agent exiting 78 answers its caller with the error, and the next message waits unread; `reload` after a fix serves it again; any other exit code keeps today's restart and failure behaviour
  - Mutant: restart on 78, keep reading the inbox, drop the reason, or answer nothing, and a check must fail
  - Built: de0a205 (0.8.193) by Codex; OpenCode reviewed it with no P1/P2, the stderr-capture hardening followed in b514fc7 and was independently reviewed

- ⏳ M.8 Deploy over the bus
  - Builds: install, configure and remove as calls to the runner ([reaching the runner](runner-implementation.md#reaching-the-runner)); configuration write-only
  - Code sources settled by owner, 2026-10-30 15:25, A+B: a bundled agent name, or a git URL pinned to a full commit id; a branch, tag or abbreviated revision is refused. The host needs git and a route; a private repository uses a host key, never one sent in the call. Options C and D come later. The calls' bodies remain the implementation contract ([reaching the runner](runner-implementation.md#reaching-the-runner)).
  - Acceptance: an agent is installed and served with no shell on the host; reading a configuration back is refused; a caller outside the runner's ACL is refused
  - Mutants: shortened pins, URL credentials, wrong fetched HEAD, implicit replacement, old-file overlay, token loss, subdirectory layout, changing a read-only root, escaping symlinks, bundled-name removal and missing management checks each fail an owning assertion.
  - Built on `ab-w1c/M8`, awaiting review and gate: management-gated install/remove, exact pinned Git fetch with isolated credentials, checked root packages, whole-tree replacement, bundled-name removal refusal and preserved work/tokens/records. Focused, real-bus and real-Git fixtures plus assertion-red mutants are in [the evidence](DONE.md#m8-deploy-over-the-bus).
  - Follow-ups from review (LOW, 2026-10-30): (1) the explicit read-only `--agents` check in `deployRoot` is shadowed by `MkdirTemp`'s error, which carries the same "writable --agents" text: mutating only the check survives, so the operator may not see the settled "read-only agents dir" message in every case; make the check run first and assert its exact wording, and drop the shadow. Worth doing soon. (2) `-c core.hooksPath=/dev/null` on every deploy git call has no test: removing it survives; a fetch brings no hooks and `init.templateDir` is empty, so it is defence in depth, but an assertion that no hook runs would pin it

## Bundled agents

Asked for by the owner on 2026-10-17. Each is an entry of the
[R1.2 catalogue](../R1.2/services.md#the-catalogue) brought to R1 with the
runner that keeps it, except systemd, which the runner cannot keep and which
stands alone ([agents for the runner](agents.md#what-moved-and-what-r1-adds)).
The catalogue's own rules hold — the daemon applying the ACL before delivery,
and no daemon change. A row that needs a format first proposes it for approval.

- ✅ B.1 telegram — [plan and tasks](done/telegram/README.md#telegram-agent), TG.1–TG.7: built as [the Telegram agent](../../src/agents/telegram/README.md#the-telegram-agent) (src/agents/telegram), the tests in smoke's agent_telegram shard
  - Builds: `#telegram/<account>@<realm>`: calls out, and a subscription forwarding a chat to an agent, 📮 queue or 📣 pubsub; the bot token read from the record's bus secret and nowhere else; `#group` and `@user` resolved to a chat id; a default channel declared in the instance's configuration ([telegram](agents.md#telegram))
  - Format: settled 2026-10-18 as the Slack agent's methods where Telegram allows ([methods](done/telegram/README.md#methods)); the configuration's fields are proposed in TG.1
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

- ⏳ RT22 Bus setup: the Configuration of `#bus` on the ordinary edit page, daemon Owner only ([approved plan](daemon-configuration.md#approved-plan), [RD108](DECISIONS.md#recorded-decisions), [RD158](DECISIONS.md#recorded-decisions)). Daemon and edit page built; web-form realm fix: #ab-w1c, two readers. RT22 stays ⏳ until RT22b lands.
  - Approved by the owner on 2026-10-20; realm first, other settings read-only or kept in their owning interface; changing the default never renames existing records.
  - Acceptance: only the daemon Owner sees and saves the page; a new record defaults to the configured realm; an existing name never changes silently. [Acceptance map and web fix](DONE.md#rt22-bus-setup-web-acceptance).
  - RT22b: launcher and installer automatic names take the configured `#bus` realm; existing bound names never change. Assigned #ab-w1b, due 2026-11-02 18:00; a test per path, red before, plus a mutant.

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
    - Fixture leak fix (J32, `ab-w1b/T-12`): db fixtures and the private-store harnesses leave no container or run directory on any signal, a start in flight included, and write nothing to the host journal; `agent_db` and `container_image` run under a journal check; a fixture or guarded harness refuses to start a container nobody holds the `smoke-gate` lock for (`ab-w1a/J32-lock`); [issue](../issues/2026-10/2026-10-30.md#fixtures)
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

- ✅ B.25 `#git` autopull from bus channels (built 0.9.160 core `7ea886eb`, scripts 0.9.162 `666671da`, owner's-flow check in [DONE](git-autopull/DONE.md#owners-flow-end-to-end); pending, the owner's: the own-reader reinstall, the node's `autopull-cmd`/`autopull-log` records and the realty servers' switch from Redis autopull): `autopull-cmd` events, start check and operator force, `ff`/`mirror` modes, revision deploy, JSON results on `autopull-log`, success/fail scripts; the timer is removed — [design](git-autopull/README.md#two-channels), [tasks](git-autopull/TODO.md#b25-channel-driven-autopull-in-git) (owner, 2026-10-29)

- ✅ B.26 `#webhook` publishes a hosting service's push to `autopull-cmd` (built 0.9.157 `f5114e4e`; eight assertion-red mutants; `#ab-qa2` no findings; acceptance and tests in [DONE](git-autopull/DONE.md#b26-webhook-publishes-to-a-pubsub-channel))

- ⏳ B.29 Unified subscriptions: one shared `sub` implementation (Python module plus a TypeScript twin on shared JSON vectors) and one behaviour for every agent; refused sends dropped with a warning, never paused — [plan](subscriptions.md#one-behaviour) (owner, 2026-10-29). Shared Python module and vectors ✅ (0.9.171, [evidence](agent-events/DONE.md#shared-subscriptions)); TS twin ✅ (0.9.173); logwatch uses the shared Python engine ([result](DONE.md#b29-logwatch-subscriptions)); imap moved ✅ ([evidence](agent-events/DONE.md#imap-on-shared-subscriptions)); moving cron and `#at` xAB2 after B.25 scripts; Slack and Telegram migrated on `ab-w2c/B29`, pending review and landing ([evidence](DONE.md#b29-slack-and-telegram)); review `#claude/ab2-review`
  - Acceptance: the same vectors pass in Python and TypeScript; for each moved agent a refused send drops one message with one warning and the next match is delivered; the may-send check fails a send for a revoked author; no agent keeps its own subscription code; the TypeScript Port adapter omits wait on send, pinned by a real-daemon delivery check and a mutant (Slack/Telegram migration); a migrated agent imports the shared engine, proven by a check that fails if its local module shadows it (IMAP ✅: `EngineTests` fails on a local `imap/subscriptions.py`)
  - Logwatch: the J21 check fails before migration (an id is returned for `/(/`) and passes with the shared engine: `sub add <target> /(/` answers `invalid_filter` and stores nothing. Refused copies drop once with one warning, later matches deliver, and revoking author access to target or helper stops delivery; [evidence](DONE.md#b29-logwatch-subscriptions), [J21](../issues/done/2026-10/2026-10-30.md#subscriptions).
  - Logwatch IDs: non-12-hex legacy IDs move deterministically with collision checks; the new commit precedes old-key removal, and restart completes an interrupted move before serving. `sub edit` is refused with "delete it and add it again"; [checks](DONE.md#b29-logwatch-subscriptions).
  - Cron and `#at` at-most-once: a denied or vanished slot is claimed `dropped` before it is counted, so a second tick or a restored ACL in the same minute cannot send it; a legacy id's `fired.<old>.*` claims move with the entry; a legacy `unconfirmed`, `denied`, `gone` or `retrying` claim becomes `dropped` at start, never deleted, so its slot or one-shot is not sent again. Checks in `cron/test_cron.py` (Authority, Migration, legacy claims), each with a mutant.
  - Acceptance: the Slack and Telegram migration isolates a bad filter to its own subscription: a stored `/(/` ahead of a valid subscription on the same channel drops only its own copy with `filter_invalid`, and the valid one is delivered, a check that fails on today's code ([shared behaviour](../../docs/agents/subscriptions.md#the-shared-behaviour))

- ✅ B.28 `#eventlog` agent — **0.9.183–0.9.187**, `0d60d4ff`; deployed 2026-10-30 ([result](DONE.md#b28-eventlog-agent); [design](agent-events/eventlog.md#what-it-does), [tasks](agent-events/TODO.md#b28-eventlog-agent) (owner, 2026-10-29)). The web page is not built

- ⏳ B.27 Messaging agents, the [Slack and Telegram shape](../R1.2/services.md#people-and-the-world-outside): post and read, replies routed to the asking session, `sub` forwarding a conversation; one instance per account, the provider token as the record's Secret (owner, 2026-10-29). Unassigned; each needs a plan of its own, with methods and Configuration proposed for approval, before code ([plans](messaging/README.md#plans))
  - B.27a Discord: moved to R1.1 (owner, 2026-10-30: "move discord to 1.1"); the code stays on main, outside the R1 bundle and archives. [R1.1 task](../R1.1/TODO.md#discord)
  - ✅ B.27m Discord leaves the R1 bundle: `src/agents/package.sh` and setup's file list drop it, and the packager test fails if an R1 package holds a discord tree
  - B.27b Google Chat: [plan](messaging/google-chat.md#proposed-methods-for-approval), technical choices decided
  - B.27c Microsoft Teams: deferred to R2.0 until a public HTTPS address is decided, [plan](../R2.0-Future/teams.md)
  - B.27d WhatsApp: deferred to R2.0 until a public HTTPS address is decided, [plan](../R2.0-Future/whatsapp.md)
  - B.27e Facebook Messenger: deferred to R2.0 until a public HTTPS address is decided, [plan](../R2.0-Future/messenger.md)

- ⏳ B.15 Node cleanup with a lasting bound ([RD110](DECISIONS.md#recorded-decisions))
  - Observation: [report-only script](../../src/maintenance/OBSERVATION.md), run manually as an ordinary user; timestamped samples stay in ignored `tmp/b15/`. First sample captured 2026-10-30; growth and bound choices await repeated observations. No new timer, install or deletion.
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
  - Settled by `#ab-dir`, 2026-10-30: text remains the default; `status --json`, `version --json` and `ls … --json` return the existing API result without a new schema, preamble or Markdown fence. `help` stays text; `help-json` describes the options; ask `status` stays the RT23 line. Version JSON is `/identity`; status JSON is the full caller-authorized `/status` result; listing/lookup retain their current selection, visibility and fields, with `[]` for an empty list.
  - Counts stay the existing `StatusFor` node snapshot: records are active/live records of all kinds (not the caller's visible records); readers are outstanding polls including filtered waits, not unique agents or healthy workers. A busy handler need not be waiting. Uptime is this process's; queued is current depth, dropped/expired are cumulative since start. Owner-inactive detail remains absent except for the daemon Owner/active Administrators, never fabricated as zero for others.
  - `ok` means that this authenticated status request was served. No `degraded` branch until an existing observable current fault is named; zero readers, backlog or historical counters alone are not a new health score.
  - Remaining acceptance: preserve text/status asks; JSON parses directly and empty listing is `[]`; callers with different listing visibility see the same general node totals, with privileged fields still gated; fixtures distinguish all record kinds/inactive state, filtered polls/unique agents, busy handlers and current/cumulative counters. Owning mutants for format wrapping, broadened listings, incorrect counts and privileged disclosure must fail on assertions. API adapter/tests and bus guide only; no core change.
  - State 2026-10-30, branch `ab-w2b/RT24`: `status --json`, `version --json` and `ls [name] … --json` answer with the daemon's own `/status`, `/identity`, `/ls` and `/lookup` results (shared helpers, no new shape); `help` and extra arguments are refused; `help-json` declares the option and `json` as a body format. A name given to `ls --json` answers that record as `/lookup` does, a list as an array, `[]` when empty. Checks: `TestBus*JSON*` in `internal/api/daemon_agent_json_test.go`; mutants in the branch's commit.
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
  - Settled by `#ab-dir`, 2026-10-30 ([decision](DECISIONS.md#recorded-decisions)): the runner holds an ask-only reader only for a refused or unconfigured entry that has a token, and for a kept agent while no process of it is running (the reader goes when the process starts, so a running agent's `status` still reaches the script, RD118). It answers `status` from the observed state and `help`, `help-json`, `help-config`, `help-secret` and `mcp` from the manifest through the existing handlers, an unreadable manifest answering `status` only, and `unsupported: <ask>` for any other ask; it never runs the script and never takes an ordinary message. Slack answers `status: starting: <what>` from its own ask-only read (`asks: "only"` in `src/clients/typescript`) while it paginates workspace users and channels, and its ordinary messages stay queued until it serves. Telegram and the other agents get no start phase. The Python client's option is a FUTURE line. Acceptance: the ask-only reader consumes no ordinary message, the queue keeps its messages, and both readers coexist; a mutant for each fails on an assertion.
  - Q260 settled by owner, 2026-10-30 15:25, option (1): a consume parameter selects only messages whose `ask` is set; ordinary messages stay queued, and the ask-only read may coexist with the unfiltered reader under the caller's existing authority. Build after J39; pin admission, both-reader behavior and ordinary-queue retention in tests. No separate endpoint or `#bus` proxy reader. Spelled `GET /consume?asks=only` (#ab-dir, 2026-10-30): any other value is a 400 naming `asks`; only `only` is built. Built on `ab-w2b/RT25a`: core and HTTP, with mutants; the client libraries, the CLI and Slack's use of it follow
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

- ✅ RT32 (landed 0.9.131; verified after the 2026-10-30 daemon restart; optional step 6, a Docs row in the agent page's About card, is not built) Every bundled agent has its own doc page in one structure, listed on one index page, and returns the page's link (owner, 2026-10-28: "make sure to have good doc page with a list of bundled agents; every agent must have its own doc page; return this page from agent (use github link); check how we do these pages (structure)")
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

- ✅ RT33 The daemon, clients and cron, at, IMAP, Slack, Telegram and logwatch paths satisfy the author-admission contract ([RD141](DECISIONS.md#recorded-decisions)); B.29 supplies the shared send paths. Acceptance maps to existing checks; unavailable-authority gaps are covered by three adapter checks, two handler checks and eight assertion-red mutants. No agent code changed; [result and file:test mapping](done/rt33-may-send.md#acceptance-mapping).

## Distribution acceptance

Implement the [release artifacts](distribution.md#release-artifacts) and [container contract](distribution.md#container-runtime).

- ✅ T-10 One notice file, `licenses/NOTICE.md` — **0.9.189**, `0e7868ee` ([result](DONE.md#t-10-one-notice-file))
- Deferred P.4 Shell installer: moved to [FUTURE](FUTURE.md#shell-installer) by RD178. Earlier acceptance is [history](done/installer-acceptance.md#checks), not a remaining key or publication prerequisite.
  - Native aarch64 build and release evidence passed 2026-10-30 ([result](done/p3-aarch64-mac-run.md#steps)); the clean install there passed on 0.9.223 (21 checks) after J59, once setup stopped restarting units faster than systemd's start limit ([result](done/p3-aarch64-mac-run.md#steps)). This is retained evidence, not an npm publication blocker.

- ⏳ P.1 npm package
  - Contract: [npm packaging and local acceptance](distribution.md#npm). Full and client editions carry the release executables, compiled MCP runtime and its required assets, launchers, bundled runtime files, license and notices. Build stamped release archives on the tag, verify `SHA256SUMS` and manifests, then wrap them with `src/npm/pack.py` and `npm pack`.
  - Manifest inventory: the builder’s generated command `package.json` names change from `agent-bus` / `agent-bus-client` to `ai-agent-bus` / `ai-agent-bus-client`; generated platform names are `ai-agent-bus-{linux-x64,linux-arm64}` and `ai-agent-bus-client-{linux-x64,linux-arm64}`. Audit the `name`, dependency and platform-name references together and drop any published `@agent-bus/` variants; change the builder defaults in a separate implementation commit and retain the installed command `agent-bus`. Add macOS platform names only with built and tested assets. No manifest edits in this plan change.
  - The tracked `src/clients/typescript/package.json` (`@agent-bus/client`), `src/mcp/package.json` (`@agent-bus/mcp`) and `src/web/package.json` (`@agent-bus/web`) are private development manifests. Keep them unpublished; if any becomes a published artifact, rename it to an unscoped name and update its references first. There is no npm org; publishing uses the owner’s account, per the [decision](distribution.md#npm).
  - T-16 implementation: the wrapper defaults select the owner’s reserved package names while preserving the `agent-bus` command; naming/bin assertions and local installed-command acceptance cover the change. Other distribution migration remains pending.
  - ✅ P.1a Release signing retired in the workflow, verifier, checksum generator and installer fixtures; [result and acceptance](done/p1a-unsigned-release.md#result). Both native builds and checksum/manifest verification precede unsigned GitHub build-artifact publication.
  - Pending P.1b: adapt clean-install acceptance to npm and add macOS npm platform support.
  - Acceptance: install the local `npm pack` tarballs in a clean supported container with npm but no checkout, Go or bun; resolve dependencies only from those tarballs (a private loopback registry is allowed). Start the installed bus and an installed agent, complete a request/reply and an MCP tool call. Install the client-only package as a second account and prove it joins without a daemon or writes outside its prefix. Compare installed executables and MCP assets to the release manifest.
  - Mutants: omit a required executable, then an MCP runtime asset, repack and reinstall; each must fail the corresponding runtime assertion. A corrupt archive checksum and an incomplete manifest are refused before unchecked files are installed. Keep update/configuration/data and rollback checks.
  - State P.1b 2026-10-30, branch `ab-w2b/P1b`: `src/acceptance/npm-clean-install.sh` runs [`src/npm/acceptance.py`](../../src/npm/README.md#clean-install-acceptance) (the control, the omitted executable and MCP asset, the corrupt archive, altered file and unchecked file) and the container check, now with an installed agent and the client package installed by a second account without a daemon or writes outside its prefix. `pack.py` refuses a release with a file its manifest does not check, apart from the unpackaged review-settings example. Not built here: the update, configuration, data and rollback checks stay as they are, and macOS support is P.1c.
  - First real publication: both `ai-agent-bus` and `ai-agent-bus-client` are reserved under `parf73` as README-only 0.0.1 placeholders (2026-10-30); publish a version above 0.0.1. The shared `VERSION` already exceeds it; the owner’s go is still required.
  - No npm publish or npm login by any worker. Publication waits for the owner’s explicit go; no live publication is needed for acceptance.

- ✅ P.2 Ready-to-use container image: [result](DONE.md#done--r1) and [container guide](../../src/container/README.md#container-image); npm does not replace this separate artifact.

- ✅ P.5 Homepage: live 2026-10-30 from parf.dev `8b0e411`, flowcharts fixed in live `d65e4af`; [result and archived acceptance](done/homepage.md#result)

- ✅ P.3 Release evidence
  - Acceptance: Query every shipped program's version in both distributions and compare with the release source; require stamped Go build information and packaged license. Substitute an unstamped binary, alter one version or omit the license: each fails its corresponding check
  - Mutant: Substitute an unstamped binary, alter one version or omit the license: each fails its corresponding check
  - Built: each native build in `.github/workflows/release.yml` runs `src/release-verify.sh` on every full/client archive before upload; the release job requires both builds to succeed and verifies the complete unsigned checksum set before publication (P.1a). The owning test executes that exact workflow command and refuses unstamped, wrong-version and license-free artifacts, including a bad client beside a valid full archive; [result](DONE.md#p3-release-run-evidence), [release evidence](../../docs/07-setup.md#release-evidence).
  - Native aarch64 build and version evidence passed ([result](done/p3-aarch64-mac-run.md#steps)), and the clean install there passed 21 checks (0.9.224; [the PASS list](done/p3-aarch64-mac-run.md#the-passing-runs-result)); installed npm acceptance remains under P.1. Require both native builds and checksum/manifest verification before the GitHub build release or npm wrapping; no signature, owner key or signing wait. This docs-only change pushes no tag and publishes nothing.

## macOS

Plan and acceptance in [macOS](macos.md#tasks); decided in [RD166](DECISIONS.md#recorded-decisions). The owner approved starting the cheaper path on 2026-10-30 (Q221).

- ✅ MAC.1 Client and runner on macOS without a sandbox: [client and runner](macos.md#mac1-client-and-runner-on-macos); shares `src/cmd/agent-bus-runner/`, `src/internal/cgroup/` and `src/internal/sandbox/` with team 2's M.5 and DR.1
  - State 2026-10-30, branch `ab-w1b/MAC1`: built and rebased on main; on the Mac (arm64, macOS 26.5.2) the throwaway release passes I2 install, I3 version and build stamp, I4 already current, I5 tampered archive refused with nothing written, I6 full node refused, and R1 `--sandbox=on` is refused with exit 1 and no output from the script. R2 passed 2026-10-30 17:49 over the pinned-TLS path (a loopback tunnel to the node's daemon, a credential issued for the one run and revoked): the script agent `sh -c "id -un"` answered `parf`, the account that started the runner, with the sandbox off. R3 (the same as a standard non-administrator account) skipped: optional by the [plan](mac-run-list.md#d-client-install-and-runner-on-macos-mac1) ("optional, the owner decides"), and MAC.2 proved the non-root service account (`#ab-ceo`, 2026-10-30); the Mac holds 57M under `~/agent-bus-dev`, nothing of this run
- ⏳ MAC.2 Daemon with a reduced setup: code and native daemon/setup/admin acceptance verified ([result](done/mac2.md#checks-and-results)); [current setup](../../docs/07-setup.md#the-daemon).
  - Cleanup uses `sysadminctl` and `dseditgroup`, verifies absence (an already removed group succeeds) and reports named leftovers on failure; local stubs fail on the old cleanup and pass after the fix. The owner removed both roles and the CEO verified absence ([cleanup](done/mac2.md#owner-cleanup)). Remaining: full darwin archive validation with MAC.1/distribution. No more worker sudo on the Mac.
- ☐ MAC.3 Sandbox: [sandbox](macos.md#mac3-sandbox); deferred, built only if the owner asks

## Web scale follow-up from MVP

- ✅ F.13.6 follow-up — growing pages and directory reads
  - Origin: the released MVP's [measured findings](../done/R0.8-MVP/done/web-acceptance.md#findings), carried forward when R0.8 was archived; the original acceptance is complete.
  - Decision 2026-10-30 (`#ab-dir`, [RD177](DECISIONS.md#recorded-decisions)): Account and the Owner's User page paginate their owned records with the existing shared `PAGE_SIZE` (100), `?q=` search and pager. Filtering precedes slicing the records already fetched; no new daemon API.
  - Leave Groups, Activity and the `/ls` read unchanged. Revisit Groups at about 1,000 groups and the directory read at about 10,000 records; these thresholds follow the [measurements and recommendations](done/web-scale-measurements.md#recommendations).
  - Acceptance: with 5,000 owned records both pages stay below 100 KB; a record past page one is reachable through a name-narrowed `?q=` query. Dropping the slice must fail the size checks on an assertion.
  - Result: owned-record cards use the same search and pager, retain inactive records and preserve User navigation. Checks: [real-handler directory fixture](../../src/web/test/owned-records.test.ts) for both size bounds, page-two search, query/navigation preservation, inactive matches and ownership scope. Focused checks: 3/0, 79 assertions; removing the shared slice typechecks and fails both named 100 KB size assertions (about 1.53 MB each), restored green. TypeScript checking and the full web suite pass (198/0, 2,202 assertions); normal gate and non-author review remain the landing steps.

## Docs rewrite

Owner, 2026-10-30: every doc leads with why, then what you get, then real working examples, then technical details ([plan, rule, inventory and acceptance](docs-rewrite.md#the-rule)). `DW…` labels this section's rows; each batch is its own worktree and landing, no bump.

- ✅ DW0 The docs-order checker with mutants — **0.9.183**, `2cb73ea2` ([result](DONE.md#docs-rewrite-dw0))
- ✅ DW1 Agent guides, first batch: ai, db, fetch, git, git-autopull, rag, qdrant, voyage, server-info, systemd — **0.9.186**, `777b9075` ([result](DONE.md#docs-rewrite-dw1))
- ✅ DW2 Agent guides: smtp, sms, webhook, tmux-sessions, bus, at, README — **0.9.186**, `0cafa5f0` ([result](DONE.md#docs-rewrite-dw2))
- ✅ DW3 Agent guides: imap, logwatch, slack, telegram, cron — **0.9.187**, `cdeb04a7` ([result](DONE.md#docs-rewrite-dw3))
- ✅ DW4 Agent guides: debug and eventlog — **0.9.187**, `41b313eb` ([result](DONE.md#docs-rewrite-dw4))
- ✅ DW5 `docs/00`-`05` — **0.9.189**, `74bc3646` ([result](DONE.md#docs-rewrite-dw5))
- ✅ DW6 `docs/06`-`09` except setup — **0.9.189**, `d3f2ab22` ([result](DONE.md#docs-rewrite-dw6))
- ⏳ DW7 `docs/07-setup.md`; starts after distribution (T-6) lands
- ✅ DW8 `docs/runner/*` and `docs/user/*` — **0.9.190**, `cded895b` ([result](DONE.md#docs-rewrite-dw8))
- ✅ DW9 Agent manifests — **0.9.191**, `99b1f94f`; Discord **0.9.206**, `4e74e3b6` ([result](DONE.md#docs-rewrite-dw9))
- ✅ DW10 Web page intros and CLI help text — **0.9.189**, `79f03e08` ([result](DONE.md#docs-rewrite-dw10))
- ⏳ DW11 Client READMEs and `src/*.md`; RT8 has landed
  - State 2026-10-30, branch `ab-w2b/DW11`: `src/README.md`, `INSTALL.md`, `MODULES.md`, the client hub, the Go, Java, Python, Rust and TypeScript READMEs, the examples README and its five example pages follow the order and are in the checker's ENFORCED list. Examples ran on disposable nodes with private data directories: the six-language examples harness, one snippet per language client, the three-terminal sequence and the first-agent example; `INSTALL.md`'s install and update commands are root steps and were not run, only its read-only checks (`agent-bus status`, `systemctl is-active`). Left: `src/clients/php/README.md`, after J43 lands
  - Acceptance for all rows: each page passes the checker and the first-20% check, every example was run on a real node, links and anchors resolve, built/pending status is unchanged
