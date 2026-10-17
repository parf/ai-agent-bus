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

| Task | Builds | Done when; mutation that must fail | Format to approve first |
|---|---|---|---|
| ☐ M.1 The runner is an agent | a new `agent-bus-runner` program and unit (`Delegate=yes`); registers `#runner@<host>`; the `agent-bus-runner` account given a principal so its socket is served; reads `agent.d/` and `runner/agents.json`; `reload` and a `status` call | the runner registers and answers `status` listing what is installed; drop the principal and the socket check fails; a malformed `agent.json` is reported by name, and skipping it silently fails | `agent.json`, `agents.json` fields; the `status` answer |
| ☐ M.2 It serves agents | each enabled agent registered with `deliver_to` to the runner; dispatch on `original_to`; the `args` and `json` forms (the foreground runner's serve loop, moved into a shared module); the reply comes from the addressed name; an unknown `original_to` refused to its sender; env layers, templates and instances | a call to `#hello@h` is answered by `#hello@h` through the runner; a message for an unserved name gets an error back, holding it fails; an instance missing a required variable is refused at `reload` | — |
| ☐ M.3 Processes | `processes` counts and ranges; the `jsonl` kept form; `SIGTERM`/`SIGHUP`, the 5-minute idle, the deadline, restart with backoff; `serves: itself` | a pool grows under load and shrinks after idle; a killed child is restarted and reported; an unhandled `SIGHUP` restarts it; removing the idle stop fails a bounded check | the `jsonl` frame |
| ☐ M.4 Sandboxing | one cgroup per agent in the delegated subtree, `CLONE_INTO_CGROUP`; bubblewrap with a PID and network namespace; the system-call filter; `limits` and `needs`; sized `/tmp`; the soft work-directory limit; fail closed | an agent cannot see a sibling's processes, write outside its work directory, or exceed its memory; on a host without `bwrap` it is refused; each removed limit fails its check | the filter's library ([modules](../../src/MODULES.md#external-tools)) |
| ☐ M.5 The host's controls | `enable`/`disable`/`start`/`stop`/`restart` calls; override files (JSON Merge Patch, mounted); `depends` ordering; the `backup` verb | an override changes the agent's view of its file and a refused key is named; a cycle in `depends` is refused; `backup` restores with the intended key only | the verbs' bodies |
| ☐ M.6 Pools and edges | `share`; the member hostname field (daemon change); edge runners under the bus's realm | two runners serve one name taking turns and a listing shows both hosts; an edge runner registers as `#runner/<edge>@<realm>` | the hostname field |
| ☐ M.7 Named hosts | the filtering proxy for `needs` hosts | a named host is reached and another refused; bypassing the proxy has no route | — |

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
