# Team v2 questions

## Open questions

Only unresolved choices for [Team v2](README.md#team-v2).

- ✅ Q216 → [TD9](DECISIONS.md#TD9)

- ❓ Q217 The receipt and the board
  - Details: what a receipt carries upward and how the board is stored on the bus
  - Format: to approve before any code, only when the owner asks
  - Settled by: owner
- ✅ Q343 → [TD8](DECISIONS.md#TD8)

## Less overhead (2026-11-06 review)

Facts, 2026-11-05 18:00 → 11-06 01:40: 338 commits, of which 47 version bumps, 47 changelog edits and 41 issue-file commits; 107 gate runs, at least 3 of them green only on a rerun. Of about 35 landed items, roughly a third were real bugs or features (T-38, J104–J106, B6, B7, C2-E2, F1, F2, J90, A5), a third polish or edge cases (F4, F5, F7, K1–K3, G2, J4, D2/D3, B4/B4b, A8, G5, N-docs), and a third test plumbing (J79, J93, J108, J108b, J110, J112, J114). Each small item still drew two readers, mutants, a gate, a version bump, a changelog line and a separate close-out commit.

- ✅ Q348 → [TD7](DECISIONS.md#TD7)

- ✅ Q349 → [TD7](DECISIONS.md#TD7)

- ✅ Q350 → [TD6](DECISIONS.md#TD6)

- ✅ Q351 → [TD7](DECISIONS.md#TD7)

- ✅ Q352 → [TD7](DECISIONS.md#TD7)

## mdhouse coordination

- ✅ Q357 mdhouse's CEO reviews and commits #ab-ceo's start-agents.sh edits (owner, 2026-11-06); told on the bus

- ✅ Q358 → [TD10](DECISIONS.md#TD10)


## Repository rules

- ❓ Q366 May a plan hold its own questions in a combined document
  - Context: CLAUDE.md § Working rules, "Open choices", makes the owning plan's QUESTIONS file canonical, and a topic points to it. On 2026-11-06 the owner asked for RT19's questions to live in its combined plan ([root-runner.md § Questions](../R1.1/root-runner.md#questions)), with R1.1 QUESTIONS only pointing there; a lead then flagged it as a rule break
  - ( ) Add to the rule: "or a combined plan document, when the owner asks for one, with the QUESTIONS file pointing to it" 🌟
  - ( ) Keep the rule; RT19 is a one-off exception ⭐
  > 💡👾 Add it: the rule then matches what the owner does, and nobody flags it again.

- ❓ Q392 Add the docs-only landing rule to CLAUDE.md § Verification
  - Context: J121's gate is live (gate.sh a4b34ddad8e1, 2026-11-06 14:34): a docs-only diff lands on link checks plus the fast checks that read the changed pages, with no lock, following the owner's "do not run tests for minor changes". The rule is not yet in CLAUDE.md. Proposed line: "A Markdown-only diff containing docs/ pages, optionally with Plans/ or root Markdown, lands on link and marker checks plus selected fast checks, without a lock; LICENSE.md is excluded."
  - ( ) Add the line as written 🌟
  - ( ) Add it reworded (say how)
  - ( ) Leave CLAUDE.md as it is ⭐
  > 💡👾 Add it: the gate already behaves this way, and CLAUDE.md is where every agent reads the landing rules.
