# Review of 0.9.0–0.9.10

Historical review, 2026-10-19. Reviewed `1402a98..dd889033`, with runtime
probes against a detached `dd889033` checkout. This report records findings,
not fixes or approval of a subsequent revision. No live service, record,
credential or repository was changed during the review.

**Result: two P2 and two P3 findings, all reproduced; no reproduced P1.**
Source line numbers below refer to `dd889033`.

## Findings

### P2 — Bot author filters silently discard current Slack app messages

Location: `src/agents/slack/forward.ts:139–140`; normalization:
`src/agents/slack/slack.ts:242` (0.9.10).

A subscription selecting a bot by name or user ID stores its `profile.bot_id`,
for example `BBUILD`. The forwarder uses that ID only when the event has
`subtype: "bot_message"`. A current Slack app can instead send a message with
`user: "UBUILD"`, `bot_id: "BBUILD"` and `bot_profile`, without that subtype.
The forwarder then compares `UBUILD` with `BBUILD` and drops the message.
This breaks the bot subscription use case even though creation succeeds.

**Reproduced:** the actual `Forwarder`, with `from_users: ["BBUILD"]`, received
one such modern event and one classic `bot_message` event from the same bot.
After its drain completed, the only forwarded text was `"classic"`; `"modern"`
was absent. No Slack message was sent. This is a simulated documented payload,
not an observation of a particular live workspace event.

