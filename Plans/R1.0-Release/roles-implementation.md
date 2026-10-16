# Assigned roles implementation

Draft checklist for [R1 roles](roles.md#role-syntax), 2026-10-16.
Planning only; generated roles and message transport already exist.

## Boundaries

- [ ] Resolve [representation and group compatibility](QUESTIONS.md#assigned-roles)
  before implementing storage. Do not add an endpoint or wire format in this plan.
- [ ] Preserve existing plain ACLs, empty-ACL access, Personal restrictions,
  caller standing, management authority and generated roles. `admin` is an
  application role, not node administration; `maintainer` cannot be assigned.
- [ ] Keep access matching separate from role collection: access may stop at
  its first grant; collection must union every matching grant. Management
  access must not skip collection. Supported `Record.Roles` remains informational.
- [ ] Resolve roles once in `Send` for the originally addressed record, as
  today. Preserve forwarding, fanout, cross-owner checks and queued roles;
  never recalculate at consumption or restore of an existing message.

## Parsing and validation

- [ ] Add one shared actor-term parser; keep the current name, group and
  runtime-term validators authoritative for the undecorated actor.
- [ ] Scan text once, splitting entries only outside parentheses. Preserve
  `@dev(deploy, read_only)` as one entry; reject unbalanced/nested parentheses,
  empty roles, trailing junk and reserved generated role assignments.
- [ ] Reuse the existing role-name validator and its length bound. Normalize
  actor names as today; never lowercase or otherwise rewrite role names.
- [ ] Define duplicate actor/role handling in the approved representation;
  never let deduplication discard roles from another matching assignment.
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
- [ ] Avoid persistent caller-role caches. Group membership, ownership, status
  and Maintainers edits must affect the next send without registry-wide rebuilds.
- [ ] Publish or discard derived state with the record's transaction. A failed
  commit, deletion, recreation or restore must leave no stale assignments.
  Unrelated metadata edits should reuse unchanged parsed assignments.
- [ ] Benchmark before choosing further indexing. Compare paired plain-ACL
  baselines and the candidate for send, lookup/list and allocation counts;
  require zero incremental assignment allocations when no assignments exist.
  Separately measure nonmatching assignments, overlapping grants, large ACLs,
  nested/cyclic groups and fanout. Report CPU and lock hold time; no global
  slowdown may be hidden by a role-heavy aggregate.

## Faces and persistence

- [ ] Update CLI ACL splitting and shared web register/edit fields to preserve
  parentheses and internal commas; show errors without dropping assignments.
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
  nonmatching roles; invalid/reserved names; and management separation.
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
- [ ] Obtain Claude and opencode reviews of this checklist, then cross-review
  the implementation and performance evidence. Run the full slow gate after
  code changes; publish commands and results, not just a green summary.
- [ ] Promote accepted behavior into current docs only when implemented;
  record pending/built status accurately. Do not push without the owner's go-ahead.
