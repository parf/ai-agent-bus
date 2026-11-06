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
  - Home: pending T-44 (CLAUDE.md § Versioning, [roles](README.md#roles), lead and worker role files, gate.sh)

