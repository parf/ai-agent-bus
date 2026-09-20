# MVP questions

📌 **TL;DR:** Settle the remaining forwarding mechanics and audit-retention default for 0.7.

## Open questions

Q79–Q82 were settled on 2026-09-20 and moved to the
[decision index](../../docs/decisions.md#settled). Q83 was withdrawn after a
publish-versus-reload wording error. Q84 was withdrawn because the existing
no-compatibility rule already requires nonconforming state to be fixed before
activation. Their IDs remain reserved, as do all earlier settled question IDs.

## Constitution forwarding

The [constitution](../../docs/constitution.md#forwarding-details) accepts
forwarding with destination access. These choices block
[K.15](0.7.0-TODO.md#delivery-and-release):

- Loop detection.
- Which User or Agent supplies forwarding authority, and when access is checked.
- TTL and deadline handling.
- Overflow and failure accounting.
- Sender attribution and whether `original_to` is needed.
- Q85: when a destination would forward a second time, who receives the explicit
  error? The answer must say whether the original send is refused before any
  message is stored or the error is routed elsewhere.

The maximum forwarding depth is one. Loop detection and forwarding-specific TTL
or deadline rules are settled as unnecessary and are not open choices.

## Constitution audit retention

| ID | Question | Blocks |
|---|---|---|
| Q86 | What finite retention default ships for durable audit events? The mechanism is settled and tested with an overridden bound of three; the production default still needs an owner value. | [K.14](0.7.0-TODO.md#authority-and-lifecycle) |

Settled and deferred choices remain in the
[decision index](../../docs/decisions.md#settled); their IDs remain reserved.
