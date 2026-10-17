# TODO R1

## Objective

Prepare the [proposed scope](README.md#scope). Started ahead of the release on
the 0.8 line; no release wave is committed yet. ☑ marks what is built
([DONE](DONE.md)), ☐ what remains.

| | Work | State |
|---|---|---|
| ☑ | [Shared locks](done/locks.md#shared-locks) | built in 0.8.66 |
| ☑ | [Resource records](done/resources.md#resource-records) | built in 0.8.70 |
| ☑ | [Key-value store](done/kv.md#per-record-storage) | built in 0.8.77 |
| ☑ | [Roles reach Agents](roles.md#roles-are-for-agents): transport, generated `owner` and `maintainer`, supported list | built in 0.8.97–0.8.99 |
| ☑ | [Client libraries](done/client-libraries.md#scope): Python, TypeScript, Go, Rust, Java and PHP, with native examples | built in 0.8.100 |
| ☑ | [Assigned roles](roles.md#role-syntax) in allow lists | built in 0.8.110; [checks](done/assigned-roles.md#verification) |
| ☑ | [Record tags](tags.md#classification-and-filtering) | built 0.8.143; [verification](done/record-tags.md#verification); multiple-tag matching remains Q145 |
| ☐ | A formal protocol description for the clients | [Q17](QUESTIONS.md#open-questions) |
| ☐ | [Agent method metadata](method-metadata.md#method-metadata) | needs an owner-approved representation |
| ☐ | [Runner](runner.md#runner) | decided; [waves](#runner-waves) proposed |
| ☐ | [Agents for the runner](agents.md#what-moved-and-what-r1-adds): telegram, systemd, logwatch | owner, 2026-10-17; [tasks](#bundled-agents) |
| ☐ | [Backups](runner-implementation.md#backing-it-up) | decided: the runner's `backup` verb |
| ☐ | [Installable distribution](distribution.md#release-artifacts): P.1–P.3 | publication names and platforms |
| ☐ | [Federation](federation.md#chaining) | on hold |

## Next step

Assigned roles are built ([verification](done/assigned-roles.md#verification)). The other kept topics — method metadata, federation, installable distribution,
the runner — need resolution of their
[questions](QUESTIONS.md#open-questions), the split's Q137 first. Federation is on hold. Existing
decisions describe targets, not completed code. Identity, AUTH, encryption and
observability are [R1.1](../R1.1/TODO.md#objective).

## Dependencies

| Candidate work | Must precede it |
|---|---|
| ☑ [Record-defined roles](roles.md#record-defined-roles) | Built using the existing allow list and nested membership; [current contract](../../docs/03-records-agent.md#roles) |
| ☐ Federation — on hold | The owner taking it off hold; then the namespace decision ([Q20](QUESTIONS.md#open-questions)) and what chaining needs from R1.1 ([Q137](QUESTIONS.md#open-questions)) |
| ☐ Runner | [method metadata](method-metadata.md#method-metadata); the pool hostname field, now R1 ([where a member says it is](runner-implementation.md#where-a-member-says-it-is)) |
| ☐ [Agents for the runner](agents.md#dependencies) | Telegram and logwatch: the runner keeping an agent it did not write (M.1 and M.2, built; M.11 for the install) and the kept process form for state between messages (M.3, M.9). Systemd needs none of them — it is a root unit of its own running `agent-bus run` |
| ☐ [Agent method metadata](method-metadata.md#method-metadata) | The MVP description-only behavior it replaces ([decision](../../docs/decisions.md#settled)); an owner-approved representation |
| ☑ [Client libraries](done/client-libraries.md#scope) | Built in 0.8.100, all six with examples and slow-gate checks; formal protocol description remains [Q17](QUESTIONS.md#open-questions) |
| ☐ Release distributions | Release builds and runnable daemon/runner roles; choose publication names and supported platforms before packaging |

Name implementation waves and falsifiable acceptance after those choices.
Federation acceptance must exercise distinct nodes.

## Record tags acceptance

Built 0.8.143. [Current contract](../../docs/03-records.md#tags),
[verification](done/record-tags.md#verification). Multiple-tag matching remains
[Q145](QUESTIONS.md#record-tags); the implemented UI selects one label.

## Runner waves

Proposed 2026-10-16, for owner agreement before any code. Each wave is its own
version bump, `--slow` gate and peer review. A wave that needs a format first
proposes it for approval ([working rules](../../CLAUDE.md#working-rules)).
Rows run in the order listed; M.9 and M.8 are placed by what they need.

Until M.8, an agent is installed the way the runner's
[use case](runner.md#how-it-works) does it: `git clone` into `agent.d/`, its
configuration written under `runner/` on the host, then a `reload` call.

| Task | Builds | Done when; mutation that must fail | Format to approve first |
|---|---|---|---|
| ☐ M.1 The runner is an agent (done: the runner's User, 0.8.114; the program with `status` and `reload`, 0.8.117; its unit, 0.8.118; deployed 2026-10-16) | a new `agent-bus-runner` program and unit (`Delegate=yes`); registers `#runner@<host>`; the `agent-bus-runner` account given a principal so its socket is served; reads `agent.d/` and `runner/agents.json`; `reload` and a `status` call; its own heartbeat; each child's output captured to its log | the runner registers, stays alive in listings by its heartbeat, and answers `status` listing what is installed; drop the principal and the socket check fails; a malformed `agent.json` is reported by name, and skipping it silently fails | `agent.json`, `agents.json` fields; the `status` answer |
| ☐ M.2 It serves agents (deployed 2026-10-17 at 0.8.124, serving `#debug`; built 0.8.123: the read filter, forwarding, `args` and `json`, env layers, templates and instances, bounded answers; the deployer as Maintainer waits for M.8, which knows the deployer) | each enabled agent registered with `deliver_to` to the runner; dispatch on `original_to`; the `args` and `json` forms (the dedicated runner's serve loop, moved into a shared module); the reply comes from the addressed name; an unknown `original_to` refused to its sender; env layers, templates and instances; stats reported per agent | a call to `#hello@h` is answered by `#hello@h` through the runner; a message for an unserved name gets an error back, holding it fails; an instance missing a required variable is refused at `reload`; a script's output beyond a bound is cut and said, never buffered whole in the runner; each agent registered with its deploying User as a Maintainer ([who it runs as](runner-implementation.md#who-it-runs-as)); answers are sent off the read loop, so a slow send never leaves the runner without a reader; a template shows in `status` as one, not as a stopped agent | the env files' syntax; the read filter's name on the consume call ([Q162](runner-implementation.md#many-names-into-one-inbox)) |
| ☐ M.3 Processes | `processes` counts and ranges; the `jsonl` kept form with `in_flight`; `SIGTERM`/`SIGHUP`, the 5-minute idle, the deadline, restart with backoff | a pool grows under load and shrinks after idle; a killed child is restarted and reported; an unhandled `SIGHUP` restarts it; removing the idle stop fails a bounded check; `in_flight` 2 holds two messages at once and answers both in any order | the `jsonl` frame; [Q160](QUESTIONS.md#open-questions) |
| ☐ M.9 Agents that serve themselves | `serves: itself`: started and restarted, never registered by the runner, a fixed count only | such an agent registers itself and reads its own inbox; the runner registering it fails the check; a range on it is refused at `reload` | — |
| ☐ M.4 Sandboxing (in progress: bubblewrap namespaces, the filesystem and network rules and a clean environment, 0.8.126; a cgroup per agent with `CLONE_INTO_CGROUP`, `limits`, `needs.memory`, sized `/tmp` and failing closed without a delegated subtree, 0.8.150; the system-call filter is [Q166](QUESTIONS.md#open-questions), the disk limit [Q167](QUESTIONS.md#open-questions)) | one cgroup per agent in the delegated subtree, `CLONE_INTO_CGROUP`; bubblewrap with a PID and network namespace; the system-call filter; `limits` and `needs`; sized `/tmp`; the soft work-directory limit; fail closed | an agent cannot see a sibling's processes, read a sibling's files or env, connect to the runner's or another account's socket, write outside its work directory, or exceed its memory; on a host without `bwrap` it is refused; each removed limit fails its check | the filter's library ([modules](../../src/MODULES.md#external-tools)) |
| ☐ M.5 The host's controls | `enable`/`disable`/`start`/`stop`/`restart` calls; override files (JSON Merge Patch, mounted); `depends` ordering; the `backup` verb; a configuration change applied at `reload` ([Q19](runner.md#stop-and-reload)) | an override changes the agent's view of its file and a refused key is named; a cycle in `depends` is refused; `backup` restores with the intended key only; at `reload` a changed agent's kept processes get `SIGHUP` and an untouched sibling gets none | the verbs' bodies; how an archive larger than one answer travels; [Q161](QUESTIONS.md#open-questions) |
| ☐ M.6 Pools and edges | `share`; the member hostname field (daemon change); edge runners under the bus's realm | two runners serve one name taking turns and a listing shows both hosts; an edge runner registers as `#runner/<edge>@<realm>` | the hostname field; [Q159](QUESTIONS.md#open-questions) |
| ☐ M.7 Named hosts | the filtering proxy for `needs` hosts | a named host is reached and another refused; bypassing the proxy has no route | — |
| ☐ M.10 The runner acts as its person (owner decisions 2026-10-17, moved from R1.1; built: `register` answers the token of an agent an Agent created, 0.8.141; the runner as `#runner/<person>`, creating its agents create-only, keeping their tokens and reading each one's own inbox, 0.8.142; the Linux account with M.11) | the runner runs as a Linux account of its own, likely `$user-ab-runner`, so its secrets stay apart, but has **no bus identity of its own**: it is the person's agent `#runner/<person>`, and the person owns every agent it serves. **`register` answers with the new agent's token when, and only when, that call created the record**, so the runner creates an agent, keeps its token in its private directory and makes itself the agent's Maintainer. An agent that already exists is handed over once by its Owner. No new flag. Replaces the keyless User `agent-bus-runner`, and makes the serve-only grant (Q164) unneeded | a re-registration of an existing record answers no token; a register that created the record answers its token, which works; the runner never holds its person's token and serves no name it has no token for; removing the created-only guard lets a re-registration collect an existing agent's token, and the check fails | the `token` field on the register answer |
| ☐ M.11 Runner install through setup (owner, 2026-10-17; built 0.8.149: `--runner install\|uninstall\|enable\|disable --for <person> [--purge]`, the `<person>-ab-runner` account and `agent-bus-runner@.service` template, nothing installed by default; [a person's runner](../../docs/07-setup.md#a-persons-runner)) | `agent-bus-setup` options that install, uninstall, enable and disable a person's runner: the Linux account, its home, the unit, and with M.10 the person's `#runner/<person>`; nothing installed by default | each verb leaves the host in its stated state and a second run changes nothing; uninstall leaves no account, unit or link behind and keeps the person's `agent.d` unless asked; disable stops and disables the unit and keeps the files; a verb for a person who is not a bus User is refused | the option names |
| ☐ M.8 Deploy over the bus | install, configure and remove as calls to the runner ([reaching the runner](runner-implementation.md#reaching-the-runner)); configuration write-only | an agent is installed and served with no shell on the host; reading a configuration back is refused; a caller outside the runner's ACL is refused | the calls' bodies; how code arrives |

## Bundled agents

Asked for by the owner on 2026-10-17. Each is an entry of the
[R1.2 catalogue](../R1.2/services.md#the-catalogue) brought to R1 with the
runner that keeps it, except systemd, which the runner cannot keep and which
stands alone ([agents for the runner](agents.md#what-moved-and-what-r1-adds)).
The catalogue's own rules hold — the daemon applying the ACL before delivery,
and no daemon change. A row that needs a format first proposes it for approval.

| Task | Builds | Done when; mutation that must fail | Format to approve first |
|---|---|---|---|
| ☐ B.1 telegram | `#telegram/<account>@<realm>`: calls out, and a subscription forwarding a chat to an agent, 📮 queue or 📣 pubsub; the bot token read from the record's bus secret and nowhere else; `#group` and `@user` resolved to a chat id; a default channel declared in the instance's configuration ([telegram](agents.md#telegram)) | a post reaches its chat and answers what identifies it; a call with no destination goes to the instance's declared default, and the same call against an instance that declares none is refused; an unknown alias is refused rather than resolved to anything; an upstream refusal comes back as an answer; a subscription's match reaches its target and a non-match does not. Mutants: the token read from the process env or from `telegram.toml` at run time; the declared default replaced by the script's first-configured-chat fallback, which must fail the no-default refusal; the alias miss falling back instead of refusing | which API methods beyond post and read; where the alias tables live ([Q169](QUESTIONS.md#agents-for-the-runner)) |
| ☐ B.2 systemd | `#systemd/<set>@<host>`, **standalone**: its own install script writes a root unit running `agent-bus run` with the sandbox off, reaching the bus over IP with a token from the unit's `EnvironmentFile`. `list` and `status` at no role, `start`/`restart`/`reload` at the operate tier, `stop`/`enable`/`disable` at admin, over the unit set its instance declares ([systemd](agents.md#systemd)) | a caller with no role lists and reads status and is refused `start`; an operate caller starts, restarts and reloads and is refused `enable`; an admin caller reaches all of them; a unit outside the instance's declared set is refused at every tier including admin; the install script run twice leaves one unit and one registered name, and its removal leaves no unit, no token file and no orphaned record. Mutants: the role check removed, so a no-role caller starts a unit; the unit-set check removed, so admin reaches an undeclared unit; the tier read from the call body instead of the delivered roles, which must fail for a caller claiming a role it was not granted; the unit given `NoNewPrivileges=yes`, which must fail every acting verb and is what says the agent is standalone for a reason | the tier names ([open choices](agents.md#open-choices)) |
| ☐ B.3 logwatch | `#logwatch/<sources>@<host>`: a bounded ring over file globs, journald units and syslog; last N verbatim; an asked-for filter; subscriptions forwarding matching lines to an agent, 📮 queue or 📣 pubsub ([logwatch](agents.md#logwatch)) | the last N come back unfiltered by default and oldest first; a filter is applied only when asked for; a line arriving after a subscription was added reaches its target and a non-matching one does not; a source that appears after start is picked up; writing far beyond the ring's size leaves the ring at its bound and the process at its memory limit. Mutants: the ring's bound removed, which must fail the bounded check rather than hang it; a default filter applied when none was asked for; a new file never rescanned | which syslog input it takes; whether a source set is one instance or several |
| ☐ B.4 Choose the next set | — | the owner picks from [still to approve](agents.md#still-to-approve), or names others; each chosen entry gets its own row here before any code | — |

## Backup acceptance

☐ The [backup contract](runner-implementation.md#backing-it-up) is settled, invoked as the runner's `backup` verb. Implementation must restore an archive with
the intended user's key and refuse an unrelated key. Replace encryption with
plaintext output and the format/decryption check must fail; encrypt to the
wrong recipient and the intended-user restore must fail. Confirm backup creation
works with only the public key available.

## Distribution acceptance

Implement the [release artifacts](distribution.md#release-artifacts) and [container contract](distribution.md#container-runtime).

| Task | Done when; mutation that must fail |
|---|---|
| ☐ P.1 npm package | On a clean supported host without the checkout or Go toolchain, install the published package, start the bus, complete an agent request/reply and an MCP tool call. Omit a required executable or MCP runtime asset from the package and the corresponding exercise fails |
| ☐ P.2 Container image | Pull the published image on a clean host, start each role from the supplied instructions and complete an agent request/reply. Break either role's entry point and delivery fails. Recreate the daemon container with its volume and verify the registered service and issued credential still work; move storage outside the volume and recovery fails. Request unavailable sandboxing and require refusal; silently downgrading must fail the check |
| ☐ P.3 Release evidence | Query every shipped program's version in both distributions and compare with the release source; require stamped Go build information and packaged license. Substitute an unstamped binary, alter one version or omit the license: each fails its corresponding check |
