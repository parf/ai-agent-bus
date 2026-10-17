# Slack agent

📌 **TL;DR:** `#slack@<realm>` puts a Slack workspace on the bus: it posts,
reads, looks up channels and users, and forwards a Slack channel's messages to
a bus agent, 📮 queue or 📣 pubsub, filtered by an optional regex. Its
subscriptions live in its own KV store. It is one TypeScript process on bun,
serving itself through the [TypeScript client](../../../src/clients/typescript/README.md).

Status: **built 2026-10-17 at 0.8.137** (S.1–S.4 by OpenCode, review fixes and
S.5–S.7 by Claude, every task reviewed by Codex); not yet deployed. Running it:
[the agent's README](../../../src/agents/slack/README.md#running-it) ([TODO](TODO.md#tasks)). Code goes in `src/agents/slack/`.

## One agent, both ways

The owner asked for one agent with post, read and subscribe. That revises, for
Slack, two [catalogue rules](../services.md#rules-they-all-obey):
- "inbound is a publisher, outbound is a call" becomes one agent doing both;
- "none of them enforces access" becomes one check: editing subscriptions
  needs the caller's daemon-asserted `owner` or `maintainer` role, so it is not
  a second ACL.

The other catalogue rules hold:
- secrets kept out of the checkout (here in the record's bus secret, the one source it reads);
- one template, many instances (`#slack/<workspace>@<realm>`);
- an upstream refusal is an answer.

## How it runs

- One long-lived process: Socket Mode needs a kept websocket, ported from
  `/rd/bin/slack-tail` (`connectOnce`: backoff, a 15-minute watchdog, each
  envelope acked at once).
- `agent.json` says `serves: itself`: the runner keeps it running and restarts it
  ([№M.9](../../R1.0-Release/TODO.md#runner-waves)), packaged by `package.sh`.
- Secrets live in the record's bus secret, dotenv form, read with `record.secret()`:
  - `SLACK_BOT_TOKEN` (xoxb);
  - `SLACK_APP_TOKEN` (xapp, Socket Mode);
  - `SLACK_USERNAME` and `SLACK_ICON`, optional: the name and icon it posts under;
  - `SLACK_API_URL`, optional, used by tests to point at a fake Slack.
- Posting is modelled on `/rd/bin/slack-post`: email to DM, the permalink
  answered, and a name of its own rather than the bot's.
- The bot token may be one other programs share, such as CI or `slack-post`.

## Methods

The text form and the JSON form map to one handler, and the answer mirrors the
request's form.

| Method | Text | JSON | Does |
|---|---|---|---|
| post | `post <#chan\|CID\|email> <text…>` | `{"method":"post","channel","text","thread_ts"?,"username"?,"icon"?}` | `chat.postMessage`. An email address resolves to a DM through `users.lookupByEmail`, as slack-post does. Answers the permalink from `chat.getPermalink`, falling back to `channel ts=` |
| read | `read <#chan\|CID> [N]` | `{"method":"read","channel","limit"?}` | `conversations.history`; N defaults to 20, at most 200. Answers the messages oldest first: ts, user, text |
| sub list | `sub list` | `{"method":"sub.list"}` | every subscription: id, channel, target, filter, created_by, created_at, forwarded, last_error |
| sub add | `sub add <#chan\|CID> <target> [filter]` | `{"method":"sub.add","channel","target","filter"?}` | stores one subscription; answers its id |
| sub edit | `sub edit <id> [channel=…] [target=…] [filter=…]` | `{"method":"sub.edit","id",…}` | changes only the fields named; `filter=` empty clears it |
| sub delete | `sub delete <id>` | `{"method":"sub.delete","id"}` | removes it |
| channels | `channels [filter]` | `{"method":"channels","filter"?,"member_only"?}` | `conversations.list` (public and private the bot is in): id, #name, member count, is_member, topic; filter is a substring of the name |
| users | `users [filter]` | `{"method":"users","filter"?}` | `users.list`, without deleted users and bots: id, name, real name, email (with `users:read.email`); filter matches name, real name or email |
| user | `user <id\|email\|@name>` | `{"method":"user","who"}` | one user (`users.info` / `users.lookupByEmail`): id, names, email, tz, title, is_bot |
| channel | `channel <#chan\|CID>` | `{"method":"channel","channel"}` | `conversations.info`: id, name, topic, purpose, members count, created |
| thread | `thread <#chan\|CID> <ts> [N]` | `{"method":"thread","channel","ts","limit"?}` | `conversations.replies`: a thread's messages, oldest first |
| react | `react <#chan\|CID> <ts> <emoji>` | `{"method":"react","channel","ts","emoji"}` | `reactions.add`, e.g. a ✅ on a message an agent handled |
| edit | `edit <#chan\|CID> <ts> <text…>` | `{"method":"edit","channel","ts","text"}` | `chat.update`, only the bot's own messages (Slack enforces) |
| delete | `delete <#chan\|CID> <ts>` | `{"method":"delete","channel","ts"}` | `chat.delete`, only the bot's own messages |
| join | `join <#chan\|CID>` | `{"method":"join","channel"}` | `conversations.join`, so the bot can read or subscribe to a public channel it is not in |
| whoami | `whoami` | `{"method":"whoami"}` | `auth.test`: the bot's user, team, workspace URL; the first thing to call when setting up |
| help | `help` or anything unknown | — | lists the methods |

**JSON answers.** A JSON call is answered in JSON, in Slack's own convention:
`{"ok": true, …}` with the method's fields, or `{"ok": false, "error": "…"}`.
This shape was chosen by the coordinator, on Codex's review, for the owner's
review.

| Method | `ok: true` fields |
|---|---|
| post | `channel` (the conversation Slack answered with), `ts`, `permalink` (absent when its lookup failed) |
| read, thread | `messages`: `[{ts, user, text, thread_ts?}]`, oldest first |
| channels | `channels`: `[{id, name, members, is_member, topic}]` |
| users | `users`: `[{id, name, real_name, email?}]` |
| user | `user`: `{id, name, real_name, email?, tz, title, is_bot}` |
| channel | `channel`: `{id, name, topic, purpose, members, created}` |
| react, edit, delete, join | `channel`, and `ts` where the call names one |
| whoami | `user`, `user_id`, `team`, `team_id`, `url` |
| sub list | `subs`: `[{id, channel, target, filter?, created_by, created_at, forwarded, last_error?}]` |
| sub add | `id` |
| sub edit, sub delete | `id` |
| help | `help`, the text of the method list; a malformed JSON call is `{"ok": false, "error": "…", "help": "…"}` |

Timestamps stay strings. A text call keeps its text answers.

Left out on purpose:
- `search.messages` needs a user token, not a bot's.
- File upload (`files.uploadV2`) needs a binary body the text/JSON call does not carry. Both go to FUTURE.

Channel and user resolution:
- A `#name` is resolved through a cached `conversations.list`, refreshed when it misses.
- `@name` and email are resolved through `users.list` or `users.lookupByEmail`.
- An unknown name answers `error: no channel #x`.


## Subscriptions, authority, errors

- **Text form values:** a filter runs to the end of the line, in `sub add` (after the target) and in `sub edit` (`filter=` is the last key, and everything after it is the filter). `filter=` with nothing after it clears it.
- **Rate limits:** a Slack 429 waits its `Retry-After`, at most 30 s, and retries once; a second 429 is answered as `error: ratelimited`.
- **Filter:**
  - `/pattern/flags` is a JS RegExp with its flags (e.g. `/error/i`);
  - any other text is a pattern with no flags (e.g. `^agent: `);
  - it is tested against the message text;
  - an invalid regex is refused at add or edit, naming the error.
- **Subscription storage:** the agent's own KV.
  - Each subscription is one JSON value named `sub.<id>`, with id a short random hex, so concurrent edits never clobber one another.
  - The listing walks the store.
  - It survives restarts, and the KV page in the web face shows it.
- **Forwarding:** for each Socket Mode message event that matches a subscription's channel and filter, it sends to the target as `#slack`.
  - topic `slack`, tag `<channel id>`;
  - the body is the JSON of `{channel, channel_name, user, text, ts, thread_ts}`, slack-tail's shape without `raw`.
  - A 📣 target publishes; an agent or a 📮 target queues.
  - It drops the posts it made itself, so it never re-triggers itself; other posts in the same bot's name are forwarded.
  - A refused send (the target's ACL lacks `#slack`) is stored in that subscription's `last_error` and its count, logged, and never retried in a loop.
- **Authority:**
  - The daemon's ACL decides who may call at all.
  - `sub add`, `sub edit` and `sub delete` also require the caller to hold the generated `owner` or `maintainer` role. Those roles are delivered on the message and asserted by the daemon, so this is no second ACL.
  - The roles count only on a message addressed to `#slack` itself: its `original_to` is empty or `#slack`'s own name. Roles are worked out at the record first addressed and carried unchanged through a forward, so a Maintainer of another record of the same Owner that forwards to `#slack` would otherwise arrive as `#slack`'s Maintainer. A subscription edit arriving by any forward is refused.
  - `post`, `read` and `sub list` are open to anyone the ACL admits.
  - Owner-facing tip: `@owner` in `#slack`'s ACL admits its Owner and the agents that Owner owns; any other `@name` is a stored group and needs membership.
- **Upstream errors are answers**, per the catalogue contract: a Slack `ok:false` comes back as `error: <slack error>`. Only the agent itself being broken produces no answer.
- **Socket Mode:**
  - reconnect with backoff;
  - a 15-minute watchdog;
  - ack each envelope at once.

  This is ported from slack-tail's `connectOnce`.


## Behaviour pinned by the review

Codex's TODO review, 2026-10-17. Where it left a choice, the conservative
option is taken here, **for the owner's review**.

| | |
|---|---|
| **One subscription, one writer at a time** | each subscription is its own JSON value. An edit is a KV JSON operation that sets only the named top-level fields, so two disjoint edits both land. A create uses mode `add`, so an id collision is refused, not overwritten. A delete removes the value; nothing may write it back |
| **Counters are not the subscription** | `forwarded` and `last_error` live in memory and are flushed once a minute, the owner's rule for statistics, into separate int and string values `stat.<id>.*`, only for ids still present. A crash loses at most a minute of counts. add, edit and delete persist at once |
| **Filters** | flags `i`, `m`, `s`, `u` only; `g` and `y` are refused, since they carry state between tests. A pattern is at most 200 characters, tested against at most the first 4 KiB of text. A pattern is checked on add and on edit, and a refused edit keeps the old subscription. Filters run in a worker that is terminated at 50 ms, so a costly pattern cannot hold up the socket's acks or the bus; a filter that hits the limit is disabled, with `last_error` saying so |
| **Socket Mode delivery** | each envelope is acked at once, before anything is forwarded, so a slow bus never delays Slack's ack. Events are deduplicated by `event_id` over the last 1,000 events, in memory, so a restart may forward one again. A bounded queue of 1,000 events sits between socket and bus; at overflow the oldest is dropped, counted and logged. A bus send that fails is recorded in `last_error` and never replayed, since an uncertain write must not be retried blindly. Shutdown closes the socket, stops reconnect and watchdog, and drains the queue for up to 5 s. What this promises: one bus attempt per admitted event, with duplicates suppressed only within the 1,000-event window. Past that window or after a restart Slack may deliver again. An event acked but lost before its send, by a crash or an overflow, is not recovered. Nothing durable, and no exactly-once |
| **Which events** | plain messages, `thread_broadcast` and `bot_message` are forwarded. `message_changed`, `message_deleted` and every other subtype are dropped, as slack-tail does. A message in the bot's name (its user id, bot id or app id, from `auth.test`) is dropped only when this agent posted it: it keeps the channel and ts of each post for 10 minutes, at most 1000, and an event that arrives while a post awaits its answer waits for it, at most 5 s. Until the bot's identity is known, nothing is forwarded Non-message payloads are acked and ignored |
| **Posting** | the name is the call's `username`, else the secret's `SLACK_USERNAME`, else `cli@<host>` as slack-post's; an empty `SLACK_USERNAME` posts as the bot. The icon is the call's, else `SLACK_ICON`. Both need `chat:write.customize`. An `icon` of `:name:` is `icon_emoji`, and a URL is `icon_url`. An email resolves to a user id through `users.lookupByEmail` and is posted to that id, with no `conversations.open` and no `im:write`. Timestamps stay strings. A permalink lookup that fails after a post answers `channel ts=…` and never posts again |
| **HTTP** | every call is form-encoded, which every method accepts and `chat.getPermalink` requires; a `missing_scope` answer names the scope needed. Each Slack call is bounded at 15 s and by the caller's deadline. A write (`chat.*`, `reactions.add`, `conversations.join`) is never retried, except after a 429, which Slack did not carry out. A read retries once on 5xx. A malformed answer is an `error:` answer. `read` and `thread` follow the cursor until N are had |
