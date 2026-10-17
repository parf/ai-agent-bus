# Slack agent

📌 **TL;DR:** `#slack@<realm>` puts a Slack workspace on the bus: it posts,
reads, looks up channels and users, and forwards a Slack channel's messages to
a bus agent, 📮 queue or 📣 pubsub, filtered by an optional regex. Its
subscriptions live in its own KV store. It is one TypeScript process on bun,
serving itself through the [TypeScript client](../../../src/clients/typescript/README.md).

Status: **planned 2026-10-17, owner request; built by OpenCode, reviewed by
Codex** ([TODO](TODO.md#tasks)). Code goes in `src/agents/slack/`.

## One agent, both ways

The owner asked for one agent with post, read and subscribe. That revises, for
Slack, two [catalogue rules](../services.md#rules-they-all-obey):
- "inbound is a publisher, outbound is a call" becomes one agent doing both;
- "none of them enforces access" becomes one check: editing subscriptions
  needs the caller's daemon-asserted `owner` or `maintainer` role, so it is not
  a second ACL.

The other catalogue rules hold:
- secrets in the instance's env;
- one template, many instances (`#slack/<workspace>@<realm>`);
- an upstream refusal is an answer.

## How it runs

- One long-lived process: Socket Mode needs a kept websocket, ported from
  `/rd/bin/slack-tail` (`connectOnce`: backoff, a 15-minute watchdog, each
  envelope acked at once).
- `agent.json` says `serves: itself`. Until runner [wave M.9](../../R1.0-Release/TODO.md#managed-runner-waves)
  it is started by hand or by a user systemd unit; then the managed runner
  starts and restarts it.
- Secrets live in the record's bus secret, dotenv form, read with `record.secret()`:
  - `SLACK_BOT_TOKEN` (xoxb);
  - `SLACK_APP_TOKEN` (xapp, Socket Mode);
  - `SLACK_API_URL`, optional, used by tests to point at a fake Slack.
- Posting is modelled on `/rd/bin/slack-post`: email to DM, the permalink answered.

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

Left out on purpose:
- `search.messages` needs a user token, not a bot's.
- File upload (`files.uploadV2`) needs a binary body the text/JSON call does not carry. Both go to FUTURE.

Channel and user resolution:
- A `#name` is resolved through a cached `conversations.list`, refreshed when it misses.
- `@name` and email are resolved through `users.list` or `users.lookupByEmail`.
- An unknown name answers `error: no channel #x`.


## Subscriptions, authority, errors

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
  - It drops its own posts (bot user, app id) so it never re-triggers itself, as slack-tail does with `--ignore-app`.
  - A refused send (the target's ACL lacks `#slack`) is stored in that subscription's `last_error` and its count, logged, and never retried in a loop.
- **Authority:**
  - The daemon's ACL decides who may call at all.
  - `sub add`, `sub edit` and `sub delete` also require the caller to hold the generated `owner` or `maintainer` role. Those roles are delivered on the message and asserted by the daemon, so this is no second ACL.
  - `post`, `read` and `sub list` are open to anyone the ACL admits.
  - Owner-facing tip: an `@user` term in the ACL lets that user's own agents call it.
- **Upstream errors are answers**, per the catalogue contract: a Slack `ok:false` comes back as `error: <slack error>`. Only the agent itself being broken produces no answer.
- **Socket Mode:**
  - reconnect with backoff;
  - a 15-minute watchdog;
  - ack each envelope at once.

  This is ported from slack-tail's `connectOnce`.

