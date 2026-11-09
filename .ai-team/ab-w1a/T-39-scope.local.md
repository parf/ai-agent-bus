# T-39 step A — agent-pack rebase scope

📌 **TL;DR:** `ab-w1a/agent-pack` merges onto local main with no textual conflict. Main is at `c722581c`, and the merge was checked read-only with `git merge-tree`. Five things need work before the pack can land:
- the starting pack version;
- the gate's `bump-agents` word;
- the agent paths no check owns;
- the runner/daemon → agents coverage drop;
- the image tag rule.

release.sh needs no owner question; the image tag does (Q-A below).

## 1. Conflicts against local main

| Check | Result |
|---|---|
| Branch | 5 commits, 42 files, +538 −405, base `97fa11f8` (2026-11-04) |
| Main since base | 51 commits, 62 files; VERSION `1.0.0` → `1.0.6` |
| Files both sides touched | `docs/agents/README.md` only; merges clean (pack adds a "Helper version" section, main edited elsewhere) |
| `git merge-tree --write-tree main ab-w1a/agent-pack` | rc 0, no conflicts |
| Pending branches touching `src/smoke.sh` / `CLAUDE.md` | `ab-w1a/T-35-state`, `ab-w1a/J-batch` (mcp_rpc line), `ab-w1b/G-low` (CLAUDE.md deploy row, maintenance check) — each merges clean with the pack |
| `src/smoke.sh` on main since base | unchanged, so the shard move (273 lines out, into `src/agents/smoke.sh`) applies as is |

Semantic, not textual:
- **Pack start version.** The pack says `1.0.0 — 2026-11-04`. Since then main shipped agent changes under bus bumps up to 1.0.6: #ai models/embed/image, fetch and webhook probe deadlines, discord README. Proposal: keep `1.0.0` as the pack's first version and re-date the changelog entry to the landing day. Its line should say the pack starts from the agents as of that bus version. The pack's own history starts there; no retroactive pack bumps.
- **Main's own agent commits since base** (`aa5342ba`, `db97076d`, `0891eef0`, `ed276b76`, `a4a0c9de`, `f83b0693`) do not touch the files the pack edits. Their new tests live in the agents' own directories, already mapped. Re-run selection after the rebase to confirm (a `smoke_select_test.py` case).
- **Gate.** `.ai-team/gate.sh` parses `<branch> [T-n] [bump | stable]`; it knows no `bump-agents`. CLAUDE.md's new row says the lead marks `bump-agents` and `#ab-gate` raises the pack's PATCH and changelog. gate.sh needs that word (raise `src/agents/VERSION`, prepend `src/agents/CHANGELOG.md`, refuse `bump-agents` with `stable`). It is untracked and #ab-gate's, so a separate small task; until then the pack lands without `bump-agents`.

## 2. What agent-only selection does to the smoke rules

| Area | Today (main) | With the pack |
|---|---|---|
| Ignored paths | `src/internal/version/VERSION`, root `CHANGELOG.*.md` | + `src/agents/VERSION`, `src/agents/CHANGELOG.md` |
| A diff only in `src/agents/` (L, with N/O/P prose) | L at normal, every other subsystem at fast | only checks that own a changed path, plus `agent_pack`; everything else **skipped** (`agents_only`) |
| A runner/daemon change (B, C, F, G) | agent shards tagged `GL`, `FGL`, `BGL`, `CGL` run at normal | those shards are re-tagged `L`; L is dropped from the reverse-dependency promotion, so agents run at **fast** only |
| Shared helpers | `src/agents/shared/` prefix owns every shared file | an explicit per-file list per check |
| Shard home | `src/smoke.sh` | `src/agents/smoke.sh`, sourced by `src/smoke.sh`; `agent_pack` is the pack contract (help goldens; at normal, docs, docs order, packaged tree outside the checkout, manifest reload) |

