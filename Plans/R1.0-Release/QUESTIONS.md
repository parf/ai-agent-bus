# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions).

## Distribution

Owner-only choices for the [Linux distribution](distribution.md#simple-install) ([RT14](TODO.md#objective)). None blocks the Linux work: each names what stays undone until it is answered.

- ❓ Q220 Which Mac the owner provides for macOS tests, and how the team reaches it
  - Options: a machine reachable over ssh with a named account; or the owner runs the macOS checks by hand and reports; or no Mac until later
  - Impact: without one, macOS stays unbuilt and unchecked; the macOS questions below cannot be settled ([macOS test host](TODO.md#objective))

- ❓ Q221 Which macOS parts ship and how they install
  - Options: client-only edition only (CLI, runner, launchers), as the Linux `--client`, through the same `install.sh`; or also a Homebrew formula; or an archive without a script
  - Impact: the daemon and sandbox are Linux-only today, so the process-boundary equivalent on macOS is part of this choice; settled on the Mac of Q220

- ❓ Q222 Apple Developer account for signing and notarizing macOS binaries
  - Options: the owner's existing account; a new paid account; or unsigned binaries with the quarantine attribute cleared by the installer
  - Impact: unsigned macOS binaries are refused by Gatekeeper unless the installer handles it

- ❓ Q223 The release signing key ([RD163](DECISIONS.md#recorded-decisions))
  - Options: the owner generates the ed25519 key off this node and gives the team only the public key; the namespace string; the spare second key to embed in `install.sh` for rotation, now or at the first rotation
  - Impact: until the public key exists `install.sh` carries a placeholder and no real release can be signed or installed; the team never holds the private key

- ❓ Q224 npm publication: package name and account
  - Options: `agent-bus` if free, else a scope such as `@parf/agent-bus`; the account or organization that owns it; publish by the owner's own login with two-factor, never a token on this node
  - Impact: the package, its per-platform optional packages and `npx agent-bus` use the name; P.1 can only be proven against a local registry until it is chosen

- ❓ Q225 Container image registry and name
  - Options: `ghcr.io/parf/agent-bus` beside the repository; or Docker Hub; or both
  - Impact: P.2 is proven with local images until the registry and the account that pushes are chosen

- ❓ Q226 GitHub release and Actions: who enables and publishes
  - Options: the owner makes `parf/ai-agent-bus` public and enables Actions, the tag-triggered build makes a draft release, the owner signs `SHA256SUMS` and publishes it; or the build runs on this node and the owner uploads the assets
  - Impact: the published `install.sh` URL ([RD163](DECISIONS.md#recorded-decisions)) and every download exist only after the owner publishes; the team never pushes, uploads or publishes

- ❓ Q227 When `.deb` and `.rpm` packages and their repositories start, and where they are hosted
  - Options: after the first Linux release; or never in R1; with a host such as a GitHub Pages repository or the owner's own domain
  - Impact: the "later" row of [simple install](distribution.md#simple-install); nothing is built for them before this

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