[Slack's event documentation](https://docs.slack.dev/reference/events/message/bot_message/)
distinguishes classic bot-message events from current apps identified by
`bot_id` and `bot_profile`. The existing S.9 mutation tests exercise the classic
shape and do not catch this mismatch.

Evidence: `tmp/rv09/tmp/review-slack.test.ts` and `review-slack.log`.
Run from `tmp/rv09`: `bun test tmp/review-slack.test.ts`.
The probe asserts the observed defect, so its passing result means reproduced,
not that the intended behavior passes.

### P2 — Accepted Configuration can make every script start fail

Location: `src/internal/script/config.go:26`, used by
`src/cmd/agent-bus-runner/serve.go:692` and the CLI run path (0.9.6).

Every non-null Configuration becomes one `AGENT_BUS_CONFIG` environment string.
The daemon accepts JSON objects larger than Linux's per-string exec limit.
Consequently, an otherwise valid stored configuration can prevent execution of
any command, including one which never reads configuration. Existing records
with such configurations also become unusable when upgraded to this behavior.
The editor/save succeeds; failure appears later as an exec error.

**Reproduced on this Linux host:** production `core.Configure` accepted a
140,015-byte object (`{"settings":"` plus 140,000 `x` characters plus `"}`).
Production `script.ConfigEnv` accepted it. Executing `/bin/true` with the returned
environment failed with `argument list too long`. This establishes the storage,
environment and real OS execution failure; it is not a deployed-agent test.
There is no corresponding size refusal in this path or documented usable limit.

Evidence: `TestReviewConfigEnvironmentLimit` in
`tmp/rv09/tmp/review_cli_test.go`, output in `review-cli.log`.
A fix needs a deliberate policy for oversized existing and new configurations;
simply documenting successful storage would not address the execution failure.

### P3 — Tagged-call setup is outside the advertised shared deadline

Location: `src/cmd/agent-bus/call.go:109–113` (0.9.4);
contract: `docs/user/cli.md`, “Calling many agents at once”.

`callTagged` calls `callAddress` before creating its timeout context. Caller
identity/lookup/registration therefore use their independent transport bounds.
A slow daemon can consume longer than `--wait` before tagged discovery or any
exchange starts. The exchanges themselves share a deadline correctly; the gap
is the setup phase, not serialization of target calls or repeated sends.

**Reproduced:** with an isolated HTTP fixture delaying `/lookup` by 300 ms,
`callTagged(..., --wait=30ms)` returned after 301 ms. Identity was stubbed so the
measurement isolates just the real registration-check path. It returned the
empty-list error after the delayed lookup, rather than honoring 30 ms end to end.

Evidence: `TestReviewTaggedSetupDeadline` in
`tmp/rv09/tmp/review_cli_test.go`, output in `review-cli.log`.

### P3 — Runner Configuration fetch ignores the message deadline

Location: `src/cmd/agent-bus-runner/serve.go:134–137,692` (0.9.6).

`runOne` derives a context from the agent and message deadlines, but its new
Configuration fetch creates a separate background context with a 30-second
limit. A delayed `/config` response holds the runner's busy/concurrency slot
past the caller's deadline. The later command context prevents execution after
expiry; the defect is retained worker capacity and delayed completion, not
execution of an expired request.

**Reproduced:** the real `runOne`, an agent deadline and envelope deadline of
20 ms, and an isolated HTTP fixture delaying `/config` by 300 ms. `runOne`
returned after 301 ms. The transport and command were local fixtures; no live
runner was involved.

Evidence: `TestReviewConfigurationIgnoresDeadline` in
`tmp/rv09/tmp/review_runner_test.go`, output in `review-runner.log`.

## Reproduction and verification

The Go probes use an overlay, leaving reviewed source unchanged. From
`tmp/rv09/src`:

```sh
go test -overlay=../tmp/overlay.json -run '^TestReview' -v ./cmd/agent-bus ./cmd/agent-bus-runner
```

These tests assert the faulty observations described above. Probe inputs are
synthetic and contain no credentials. Scratch evidence is retained under
`tmp/rv09/tmp`; it is not a permanent regression suite.

The independent full `--slow` attempt produced **541 passed, 1 failed**:
the core shard could not bind port 16500 because it was already occupied.
This is an environmental failure, not a product finding, and is not credited
as a green full gate. Every other shard completed, including Go race checks,
web contracts, launcher tests, client conformance and all bundled-agent checks.
See `tmp/rv09/tmp/review-slow.log`. The focused core rerun on free port 18500 passed **595/0**, recorded
in `tmp/rv09/tmp/review-core.log`. Thus all 1,136 checks passed across the
original run and core rerun; this was not one uninterrupted green full gate.

## Coverage and mutation evidence

Reviewed the behavior and associated tests/docs across the complete range:

| Version | Review focus |
|---|---|
| 0.9.0 | Metadata cut, historical changelog relocation and current-version links |
| 0.9.1 | Populated atomic lock publication, exact owner-file stale cleanup, legacy PID compatibility, fixed-name durability |
| 0.9.2 | Required descriptions, standalone metadata/ExecStart, help, bare-metal virt result |
| 0.9.3 | All-unit systemd scope, strict arguments, delivered role tiers, journal bounds, tags and discovery docs |
| 0.9.4 | Visible tagged-agent selection, fresh correlations, one send per target, concurrent shared deadline, unchanged plain correlation tag |
| 0.9.5 | Empty failed-unit text versus JSON result |
| 0.9.6 | Owner/Maintainer configuration access, Secret separation, lexical JSON formatting, retained invalid edits, environment replacement and kept-process reload |
| 0.9.7 | Listener ownership before credentials, descendant proof, fail-closed lookup, retry/cleanup, malformed session listings, bytecode suppression |
| 0.9.8 | Fixed repository mappings, delivered roles, option/path/revision boundaries, server Git/SSH identity, output/time bounds and transport-child cancellation |
| 0.9.9 | Signal registration before readiness/Serve and graceful shutdown ordering |
| 0.9.10 | Author resolution before writes, unknown/deleted refusal, edit/clear semantics, author AND regex filtering and existing destination authority |

Also inspected the Go socket test-fixture sweep and version-comparison fixture
change. They do not alter runtime behavior.

Existing mutation evidence was inspected rather than treating a green suite as
proof. Examples retained on this node: `tmp/lr/tmp/lock-review-mutant.log`
(old publication/stale cleanup), `tmp/cmm/tmp/cm-mutant-{selector,capped}.log`,
`tmp/c3gm/tmp/mutant-{options,body-role,timeout}.log`,
`tmp/c3sig/tmp/signal-mutant-slow.log`, and
`tmp/c3sm/tmp/s9-mutant-{ignored-author,unresolved-author,any-bot}.log`.
These show named assertion failures, rather than compiler failures. They are
prior execution evidence; this review did not rerun every historical mutant.
The new Slack finding demonstrates a payload-shape gap despite that evidence.

No new token, role, ACL or shell-option bypass was reproduced. This is a review
result, not a proof that every possible bypass is absent. The launcher's
post-ownership runtime-death/rebind window is already explicitly documented and
accepted; it is not presented here as a newly reproduced exploit. Systemd's
whole-journal operate access and git deploy's destructive operate tier are
explicit owner decisions, not accidental privilege findings. No extra framework
or production change was introduced during review.
