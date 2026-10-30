# Agent events results

📌 **TL;DR:** Shared subscription work has landed. This branch adds named
event logs, durable alert rings and their checks, awaiting QA and landing.
Live eventlog deployment and its web view remain pending.

## Shared subscriptions

2026-10-30, T-2, `task/eventlog`: shared authority, creator/direct-manager
visibility, bounded portable regex, once-only forwarding and persisted sent/drop
counts. Refusal drops one copy with one warning; the next entry still sends.
Creator→target and creator→source grants are checked again for each entry.
Overlong stored filters remain visible to their creator and inert, like other
unsupported filters. Runtime export, setup required-file inventories and the
`agent_subscriptions` fast/normal shard include the new shared modules/checks.
Existing agent runtimes are not migrated in this step.

Focused verification: 10 Python tests (filter and operation JSON vectors,
real send-helper HTTP fixture, regex/helper lifecycle); setup exported-runtime,
build/install and missing-artifact tests; 12 smoke selection/fast-driver regression
tests. The actual fast shard passed in 2.75 seconds. No normal gate run by the worker;
`#ab-gate` owns that landing gate.

Compiling mutants failed on named assertions, then were restored:

| Mutant | Owning test in `src/agents/shared/test_subscriptions.py` |
|---|---|
| Ignore text bound | `Parity.test_filter_vectors` |
| Skip daemon admission; trust forwarded manager; omit drop increment; broaden filter | `Parity.test_operation_vectors` |
| Treat refused send as sent | `Delivery.test_refused_copy_warns_once_then_next_entry_is_delivered_once` |
| Skip creator grant checks in real child | `Delivery.test_revoked_creator_to_target_or_source_prevents_delivery` |
| Extend authority timeout | `Delivery.test_stalled_authority_is_bounded_and_leaves_no_worker` |
| Extend per-pattern timer | `Helpers.test_regex_helper_timeout_is_per_pattern_and_group_reaped` |
| Remove JS whitespace range | `Helpers.test_js_whitespace_is_not_a_range_string_or_hyphen` |
| Let disarm-time alarm raise | `Helpers.test_alarm_during_disarm_does_not_drop_the_batch` |
| Kill leader alone | `Helpers.test_helper_kills_leader_and_descendant_holding_stdout` |
| Pass bus token to helper environment | `Helpers.test_helper_environment_has_no_bus_or_role_credentials` |

Removing either runtime module from `agents/package.sh` failed
`TestBundledAgentsRunOutsideCheckout` on its missing-artifact assertion.
The stored-overlong-filter vector failed on the old row-length rejection and
passes with that rejection removed; reinstating the rejection also failed that
vector on an assertion. Its row remains under the storage bound.

