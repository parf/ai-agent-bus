# Discord implementation

The [current guide](../../../docs/agents/discord.md#discord) owns user behaviour.

- `api.ts` reads credentials through a callback, serializes requests under a deadline, bounds replies and stops invalid-request loops. URL overrides accept loopback fixtures only.
- `gateway.ts` owns hello, intents, heartbeat ACKs, session resume, fatal refusals and explicit reload. No inbound HTTP endpoint runs in the agent.
- `app.ts` serializes calls and events. The shared TypeScript `Subscriptions` owns all subscription policy; `store.ts` adapts KV and bus transport only. The shared `Router` owns reply/tag routing. Split reply parts retain their root mapping in the record's KV.
- `discord.ts` loads Configuration and Secret from its own authenticated record. The process environment supplies bus connection information, never the Discord token.
- `package.sh` emits one Bun bundle, manifest, Secret declaration, license and notices. Run `bun test src/agents/discord` from the checkout; it uses only fake providers and private buses. Fast checks are in `fast.test.ts`, with no server, process or wait.
