# R1 questions

## Open questions

Only unresolved choices other than the above. Accepted answers and their complete threads are in
[decisions](DECISIONS.md#recorded-decisions), including [№Q17](DECISIONS.md#recorded-decisions), [№Q20](DECISIONS.md#recorded-decisions), [№Q137](DECISIONS.md#recorded-decisions),
[№Q145](DECISIONS.md#recorded-decisions), [№Q146](DECISIONS.md#recorded-decisions), [№Q147](DECISIONS.md#recorded-decisions) and [№RQ1](DECISIONS.md#recorded-decisions). Existing question IDs are retained; none are allocated
by this update.

## Record tags

[№Q145 is settled](DECISIONS.md#recorded-decisions): the accepted query rule is
in [record tags](../../docs/03-records.md#tags); its unfinished implementation
is [№RTA1](TODO.md#record-tags-acceptance).

## Assigned roles

№Q146 is settled: representation and duplicate grants as
[№RD92](DECISIONS.md#recorded-decisions), applicability as
[№RD95](DECISIONS.md#recorded-decisions), which carries the complete thread. The
behavior is documented in [roles](../../docs/03-records-agent.md#roles).

[№Q147 is settled as №RD87](DECISIONS.md#recorded-decisions); the remaining
non-recursive group walks are [№RGW1](TODO.md#group-walk-acceptance), not an
unanswered membership-policy question.

## Federation context

Federation moved to [R1.2](../R1.2/federation.md#chaining) on 2026-10-17
([№Q137](DECISIONS.md#recorded-decisions)). The owner also settled [№Q20](DECISIONS.md#recorded-decisions): the realm supplies the namespace;
`/` remains template/instance identity. Both full threads are preserved in
[decisions](DECISIONS.md#recorded-decisions).

## Split context

The earlier R1/R1.1 dependency conflict for chaining is closed by the move to
[R1.2](../R1.2/README.md#scope), recorded as
[№RD91](DECISIONS.md#recorded-decisions). It no longer blocks R1.

## Runner context

The runner identity, environment and correlation choices have been recorded
in [decisions](DECISIONS.md#recorded-decisions); implementation is tracked in
[runner waves](TODO.md#runner-waves).

## Agents for the runner

`RQ` labels an existing owner choice without an allocated Q number.
[№RQ1 is settled as №RD93](DECISIONS.md#recorded-decisions): the accepted tier
name is in [systemd](agents.md#systemd). The earlier Q168–Q170 decisions and
their replacements remain in [decision history](DECISIONS.md#superseded).
Execution stays in [bundled-agent tasks](TODO.md#bundled-agents).

## Modules context

[№Q17 is settled as №RD90](DECISIONS.md#recorded-decisions). The accepted
conformance-first approach and its unfinished work are in
[protocol conformance](modules.md#protocol-conformance) and
[№RT8](TODO.md#objective). No new schema or wire format was approved.

## Short prompts

Choices for the [shorter prompts plan](short-prompts.md#proposal), to discuss with the owner.

№Q182 is settled as [№RD103](DECISIONS.md#recorded-decisions): fewer receipts, sent by the model only when the answer needs another turn.

- ❓ Q183 Where the protocol card lives
  - Options: inline in the MCP instructions, read once at start; or a short doc the instructions tell the session to read; or both, with the instructions holding the minimum
  - Impact: inline costs its bytes on every session start, but cannot be skipped

- ❓ Q184 How strict the message style is
  - Options: guidance in the card only; or also a soft size hint from the face, such as a warning when a body repeats the push header or exceeds a length
  - Impact: guidance alone is free but easy to ignore; a face check is code for a style rule
