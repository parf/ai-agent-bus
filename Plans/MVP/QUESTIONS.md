# MVP questions

📌 **TL;DR:** One open question: whether the daemon parses a service secret.

## Open questions

| ID | Question |
|---|---|
| Q77 | Whether shell `KEY=value` is a caller convention over bytes the daemon stores opaquely, or a grammar the daemon checks. Opaque is how [configuration](../../docs/03-services-and-topics.md#configuring-a-template) already behaves and is the cheaper promise; checking means defining blank lines, comments, `export`, duplicate keys and invalid identifiers. Needed before [J.6](0.6.0-TODO.md#remaining-work) |

Settled and deferred choices are in the [decision index](../../docs/decisions.md#settled); their IDs remain reserved.
