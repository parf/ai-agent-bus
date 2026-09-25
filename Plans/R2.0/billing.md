# Billing role — future

**Deferred.** Nothing in the bus depends on billing, and it needs a RADIUS
server and a payment provider before it is worth anything, so it is not
scheduled into a stage ([stages § R1](../R1/README.md#scope)). The
design is kept whole here so it does not have to be rediscovered: turning it
on is adding a child process, not reopening the design.

Optional child process of `agent-busd` (`billing: on`). Minimal by intent: the
bus counts, RADIUS holds the balance, a payment provider holds the money.
Together they make agent-bus a **paid public API platform** — a stranger
**registers**, **pays** and **uses services** through the bus's API. A web site
in front of it is somebody's ordinary web site, not part of agent-bus.

| Aspect | Rule |
|---|---|
| Price | declared by the **service** in its record: **flat** (a fee for access) or **per call** (the service sets the cost) |
| Balance | lives in a **RADIUS** server on **our own network** — the bus asks it *"may this principal call this service?"*, caches the answer and refreshes it independently of [token lifetime](../../docs/02-access.md#token-lifetime); accounting records go the same way. The standard client, not a RADIUS stack of our own ([modules § external tools](../../docs/10-modules.md#external-tools)). Never exposed to the public side; the bus is its only client |
| Usage | the bus records every billable call per (principal, service) and reports it to the balance service; the dashboard shows the counts it already keeps ([discovery § dashboard](../../docs/05-discovery.md#dashboard)) |
| Denied | **no balance = call denied**, with a clear error, like a missing role |
| What is billable | what the bus can see — a message and its size ([messaging § envelope](../../docs/04-messaging.md#envelope)) |
| Sign-up | a service with `allow: *` ([identity § acl](../../docs/02-access.md#acl)); the newcomer's first call enrols them. A web site may call it on the user's behalf — that site is not agent-bus |
| Pay | the **payment gateway is a citizen of the bus** — a service like any other, talking to a provider and topping up the principal's RADIUS balance. The core never handles money; it routes to the service that does |
| Money | never in the core: no currency, no invoices, no card data — the `pay` service and the provider behind it own that; the core owns counting and denying |

Unresolved details: [questions](QUESTIONS.md#open-questions).