Rule conflicts to settle before landing:
1. **Fail-closed gap.** In `agents_only`, a changed path no check owns runs only `agent_pack`. Fourteen pack files are owned by no check:
   - 4 under `shared/`: `metadata_contract.py`, `test_envfile.py`, `test_help_golden.py`, `subscriptions.md`;
   - 3 under `standalone/`: `systemctl-fixture.py`, `README.md`, `starter.md`;
   - 7 at the top: `agent_docs.py`, `agent_docs_test.py`, `docs_order_test.py`, `package.sh`, `smoke.sh`, `VERSION`, `CHANGELOG.md`.

   `agent_pack` covers several of these, but not all:
   - `metadata_contract.py` is imported by five agents' `test_metadata.py` (debug, git, logwatch, server-info, systemd), yet a change to it runs none of them;
   - `systemctl-fixture.py` feeds the systemd tests;
   - `test_envfile.py` never runs.

   This contradicts CLAUDE.md's "unmapped paths promote every subsystem". Fix: an unowned path under `src/agents/` promotes L to the requested mode (fail closed). Add a map test that every tracked `src/agents/` file is owned by a check, by `agent_pack`, or by an explicit prose list.
2. **Normal coverage moves.** Re-tagging `GL`/`FGL`/`BGL`/`CGL` → `L` moves agent regression checks out of a runner/daemon change's normal run. CLAUDE.md: "Normal retains its regression and security checks; slow-set moves require proving the same production mutant fails a faster normal check." This is a normal → fast move for B/C/F/G changes and needs the same proof. Alternatively, keep the old tags for the shards that exercise the runner (sandbox, reload, root installs). Recommendation: keep `G` on the shards that start agents under a runner, and drop it only where the shard tests agent logic alone.
3. **CLAUDE.md wording** (pack commit `01aba112`): the selection paragraph grows by about 90 words in an already long paragraph. Split it into the paragraph plus a two-row table (agent-only diff / other diff). Same substance, and it keeps one home.
4. **Mutation rule.** Unchanged. A mutant still runs only its owning test, and `src/agents/smoke.sh` shards are "owning tests" like any other.

## 3. Proposal: what ships as the agent-pack version

One release tree still carries everything; the pack version is data inside it, never a second release unit.

| Carrier | Proposal |
|---|---|
| Release tree (`build.sh` → `src/agents/package.sh`) | `agents/VERSION` beside `internal/version/VERSION` (already in the pack); each package's `support/agents/VERSION` (already) |
| `release.sh` | Directory name stays `releases/<bus version>-<sha>`: the sha already separates an agent-only change. Add `agents/VERSION` to its layout check (line 39, beside `internal/version/VERSION`). Print `agents: <pack version>` on the `live:` line. No owner question: nothing about selection, rollback or naming changes |
| `agent-bus-setup` | Requires `agents/VERSION` (pack commit `e8fad8f6`); no other change proposed |
| Container image | Version from `internal/version/VERSION` as now; add `LABEL org.opencontainers.image.version=<bus>` (if absent) and `LABEL io.agent-bus.agents.version=<pack>`. The tag question is Q-A |
| Help/status | Agents report the pack version (pack commit `e8fad8f6`); `#debug` doctor could list both versions; follow-up, not in T-39 |

## 4. Owner question

release.sh needs none. The image push does, because the owner's standing OK names the tag scheme ("tagged with that commit's VERSION and `latest`").

### Q-A: image tag after an agent-only change

An agent-only landing raises the pack version, not the bus VERSION. A rebuilt image would re-push the existing `1.0.6` tag with different agents, and a pulled `1.0.6` would then no longer name one image.

- ( ) Keep the tag scheme; push an image only when the bus VERSION changes, so agent-only changes ship with the next bus bump 🌟
- ( ) Tag `<bus>-agents.<pack>` (e.g. `1.0.6-agents.1.0.3`) plus `latest`; never re-push an existing tag ⭐
- ( ) Re-push the bus tag with the new agents (a tag no longer names one image)

💡👾 The first option: no new tag scheme and no change to the standing authorization. The cost is that agent fixes reach the image only at the next bus bump, which today comes several times a day.

## Suggested T-39 steps after A

1. Rebase on local main; re-date the pack changelog; rerun `smoke_select_test.py` and `agent_pack`.
2. Fail-closed for unowned `src/agents/` paths, plus the ownership map test (mutant: drop the promotion).
3. Restore `G` on runner-exercising agent shards, or prove the moves with mutants.
4. Add `agents/VERSION` to release.sh's layout check and `live:` line, and the container labels.
5. A gate.sh `bump-agents` word (#ab-gate's file).
6. After landing: owner Q-A in the owning plan's QUESTIONS.
