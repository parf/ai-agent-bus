# Google Chat agent (B.27b)

📌 **TL;DR:**
- **Problem:** your team uses Google Chat and your AI assistants cannot post there or be asked from it.
- **Outcome:** `#googlechat/<account>@<realm>` puts one Chat app on the bus like the Slack helper: your assistant posts to spaces, people message or mention the app, and the reply reaches the right session.
- **How:** a Chat app with its own service account, receiving what people send it through a Google Pub/Sub subscription the runner pulls, so no public address is needed.
- **Limit that shapes it:** a Chat app is told only about direct messages and @mentions, so it forwards those, not every message in a space.
- **Status:** technical choices decided 2026-10-30 ([decisions](#decided)); the Google account and project are the owner's open questions (Q247, Q248). No code. Facts from Google's developer docs fetched 2026-10-30; unconfirmed ones are marked.

## What you get

- **Post:** an assistant tells a space what it did, or asks a question.
- **Reply routing:** message or @mention the app, or reply in a thread, and that assistant's session gets it.
- **Forwarding:** `sub` sends the messages addressed to the app that match a filter to a session.
- **Safe keys:** the service account key stays on the server in the record's Secret.

## One way to start (once approved)

- In a Google Cloud project, enable the Chat API, create the Chat app with a service account and a Pub/Sub topic as its connection.
- Add the app to a space (or have an administrator install it).
- Register `#googlechat/team`, set the service-account key as its Secret, give a session access, call `post`.

## Proposed methods (for approval)

| Method | Text form | Chat API | Differs from Slack/Telegram |
|---|---|---|---|
| `post` | `post [<space>] <text…>` | create message | plain text; JSON `reply_to` keeps the thread; answers space and message name |
| `read` | `read [<space>] [N]` | none | answers the last N messages the app received in that space (a ring, as Telegram); real history is not used |
| `spaces` | `spaces [filter]` | list spaces | only the spaces the app is a member of |
| `space` | `space <space>` | get space | name, type, display name |
| `edit`, `delete` | as Telegram | update, delete message | the app's own messages |
| `whoami` | `whoami` | none | the app's name and project |
| `sub …`, `help` | as Slack | | the channel is a `<space>`; matches only messages the app received |

- **Left out:** `members` and real message history (they need an administrator-approved scope), cards and dialogs, file upload, reactions, search (not available to an app), `join` (a person adds the app).
- **`<space>`** is an alias or a space name such as `spaces/AAA…`.

## Proposed Configuration keys (for approval)

| Key | Says |
|---|---|
| `users` | Google user id to bus person: who may pick sessions by tag |
| `tags`, `route` | as Slack |
| `spaces` | per space: an alias, `route`, `tags`, `anyone` |
| `default` | the space a `post` with none goes to; none declared means refused, naming the key |
| `subscription` | the Pub/Sub subscription the app pulls from |

- **Secret keys:** `GOOGLE_CHAT_SERVICE_ACCOUNT_JSON`; test-only `GOOGLE_CHAT_API_URL` and `GOOGLE_PUBSUB_API_URL`.

## How it receives

- **Pub/Sub pull:** the app's connection is a Pub/Sub topic; the runner pulls the subscription, so only outbound calls are made.
- **Other options** the console offers: an HTTP endpoint (needs a public address and verifying Chat's requests), Apps Script, Dialogflow.
- **What arrives:** message events for direct messages and @mentions, added-to and removed-from space, and card clicks; the app is not told about other messages in a space.
- **Not forwarded:** the app's own posts.

## Provider facts that matter

| Fact | Source | Status |
|---|---|---|
| App authentication uses a service account; `chat.bot` is non-sensitive, works with app authentication only, and needs no administrator approval | [Authenticate and authorize](https://developers.google.com/workspace/chat/authenticate-authorize) | confirmed |
| Listing messages as an app needs `chat.app.messages.readonly` and one-time administrator approval; searching messages does not support app authentication | same | confirmed |
| With app authentication, methods see only what the app can access: `spaces.list` returns the spaces the app is in | same | confirmed |
| A Chat app can receive events by HTTP endpoint, Apps Script, Pub/Sub topic or Dialogflow; an HTTP endpoint must answer within 30 seconds or answer later through the API | [Receive and respond](https://developers.google.com/workspace/chat/receive-respond-interactions) | confirmed |
| Message events fire for direct messages, @mentions, matching link previews and menu input | same | confirmed (page does not say the app sees all messages; the list implies it does not) |
| A Pub/Sub path exists for apps behind a firewall | [Pub/Sub quickstart](https://developers.google.com/workspace/chat/quickstart/pub-sub) | confirmed by title and summary; steps not read |
| A message sent with user authentication is limited to 32,000 bytes | search summary | unconfirmed for app authentication; confirm in the message reference |
| A Google Workspace account is needed to use Chat apps; personal Google accounts | not read | unconfirmed |
| Cost: the Chat API, Pub/Sub volume, Workspace licences | not read | unconfirmed; no price read |
| Review: a private app inside one organisation needs no Marketplace review; the approved `chat.app.*` scopes need a Marketplace-compatible OAuth client | authenticate page | confirmed for the scopes, unconfirmed for the private-app review |

## Test plan

- **No live Google call.** A fixture serves the Chat REST calls and a Pub/Sub pull with canned events.
- **Per method and routing:** as the Discord plan; 429 or quota errors answered `error: …`, never retried.
- **Mutants:** the key read from the environment, a space outside `spaces` acted on, the app's own post forwarded, a pull acknowledged before the session got it (loses a message), a duplicate delivery sent twice; each fails an assertion.

## Decided

- **Pub/Sub pull** (Q245): outbound only; no HTTP endpoint. #ab-dir, 2026-10-30.
- **Direct messages and @mentions only** (Q246): history and members stay off. #ab-dir, 2026-10-30.
- **Record:** [decision RD173](../DECISIONS.md#recorded-decisions).

## Open for the owner

- **Q247** Which Google account type hosts the app (Workspace or personal)?
- **Q248** Who creates the Cloud project, the app and its service account, in which organisation?
- See [questions](../QUESTIONS.md#messaging-agents).
