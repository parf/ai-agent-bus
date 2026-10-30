# Git autopull: update every server when new code is ready

📌 **TL;DR:**
- **Problem:** shipping code to many servers means logins, deploy scripts that know every host, or timers that guess.
- **Outcome:** when your build finishes green, it announces the new code once, and every server that follows the project updates itself, runs its own after-update script and reports one result line.
- **Start:** check the last automatic update on a server: `agent-bus call '#git@node.example' 'autopull site'` ([See it on a real node](#see-it-on-a-real-node)).

## Why you'd want it

- 🔑 **Your build system needs SSH to every host.** Now it only announces new code; servers pull what they are allowed to.
- 🕒 **Timers guess when to update.** Now an update starts on an event, once at start to catch up, or when an operator asks.
- ➕ **A new server means changing the pipeline.** Now adding a server is a Configuration change on that server.
- 👀 **You cannot see what happened.** Now every attempt is one result record, host first.
- ↩️ **A bad release needs a fast way back.** Now one `deploy` event moves every server to one exact revision.

## What you get

- ✅ **One announcement updates every server** that follows that project and branch ([how it works](#how-it-works)).
- ✅ **Safe by default:** `ff` mode only moves forward and never discards local work; `mirror` (production copies) is chosen per project, and only where the project allows deploy ([two modes](#two-modes)).
- ✅ **After the update,** each server runs its own success or fail script from a host-approved list: reload the application, run tests, post to chat ([after an update](#after-an-update)).
- ✅ **Visibility:** every attempt and every failed script is one JSON record, host first, on the log channel; `autopull <project>` shows the last one on that server.
- ✅ **Control:** the announcement channel's access list says who may announce code; `deploy <project> <branch> [<rev>]` moves one server, a `deploy` event moves all of them to one revision, for roll-out or roll-back.
- 🚫 **Not a CI system,** not a build step, not a scheduler: it reacts to announcements, catches up once at start, and does what an operator asks.

## How it works

| Step | What happens |
|---|---|
| 1 | You push; CI runs; on success it publishes a `push` event to the `autopull-cmd` pubsub channel |
| 2 | Every [`#git`](git.md#what-it-does) whose repository follows that remote and branch receives it |
| 3 | Each updates its copy: `ff` only moves forward, `mirror` makes the copy equal to the remote |
| 4 | Each runs its own success or fail script, for example reload the application and run tests |
| 5 | Each publishes one JSON result, with its host first, to the `autopull-log` pubsub channel, for people, dashboards and waiting pipelines |

- **No timers:** an update starts on an event, once at the agent's start to catch up, or when an operator asks (`autopull site now`, `deploy site main <rev>`).

## See it on a real node

- Ask about the last automatic update (read-only; ran on the development node on 2026-10-30):

```sh
agent-bus call '#git@parf.us' 'autopull ab'
```

```text
no autopull attempt recorded
```

- That node has no announcement channel wired yet, so no attempt has happened; on yours the answer is the last result line, for example `FAIL site@node.example main: diverged (ff)`.

## Two modes

Each project picks one mode in its Configuration (`"mode":"ff"` or
`"mode":"mirror"`); `ff` is the default.

| | `ff` — safe merge | `mirror` — production deploy |
|---|---|---|
| In one line | Move forward only; never lose anyone's work | Make the copy exactly what the branch says |
| On new code | Fast-forwards the branch to the fetched tip | Returns to the branch, resets hard to the fetched tip, removes untracked files (`clean -fd`) |
| Local edits or untracked files | Leaves the copy alone, result `fail` with `dirty` | Discards them, and counts what it discarded in the result |
| Local commits or diverged history | Leaves the copy alone, result `fail` with `diverged` | Discards them, counted the same way |
| Ignored files and nested repositories | Untouched | Untouched |
| Use it for | Development and staging servers, shared copies where someone may be working | Dedicated production copies nobody edits by hand |
| Needs | Project access `deploy` or `commit` | The same, chosen explicitly |

### Which one, and why

| Situation | Mode | Why |
|---|---|---|
| A developer or reviewer may edit files on that server | `ff` | Their work is never thrown away; the update waits, reports `dirty`, and continues once they commit or clean up |
| A staging server that should track `main` but where hot-fixes sometimes happen | `ff` | A hot-fix shows up as `diverged` instead of vanishing silently; someone decides |
| A production copy deployed only through CI | `mirror` | Stray edits, leftover build files or a half-applied hot-fix must never block a release; the server always ends up exactly on the announced code |
| A fleet of identical web servers | `mirror` | Every host converges to the same commit; the result's `discarded` count shows which host had drifted |
| You need one exact revision now, or a roll-back | either mode, with `deploy` | `deploy` moves to the named revision in both modes |

Rule of thumb: if a person might ever type on that server, use `ff`; if only
the pipeline should change it, use `mirror`.

Only projects with `"follow":true` react to channel events and check once at
start. A `push` event always moves to the fetched tip, so replaying an old
announcement never rolls a server back. A `deploy` (event or call) moves to an
exact revision in **either** mode and is destructive in both: it is the
roll-out and roll-back tool.

## Set it up

<details><summary>Configuration, channels and event format</summary>

```json
{"autopull-cmd":"autopull-cmd@node.example", "autopull-log":"autopull-log@node.example",
 "repos":{"site":{"path":"/srv/site","access":"deploy","branch":"main","mode":"ff","follow":true}}}
```

Both channels are ordinary [pubsub channels](../03-records-channel.md#the-two-channel-kinds).
The `autopull-cmd` pubsub channel's ACL controls who may announce updates and rollbacks; its
Deliver-To includes each git agent. The `autopull-log` pubsub channel permits the git agents to
publish; listeners receive copies in their own inboxes. Pubsub retains no
history. For a bounded listener inbox choose [ring overflow](../04-messaging.md#overflow).
Settings apply to the next received message, with no reinstall.

Only a delivery through the exact configured `autopull-cmd` pubsub channel is an event.
Sending the same JSON directly to the agent grants no update authority.
Channel names without a realm use the git agent's realm.

```json
{"v":1,"type":"push","remote":"team/site","branch":"main","commit":"0123456789abcdef0123456789abcdef01234567"}
{"v":1,"type":"deploy","remote":"team/site","branch":"main","commit":"0123456","id":"pipeline-81234"}
```

`remote` is the hosting project path from `origin`, without its host, URL or
`.git` suffix. Local upstreams use the bare directory's basename. The `autopull-cmd` and `autopull-log` pubsub channels must differ. The agent ignores its own forwarded
records to prevent feedback loops. Fields are
strict: version must be integer 1, push commit exactly 40 lower-case hex,
deploy commit 7–40 hex, optional id 1–64 letters, digits, dot, underscore or
hyphen. Unknown fields are refused. Unknown remotes or another branch are ignored.
An invalid event for a served remote produces `invalid_event`; ambiguous or
malformed JSON cannot identify a repository and is ignored.

A push always moves to the **fetched tip**; its commit must be visible and an
ancestor of that tip. Replaying an old push never rolls back a mirror.
Deploy may select any reachable older revision. A commit not visible after
three bounded fetch attempts fails `not_in_branch` without changing the copy.

</details>


## Run or inspect an update

<details><summary>Calls, roles, locking and limits</summary>

```sh
agent-bus call '#git@node.example' 'autopull site'
agent-bus call '#git@node.example' 'autopull site now'
agent-bus call '#git@node.example' 'deploy site main'
agent-bus call '#git@node.example' 'deploy site main 0123456' | jq -e '.result == "ok"'
```

Status is one line, for example `FAIL site@node.example main: diverged (ff)`.
`now` needs the caller's deploy/commit role; the status form is read-only.
Deploy returns one JSON record; the transport CLI exit code does not interpret
its `result`. JSON calls use `method`, `repo`, optional `branch`, `rev`, `id`
for deploy, or `now:true` for an explicit autopull attempt.
A pinned branch cannot be overridden. Otherwise a deploy selects the effective
branch, saved outside the checkout and used after restart or a detached read;
a later Configuration pin takes precedence.

`ff` treats tracked changes, untracked files and an in-progress merge/rebase as
dirty. Mirror/deploy also refuse in-progress operations; resolve those first.
Mirror resets the named local branch and cleans with `-fd`, leaving ignored
files and nested repositories. `discarded.files` counts dirty paths cleared from the snapshot immediately before the write;
external writers that ignore the checkout lock can race that snapshot;
`discarded.commits` counts unique commits reachable from the old HEAD or local
branch tip but not the target. No stash or implicit hold is created.

Updates on one checkout are locked through publication and post-update work;
different checkouts can proceed independently. Git and lock wait share a
60-second budget; checkout hooks run as the server account and are killed with
the Git process group on timeout. Two update workers admit at most eight jobs;
a separate bounded read pool serves other calls. Excess work is refused before
admission; those events and events dropped on invalid Configuration leave no result record; channel publishers must not rely on an inbox as a durable job queue.

</details>


## Results

<details><summary>Result records and delivery failures</summary>

Each admitted attempt saves its last result locally and, when `autopull-log` is
set, makes one publication attempt. Topic is `<repo>@<host>`, tag is `ok`, `skip`
or `fail`. `ok` means an update changed the copy; `skip` means already there.

```json
{"v":1,"host":"node.example","repo":"site","branch":"main","result":"ok",
 "action":"autopull","trigger":"event","mode":"mirror","at":"2026-10-29T12:00:00+00:00",
 "agent":"#git@node.example","remote":"team/site","requested_by":"#ci@node.example","id":"pipeline-81234",
 "from":"0123456789abcdef0123456789abcdef01234567","to":"123456789abcdef0123456789abcdef012345678",
 "requested":null,"discarded":{"files":2,"commits":0},"script":null,"error":null,"ms":1200}
```

`action` is `autopull`, `deploy` or a failed post-attempt `script`; `trigger` is `event`, `start` or `operator`.
`from`, `to` and `requested` are resolved full commit ids, or null when unavailable;
`to` is the actual final HEAD even after a failed write. Discard counts are present for every destructive attempt (deploy in either
mode or mirror autopull), and null for plain ff autopull. Errors have a fixed `code` and safe `text`: `dirty`,
`diverged`, `not_in_branch`, `fetch`, `timeout`, `lock`, `invalid_event` or `script`.
The Git component never adds checkout paths, origin URLs, credentials or commit
messages to these records or update answers. A script-failure record includes
the script's own first stderr line; that text may contain such details, so the
script author must choose safe diagnostic output. Legacy `log` still returns requested commit titles.

Log failure changes neither the Git result nor post-update work. It is never
retried: acceptance may be uncertain. The direct answer remains pure record
JSON; `autopull <repo>` and content-free agent stderr show the last publication
failure. Without an `autopull-log` pubsub channel, no publication is attempted.

</details>


## After an update


A host administrator may install scripts in `autopull.d` under the Git agent's
state directory: `~/.local/state/agent-bus-git/autopull.d`, or
`$XDG_STATE_HOME/agent-bus-git/autopull.d` when that variable is set. Configuration
only chooses an installed bare name per repository:

```json
{"path":"/srv/site","access":"deploy","mode":"mirror","follow":true,
 "autopull-success-script":"reload-site", "autopull-fail-script":"report-failure"}
```

| Setting | Runs after |
|---|---|
| `autopull-success-script` | An `ok` Git result |
| `autopull-fail-script` | A `fail` Git result, or a failed success script |

Neither runs on `skip`, or when no safe checkout lock/root was obtained. Owners
and Maintainers can choose installed names through Configuration; they cannot
supply a command line or a path. Names use 1–128 ASCII letters/digits, dots,
underscores or hyphens, start with a letter/digit and contain no `..`.

<details><summary>Installing scripts and understanding outcomes</summary>

- The script file, its directory and all parents must be owned by root or the
  Git account and not group/other-writable. Executables must not be setuid/setgid.
  Use 0700 for an agent-owned script or 0755 for a root-owned one. Trusted aliases
  are allowed only when their script target stays inside `autopull.d`. Interpreter
  aliases and all their parent directories are checked too.
- Scripts must be executable ELF programs or have an absolute shebang interpreter.
  Strict Configuration validation checks the installed file. Every run rechecks
  the open file and chosen name; a removed or newly unsafe script is refused with
  one script-failure record. Such file drift does not stop the Git update or other
  repositories from working.
- The attempt record is JSON on stdin (at most 16 KiB), after its one log
  publication attempt. Scripts still run when the log is unset, refused or its
  acceptance is uncertain; nothing retries that publication. The checkout lock
  stays held through scripts, so a later update cannot change the revision under
  a running script. Other repositories can update independently.
- Each script has 30 seconds; success plus fail share 60 seconds, separate from
  the Git/lock budget. Non-zero exit, signal death, timeout or **any stderr byte**
  counts as failure, including a newline with exit 0. Stdout is discarded and
  cannot fill a pipe. Stderr is drained; only its first line, up to 200 characters
  with ANSI/control characters removed, is retained. An empty first line remains
  empty but still means failure.
- Each failed script adds exactly one `action: script`, `result: fail` record,
  preserving `id`, `from` and `to`. `error.code` is `script`; its `script` field
  has `run` (`success`/`fail`), `exit` (null on timeout), `stderr` (or null) and
  `ms`. A failed success script passes that failure record to the fail script
  after its single publication attempt. A failing fail script adds one record
  and stops; it never calls itself. The Git result and script results remain
  separate records.
- Scripts run as the Git account with the checkout as cwd, fixed umask 0077 and
  PATH `/usr/bin:/bin`. Bus credentials, caller roles, Git settings and SSH-agent
  variables are not inherited. Resource limits are inherited from the service;
  the helper adds no extra rlimits.
- A trusted script is executed through a held file descriptor. Shebang scripts
  may see `$0` as `/proc/self/fd/N`; use the checkout cwd or absolute paths rather
  than `dirname "$0"`. The script fd remains open/inheritable. The kernel resolves
  the trusted interpreter at exec; a privileged interpreter swap remains outside
  the script-file fd guarantee.
- Remaining process-group children are killed on timeout **and normal exit**.
  Do not start background work expecting it to survive. A child deliberately
  changing session (or privileged descendants the account cannot kill) can escape
  this group; these are admin-installed programs, not an untrusted-code sandbox.

</details>

