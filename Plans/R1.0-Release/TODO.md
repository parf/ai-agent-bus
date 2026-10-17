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
| ☐ | A formal protocol description for the clients | [Q17](QUESTIONS.md#open-questions) |
| ☐ | [Agent method metadata](method-metadata.md#method-metadata) | needs an owner-approved representation |
| ☐ | [Managed runner](runner.md#managed-runner) | decided; [waves](#managed-runner-waves) proposed |
| ☐ | A serve-only grant, so a runner serves a name its Owner keeps | to discuss: [Q164](QUESTIONS.md#open-questions) |
| ☐ | [Backups](runner-implementation.md#backing-it-up) | decided: the runner's `backup` verb |
| ☐ | [Installable distribution](distribution.md#release-artifacts): P.1–P.3 | publication names and platforms |
| ☐ | [Federation](federation.md#chaining) | on hold |

## Next step

Assigned roles are built ([verification](done/assigned-roles.md#verification)). The other kept topics — method metadata, federation, installable distribution,
the managed runner — need resolution of their
[questions](QUESTIONS.md#open-questions), the split's Q137 first. Federation is on hold. Existing
decisions describe targets, not completed code. Identity, AUTH, encryption and
observability are [R1.1](../R1.1/TODO.md#objective).

## Dependencies

| Candidate work | Must precede it |
|---|---|
| ☑ [Record-defined roles](roles.md#record-defined-roles) | Built using the existing allow list and nested membership; [current contract](../../docs/03-records-agent.md#roles) |
| ☐ Federation — on hold | The owner taking it off hold; then the namespace decision ([Q20](QUESTIONS.md#open-questions)) and what chaining needs from R1.1 ([Q137](QUESTIONS.md#open-questions)) |
| ☐ Managed runner | [method metadata](method-metadata.md#method-metadata); the pool hostname field, now R1 ([where a member says it is](runner-implementation.md#where-a-member-says-it-is)) |
| ☐ [Agent method metadata](method-metadata.md#method-metadata) | The MVP description-only behavior it replaces ([decision](../../docs/decisions.md#settled)); an owner-approved representation |
| ☑ [Client libraries](done/client-libraries.md#scope) | Built in 0.8.100, all six with examples and slow-gate checks; formal protocol description remains [Q17](QUESTIONS.md#open-questions) |
| ☐ Release distributions | Release builds and runnable daemon/runner roles; choose publication names and supported platforms before packaging |

Name implementation waves and falsifiable acceptance after those choices.
Federation acceptance must exercise distinct nodes.

## Managed runner waves

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
| ☐ M.2 It serves agents (deployed 2026-10-17 at 0.8.124, serving `#debug`; built 0.8.123: the read filter, forwarding, `args` and `json`, env layers, templates and instances, bounded answers; the deployer as Maintainer waits for M.8, which knows the deployer) | each enabled agent registered with `deliver_to` to the runner; dispatch on `original_to`; the `args` and `json` forms (the foreground runner's serve loop, moved into a shared module); the reply comes from the addressed name; an unknown `original_to` refused to its sender; env layers, templates and instances; stats reported per agent | a call to `#hello@h` is answered by `#hello@h` through the runner; a message for an unserved name gets an error back, holding it fails; an instance missing a required variable is refused at `reload`; a script's output beyond a bound is cut and said, never buffered whole in the runner; each agent registered with its deploying User as a Maintainer ([who it runs as](runner-implementation.md#who-it-runs-as)); answers are sent off the read loop, so a slow send never leaves the runner without a reader; a template shows in `status` as one, not as a stopped agent | the env files' syntax; the read filter's name on the consume call ([Q162](runner-implementation.md#many-names-into-one-inbox)) |
| ☐ M.3 Processes | `processes` counts and ranges; the `jsonl` kept form with `in_flight`; `SIGTERM`/`SIGHUP`, the 5-minute idle, the deadline, restart with backoff | a pool grows under load and shrinks after idle; a killed child is restarted and reported; an unhandled `SIGHUP` restarts it; removing the idle stop fails a bounded check; `in_flight` 2 holds two messages at once and answers both in any order | the `jsonl` frame; [Q160](QUESTIONS.md#open-questions) |
| ☐ M.9 Agents that serve themselves | `serves: itself`: started and restarted, never registered by the runner, a fixed count only | such an agent registers itself and reads its own inbox; the runner registering it fails the check; a range on it is refused at `reload` | — |
| ☐ M.4 Sandboxing | one cgroup per agent in the delegated subtree, `CLONE_INTO_CGROUP`; bubblewrap with a PID and network namespace; the system-call filter; `limits` and `needs`; sized `/tmp`; the soft work-directory limit; fail closed | an agent cannot see a sibling's processes, read a sibling's files or env, connect to the runner's or another account's socket, write outside its work directory, or exceed its memory; on a host without `bwrap` it is refused; each removed limit fails its check | the filter's library ([modules](../../src/MODULES.md#external-tools)) |
| ☐ M.5 The host's controls | `enable`/`disable`/`start`/`stop`/`restart` calls; override files (JSON Merge Patch, mounted); `depends` ordering; the `backup` verb; a configuration change applied at `reload` ([Q19](runner.md#stop-and-reload)) | an override changes the agent's view of its file and a refused key is named; a cycle in `depends` is refused; `backup` restores with the intended key only; at `reload` a changed agent's kept processes get `SIGHUP` and an untouched sibling gets none | the verbs' bodies; how an archive larger than one answer travels; [Q161](QUESTIONS.md#open-questions) |
| ☐ M.6 Pools and edges | `share`; the member hostname field (daemon change); edge runners under the bus's realm | two runners serve one name taking turns and a listing shows both hosts; an edge runner registers as `#runner/<edge>@<realm>` | the hostname field; [Q159](QUESTIONS.md#open-questions) |
| ☐ M.7 Named hosts | the filtering proxy for `needs` hosts | a named host is reached and another refused; bypassing the proxy has no route | — |
| ☐ M.8 Deploy over the bus | install, configure and remove as calls to the runner ([reaching the runner](runner-implementation.md#reaching-the-runner)); configuration write-only | an agent is installed and served with no shell on the host; reading a configuration back is refused; a caller outside the runner's ACL is refused | the calls' bodies; how code arrives |

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
