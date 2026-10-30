# Google Chat agent (B.27b)

📌 **TL;DR:**
- **Problem:** your team uses Google Chat and your AI assistants cannot post there or be asked from it.
- **Outcome:** `#googlechat/<account>@<realm>` puts one Chat app on the bus like the Slack helper: your assistant posts to spaces, people message or mention the app, and the reply reaches the right session.
- **How:** a Chat app with its own service account, receiving what people send it through a Google Pub/Sub subscription the runner pulls, so no public address is needed.
- **Limit that shapes it:** a Chat app is told only about direct messages and @mentions, so it forwards those, not every message in a space.
- **Status:** proposal for the owner's approval, no code. Facts from Google's developer docs fetched 2026-10-30; unconfirmed ones are marked.

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
| `read` | `read [<space>] [N]` | none by default | answers the last N messages the app received in that space (a ring, as Telegram); listing real history needs an administrator-approved scope, see Q246 |
| `spaces` | `spaces [filter]` | list spaces | only the spaces the app is a member of |
| `space` | `space <space>` | get space | name, type, display name |
| `members` | `members <space>` | list memberships | needs the app memberships scope; proposed off, see Q246 |
| `edit`, `delete` | as Telegram | update, delete message | the app's own messages |
| `whoami` | `whoami` | none | the app's name and project |
| `sub …`, `help` | as Slack | | the channel is a `<space>`; matches only messages the app received |

- **Left out:** cards and dialogs, file upload, reactions, search (not available to an app), `join` (a person adds the app).
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

- **Pub/Sub pull** (proposed): the app's connection is a Pub/Sub topic; the runner pulls the subscription, so only outbound calls are made.
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

## Questions for the owner (drafted, not filed)

- **Q245** Pub/Sub pull or an HTTP endpoint? Proposed pull (outbound only).
- **Q246** Accept that the app sees only DMs and @mentions, and leave real history and members off? The alternative needs an administrator's one-time approval and a Marketplace-compatible OAuth client.
- **Q247** Which Google account type will this run on (Workspace or personal)? Unconfirmed that personal accounts can host Chat apps.
- **Q248** Who creates the Cloud project, the app and the service account, and in which organisation?