2026-10-30, T-2 follow-up: recorded the owner's damaged-state decision in
[the B.29 contract](../subscriptions.md#one-behaviour) and its decision index.
The `invalid-state-visible-inert-manager-repair-counts-limit` operation vector
pins exact `invalid_state` listing for direct managers, hidden state for other
callers/forwarded managers, no delivery, limit accounting and manager-only
repair. Changing the emitted reason to `other_reason` compiled and failed
`Parity.test_operation_vectors` on its exact rows assertion; restored afterward.

## Subscription sends without a caller wait

2026-10-30, T-2 urgent follow-up, `ab-w1c/T-2-wait`: remove explicit numeric
zero `wait` from the shared send helper. The daemon requires a positive caller
wait when supplied; forwarding is fire-and-forget and leaves that field absent.
The real-daemon engine check
`DaemonDelivery.test_actual_daemon_accepts_refuses_and_delivers_next_match`
proves an accepted copy reaches its inbox once, a full inbox refuses one copy
with one warning, and the next entry reaches the inbox once after draining.
Durable counts are sent=2/dropped=1. It runs inside the shared engine suite and
therefore the existing subscription smoke shard, not only adapter checks.
The original numeric-zero mutant compiles and fails the accepted-delivery
assertion; restored afterward. The HTTP fixture also pins the absence of wait.
Frozen subscription vectors are unchanged. No SDK or daemon change in this fix.

## Shared rings

2026-10-30, T-2 step 2, `ab-w1c/T-2` (pending QA/landing): shared
`event_rings.py`, six language-neutral JSON operation cases, 12 deterministic
checks. Independent size/TTL/severity; fixed first-instant folding; sanitized
scalar truncation; one ID sequence per log; durable block reservations and
alert snapshots; oldest-first destructive pull; inclusive partial clean;
full-clean slot retirement with permanent tombstones; strict state/config
validation, lifetime generation lock and poisoned uncertain mutations.

20 compiling mutants failed on assertions and were restored. Owning ring tests:
`test_sizes_ttl_and_severity_are_independent` (size, TTL),
`test_fold_uses_first_instant_and_latest_live_matching_text` (window, count,
backward clock), `test_single_line_sanitization_precedes_scalar_truncation`,
`test_shared_sequence_restart_gap_and_alert_fold_survive`,
`test_pull_oldest_once_closes_fold_and_persists_alert_removal`,
`test_partial_clean_id_boundary_and_full_clean_tombstone` (boundary, retirement),
`test_invalid_names_log_and_lifetime_caps_preserve_existing` (syntax, active
cap, lifetime cap), `test_corrupt_oversize_and_symlink_state_fail_closed`,
`test_atomic_failures_poison_until_reopen_and_never_reuse_ids` (fsync, poison),
`test_lifetime_lock_refuses_overlapping_generation`,
`test_configuration_overrides_bounds_and_strict_types`, `test_json_ring_vectors`.
The size mutant initially survived because read swept the overflow away; the
check now asserts capacity immediately after write, before a second operation.
`PureSupport.test_manager_identity_is_case_insensitive_but_not_forwarded`
failed when original_to normalization was removed. Mutant batch used the smoke
lock, released immediately afterward. No normal gate run by the worker.

## Eventlog agent

2026-10-30, T-2 step 2, `ab-w1c/T-2` (pending QA/landing): serialized kept
Python agent, strict text/JSON calls, direct read/write roles and metadata,
source/destination creator checks through shared subscriptions, selector-only
preflight before storage, subscription copies before pull, isolated work state,
package/runtime inventories, guide/index and fast/normal shard. Owner pinned
read subscription access; management links the shared current subscription
section owned by team 2. No frozen subscription vectors changed.

27 focused tests pass: 12 ring tests plus 15 adapter/support/package/daemon
tests. Five adapter tests use the isolated real daemon, including a kept-process
launch/config/work-directory/SIGTERM check. The fast shard passes in under two
seconds. Setup export, build/install, missing-artifact and archive-delivery
checks pass; five guide/index checks and 12 smoke-map/fast-driver tests pass.
No normal gate or live deployment was performed by the worker.

19 additional compiling mutants failed on assertions and were restored:

| Mutants | Owning test in `src/agents/eventlog/test_eventlog.py` |
|---|---|
| Unknown/missing fields, duplicate JSON, boolean count | `Pure.test_text_json_grammar_and_strict_bounds` |
| Forwarded roles | `Pure.test_direct_roles_status_and_metadata_ignore_ambient_and_forwarded_roles` |
| Reader destructive calls | `Pure.test_read_only_refuses_destructive_calls_without_removal` |
| Pre-storage deadline floor | `Pure.test_deadline_preflight_is_before_storage_but_no_candidates_have_no_floor` |
| Per-call source KV budget | `Pure.test_source_kv_budget_shares_deadline_and_restores_client` |
| Uncertain-storage poison | `Pure.test_storage_uncertainty_never_forwards_or_exposes_failure_details` |
| Escaped-ASCII reply overflow | `Pure.test_reply_bound_counts_newest_order_and_json_shapes` |
| Read caller deleting another creator's subscription | `Daemon.test_authenticated_reader_roles_and_foreign_subscription_delete` |
| Refusal counted as sent | `Daemon.test_full_target_drops_warns_once_then_next_delivers_and_revocation_blocks` |
| Sender claims delivery without inbox copies | `Daemon.test_subscriptions_arrive_before_pull_once_per_fold_and_selector` |
| Bare ACL data access | `Daemon.test_real_forwarded_owner_and_bare_acl_cannot_read_or_destroy` |
| Ignored work directory | `Daemon.test_kept_entry_uses_bus_config_work_directory_and_clean_shutdown` |
| Remove read from metadata's subscription roles | `Pure.test_help_json_exposes_reader_subscriptions_and_status_with_distinct_examples` |
| Omit each of four shared support files | `PurePackage.test_exported_helpers_config_and_offline_metadata_need_no_checkout_or_credentials` |

The unknown-field mutant initially survived because the negative fixture's
integer value was refused by argument typing. It now uses a valid string in
an unknown field, plus a separate missing-field case. Neither can rely on a
different validation path. Both mutant batches used/released the smoke lock;
all smoke invocations use PORT=8211.

QA lows: direct original_to comparison is normalized without trusting forwarded
roles; helper fingerprint inheritance is documented precisely. Failure-code
classification and caller replay/idempotency scope remain in
[FUTURE](FUTURE.md#subscription-failure-classification). No 200-message seen
cache was introduced. The numeric-zero caller-wait bug discovered against the
real daemon was separated into urgent `ab-w1c/T-2-wait` for its own landing.

## Step 2 QA repairs

- Rebased over the landed caller-wait fix (`f46036e5`). The HTTP fixture now
  explicitly asserts that the send has no `wait` key.
- After restarting with a smaller alert size, the test counts persisted alerts
  on the first read and checks that only the newest remains.
- The overlapping-generation test uses its own subprocess deadline. A removed
  production wait bound fails an assertion after three seconds, never hangs.
- The lock-file mode is asserted under an explicitly permissive fixture umask,
  so a requested 0644 cannot hide behind the host's umask.

| Mutant | Failing assertion |
|---|---|
| Remove `_sweep` size slice | `test_configuration_overrides_bounds_and_strict_types`: `3 != 1` |
| Remove lock-wait bound | `test_lifetime_lock_refuses_overlapping_generation`: `overlapping generation did not refuse within the test deadline` |
| Open events.lock as 0644 | `test_shared_sequence_restart_gap_and_alert_fold_survive`: `420 != 384` (0644 vs 0600) |
| Re-add numeric `wait=0` | `test_refused_copy_warns_once_then_next_entry_is_delivered_once`: `'wait' unexpectedly found` |

- Each mutant compiled, failed on its named assertion and was restored. One
  smoke-lock batch; released immediately, naming team 2 next.
- Final focused checks: 27 eventlog tests and 11 shared-subscription tests;
  eventlog fast shard 2.36 s, subscriptions fast shard 4.39 s. Setup runtime
  export/install checks, 13 smoke-selection checks and five guide checks pass.
- Smoke runs now leave PORT unset for the automatic block allocator. Frozen
  subscription vectors, version and changelog were not edited.

## Step 2 gate integration repairs

Both gate attempts reproduced two eventlog integration failures. The runner
refused unsupported metadata types, and setup omitted the agent command from
its executable inventory. Metadata now uses the supported `int` and `json`
types; setup installs `eventlog.py` executable.

| Restored defect | Failing assertion |
|---|---|
| Metadata `integer` | `TestTheShippedAgentsLoad`: `unknown type "integer"` |
| Omit executable inventory entry | `TestEveryBundledAgentCommandIsInstalledExecutable`: `setup would install 0644` |

Both mutants compiled and failed their named assertions under one smoke-lock
hold; both were restored and the lock released. The two checks and four focused
bundle/install checks pass. The eventlog fast shard passes in 1.93 seconds.

The first gate attempt also caught an unrelated `agent_sessions` failure:
`test_immediate_exit_failure_preserves_sibling_and_allows_retry` expected
`: failed:` but received `Bad: started`. That failure was reported separately;
the second attempt repeated only the two integration failures above.

## Empty Configuration defaults

A newly registered record returns null Configuration. The eventlog startup
precheck rejected it before opening state, despite the documented defaults.
`config()` and `config(None)` now resolve to the same defaults as `{}`; explicit
malformed non-null values remain refused. Rings uses this same normalization.

`Pure.test_null_or_absent_configuration_uses_documented_defaults` fails on
0.9.184, passes after the fix, checks each default and starts an Agent with null
Configuration to write an entry. Removing null normalization compiles and fails
its named `null Configuration must use defaults` and `absent Configuration must
use defaults` assertions. The one-mutant smoke-lock batch was restored/released.

Final checks: 12 ring tests, 11 pure/package tests, five guide checks; fast
agent_eventlog shard 1.85 seconds. Deployment remains frozen; this branch makes
no registration, credential, inventory or reload change.
