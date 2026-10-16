# Local queue library idea

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## In process queue

Local work *inside* an agent — not a bus feature. Bounded Go channel per
named queue: `Push` non-blocking → `ErrFull` (backpressure); `Pop(ctx)`
blocking; N goroutines = consumer group. Non-durable by design; if one queue
ever needs durability, back *that one* with a WAL file. Cross-agent messages
go through channels and inboxes ([messaging](../../docs/04-messaging.md#messaging)), not these.
