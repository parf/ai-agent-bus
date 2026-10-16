# Agent-to-agent credentials

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Agent to agent

**An agent that holds a credential may ask for one scoped to the agent it
is about to call** — over the bus, with the token it already has. No key, no
sshd, and nobody at a keyboard. It is the same `token <agent>` request as
[token scope](../../docs/02-access.md#what-a-call-carries), reached from inside the bus instead of over SSH.

| | |
|---|---|
| **it is A as A** | not delegation. **on-behalf-of** is a claim A adds when it is acting for a *person* ([identity § delegation](../R1.1/identity.md#delegation)); this is A calling B as itself, which most agents calling other agents are doing. The two compose and neither implies the other |
| **asking is not access** | the target's `allow` decides the call, as it always did ([identity § acl](../../docs/02-access.md#acl)). What the narrow token buys is the other thing: an agent you called cannot replay your credential somewhere else ([token scope](../../docs/02-access.md#what-a-call-carries)) — that is the whole reason a token is per destination in the first place |
| **it does not solve the first one** | an agent still gets its *initial* credential the way it does today — the runner collects it for an agent it started, or a person does. This is every hop after that, which is most of them |
| **nothing new about the token itself** | follows the common [token lifetime](../../docs/02-access.md#token-lifetime) |

⚠️ **An agent on a host with no runner and nobody to ask is still open.** It
is the gap that makes a key of an agent's own worth considering at all
([1.2 § why an agent would want a key of its own](../R1.3/exploration.md#why-an-agent-would-want-a-key-of-its-own)),
and it is the *first* credential, never the ones after it.
