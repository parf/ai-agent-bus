# Shared subscription implementation contract

The Python engine and TypeScript twin implement the same pure-operation JSON
vectors in `src/testdata/subscriptions.json`. Agent adapters validate their
method roles and agent-specific selector before invoking the engine. A selector
and its matching event remain agent-specific; admission, storage, filtering,
visibility, delivery accounting and failure behaviour do not.

## API to implement

`Subscriptions(bus, source, *, validate_selector, selector_matches, warning,
clock, id_factory, max_subscriptions=100, max_per_creator=20, delivery=None)`

- `add(message, target, selector, filter='') -> row`: validate before network;
  canonical queue/agent destination; creator comes only from the authenticated
  envelope. Check creator→target, then creator→source. Persist `sub.<id>` with
  mode add. A collision tries another injected id, at most three attempts.
- `list(message) -> list[row]`: creator's rows, or all for this source's direct
  Owner/Maintainers. Forwarded roles never grant manager visibility.
- `delete(message, id) -> bool`: creator or direct source Owner/Maintainers;
  delete storage before changing memory. Invalid/damaged rows are repairable
  only by the source's direct Owner/Maintainers, never silently removed.
- `deliver(event, text, body, *, topic, tag, deadline) -> {matched,sent,dropped}`:
  selector first, bounded regex second, then one checked send per match. No
  history is scanned or replayed. Complete/kill/reap all delivery workers before
  returning; no send task can start after this call returns.
- `close()`: terminate/reap any owned helpers; no registry operation.

An agent adapter checks its own application roles before calling add/list/delete.
The engine's manager rule uses only `message.roles` from the authenticated
message whose original_to is absent, empty or the source itself. Bodies never
supply author, roles, counters or creation time.

## Stored row and errors

Stored row fields: `id`, `target`, `selector`, `filter`, `created_by`,
`created_at`, `sent`, `dropped`. Ids are 12 lower-case hex characters;
clock and id generation are injectable. Creation time is UTC RFC3339 with
three fractional digits and Z. Counts are non-negative integers at most 2^53-1 and saturate there (never wrap). Clock seconds are floored to milliseconds, not rounded.
A visible invalid stored filter adds `reason: invalid_filter` to the LISTED
row, without rewriting it to match-all or hiding/deleting it.

Keep `sub.<id>` keys. Agent migrations map their old selector fields into
selector, preserve creator and valid counts, and remove obsolete pause/disabled
and held-copy retry state. Unsupported filters remain visible and inert with a
fixed reason/warning; only deletion/re-add repairs them. Migration never turns
an unsupported filter into match-all.

`SubscriptionError.code` is stable; prose is not the parity contract. Initial
codes: `invalid_target`, `invalid_selector`, `invalid_filter`, `invalid_id`,
`limit`, `not_found`, `not_author`, `admission_refused`, `admission_unavailable`,
`store_unknown`. State is reloaded after an uncertain KV mutation; no send is
retried to repair a counter write. Delivery warnings use `admission_refused`,
`admission_unavailable`, `filter_invalid`, `filter_timeout`, `send_refused`,
`send_unknown`, `deadline`, `store_unknown`, without event text or credentials.

## Ordering, budgets and failures

Rows are processed/reduced in id order. For one row, target admission precedes
source admission; an unsuccessful first check short-circuits the second.
Cache repeated (creator,target) questions within one bounded batch, including
creator→source; refresh for every new entry. A 404 is a refusal, an unexpected
or unavailable answer is unavailable, never true. Either failure counts one
dropped copy and one warning, not two. Check destination kind at creation and
before sending; it must still be queue or agent.

One matched copy has one send attempt. Refusal, transport failure, unknown
acceptance or deadline drops/counts that copy, warns and leaves the subscription
active for the next entry. Dropped INCLUDES copies whose acceptance is unknown
and which may have reached the inbox. No pause/retry/outbox/cursor/backlog.
Counter/KV writes occur on the calling thread after every worker has joined, never in a worker: at most one row write per subscription per entry. A failed counter write warns once per batch (not once per row). Counters are persisted best effort after the outcome; a failed counter write
warns `store_unknown` and can leave durable counts behind actual deliveries,
but never causes an entry replay. The delivery result reports known outcomes.

