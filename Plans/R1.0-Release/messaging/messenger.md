# Facebook Messenger agent (B.27e)

📌 **TL;DR:**
- **Problem:** people write to your Facebook Page in Messenger, and your AI assistants cannot answer them or follow up.
- **Outcome:** `#messenger/<page>@<realm>` puts one Facebook Page's inbox on the bus like the Slack helper: people message the Page, the reply reaches the right session, and the session answers in the same conversation.
- **How:** Meta's Messenger Platform: incoming messages arrive as webhook requests and replies are sent through its Send API, so it needs a public HTTPS address.
- **Hard parts:** only a Facebook Page works (not a personal profile), a person must write first, and free-form replies are allowed only within the 24-hour window.
- **Status:** proposal for the owner's approval, no code. Facts from Meta's Messenger overview fetched 2026-10-30; unconfirmed ones are marked.

## What you get

- **Answer people:** someone messages your Page, an assistant answers within 24 hours.
- **Reply routing:** a person's message goes to the session they last talked to, or the Page's default session.
- **Forwarding:** `sub` sends matching incoming messages to a session.
- **Safe keys:** the Page access token stays on the server in the record's Secret.

## One way to start (once approved)

- Register a Meta app with the Messenger use case, link it to your Facebook Page, create the Page access token.
- Register `#messenger/page`, set the token as its Secret, point the app's webhook at the agent's HTTPS address (the owner's decision, Q259), message the Page from a test account.

## Proposed methods (for approval)

| Method | Text form | Messenger | Differs from Slack/Telegram |
|---|---|---|---|
| `post` | `post <person> <text…>` | send API, text | `<person>` is a page-scoped id the Page has seen or an alias; only to someone who wrote first and only inside the window; answers the message id |
| `read` | `read [<person>] [N]` | none | answers the last N messages received from that person (a ring); history through Graph is not planned |
| `chats` | `chats [filter]` | none | aliases and people who wrote, with the time their window closes |
| `seen` | `seen <person>` | sender action | marks the conversation read |
| `whoami` | `whoami` | Page info | the Page's name and id |
| `sub …`, `help` | as Slack | | the channel is a `<person>` |

- **Left out:** media, quick replies and buttons, templates and message tags, handover to a human agent, comments, Instagram messaging, reactions, `edit` and `delete`.

## Proposed Configuration keys (for approval)

| Key | Says |
|---|---|
| `users` | page-scoped id to bus person: who may pick sessions by tag |
| `tags`, `route` | as Slack |
| `contacts` | per person: an alias, `route`, `tags`, `anyone` |
| `page_id` | which Page this instance answers for |
| `listen` | the address the agent listens on |

- **Secret keys:** `MESSENGER_PAGE_TOKEN`, `MESSENGER_APP_SECRET` (to verify webhook signatures), `MESSENGER_VERIFY_TOKEN`; test-only `MESSENGER_API_URL`.

## How it receives

- **Webhook:** Meta posts a notification when someone messages the Page; the agent checks the request's signature with the app secret and the verify token at set-up before reading anything.
- **Not forwarded:** the Page's own sends and delivery or read notices.
- **No outbound-only path** was found.

## Provider facts that matter

| Fact | Source | Status |
|---|---|---|
| A Meta developer account, a Meta app with the Messenger use case and a linked Facebook Page are required | [Messenger overview](https://developers.facebook.com/docs/messenger-platform/overview) | confirmed |
| Messaging only your own Page needs no App Review; Advanced Access needs it, and Business Verification is needed if people without a role on the app will use it | same | confirmed |
| Webhooks fire when someone messages the Page; no polling needed | same | confirmed |
| Most user actions open a 24-hour standard messaging window, up to 7 days after a click-to-Messenger ad | same | confirmed |
| A person must start the conversation; the platform is free to use | same | confirmed |
| Permissions: `pages_show_list`, `pages_manage_metadata`, `pages_messaging`, `pages_read_engagement`, `business_management`; the user granting the token needs the MESSAGING and MODERATE Page tasks | same | confirmed |
| Personal profiles cannot be automated | the page covers only Pages and Instagram Professional accounts | unconfirmed as a statement; the plan assumes Pages only |
| Send API request shape, text length, rate limits, message tags outside the window | Get started guide not read | unconfirmed |
| Webhook signature header and algorithm | not read | unconfirmed; confirm in the webhook guide |
| Cost | the page says the platform is free to use | confirmed for the platform; hosting is the owner's |

## Test plan

- **No live Meta call, no sign-up, nothing sent.** A fixture stands in for the Graph API and posts signed webhook requests.
- **Window rule:** a `post` to someone who never wrote, or after the window, is refused locally and never sent.
- **Webhook:** a bad signature or verify token is refused before the body is read; a duplicate delivery is dropped.
- **Mutants:** signature check skipped, window not enforced, a message from another Page accepted, token read from the environment, the Page's own send forwarded; each fails an assertion.

## Questions for the owner (drafted, not filed)

- **Q257** Which Facebook Page and Meta app will this use, and who owns them?
- **Q258** Only your own Page (no App Review) or a Page of others (Advanced Access and review)? Proposed: your own.
- **Q259** May the agent be exposed on a public HTTPS address for Meta's webhook?
