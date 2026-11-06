# Team v2 decisions

## Recorded decisions

- ✅ TD1 Start with one team
  - Date: 2026-10-30
  - Why: `#gate`, the receipts and the roles settle on one team before a second doubles the sessions; answers Q218
  - Replaced by: TD3
  - Home: [status](README.md#status)
- ✅ TD2 `#ceo` replaces the manager, and the team uses simple names
  - Date: 2026-10-30
  - Why: the manager `#claude/ab-dvp-b` lost its reader ([J17](../issues/2026-10/2026-10-30.md)) and the owner stopped the old team; answers Q219. Names carry the project prefix `ab-`: `#ab-ceo`, `#ab-dir`, `#ab-lead1`, workers `#ab-w1a`, `#ab-w1b`, `#ab-w1c`, `#ab-qa1`, `#ab-gate`
  - Home: [status](README.md#status)
- ✅ TD3 Two teams, `#ab-lead1` for team 1 and `#ab-lead2` for team 2
  - Date: 2026-10-30
  - Why: owner instruction; each team has three workers (Claude on account 1, Claude on account 2, Codex) and a QA on z.ai, and `#ab-dir` splits work between them by subsystem
  - Supersedes: TD1
  - Home: [shape](README.md#shape)

- ✅ TD4 Team v2 is the adopted development structure
  - Date: 2026-11-03
  - Why: the owner confirms that most of the 0.9 development was carried out with this team; it is decided, not an unapproved proposal
  - Responsibilities: consolidate standing CEO/DIR handoff instructions; omit temporary capacity, queue and deployment state
  - Home: [status](README.md#status), [roles](README.md#roles), [operations](README.md#operations-and-authority)
- ✅ TD5 Shared team rules adopted from the mdhouse team, and the role prompts move into the repository
  - Date: 2026-11-05
  - Why: owner, after comparing the two teams: "yes, apply this rules to us too". Both Claude workers ran out of quota mid-release with nobody tracking it, and a stale-worktree commit nearly reverted a reviewed fix
  - Home: [shared rules](roles/common.md#team-v2--shared-rules)
- ✅ TD6 <a id="TD6"></a>One version bump per deploy; small and medium tasks land together
  - Date: 2026-11-06
  - Why: owner on Q350: "yes - makes bumps more significant; combine small/medium tasks into one make => test together". The night of 11-05 gave 20 PATCH numbers and 47 changelog edits for about 35 items
  - Rule: a landing carries no bump; #ab-ceo raises PATCH once per deploy window and writes one changelog entry for it. Leads group small and medium tasks into one branch and one gate run instead of a gate per task
  - Home: [roles](README.md#roles); one sentence in CLAUDE.md § Versioning and the lead and worker role files (T-44); gate.sh refuses `bump`
- ✅ TD7 <a id="TD7"></a>Less overhead: real findings only, risk-based reads, bookkeeping in the landing, flaky checks quarantined
  - Date: 2026-11-06
  - Why: owner on Q348, Q349, Q351 and Q352, after the 11-05 night review (about a third of the items were polish): "moreover - we should be sceptical and reject findings that are useless / have no impact"
  - Home: [shared rules § Less overhead](roles/common.md#less-overhead)
- ✅ TD8 <a id="TD8"></a>The team is started by hand; no boot relaunch, and the old team's tmux instance is disabled
  - Date: 2026-11-06
  - Why: owner on Q343: "we have a new team now; we need to disable old one; new one started manually". The old instance is `#tmux-sessions@parf.us` (`tmux-sessions/main`: windows C1, C2, X1) on the user runner
  - Home: [roles](README.md#roles); J107 closes with this decision
- ✅ TD9 <a id="TD9"></a>#ab-dir and both leads may restart a stuck worker window themselves
  - Date: 2026-11-06
  - Why: owner on Q216: "do this"
  - Rule: `#ab-dir`, `#ab-lead1` and `#ab-lead2` are on the allow list of `#tmux-sessions/ab-dvp@parf.us`, between tasks only, telling #ab-dir; config and commands stay the owner's
  - Home: [roles](README.md#roles)
- ✅ TD10 <a id="TD10"></a>Another project's sessions are restarted only to debug a critical bus issue, and only after asking the owner
  - Date: 2026-11-06
  - Why: owner on Q358: "you can do this only when debugging our critical issue (agent not attaching to session); next time - ask Parf before"
  - Home: [roles](README.md#roles)

