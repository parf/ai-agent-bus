# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions).

## Distribution

Owner-only choices for the [Linux distribution](distribution.md#simple-install) ([RT14](TODO.md#objective)). None blocks the Linux work: each names what stays undone until it is answered.

- ❓ Q226 Who enables Actions and publishes the release
  - Context: the build on a version tag into a draft release is settled ([RD163](DECISIONS.md#recorded-decisions))
  - Options: the owner makes `parf/ai-agent-bus` public, enables Actions and presses publish after signing `SHA256SUMS`; or the build runs on this node and the owner uploads the assets
  - Impact: the published `install.sh` URL and every download exist only after the owner publishes; the team never pushes, uploads or publishes
  - Settled by: the owner

## Messaging agents

Owner-only choices for the two messaging agents planned for R1 ([B.27](TODO.md#bundled-agents): Discord and Google Chat). The technical choices are decided ([RD172](DECISIONS.md#recorded-decisions), [RD173](DECISIONS.md#recorded-decisions)); Teams, WhatsApp and Messenger wait in [R2.0](../R2.0-Future/QUESTIONS.md#open-questions). No sign-up or live call was made.

- ❓ Q244 Discord: who creates and owns the application and bot, for which server
  - Context: Needed before any live test; no sign-up was made
  - Proposal: The owner names the account and the server
  - Impact: none of the Discord agent can be tried live before this
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
