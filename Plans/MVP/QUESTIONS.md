# MVP questions

📌 **TL;DR:** Settle the one remaining forwarding choice for 0.7.

## Open questions

Q79–Q82, Q85 and Q88–Q91 were settled on 2026-09-20 and moved to the
[decision index](../../docs/decisions.md#settled). Q83 was withdrawn after a
publish-versus-reload wording error. Q84 was withdrawn because the existing
no-compatibility rule already requires nonconforming state to be fixed before
activation. Q86, Q92, and Q93 were withdrawn when the owner limited the request
to logging entity edits rather than an audit subsystem. Their IDs remain
reserved, as do all earlier settled question IDs.

## Constitution forwarding

The [constitution](../../docs/constitution.md#forwarding-details) accepts
forwarding with destination access. These choices block
[K.15](0.7.0-TODO.md#delivery-and-release):

| ID | Question |
|---|---|
| Q87 | For an Agent record, does the right that makes a forwarding route valid come from its owning User or from the Agent principal that reads its inbox? User and Queue routes use their owning User. Delivery separately checks the original sender under the destination's rules. |

The maximum forwarding depth is one. A send whose destination would forward
again is refused before storage. Loop detection and forwarding-specific TTL or
deadline rules are settled as unnecessary and are not open choices. A route is
storable and usable only while Q87's principal may write to its destination.
The route grants nothing: each delivery applies the destination's current rules
to the original sender. A forwarded envelope carries the required single-valued
`original_to` and retains the original sender. Forwarding moves one message:
the source inbox keeps no copy and changes neither `in` nor `out`; the
destination increments `in`, then `out` only when a reader receives it. Access
or inactive-state refusal rejects the original send and changes no counter.
Strict overflow does the same; ring overflow evicts the destination's oldest
message and increments that destination's own `dropped`.

Settled and deferred choices remain in the
[decision index](../../docs/decisions.md#settled); their IDs remain reserved.
