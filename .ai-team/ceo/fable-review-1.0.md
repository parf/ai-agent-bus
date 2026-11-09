# Final Fable review for 1.0.0

📌 **TL;DR:** the last review before the STABLE commit: ten read-only Fable reviewers read the whole
tree at T0 against the 1.0 contract and report reproduced issues to the day's issues file. Run by
`#ab-ceo` after the `--slow all` run on T0 is green ([runbook](../ab-w2b/release-1.0.md),
step 2). Fixes go through `#ab-dir` and the normal gate.

## Scope

- Tree: T0 = `af1c7e06` (0.9.294), the commit the slow run read. The report names the tree it read.
- Contract: [the client contract and stability rules](../../docs/09-daemon-api.md#the-client-contract),
  [access](../../docs/02-access.md#acl), [the process boundary](../../docs/08-processes.md#the-rule),
  [setup](../../docs/07-setup.md#install), `.github/workflows/release.yml` and the packagers.
- New since the last review (2026-11-01): agent events and subscriptions in the daemon (store schema 10
  and 11), runner event forwarding, client event support, the agent guide and its examples, macOS
  release archives, the Slack start-up fix, the smoke parallelism cap.

## Rules

- [find-issues](../../.claude/skills/find-issues/SKILL.md#reviewers) and
  [the checklist](../../.claude/skills/review-checklist.md), every section.
- Read-only: no edits, no commits, no gate lock, never the live daemon, its database or the web face it
  serves. Probes use a scratch daemon in `tmp/` and remove it afterwards.
- Light on the host: no `--normal` or `--slow` runs, no load loops; one focused test at a time.
- Reproduce before reporting; a claim of absence names every implementation checked.
- Never raise again what `Plans/issues/` holds from 2026-10-28 on, open or closed, unless its premise changed.
- At most 10 issues per reviewer, worst first, in the find-issues item form.

## Reviewers

| # | Subsystems | Focus |
|---|---|---|
| 1 | A registry core | ACL and authority, ownership, enrolment; agent-event ACL, subscription authority at creation (Q325) |
| 2 | A registry core, E HTTP API | delivery, queues, counters, `#bus` dispatch lanes; routes against the docs/09 route table and refusal kinds |
| 3 | B authentication and keys | token lifetime, sessions, key proofs, TLS pinning; secrets never echoed |
| 4 | C storage and logs | schema 10 and 11 migrations, what a failed write leaves, restore and retirement pruning, rollback to 0.9.288 |
| 5 | D parsers and wire | names, envelopes, env files, `#bus` event verb parsing, input bounds |
| 6 | F runner, security | exec, environment delivery, sandbox and cgroup limits, privilege |
| 7 | F runner, correctness; L bundled agents | event forwarding queue and rate limits, reload and stop; Slack start-up (J89), Telegram and Discord |
| 8 | G setup, install and release | release.yml with darwin archives, npm packages, packagers, file modes, build stamping |
| 9 | Q container image | root until the role drop, volume ownership, archive verification, sandbox never downgraded |
| 10 | N docs, K clients, I web, J MCP | docs against code (docs/09, docs/10 guide, example pages, README), six clients' event support, the web event view, MCP contracts |

## Output

`Plans/issues/2026-11/2026-11-02.md`, appended under `# Issues — Nov 2, 2026 · <last reviewed>..af1c7e06 · tree <T0 tree hash>`.
Every 🔴 and 🟠 becomes a row in the R1 TODO block "1.0 final review" with an owner; `#ab-dir` decides
each 🟠 (fix, or not fixing with a reason). No open 🔴 and every 🟠 closed or accepted before STABLE.
