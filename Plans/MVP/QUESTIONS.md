# MVP questions

📌 **TL;DR:** Settle forwarding mechanics for 0.7; other constitution decisions stand.

## Open questions

## Constitution forwarding

The [constitution](../../docs/constitution.md#forwarding-details) accepts
forwarding with destination access. These choices block
[K.15](0.7.0-TODO.md#delivery-and-release):

- Loop detection.
- Which User or Agent supplies forwarding authority, and when access is checked.
- TTL and deadline handling.
- Overflow and failure accounting.
- Sender attribution and whether `original_to` is needed.

Settled and deferred choices remain in the
[decision index](../../docs/decisions.md#settled); their IDs remain reserved.
