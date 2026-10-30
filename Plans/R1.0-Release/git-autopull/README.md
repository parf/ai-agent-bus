# Git autopull from bus channels

📌 **TL;DR:** a universal component: `#git` keeps a checkout up to date when
anyone allowed announces new code on the `autopull-cmd` pubsub channel, in one
of two modes per repository: `ff` (safe, never discards work) or `mirror`
(production deploy). A server may run its own success or fail script after an
attempt. Every attempt is one JSON record on `autopull-log`.
No timers. The owner's realty flow is one [use case](#use-cases) that must work.

Owner, 2026-10-29. Decisions: [RD147, RD148, RD150, RD151](../DECISIONS.md#recorded-decisions).
Tasks: [TODO](TODO.md#tasks).

## Two channels

| Channel | Who publishes | Who receives (Deliver-To) | Carries |
|---|---|---|---|
| `autopull-cmd` | whoever its ACL allows: a pipeline, `#webhook`, an operator, another agent | every `#git` that serves a matching repository | a [push or deploy event](#new-code-event) |
| `autopull-log` | every `#git` | people, dashboards, pipelines waiting for their `id`, and optionally services that act on `ok` | one [result record](#result-record) per attempt |

Both are ordinary pubsub records: their ACL says who may publish, their
Deliver-To says who gets a copy of everything. Work after a deploy is usually
the [server's own script](#after-an-attempt); a service that acts on
`autopull-log` records still works, as an option (owner: "the old scheme with
autopull-log works; just optional"). Pubsub stays a stateless fan-out with no
filters and no history: each receiver filters what it gets by message `topic`
and `tag` (owner, 2026-10-29).

## Modes

| Mode | Does | For |
|---|---|---|
| `ff` (default) | fetch, fast-forward only; a dirty or diverged copy is left alone and reported `fail` | developer machines, shared copies |
| `mirror` | fetch, return to the configured branch, `reset --hard` to the target, `clean -fd`; reports what it discarded | dedicated production copies |

`mirror` is chosen explicitly per repository and only where its access is at
least `deploy`. There is no release-tag filter: what is pushed is deployed
(owner: "let them think when they push").

## What starts an attempt

| Trigger | When |
|---|---|
| `event` | a new-code event names this repository's remote and branch |
| `start` | once per repository when `#git` starts, as the real autopull's init event, to catch up on anything missed while it was down |
| `operator` | `autopull <repo> now` (the configured mode), or [`deploy`](#deploy) |

No timer and no polling. Attempts on one repository run one at a time under its
existing lock; an event for a commit that is not yet visible waits briefly for
the fetch to see it, then fails with `not_in_branch`.

## Deploy

The usual call, `deploy <repo> <branch> [<rev>]` (owner: "usual #git deploy
repo branch"):

| Form | Does |
|---|---|
| `deploy site main` | fetch, check out `main`, `reset --hard origin/main`, `clean -fd` |
| `deploy site main 1a2b3c4` | the same, to that revision: 7–40 hex, present after fetch and reachable from `origin/main` |
| `deploy site` | the repository's branch, as today |

A repository whose Configuration pins `branch` refuses any other branch. Without
a pin, the deployed branch becomes the copy's branch, and push events follow it.
A `deploy` event on `autopull-cmd` does the same on every matching server at
once (the replacement for realty's `request-revision-pull`). A later push event
moves a `mirror` copy on again; there is no hold. Every deploy writes a
[result record](#result-record) with `action: deploy`.

## Use cases

`#git` does not care who publishes or why; each case is a choice of publisher,
mode and scripts. All must work:

| Case | Publisher | Mode | After |
|---|---|---|---|
| **Owner's flow (realty today)**: push → GitLab CI → on success CI publishes `push` → every server on the branch pulls | the pipeline, directly or through `#webhook` | `mirror` on production, `ff` on a development server | each server's success script (reload, tests); fail script alerts |
| Every push, no CI | a hosting webhook through `#webhook` ([B.26](TODO.md#b26-webhook-publishes-to-a-pubsub-channel)) | `ff` | none, or a script |
| A developer copy that follows its branch | as above | `ff`: never discards local work, reports `dirty` | none |
| Roll out or roll back one revision everywhere | a pipeline or operator publishes `deploy` with a commit | per repository | scripts as usual |
| One known server, wait for the answer | none: `agent-bus call '#git@host' 'deploy site main <sha>'` | — | the answer is the result; the exit code says ok or fail |
| A pipeline that waits for every server | publishes with an `id`, then reads records with that `id` from its own queue in the `autopull-log` Deliver-To until each expected host answered or it times out | per repository | — |
| React elsewhere on success | any service in the `autopull-log` Deliver-To | — | that service acts on `ok` (optional) |

A pipeline or other publisher is an ordinary bus caller with its own name (for
example `#ci/realty@parf.us`) on the channel's ACL. `#git` does not know which
servers should answer; a caller that waits names its expected hosts.

## New-code event

Message body, JSON:

```json
{"v":1, "type":"push", "remote":"realmo/realty", "branch":"main", "commit":"<40 hex>"}
{"v":1, "type":"deploy", "remote":"realmo/realty", "branch":"main", "commit":"<7–40 hex>", "id":"pipeline-81234"}
```

| Field | Meaning |
|---|---|
| `v` | 1; any other version is refused |
| `type` | `push`: bring the branch to its tip by the repository's mode; `deploy`: move to exactly `commit` |
| `remote` | the project path on the hosting service; `#git` matches it against each repository's `origin`, not its local name |
| `branch` | must equal the copy's branch (pinned, else checked out), otherwise the event is ignored |
| `commit` | 40 hex for `push`; 7–40 hex for `deploy` |
| `id` | optional, 1–64 of `A-Za-z0-9._-`; copied into every result record so a pipeline finds its own |

Unknown fields are refused. An event that names no served repository is ignored
without a record; one that names a served repository but is invalid gets a
`fail` record.

## Result record

One message per attempt on `autopull-log`. Message `topic` is
`<repo>@<host>`, `tag` is the `result`. Body, JSON:

```json
{"v":1, "host":"realmo-parf.ny.7w7.us", "repo":"site", "branch":"main",
 "result":"ok", "action":"autopull", "trigger":"event", "mode":"mirror",
 "at":"2026-10-30T19:23:42-04:00", "agent":"#git@parf.us",
 "remote":"realmo/realty", "requested_by":"#ci/realty@parf.us", "id":"pipeline-81234",
 "from":"<40 hex>", "to":"<40 hex>", "requested":"<40 hex>",
 "discarded":{"files":2,"commits":0}, "script":{"run":"success","exit":0,"ms":5210},
 "error":null, "ms":7050}
```

| Field | Meaning |
|---|---|
| `host` | the machine's full name (`hostname -f`); first, so a server is easy to find |
| `repo`, `branch`, `remote` | local repository name, its branch, its hosting path |
| `result` | `ok` moved, `skip` already there, `fail` |
| `action` | `autopull` (branch tip) or `deploy` (a requested revision) |
| `trigger` | `event`, `start` or `operator` |
| `mode` | `ff` or `mirror` |
| `at`, `ms` | finish time (RFC 3339 with offset) and duration |
| `agent`, `requested_by` | the `#git` name, and the bus sender of the event or call (`null` for `start`) |
| `id` | the event's or call's `id`, else `null` |
| `from`, `to`, `requested` | full commit ids; `to` equals `from` when nothing moved; `requested` is the event's or call's commit, else `null` |
| `script` | `null` when none ran; else `run` (`success` or `fail`), `exit` (`null` on time-out) and `ms`; with both, the fail script's |
| `discarded` | `mirror` only: local files and commits removed; otherwise `null` |
| `error` | `null`, or `{"code":…, "text":…}` with code one of `dirty`, `diverged`, `not_in_branch`, `fetch`, `timeout`, `lock`, `invalid_event`, `script` |

Never in a record: local paths, remote URLs or keys, commit messages.

A one-line human form, used by `autopull <repo>` status:
`FAIL site@realmo-parf.ny.7w7.us main: diverged (ff)`.

## Configuration

Per repository, in `#git`'s Configuration as usual (owner: "Configuration —
yes, as usual"): its mode and whether it follows new-code events. The two
channel names are agent-wide Configuration, defaulting to `autopull-cmd` and
`autopull-log` (owner: "autopull-* names"). Changing them needs no reinstall.
The install-time `--autopull` timer option is removed.

## After an attempt

Each repository's Configuration may name two scripts, run on that server after
the attempt (owner: "autopull-success-script & autopull-fail-script"):

| Key | Runs when |
|---|---|
| `autopull-success-script` | `result` is `ok`: reload php-fpm, run tests, warm caches |
| `autopull-fail-script` | `result` is `fail`, including a failed success script: alert, roll back |

Neither runs on `skip`. A script gets the result record on stdin, runs in the
checkout as `#git`'s own user, with a time limit, and its exit code and duration
go into the record's [`script`](#result-record) field; the record is published
after it ends. A failed success script turns the result into `fail` with code
`script`, then the fail script runs. ⚠️ A script is code run on the server, so
only the record's Owner may set it, the path is absolute, and a file writable by
anyone but root or `#git`'s user is refused.

## Log size

Nothing new: pubsub keeps no state, and each copy lives in its receiver's inbox.
A receiver that wants a bounded log sets its own inbox to **When full: drop
oldest** (`--overflow ring`, [overflow](../../../docs/04-messaging.md#overflow))
with a bound of its choice. A receiver added later or restarted gets only new
records, no history (owner, 2026-10-29).

Chat posts are the scripts' business, not `#git`'s: in realty the success script
runs the tests, and the tests post to Slack (owner, 2026-10-29). A script has
the checkout, so it can add the author and subject a chat line wants.
