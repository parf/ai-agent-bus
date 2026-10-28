# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions).

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

- ❓ Q201 Which Qdrant administrative API operations the transparent proxy exposes
  - R.4 initially says any Qdrant API path. Snapshot recovery/upload and shard recovery can make the store fetch URLs or read local files; cluster administration can change peers. Other administrative roots include telemetry, metrics, issues and locks. The proxy's runner sandbox does not constrain the separate store process.
  - Recommended: keep the requested transparent full-store-admin block; default ACL `@owner`, with an explicit warning that any grant conveys the API key's full authority, including arbitrary collection deletion and store-host-affecting operations. No per-verb or per-collection promise.
  - Alternative: deny recovery/upload and cluster operations, with an explicit API-subset contract and owning mutants. This narrows the original “any API path” scope; the owner chooses, not the reviewer.
  - Raised by the [R.4 plan review](rag/TODO.md#steps), 2026-10-28; awaiting owner through AB2.

- ❓ Q202 Whether the Qdrant proxy accepts an operator-selected remote HTTP URL
  - Plain HTTP to a remote store carries `api-key` unencrypted. RD131 allows the store anywhere and says the operator chooses its mapping; HTTPS uses normal certificate verification.
  - Recommended: permit the explicitly configured HTTP(S) URL, document cleartext transport, and emit one safe notice per invocation for non-loopback HTTP (the runner is per-message, so there is no persistent one-time process state). No URL query/body/key in the notice.
  - Alternative: require HTTPS off loopback unless an explicit Configuration opt-in allows cleartext HTTP. This changes the Configuration contract and needs the owner's decision before implementation.
  - Raised by the [R.4 plan review](rag/TODO.md#steps), 2026-10-28; awaiting owner through AB2.
