# DONE — PoC

| Wave | Result |
|---|---|
| **A — two shells talk** | `agent-busd` on a unix socket and loopback HTTP, in-memory registry and inboxes; `agent-bus` with `status`, `register`, `ls`, `send`, `consume`, `reply`. `src/smoke.sh` runs the wave's acceptance: 13 checks, all passing. |

**A, what it proved**

- Both listeners speak the same HTTP+JSON; the CLI reaches either.
- The two parameters are enforced: a wrong token and a name without a realm are both refused.
- A message sent while nobody is reading waits, and arrives afterwards.
- `reply <message-id>` works with **no reply state in the daemon** — the CLI resolves the id from the envelope it consumed ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)).
- One reader per inbox: a second unfiltered `consume` is refused with 409, while a filtered waiter is served beside it ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)).

**Not done in A, on purpose**: `call`, `ack`, topics, script services, the MCP
face, any push adapter, SSH token issuance.
