# T-32 Agent-author guide completion

## Result

The owner's Q324 audience has a [current guide](../../../docs/10-writing-an-agent.md):
three ways to run an agent, minimum/suggested/optional responsibilities, and
worked CLI, runner and Python-process pages quoting executed sources.
The [plan](../agent-guide.md#3-ways-to-run-agents) is historical design.

## Landed evidence

| Part | Commit |
|---|---|
| Approved plan and verification budget | `69228913` |
| Guide and five rules in their owning current docs | `50fbd5b3` |
| Executed example packages and isolated-bus shard | `4ad6a9ab` |
| Three worked pages and fail-closed source/output checks | `99035670` |
| Runner checks back at normal under Q327; deliberate CPU enforcement stays slow | `14cc6cf4` |
| Current severity Levels home promoted | `87f3b80b` |

## Completed acceptance

The checklist concerns decisions in agent code: minimum help/answer, suggested
status and Configuration/Secret separation, and optional capabilities linked to
their owners. Shared examples cover the same time service across three execution
modes without giving per-message scripts credentials.

One isolated-bus shard checks the examples, source quotes and asserted output,
with H/F/K/N/M selection. Normal includes packaged-runner checks; deliberate
over-limit enforcement remains slow. Q327 permits systemd's own scope notice;
our programs still write nothing to the host journal at fast/normal.

The former severity and own-log gaps now point to current
[Levels](../../../docs/agents/eventlog.md#levels) and
[Agent logs](../../../docs/agents/bus.md#agent-logs). The guide retains no
pending-promotion notice or link to the Levels plan as its current reference.
