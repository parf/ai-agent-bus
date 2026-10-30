# RT33 may-send close-out

📌 **TL;DR:** The daemon, Python/TypeScript clients and all six forwarding/scheduling agents check the authenticated author’s admission. Existing checks cover refusal and revocation; three added adapter checks and two handler checks cover unavailable authority. No agent implementation changed. Verified on the checkout, 2026-10-30; no live provider call or deployment.

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
| Slack sub | [s4.test.ts](../../../src/agents/slack/s4.test.ts): `RT33 a caller refused by the target cannot subscribe through the Slack handler`; [subscription-port.test.ts](../../../src/agents/slack/subscription-port.test.ts): `chat authority refusal or unavailability stores nothing at add and never sends a blocked copy` (new, real daemon) | same file: `the real daemon's revoked author grant drops a copy and never sends as the helper instead` | new chat test, through the actual shared ChatPort |
| Telegram sub | [t6.test.ts](../../../src/agents/telegram/t6.test.ts): `RT33 a caller refused by the target cannot subscribe through the Telegram handler`; shared ChatPort and common vectors also pin refused adds | [forward.test.ts](../../../src/agents/telegram/forward.test.ts): `Telegram checks author authority for each message and stop prevents later sends`; shared real ChatPort revocation test | same shared ChatPort test as Slack; no per-agent admission implementation |
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
| Slack handler supplies the helper as the caller | new Slack handler test at `s4.test.ts:130`: expected `admission_refused`, received an added ID |
| Telegram handler supplies the helper as the caller | new Telegram handler test at `t6.test.ts:233`: expected `admission_refused`, received an added ID |

Restored focused checks pass: cron Authority 21, at Firing 8, IMAP forwarding 15 plus its real-daemon authority check, logwatch authority and real-daemon revocation checks, Python operation vectors, API may-send checks and both client checks. The Slack/Telegram handler suites pass 15 tests / 64 assertions, including the two refusal/grant controls. The combined shared/chat/Telegram Bun selection passes 49 tests / 383 assertions; the TypeScript client selection adds one test / 13 assertions. Links and whitespace pass.

## Remaining work

RT33 has no remaining implementation or acceptance gap in this audit. Live agent deployment is a separate operational step; this close-out makes no claim about which instances were reloaded.

The owner’s T-16 follow-up is separately named under P.1 as **Release verifier without signing** in [distribution acceptance](../TODO.md#distribution-acceptance): remove `--signers` and its signing tests, retain checksum/manifest checks, and prove the checksum-bypass mutant. This is pending work, not implemented by RT33.

## Completed implementation plan

Historical task detail moved from TODO on acceptance. RD161 replaces the old pause/retry policy; the current contract is linked above. Original implementation owners, scope, checks and required mutants follow.


- ⏳ RT33 `GET /may-send?who=<name>&name=<target>` and its use by every agent that sends for someone else (daemon and clients ✅ 0.9.141 `2889a06b`/`f5442b9a`, deployed, 14 mutants, reviewed; Slack, Telegram and logwatch `sub` ✅ 0.9.145 `a54afe01`/`ba3b9c82`, 20 mutants, gate 233/0, reviewed, not yet deployed; IMAP `sub` ✅ in B.21, 0.9.150: author's may-send checked at add and at every send; all `sub` code moves to the shared module under [B.29](../TODO.md#bundled-agents)) ([RD141](../DECISIONS.md#recorded-decisions))
  - Daemon (AB): `who` is optional; without it the answer is for the caller, as today. With it, the daemon answers by Send's own admission rule for `who`, only for a target the caller can see, exactly as `/manages?who=` limits disclosure; a hidden target answers as not found. No refusal counter or activity is recorded. Docs: [daemon API § checking permission to send](../../../docs/09-daemon-api.md#checking-permission-to-send). Clients: Python and TypeScript `may_send(who, name)`. Daemon restart with the owner's OK
  - Agents, after the daemon lands: cron and `#at` (AB2, ✅ 0.9.144, commit 7114ba9a: one creator verdict before every firing and retry, per-pass circuit breaker, 85 mutants, gate green, deployed disabled), IMAP `sub` (xAB2), Slack, Telegram and logwatch `sub` (whoever is free). Add and each send check the author; a refusal at add stores nothing, a refusal at send drops that one message with one warning and the next match is delivered (B.29, owner 2026-10-29; the pause of RD140 is superseded); an unavailable check refuses rather than sends
  - Acceptance: a caller who may not send to X cannot schedule, defer or subscribe to X through any of these agents; revoking the author's access stops the next send; the caller-only form is unchanged
  - Mutants: answer for the caller instead of `who`; disclose a hidden target; skip the check at add; skip it before a send; send when the check is unavailable — each fails a check
  - Plan for the daemon and clients (AB, 2026-10-28), for review before code
    - Contract: `who` present once and nonempty selects the form; an empty or repeated `who`, or any other key, is 400; absent keeps today's caller-only form exactly. The form with `who`: a target the caller cannot see (`canSee`: manages or may), absent or inactive is 404 `no such name`, the same text for each, so hidden and absent cannot be told apart. For a visible target the answer is 200 `{"may_send":true|false}`: false when `who` is unknown or not acting (inactive), or when Send's admission refuses `who` for any reason (ACL, Personal, User-inbox rule, external kind). A bad name in `who` or `name` is 400. The reason of a refusal is not returned, as `/manages` returns none
    - Core: `Bus.MaySendAs(caller, who, to) (bool, error)` beside `Manages`: canon both, one lock hold, `entity`+`canSee` else `ErrUnknown`, then `acting(who)` and `sendTarget(who, to)`; the true/false form never touches the refusal counter, the queues or activity. The caller-only `MaySend` is untouched. `api.maySend` branches on `who`
    - Clients: Python `Bus.may_send(who, name) -> bool` and TypeScript `bus.maySend(who, name): Promise<boolean>` (`maySend`, as `manages` is `manages`) call `GET /may-send` with both; true only for `{"may_send":true}`; a 404, a refusal, a timeout or any error raises, never returns false, so an unavailable check is never read as a no
    - Docs: `docs/09-daemon-api.md` § checking permission to send: the `who` form, its disclosure limit, and the client calls; the "no `who` option" sentence is replaced
    - After ab2-review (2026-10-28): the answer echoes `who` in canonical form, `{"may_send":b,"who":w}`, and a client raises on a missing or different `who` (an older daemon ignores `who` and answers for the caller). Disclosure: the allow list is already public on a visible record (`Record.Public` keeps `Allow`), so the answer adds no new read: accepted as `/manages?who=` precedent and documented, with the rule that an agent passes only the authenticated `from`. Unknown, inactive and refused `who` answer the same bytes but the echo. A refusal counter is not moved (the `who` form answers through `dryRefusal`, not `reply`). Clients raise the daemon's 404 as its refusal and everything else as an error. More mutants: unknown who as 404; repeated `who` accepted; echo unchecked in a client; `name` used as `who`; Manages semantics; caller-only form changed
    - Tests: API: for each of allowed, ACL-refused, Personal-refused, User-inbox, external, inactive `who`, unknown `who`, the answer equals what `/send` would do for `who`; a hidden target and an absent one answer identically 404; counters, queues, recent and activity are unchanged; `who` empty or repeated is 400; the caller-only form is unchanged. Clients: against the fixture daemon, true, false, 404 raises. Mutants (⚠️ authority): answer for the caller instead of `who`; disclose a hidden target (skip `canSee`); skip `acting(who)`; record a refusal; treat an error as false in a client
