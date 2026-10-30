# Git: see and update code on a server

📌 **TL;DR:** Git keeps the history of a project's code. This helper lets you
or your AI see what changed in a project on a server and bring an application
up to date, without a server login. An administrator decides which projects it
may touch and how far anyone may go in each.

## Why you'd want it

**The problem.** Updating an application usually means someone logs in,
pulls the new code and hopes nobody's unsaved work is in the way. Giving
everyone that login is risky.

**The idea.** One helper on the server works on a short list of approved
projects. Each project has a ceiling, such as "look only" or "may update", and
nobody, not even the owner, can go above it. It uses the server's own Git
account and keys, never the caller's.

**When to use it**

- Check what is deployed, or what changed, before or after a release.
- Let a team or an AI update its application without full server access.
- Run automatic updates of a dedicated application copy.

**When not to:** day-to-day coding in a shared working copy, merging or
resolving conflicts, or anything needing arbitrary Git commands.

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| Release review | Ask for the last 20 changes | Short change ids and titles |
| Pre-deploy check | Ask for status | Whether the copy is clean and up to date |
| Pull and restart after a merge | Pull, then restart with [systemd](systemd.md) | The application runs the new code |
| Stuck application copy | Deploy (force to the remote version) | Old and new versions; local edits discarded |
| Approved docs edit | Add, commit, push | The change saved under the server's identity |
| Overnight update check | Ask about autopull | Time and result of the last automatic update |

## What it does

| Ability | Calls |
|---|---|
| Look | `list`, `status`, `log`, `rev`, `fetch`, `diff`, `autopull` |
| Update | `pull`, `deploy`, `reset`, `clean` |
| Save | `add`, `commit`, `push` |

