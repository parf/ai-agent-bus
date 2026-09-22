# MVP questions

📌 **TL;DR:** Settle the remaining forwarding principal choice for 0.7.

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

The [constitution](../../docs/constitution.md#-channels) owns settled forwarding
rules, including the [PubSub aggregate outcome](../../docs/constitution.md#pubsub-routing).

Settled and deferred choices remain in the
[decision index](../../docs/decisions.md#settled); their IDs remain reserved.
