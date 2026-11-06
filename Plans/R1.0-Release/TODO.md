# TODO R1

## AI-assisted installation and agent setup

- ✅ T-33 AI Markdown workflow for installation, configuration and updates
  - State: Markdown entry point written at [AI-SETUP.md](../../AI-SETUP.md#how-to-start); documentation and installed-node inspection checked. The short Ubuntu RC2 workflow check passes 32/32 and preserves inventory/Configuration/KV on second-run inspection; existing five-system Linux RC evidence remains valid. Real provider setup requires designated accounts and authorization.
  - Home: [AI-assisted setup](ai-setup.md#markdown-workflow).
  - Acceptance: on a clean test system, an AI follows the Markdown entry point and does most installation itself; it asks for needed choices/account input and helps configure email and Telegram agents, then demonstrates the selected agents working.
  - Existing installation: offer tuning after inspection; never reinstall or update automatically. Read the main docs and selected guides before answering or changing an agent.
  - Acceptance: a second run and a requested update preserve existing setup and give a truthful completion summary. External account checks use designated accounts with user authorization, or remain explicitly pending.

- T-34 Web Face AI chat moved to [R1.1](../R1.1/TODO.md#web-face-ai-chat) by the owner, 2026-11-03.

## Clean R1 inputs

- ✅ [Prerelease compatibility removal](legacy-removal.md#scope) is R1 work, not a patch-release deferral.
  - One canonical CLI, wire, persisted-state and package form; no prerelease converters, retired aliases or silent field repairs.
  - Current-format clean installation, explicit updates and interrupted-operation recovery remain supported.
  - Implementation, independent review and mutation evidence are recorded in the [integration review](../issues/2026-11/2026-11-04.md). [Normal verification passed](legacy-removal.md#implementation-and-checks).

## Objective

Migration labels: `RT` identifies the original unnumbered work rows; `RBK`
identifies the backup acceptance paragraph. Existing M/B/P wave IDs are unchanged.

Prepare the [proposed scope](README.md#scope). Started ahead of the release on
the 0.8 line; R1 releases as 1.0.0 ([decision](DECISIONS.md#recorded-decisions)). Item glyphs follow
[plan markup](../MARKUP.md#glyphs); built results are in [DONE](DONE.md).

- ✅ RT1 [Shared locks](done/locks.md#shared-locks)
  - State: built in 0.8.66

- ✅ RT2 [Resource records](done/resources.md#resource-records)
  - State: built in 0.8.70

- ✅ RT3 [Key-value store](done/kv.md#per-record-storage)
  - State: built in 0.8.77

- ✅ RT4 [Roles reach Agents](../../docs/03-records-agent.md#roles): transport, generated `owner` and `maintainer`, supported list
  - State: built in 0.8.97–0.8.99

- ✅ RT5 [Client libraries](done/client-libraries.md#scope): Python, TypeScript, Go, Rust, Java and PHP, with native examples
  - State: built in 0.8.100
  - Evidence: Built in 0.8.100, all six with examples and slow-gate checks; conformance-first protocol specification is settled as [№RD90](DECISIONS.md#recorded-decisions), implementation in №RT8

- ✅ RT6 [Assigned roles](../../docs/03-records-agent.md#roles) in allow lists
  - State: built in 0.8.110; [checks](done/assigned-roles.md#verification)

- ✅ RT7 [Record tags](tags.md#classification-and-filtering)
  - State: built 0.8.143; [verification](done/record-tags.md#verification); multiple-tag matching is built ([№RTA1](#record-tags-acceptance))

- ✅ RT8 A shared executable protocol conformance suite for the clients ([result](DONE.md#rt8-shared-client-conformance))
  - Home: [protocol conformance](../../src/clients/README.md#protocol-conformance), [№RD90](DECISIONS.md#recorded-decisions)

- ✅ RT9 [Agent method metadata](done/method-metadata.md#method-metadata)
  - Format: settled with the owner on 2026-10-19 ([settled so far](done/method-metadata.md#settled-so-far)): typed fields in `agent.json`, commented `env.dist`, and the `help`, `help-config`, `help-secret` and `help-json` asks; MCP generation is [R1.2](../R1.2/TODO.md#dependencies)
  - ✅ RT9.1 The runner and `agent-bus run` read the new `agent.json` fields strictly and answer `help`, `help-config`, `help-secret` and `help-json` themselves for packaged agents, without running the script; the CLI and MCP face send the three new asks (`agent-bus call <to> --help-config`, `--help-secret`, `--help-json`). Acceptance: a packaged fixture with no help code answers all four from its files; `help-secret` names keys and comments and never a value (sentinel); an unknown field is refused by name; an enum value outside the list is shown. Mutants: the script run for a help ask; a secret value in `help-secret`
  - ✅ RT9.3 The web face shows it all (owner, 2026-10-19: "once done, the web must show all this data"): an agent's record page gets an "About this agent" card (description, version, license, author, homepage), a Methods table (name, arguments with types and enums, returns, role, destructive), and the Configuration editor shows each documented key's type, help and example beside the JSON, warning about undocumented keys. The Secret field lists the expected keys from `env.dist` with their comments and which are required, never a value. The web gets it through the same asks, or from the runner for its packaged agents. Acceptance: a fixture agent's page shows every field; a secret sentinel never appears; an agent answering only `--help` shows that text and no tables. Mutants: the secret value rendered; an enum dropped
  - ✅ RT9.2 Every bundled agent's `agent.json` and `env.dist` carry the metadata (methods, args, Configuration keys, description, license, author, homepage) and their hand-written help code is removed where RT9.1 covers it; the guides link `--help-config`, `--help-secret` and `--help-json`. Acceptance: each agent's `--help` matches its real methods (a parser check per agent); removing a method from the code without its entry fails the check
  - ✅ RT9.4 [Usable Methods cards](../../src/web/web-face/records.md#method-rows), built in 0.9.44: native disclosure rows, signatures, typed args/enums, CLI/JSON examples from declared `body_formats`, GET filter, meaningful fields and compact settings. RT9.4b reports only role-hidden counts/needed roles. Phone-width/no-JS browser check passed; help-listed and hidden-count mutants compiled and failed assertions under `--slow`; restored full gate 1164/0. A call-execution form remains later.

- ✅ RT10 [Runner](runner.md#runner)
  - Details: decided; every [wave](#runner-waves) of R1 is built (M.6b, M.7 and RT19 moved to R1.1, Q299)
  - State: one-message serving built 0.8.123; M.1, M.2, M.4, M.6a and M.9–M.13 are ✅ in [runner waves](#runner-waves)
  - Depends: [method metadata](done/method-metadata.md#method-metadata); the pool hostname field, now R1 ([where a member says it is](runner-implementation.md#where-a-member-says-it-is))

- ✅ RT11 — RW4/RW8 moved to [R1.1](../R1.1/TODO.md#runner-web-additions), low priority, by the owner ([Q311](DECISIONS.md#recorded-decisions)). [Runners section in the web face](runner/web-face.md#goal) — status, per-agent controls, host files and instances are built, and RW18–RW27 are [met](runner/web-face.md#acceptance-checklist); while [RW10–RW13 and RW15–RW17 move to R1.1; RW14 is built, 0.8.211](../R1.1/TODO.md#runner-web-additions) (Q299)
  - State: the information page and per-agent management page are separate; denied management shows Access denied
  - Home: [Minimum and acceptance checklist](runner/web-face.md#minimum-checklist)

- ✅ RT12 [Agents for the runner](agents.md#what-moved-and-what-r1-adds): telegram, systemd, logwatch
  - Depends: Telegram and logwatch: the runner keeping an agent it did not write (№M.1 and №M.2, built; №M.11 for the install) and the kept process form for state between messages (№M.3, №M.9). Systemd needs none of them — it is a root unit of its own running `agent-bus run`
  - Settled by: owner, 2026-10-17
  - Home: [tasks](#bundled-agents)

- ✅ RT29 Record export and import (0.9.108)
  - Built: Owner-only encrypted permanent records, private KV and credentials, explicit selection/conflict/realm choices, atomic import, public matching backup-info, and independent bounded backup storage; [current contract](../../docs/administration.md#encrypted-record-backups).
  - Accepted on disposable daemons: real age/CLI/SQLite rebuild, old runner and standalone token clients after restart, wrong key and sidecar refusal, whole-transaction rollback, Owner doors and explicit conflicts; sixteen owning mutants and normal1187/0. No live export/import run.

- ⏳ RT14 [Installable distribution](distribution.md#release-artifacts): P.1–P.3
  - Remaining: the first npm publish at the 1.0.0 release (runbook); macOS ships (`c6b8f8ec`).
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

Plan for discussion of the note above: [shorter prompts](short-prompts.md#proposal), settled choices [Q183–Q184](DECISIONS.md#recorded-decisions); №Q182 is settled in the [receipt rules](../../docs/04-messaging.md#receipts).

> **Serg Parf:** Define "official" method description syntax for agents; AI prompts to generate one; try to use official standard

> **Serg Parf:** root level runner?

- ✅ RT20 [A may-send dry run](../../docs/09-daemon-api.md#checking-permission-to-send)
  - State: built in 0.9.41; CLI `agent-bus may-send <name>`, shared target-admission function with Send; no queue, refusal counter or activity changes. The caller asks only for itself; Send keeps its ACL `403`, including invisible targets.
  - Verification: API parity for active/allowed, ACL/Personal refused, invisible, inactive, absent, external and User-inbox cases; CLI success/refusal; no on-behalf input. Enqueue and divergent-answer mutants compiled and failed assertions under `--slow go_checks`; restored full gate 1160/0.

- ✅ RT21 A client calling itself gets its handler's answer, never its own request (`3d191597`)
  - Found by Codex #3 in B.13, 2026-10-19: the Python client's `Bus.call` to its own name consumes the request it just sent (the same inbox, topic and tag) as the correlated answer, before the handler runs. All six clients had it
  - Built on `ab-w2a/RT21`, with #ab-w2c's RT8 Python, TypeScript and Go slices: an unmarked message carrying the call's own message ID is work for a free serving worker, drained like other taken work; PHP, serving one message at a time, runs its handler in the call and refuses outside `serve()`. Shared `self-call.json` runs in all six adapters, direct and through another reply inbox; red on the old code in each
  - Mutants, 2026-10-30: the own-request check off in each client compiled and failed its self-call checks on assertions (Python 5, TypeScript 4, Go 3, Rust 2, Java 4, PHP 5)

- ✅ J27 Lock waiters are served in arrival order ([issue](../issues/2026-10/2026-10-30.md#locks)) — **0.9.185**, `de98f55d`; deployed 2026-10-30 ([result](DONE.md#j27-lock-waiters-in-arrival-order))

- RT19 Moved to [R1.1](../R1.1/TODO.md#runner-extensions) by owner decision (Q299), 2026-11-01.

> **Serg Parf:** git agent; ~ autopull / pull on demand,; status, more - come with ideas

## Next step

Built on 0.9: assigned roles, method metadata, the runner waves, the bundled
agents, distribution packaging and its acceptance, and the protocol freeze.
T-24's last step, S2v, landed as `84f3534a`, 0.9.284; agent events
([RT34](#rt34-agent-events)) have also landed.
Still open for R1, besides what the rows below say: the first npm publication
at the release (P.1, [Q300](DECISIONS.md#recorded-decisions)). B.31 moved to
[R1.1](../R1.1/TODO.md#imap-large-transfer). RT11 is closed: RW4/RW8 moved to
[R1.1](../R1.1/TODO.md#runner-web-additions) ([Q311](DECISIONS.md#recorded-decisions)).
Federation is [R1.2](../R1.2/federation.md#chaining); its R1.1 prerequisites
no longer block this release.

## Dependencies

Repeated work has one editable item under Objective, with its dependency
cells retained there. Distinctly named work keeps its own item here:

- ✅ RT16 [Record-defined roles](../../docs/03-records-agent.md#roles) — no code of its own: the existing allow list and nested membership already do it (contract `2727d193`)
  - Evidence: Built using the existing allow list and nested membership; [current contract](../../docs/03-records-agent.md#roles)

- [№RT15](#objective) — Federation moved to [R1.2](../R1.2/TODO.md#federation)
- [№RT10](#objective) — Runner
- [№RT12](#objective) — [Agents for the runner](agents.md#dependencies)
- [№RT9](#objective) — [Agent method metadata](done/method-metadata.md#method-metadata)
- [№RT5](#objective) — [Client libraries](done/client-libraries.md#scope)

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
  - Acceptance: an agent cannot see a sibling's processes, read a sibling's work directory or env, connect to the runner's or another account's socket, write outside its work directory, or exceed its memory; on a host without `bwrap` it is refused; each removed limit fails its check. What is installed under `/usr`, a sibling's package under `/usr/local` included, is bound read-only and stays readable ([isolation today](../../docs/runner/security.md#isolation-today)); narrowing that is [M.14](../R1.1/TODO.md#runner-hardening)
  - Mutant: each removed limit fails its check

- ✅ DR.1 The dedicated runner applies the agent's bus secret — **0.9.174**, `70fcfa4a` ([result](DONE.md#dr1-the-dedicated-runner-applies-the-agents-bus-secret))

- ✅ M.5 The host's controls
  - M.5a dependency order/cycle refusal was already built. M.5b reload SIGHUP and drain use M.3; the sibling test registers a second agent and proves the untouched keeper gets no signal. M.5c shipped JSON overrides and their bus read/write/delete calls are built on `ab-w2c/M5`, stacked after M.3.
  - A settings patch is an object naming a file the agent ships; agent then instance JSON Merge Patches are mounted read-only by bubblewrap. Null patches, unshipped files and sandbox-off overrides are refused by file name. The digest includes the override so a changed kept agent restarts after drain; the author file stays intact.
  - Existing enable/disable/start/stop/restart and host-file controls remain; backup moved to R1.1. The current contract is [runner operations](../../docs/runner/operations.md#the-hosts-files-over-the-bus) and [settings overrides](../../docs/runner/configuration.md#overriding-a-settings-file).
  - Result: the combined runner/ports/sandbox runtime and tests are byte-identical to reviewed `e80f5143`; focused runner+sandbox `-race` 154/0, no skips, and guides3/0. [Evidence](DONE.md#m5-host-controls-and-settings-overrides). The inherited malformed/unknown answer-line test LOW is J50, a separate follow-up after landing.

- ✅ M.6 Pools and edges (M.6a `9871870d`, docs `553faa81`) (audit 2026-10-30: the daemon already lets two readers share an inbox; `share` in `agents.json` is parsed and read by nothing; no member hostname field)
  - M.6a wire `share` to the consume call and add the member hostname field with the listing (L, daemon change, owner restarts); M.6b moved to [R1.1](../R1.1/TODO.md#runner-extensions) (Q299)
  - State M.6a 2026-10-30, branch `ab-w2b/M6a` (owner chose A in `local/w2a/m6a-member-host.md`): the daemon part is built: `host` on consume (1–64 of letters, digits, `.`, `-`; a 400 names it), a per-name map of host to last read, 2 minutes, at most 32 with a `hosts_cut` flag, in memory only; the lookup's `hosts` list, `ls --table <name>`'s Hosts line, the web record page. Wired on `ab-w2b/M6a-share`: the runner and `agent-bus run` state their host on every read (`AGENT_BUS_READER_HOST` when the unit sets it, else the machine's hostname; none is sent if it fails the 1–64 `[A-Za-z0-9.-]` check), `consume --host=`, and a served agent's `share` in `agents.json` rides its reads. Two runners serve one name, sharing its messages between them, and the listing shows both hosts, through the token handed over once (`TestTwoRunnersServeOneNameSharingMessagesAndTheListingShowsBothHosts`, no new code needed); `agent-bus run`'s host send is pinned by a real run. Landed and deployed: two runners serving one name is checked by `9871870d`
  - Builds: `share`; the member hostname field (daemon change)
  - Hostname settled by owner, 2026-10-30 15:25, option A: optional `host` on each consume, refreshed when the read begins or ends, held in memory for 2 minutes; bounded to 32 hosts per name, never routing/authority or durable state. Validation and listing substance: [where a member says it is](runner-implementation.md#where-a-member-says-it-is). Pool members still read the pool's own inbox ([№Q159](DECISIONS.md#recorded-decisions)).
  - Acceptance: two runners serve one name, shared between them, and a listing shows both hosts
  - Mutant: built; mutant not recorded (the two-runner test pins the behaviour).

- M.7 Moved to [R1.1](../R1.1/TODO.md#runner-extensions) by owner decision (Q299), 2026-11-01.


- ✅ M.10 The runner acts as its person
  - State: owner decisions 2026-10-17, moved from R1.1; built: `register` answers the token of an agent an Agent created, 0.8.141; the runner as `#runner/<person>`, creating its agents create-only, keeping their tokens and reading each one's own inbox, 0.8.142; the Linux account `<person>-ab-runner`, with №M.11, 0.8.149; a transferred or rotated agent dropped at reload, 0.8.153
  - Builds: the runner runs as a Linux account of its own, likely `$user-ab-runner`, so its secrets stay apart, but has **no bus identity of its own**: it is the person's agent `#runner/<person>`, and the person owns every agent it serves. **`register` answers with the new agent's token when, and only when, that call created the record**, so the runner creates an agent, keeps its token in its private directory and makes itself the agent's Maintainer. An agent that already exists is handed over once by its Owner. No new flag. Replaces the keyless User `agent-bus-runner`, and makes the serve-only grant ([№Q164](DECISIONS.md#recorded-decisions)) unneeded
  - Format: the `token` field on the register answer
  - Acceptance: a re-registration of an existing record answers no token; a register that created the record answers its token, which works; the runner never holds its person's token and serves no name it has no token for; removing the created-only guard lets a re-registration collect an existing agent's token, and the check fails
  - Mutant: removing the created-only guard lets a re-registration collect an existing agent's token, and the check fails

- ✅ M.11 Runner install through setup (plan rules under test with six assertion-red mutants, branch ab-w2b/T-7-m11; real-host acceptance of every verb passed 2026-11-01 on 0.9.262, 38 checks/0 failures, on an Arch container only, with no real agent served, no Debian host, the bus record and `runner.env` mode read back but not asserted, and `enable` before install not exercised: [result](done/m11.md#what-each-verb-left))
  - State: owner, 2026-10-17; built 0.8.149: `--runner install|uninstall|enable|disable --for <person> [--purge]`, the `<person>-ab-runner` account and `agent-bus-runner@.service` template, nothing installed by default; [a person's runner](../../docs/runner/setup.md#a-persons-runner)
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

- ✅ M.8 Deploy over the bus: Implementation `6945c1a2`, landed through final gate/bump commit `e952c374` (0.9.228), with M.8b `20769d8f` (0.9.233) and M.8c `44497ffe` (no bump). [Acceptance and original task detail](done/m8-deploy-close-out.md#acceptance-map); [current contract](../../docs/runner/operations.md#deploying-an-agent).

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
  - Format: [B.2 calls and delivered tiers](../../docs/agents/systemd.md#who-may-use-it); no installed unit-set file; `agent.json` supplies the `systemd` discovery tag.
  - Acceptance: no-role list/timers/failed/status, operate start/restart/reload/logs, admin stop/enable/disable; an unconfigured service or timer is listed, statused and started by operate; every tier refuses invalid names; install twice leaves one unit and one name, removal leaves no unit, token or record. Running as a non-root account leaves list/status working and every acting verb returns systemctl's refusal as an error, never silent success.
  - Mutants: remove role enforcement; trust a role claimed in the body; suppress errors from the non-root unit's acting verbs. NoNewPrivileges is not a privilege-loss mutant: it does not drop existing root credentials.

- ✅ B.9 systemd dedicated instances (`b4b3fd92`)
  - Builds: a `units` entry in the instance's Configuration, a comma-separated list of exact unit names, which limits that instance to those units ([dedicated instances](../../docs/agents/systemd.md#dedicated-instances)). Owner, 2026-10-19: "support "config" entry; with comma separated list of units (timers, ... whatever systemd manages); this way we can create dedicated instances of specific service status/restarters for ordinary users"
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

- ✅ B.4 Choose the next set (a planning row, no build) — the owner chose mail, then webhook, cron and fetch, 2026-10-27 ([RD125](DECISIONS.md#recorded-decisions)); rows B.5, B.6, B.16–B.18

- ✅ B.5 IMAP client — [guide](../../docs/agents/imap.md#what-it-does), [verification and disabled deployment](done/mail.md#b5-imap)
  - Plain agent and three owner-requested instances installed disabled, sandboxed and owner-only; publication target unset. Manager-controlled live enable/small sync remains separate from fixture proof.

- ✅ B.6 SMTP client — [guide](../../docs/agents/smtp.md#what-it-does), [app-password and XOAUTH2 evidence](done/mail.md#b6-xoauth2)
  - Installed disabled, without an account. The owner-approved real Gmail read/send check remains pending; fixture proof does not claim real-provider verification.

- ✅ B.7 server-info — built: [agent and verification](../../src/agents/standalone/server-info/README.md#verification), shared standalone starter for №B.2
  - Builds: `#server-info@<host>`, **standalone like systemd** (owner, 2026-10-19: "we need separate starter same as for systemd one"). Its own install script writes a unit running `agent-bus run` with the sandbox off, reaching the bus over IP with a token from the unit's `EnvironmentFile`. It answers the [catalogue's sections](../R1.2/services.md#server-info): `uname -a`, `ip a`, `ip r`, `free`, `lsblk` with models and serials plus `df`, `uptime`, load average, and the accepted additions: OS release, CPU, inodes, pressure, failed units, time sync, DNS, listening ports, users, virtualization, and root-only disk health and sensors
  - Depends: none in the runner; the systemd agent's install script (№B.2) is the pattern, shared rather than copied
  - Format: [section names, text and JSON calls](../../docs/agents/server-info.md#calls)
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
  - [Install-time timers and status](../../docs/agents/git.md#autopull): an optional `--autopull ab=5m` for each configured repository with deploy or commit access, ff-only under the same lock; dirty or diverged trees are never forced, errors are logged and returned by `autopull ab`. No timer by default and no service restart. Owner revised the initial Configuration proposal to install-time systemd timers on 2026-10-19.
  - Acceptance: a real private timer brings a fixture upstream commit within a bounded interval; a dirty tree remains intact and its failure is returned; removal removes the timer. Mutants: timer not enabled; dirty tree reset/cleaned.

- ✅ B.10 Git roots and access levels in Configuration (`b7ec1957`)
  - Builds: [repositories and access levels](../../docs/agents/git.md#repositories-and-access-levels): `repos` with path and `access` (`read`, `deploy`, `commit`, cumulative) in the Configuration, replacing `--repo`; roles `deploy` and `commit` replace `operate` and `admin`; the effective level is the lower of the repository cap and the caller's role. Owner, 2026-10-19: "in config - list git roots and allowed access level … even with best role user can not go over repo cap"
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

- ✅ RT22 Bus setup: the Configuration of `#bus` on the ordinary edit page, daemon Owner only ([approved plan](daemon-configuration.md#approved-plan), [RD108](DECISIONS.md#recorded-decisions), [RD158](DECISIONS.md#recorded-decisions)). Daemon, edit page, web-form realm default and launcher/installer automatic naming built; [automatic naming result](DONE.md#rt22b-automatic-names).
  - Approved by the owner on 2026-10-20; realm first, other settings read-only or kept in their owning interface; changing the default never renames existing records.
  - Acceptance: only the daemon Owner sees and saves the page; a new record defaults to the configured realm; an existing name never changes silently. [Acceptance map and web fix](DONE.md#rt22-bus-setup-web-acceptance).
  - RT22b: launchers (`ab-claude`, `ab-codex`, `ab-opencode`) and the runner installer (`#runner/<person>[-user]@<realm>`) take the realm from `status`; a bound session resumes in its own realm even when the bus names none; an already-registered runner record is kept under any realm, two refuse the install; a bus with no realm refuses a new automatic name. Tests: `launchers/local.test.ts`, `rename-smoke.ts`, `cmd/agent-bus-setup/realm_test.go`

- ✅ B.19 voyage — built as RAG R.1, 0.9.123; [result, gate and review](rag/DONE.md#r1-voyage-proxy). The manager/owner makes the first live provider call.

- ✅ B.20 Database proxy, built in 0.9.129 (commit 50f7df42; gate --normal 1208/0; deployed to `agent.d/db` and disabled in the runner inventory until the owner gives a database; [guide](../../src/agents/db/guide.md#what-it-does); fixtures MariaDB, PostgreSQL, ClickHouse; 14 mutants each failing on an assertion; MySQL itself unproven, MariaDB tested) (owner, 2026-10-28: "add DB proxy with special role (read) - allow selects only")
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

- ✅ B.30 IMAP full mail automation — **approved 2026-10-31 12:05**, all five steps landed on 2026-10-31: (1) `785fa93c` 0.9.244, completed by `871c71c3` 0.9.245; (2) `e2d01c52` 0.9.246; (3) `386d26f2` 0.9.247; (4) `569ca838` 0.9.248; (5) `ddbf2b61` 0.9.249. [Call list](imap-folders.md#help-text), [result and evidence](DONE.md#b30-status-close-out); [current guide](../../docs/agents/imap.md#what-it-does). Q270 settled (owner: one shared cache budget, [decision](DECISIONS.md#recorded-decisions)). The later `--big` → `--bigfile` rename came with [B.32](#bundled-agents) (T-20 2c, T-22).
  - Steps: (1) `folders`, `--folder` on sync, list, search, read, mark and publish, per-folder cache; (2) mark/flag, server-side search, headers, parts; (3) copy, move, trash, delete, compact_trash, `allow_destructive`; (4) folder create/rename/delete, append; (5) `sub add --folder`, per-folder status, IDLE
  - Builds: code in `src/agents/imap`, guide `docs/agents/imap.md`
  - Depends: B.5, B.21, B.24
  - Acceptance: [the plan's acceptance](imap-folders.md#acceptance-for-the-todo-item)
  - Mutants: [the plan's list](imap-folders.md#acceptance-for-the-todo-item)

- B.31 Moved to [R1.1](../R1.1/TODO.md#imap-large-transfer); remains parked
  per the owner's 2026-11-01 decision. [Plan](../R1.1/imap-large-transfer.md#help-text)
  and [questions](../R1.1/QUESTIONS.md#imap-large-transfer) moved with it.

- ✅ B.32 Bigfiles (formerly big keys; renamed in T-22) — **approved 2026-11-01 11:05**, built in stages: 2a `90dddc07` 0.9.252 (API, memory holder, authority); 2b `36d43221` 0.9.253 (file holder, limits, expiry, 503 when the directory fails); 2d `5d259db5` 0.9.258 (early refusals drained); 2c `1a95ec19` 0.9.260 (IMAP `part`/`append`); T-22 `07a3422c` 0.9.262 (the rename, `bigfile/` directory); T-23 `516fae57` 0.9.263 (index in the database). [Plan](bigfile.md#use-it), [result and evidence](DONE.md#b32-status-close-out), [guide](../../docs/03-records-kv.md#bigfiles); [decision](DECISIONS.md#recorded-decisions). The Rust, PHP and Java clients do not carry bigfile (T-24).
  - Builds: a bus store of named big values in memory or a file, with `bigfile put|get|ls|rm`; Maintainers upload and download, allow-list principals download only; IMAP `part --bigfile` and `append --bigfile` use it
  - Depends: the key-value store; B.30 steps 2 and 4 for the IMAP part
  - Acceptance: [the plan's acceptance](bigfile.md#acceptance)
  - Mutants: [the plan's list](bigfile.md#acceptance)

- ✅ T-27 Per-person files folder (Q303) — setup makes `/var/lib/agent-bus/users/<person>/files` for each person's user-account runner: theirs, 0700, `users/` root 0755; idempotent; a link, a non-directory or another owner is refused, a looser mode is set to 0700 and said; contents untouched. [Setup guide](../../docs/runner/setup.md#a-persons-user-account-runner), [bigfiles](../../docs/03-records-kv.md#bigfiles); launchers do not default to it
  - Evidence: `TestPersonFiles*` in `src/cmd/agent-bus-setup/person_files_test.go` (create, rerun, hand-made folder, tighten, link at each level, file in place, wrong owner, link at the state root); four mutants (follow links, no owner check, no tighten, link at the root followed) each failed on an assertion; the whole `agent-bus-setup` package passes; macOS has no per-person runner, so none there


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

- ✅ B.29 Unified subscriptions: All planned adapters are landed, including cron and `#at` in `823e17c0` (0.9.222); B.29b `9ca154bd` (no bump) completes legacy `gone` claim restart/drop/no-replay coverage. [Acceptance and original task detail](done/b29-subscriptions-close-out.md#acceptance-map); [current contract](../../docs/agents/subscriptions.md#the-shared-behaviour).

- ✅ B.28 `#eventlog` agent — **0.9.183–0.9.187**, `0d60d4ff`; deployed 2026-10-30 ([result](DONE.md#b28-eventlog-agent); [design](agent-events/eventlog.md#what-it-does), [tasks](agent-events/TODO.md#b28-eventlog-agent) (owner, 2026-10-29)). The web page is not built

- ✅ B.27 Messaging agents, the [Slack and Telegram shape](../R1.2/services.md#people-and-the-world-outside): post and read, replies routed to the asking session, `sub` forwarding a conversation; one instance per account, the provider token as the record's Secret (owner, 2026-10-29). Built: Slack (`62d90c9e`, [guide](../../docs/agents/slack.md#what-it-does)) and Telegram (B.1, [guide](../../docs/agents/telegram.md#what-it-does)); every other service needs a plan of its own, with methods and Configuration proposed for approval, before code ([plans](messaging/README.md#plans))
  - B.27a Discord: moved to R1.1 (owner, 2026-10-30: "move discord to 1.1"); the code stays on main, outside the R1 bundle and archives. [R1.1 task](../R1.2/TODO.md#discord)
  - ✅ B.27m Discord leaves the R1 bundle: `src/agents/package.sh` and setup's file list drop it, and the packager test fails if an R1 package holds a discord tree
  - B.27b Google Chat: moved to [R1.2](../R1.2/TODO.md#messaging-agents) (owner, 2026-11-01: a corporate account is needed); [plan](../R1.2/google-chat.md#proposed-methods-for-approval)
  - B.27c Microsoft Teams: deferred to R2.0 until a public HTTPS address is decided, [plan](../R2.0-Future/teams.md)
  - B.27d WhatsApp: deferred to R2.0 until a public HTTPS address is decided, [plan](../R2.0-Future/whatsapp.md)
  - B.27e Facebook Messenger: deferred to R2.0 until a public HTTPS address is decided, [plan](../R2.0-Future/messenger.md)

- B.15 Moved to [R1.1](../R1.1/TODO.md#node-cleanup) by owner decision (Q299), 2026-11-01.

- RT23 Moved to [R1.1](../R1.1/TODO.md#agent-capabilities) by owner decision (Q299), 2026-11-01.

- ✅ RT24 The bus on the bus: Landed as `bfeb5f58` (0.9.224): the settled read-only help/status/version/ls set is complete; AI CLI `ab_send` with ask `help` has component coverage only, with no end-to-end test. Management methods remain future scope. [Acceptance and original task detail](done/rt24-bus-close-out.md#acceptance-map); [current contract](../../docs/agents/bus.md#what-it-does).

- ✅ RT25 Ask-first dispatch and initialization: Initial ask-first dispatch (0.9.101), RT25a `7b03df8b` (0.9.236) and RT25b `0430a095` (0.9.237) are landed. [Acceptance and original task detail](done/rt25-asks-close-out.md#acceptance-map); [current contract](../../docs/agents/authoring.md#checklist-writing-an-agents-description-and-help).

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

- ✅ RT31 A person's user-account runner — implementation `c952c241` (0.9.149), state-home handover `9905f3ff` (0.9.151), live close-out 2026-11-01 on 0.9.262. Both tmux instances are served by `#runner/parf-user@parf.us`; standalone unit files are removed. [Acceptance, live observations and baseline limits](done/rt31-user-account-runner.md#acceptance-map); [current contract](../../docs/runner/setup.md#a-persons-user-account-runner).

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

## T-24 Protocol freeze

- ✅ T-24 Protocol freeze — owner GO (Q296); team 1 owns the item, team 2 updates S2. Steps: S1 ✅ `0.9.264` (identity `protocol`/`protocol_min`, refusal `kind`, 404 `no_route`, aliases removed); C1 ✅ `0.9.266` `bb897701` (clients check the version and keep the kind); D1 ✅ `0.9.272` `c1a89aa0` ([contract and stability policy](../../docs/09-daemon-api.md#protocol-version)); V1 ✅ `0.9.272` `c1a89aa0` ([vectors](../../src/testdata/client-conformance/README.md#state-identity-and-refusal-vectors) for kv, locks, bigfile, identity and refusals through the Go, Python and TypeScript clients, and the route coverage check; 28 mutants failed on assertions; the Rust, PHP and Java adapters are team 2's, S2v); S2 ✅ `0.9.269` `f7c4f39f` (the Rust, PHP and Java clients carry the protocol check and refusal kinds); S2v ✅ (the Rust, PHP and Java adapters run V1's kv, lock, identity and refusal vectors; they found and fixed three client differences, see [done](DONE.md#t-24-s2v-the-rust-php-and-java-clients-run-the-shared-vectors))
  - Depends: S1, C1, S2
  - Acceptance: V1's shared vectors in `src/testdata/client-conformance` are green on all six client adapters, and the route coverage check fails when a registered route has no row in [the route table](../../docs/09-daemon-api.md#every-route-and-its-class)

## 1.0 final review

Final Fable review of T0 `af1c7e06` ([issues](../issues/2026-11/2026-11-02.md), `c6e5b4c9`): no 🔴, 12 🟠, all fixed for 1.0.0 (`#ab-dir`, 2026-11-02), each minimal. Then `--slow all` again and a delta review by `#ab-ceo` before the STABLE commit. Authority and secret fixes go to Claude windows (J85). The ⚪/🔵 items do not block.

- ✅ FR.B1 team 1: `agent-bus enrol` posts without a token (`307ec862`, 0.9.296).
- ✅ FR.B2 team 1: GitHub-backed directory lookups are cached per login and share one daemon-wide uncached allowance with 429; local files remain uncached and unbounded (`2a4cc233`, 0.9.301).
- ✅ FR.J1 team 2: the Codex pusher declines the file tools (`506d7239`).
- ✅ FR.G1 team 1: the release step creates or uploads with `--clobber`; distribution.md gives the tag procedure (`88a1b6dc`, 0.9.295).
- ✅ FR.Q1 team 1: the Containerfile verifies archives against `SHA256SUMS` (`a115609c`, 0.9.298).
- ✅ FR.I1 team 1: the 403 case asserts the page renders its own rows (`3a3a2b97`, no bump).
- ✅ FR.I2 team 1: `src/web/web-face/records.md` describes the Events card (`0ffd8572`, no bump).
- ✅ FR.A2 team 2: agent-log calls resolve the target with `b.entity` and answer `not_found` for an inactive record or User (`09018860`).
- ✅ FR.A3 team 2 (with A4): `sub add` refuses a target that cannot admit `#bus`, and `#bus` itself; the bus guide says a Personal agent is no target (`4d0fb211`).
  Letting Personal agents admit `#bus` was [№Q328](../R1.1/resolved.md#Q328), now [the Personal rules](../../docs/03-records.md#personal-and-shared).
- ✅ FR.D1 team 1: one name grammar through `protocol.ParseName`, with a shared vector for the Python and TypeScript helpers (`f32f605d`, 0.9.303).
- ✅ FR.C1 team 2: one daemon-wide bucket groups dirty alert snapshots into transactions (`4b2610a5`).
  Alerts reach disk within about 100 ms; a clean stop flushes all. `dropped` stays admission-only and unchanged.
- ✅ FR.F1 team 1: `docs/runner/security.md` says masked stderr reaches the agent's log, readable by its Owner and Maintainers. Not fixing in 1.0.0: the per-entry host setting `"events": "off"` waits for 1.0.1 (`#ab-dir`: the readers are the agent's own managers, it is disclosed once documented, and no reviewer quota is left for a runner change with a mutant) (`ac28de6d`, no bump).

## T-31 Client APIs and event examples

- ✅ T-31 All six clients run the shared event-log and subscription vectors with executed native examples, using plain send/call to `#bus`; [completion evidence](done/t31-client-events.md#landed-evidence) records Q322 approval and the landed parts. The [audit](../issues/done/2026-11/2026-11-02-client-api-audit.md#audit-snapshot) remains a dated snapshot.

## T-32 Good-agent checklist

- ✅ T-32 [Good-agent guide](../../docs/10-writing-an-agent.md), three worked pages and the checked examples/shard are built; [completion evidence](done/t32-agent-guide.md#landed-evidence) records the plan, five owning rules and Q327 normal-run placement.

## RT34 Agent events

- ✅ RT34 R1-min landed as T-29 parts P1–P5, `3b87ce4c` through
  `ca87beb0`, 0.9.289–0.9.293; deployed as 0.9.293.
  [Accepted scope](agent-events/TODO.md#r1-min) and
  [completed checks](agent-events/done/r1-landed-parts.md) record the result.
  [Post-1.0 parts](agent-events/TODO.md#post-10-parts) remain separate.

## RT33 May-send for another name

- ✅ RT33 The daemon, clients and cron, at, IMAP, Slack, Telegram and logwatch paths satisfy the author-admission contract ([RD141](DECISIONS.md#recorded-decisions)); B.29 supplies the shared send paths. Acceptance maps to existing checks; unavailable-authority gaps are covered by three adapter checks, two handler checks and eight assertion-red mutants. No agent code changed; [result and file:test mapping](done/rt33-may-send.md#acceptance-mapping).

## Distribution acceptance

Implement the [release artifacts](distribution.md#release-artifacts) and [container contract](distribution.md#container-runtime).

- ✅ T-10 One notice file, `licenses/NOTICE.md` — **0.9.189**, `0e7868ee` ([result](DONE.md#t-10-one-notice-file))
- ✅ P.3a Native aarch64 build and release evidence: passed 2026-10-30 ([result](done/p3-aarch64-mac-run.md#steps)); the clean install there passed on 0.9.223 (21 checks) after J59, once setup stopped restarting units faster than systemd's start limit ([result](done/p3-aarch64-mac-run.md#steps)). Retained evidence, not an npm publication blocker; the shell installer (P.4) is in [FUTURE](FUTURE.md#shell-installer).

- ✅ P.1 npm packages published as 1.0.0: full/client Linux x64 and arm64, client Darwin x64 and arm64, plus `ai-agent-bus` and `ai-agent-bus-client`. All eight public tarballs match the checked uploads and exact-version dependencies. Clean installed-tree acceptance and the hosted release result are in the [publication record](done/release-1.0-publication.md#registry-publication).
  - Packaging trust and installation contract: [distribution](distribution.md#npm). Private `@agent-bus/` development manifests stay unpublished.
  - P.1a unsigned-release checks: [result](done/p1a-unsigned-release.md#result). P.1c native Mac client acceptance: [result](done/p1c-macos-npm.md#native-acceptance).

- ✅ P.2 Ready-to-use container image: [result](DONE.md#done--r1) and [container guide](../../src/container/README.md#container-image); npm does not replace this separate artifact.

- ✅ P.5 Homepage: live 2026-10-30 from parf.dev `8b0e411`, flowcharts fixed in live `d65e4af`; [result and archived acceptance](done/homepage.md#result)

- ✅ P.3 Release evidence
  - Acceptance: Query every shipped program's version in both distributions and compare with the release source; require stamped Go build information and packaged license. Substitute an unstamped binary, alter one version or omit the license: each fails its corresponding check
  - Mutant: Substitute an unstamped binary, alter one version or omit the license: each fails its corresponding check
  - Built: each native build in `.github/workflows/release.yml` runs `src/release-verify.sh` on every full/client archive before upload; the release job requires both builds to succeed and verifies the complete unsigned checksum set before publication (P.1a). The owning test executes that exact workflow command and refuses unstamped, wrong-version and license-free artifacts, including a bad client beside a valid full archive; [result](DONE.md#p3-release-run-evidence), [release evidence](../../.claude/docs/deployment.md#release-evidence).
  - Native aarch64 build and version evidence passed ([result](done/p3-aarch64-mac-run.md#steps)), and the clean install there passed 21 checks (0.9.224; [the PASS list](done/p3-aarch64-mac-run.md#the-passing-runs-result)); installed npm acceptance remains under P.1. Require both native builds and checksum/manifest verification before the GitHub build release or npm wrapping; no signature, owner key or signing wait. This docs-only change pushes no tag and publishes nothing.

## macOS

Plan and acceptance in [macOS](macos.md#tasks); decided in [RD166](DECISIONS.md#recorded-decisions). The owner approved starting the cheaper path on 2026-10-30 (Q221).

- ✅ T-30a Release workflow ships Darwin arm64/x64 full and client archives plus the two client npm packages (`c6b8f8ec`, 0.9.288; [Q313 decision](DECISIONS.md#recorded-decisions)); the first real tag run remains the [tag-only proof](distribution.md#macos).
- ✅ MAC.1 Client and runner on macOS without a sandbox (`b4197cf2`): [client and runner](macos.md#mac1-client-and-runner-on-macos); shares `src/cmd/agent-bus-runner/`, `src/internal/cgroup/` and `src/internal/sandbox/` with team 2's M.5 and DR.1
  - State 2026-10-30, branch `ab-w1b/MAC1`: built and rebased on main; on the Mac (arm64, macOS 26.5.2) the throwaway release passes I2 install, I3 version and build stamp, I4 already current, I5 tampered archive refused with nothing written, I6 full node refused, and R1 `--sandbox=on` is refused with exit 1 and no output from the script. R2 passed 2026-10-30 17:49 over the pinned-TLS path (a loopback tunnel to the node's daemon, a credential issued for the one run and revoked): the script agent `sh -c "id -un"` answered `parf`, the account that started the runner, with the sandbox off. R3 (the same as a standard non-administrator account) skipped: optional by the [plan](mac-run-list.md#d-client-install-and-runner-on-macos-mac1) ("optional, the owner decides"), and MAC.2 proved the non-root service account (`#ab-ceo`, 2026-10-30); the Mac holds 57M under `~/agent-bus-dev`, nothing of this run
- ✅ MAC.2 Daemon with a reduced setup: code and native daemon/setup/admin acceptance verified ([result](done/mac2.md#checks-and-results)); the archive-based Mac run passed on 2026-11-01 at 15:14: `rc=0` in 27.059 s, revision `c49d51f3` (0.9.267), arm64, stamped `darwin-aarch64` full archive, the missing role group restored at GID 206, nothing left behind ([evidence](done/mac2.md#archive-acceptance)); [current setup](../../docs/platforms/macos.md#the-daemon).
  - Native reduced setup and cleanup landed in `f11c9d2a`: `sysadminctl` and `dseditgroup` verify absence, including an already removed group, and report named leftovers on failure. Local stubs and the owner cleanup proof pass ([cleanup](done/mac2.md#owner-cleanup-verified)).
  - MAC.2b (#ab-w1c, due 2026-11-02 18:00): full and client Darwin archives now build for both CPUs, with per-edition daemon checks, release manifests and Mach-O CPU/version/stamp verification. npm Darwin packages remain client-only. [Build evidence](done/mac2b.md#checks).
  - MAC.2c closes the revision-tie and release-set LOWs: native program stamps must name the release-source commit; collector and verifier agree on optional Darwin full sets ([checks](done/mac2c.md#checks)).
  - MAC.2d account-deletion partial is superseded by [MAC.2e role reuse](mac2b-run-list.md#role-account-reuse): acceptance keeps the verified hidden non-login account; fresh creation was already proven in the [10-31 log](done/mac2d-native-2026-10-31.log).
  - MAC.2g restores a missing role group at the existing user's unused PrimaryGroupID without changing the user; preflight accepts that safe user-only state, refuses group-only and GID clashes, and fails closed on unreadable privileged membership ([checks](done/mac2g.md#local-checks)). The native Mac run restored the group as designed ([result](done/mac2.md#archive-acceptance)).
- MAC.3 Moved to [R1.1](../R1.1/TODO.md#macos-sandbox) by owner decision (Q299), 2026-11-01.

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
- ✅ DW7 `docs/07-setup.md` — `dd56e796`; setup rewrite landed after distribution, with current release evidence and replacement packaging wording ([result](DONE.md#docs-rewrite-dw7))
- ✅ DW8 `docs/runner/*` and `docs/user/*` — **0.9.190**, `cded895b` ([result](DONE.md#docs-rewrite-dw8))
- ✅ DW9 Agent manifests — **0.9.191**, `99b1f94f`; Discord **0.9.206**, `4e74e3b6` ([result](DONE.md#docs-rewrite-dw9))
- ✅ DW10 Web page intros and CLI help text — **0.9.189**, `79f03e08` ([result](DONE.md#docs-rewrite-dw10))
- ✅ DW11 Client READMEs and `src/*.md`: all 16 pages rewritten; DW11b proves the verbatim checkout install/update blocks and the final two-identity PHP example ([result](DONE.md#docs-rewrite-dw11b)).
  - Acceptance for all rows: each page passes the checker and the first-20% check, every example was run on a real node, links and anchors resolve, built/pending status is unchanged
