# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions).

## Distribution

Owner-only choices for the [Linux distribution](distribution.md#simple-install) ([RT14](TODO.md#objective)). None blocks the Linux work: each names what stays undone until it is answered.

- ❓ Q225 Container image registry and name
  - Options: `ghcr.io/parf/agent-bus` beside the repository; or Docker Hub; or both
  - Impact: P.2 is proven with local images until the registry and the account that pushes are chosen
  - Settled by: the owner

- ❓ Q226 Who enables Actions and publishes the release
  - Context: the build on a version tag into a draft release is settled ([RD163](DECISIONS.md#recorded-decisions))
  - Options: the owner makes `parf/ai-agent-bus` public, enables Actions and presses publish after signing `SHA256SUMS`; or the build runs on this node and the owner uploads the assets
  - Impact: the published `install.sh` URL and every download exist only after the owner publishes; the team never pushes, uploads or publishes
  - Settled by: the owner

## Messaging agents

Owner-only choices for the five [messaging agents](TODO.md#bundled-agents) (B.27: Discord, Google Chat, Microsoft Teams, WhatsApp, Facebook Messenger). Each agent is proposed in its own plan; none is built or approved, and no sign-up or live call was made. A fact the plans could not confirm is marked there.

- ❓ Q241 Discord: receive through the gateway or through interactions
  - Context: The bot can hold an outbound WebSocket to Discord's gateway, or receive interactions as web requests at a public address
  - Proposal: the gateway, which needs no public address
  - Impact: gateway: outbound only; interactions: a public HTTPS address and request verification
  - Settled by: the owner

- ❓ Q242 Discord: rely on the privileged message-content intent
  - Context: Without it the bot receives empty message content except its own messages, direct messages to it and messages that mention it
  - Proposal: switch it on in the developer portal; below 100 servers no approval is needed. Alternative: mentions and direct messages only
  - Impact: on: the bot reads ordinary channel chatter and `sub` can match it; off: only addressed messages
  - Settled by: the owner

- ❓ Q243 Discord: `users` and `user` methods
  - Context: Listing members needs the privileged members intent
  - Proposal: leave both out; ids come from `whoami` and the reply router
  - Impact: included: a second privileged intent to switch on
  - Settled by: the owner

- ❓ Q244 Discord: who creates and owns the application and bot, for which server
  - Context: Needed before any live test; no sign-up was made
  - Proposal: The owner names the account and the server
  - Impact: none of the Discord agent can be tried live before this
  - Settled by: the owner

- ❓ Q245 Google Chat: receive by Pub/Sub pull or by an HTTP endpoint
  - Context: A Chat app can receive events through a Pub/Sub topic or a public HTTP endpoint
  - Proposal: Pub/Sub pull, outbound only
  - Impact: pull: a Cloud project with Pub/Sub; endpoint: a public address and request verification
  - Settled by: the owner

- ❓ Q246 Google Chat: accept that the app sees only direct messages and @mentions
  - Context: Listing messages and members as an app needs an administrator's one-time approval of restricted scopes and a Marketplace-compatible OAuth client
  - Proposal: accept; leave history and members off
  - Impact: accepted: `read` answers what the app received; otherwise an approval process comes first
  - Settled by: the owner

- ❓ Q247 Google Chat: which Google account type hosts the app
  - Context: Whether personal Google accounts can host Chat apps is unconfirmed; Workspace is assumed
  - Proposal: Workspace, or a personal account if it works
  - Impact: decides who may install and administer the app
  - Settled by: the owner

- ❓ Q248 Google Chat: who creates the Cloud project, the app and its service account, in which organisation
  - Context: Needed before any live test; no sign-up was made
  - Proposal: The owner names them
  - Impact: none of the Google Chat agent can be tried live before this
  - Settled by: the owner

- ❓ Q249 Microsoft Teams: which SDK
  - Context: The Bot Framework SDK is no longer maintained since 31 December 2025; the Teams SDK and the Microsoft 365 Agents SDK replace it, or a small hand-written Bot Connector client is possible
  - Proposal: Open; no recommendation until the SDKs' weight and licences are read
  - Impact: a dependency under the modules rule and the size of the security surface
  - Settled by: the owner

- ❓ Q250 Microsoft Teams: expose the agent on a public HTTPS address
  - Context: Teams delivers each message as a web request to a messaging endpoint; no outbound-only path was found
  - Proposal: Yes, or Teams cannot reach the agent; the same decision as for the webhook agent
  - Impact: exposure needs the owner's approval, TLS and request authentication
  - Settled by: the owner

- ❓ Q251 Microsoft Teams: ask each team owner for the all-messages consent
  - Context: Without the two resource-specific permissions the bot receives only @mentions in channels and group chats
  - Proposal: stay with @mentions and direct messages first
  - Impact: consent: the bot receives every user message, with the data-use statement Store approval needs
  - Settled by: the owner

- ❓ Q252 Microsoft Teams: which tenant and Azure subscription host the bot, and who pays
  - Context: Azure bot registration and hosting cost is unconfirmed
  - Proposal: The owner names them
  - Impact: none of the Teams agent can be tried live before this
  - Settled by: the owner

- ❓ Q253 WhatsApp: which business number and Meta business account, and who pays Meta's charges
  - Context: Meta bills per delivered message from businesses; service messages become chargeable on 1 October 2026 with 1,000 free per number per month
  - Proposal: The owner names the number and the payer
  - Impact: a business number is required; a personal WhatsApp account cannot be used
  - Settled by: the owner

- ❓ Q254 WhatsApp: allow `template` messages or answer inside the 24-hour window only
  - Context: Outside the window only approved templates can reach a person, and the person must have opted in
  - Proposal: answer-only first
  - Impact: template: approval of templates and a recorded opt-in per person
  - Settled by: the owner

- ❓ Q255 WhatsApp: expose the agent on a public HTTPS address for Meta's webhook
  - Context: Incoming messages arrive as webhook requests; no outbound-only path was found
  - Proposal: Yes, or WhatsApp cannot reach the agent; the same decision as for the webhook agent
  - Impact: exposure needs the owner's approval, TLS and signature verification
  - Settled by: the owner

- ❓ Q256 WhatsApp: who records and honours each person's opt-in, and where
  - Context: Meta requires the person's opt-in before a template is sent
  - Proposal: The owner decides; proposed: a per-contact Configuration flag the agent enforces
  - Impact: an unrecorded opt-in blocks `template`
  - Settled by: the owner

- ❓ Q257 Messenger: which Facebook Page and Meta app, and who owns them
  - Context: Messenger works for a Facebook Page, not a personal profile
  - Proposal: The owner names the Page and the app
  - Impact: none of the Messenger agent can be tried live before this
  - Settled by: the owner

- ❓ Q258 Messenger: your own Page only, or Pages of others
  - Context: Messaging only your own Page needs no App Review; Advanced Access for others needs review and Business Verification
  - Proposal: your own Page
  - Impact: own: no review; others: a review process before any use
  - Settled by: the owner

- ❓ Q259 Messenger: expose the agent on a public HTTPS address for Meta's webhook
  - Context: Incoming messages arrive as webhook requests; no outbound-only path was found
  - Proposal: Yes, or Messenger cannot reach the agent; the same decision as for the webhook agent
  - Impact: exposure needs the owner's approval, TLS and signature verification
  - Settled by: the owner

## Short prompts

Choices for the [shorter prompts plan](short-prompts.md#proposal), to discuss with the owner.

- ❓ Q183 Where the protocol card lives
  - Options: inline in the MCP instructions, read once at start; or a short doc the instructions tell the session to read; or both, with the instructions holding the minimum
  - Impact: inline costs its bytes on every session start, but cannot be skipped

- ❓ Q184 How strict the message style is
  - Options: guidance in the card only; or also a soft size hint from the face, such as a warning when a body repeats the push header or exceeds a length
  - Impact: guidance alone is free but easy to ignore; a face check is code for a style rule

## Agent capabilities

- ❓ Q191 How an agent says which asks it answers, so callers stop asking agents that cannot answer
  - Context: the web and doctor send `help-json`, `help-config`, `help-secret` and `mcp` to every agent; a plain agent or a live AI session cannot answer some of them and the caller waits out its deadline (owner, 2026-10-26: "decide how to pass what an agent can do, so we do not ask dumb agents what they cannot do"). [RT23](TODO.md#bundled-agents)
  - Minimum (owner): every agent MUST answer `help`
  - Options: the `help` answer carries one machine-readable capabilities line, for example `Asks: help help-json help-config help-secret mcp`, and callers read it before asking further (owner's "read capabilities from --help", recommended, no new ask or record field); or a separate `capabilities` ask returning the list as JSON; or the list stored on the record at registration so callers need no round trip
  - Owner, 2026-10-26: the capabilities line in `help` must be both machine-readable and human-readable. Sketch to refine: one line of its own, a fixed prefix and comma-separated ask names, each with a short gloss in parentheses, for example `Also answers: help-json (methods as JSON), help-config (settings), help-secret (environment names), mcp (MCP tools)`; a reader takes the words before each `(`. Still to settle: the exact prefix, whether `help` itself is listed, how an unknown future ask is ignored, and where the line sits in the help text
  - Owner, 2026-10-26: add an optional `status` the agent may declare in that line; the web shows it and may offer actions based on it. Settled by the owner: `status` is its own ask, like `help`; its answer is a state word followed by short text (`degraded: Slack socket reconnecting`)
  - Still open: the fixed set of state words. How the industry does it: two separate axes, lifecycle and health. systemd ActiveState has `activating`, `active`, `deactivating`, `inactive` and `failed`; Kubernetes splits startup, liveness and readiness probes from pod phases (`Pending`, `Running`, `Failed`, `Unknown`); health checks use `pass`/`warn`/`fail` (IETF health-check draft), `UP`/`DOWN`/`OUT_OF_SERVICE`/`UNKNOWN` (Spring Boot) or `OK`/`WARNING`/`CRITICAL`/`UNKNOWN` (Nagios); gRPC health has `SERVING`, `NOT_SERVING` and `UNKNOWN`
  - Settled by the owner, 2026-10-26: the answer format is exactly `word: text`
  - Settled by the owner, 2026-10-26: the six words `starting`, `stopping`, `ok`, `degraded`, `failed`, `unknown`; the web asks `status` only while someone views that agent's page, no background polling
  - Settled by the owner, 2026-10-26: expose both to bus MCP users: `status` joins the `ask` values of `ab_send` (next to help, help-json, help-config, help-secret, mcp), and the MCP instructions say to read the help capabilities line before other asks. Still open: whether `ab_ls` also shows a status, which would mean asking every listed agent
  - Proposal, recommended: one word from six, joining both axes as systemd does: lifecycle `starting` and `stopping` (the owner's "initializing / finishing"; `starting`/`stopping` is the commoner spelling), and health `ok`, `degraded` (Nagios WARNING), `failed` (CRITICAL) and `unknown`. Lowercase, exactly one first word, `:` then the text (`starting: loading 3 subscriptions`); any other first word reads as `unknown`. The web shows `starting` and `stopping` as neutral, `degraded` as a warning and `failed` as an alarm; a manager may be offered restart on `failed`. Still open: which actions each state may drive
  - Settled by the owner, 2026-10-26 ([RD116](DECISIONS.md#recorded-decisions)): the runner answers from metadata what it can; any other ask, `status` included, reaches the script as `AGENT_BUS_ASK=<ask>` with empty input. A script that knows nothing of asks runs with `--no-asks` ([RD117](DECISIONS.md#recorded-decisions)). The runner answers `status` itself when it can ([RD118](DECISIONS.md#recorded-decisions))
  - Impact: the help line keeps the daemon out of describing agents (constitution § agent self-description) and needs no new wire field, at the cost of one `help` round trip before any further ask; a record field saves that trip but makes the daemon carry agent self-description

## RAG service

- ❓ Q194 Which keyword half of [RAG](rag/README.md#rules) hybrid search wins the trial on the owner's real Russian and English texts: Qdrant's built-in BM25 or the agent's own sparse vectors with stemming
  - Settled by: the [R.5 trial](rag/TODO.md#steps), then the owner
