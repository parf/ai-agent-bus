# Review of 0.9.10 through 0.9.24

Historical review, 2026-10-19. The fixed range is `dd889033..f46817bf`
(0.9.10 through 0.9.24); “HEAD” in the filename means that endpoint, not a
moving approval. Review was source inspection and isolated local probes.
No live service, record, credential or repository was modified.

**Result: two reproduced P2 findings and one reproduced P3; no P1.**
No production fixes are included in this report. Line numbers refer to
`f46817bf`. S.10 was still uncommitted: its shared-router snapshot was inspected
for the same defects, but is not included in the reviewed release range.

## Findings

### P2 — MCP-shaped answers never reach the Telegram conversation

Locations: `src/agents/telegram/router.ts:122–124`,
`src/clients/typescript/src/bus.ts:159–169`,
`src/mcp/server.ts:346,395–400`.

An incoming Telegram question is sent to a registered AI session. That session
answers through the supplied MCP `ab_send` route with the original topic and
tag. MCP sends no `re`; `ab_reply` likewise sends only destination, body, topic
and tag. The TypeScript server dispatches an answer to `onReply` only when it
has `re`, and the router independently requires `re` to find its saved route.
Consequently the valid correlated answer is treated as ordinary work or dropped,
and the human receives no answer. Requiring a field the instructed answering
face does not send breaks the main AI conversation path.

**Reproduced:** production `Router` with user `42`, a configured destination
`#claude/session@h`, and fake bus/Telegram transports. After routing question 1,
submit an answer from that destination with exactly the emitted topic/tag but
no `re`: zero Telegram posts, and “dropped unmatched session reply” is logged.
Submit the same correlation with `re: "request-id"`: one post. Source tracing
establishes that MCP emits the first shape; this was not a live Telegram test.

The same requirement was present in S.10's frozen shared extraction. Codex #2
copied and independently reproduced the probe before beginning a fix. That fix
was still undergoing checks when this report was written; this finding is not
closed by an unreviewed working-tree change.

### P2 — A failed forwarding attempt becomes a permanently seen update

Locations: `src/agents/telegram/receive.ts:172–178`, and the poll's flush/offset
advance at `receive.ts:115–122`.

`absorb` marks a message seen before awaiting its handler. If the new routing
handler fails before the bus accepts a send, the outer poll retries the batch.
On that retry the seen check skips the handler. The loop then flushes the seen
state and advances the Telegram offset: the question was delivered nowhere,
and normal retry can no longer recover it. This is a definite pre-send failure,
not the unavoidable uncertain-delivery case where retry might duplicate a send.

**Reproduced:** production `Receiver` and `Seen`, with a fake poll returning
update 10 until its offset advances. The handler's first invocation throws
“bus unavailable before send”; a second invocation would deliver successfully.
Observed result: `attempts=1`, `delivered=0`, persisted `offset="11"`.
Only retry sleeps were accelerated; no Telegram or live bus request was made.

The mark-before-handler pattern predates this range. The new Telegram routing
uses that existing receive path and inherits the loss; this review does not
claim TG.9 introduced the original dedup implementation. Codex #2 reproduced
this case and is addressing it in S.10 alongside the shared routing work.

### P3 — An in-flight acknowledgement retains an expired runner slot

Locations: `src/cmd/agent-bus-runner/serve.go:676–685,792`,
`src/cmd/agent-bus-runner/main.go:531–534`.

RD103 correctly delays an ack and joins it before sending an answer. However,
the ack uses `client.post`, whose independent background timeout is 30 seconds.
If the ack request stalls after the two-second timer fires, `finishAck` waits
for it even after the script/message deadline expires. `runOne` therefore
retains busy/concurrency capacity after the caller has given up. An expired
script is killed; the defect is the retained slot, not late script execution.

**Reproduced under `go test -race`:** an isolated runner/served instance, a
2.2-second envelope deadline, a five-second script, and a transport that returns
Configuration immediately but blocks the ack's `/send`. At 2.5 seconds,
`runOne` had not returned. The ack context had approximately 30 seconds remaining
when it started, rather than the message's remaining 0.2 seconds. Releasing the
ack transport made `runOne` return within one second. The probe passed without
a race report and joined all its goroutines.

The background receipt transport is older than RD103; joining the delayed ack
exposes this remaining deadline gap. This is separate from the Configuration
fetch deadline defect fixed earlier in this range.

## Coverage and verification

| Area | Review and evidence |
|---|---|
| Telegram TG.9/TG.10 and shared routing | Traced sender authority, chat-local fallback, `anyone`, channel announcements, tag/reply matching, dictionary ownership and receive persistence. Router/Telegram/MCP focused tests: 18 passed, 139 assertions. Two additional defect probes above |
| Git B.10 and autopull | Traced per-call closed Configuration, declared aliases, cumulative role/cap checks before git, option separation, trusted paths, branch fallback, key handling, private process-group cancellation and timer configuration reads. Git unit suite: 15 passed |
| Systemd B.9 | Traced malformed Configuration refusal, trimmed scope, inventory operands, journal `-u` selection, membership before execution and unchanged delivered-role tiers. Unit suite: 20 passed |
| Cgroup kill workaround | Inspected killed-group tracking, bounded empty wait, removal/recreation, limit reapplication and both launch paths. Cgroup race unit suite: 3 passed. Did not rerun the privileged kernel fixture |
| Package-digest reload | Inspected digest inputs and keeper replacement condition; focused race test for restart on changed package passed |
| Earlier review fixes | Inspected modern Slack bot identity, compact Configuration cap/load refusal, call setup deadline and message-context Configuration fetch; focused Go race checks passed |
| RD103 | Inspected instructions/tool descriptions/footer and timer join/cancel paths, plus deadline probe above. Fast/slow receipt tests included in its implementation gate; no second full gate claimed here |
| Docs and simplicity | Compared current agent README/help and permission/configuration descriptions with implementation. No additional concrete mismatch or justified simplification finding beyond the routing contract failure |

Commands run from the isolated review checkout included:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s src/agents/standalone/git -p test_git.py
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s src/agents/standalone/systemd -p 'test*.py'
bun test src/agents/telegram/router.test.ts src/agents/telegram/t10.test.ts src/agents/telegram/maps.test.ts src/mcp/messages.test.ts
cd src
go test -race ./internal/core ./cmd/agent-bus ./cmd/agent-bus-runner -run 'Config|Tagged|ReloadRestartsRepackaged|ReviewSlowAck' -count=1
go test -race ./internal/cgroup -count=1 -v
```

The temporary reproduction tests assert the faulty observations, not desired
behavior. Their passing results mean reproduced defects, not feature approval.
Their inputs and observations are recorded above; scratch is removed under the
repository cleanup rule. Both router probes were handed to Codex #2 first.

Mutation-first evidence was audited in the committed implementation reports:
B.9 names acting/journal/malformed-scope mutants; B.10 names cap/role/alias
mutants; TG.10 names seven route/authority/prototype mutants; cgroup reuse names
the real kernel probe and reuse mutant. RD103's immediate-ack and old-footer
mutants failed named assertions under their slow shards before landing, with a
fixed full gate of 1136/0. Those are implementation evidence, not a claim that
this review independently reran every historical mutant. The two router probes
show missing coverage despite those passing suites.

No full `--slow` gate or live mutation was performed for this report-only review.
Passing focused checks do not close the three findings or prove the absence of
other defects. Further changes, including S.10, require their own verification.