Eventlog's forwarding budget is at most 15 seconds, with at most eight bounded
workers, about one second per subscription and every worker joined/reaped. Python send-only Bus clients do not register/unregister: identity is GET /status; each is bounded inside a private process group to cover resolver stalls too.
A write with selector candidates and less than three seconds left is refused
before storage; no-candidate writes have no artificial forwarding floor. A
shorter admitted deadline shortens the batch. Matching has a one-second total
budget and 50 ms per pattern; one pathological pattern drops only its copy.
Python uses private process groups to make DNS/regex/pipe deadlines hard;
helpers never register a bus name, tokens never appear in argv/stdout/stderr,
and send helpers use only the source's own connection credentials. Transport
acceptance after a timeout remains unknown. The TS twin uses bounded workers
and abortable requests. Timing is checked by owning runtime tests, not vectors.

## Common filter grammar

Empty means all. A bare pattern or `/pattern/flags` uses the same grammar; flags
are i m s u once each, never g/y. Pattern length at most 200 Unicode scalar
characters; input text at most 8192 UTF-8 bytes. NUL/control patterns refused.
Ordinary/noncapturing groups, alternation, anchors, literals, ASCII character
classes, dot, and bounded ordinary quantifiers are supported. Refuse lookaround,
named groups, backreferences, inline flags, possessive/stacked quantifiers,
class set operations, Unicode property classes and non-ASCII classes.
Explicit d/w/s escapes follow the existing IMAP common JS/Python subset (JS
ASCII digit/word and explicit JS whitespace); never Python's Unicode-default
classes. `\Z` and engine-specific escapes are refused. Astral/no-u, Unicode
case folding, newline/anchor and empty-match cases are mandatory vectors.
A refused old stored pattern is inert, not a broadened selector.

### Exact regex compatibility

The semantics are ECMAScript's common subset, not Python re defaults. Without
u, pattern/text are viewed as UTF-16 code units; with u they are Unicode scalar
values. Lone surrogates are refused as invalid_text (or invalid_filter in a
pattern). Dot without s excludes LF, CR, U+2028 and U+2029; m anchors recognize
all four. Without m, $ is end-of-input only, never before a trailing newline.
Case-insensitive legacy mode uses ECMAScript Canonicalize (single-unit uppercase,
never folding a non-ASCII unit into ASCII); iu uses Unicode simple folding,
not full multi-character casefolding. Thus é/É can match in legacy i, but Kelvin
sign/long-S never fold into ASCII without u, and dotted/dotless I never do.
Python translates literals, classes, dot and anchors explicitly and canonicalizes
pattern/text; it never applies re.IGNORECASE to a caller pattern.

Split a delimited filter after the LAST UNESCAPED slash; /a/b/ means pattern
a/b. A bare pattern can contain slash. Supported escapes: d D w W s S, n r t,
and escaped punctuation `\ . / - ^ $ | ? * + ( ) { } [ ]`. b/B, 0, x/u/c/k/Z
and every other escape are refused. Escaped hyphen is normalized to a literal hyphen outside a class, including u mode; // and //i mean match-all. NUL, DEL and C0/C1 controls in patterns are
refused; escaped n/r/t remain valid. Spaces are literal, not stripped. Quantifier
bounds are at most100, at most20 quantifiers; lazy, possessive and stacked forms
are refused. Empty alternation is allowed. Nested quantified groups are refused;
other backtracking remains bounded by the per-pattern/helper timers. Limits and
syntax are checked before may-send/KV. Unsupported stored patterns are inert,
visible with reason invalid_filter, and warn, never silently converted.

Pure vectors check exact queries (who,target), not just a queue of booleans.
Deadline already expired means no admission query, one deadline drop per selector
candidate. A kind that changed to pubsub gives admission_refused and no send.
Per-batch cache keys include both creator and target and expire with that batch.

Invalid delivery match-text (non-string, >8192 UTF-8 bytes or lone surrogate) drops every selector candidate with invalid_text, including match-all rows; no query/send is attempted. The empty filter never bypasses UTF-8/size checks.

Differential simple-fold vectors are selected C+S mappings from
[Unicode 16.0.0 CaseFolding.txt](https://www.unicode.org/Public/16.0.0/ucd/CaseFolding.txt),
including titlecase, Greek extended and Cherokee. The parity contract assumes
those Unicode 16.0.0 mappings, not a runtime's full-casefold expansion. Refuse
classes with multiple hyphens (such as [a-c-e]) in both modes; a lone leading or
trailing class hyphen remains literal. Open upper bounds {n,} remain unbounded
like * and +; every explicit numeric bound is <=100. Lazy quantifiers, a
quantifier after a group containing a quantifier, and lone { } ] are refused.
