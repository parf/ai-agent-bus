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

- `deploy` fetches, returns to the configured branch, resets hard to its upstream and runs `clean -fd`, then answers with the old and new commit ids.
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
- Add `--autopull` for [automatic updates](#autopull).
- Remove: same installer, `remove --name '#git@node.example'`, with current credentials; its timers go too.

</details>

## Configuration

The list of projects, each with its folder, ceiling and branch, and optionally
which SSH key to use. Changes apply on the next call.

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `repos` | Name → `path` (absolute), `access` (`read`, `deploy`, `commit`), optional `branch` |
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
- The branch's upstream (the remote branch it follows) must be configured for pull and deploy.

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

Optional automatic updates every few minutes, chosen at install time (not a
Configuration key). It never throws away work: a copy with local edits is left
alone and reported. Never enable it in a shared working copy.

<details><summary>Technical details: autopull</summary>

- Reinstall with `--autopull site=5m` added; without it no timer exists. Reinstall without it to remove the timer; an old result can remain.
- Each attempt uses the current settings and keys, refuses local edits and only moves forward. It never resets, cleans, stashes, switches branches or restarts an application.

</details>

## Warnings and alerts

`#git` has its own Linux service, so the [runner entries](README.md#warnings-and-alerts) do not apply; it reports these itself. The page that shows them is in work.

| Level | When | What to do |
|---|---|---|
| alert | Configuration `repos` is missing or malformed, or `ssh_key` is not a readable file: every call is refused | Fix Configuration |
| alert | A declared `path` is not a repository root | Fix `path`, or clone the copy there |
| alert | Autopull fails: SSH sign-in refused, remote unreachable, upstream missing, or settings unreadable | Check the account's key, known hosts and the branch's upstream |
| alert | A success or fail script run after an update failed, timed out or wrote to its error output | Read the script's line in the result; fix the script or what it checks |
| warning | Autopull left a copy alone: local edits, or histories that differ | Resolve the local work; autopull never discards it |

## Troubleshooting

| If you see | Do this |
|---|---|
| Repository is not declared | Add its name and absolute path to `repos` |
| Permission refused | Check both the caller's role and the project's `access` |
| Dirty or non-fast-forward refusal | Resolve the local work; automatic updates never discard it |
| SSH authentication failed | Check the account and key; a configured key never falls back to others |
| Missing branch or upstream | Set the branch and its upstream on the server |
| Commit identity unknown | Configure the account's Git name and email |
