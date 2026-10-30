# Git autopull from bus channels

📌 **TL;DR:** `#git` keeps a server's checkout up to date when new code is
announced on the `autopull-cmd` pubsub channel, in one of two modes per
repository: `ff` (safe, never discards work) or `mirror` (production deploy).
Every attempt is one JSON record on `autopull-log`, and goes to filtered
subscribers as `#imap` and `#logwatch` forward theirs, so other services can act
on a successful deploy. No timers.

Owner, 2026-10-29. Decisions: [RD147, RD148, RD149](../DECISIONS.md#recorded-decisions).
Tasks: [TODO](TODO.md#tasks).

## Two channels

| Channel | Who publishes | Who receives (Deliver-To) | Carries |
|---|---|---|---|
| `autopull-cmd` | a hosting webhook through `#webhook`, CI, an operator | every `#git` that serves a matching repository | a [push or deploy event](#new-code-event) |
| `autopull-log` | every `#git` | services that act on a deploy: reload, tests, chat relay, an AI | one [result record](#result-record) per attempt |

Both are ordinary pubsub records: their ACL says who may publish, their
Deliver-To says who gets a copy of everything. The bus has no per-subscriber
filter, so for a filtered feed `#git` keeps [subscriptions](#subscriptions) of
its own (owner: "autopull is like our subscriptions").

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

## New-code event

Message body, JSON:

```json
{"v":1, "type":"push", "remote":"realmo/realty", "branch":"main", "commit":"<40 hex>"}
```

| Field | Meaning |
|---|---|
| `v` | 1; any other version is refused |
| `type` | `push`: bring the branch to its tip by the repository's mode; `deploy`: move to exactly `commit` |
| `remote` | the project path on the hosting service; `#git` matches it against each repository's `origin`, not its local name |
| `branch` | must equal the copy's branch (pinned, else checked out), otherwise the event is ignored |
| `commit` | 40 hex for `push`; 7–40 hex for `deploy` |

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
 "remote":"realmo/realty", "requested_by":"#webhook@parf.us",
 "from":"<40 hex>", "to":"<40 hex>", "requested":"<40 hex>",
 "discarded":{"files":2,"commits":0}, "error":null, "ms":1840}
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
| `from`, `to`, `requested` | full commit ids; `to` equals `from` when nothing moved; `requested` is the event's or call's commit, else `null` |
| `discarded` | `mirror` only: local files and commits removed; otherwise `null` |
| `error` | `null`, or `{"code":…, "text":…}` with code one of `dirty`, `diverged`, `not_in_branch`, `fetch`, `timeout`, `lock`, `invalid_event` |

Never in a record: local paths, remote URLs or keys, commit messages.

A one-line human form, used by `autopull <repo>` status and chat relays:
`FAIL site@realmo-parf.ny.7w7.us main: diverged (ff)`.

## Subscriptions

The same `sub` commands as [`#imap`](../../../docs/agents/imap.md#what-it-does)
and [`#logwatch`](../../../docs/agents/logwatch.md#what-it-does):

| Command | Does |
|---|---|
| `sub add <target> [/regex/]` | Sends each future result record to a queue or agent; the filter matches the one-line form, so `/^OK site@realmo-/` picks results by result, repo or host; no filter means all |
| `sub list`, `sub delete <id>` | Shows or stops them |

The target gets the JSON record, as on `autopull-log`. Adding one needs
permission to send to the target, as for `#imap`. Subscriptions start from now:
no backlog is replayed.

## Configuration

Per repository, in `#git`'s Configuration as usual (owner: "Configuration —
yes, as usual"): its mode and whether it follows new-code events. The two
channel names are agent-wide Configuration, defaulting to `autopull-cmd` and
`autopull-log` (owner: "autopull-* names"). Changing them needs no reinstall.
The install-time `--autopull` timer option is removed.

## After a deploy

`#git` runs no hooks and restarts nothing. Services that must act on a
successful deploy (reload php-fpm, run tests, post to chat) receive
`autopull-log`, or subscribe with a filter, and act on `result: ok`.
A chat relay for these records is an [open question](../QUESTIONS.md#git-autopull).
