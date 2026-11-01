# Large bodies

📌 **TL;DR:**
- **Not needed now:** the bus carries no large message ([owner, 2026-11-01](#decided)); nothing is built.
- **If ever needed:** set up Redis, put the file there, and pass the key name in the message.
- **Today:** [IMAP large transfer](../R1.0-Release/imap-large-transfer.md#the-three-options) moves up to 25 MB in chunked calls inside its own agent, with no bus change.

## The shape

- The bus message stays small and carries only the Redis key name; the bytes never enter a queue, so the [queue flush](../../docs/04-messaging.md#durability) and the bus size caps are untouched.
- Redis is run by the host, outside the bus. Who may read a key, how long a key lives and who deletes it are not designed here; the bus [ACL](../../docs/02-access.md#acl) does not govern a Redis key, so each would be a question before any build.
- No other agent exceeds the caps today; only IMAP does, and it needs no shared mechanism.

## Decided

- **Large messages on the bus itself are not needed now** (Q285; owner, 2026-11-01). If they are ever needed, the shape above is the owner's choice. [Decisions](DECISIONS.md#recorded-decisions).
