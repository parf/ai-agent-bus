# Agent events results

📌 **TL;DR:** B.28 step 1a implements shared Python subscriptions and the
portable vector contract. This branch awaits QA and landing; eventlog rings,
the agent, its guide/package/deploy and the TypeScript twin remain pending.

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
