# Contact routes

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

### How to reach a person

The record also carries **an ordered list of ways to reach this person, per
severity** — a warning goes one way, something that has to wake them goes
another. Each entry names a service that does the delivering
([bundled services § people and the world outside](services.md#people-and-the-world-outside))
and the address that service understands.

Beside it, **the aliases this person is known by elsewhere**: a Telegram
handle, a Slack member id, an address somebody types out of habit. They are
what lets a message addressed to any of those reach the right human
([bundled services § people and the world outside](services.md#people-and-the-world-outside)).

| | |
|---|---|
| **it is the person's, so it lives with the person** | not in an alerter's configuration. One person is reached by several alerters — one per host, one per team — and a phone that changed has to change **once**. The daemon is already where the person is |
| **a maintainer writes it, like every other field** | ([who may write a record](../../docs/01-identity-and-roles.md#users-and-profiles)) — which is what makes a destination trustworthy enough to page somebody on. Somebody who wants their evening phone in there asks for it |
| **an alias is a lookup key and never a principal** | it resolves *to* a name and the name is what travels — the same rule that keeps a provider's numeric id out of being an identity ([names](../../docs/01-identity-and-roles.md#names)). Nothing is ever authorised as `@someone` on Telegram |
| **nothing enforces it** | it is a list of preferences, not an access decision. What acts on it is `im` and the `alerter`, ordinary services reading an ordinary record ([bundled services § the bus watching itself](services.md#the-bus-watching-itself)) |

A record holding a phone number is worth protecting for reasons that have
nothing to do with the bus, which is the whole of that question.



Unresolved details: [questions](QUESTIONS.md#open-questions).
