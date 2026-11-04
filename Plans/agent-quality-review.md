# Bundled agent quality review

📌 **TL;DR:** The [agent checklist](../docs/10-writing-an-agent.md#absolute-minimum--every-agent)
separates essential behavior, safety required by an agent’s capabilities, and useful optional features.
The source review covers every shipped package and the built-in `#bus`. Three help-contract gaps
need correction; this review does not certify unexecuted provider or lifecycle acceptance.

## Scope and evidence

Reviewed against `f308c291` on 2026-11-03 by two independent Astra inventory reviewers.
The inventory comes from [the release exporter](../src/agents/package.sh) and
[setup’s bundle contract](../src/cmd/agent-bus-setup/package_install.go), not directory names.
There are 20 bundled packages and one built-in agent. DB and Discord have individual source
packagers, but are absent from the release exporter and bundle contract.

“Inspected” below means both implementation and owning test source were read. It does not mean
those tests ran or that an external provider was contacted. A test source alone is not proof
that it passes or can detect a regression. Focused executed results are recorded separately below.

## Coverage by agent

| Agent | Applicable checks inspected | Owning tests under src/agents/ | Remaining concern |
|---|---|---|---|
| bus | Caller visibility, asks, reply route, log subscriptions | ../internal/api/daemon_agent_test.go; daemon_agent_json_test.go | Separate daemon subscription authority profile remains deliberate. |
| ai | Ask-first, secret redaction, provider deadline and response limits | ai/test_ai.py | Explain uncertain paid-request outcomes more clearly. |
| at | Scheduling authority, send-once claims, packaged serving | at/test_at.py; cron/test_cron.py | No general pub/sub feature is needed for one-shot scheduling. |
| cron | Creator reach, delivery refusal, restart/send-once state | cron/test_cron.py | No gap established by static review. |
| debug | Delivered roles, dangerous-method refusal, request budget, drain | debug/test_metadata.py; test_required.py; test_doctor.py | Bus health is not another agent’s business acceptance. |
| eventlog | Role checks, rings, deadlines, revoked subscription, shutdown | eventlog/test_eventlog.py | No gap established by static review. |
| fetch | Grammar, network policy, credential isolation, bounded child output | fetch/test_runtime.py | No gap established by static review. |
| git | Scoped native help, role/cap intersection, subprocess timeout | standalone/git/test_git.py; test_autopull*.py | Full kept-mode lifecycle remains unexecuted in this audit. |
| imap | Secret/config validation, destructive opt-in, forwarding, stop | imap/test_runtime.py; test_budget.py; test_forwarding*.py; test_subscriptions_bus.py | No gap established by static review. |
| logwatch | Validated filters, creator reach, rings, worker stop, migration | standalone/logwatch/test_logwatch.py; test_delivery.py | No gap established by static review. |
| qdrant | Route roles, pre-connect refusal, redaction, uncertain writes | qdrant/test_qdrant.py | No gap established by static review. |
| rag | Ask-first, downstream budget, authority, interrupted writes, drain | rag/test_runtime.py; test_rag.py | No gap established by static review. |
| server-info | Fixed commands, account scope, timeout, help | standalone/server-info/test_server_info.py; test_metadata.py | Machine-help scope mismatch; local output has no byte cap. |
| slack | Ask-first, deadlines, forwarding authority/counts, shutdown | slack/subscription-port.test.ts; forward.test.ts | No gap established by static review. |
| sms-telnyx | Ask-first, one provider POST, uncertainty, redaction, deadline | sms-telnyx/test_sms.py | No gap established by static review. |
| smtp | TLS, redaction, uncertainty/no retry, deadline, asks | smtp/test_smtp.py | Declared body help is not implemented. |
| systemd | Delivered-role tiers, configured scope, fixed argv, bounded logs | standalone/systemd/test_scope.py; test_systemd.py; test_logs.py | Per-message lifecycle belongs to the runner. |
| telegram | Ask-first, deadlines, retry rules, author reach, stopping | telegram/t4.test.ts; forward.test.ts | No gap established by static review. |
| tmux-sessions | Configured names, fixed argv, stripped environment, PID identity | standalone/tmux-sessions/test_sessions.py; test_metadata.py | Declared body help is not implemented; grammar test masks it. |
| voyage | Fixed provider origin, grammar, secret redaction, response/deadline bounds | voyage/test_voyage.py | No gap established by static review. |
| webhook | Loopback default, scoped destinations, asks/roles, request drain | webhook/test_webhook.py; test_runtime.py; test_outgoing.py | No gap established by static review. |

## Corrections

The three verified source-level help inconsistencies are tracked separately in
[the review findings](issues/2026-11/2026-11-03.md#bundled-agents).
They concern advertised behavior, not an instruction to add features to every agent.

## Useful improvements, in priority order

1. **Bound server-info output locally.** Commands have a time limit, but capture_output buffers all
   bytes before the runner can limit the reply. A byte cap with explicit truncation protects a
   large host inventory. This is a robustness improvement, not an unbounded-runtime finding.
2. **Clarify AI request uncertainty.** A provider timeout may follow a paid or completed request;
   tell callers not to automatically repeat it. No automatic retry was found in the implementation.
3. **Retain operational events selectively.** Retained workers benefit from startup failure,
   recovery and dropped-delivery diagnostics. Do not add credentials to per-message scripts
   merely to write logs, or log every successful read.

Status is useful for retained workers with meaningful activity or backlog. Pub/sub is useful
for event sources; KV for real durable progress. Neither is a universal checkbox. Roles,
locks and bigfile support are needed only where the agent’s operations call for them.

## Executed checks

An independent reviewer ran **24 focused tests, all passing**:

- Six Python metadata tests, each in its own process, for git, systemd, server-info, logwatch,
  tmux-sessions and debug: `python3 -m unittest discover -s src/agents/<directory> -p test_metadata.py -v`.
- One shared golden test covering 22 fixtures: `python3 -m unittest discover -s src/agents/shared -p test_help_golden.py -v`.
- Nine Bun metadata tests, 159 assertions: `bun test ./src/agents/slack/metadata.test.ts ./src/agents/telegram/metadata.test.ts`.
- Six catalogue tests: `python3 -m unittest discover -s src/agents -p agent_docs_test.py -v`.
- AI and SMS each passed their `AskDispatch` test, exercising seven asks without provider setup.

Python runs used `PYTHONDONTWRITEBYTECODE=1`. The server-info collector was mocked; tmux status
used invalid fixture settings and returned before running tmux. These checks do not contradict
L1–L3: the identified missing body/scope assertions are not covered by the passing checks.

At, cron, eventlog, fetch, imap, qdrant, rag, smtp, voyage and webhook received golden rendering
and documentation checks here, not executed serving acceptance. DB and Discord golden fixtures
do not make either agent release-bundled. No provider calls, live daemon operations, host changes,
normal gates or mutation claims are part of this run.
