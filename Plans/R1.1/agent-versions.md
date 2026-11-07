# Agent versions: every agent has its own

📌 **TL;DR:**
- **Problem:** every bundled agent but `#rag` reports the pack's version (1.0.4), so a user cannot tell which agent changed or when.
- **Outcome:** each agent has its own SemVer and its own `released`, shown in `--help`, `--help-json` and its web page; the pack version stays only as deploy bookkeeping.
- **Start:** [the rules](#rules), then [the readers](#every-reader-of-the-pack-version) and [the doc edits](#doc-edits). Task: [AV.1](TODO.md#agent-versions). Owner choices, settled: [№Q393](resolved.md#Q393), [№Q394](resolved.md#Q394).

Owner decisions, 2026-11-06: every agent has its own SemVer; agents gain a `released` beside it ([decision](DECISIONS.md#recorded-decisions)). This replaces [Q331](resolved.md#Q331), which kept one version for the pack.

## Rules

| Rule | Value |
|---|---|
| Where | `version` and `released` in the agent's `agent.json`, next to `description`, `license`, `author`, `homepage`, `docs` |
| Start | every bundled agent starts at 1.0.0, with a note in its changelog; the pack was never deployed to a user, so the step back is safe; `#rag` keeps its app line, 0.4.x ([№Q393](resolved.md#Q393)) |
| Changelog | `CHANGELOG.md` beside each `agent.json`, newest first; `#ab-ceo` writes the entry at deploy (`#rag` has one already) |
| PATCH | rises once per deploy, by `#ab-ceo` through `deploy-bump.sh`, when anything the agent ships changed since its own last bump commit; MINOR and MAJOR follow the usual rules |
| Last bump | the newest commit in `git log` of that `agent.json` that changed `version`; no state file |
| What an agent ships | its folder; the `src/agents/shared` files it declares in `src/smoke-map.json` (its `agent_<name>` paths); the bundled Python client `src/clients/python/agent_bus` when its package bundles it, declared the same way |
| `released` | the UTC day (`YYYY-MM-DD`) the agent's version was bumped, written by the same `deploy-bump.sh` step and commit as `version`; they move as a pair; never the build or package time, so a repackage or rebuild leaves it alone |
| First value | written by the first per-agent bump; none is back-filled |
| Users see | `version` and `released` in `--help`, `--help-json` and the web agent page, never the pack version |
| Missing | every bundled agent under `src/agents` carries both `version` and `released`; `agent_pack` fails when any lacks either, and the pack fallback goes ([one interface](../../CLAUDE.md#working-rules)). "Absent shows nothing" applies only to outside agents: an agent that is not bundled and has no `released` shows no date, with no placeholder or fallback; a missing `version` likewise shows nothing, never the bus version ([№Q394](resolved.md#Q394)) |
| Pack version | `src/agents/VERSION` and `src/agents/CHANGELOG.md` stay as internal deploy bookkeeping: they mark a package as the pack's and name what a deploy carried; no user surface shows them |

## First bump

The `agent_pack` check is red until every agent has both fields, so the first per-agent bump seeds `version` (1.0.0, [№Q393](resolved.md#Q393)) and `released` (that deploy's UTC date) in **every** `agent.json` at once, changed or not. It is one `#ab-ceo` deploy step: `deploy-bump.sh` run in its seeding form writes all agents, in the same commit and deploy window as the code change that adds the check. Later bumps touch only changed agents.

Done at 1.0.54 (2026-11-07): the seeding landed in the AV.1 branch, so the deploy needed no seeding bump. Two copies were not refreshed by `git-install.sh` and were fixed by hand: the standalone units `agent-bus-server-info` and `agent-bus-systemd-web` had to be restarted (server-info still answered 0.9.303), and the user runner's `tmux-sessions` package copy in `~/.local/share/agent-bus/agent.d/` (from 2026-10-29, with no `version`) had to be repackaged with its `package.sh`. The gap is J128.

The Go (`agentmeta`) and web change is a later code task, after the owner's OK on the plan; [№Q393](resolved.md#Q393) and [№Q394](resolved.md#Q394) are settled.

## Every reader of the pack version

Searched `src/`, `docs/`, `Plans/`, `CLAUDE.md` and the untracked `local/deploy-bump.sh` on 2026-11-06 for `agents/VERSION`, `support/agents/VERSION`, `AGENT_BUS_BUILD_AGENTS_VERSION`, `metadata.VERSION`, `packVersion`.

| Reader | Does | Verdict |
|---|---|---|
| `src/agents/shared/metadata.py:10,71` | reads `../VERSION`; reports `manifest.get('version', VERSION)` | drop: `version` required from `agent.json`, no fallback |
| `src/agents/shared/metadata.ts:45`, `shared/version.ts` | `manifest.version ?? VERSION` | drop `version.ts` and the fallback; its three `smoke-map.json` path entries go with it |
| `telegram`, `slack`, `discord` `package.sh` | set `AGENT_BUS_BUILD_AGENTS_VERSION=$(cat ../VERSION)` for the Bun build | drop the variable: the build writes the agent's `agent.json` version and `released` in |
| `src/internal/agentmeta/help.go:127,290` (`packVersion`) | `agent.json` version, else `support/agents/VERSION` or the pack `VERSION`, else the bus version | drop both fallbacks; report `version` and `released` only when `agent.json` has them (code task) |
| `src/internal/agentmeta/metadata_test.go` | tests the fallback chain | rewrite to the new rule |
| `src/web/agent-metadata.tsx:93` | shows `d.version` from the help answer | keep; add a Released row beside it; omit the row when it is absent |
| `src/agents/shared/test_help_golden.py` and its goldens | patch `metadata.VERSION` to `<version>` | change: the golden carries each agent's own `version` and `released` placeholders |
| 17 agent `package.sh` and `src/agents/package.sh`, `package-tree.sh` | install `VERSION` as `support/agents/VERSION` or `agents/VERSION` | keep: the file is the marker that a package is the pack's |
| `src/cmd/agent-bus-runner/pack_protocol.go:55` and `docs/runner/configuration.md:73` | `support/agents/VERSION` marks a pack package that must carry `protocol.json` | keep |
| `src/cmd/agent-bus-setup/package_install.go` (allowlists) | require the file in a package | keep |
| `src/release.sh:39,60` | layout check; prints `agents: <version>` after a release install | keep line 39; line 60 is the one operator-facing exception: it prints the pack bookkeeping version in the installer's own output, never in help or on a web page |
| `src/smoke-select.py:23` and its test | ignore `agents/VERSION` and `agents/CHANGELOG.md` when choosing checks | keep; add `src/agents/*/CHANGELOG.md`. A `deploy-bump.sh` edit to `agent.json` is not ignorable by path; bump commits are not gated, so nothing selects on them |
| `src/acceptance/agent-runtime.py:61` | compares packaged agents' reported version with `agents/VERSION` | change: compare each agent with its own `agent.json` |
| `src/agents/slack/package.test.ts:27`, `telegram/package.test.ts:27`, `discord/process.test.ts:20` | assert on `../VERSION` in the Bun bundle | change: each asserts the bundle carries the agent's own `version` and `released` from its `agent.json` |
| `src/agents/standalone/server-info/check.py:26` | comment: metadata.py reports the pack version | edit the comment |
| `src/agents/standalone/tmux-sessions/test_package.py:37` | lists `support/agents/VERSION` among the package files | keep |
| `src/agents/protocol_test.py`, `package_delivery_test.go`, `package_install_test.go`, `release_layout_test.go`, `agent_runtime_test.go`, `install_test.go`, `checkout_*_test.go`, `pack_protocol_test.go` | build or check packages that carry the file | keep; `agent_runtime_test.go` also asserts the reported version: change to the agent's own |
| `local/deploy-bump.sh` (`#ab-ceo`, untracked) | raises `src/agents/VERSION` once per deploy on any `src/agents/` change | rewrite: per agent, from its `agent.json` log; keep the pack file's own bump so the marker moves with a deploy |

## Doc edits

Made by [AV.1](TODO.md#agent-versions), not now; `CLAUDE.md` is `#ab-ceo`'s.

| File | Edit |
|---|---|
| `CLAUDE.md:140` Shared SemVer | "The agent pack is the one exception" becomes "Each bundled agent is the exception" |
| `CLAUDE.md:141` Agent pack row | replace with an Agent versions row: own `version` and `released` in `agent.json`, own `CHANGELOG.md`, PATCH per deploy by `#ab-ceo` when the agent changed since its last bump; the pack file is internal bookkeeping |
| `CLAUDE.md:143` Canonical value | the agent's version is its `agent.json`; `src/agents/VERSION` is bookkeeping |
| `docs/agents/README.md:151` ("Helper version") | each helper has its own version and release date, shown in its help |
| `docs/decisions.md:58` | a helper has its own version, so it changes without a new bus release or another helper's bump |
| `docs/runner/configuration.md:96` | `version` and `released` rows: bundled agents carry both; any other agent without them shows nothing |
| `docs/agents/create-agent-prompt.md:62` | "bundled agents omit it" becomes "a bundled agent sets both" |
| `Plans/subsystems.md:22` | the pack's own version becomes each agent's own version |
| `Plans/Team.v2/README.md:39` and `roles/ceo.md:29` | `--agents-summary` becomes the per-agent entry rule |
| `src/agents/CHANGELOG.md:4` | header says the pack file is deploy bookkeeping |
| `Plans/R1.1/TODO.md` AS.1 (lines 228–235) and Q341 context | "pack's own version" becomes per-agent |

No other document names the pack version (checked with the same searches).

## Cost

Docs now. The code task is small: one `agent_pack` assertion, the Go and web reads, `deploy-bump.sh`, and 19 `agent.json` edits. ETA for the code task: one worker, about an hour once the owner answers.
