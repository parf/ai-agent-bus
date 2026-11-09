# B.27 questions Q241-Q259: triage

Node-local. Types: **technical** (#ab-dir can settle), **owner: account** (who owns what), **owner: public endpoint** (exposure), **owner: cost** (money). Recommendations are mine; unconfirmed facts are marked in each plan.

| Q | Question | Type | Recommended answer |
|---|---|---|---|
| Q241 | Discord: gateway or interactions | technical | Gateway: outbound only, no public address |
| Q242 | Discord: use the privileged message-content intent | technical | Yes, switch it on in the portal (below 100 servers no approval); without it the bot cannot read channel chatter |
| Q243 | Discord: `users` and `user` methods | technical | Leave out; needs a second privileged intent and adds little |
| Q244 | Discord: who creates and owns the app and bot, for which server | owner: account | The owner names the account and server; nothing can be tried live before |
| Q245 | Google Chat: Pub/Sub pull or HTTP endpoint | technical | Pub/Sub pull: outbound only |
| Q246 | Google Chat: accept DMs and @mentions only; leave history and members off | technical | Accept; history needs an admin-approved scope and a Marketplace-compatible OAuth client |
| Q247 | Google Chat: Workspace or personal Google account | owner: account | Workspace; unconfirmed that a personal account can host a Chat app, confirm before building |
| Q248 | Google Chat: who creates the Cloud project, app and service account | owner: account | The owner names the project and organisation |
| Q249 | Teams: which SDK (Teams SDK, Agents SDK or a hand-written connector) | technical | Teams SDK or Agents SDK for TypeScript after reading their weight and licences; avoid a hand-written connector (JWT verification is the risk); the Bot Framework SDK is unmaintained since 31 Dec 2025 |
| Q250 | Teams: expose the agent on a public HTTPS address | owner: public endpoint | Needed, no outbound-only path found; owner decides with the webhook agent's exposure, behind TLS and request authentication |
| Q251 | Teams: ask each team owner for the all-messages consent | technical | Start with @mentions and DMs; add the RSC consent per team later |
| Q252 | Teams: tenant, Azure subscription, who pays | owner: cost | The owner names them; Azure bot and hosting cost is unconfirmed |
| Q253 | WhatsApp: business number, Meta account, who pays | owner: cost | The owner names number and payer; Meta bills per delivered message and charges service messages from 1 Oct 2026 (1,000 free per number per month) |
| Q254 | WhatsApp: allow `template` or answer inside the 24-hour window only | technical | Answer-only first; templates add approval and opt-in work |
| Q255 | WhatsApp: expose the agent for Meta's webhook | owner: public endpoint | Same decision as Q250: needed, behind TLS and signature checks |
| Q256 | WhatsApp: who records and honours opt-in | technical | A per-contact Configuration flag the agent enforces; a missing flag blocks `template` |
| Q257 | Messenger: which Page and Meta app | owner: account | The owner names the Page and app |
| Q258 | Messenger: own Page only or Pages of others | technical | Own Page only: no App Review |
| Q259 | Messenger: expose the agent for Meta's webhook | owner: public endpoint | Same decision as Q250 and Q255 |

## Count

- **Technical (#ab-dir):** Q241, Q242, Q243, Q245, Q246, Q249, Q251, Q254, Q256, Q258 (10).
- **Owner, account:** Q244, Q247, Q248, Q257 (4).
- **Owner, public endpoint:** Q250, Q255, Q259 (3, one decision covers all three).
- **Owner, cost:** Q252, Q253 (2).

## One owner decision unlocks most

- **Public HTTPS exposure (Q250, Q255, Q259):** one yes or no for the webhook-style agents (Teams, WhatsApp, Messenger). A no leaves only Discord and Google Chat, which need no public address.
- **Accounts (Q244, Q247, Q248, Q257):** nothing is tried live until the owner names the accounts; no sign-up was made.