Every call names a project declared in [Configuration](#repositories-and-access-levels).
`pull` only moves forward and refuses when histories differ. **`deploy`,
`reset` and `clean` throw away local edits**: use them only on a dedicated
application copy.

<details><summary>Technical details: what each update does</summary>

- `deploy` fetches, returns to the configured branch, resets hard to origin branch and runs `clean -fd`, then answers with a JSON result record.
- `diff` returns a summary and untracked paths, not a full patch or a date-based report. `log ab 20` has no date filter.
- Commits use the server account's Git name and email; `push` uses its remote and keys. It never pretends to be the person who asked.
- Options and shell commands are never accepted. Restarting the application is a separate [systemd](systemd.md) call.

</details>

## You'll need

- An existing Git copy of the project on the server, readable (writable for updates) by the chosen Linux account.
- Git name, email and remote keys working for that account.
- An administrator to install it.

## Try this first

```sh
agent-bus help '#git@node.example'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

## Use examples

| I want to… | Command |
|---|---|
| See declared projects | `agent-bus call '#git@node.example' list` |
| Review recent changes | `agent-bus call '#git@node.example' 'log ab 20'` |
| Check before a deploy | `agent-bus call '#git@node.example' 'status site'` |
| Pull after a merge | `agent-bus call '#git@node.example' 'pull site'` |
| Force a stuck copy to the remote | `agent-bus call '#git@node.example' 'deploy site'` |
| Save an approved edit | `agent-bus call '#git@node.example' 'commit docs "Explain backups"'` |
| Check the automatic update | `agent-bus call '#git@node.example' 'autopull site'` |

<details><summary>Rules and more calls</summary>

- Save needs `add docs README.md` first and `push docs` after; a project capped at `commit`.
- See [Calls](#calls) for `fetch`, `rev`, a detached revision and one-file restore.

</details>

## Installing it

An administrator installs it once as a Linux service under the account that
owns the project, then declares projects in Configuration.

<details><summary>Step by step</summary>

With [installation credentials](README.md#standalone-helpers):

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  python3 src/agents/standalone/git/install.py install \
  --name '#git@node.example' --user deploy --allow '@owner'
agent-bus config '#git@node.example' '{"repos":{"ab":{"path":"/srv/ab","access":"read","branch":"main"}}}'
agent-bus call '#git@node.example' list
unset AGENT_BUS_TOKEN
```

- First check that the `--user` account can read the project's history there.
- The installer prints `agent-bus-git.service`; `list` should show `ab`. Calls are refused until Configuration declares projects; the installer takes no `--repo`.
- Choose event-driven [automatic updates](#autopull) in Configuration. The removed `--autopull` timer flag is refused; reinstall retires owned old timer units.
- Remove: same installer, `remove --name '#git@node.example'`, with current credentials; owned old timers go too.

</details>

## Configuration

The list of projects, each with its folder, ceiling and branch, and optionally
which SSH key to use. Changes apply on the next call.

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `repos` | Name → `path` (absolute), `access` (`read`, `deploy`, `commit`), optional `branch`, `mode` (`ff` default or `mirror`), `follow` (boolean, false default) |
| `autopull-cmd` | Optional pubsub channel whose authenticated deliveries announce updates; required by `follow` |
| `autopull-log` | Optional pubsub channel for result records; no name means no publication |
| `ssh_key` | Optional absolute path to a private-key file the account already holds; never key contents |

```json
{"repos": {
  "ab":   {"path": "/srv/ab",   "access": "read",   "branch": "main"},
  "site": {"path": "/srv/site", "access": "deploy", "branch": "main"},
  "docs": {"path": "/srv/docs", "access": "commit", "branch": "main"}
}, "ssh_key": "/home/deploy/.ssh/site-deploy"}
```

A missing or malformed list is an error, never broad access; undeclared names are refused.

</details>

### Repositories and access levels

Each project's ceiling: `read` (look only), `deploy` (also update), `commit`
(also save and push). A caller's permission and the ceiling both apply; the
lower wins.

<details><summary>Technical details: branches</summary>

- `branch` sets the branch for deploy, reset and ahead/behind counts. Without it the current branch is used; a detached copy needs a usable remote default branch or an explicit `branch`.
- Pull uses the branch upstream; event updates and deploy use `origin/<branch>`.

</details>

## Secret keys

None. With `ssh_key` set only that key is used; otherwise the account's normal
keys and SSH settings apply. Known-hosts checking always applies.

## Who may use it

Permissions come from the bus, never from the call's text. Owner and
Maintainers have every permission; the project ceiling still binds everyone.

| Permission | Allows (within the ceiling) |
|---|---|
| On the access list | Look calls |
| `deploy` | Also update calls |
| `commit` | Also save calls; includes deploy |

Example: `agent-bus manage '#git@node.example' --add-allow '#claude/deploy@node.example(deploy)'`.

### Calls

<details><summary>More calls</summary>

`fetch ab`, `rev ab`, `pull ab v1.2` for a detached revision, `pull ab main README.md` to restore one file's working copy. `reset ab` and `clean ab` are destructive. Paths must stay inside the project. `--help` lists every call and its JSON form.

</details>

## Autopull

Follow a command channel to catch new code without a timer. `ff` refuses local
changes or diverged history; `mirror` discards local changes in a dedicated
application copy. Only repositories with `follow:true` handle events and check
once at startup. Both following and mirror mode require access `deploy` or `commit`.

<details><summary>Configure a following copy</summary>

```json
{"autopull-cmd":"autopull-cmd@node.example", "autopull-log":"autopull-log@node.example",
 "repos":{"site":{"path":"/srv/site","access":"deploy","branch":"main","mode":"ff","follow":true}}}
```

Both channels are ordinary [pubsub channels](../03-records-channel.md#the-two-channel-kinds).
The command channel's ACL controls who may announce updates and rollbacks; its
Deliver-To includes each git agent. The log channel permits the git agents to
publish; listeners receive copies in their own inboxes. Pubsub retains no
history. For a bounded listener inbox choose [ring overflow](../04-messaging.md#overflow).
Settings apply to the next received message, with no reinstall.

Only a delivery through the exact configured command channel is an event.
Sending the same JSON directly to the agent grants no update authority.
Channel names without a realm use the git agent's realm.

```json
{"v":1,"type":"push","remote":"team/site","branch":"main","commit":"0123456789abcdef0123456789abcdef01234567"}
{"v":1,"type":"deploy","remote":"team/site","branch":"main","commit":"0123456","id":"pipeline-81234"}
```

`remote` is the hosting project path from `origin`, without its host, URL or
`.git` suffix. Local upstreams use the bare directory's basename. The command and log channels must differ. The agent ignores its own forwarded
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

<details><summary>Run or inspect an update</summary>

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
failure. Without a log channel, no publication is attempted.

</details>

### After an attempt

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

## Warnings and alerts

`#git` has its own Linux service, so the [runner entries](README.md#warnings-and-alerts)
do not apply. Update failures are reported in result records and autopull
status; log-delivery and keeper failures also use content-free service stderr.
These are operational reports, not a separate formal agent-alert emitter.

| Report | What to do |
|---|---|
| Invalid Configuration or an unreadable SSH key | Fix Configuration and the service account's access |
| A declared path is not a repository root | Fix the path or clone the copy there |
| Update failed or the requested revision is not reachable | Check origin, SSH access and the branch/revision |
| ff refused dirty or diverged history | Resolve the local work; deploy is destructive even on an ff copy |
| A post-attempt script failed | Inspect its exit/stderr result; a timeout has null exit, and failure does not roll back the Git update |
| Log publish failed or its acceptance is unknown | Inspect autopull status; no automatic resend is made |

## Troubleshooting

| If you see | Do this |
|---|---|
| Repository is not declared | Add its name and absolute path to `repos` |
| Permission refused | Check both the caller's role and the project's `access` |
| Dirty or non-fast-forward refusal | Resolve the local work; ff autopull never discards it, while mirror and every deploy are destructive |
| SSH authentication failed | Check the account and key; a configured key never falls back to others |
| Missing branch or upstream | Set the branch; deploy and autopull require origin to serve it |
| Commit identity unknown | Configure the account's Git name and email |
