# MCP Resources

Status: **built in 0.8.70.** The contract is
[Resource records](../../docs/03-records.md#resource-records); the design
decisions are in [R1 decisions](DECISIONS.md#recorded-decisions).

## Resource records

What remains here, not built:

| | |
|---|---|
| Web view | the cards in the web face, with 📄 and 🧩 (in progress) |
| Latest-spec typing | `ttlMs`, `cacheScope` and `resource_link` pass through untyped until the face's SDK speaks 2026-07-28 ([K.37](../R0.8-MVP/TODO.md#constitution-conformance)) |
| Private addresses | whether the face refuses loopback and private sources ([Q141](../R0.8-MVP/QUESTIONS.md#open-questions)) |
| Subscriptions | `subscriptions/listen` and `notifications/resources/updated` are not part of this; a change fan-out is what a 📣 PubSub already is |
