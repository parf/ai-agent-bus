# MVP questions

📌 **TL;DR:** Two open questions: whether the daemon parses a service secret, and whether an external service may be sent to over this bus.

## Open questions

| ID | Question |
|---|---|
| Q77 | Whether shell `KEY=value` is a caller convention over bytes the daemon stores opaquely, or a grammar the daemon checks. Opaque is how [configuration](../../docs/03-services-and-topics.md#configuring-a-template) already behaves and is the cheaper promise; checking means defining blank lines, comments, `export`, duplicate keys and invalid identifiers. Needed before [J.6](0.6.0-TODO.md#remaining-work) |
| Q78 | Whether a `service` record may be sent to, subscribed by and consumed from like any other. The [five kinds](../../docs/03-services-and-topics.md#five-record-kinds) say a service is external and that nothing here answers for it, but the daemon still gives it a queue and delivers to it: `Send`, `fanout` and `ConsumeAs` ask the ACL and not the kind. Refusing would make the documented meaning enforced and would also decide what the web page shows a service; allowing keeps one delivery rule for every record. Raised by codex reviewing 0.6.3; not implemented either way |

Settled and deferred choices are in the [decision index](../../docs/decisions.md#settled); their IDs remain reserved.
