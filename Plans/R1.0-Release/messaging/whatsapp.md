# WhatsApp agent (B.27d)

📌 **TL;DR:**
- **Problem:** customers, partners or your own people use WhatsApp, and your AI assistants cannot answer there or tell them something.
- **Outcome:** `#whatsapp/<account>@<realm>` puts one WhatsApp business number on the bus like the Slack helper: people message the number, the reply reaches the right session, and the session answers in the same chat.
- **How:** Meta's WhatsApp Business Platform (Cloud API): outgoing messages are web calls, incoming ones arrive as webhook requests, so it needs a public HTTPS address.
- **Hard parts:** only a business number works (not a personal WhatsApp), free-form answers are allowed only inside 24 hours of the person's last message, and Meta charges per message from 1 October 2026.
- **Status:** proposal for the owner's approval, no code. Facts from Meta's developer docs fetched 2026-10-30; unconfirmed ones are marked.

## What you get

- **Answer people:** someone writes to your business number, an assistant answers within the 24-hour window.
- **Notify people:** outside the window only an approved template message can start a conversation.
- **Reply routing:** a person's message goes to the session they last talked to, or the number's default session.
- **Forwarding:** `sub` sends matching incoming messages to a session.
- **Safe keys:** the access token stays on the server in the record's Secret.

## One way to start (once approved)

- In Meta Business, create a business portfolio, a WhatsApp business account and a business phone number; make a token.
- Register `#whatsapp/support`, set the token as its Secret, point Meta's webhook at the agent's HTTPS address (the owner's decision, Q255), message the number from a phone.

## Proposed methods (for approval)

| Method | Text form | Cloud API | Differs from Slack/Telegram |
|---|---|---|---|
| `post` | `post <to> <text…>` | send text message | `<to>` is a phone number in international form or an alias; refused outside the 24-hour window unless the JSON names a `template`; answers the message id |
| `template` | `template <to> <name> [params…]` | send template message | only an approved template; needs the person's opt-in recorded in Configuration |
| `read` | `read [<to>] [N]` | none | answers the last N messages received from that person (a ring); WhatsApp has no history API |
| `chats` | `chats [filter]` | none | aliases and people who wrote, with the time of their last message (window open or closed) |
| `react` | `react <to> <id> <emoji>` | reaction message | to a message the person sent |
| `seen` | `seen <id>` | mark as read | |
| `whoami` | `whoami` | none | the business number and account |
| `sub …`, `help` | as Slack | | the channel is a `<to>` |

- **Left out:** media, groups, calls, interactive buttons and lists, business profile, `edit` and `delete`.

## Proposed Configuration keys (for approval)

| Key | Says |
|---|---|
| `users` | phone number to bus person: who may pick sessions by tag |
| `tags`, `route` | as Slack |
| `contacts` | per number: an alias, `route`, `optin` (true only when the person agreed to be messaged) |
| `templates` | the approved template names `template` may send, with their parameter counts |
| `phone_number_id` | which business number this instance sends from |
| `listen` | the address the agent listens on |

- **Secret keys:** `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_APP_SECRET` (to verify webhook signatures), `WHATSAPP_VERIFY_TOKEN`; test-only `WHATSAPP_API_URL`.

## How it receives

- **Webhook:** Meta posts incoming messages and delivery statuses as JSON to the agent's address; each request's signature is checked with the app secret before anything is read; a first call with the verify token proves ownership.
- **Not forwarded:** statuses of the agent's own sends (they update a send's outcome only).
- **No outbound-only path** was found.

## Provider facts that matter

| Fact | Source | Status |
|---|---|---|
| Send by POST to the business phone number's `/messages`; types include text, media, interactive; receive by webhook | [Cloud API overview](https://developers.facebook.com/docs/whatsapp/cloud-api/overview) | confirmed |
| A business portfolio, WhatsApp business account, business phone number (real or virtual) and tokens are needed; 80 messages per second per number by default | same | confirmed |
| The customer service window opens when a user messages or calls and resets to 24 hours on each new message; inside it free-form messages are allowed, outside only approved templates; the person must have opted in | [Send messages](https://developers.facebook.com/docs/whatsapp/cloud-api/guides/send-messages) and pricing page | confirmed |
| Billing is per delivered message from businesses, by category and the recipient's country; service messages become chargeable on 1 October 2026 with 1,000 free per number per month; no payment method by 30 September 2026 may stop delivery after the free tier | [Pricing](https://developers.facebook.com/documentation/business-messaging/whatsapp/pricing) | confirmed from the page; rates not recorded here |
| About US$0.004 for a utility or authentication message and US$0.025 for marketing in the US | third-party blogs | unconfirmed; use Meta's rate card |
| Text body maximum length | the send page does not state it | unconfirmed |
| Request shape for contextual replies, mark as read and reactions | linked pages not read | unconfirmed |
| A personal WhatsApp account cannot be used; automating one is against the terms | overview page is silent | unconfirmed; the plan assumes business numbers only |
| Template approval time and business verification steps | not read | unconfirmed |

## Test plan

- **No live Meta call, no message sent, no sign-up.** A fixture stands in for the Graph API and posts signed webhook requests.
- **Window rule:** a `post` outside 24 hours without a template is refused locally and never sent; a reply inside the window is sent.
- **Webhook:** a bad signature or verify token is refused before the body is read; a duplicate delivery is dropped by message id.
- **Mutants:** signature check skipped, window not enforced, opt-in ignored, token read from the environment, the same webhook delivered twice forwarded twice; each fails an assertion.

## Questions for the owner (drafted, not filed)

- **Q253** Which business number and Meta business account will this use, and who pays Meta's per-message charges from 1 October 2026?
- **Q254** Allow `template` (starting conversations) or answer-only inside the window?
- **Q255** May the agent be exposed on a public HTTPS address for Meta's webhook?
- **Q256** Who records and honours each person's opt-in, and where?
