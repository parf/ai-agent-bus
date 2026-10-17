# R1 questions

## Open questions

Only unresolved choices, and none is open. Accepted answers and their complete threads are in
[decisions](DECISIONS.md#recorded-decisions), including [№Q17](DECISIONS.md#recorded-decisions), [№Q20](DECISIONS.md#recorded-decisions), [№Q137](DECISIONS.md#recorded-decisions),
[№Q145](DECISIONS.md#recorded-decisions), [№Q146](DECISIONS.md#recorded-decisions), [№Q147](DECISIONS.md#recorded-decisions) and [№RQ1](DECISIONS.md#recorded-decisions). Existing question IDs are retained; none are allocated
by this update.

- ❓ Q175 Is a bundled agent served as soon as it arrives (№M.12), or only once the host enables it in `agents.json`?
  - Settled by: owner, before [№M.12](TODO.md#runner-waves)
  - Home: [№M.12](TODO.md#runner-waves)
  - ( ) Opt-in: a bundled agent is listed in `status` as available and served only after `agents.json` enables it; an agent in the person's own `agent.d` stays enabled by default 🌟
  - ( ) Served at once, like a directory in `agent.d`; the host disables what it does not want ⭐
  - ( ) Opt-in per agent: its `agent.json` says whether it starts on arrival
  > 💡👾 Recommend opt-in 🌟. Every person's runner would otherwise start every bundled agent the moment an upgrade brings it: the Slack agent with no secret restarting in a loop, the debug agent answering `env` to whoever its ACL admits. Copying an agent into your own `agent.d` stays the one-step way to run it. Served at once ⭐ is simpler, but each upgrade could then start something nobody asked for.

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
