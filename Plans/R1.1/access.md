# Service to service credentials

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Service to service

**A service that holds a credential may ask for one scoped to the service it
is about to call** — over the bus, with the token it already has. No key, no
sshd, and nobody at a keyboard. It is the same `token <service>` request as
[token scope](../../docs/02-access.md#what-a-call-carries), reached from inside the bus instead of over SSH.

| | |
|---|---|
| **it is A as A** | not delegation. **on-behalf-of** is a claim A adds when it is acting for a *person* ([identity § delegation](../R1.0-Release/identity.md#delegation)); this is A calling B as itself, which most services calling other services are doing. The two compose and neither implies the other |
| **asking is not access** | the target's `allow` decides the call, as it always did ([identity § acl](../../docs/02-access.md#acl)). What the narrow token buys is the other thing: a service you called cannot replay your credential somewhere else ([token scope](../../docs/02-access.md#what-a-call-carries)) — that is the whole reason a token is per service in the first place |
| **it does not solve the first one** | a service still gets its *initial* credential the way it does today — the runner collects it for a service it started, or a person does. This is every hop after that, which is most of them |
| **nothing new about the token itself** | follows the common [token lifetime](../../docs/02-access.md#token-lifetime) |

⚠️ **A service on a host with no runner and nobody to ask is still open.** It
is the gap that makes a key of a service's own worth considering at all
([1.2 § why a service would want a key of its own](../R1.2/exploration.md#why-a-service-would-want-a-key-of-its-own)),
and it is the *first* credential, never the ones after it.
