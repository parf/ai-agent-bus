# Git autopull TODO

📌 **TL;DR:** replace `#git`'s timer autopull with channel-driven autopull and
revision deploy, as [the design](README.md#two-channels) says, then let
`#webhook` publish a hosting service's push to `autopull-cmd`. Two tasks; B.26
can land after B.25, which is testable with `agent-bus send` alone.

## Tasks

### B.25 Channel-driven autopull in `#git`

Unassigned. Code: `src/agents/standalone/git/`. ⚠️ parsers and a destructive
mode: mutants required.

| Step | Work |
|---|---|
| 1 | Per-repository mode `ff`/`mirror` and follow on/off, plus the two channel names, in Configuration; `mirror` refused unless the repository's access is `deploy` or `commit` |
| 2 | Parse the [new-code event](README.md#new-code-event) strictly: `v`, known fields only, hex lengths, branch match, `remote` matched to `origin` |
| 3 | `ff` and `mirror` runs under the repository's lock; `mirror` counts what it discards |
| 4 | Triggers: `autopull-cmd` events, one check per repository at start, `autopull <repo> now`, `deploy <repo> <branch> [<rev>]`; `deploy <repo>` keeps working |
| 5 | Publish one [result record](README.md#result-record) per attempt to the configured log channel, topic `<repo>@<host>`, tag = result; no log name, no publish |
| 6 | `autopull-success-script` / `autopull-fail-script` per repository: Owner-only, absolute path, writable only by root or the agent user, record on stdin after it is published, time limit; a non-zero exit, time-out or any stderr publishes one `action: script` fail record |
| 7 | Remove the install-time `--autopull` timer; reinstall removes an existing timer unit |
| 8 | Update [the git guide](../../../docs/agents/git.md#autopull) and decisions; bump, changelog |

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
- A `deploy` event with `id` gives records carrying that `id` on every matching copy; a direct `deploy` call exits non-zero when the result is `fail`; an `id` outside its character set is `invalid_event`.
- After an `ok` attempt the success script runs once in the checkout with the record on stdin, after that record is on the log; exit 0 adds no record; on `skip` no script runs.
- A success script exiting 1 adds exactly one `action: script` record with `result: fail`, `error.code: script`, the attempt's `id` and `to`, and the fail script runs once; a fail script exiting 1 adds one more; a script past its time limit is killed and records `exit: null`.
- A success script that exits 0 but writes one stderr line adds one `action: script` fail record whose `script.stderr` is that line, and the fail script runs; one writing only to stdout adds nothing.
- With no log channel configured, an attempt and a failed script publish nothing and still run.
- A script path that is relative, group-writable or set by a non-Owner is refused at configuration time and nothing runs.
- The owner's flow end to end on two scratch checkouts: one `push` event from a pipeline name gives each `mirror` copy the commit, each runs its success script once, and `autopull-log` holds one `ok` record per host; with a dirty `ff` third copy, that copy is unchanged and its fail script runs.
- After reinstall, no `--autopull` timer unit exists; `--autopull` is refused with a pointer to the new Configuration.

### B.26 `#webhook` publishes to a pubsub channel

Unassigned. A webhook route may name a pubsub record as its target, so a GitLab
or GitHub push reaches `autopull-cmd`. The route maps the hosting payload to the
[new-code event](README.md#new-code-event); the payload stays untrusted.

Acceptance:

- A GitLab push payload posted to a configured route yields one `autopull-cmd` message equal to the expected event, and every Deliver-To receiver gets it.
- A pipeline's plain JSON deploy request posted to its route yields one `deploy` event with its `id`.
- A payload for another branch, or with a bad signature or secret, publishes nothing.
- A route to a pubsub record `#webhook` may not publish to is refused at configuration time.

## Dependencies

- B.25 needs nothing new from the daemon: pubsub records, Deliver-To and ring inboxes exist.
- Setup on this node after B.25: the `autopull-cmd` and `autopull-log` pubsub records; listeners set their inboxes to drop oldest.
- The owner chooses the realty servers' switch from Redis autopull, after B.25 and B.26.
