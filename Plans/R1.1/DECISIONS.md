# R1.1 decisions

## Recorded decisions

Migrated 2026-09-13 and split from the old R1 on 2026-09-30; the R1 rows are in [R1 decisions](../R1.0-Release/DECISIONS.md#recorded-decisions). Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| Q337 `#ai` defaults to `anthropic/claude-haiku-5.5` (its Configuration `model`) | [resolved](resolved.md#Q337) | Owner, 2026-11-05 |
| Q268, Q178 MCP: hand-write the Go face's small JSON-RPC part; the runner adds the `mcp` tag | [resolved](resolved.md#Q268) | Owner, 2026-11-05 |
| Q326, Q328 Subscriptions: one rule everywhere (source checked at creation); a Personal agent may list `#bus` in its Allow | [resolved](resolved.md#Q326) | Owner, 2026-11-05 |
| Q334, Q305–Q310, Q314–Q320 Build FileScan and DBSync: shared core, replay the existing base by default, system paths allowed, MySQL and PostgreSQL first | [resolved](resolved.md#Q334) | Owner, 2026-11-05 |
| FileScan host boundary | [host and sandbox](filescan.md#host-and-sandbox) | Owner, 2026-11-05 (Q308) |
| FileScan content | [what a message carries](filescan.md#what-a-message-carries) | Owner, 2026-11-01 and 2026-11-05 |
| Kept-process deployment | [host and sandbox](filescan.md#host-and-sandbox), [DBSync decisions](dbsync.md#decisions) | Owner, 2026-11-05 |
| DBSync credential boundary | [DBSync decisions](dbsync.md#decisions), [who may use it](dbsync.md#who-may-use-it) | Owner, 2026-11-05 |
| Q281–Q284, Q286, Q287 IMAP large transfer: chunked calls, bounded staging, caller-built mail, optional hash, base64 windows | [resolved](resolved.md#Q281) | Owner, 2026-11-05 |
| Q331 The agent pack keeps its own SemVer inside this one repository; Q332 an `agent.d` package may be a link to its checkout | [resolved](resolved.md#Q331) | Owner, 2026-11-05: "we have one monorepo, we just use diff semvers for daemon and agent-pack" |
| Q333 AI agent images: only `#ai`'s Owner and Maintainers may generate them, stored in `#ai`'s own bigfile record with a 24-hour expiry; reference images by URL or inline | [T-36](TODO.md#ai-agent-methods) | Owner, 2026-11-05: "OK for #ai bigfile" |
| T-34 Web Face AI setup chat moves from R1; T-33 Markdown workflow stays in R1 | [Chat scope](ai-setup.md#web-face-chat) | Owner, 2026-11-03 |
| Q191: agent status only on its own page, no mass polling or `ab_ls` status; web restart on `failed` only | [RT23 proposal](agent-capabilities.md#proposal) | Owner, 2026-11-01; [decision](../R1.0-Release/DECISIONS.md#recorded-decisions) |
| R1 cut scope: RT23, M.7, M.6b, RT11 RW10–RW17, RT19, MAC.3 and B.15 moved to R1.1 (Q299), retaining IDs | [Runner extensions](TODO.md#runner-extensions), [web additions](TODO.md#runner-web-additions), [capabilities](TODO.md#agent-capabilities), [cleanup](TODO.md#node-cleanup), [macOS](TODO.md#macos-sandbox) | Owner, 2026-11-01; [decision](../R1.0-Release/DECISIONS.md#recorded-decisions) |
| M.14 moved from R1 | [Runner hardening proposal](runner-hardening.md#runner-hardening-proposal), [task](TODO.md#runner-hardening), [Q276–Q280](QUESTIONS.md#runner-hardening) | 2026-11-01 owner decision: defer to R1.1 |
| Windows CLI client: a client-only `agent-bus` build for Windows in R1.1; reverses the 2026-10-19 "no windows" for that build alone | [windows CLI](windows-cli.md#windows-cli-client), [WIN.1](TODO.md#windows-cli-client) | 2026-10-30 owner instruction; reverses 2026-10-19 "no windows, mac/linux" ([R1 platforms](../R1.0-Release/TODO.md#objective))
| Windows client state: the CLI writes only its reply-context cache under the user's cache directory and needs no state directory or lock file; the keeper's file-name encoding is dropped | [files the client writes](windows-cli.md#files-the-client-writes-and-reads) | 2026-10-30 owner answer (was Q261: "what files? it is a simple client") |
| Windows `agent` (keeper) and `run` left out of the build, with a refusal | [what stays out](windows-cli.md#what-stays-out) | 2026-10-30 owner answer (was Q262) |
| Windows real-daemon run (WIN.1b): the owner provides a Windows server when it is due; the team then asks `#ab-ceo` through the lead | [testing](windows-cli.md#testing-without-a-windows-box) | 2026-10-30 owner answer (was Q263) |
| Windows launchers `ab-claude` and `ab-codex` ship in R1.1 with the CLI, with reduced features | [launchers on Windows](windows-cli.md#launchers-on-windows) | 2026-10-30 owner answer (was Q264, changed from "MCP face later") |
| Windows launchers are in Go with no bun on Windows, as one module meant for every OS so the later Linux and macOS move reuses it (replaces the bun-launchers option) | [launchers on Windows](windows-cli.md#launchers-on-windows) | 2026-10-30 owner answer (was Q267) |
| Windows `--take-over` is refused with a message | [the Go module](windows-cli.md#the-go-module) | 2026-10-30 owner answer (was Q265) |
| Windows launchers: `ab-claude`, `ab-codex`, and `ab-opencode` because OpenCode runs on Windows | [which runtimes](windows-cli.md#which-runtimes) | 2026-10-30 owner answer (was Q266) |
| The Linux and macOS switch to the Go client is R1.2 work; Linux keeps bun until then | [R1.2 item](../R1.2/TODO.md#go-only-client) | 2026-10-30 owner instruction |
| Runner backups (RT13 and RBK1) moved from R1 to R1.1; encrypted record export (RT29) stays in R1 | [backups](backups.md#scope), [acceptance](TODO.md#backups) | 2026-10-30 owner instruction |
| Discord (B.27a) moved from R1 to R1.1; its code stays on main and the R1 bundle and archives leave it out | [plan](../R1.2/discord.md#discord-agent-b27a), [task](../R1.2/TODO.md#discord) | 2026-10-30 owner instruction: "move discord to 1.1" |
| Discord uses the gateway with the message-content intent on, and has no `users` or `user` methods (RD172, settles Q241, Q242 and Q243) | [plan](../R1.2/discord.md#decided) | #ab-dir, 2026-10-30 |
| Additional storage backends assigned to R1.1 | [storage](storage.md#backends) | 2026-09-20 owner instruction; implement SQLite first in 0.7 |
| The MCP latest-only switch (K.37) postponed to R1.1, waiting for Codex and OpenCode | [K.37](TODO.md#mcp-latest-revision) | 2026-10-16 owner instruction |
| Scope | [definition](README.md#scope) | D69 |
| Registry sync | [definition](registry.md#registry-sync) | D79 |
| Dashboard extensions | [definition](discovery.md#dashboard-extensions) | D243 |
| Filescan: file contents go inline, bigfile only as the fallback | [what a message carries](filescan.md#what-a-message-carries) | Owner, 2026-11-01 |
| Filescan: state is one atomically written file in the work directory | [state](filescan.md#state) | Owner, 2026-11-01 |
| Filescan: one scan at a time | [how it works](filescan.md#how-it-works) | Owner, 2026-11-01 |
| Filescan: change notification | [finding changes](filescan.md#finding-changes) | Owner, 2026-11-01 |
| Filescan: the `mode` setting | [modes](filescan.md#modes) | Owner, 2026-11-01 |
| Filescan: host, authority and the mass-delete guard | [host](filescan.md#host-and-sandbox), [who may use it](filescan.md#who-may-use-it), [guard](filescan.md#the-mass-delete-guard) | Owner, 2026-11-01 |

## Open

Unresolved choices live in [questions](QUESTIONS.md#open-questions).

## Dashboard scope revision

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Optional dashboard additions in R1.1 | Owner confirms required/optional split | [dashboard extensions](discovery.md#dashboard-extensions) |

## Superseded

| Earlier design | Replacement |
|---|---|
| Basic groups, maintainers and activity graphs deferred to R1 | [MVP groups](../../docs/01-identity-and-authority.md#groups), [required dashboard](../../docs/05-discovery.md#required-tabs) |

## History

Original wording and superseded choices are preserved in [decision history](../done/R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../done/R0.8-MVP/done/document-migration.md#decision-mapping).
