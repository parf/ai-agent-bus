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
three fractional digits and Z. Counts are non-negative safe integers.
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
Counters are persisted best effort after the outcome; a failed counter write
warns `store_unknown` and can leave durable counts behind actual deliveries,
but never causes an entry replay. The delivery result reports known outcomes.

Eventlog's forwarding budget is at most 15 seconds, with at most eight bounded
workers, about one second per subscription and every worker joined/reaped.
A write with selector candidates and less than three seconds left is refused
before storage; no-candidate writes have no artificial forwarding floor. A
shorter admitted deadline shortens the batch. Matching has a one-second total
budget and 50 ms per pattern; one pathological pattern drops only its copy.
Python may use private process groups to make DNS/regex/pipe deadlines hard;
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
