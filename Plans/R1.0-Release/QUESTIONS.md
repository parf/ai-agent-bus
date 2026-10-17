# R1 questions

## Open questions

Only unresolved choices. The remaining item is the unanswered applicability
choice in №Q146. Accepted answers and their complete threads are in
[decisions](DECISIONS.md#recorded-decisions), including [№Q17](DECISIONS.md#recorded-decisions), [№Q20](DECISIONS.md#recorded-decisions), [№Q137](DECISIONS.md#recorded-decisions),
[№Q145](DECISIONS.md#recorded-decisions), [№Q147](DECISIONS.md#recorded-decisions) and [№RQ1](DECISIONS.md#recorded-decisions). Existing question IDs are retained; none are allocated
by this update.

- ❓ Q146 Which record kinds may carry assigned roles?
  - Details: representation and duplicate-grant behavior are recorded as [№RD92](DECISIONS.md#recorded-decisions); this question asks only applicability, which was left unchecked
  - State: owner selected the representation and duplicate-grant checkboxes on 2026-10-17; applicability remains unchecked
  - Settled by: owner
  - Home: [assigned roles](#assigned-roles)
  - ( ) Permit assignments on ACL-bearing records, excluding User records, Group membership and Maintainers 🌟
  - ( ) Restrict assignments to Agent records; queue/pubsub ACLs would no longer assign application roles ⭐
  > 💬 👤parf elaborate
  >
  > 💬👾 What is built today, so there is something concrete to confirm or
  > reject. **Representation:** labels in parentheses after an allow-list term,
  > in the existing `allow` list and not a new field — `bob(admin)`,
  > `@dev(deploy, read_only)`, `*(guest)`. **Duplicate grants for one actor:**
  > `bob(deploy)` and `bob(read_only)` may both stand and their labels combine;
  > an identical repeated term is stored once; terms differing only in role
  > order can remain separate on addition, delivery deduplicates the labels,
  > and removal matches the whole role set. **Where they are allowed:** any
  > record with an ACL, including a queue or pubsub channel that forwards to an
  > Agent — not Group membership or the Maintainers list, which take actors
  > only, and not User records, which have no ACL. **Order on the message:**
  > generated roles first, then assigned, each label once, in first-match
  > order. **Reserved to the daemon:** `owner` and `maintainer` are generated,
  > and assigning either in an ACL rejects the whole save. Source:
  > [roles](../../docs/03-records-agent.md#roles).
  >
  > ✅ 💡👾 Recommend confirming all three aspects together. They match the implemented, tested behavior and preserve existing role-free ACLs without another stored representation. Generated `owner` and `maintainer` remain reserved. If one aspect is unsuitable, leave it unchecked and explain the desired behavior before changing the contract.
  >
  > 💬 👤parf agree
  >
  > 💬👾 The first two checked aspects are recorded as [№RD92](DECISIONS.md#recorded-decisions). The third remains unchecked, so this item stays open only for which records may carry assignments. No implementation behavior changes while that choice remains open.
  >
  > 💡👾 Recommended: keep assignments on ACL-bearing records 🌟. This matches the current contract and lets a queue or pubsub channel grant the roles carried by a forwarded call; User records, membership and Maintainers remain excluded. The runner-up is Agent-only assignment ⭐: a narrower surface, but it removes queue/pubsub application-role grants and requires a contract and implementation change. Which scope should we keep?


## Record tags

[№Q145 is settled](DECISIONS.md#recorded-decisions): the accepted query rule is
in [record tags](../../docs/03-records.md#tags); its unfinished implementation
is [№RTA1](TODO.md#record-tags-acceptance).

## Assigned roles

№Q146 above retains the complete thread. Its first two checkbox selections
are preserved in the decision record; the previously unchecked applicability
rule is now the explicit pick-one question above.
The selected representation and duplicate-grant behavior are recorded as
[№RD92](DECISIONS.md#recorded-decisions); the current behavior is documented in
[roles](../../docs/03-records-agent.md#roles).

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
