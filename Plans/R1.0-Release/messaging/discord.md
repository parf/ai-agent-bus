# Discord agent (B.27a)

📌 **TL;DR:** The Discord helper implements the approved methods and Configuration with an outbound gateway and the shared subscriptions engine. Its current contract is in the [agent guide](../../../docs/agents/discord.md#discord). All acceptance runs use fake Discord endpoints; the owner's application/server choice remains Q244.

## What you get

The [current methods](../../../docs/agents/discord.md#technical-details) post and read, react, edit the bot's text, delete messages, start threads, route replies and forward subscriptions. The record's Secret holds the bot token.

## One way to start (once approved)

See [installation](../../../docs/agents/discord.md#installing-it). A live deployment still needs the owner to create and own the application and choose its server; no live Discord call or sign-up was made by this task.

## Proposed methods (for approval)

Approved by the owner, 2026-10-30; implemented in `src/agents/discord/agent.json`. The [owning guide](../../../docs/agents/discord.md#technical-details) has their current behaviour. Discord permits editing text only by its author; manage-messages permission permits deleting another person's message, not editing its text ([Discord](https://docs.discord.com/developers/resources/message#edit-message)).

## Proposed Configuration keys (for approval)

Approved by the owner, 2026-10-30; see [Configuration](../../../docs/agents/discord.md#configuration) and [Secret keys](../../../docs/agents/discord.md#secret-keys). The director confirmed the channel alias shape as within that approval: missing alias ids and duplicate resolved ids are refused at load, with both duplicate keys named.

## How it receives

See [connection details](../../../docs/agents/discord.md#technical-details). This implements the gateway choice; no incoming HTTP endpoint is exposed.

## Provider facts that matter

Primary references checked 2026-10-30: [gateway](https://docs.discord.com/developers/events/gateway), [messages](https://docs.discord.com/developers/resources/message), [channels](https://docs.discord.com/developers/resources/channel), [rate limits](https://docs.discord.com/developers/topics/rate-limits). The helper operates on existing DM channels and opens none, so the old unconfirmed DM-creation question is outside its methods.

## Test plan

- Fake REST and gateway fixtures inspect every method's request and answer, 429 delay, refusal, gateway heartbeat/ACK/reconnect/resume and duplicate dispatch suppression.
- Private-bus package checks exercise real read-only CLI calls, help, token origin, shutdown, shared delivery counts, a full target followed by a delivered match, and revoked author rights to the target and helper. No live Discord call is made.
- Owning mutants cover environment-token use, an outside guild action, forwarding the bot's own message, an oversize post, an invalid-request loop, a missing default, alias ids and duplicate ids. See [results](../DONE.md#b27a-discord).
- The fast allowlist contains only parsing, Configuration, splitting and metadata operations. Waiting, network and process fixtures run in the normal owning shard.

## Decided

- Gateway only; Message Content Intent on; no member-directory methods ([RD172](../DECISIONS.md#recorded-decisions), director, 2026-10-30).
- Methods and Configuration approved by the owner, 2026-10-30. Their substance is in the current guide linked above.

## Open for the owner

- **Q244** Who creates and owns the Discord application and bot, and for which server? Needed before a live deployment; no sign-up was made. See [questions](../QUESTIONS.md#messaging-agents).
