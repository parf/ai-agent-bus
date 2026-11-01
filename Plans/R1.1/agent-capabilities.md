# Agent capabilities

📌 **TL;DR:** RT23 moved to R1.1 on 2026-11-01 (Q299); the status policy is settled by Q191. Existing help and status behavior is not removed.

## Proposal

Every agent answers `help` as the minimum; callers learn which further asks it answers before sending them. An agent may also declare `status`, shown by the web with any approved actions. The same discovery applies to MCP users. Q191 is [settled](DECISIONS.md#recorded-decisions): status is shown only on the agent’s own page, while a visitor views it; no mass polling and no status in `ab_ls`. The web offers restart on `failed` only. These requirements are planned here; the move does not claim the remaining RT23 implementation is built.

[RT23 acceptance](TODO.md#agent-capabilities) follows that decision; today’s contract remains [agent self-description](../../docs/constitution.md#agent-self-description).
