# TODO R1.1

## Objective

Prepare the [proposed scope](README.md#scope). Not started; there are no implementation waves committed yet.

## Next step

Storage scope is assigned; its implementation follows the 0.7 prerequisites.
Other proposed work needs scope confirmation and resolution of
[questions](QUESTIONS.md#open-questions). Existing decisions describe targets, not completed code.

## Dependencies

| Candidate work | Must precede it |
|---|---|
| ☐ [Additional storage backends](storage.md#backends) | SQLite persistence contract and exclusive database access implemented in 0.7 |
| ☐ Scoped credentials and encryption | [Settled lifetime policy](../../docs/02-access.md#token-lifetime); remaining key recovery and token grammar decisions |
| ☐ D.1–D.4 encryption carried from MVP | [Encryption acceptance](encryption-wave.md#d--the-bus-stops-reading-payloads); key lifecycle decisions |
| ☐ [Peer registry](registry.md#registry-sync) | Record authenticity and clock decisions ([Q5](QUESTIONS.md#open-questions)) |
| ☐ [Optional dashboard extensions](discovery.md#dashboard-extensions) | Required MVP dashboard acceptance; the AUTH, health, stats, federation and runner data each additional view reports |
| ☐ Visitor IP on web-face edits — the face forwards the visitor's address and the audit log records it (Q128) | a trusted way for the face to state it, since the daemon sees only its socket |
| ☐ AUTH authorization freshness | [Q35](QUESTIONS.md#authorization-refresh), independently of credential lifetime |

Name implementation waves and falsifiable acceptance after those choices. Crypto acceptance must test what the daemon cannot decrypt; peer-registry acceptance must exercise distinct nodes.

## Windows CLI client

Owner, 2026-10-30; plan in [Windows CLI client](windows-cli.md#steps-and-estimate), owner answers in [decisions](DECISIONS.md#recorded-decisions), open [Q265–Q267](QUESTIONS.md#open-questions). MAC.1 has landed.

| ID | Deliverable | Acceptance and mutation |
|---|---|---|
| ☐ WIN.1 | The `agent-bus` CLI builds for Windows amd64 and reaches a remote daemon over pinned HTTPS ([plan](windows-cli.md#what-you-get)); daemon, runner, sandbox and launchers stay out | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/agent-bus` succeeds in the normal `go_checks` shard, and untagging one Linux-only file fails it. On a `windows-latest` CI job: `agent-bus --version` prints the version and a `build_info:` stamp; a request and a reply complete against an in-process pinned-TLS test daemon; a wrong fingerprint is refused with nothing sent; a missing `AGENT_BUS_ADDR` says it is required; a left-out subcommand exits non-zero naming itself. The Linux and macOS builds and their normal checks are unchanged. Mutants: drop the pin check and the wrong-fingerprint check fails; send the reply-context cache to `$HOME/.cache` on Windows and the cache-location test fails |
| ☐ WIN.1b | The same request and reply against a real daemon, from a Windows machine | Prerequisite: a Windows server the owner provides when it is due (ask `#ab-ceo` through the lead), and a reachable isolated daemon with TLS and a token; not part of WIN.1's acceptance |
| ☐ WIN.2 | `package.sh` writes `agent-bus-client-<version>-windows-x86_64.zip`; `release-verify` and `release-sign` take it | the archive holds `agent-bus.exe` and no node program; `release-verify` passes it with the build stamp read from the binary, and fails an unstamped one; `release-sign` signs it with the other archives, all or none |
| ☐ WIN.3 | `ab-claude.cmd` and `ab-codex.cmd` start or resume a named session on Windows with the bus tools, on the compiled `agent-bus-ts.exe` ([launchers](windows-cli.md#launchers-on-windows), option a) | First day, a spike on a real Windows machine: the compiled launcher starts a runtime and the face reaches a bus; if it fails, WIN.3 stops and the Go port is decided ([Q267](QUESTIONS.md#open-questions)). Then on `windows-latest`: the launcher and face unit tests pass; a binding lock taken twice is refused on the second (the NTFS `EPERM`/`EACCES` case); a killed runtime leaves no child; `--take-over` is refused with its reason. Mutants: treat `EPERM` as success and the double-lock test fails; skip the tree kill and the child test fails |

## Storage acceptance

Owner-assigned, pending. Preserve the task ID carried from 0.7.

| Task | Done when; mutation that must fail |
|---|---|
| ☐ K.1.2 Configure backend selection and add MySQL/PostgreSQL adapters | Implement the [backend contract](storage.md#backends). Run shared storage checks against real SQLite, MySQL and PostgreSQL, covering uniqueness, transactions, restart, failed commits, statistics batching, exclusive access across competing processes/hosts, backup/restore and performance. Ignoring backend selection, bypassing a transaction or allowing two daemons to serve the same database fails the corresponding check |

## MCP agents

Owner request, 2026-10-17; proposed, pending [Q178](QUESTIONS.md#open-questions).

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| ☐ K.46 | An agent answers the body `--mcp` with an MCP `tools/list` result, and is tagged `mcp`; `agent-bus call <to> --mcp` sends it ([MCP agents](mcp-agents.md#mcp-agents)). `#slack` answers it, limited to what the caller may do | Q178; [K.42](../done/R0.8-MVP/TODO.md#agent-self-description) | a valid answer adds the tag, an invalid or missing one does not; `GET /ls?tag=mcp` lists the agent; Slack's tools validate against the MCP schema. A tag added without a valid answer, or Slack's admin tools shown to a plain caller, fails the check |

## MCP latest revision

> **Waiting for Codex and OpenCode.** Postponed from R0.8 on 2026-10-16. The
> MCP face stays on 2025-11-25 until Codex and OpenCode speak MCP 2026-07-28,
> and Claude Code delivers channels on it. On 2026-10-16 Codex 0.163.0 sent
> only a 2025-06-18 `initialize`, OpenCode 2.0.30 only 2025-11-25; Claude Code
> 2.1.302 tries 2026-07-28 first. Recheck on each client release
> ([measurements](../../docs/constitution.md#external-protocols)).

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| ☐ K.37 | The MCP face speaks only the latest MCP specification, under the [2025-11-25 exception](../../docs/constitution.md#external-protocols) until then (Q143). The v2 SDK (`@modelcontextprotocol/server` 2.2.0) serves 2026-07-28 and refuses a 2025 `initialize` with -32022 | Claude Code delivering channels on 2026-07-28; Codex and OpenCode speaking it | the face and its smoke checks negotiate only 2026-07-28; a 2025 `initialize` is refused; push reaches Claude Code on that revision. Pinning the old SDK or accepting an old revision fails the check. Recheck the clients on each release |

## Backups

Owner-moved from R1 on 2026-10-30; task IDs and acceptance are preserved.

- ⏳ RT13 [Backups](backups.md#backing-it-up)
  - Details: decided: the runner's `backup` verb

## Backup acceptance

- ⏳ RBK1 Backup acceptance
  - Acceptance: The [backup contract](backups.md#backing-it-up) is settled, invoked as the runner's `backup` verb. Implementation must restore an archive with the intended user's key and refuse an unrelated key. Replace encryption with plaintext output and the format/decryption check must fail; encrypt to the wrong recipient and the intended-user restore must fail. Confirm backup creation works with only the public key available.
  - Mutant: Replace encryption with plaintext output and the format/decryption check must fail; encrypt to the wrong recipient and the intended-user restore must fail.
