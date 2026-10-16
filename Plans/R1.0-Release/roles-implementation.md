# Assigned roles implementation

Draft checklist for [R1 roles](roles.md#role-syntax), 2026-10-16.
Planning only; generated roles and message transport already exist.

## Boundaries

- [ ] Resolve [representation and group compatibility](QUESTIONS.md#assigned-roles)
  before implementing storage. Do not add an endpoint or wire format in this plan.
- [ ] Preserve existing plain ACLs, empty-ACL access, Personal restrictions,
  caller standing, management authority and generated roles. `admin` is an
  application role, not node administration. Resolve the generated-name
  exception in [role naming](QUESTIONS.md#role-naming).
- [ ] Keep access matching separate from role collection: access may stop at
  its first grant; collection must union every matching grant. Management
  access must not skip collection. Supported `Record.Roles` remains informational.
  Extend `rolesOf` after successful authorization; never collect inside `may`.
  Role matching follows the originally addressed record's caller-access rules,
  not the forwarding-specific `admits` matcher or Maintainers syntax.
- [ ] Resolve roles once in `Send` for the originally addressed record, as
  today. Preserve forwarding, fanout, cross-owner checks and queued roles;
  never recalculate at consumption or restore of an existing message.

## Parsing and validation

- [ ] Add one shared actor-term parser; keep the current name, group and
  runtime-term validators authoritative for the undecorated actor.
  The daemon owns parsing and validation; Go and TypeScript face splitters
  share conformance examples, not runtime code.
- [ ] Scan text once, splitting entries only outside parentheses. Preserve
  `@dev(deploy, read_only)` as one entry; reject unbalanced/nested parentheses,
  empty roles and trailing junk; apply the settled generated-name policy.
- [ ] Update the shared role-name validator to the
  [naming policy](roles.md#roles-are-for-agents), including supported-role lists.
  Normalize actor names as today; reject invalid role spelling rather than
  rewriting it. Keep lowercase letters, digits and `_`; refuse hyphens. Do not impose
  application vocabulary, supported-list membership or a role-specific length cap.
  Unify core `roleName` and runner `validRole`, updating diagnostics in the same
  change. Test a valid role over 64 characters through `Send` into the spawned
  child's environment: `roleEnv` must not silently skip a daemon-accepted role.
- [ ] Retain the existing uppercase `AB_ROLE_<NAME>` mapping and inherited-role
  stripping. Verify names are shell-safe before constructing exports.
- [ ] Define duplicate actor/role handling in the approved representation;
  never let deduplication discard roles from another matching assignment.
  If Q149 permits assigning generated names, deduplicate against generated
  roles too, with a named overlap check; settle this before collection code.
- [ ] Validate Personal and kind constraints against underlying actors, not
  decorated text. Reject role syntax in Maintainers and group membership;
  role edits cannot change management positions.
- [ ] Make registration, refresh, manage and restore use the same rules.
  Invalid changes are atomic refusals; malformed stored assignments follow
  the existing invalid-record loading policy, never an open-access fallback.

## Runtime and performance

- [ ] Parse and validate on ACL changes and restore, never per message or
  discovery row. Keep parsed role-bearing terms immutable and derived; persist
  only the approved source representation, not duplicate runtime caches.
- [ ] Add an explicit absent-assignment fast path. Plain ACL records retain
  their existing access behavior and generated-role calculation, with no
  added role parser, deduplication map, sorting or per-message assignment copy.
  An empty supported-role list does **not** mean there are no assignments.
- [ ] Keep `rolesMayPass`'s existing empty-envelope-role shortcut. No assigned
  roles does not imply no generated `owner` or `maintainer` role.
- [ ] For assigned-role collection, visit each applicable term once and union
  only matched roles. Allocate lazily on the first assigned match; choose
  small-slice deduplication versus a set from measured cardinalities. Do not
  introduce a global role-ID table or registry-wide index without evidence.
- [ ] Evaluate groups and runtime terms against live state under the existing
  bus lock. Reuse matching primitives; avoid repeated graph walks for repeated
  terms within one resolution. If nested groups remain supported, traversal
  must be cycle-safe and bounded by reachable vertices and edges, not paths.
  Preserve the protected Administrator group's direct-only membership rule.
  Do not reuse a traversal's partially visited `seen` set as a negative-result
  cache. Test cycles, shared subgraphs and different ACL-term orders.
- [ ] Avoid persistent caller-role caches. Group membership, ownership, status
  and Maintainers edits must affect the next send without registry-wide rebuilds.
- [ ] Publish or discard derived state with the record's transaction. A failed
  commit, deletion, recreation or restore must leave no stale assignments.
  Unrelated metadata edits should reuse unchanged parsed assignments.
  Prefer immutable derived state carried by the staged record value; a side
  map requires explicit undo and restore handling, never a best-effort eviction.
- [ ] Benchmark before choosing further indexing. Compare paired plain-ACL
  baselines and the candidate for send, lookup/list and allocation counts;
  require zero incremental assignment allocations when no assignments exist.
  Separately measure nonmatching assignments, overlapping grants, large ACLs,
  nested/cyclic groups and fanout. Report CPU and lock hold time; no global
  slowdown may be hidden by a role-heavy aggregate.
  Extend `src/internal/baseline/baseline_test.go`; add paired `ReportAllocs`
  benchmarks and an allocation assertion for the absent-assignment branch.
  Keep reproducible commands, benchmark comparisons and mutex profiles under
  `tmp/roles-performance/` for review.

## Faces and persistence

- [ ] Update CLI ACL splitting and shared web register/edit fields to preserve
  parentheses and internal commas; show errors without dropping assignments.
  Audit both CLI comma splitters and web `ui/forms.tsx` whitespace splitting.
  Preserve existing plain-text input compatibility and test spaced role lists
  through create, edit and round trip in both faces in the same release.
- [ ] Audit MCP, all six client libraries, import/export and management round
  trips. Keep the daemon authoritative and avoid separate language parsers
  unless their text input needs one. Preserve plain syntax compatibility.
- [ ] Persist only on actual data changes through the existing write-through
  commit. Sending, matching and statistics must not trigger assignment writes.
- [ ] Rebuild derived state at startup; test old plain records, assigned records,
  failed commits and queued messages predating roles. No silent role removal
  on refresh or an unrelated edit.

## Acceptance and review

- [ ] Add named checks for plain/empty ACL parity; direct, wildcard, runtime
  and group grants; unions and duplicate grants; generated-role coexistence;
  nonmatching roles; invalid names; the settled generated-name policy; and
  management separation. Refuse uppercase, hyphens and shell punctuation;
  accept valid names over 64 characters within ordinary request-size limits.
  Explicitly test assignments with an empty supported-role list and a nonempty
  supported-role list without assignments; do not gate collection on `rec.Roles`.
- [ ] Exercise live group/ownership changes, Personal constraints, restore,
  failed-commit rollback and delete/recreate. Verify executed scripts and
  envelopes receive exactly the expected roles; inherited roles stay stripped.
- [ ] Keep route tests discriminating: sender roles must differ from roles
  recomputation would give at a forward target or pubsub subscriber. Cover
  allowed/refused cross-owner paths and the no-envelope-role shortcut.
- [ ] Kill compiling mutants that skip collection, stop at the first match,
  trust sender roles, widen management, ignore membership edits, retain a failed
  write's cache, or recalculate forwarded/restored roles. Each must fail its
  named assertion in `src/smoke.sh --slow`.
- [x] Obtain Claude and opencode reviews of this checklist; verified findings
  incorporated on 2026-10-16. Representation and group questions remain open.
- [ ] Cross-review the implementation and performance evidence. Run the full slow gate after
  code changes; publish commands and results, not just a green summary.
- [ ] Promote accepted behavior into current docs only when implemented;
  record pending/built status accurately. Do not push without the owner's go-ahead.
