# IMAP subscriptions (B.21)

## Result

Implemented in shared **0.9.150**. Source **`6449f674`**, verified on main; manager deployed the IMAP package.
Current contract: [IMAP guide](../../../docs/agents/imap.md#what-it-does).

- Single-process 60-second polling, new-mail-only subscription baselines,
  sender/subject filters and creator provenance. No subscription means no
  background header/body fetch. IDLE is an optional future optimisation.
- Add and every send check the author's may-send permission; the receiver's ACL
  also admits the IMAP instance. No authority fallback. Creator/own managers
  may delete; unconfirmed sends and ACL denials pause visibly.
- Bounded durable ledgers and shared private outbox, per-subscription intake
  backpressure, 12 definite-refusal attempts, visible skipped UID ranges,
  restart-safe discovery/confirmation ordering and explicit delete+add recovery.
- Compact decoded Date/From/Subject summaries and descending manual history,
  preserving cached bodies and legacy retry/cursor on migration. Manual history
  never publishes or advances the forwarding frontier.
- Safe expired/timeout/error-class logging. The historical lost first reply was
  not retained, so deadline expiry remains a hypothesis rather than a diagnosis.

## Verification

Final `src/smoke.sh --normal`: **1214 passed, 0 failed, 45.347 s**, under the
slow lock, released before deployment. Earlier full runs were
1212/2 (stale generated index/old acceptance method set), then 1213/1 (standalone
local docs check bytecode was mistaken for an agent by existing enumeration).
Both causes were reproduced and corrected; no runtime change was needed.
An initial pre-gate build lacked local dependency links and ran no shards.

**67 compiling assertion-red mutants**, source restored: 30 timer/matching/
progress primitives; 35 ledger/outbox/delivery/provenance/subscription/session/
poll/entry/history mutations; 2 isolated runner/bwrap integration mutations.
A missing poll tick is caught by an immediate status assertion, not timeout
credit. Two initially surviving driver mutations prompted direct commit-order
and unknown-reference-sweep assertions before they were killed. Error-only
mutant runs were not credited; caught-exception type assertions made the final
throttle and intake checks fail on assertions.

Fixture evidence includes the full 176-test IMAP suite before the final error/
test additions; final gate runs include the expanded suite. The actual exported
package runs through isolated runner keeper start/restart controls, the real
fixture daemon/KV and a verified local TLS server: one new-only forward,
creator provenance, one body FETCH, no duplicate after confirmed restart.
This control fixture has sandbox off, with no cgroup, user unit, host journal
or real account. A separate actual bwrap+seccomp backend proves both hardlink
and forced-EPERM fallback publication. It is not a deployed sandbox-on control
path claim. Documentation link checks: **bad 0**.

## Review and limits

`#claude/ab2-review` closed source/metadata on **`6449f674`**, independently
verifying main ancestry, VERSION and unchanged runtime blobs from `6c1be64d`. Static reads covered each runtime layer and its fix deltas,
plus the guide's headings/first sections. Tests, integration/bwrap fixtures,
setup inventories, most guide details and mutant/gate results were not reviewed
or run by the reviewer; all verification numbers above are implementer evidence.
Final acceptance/index-only correction `6449f674` changes no runtime module.

Manager deployment report (2026-10-28): old package backed up to
`/var/lib/agent-bus/agent-backup/imap-20261028-224306`, new package swapped
atomically and byte-matched to the build (`imap.py` 0755), runner reloaded.
Eprofit runs the new code; help has sub add/list/delete, sub list reports no
subscriptions/manual cache only with zero counters, and compact list works.
Other IMAP instances remain disabled; no subscription was added. Manager's
post-deployment check: **6/6**. These host checks are manager-reported evidence;
the implementer did not independently inspect account or installed files.
No live IMAP/Gmail, Google token exchange, live forwarding, host service change
or git push was performed by the implementer. Real automatic forwarding, destination ACL/author revocation in the deployed
runner, and provider edge cases including Google XOAUTH2 remain unexercised;
they need the manager-controlled live check. No automatic forwarding is authorised until the owner adds a sub.

## Approved plan history

<details><summary>B.21 plan and refinements, retained as history</summary>

- ⏳ B.21 IMAP forwards new mail by itself ([RD137](../DECISIONS.md#recorded-decisions)) — [defaults](../agents.md#mail-clients); xAB2
  - Acceptance: a fixture mail arriving after enable reaches each matching subscription's queue or agent exactly once; `sub add/list/delete` behave as in Slack and Telegram, delivery is admitted by the destination's own ACL for `#imap/<instance>` (RD140), an ACL refusal pauses the subscription; mail present before enable is never forwarded; a restart neither loses nor repeats a forward; no STORE or flag change reaches the server
  - Mutant: forward the existing mailbox, forward twice after a restart, match the wrong subscription, resend after an ACL refusal instead of pausing, or set `\Seen`; a check must fail
  - Amended implementation plan (xAB2, 2026-10-28; H1–H5 and M6–M12 accepted with the M9 qualification below; review before code):
    - **Separate landings.** First investigate the queued-call/reload error with safe logs and a fixture expiring envelope (deadline expiry is still a hypothesis). Next matching/shared vectors and state primitives, then subscription/authority/delivery calls under a fixture hook, then the poll loop. Compact Date/From/Subject summaries and newest-first manual sync are independent reviewed landings, not prerequisites to forwarding; each has its own tests/bump when behaviour changes. Matching/state primitives are reviewed before watcher integration; B.21 is not one all-or-nothing patch.
    - **Authority ([RD140](../DECISIONS.md#recorded-decisions)).** No daemon change and no authority walk in the agent: forwards are sent under `#imap/<instance>`'s own name and the destination's ordinary ACL admits or refuses them (`*` counts). Adding a subscription needs admission to this IMAP agent; deleting needs the creator or the IMAP Owner/Maintainers. An ACL refusal at send pauses that subscription visibly (no retry). Each forward names the subscription's creator. No Configuration routes; manual `publish` keeps its existing target rule.
    - **Backpressure (H1).** Cap each subscription at 64 pending deliveries and 16 MiB of retained raw mail references/bodies; a full one pauses with reason `backlog`, holds its bounded existing pending set, and counts later matching mail skipped for it, including first/last UID. Healthy subscriptions continue. All paused states count later matches without retaining their bodies solely for that subscription. Global spool safety cap 64 MiB/1024 bodies reports degraded state; reserve per-pass work fairly and do not let one destination exhaust the common pool. Never evict pending mail silently. Oversize publication/fetch failure is a visible counted skip with UID/reason (M10), no invented notice-body format.
    - **Delivery (RD138, H3).** Persist distinct per-delivery states: pending; sending written immediately before each attempt; retrying with attempt count and next due after a definite refusal; sent; paused-unconfirmed. Restart resumes retrying but converts sending to paused-unconfirmed, never resending it. Only a parsed daemon refusal proving no acceptance is definite (known Refusal 4xx); transport/timeout/5xx/malformed response after send is uncertain. Confirmed means bus acceptance, not a stronger durability guarantee than the bus traffic-flush contract. Keep operator grammar add/list/delete: `sub list` shows pause/error, retained pending range/count and skipped matching range/count. Delete cancels held unsent work; delete+add is a fresh subscription starting at current UIDNEXT, skipping mail received while paused. State that warning in help and guide; no implicit resume or new verb. Temporary refusals get at most 12 attempts, 10 s doubling to a 5 min cap, then a counted refused skip with UID range; missing destination 404 is an immediate counted skip. ACL 401/403 is a sticky delivery pause without retry (manager refinement of RD138, 2026-10-28). Backlog/regex blocks intake only; held work still drains, and backlog intake clears when retained work reaches zero.
    - **Single-process polling first (manager approved 2026-10-28).** Interleave bus consume(wait<=10 s) and bounded 60 s mailbox polls in the main thread, with one kept read-only IMAP connection. Reconnect only as needed, never DNS per poll. No watcher child, IPC, child supervision or child spool protocol in this landing. One process owns the cache; release its lock between bounded operations, and never hold it across bus consume or the 60 s poll wait. A private bounded delivery outbox retains bodies referenced by pending sends; it is not a second cache writer. Keep alarm/socket deadlines for IMAP operations; resolver getaddrinfo may outlive a signal, so DNS is best-effort bounded by the resolver and deadline, not claimed hard-bounded. IDLE/killable child is a separate optional reviewed optimisation if the owner requires seconds-level latency. A fixture alternates manual sync and discovery to prove neither starves the other.
    - **Frontiers and migration (H5).** Separate account/mailbox identity and UIDVALIDITY from manual and forwarding progress. Validate every persisted field, integer range, retry shape and bound; accept the old identity/validity/cursor/retry cache state and migrate atomically under its existing lock. Preserve bodies and the pending UID's retry count. UIDVALIDITY/account changes reset all corresponding frontiers, retry and delivery state, visibly, before processing new mail.

      | Value | Invariant |
      |---|---|
      | B | UIDNEXT captured durably at first forwarding observation; UIDs below B are never automatically forwarded |
      | O | Next UID discovery position; advances only after bounded matching/delivery work is durably recorded; restart resumes O rather than resetting to current UIDNEXT |
      | H | Manual history's descending backfill position; walks older mail newest-first below its snapshot ceiling, independent of O; cached UIDs are skipped without FETCH |
      | S | Subscription add-time UIDNEXT, at least B; eligibility requires UID >= S, so a new subscription cannot replay old/pending mail |
      | retry | Manual FETCH refusal's UID and count survives migration/restart; distinct from forwarding retrying/delivery state |

    - Migration marks the former ascending cursor as legacy progress, retains its pending retry, and scans newest-first for uncached historical UIDs without treating cursor as proof of cached coverage; evicted/deleted cache entries are not delivery confirmations. New arrivals are handled by O independently. The manual history snapshot renews after a completed pass so explicit sync can also catch newer uncached UIDs; tests cover holes, sparse UID ranges, cached pending UID, restart, exact limit and no repeat FETCH while cached. The state implementation review will pin this before code integration.
    - **Matching/grammar (M6–M7, L13).** Text filter is the rest of line verbatim, not shlex; exact JSON arguments, target parsed as a bus name before mutation. Missing/empty filter means all new mail in this account, stated loudly. Pattern <=200 characters, flags only i/m/s/u, no g/y. Use an explicitly documented common JS/Python subset: reject inline flags, named groups, lookbehind, atomic groups, possessive quantifiers and backreferences; pin character-class/end-anchor differences with one shared vector table run by Python and TS tests, without changing existing TG/Slack accepted grammar. Bound raw header bytes before RFC2047 decoding; normalise decoded single From addr-spec (domain lowercase, no display name) plus NFC subject, flatten controls, refuse ambiguous/malformed multiple-From matching rather than guess; matched text <=1536 UTF-8 bytes. Run regex in the parent's main thread with SIGALRM <=50 ms per pair and <=200 ms per processing slice; exhaustion defers remaining work, per-filter timeout persistently disables only that subscription, visible in list. Restore nested deadline timers correctly.
    - **Polling/reconnect (M8, M11 simplified).** Fixed 60 s poll with bounded explicit O:UIDNEXT-1 windows; reject returned UIDs outside that range (never unfiltered n:*). At most 100 arrivals/min, backlog visible, remaining work yields to bus calls and delivery fairness. Keep one connection and heartbeat through the next read-only poll; reconnect on error, refresh XOAUTH2 once per reconnect, and use auth/throttle/refresh backoff >=15 min doubling to 1 h. Zero subscriptions performs no background header/body fetch; manual caching remains available, with at most a cheap UIDNEXT baseline check. Deferring IDLE is approved; zero-subscription no-fetch behavior is also manager approved (2026-10-28). No new Configuration switch is invented. All server operations remain read-only EXAMINE/BODY.PEEK/SEARCH/STATUS/NOOP, never SELECT/STORE/FLAGS/EXPUNGE/COPY/APPEND.
    - Follow-up, not required for B.21: optional IDLE for lower latency, with a separate reviewed design and tests if the owner requests it. The first forwarding landing accepts up to one-minute poll latency.
    - **Retained body accounting (M9/M12).** With subscriptions discover bounded FROM/SUBJECT headers first; reuse the cached body if present, fetch once only when a subscription matches. One immutable raw body per identity/UIDVALIDITY/UID is stored globally, with per-sub references; its bytes count once globally and once against every referencing subscription's 16 MiB allowance. Per-sub 64-item and global 64 MiB/1024-body limits apply before retaining. A round-robin subscription pass allocates each prospective new body fairly; if adding it would exceed the global cap, pause only subscriptions requiring that new retention, count that UID as skipped for them and continue discovery/healthy sends (never mailbox-wide stop). A freed healthy delivery releases its reference immediately, and the last reference frees the body. No pending body is silently evicted; tests fill the global cap with failing targets while a healthy target continues after freeing capacity, with explicit counted skips if capacity truly cannot fit its mail. Private outbox under AGENT_BUS_WORK uses 0700/0600, nofollow, atomic rename/fsync and bounded cleanup.
    - **Replay and compact state.** Creating a delivery for an already-known (identity, UIDVALIDITY, UID, subscription) is a no-op for every state, including pending/sending/retrying/sent/paused/skipped. Retain terminal progress at least until O has advanced beyond that UID; compact only behind the durable O frontier, keeping bounded holes/ranges. Crash between two subscriptions' writes and O advance must produce the missing entry once and leave existing entries untouched, including skip counters. Each sub value <=96 KiB (<128 KiB KV cap), <=100 subscriptions, no per-mail KV name; reject growth before the cap, never reinterpret a KV error as missing. Max 100 arrivals/min x100 matches =10,000 attempts and at most20,000 marker+result writes/min before batching; do not issue them as a burst. Scheduler caps 100 send attempts/min globally round-robin (at most200 marker/result writes/min plus batched discovery/progress); at most one durable sending marker before each attempt, batch completion/refusal and discovery bookkeeping where safe without moving O ahead of recorded entries. Lasting demand beyond drain rate triggers counted per-sub backlog pauses, not unbounded writes.
    - **Mailbox reset/eligibility.** On identity/UIDVALIDITY change retain subscriptions but discard old-mailbox pending evidence/bodies and report counts of held/discarded/uncertain entries in sub list/status; retain cumulative loss counters. A previously paused-unconfirmed sub is rebaselined to the new mailbox with an explicit reset note, not silently treated as delivered. New baseline and per-sub S are persisted before discovery resumes. Moved-in mail gets a new UID and is new for forwarding; a body already fetched may still be forwarded after server expunge/\Deleted because this is cached external content, without changing flags.
    - **Nested timers.** One tested main-thread deadline context helper saves the outer remaining deadline, uses the earlier inner/outer expiry, and restores remaining outer time minus elapsed on all exits; never disables or extends the outer timer. Test regex timeout under operation deadline, successful inner scope, exceptions, elapsed outer expiry and handler restoration; mutants disable outer or restore its full original interval. Matching's 200 ms slice budget yields without classifying deferred subscriptions as non-matches.
    - **Verification (all reviewer test pins accepted).** Owning assertion-red mutants for backlog/restart/permission/kind/fire-time revocation/forwarded-role laundering, one dead target versus healthy, separate retrying/sending restart, uncertain response pause, delete cancellation, literal/aggregate/PEEK/read-only limits, UIDVALIDITY reset, state compatibility/migration, outbox cap/reference accounting/modes/symlinks/cleanup, discovery replay between subscription writes, regex/timer isolation, poll/search-star quirk/auth backoff/token reconnect/flood fairness, concurrent manual sync, list LRU/compact decoded fields and newest-first no-refetch. Fixtures only, no real accounts/provider calls; fair smoke edit/mutant lock then final `--normal`, release before deploy. Each small landing uses a coordinated fresh bump/changelog, source review, links and IMAP-only package byte match/check-deploy. Preserve account Configuration and env files; manager handles the IMAP-only deployment, leaving imap/eprofit enabled as it is with no subscriptions (latest manager .150 instruction, 2026-10-28); zero subscriptions means no background header/body fetch, and no live subscriptions/forwarding without separate owner OK. Manager pushes.


</details>
