# Assigned roles implementation

Implementation checklist for [R1 roles](roles.md#role-syntax), 2026-10-16.
Built on the 0.8 line; [verification](done/assigned-roles.md#verification).

## Boundaries

`RA` labels the original assigned-role implementation checklist entries.

- ✅ RA1 Implement using the existing allow-list representation and nested-group
  - Details: behavior; [contract](../../docs/03-records-agent.md#roles). No endpoint or schema change.

- ⚠️ RA2 Obtain owner confirmation of assigned-role applicability
  - State: representation and duplicate-grant behavior confirmed 2026-10-17; nested groups confirmed as [№RD87](DECISIONS.md#recorded-decisions)
  - Open: the unchecked applicability choice in [№Q146](QUESTIONS.md#open-questions)

- ✅ RA3 Preserve existing plain ACLs, empty-ACL access, Personal restrictions,
  - Details: caller standing, management authority and generated roles. `admin` is an application role, not node administration. `owner` and `maintainer` remain generated-only; assigning either as an ACL role rejects the entire save.

- ✅ RA4 Keep access matching separate from role collection: access may stop at
  - Details: its first grant; collection must union every matching grant. Management access must not skip collection. Supported `Record.Roles` remains informational. Extend `rolesOf` after successful authorization; never collect inside `may`. Role matching follows the originally addressed record's caller-access rules, not the forwarding-specific `admits` matcher or Maintainers syntax.

- ✅ RA5 Resolve roles once in `Send` for the originally addressed record, as
  - Details: today. Preserve forwarding, fanout, cross-owner checks and queued roles; never recalculate at consumption or restore of an existing message.


## Parsing and validation

- ✅ RA6 Add one shared actor-term parser; keep the current name, group and
  - Details: runtime-term validators authoritative for the undecorated actor. The daemon owns parsing and validation; Go and TypeScript face splitters share conformance examples, not runtime code.

- ✅ RA7 Scan text once, splitting entries only outside parentheses. Preserve
  - Details: `@dev(deploy, read_only)` as one entry; reject unbalanced/nested parentheses, empty roles and trailing junk. Refuse ACL assignments of `owner` or `maintainer`.

- ✅ RA8 Update the shared role-name validator to the
  - Details: [naming policy](roles.md#roles-are-for-agents), including supported-role lists. Normalize actor names as today; reject invalid role spelling rather than rewriting it. Keep lowercase letters, digits and `_`; refuse hyphens. Do not impose application vocabulary, supported-list membership or a role-specific length cap. Unify core `roleName` and runner `validRole`, updating diagnostics in the same change. Test a valid role over 64 characters through `Send` into the spawned child's environment: `roleEnv` must not silently skip a daemon-accepted role.

- ✅ RA9 Retain the existing uppercase `AB_ROLE_<NAME>` mapping and inherited-role
  - Details: stripping. Verify names are shell-safe before constructing exports.

- ✅ RA10 Define duplicate actor/role handling in the approved representation;
  - Details: never let deduplication discard roles from another matching assignment.

- ✅ RA11 Validate Personal and kind constraints against underlying actors, not
  - Details: decorated text. Reject role syntax in Maintainers and group membership; role edits cannot change management positions.

- ✅ RA12 Make registration, refresh, manage and restore use the same rules.
  - Details: Invalid changes are atomic refusals; malformed stored assignments follow the existing invalid-record loading policy, never an open-access fallback.


## Runtime and performance

- ✅ RA13 Parse and validate on ACL changes and restore, never per message or
  - Details: discovery row. Keep parsed role-bearing terms immutable and derived; persist only the approved source representation, not duplicate runtime caches.

- ✅ RA14 Add an explicit absent-assignment fast path. Plain ACL records retain
  - Details: their existing access behavior and generated-role calculation, with no added role parser, deduplication map, sorting or per-message assignment copy. An empty supported-role list does **not** mean there are no assignments.

- ✅ RA15 Keep `rolesMayPass`'s existing empty-envelope-role shortcut. No assigned
  - Details: roles does not imply no generated `owner` or `maintainer` role.

- ✅ RA16 For assigned-role collection, visit each applicable term once and union
  - Details: only matched roles. Allocate lazily on the first assigned match; choose small-slice deduplication versus a set from measured cardinalities. Do not introduce a global role-ID table or registry-wide index without evidence.

- ✅ RA17 Evaluate groups and runtime terms against live state under the existing
  - Details: bus lock. Reuse matching primitives; avoid repeated graph walks for repeated terms within one resolution. If nested groups remain supported, traversal must be cycle-safe and bounded by reachable vertices and edges, not paths. Preserve the protected Administrator group's direct-only membership rule. Do not reuse a traversal's partially visited `seen` set as a negative-result cache. Test cycles, shared subgraphs and different ACL-term orders.

- ✅ RA18 Avoid persistent caller-role caches. Group membership, ownership, status
  - Details: and Maintainers edits must affect the next send without registry-wide rebuilds.

- ✅ RA19 Publish or discard derived state with the record's transaction. A failed
  - Details: commit, deletion, recreation or restore must leave no stale assignments. Unrelated metadata edits should reuse unchanged parsed assignments. Prefer immutable derived state carried by the staged record value; a side map requires explicit undo and restore handling, never a best-effort eviction.

- ✅ RA20 Benchmark before choosing further indexing. Compare paired plain-ACL
  - Details: baselines and the candidate for send, lookup/list and allocation counts; require zero incremental assignment allocations when no assignments exist. Separately measure nonmatching assignments, overlapping grants, large ACLs, nested/cyclic groups and fanout. Report CPU and lock hold time; no global slowdown may be hidden by a role-heavy aggregate. Extend `src/internal/baseline/baseline_test.go`; add paired `ReportAllocs` benchmarks and an allocation assertion for the absent-assignment branch. Keep reproducible commands, benchmark comparisons and mutex profiles under `tmp/roles-performance/` for review.


## Faces and persistence

- ✅ RA21 Add mouseover help to the agent edit page's roles and ACL fields, using
  - Details: the shared register/edit components. Explain that supported roles are informational, assigned roles come from matching ACL terms, and the Agent decides their meaning. Show `@dev(deploy, read_only)`, lowercase/digits/`_` naming, and reading `roles` or `AB_ROLE_DEPLOY`. Explain generated roles according to the settled policy, without implying application roles grant AgentBus management authority. Make the help available on keyboard focus too. Verify the edit page renders both help controls with this content; removing either tooltip must fail its named check.

- ✅ RA22 Update CLI ACL splitting and shared web register/edit fields to preserve
  - Details: parentheses and internal commas; show errors without dropping assignments. Audit both CLI comma splitters and web `ui/forms.tsx` whitespace splitting. Preserve existing plain-text input compatibility and test spaced role lists through create, edit and round trip in both faces in the same release.

- ✅ RA23 Audit MCP, all six client libraries, import/export and management round
  - Details: trips. Keep the daemon authoritative and avoid separate language parsers unless their text input needs one. Preserve plain syntax compatibility.

- ✅ RA24 Persist only on actual data changes through the existing write-through
  - Details: commit. Sending, matching and statistics must not trigger assignment writes.

- ✅ RA25 Rebuild derived state at startup; test old plain records, assigned records,
  - Details: failed commits and queued messages predating roles. No silent role removal on refresh or an unrelated edit.


## Acceptance and review

- ✅ RA26 Add named checks for plain/empty ACL parity; direct, wildcard, runtime
  - Details: and group grants; unions and duplicate grants; generated-role coexistence; nonmatching roles; invalid names; generated-name reservation; and management separation. Refuse uppercase, hyphens and shell punctuation; accept valid names over 64 characters within ordinary request-size limits. Test `bob(owner)` and `@dev(deploy, maintainer)` on create and edit: refuse the whole save, preserving the previous ACL and any other simultaneously edited field. A mutant that strips the reserved role or accepts the save must fail the named check. Supported-role declarations may still list these generated roles; they do not assign them. Explicitly test assignments with an empty supported-role list and a nonempty supported-role list without assignments; do not gate collection on `rec.Roles`.

- ✅ RA27 Exercise live group/ownership changes, Personal constraints, restore,
  - Details: failed-commit rollback and delete/recreate. Verify executed scripts and envelopes receive exactly the expected roles; inherited roles stay stripped.

- ✅ RA28 Keep route tests discriminating: sender roles must differ from roles
  - Details: recomputation would give at a forward target or a Deliver-To recipient. Cover allowed/refused cross-owner paths and the no-envelope-role shortcut.

- ✅ RA29 Kill compiling mutants that skip collection, stop at the first match,
  - Details: trust sender roles, widen management, ignore membership edits, retain a failed write's cache, or recalculate forwarded/restored roles. Each must fail its named assertion in `src/smoke.sh --slow`.

- ✅ RA30 Obtain Claude and opencode reviews of this checklist; verified findings
  - Details: incorporated on 2026-10-16. Representation and group questions remain open.

- ✅ RA31 Cross-review the implementation and performance evidence. Run the full slow gate after
  - Details: code changes; publish commands and results, not just a green summary.

- ✅ RA32 Promote accepted behavior into current docs only when implemented;
  - Details: record pending/built status accurately. Do not push without the owner's go-ahead.
