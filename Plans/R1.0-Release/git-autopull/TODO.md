# Git autopull TODO

📌 **TL;DR:** replace `#git`'s timer autopull with channel-driven autopull and
revision deploy, as [the design](README.md#two-channels) says, then let
`#webhook` publish a hosting service's push to `autopull-cmd`. Two tasks; B.26
can land after B.25, which is testable with `agent-bus send` alone.

## Tasks

### B.25 Channel-driven autopull in `#git`

Code: `src/agents/standalone/git/`. ⚠️ parsers and a destructive mode: mutants
required. Assigned 2026-10-29: steps 1–5, 7, 8 xAB; step 6 xAB2, on top of
xAB's landed core; the owner's-flow end-to-end check AB, after B.26; review
`#claude/ab2-review`.

| Step | Work |
|---|---|
| 1 | Per-repository mode `ff`/`mirror` and follow on/off, plus the two channel names, in Configuration; `mirror` refused unless the repository's access is `deploy` or `commit` |
| 2 | Parse the [new-code event](README.md#new-code-event) strictly: `v`, known fields only, hex lengths, branch match, `remote` matched to `origin` |
| 3 | `ff` and `mirror` runs under the repository's lock; `mirror` counts what it discards |
| 4 | Triggers: `autopull-cmd` events, one check per repository at start, `autopull <repo> now`, `deploy <repo> <branch> [<rev>]`; `deploy <repo>` keeps working |
| 5 | Publish one [result record](README.md#result-record) per attempt to the configured log channel, topic `<repo>@<host>`, tag = result; no log name, no publish |
| 6 | `autopull-success-script` / `autopull-fail-script` per repository: a bare name inside `autopull.d/` in the agent's state directory, file and directory writable only by root or the agent user, checked at configuration and before each run, record on stdin after it is published, time limit; a non-zero exit, time-out or any stderr publishes one `action: script` fail record |
| 7 | Remove the install-time `--autopull` timer; reinstall removes an existing timer unit |
| 8 | Update [the git guide](../../../docs/agents/git.md#autopull) and decisions; bump, changelog |


#### Core implementation plan

- Configuration: each `repos` entry adds `mode` (`ff` default, `mirror` only with access ≥ deploy) and `follow` (boolean, false default). Agent-wide optional `autopull-cmd` and `autopull-log` are channel names; following requires the command channel and deploy access. Only following repositories get a startup check or channel events; explicit operator attempts do not require follow. Script keys remain xAB2's separate step and are not silently accepted before it lands.
- Runtime: make the standalone keeper serve its own identity with the existing Python client (shared installer's `--serves-itself`), so it can run startup catch-up and distinguish authenticated `original_to` channel delivery from ordinary calls. Reload Configuration for each message. Legacy methods/help run in a bounded child with only the delivered roles and current Configuration added to its stripped caller environment; never change global process roles. Event and operator updates share one engine in `autopull.py`.
- Parsing: strict JSON rejects duplicate keys/non-finite values, unknown fields and malformed ids/commits/version before any write. A valid, unambiguous remote hint routes invalid served events to `invalid_event`; unknown remotes produce no record. Match origin's hosting project path (URL/SCP, stripping one `.git` suffix); never publish its URL or checkout path. Branch mismatch is ignored. Text `autopull <repo> now` and `deploy <repo> [<branch> [<rev>]]` have strict JSON equivalents with `now:true`, `branch`, `rev`, and optional `id` on deploy.
- Operations: acquire the existing per-checkout file lock with a bounded wait; budget lock + git at 60 seconds, each git process group uses the remaining budget. Fetch only origin and verify reachability before checkout/reset/clean. At most three short fetch attempts inside that budget for an announced revision not yet visible. ff refuses any dirty state and divergence before a fast-forward merge; mirror/deploy validate first, count unique dirty/untracked paths and discarded local commits, then checkout/reset/clean with fixed argv. All operations on the same checkout share that lock, including legacy calls.
- Review pins: only authenticated `original_to` equal to the canonical configured command channel takes the event path; direct JSON events stay unknown methods. Push always targets the fetched branch tip; its announced commit must be an ancestor, so old push replay cannot roll back. Strict JSON is UTF-8, <=16 KiB/depth 8, rejects duplicates/non-finite/bool version; branches use check-ref-format-compatible rules, revisions canonical lower-case hex. Mirror leaves ignored files/nested repos and refuses in-progress operations; ff treats untracked and operation state as dirty. Effective branch is stored atomically under the checkout lock; a Configuration pin takes precedence. Origins publish project paths only and errors come from a fixed table.
- Concurrency/authority: an own-identity reader dispatches to independent bounded update and legacy/read pools (two workers/eight admitted each); excess work is refused before attempt admission, no duplicate coalescing. No Git work on the reader. Legacy child env contains only authenticated AB_ROLE_* and no AGENT_BUS_TOKEN/other inherited AGENT_BUS_* credentials. Tests assert exact channel realm routing, direct-event refusal, no-role now/deploy refusal, child token absence, old-push no rollback, no hook without lock, and ignored-file counting.
- Results/publication: one exact design record per attempt, persisted atomically as last result and published immediately through the agent's own bus identity; no configured log means a no-op. Fixed safe error text only, full commit ids and project paths only. A configured log refusal/timeout never changes the git result or suppresses scripts, and is never retried. Persist a separate safe local failure note for autopull status; a direct answer appends that one-line note after the unchanged record JSON. Direct deploy answers the record JSON (callers check `.result`), not a fabricated CLI exit convention.
- Frozen step-6 seam: `state_directory()` returns `XDG_STATE_HOME/agent-bus-git` (default `~/.local/state/agent-bus-git`). `after_attempt(repo, checkout, record, publish)` is a no-op extension point, called after the single publication attempt (or no-op) while the checkout lock is STILL held, only when lock and verified checkout are available; `repo` is the validated repository entry, `checkout` a Path, `record` the complete result dict, and `publish(record)` a callable returning True for accepted/no-op and False for delivery failure, recording that failure locally. xAB2 may run at most success then fail, 30 seconds each / 60 seconds total, kill/reap the entire script process group. Its fail script receives the just-published script-failure record. Scripts select host-installed bare names under `state_directory()/autopull.d`; Owner and Maintainers may select them. The hook must not acquire another checkout lock. The core's 60-second git budget ends before the separately bounded hook budget.
- Files/checks: update git_agent.py, autopull.py, install.py and timer-retirement helper; add separate test_autopull.py plus installed private-bus fixture coverage in check.py; preserve xAB2's test_autopull_scripts.py. Extend package/setup/fixture inventories only if new runtime files are needed. Remove timer creation and replace old timer tests with owned-unit retirement/idempotence/foreign-unit refusal and old-option rejection before side effects. Update manifest args/help, end-user git guide, smoke fast allowlist and implementation README.
- Evidence: real local bare-repo and private-daemon fixtures prove every core acceptance line, exact publication counts/metadata, role/channel separation, config refresh/startup, two serialized attempts, and hook ordering/lock/rejected-publication behaviour. Named compiled assertion-red mutants cover parser fields/hex/id, remote/branch routing, cap and role boundaries, dirty/diverged ff, mirror cleanup/counts, revision reachability, startup/follow, publication optionality/counts/redaction, locking and retirement. Review source before final normal under slow; take the next free shared patch with one changelog entry, verify fast-forward ancestry, report the core hash to xAB2 immediately, manager deploys; no push.

Acceptance, each failing on the old code or by a named mutant:

- An `autopull-cmd` push event for a served remote moves an `ff` copy to the commit; `autopull-log` holds exactly one `ok` record with `host`, `from` ≠ `to`.
- The same event on a dirty `ff` copy leaves every file unchanged and records `fail` / `dirty`; on a diverged copy, `diverged`.
- A `mirror` copy with a modified and an untracked file ends equal to the commit; its record has `discarded.files` = 2.
- `mirror` configured on a `read` repository is refused at configuration time.
- `deploy site <other branch>` moves an unpinned copy to that branch's tip and later push events for it move the copy; a pinned copy refuses it unchanged.
- A `deploy` with a revision not reachable from `origin/<branch>` changes nothing and records `not_in_branch`; a reachable one moves the copy to it exactly.
- An event with `v` 2, an unknown field, a 39-hex push commit or another branch moves nothing; the served-repository ones record `invalid_event` or are ignored, as the design says; one naming no served remote writes no record.
- Restarting `#git` with the remote one commit ahead produces one `start` record and moves the copy.
- Two events for one repository at once give two records and one move; neither leaves the lock held.
- No record and no answer contains the checkout path, the remote URL or a commit message (asserted on a fixture whose path, URL and message are unique strings).
- A `deploy` event with `id` gives records carrying that `id` on every matching copy; a direct `deploy` call answers the result record, whose `result` is `fail` on a failed deploy (no bus CLI change); an `id` outside its character set is `invalid_event`.
- After an `ok` attempt the success script runs once in the checkout with the record on stdin, after that record is on the log; exit 0 adds no record; on `skip` no script runs.
- A success script exiting 1 adds exactly one `action: script` record with `result: fail`, `error.code: script`, the attempt's `id` and `to`, and the fail script runs once; a fail script exiting 1 adds one more; a script past its time limit is killed and records `exit: null`.
- A success script that exits 0 but writes one stderr line adds one `action: script` fail record whose `script.stderr` is that line, and the fail script runs; one writing only to stdout adds nothing.
- With no log channel configured, an attempt and a failed script publish nothing and still run.
- A script key holding a path, `..`, a name that is a symlink out of `autopull.d/`, or a group-writable file is refused at configuration time; a file made group-writable after configuration is refused before the run, and nothing runs.
- The owner's flow end to end on two scratch checkouts: one `push` event from a pipeline name gives each `mirror` copy the commit, each runs its success script once, and `autopull-log` holds one `ok` record per host; with a dirty `ff` third copy, that copy is unchanged and its fail script runs.
- A refused or timed-out log publish is attempted once, leaves the git result unchanged and still runs the post-attempt hook; status/direct answer show a safe local failure note.
- After reinstall, no `--autopull` timer unit exists; `--autopull` is refused with a pointer to the new Configuration.

### B.26 `#webhook` publishes to a pubsub channel

Assigned 2026-10-29: AB2; review `#opencode/oab`. A webhook route may name a pubsub record as its target, so a GitLab
or GitHub push reaches `autopull-cmd`. The route maps the hosting payload to the
[new-code event](README.md#new-code-event); the payload stays untrusted.

Acceptance:

- A GitLab push payload posted to a configured route yields one `autopull-cmd` message equal to the expected event, and every Deliver-To receiver gets it.
- A pipeline's plain JSON deploy request posted to its route yields one `deploy` event with its `id`.
- A payload for another branch, or with a bad signature or secret, publishes nothing.
- A route to a pubsub record `#webhook` may not publish to is refused at configuration time.

Code ✅ 0.9.157, commit f5114e4e: route fields `publish`, `event` (`gitlab-push`, `github-push`, `deploy`), `branches`, signature preset `gitlab`; the start check is lookup kind plus `/may-send`. Normal gate 234/0; 8 assertion-red mutants (branch filter, token, start check, id, delivery kind, deletion, event header, deploy extras); real-bus test: two Deliver-To receivers each get the exact event. Review `#opencode/oab` pending.

## Dependencies

- B.25 needs nothing new from the daemon: pubsub records, Deliver-To and ring inboxes exist.
- Setup on this node after B.25: the `autopull-cmd` and `autopull-log` pubsub records; listeners set their inboxes to drop oldest.
- The owner chooses the realty servers' switch from Redis autopull, after B.25 and B.26.
