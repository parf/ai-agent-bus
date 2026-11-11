---
description: Send a message to someone on the agent bus, and wait for the answer
---

The argument is a receiver name followed by the message: `$ARGUMENTS`

1. `ab_send` it, with a `topic` and `tag` you make up and have not used before
   in this session — the reply is matched on both.
2. `ab_consume` with that same topic and tag to collect the answer. A filtered
   wait is served even when push holds the inbox.
3. Show the answer. If nothing came back before the deadline, say that the
   message was accepted by the bus but not yet answered — do not resend it.

If the receiver's exact name is not obvious, run `ab_ls` first and match it.
