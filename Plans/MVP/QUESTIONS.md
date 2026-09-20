# MVP questions

📌 **TL;DR:** Settle the remaining forwarding mechanics and audit-retention default for 0.7.

## Open questions

Q79–Q82, Q85 and Q88–Q91 were settled on 2026-09-20 and moved to the
[decision index](../../docs/decisions.md#settled). Q83 was withdrawn after a
publish-versus-reload wording error. Q84 was withdrawn because the existing
no-compatibility rule already requires nonconforming state to be fixed before
activation. Their IDs remain reserved, as do all earlier settled question IDs.

## Constitution forwarding

The [constitution](../../docs/constitution.md#forwarding-details) accepts
forwarding with destination access. These choices block
[K.15](0.7.0-TODO.md#delivery-and-release):

| ID | Question |
|---|---|
| Q87 | For an Agent record, is destination access checked as its owning User or as the Agent principal that reads the inbox? User and Queue records use their owning User. |

The maximum forwarding depth is one. A send whose destination would forward
again is refused before storage. Loop detection and forwarding-specific TTL or
deadline rules are settled as unnecessary and are not open choices. Access is
checked twice: invalid destinations cannot be configured, and each delivery
uses the current node state. Q87 settles which principal that check uses for an
Agent record. A forwarded envelope carries the required single-valued
`original_to` and retains the original sender. Forwarding moves one message:
the source inbox keeps no copy and changes neither `in` nor `out`; the
destination increments `in`, then `out` only when a reader receives it. Access
or inactive-state refusal rejects the original send and changes no counter.
Strict overflow does the same; ring overflow evicts the destination's oldest
message and increments that destination's own `dropped`.

## Constitution audit retention

| ID | Question | Blocks |
|---|---|---|
| Q86 | What finite retention default ships for durable audit events? The mechanism is settled and tested with an overridden bound of three; the production default still needs an owner value. | [K.14](0.7.0-TODO.md#authority-and-lifecycle) |

Settled and deferred choices remain in the
[decision index](../../docs/decisions.md#settled); their IDs remain reserved.
