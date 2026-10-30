# RT33 may-send close-out

📌 **TL;DR:** The daemon, Python/TypeScript clients and all six forwarding/scheduling agents check the authenticated author’s admission. Existing checks cover refusal and revocation; three added adapter checks cover unavailable authority. No agent implementation changed. Verified on the checkout, 2026-10-30; no live provider call or deployment.

## Result

- The accepted contract stays in [checking permission to send](../../../docs/09-daemon-api.md#checking-permission-to-send) and the [shared subscription behavior](../../../docs/agents/subscriptions.md#the-shared-behaviour).
- Daemon/clients were built in 0.9.141 (`2889a06b` / `f5442b9a`); cron/at followed in 0.9.144 (`7114ba9a`), chat/logwatch in 0.9.145 (`a54afe01` / `ba3b9c82`), IMAP in B.21. B.29 (`823e17c0`) supplies the accepted shared send paths.
- A known refusal drops that matching copy under RD161. Unavailable scheduling authority defers the due call without sending; unavailable subscription authority refuses the add or drops that matched copy. No unavailable answer becomes permission.
- Slack and Telegram both import `ChatSubscriptions` from `agents/shared/subscription-port.ts`; their handlers pass the authenticated envelope to `add`. IMAP and logwatch use the shared Python engine; cron and at use `Scheduler`.

## Acceptance mapping

Test references below are `file:test`; a shared path is identified rather than copied into each adapter.

| Path | Caller cannot send to target: add refuses | Author access revoked: next send stops | Authority unavailable: no add/send |
|---|---|---|---|
| cron | [test_cron.py](../../../src/agents/cron/test_cron.py): `Authority.test_the_creator_must_be_allowed_to_send_at_add` (subscription and one-shot; no stored rows) | same file: `Authority.test_the_creator_losing_access_drops_the_copy_before_any_send`; `test_a_dropped_slot_is_dropped_once_and_stays_dropped_when_the_creator_is_restored` | same file: `Authority.test_an_unavailable_check_refuses_and_sends_nothing`; `test_a_dead_daemon_costs_one_question_per_pass` (shared Scheduler) |
| at | [test_at.py](../../../src/agents/at/test_at.py): `Firing.test_the_creator_must_be_allowed_to_send` | same test revokes admission while at remains admitted and checks no delivery | same file: `Firing.test_unavailable_authority_refuses_add_and_defers_without_sending` (new) |
| IMAP sub | [test_subscriptions_bus.py](../../../src/agents/imap/test_subscriptions_bus.py): `SubscriptionBusTests.test_authenticated_author_admission_revocation_and_delete` (real daemon, authenticated creator) | same test; [test_forwarding.py](../../../src/agents/imap/test_forwarding.py): `ForwardingTests.test_a_revoked_author_is_checked_before_every_send` | same forwarding file: `ForwardingTests.test_unavailable_authority_refuses_add_and_drops_only_that_copy` (new) |
| Slack sub | [subscription-port.test.ts](../../../src/agents/slack/subscription-port.test.ts): `chat authority refusal or unavailability stores nothing at add and never sends a blocked copy` (new, real daemon) | same file: `the real daemon's revoked author grant drops a copy and never sends as the helper instead` | new chat test, through the actual shared ChatPort |
| Telegram sub | same ChatSubscriptions add/ChatPort test as Slack; common vectors below also pin refused adds | [forward.test.ts](../../../src/agents/telegram/forward.test.ts): `Telegram checks author authority for each message and stop prevents later sends`; shared real ChatPort revocation test | same shared ChatPort test as Slack; no per-agent admission implementation |
| logwatch sub | [test_logwatch.py](../../../src/agents/standalone/logwatch/test_logwatch.py): `WatcherTests.test_author_is_checked_for_target_and_source_at_add_and_each_copy` | same test; [test_delivery.py](../../../src/agents/standalone/logwatch/test_delivery.py): `SharedDelivery.test_revoked_author_drops_a_copy_without_pausing_the_subscription` (real daemon) | same WatcherTests test covers add and forward with bus errors and unexpected answers |

| Common contract | Existing check |
|---|---|
| Caller-only API stays unchanged, including error bodies, no counters/messages/activity | [may_send_test.go](../../../src/internal/api/may_send_test.go): `TestMaySendMatchesSendWithoutMessagesOrCounters`; `TestMaySendNeverAcceptsAnOnBehalfOfCaller` |
| Explicit author follows Send’s ACL/Personal/user rules and does not disclose a hidden target | same file: `TestMaySendForAnotherNameFollowsSendAndLimitsDisclosure` |
| Python client never reads an error or foreign-author answer as permission | [test_client.py](../../../src/clients/python/tests/test_client.py): `LiveClient.test_may_send_asks_for_another_name_and_never_reads_an_error_as_no`; `TransportChecks.test_may_send_refuses_an_answer_that_is_not_for_the_asked_name` |
| TypeScript client has the same boundary | [client.test.ts](../../../src/clients/typescript/tests/client.test.ts): `maySend asks for another name and never reads an error or a foreign answer as no` |
| Shared Python/TS admission parity | [test_subscriptions.py](../../../src/agents/shared/test_subscriptions.py): `Parity.test_operation_vectors`; [subscriptions.test.ts](../../../src/agents/shared/subscriptions.test.ts): `case add-false`, `case add-404`, `case add-unexpected`, `case add-timeout`, `case revoked-target`, `case revoked-source` |

## Proof the new checks fail

Each mutation ran only its owning new check under the slow lock. Python mutations parsed; Bun mutations reached the named `expect` assertion. No build or loader failure was credited. Files were restored after each mutation.

| Mutation | Named check and assertion |
|---|---|
| Scheduler accepts an unavailable add verdict | new at test: expected `could not check`, got a stored call ID |
| Scheduler treats unavailable firing authority as allowed | new at test: `['once'] != []` |
| Shared Python admission accepts an unavailable add verdict | new IMAP test: `SubscriptionError not raised` |
| Python send helper bypasses author checks | new IMAP test: delivered target/body list differs from `[]` |
| Shared TS add ignores unavailable authority | new chat test: expected rejected promise, got a successful add |
| Shared TS delivery ignores unavailable authority | new chat test: expected sent 0/dropped 1, got a sent copy |

Restored focused checks pass: cron Authority 21, at Firing 8, IMAP forwarding 15 plus its real-daemon authority check, logwatch authority and real-daemon revocation checks, Python operation vectors, API may-send checks and both client checks. The combined shared/chat/Telegram Bun selection passes 49 tests / 383 assertions; the TypeScript client selection adds one test / 13 assertions. Links and whitespace pass.

## Remaining work

RT33 has no remaining implementation or acceptance gap in this audit. Live agent deployment is a separate operational step; this close-out makes no claim about which instances were reloaded.

The owner’s T-16 follow-up is separately named under P.1 as **Release verifier without signing** in [distribution acceptance](../TODO.md#distribution-acceptance): remove `--signers` and its signing tests, retain checksum/manifest checks, and prove the checksum-bypass mutant. This is pending work, not implemented by RT33.
