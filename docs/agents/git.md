# Git: see and update code on a server

📌 **TL;DR:**
- **Problem:** updating an application means someone logs in to the server, pulls the code and hopes no unsaved work is in the way; sharing that login is risky.
- **Outcome:** you or your AI see what is deployed and bring it up to date without a server login, within a ceiling the administrator sets per project.
- **Start:** ask what the helper can do: `agent-bus help '#git@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **Everyone needs a login to update code.** Now one helper does it, using the server's own Git account and keys, never the caller's.
- 🧱 **An update can wipe someone's work.** Now each project has a ceiling, "look only" or "may update", and nobody, not even the owner, goes above it.
- 🔍 **You cannot tell what is deployed.** Now you ask for the status or the last changes.
- 🚀 **Ten servers, ten logins.** Now one announcement from your build system updates every server that follows the project.
- 🤖 **An AI should deploy, not roam.** Now it gets "may update" on one project and nothing else.

## What it does

- ✅ **Look:** see what is deployed, recent changes and differences (`list`, `status`, `log`, `rev`, `fetch`, `diff`).
- ✅ **Update:** bring a project up to date, only forward, or force it to the remote version (`pull`, `deploy`, `reset`, `clean`).
- ✅ **Save:** keep an approved edit, under the server's Git identity (`add`, `commit`, `push`).
- ✅ **Update every server by itself:** [autopull](#autopull) follows your build system's announcement and reports one result line per server.
- ⚠️ **Forced updates throw away local edits:** use `deploy`, `reset` and `clean` only on a dedicated application copy.
- 🚫 **Not for:** day-to-day coding in a shared working copy, merging or resolving conflicts, or arbitrary Git commands.
- ↩️ **Roll out or back one revision:** publish a deploy event with a commit; every following server moves to exactly that commit.

Every call names a project declared in [Configuration](#configuration).

## You'll need

- A Git copy of the project on the server, readable (writable for updates) by the chosen Linux account.
- Git name, email and remote keys working for that account.
- An administrator to install it.

## Try this first

- Ask the helper what it can do:

```sh
agent-bus help '#git@node.example'
```

- More on this: [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

<details>
<summary>See it on a real node (commands that ran today)</summary>

List the projects the helper may touch:

```sh
agent-bus call '#git@parf.us' list
```

```text
ab
```

Check whether the copy is clean and up to date:

```sh
agent-bus call '#git@parf.us' 'status ab'
```

```text
## main...origin/main [ahead 175]
```

Review the last three changes:

```sh
agent-bus call '#git@parf.us' 'log ab 3'
```

```text
553294b4 Point source comments at archived completed plan files
2b677166 Archive completed plans and remove the empty open review shell
4915a3d2 Changelog squashes the unpushed 0.9.163–0.9.182 into one published changeset
```

Ask about the last automatic update:

```sh
agent-bus call '#git@parf.us' 'autopull ab'
```

```text
no autopull attempt recorded
```

The output is from the development node on 2026-10-30; project names and changes differ on yours.

</details>

## Use examples

- **See declared projects:** `agent-bus call '#git@node.example' list`
- **Review recent changes:** `agent-bus call '#git@node.example' 'log ab 20'`
- **Check before a deploy:** `agent-bus call '#git@node.example' 'status site'`
- **Pull after a merge:** `agent-bus call '#git@node.example' 'pull site'`
- **Force a stuck copy to the remote version:** `agent-bus call '#git@node.example' 'deploy site'`
- **Save an approved edit:** `agent-bus call '#git@node.example' 'commit docs "Explain backups"'`
- **Check the automatic update:** `agent-bus call '#git@node.example' 'autopull site'`
- **Restart the application after a pull:** a separate call to [systemd](systemd.md).

<details>
<summary>Rules and more calls</summary>

- Save needs `add docs README.md` first and `push docs` after; the project must allow `commit`.
- `fetch ab`, `rev ab`, `pull ab v1.2` for a detached revision, `pull ab main README.md` to restore one file's working copy.
- `reset ab` and `clean ab` are destructive.
- Paths stay inside the project.

</details>

## Installing it

- An administrator installs it once as a Linux service under the account that owns the project.
- Then the administrator declares the projects in [Configuration](#configuration).

<details>
<summary>Step by step</summary>

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
- The installer prints `agent-bus-git.service`; `list` should show `ab`.
- Calls are refused until Configuration declares projects; the installer takes no `--repo`.
- Choose event-driven [automatic updates](#autopull) in Configuration; the removed `--autopull` timer flag is refused and reinstall retires owned old timer units.
- Remove: the same installer, `remove --name '#git@node.example'`, with current credentials; owned old timers go too.

</details>

## Configuration

- **What:** the list of projects, each with its folder, ceiling and branch, and optionally the SSH key to use.
- **When it applies:** on the next call.
- **Failure mode:** a missing or malformed list is an error, never broad access; undeclared names are refused.

<details>
<summary>Keys and a full example</summary>

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

</details>

### Repositories and access levels

- **`read`:** look only.
- **`deploy`:** also update.
- **`commit`:** also save and push.
- **Both apply:** a caller's permission and the ceiling; the lower wins.

<details>
<summary>Technical details: branches</summary>

- `branch` sets the branch for deploy, reset and ahead/behind counts.
- Without it the current branch is used; a detached copy needs a usable remote default branch or an explicit `branch`.
- Pull uses the branch upstream; event updates and deploy use `origin/<branch>`.

</details>

## Secret keys

- **None.**
- With `ssh_key` set only that key is used; otherwise the account's normal keys and SSH settings apply.
- Known-hosts checking always applies.

## Who may use it

- **Permissions come from the bus,** never from the call's text.
- **Owner and Maintainers** have every permission; the project ceiling still binds everyone.

| Permission | Allows (within the ceiling) |
|---|---|
| On the access list | Look calls |
| `deploy` | Also update calls |
| `commit` | Also save calls; includes deploy |

- Example: `agent-bus manage '#git@node.example' --add-allow '#claude/deploy@node.example(deploy)'`.

### Calls

- Every call is a project name plus a few words; `--help` lists them all.
- Options and shell commands are never accepted.

## Autopull

- **What:** when your build system announces new code on a channel, every server that follows the project updates itself.
- **Then:** each server runs its own success or fail script and reports a result to a log channel.
- **Details:** [Git autopull](git-autopull.md) covers the summary, setup, events, results and scripts.

## Troubleshooting

| If you see | Do this |
|---|---|
| Repository is not declared | Add its name and absolute path to `repos` |
| Permission refused | Check both the caller's role and the project's `access` |
| Dirty or non-fast-forward refusal | Resolve the local work; ff autopull never discards it, while mirror and every deploy are destructive |
| SSH authentication failed | Check the account and key; a configured key never falls back to others |
| Missing branch or upstream | Set the branch; deploy and autopull require origin to serve it |
| Commit identity unknown | Configure the account's Git name and email |

## Warnings and alerts

`#git` has its own Linux service, so the [runner entries](README.md#warnings-and-alerts) do not apply; it reports these itself. Update results also go to the log channel and autopull status. The page that shows them is in work.

| Level | When | What to do |
|---|---|---|
| alert | Invalid Configuration or an unreadable SSH key | Fix Configuration and the service account's access |
| alert | A declared path is not a repository root | Fix the path or clone the copy there |
| alert | An update failed, or the requested revision is not reachable | Check origin, SSH access and the branch or revision |
| alert | A success or fail script failed, timed out or wrote to its error output | Read its first stderr line in the result; a failed script does not roll back the update |
| warning | `ff` left a dirty or diverged copy alone | Resolve the local work; deploy is destructive even on an `ff` copy |
| warning | Publishing to the log channel failed or its acceptance is unknown | Check the log channel; nothing is resent |

## Technical details

<details>
<summary>What each update does</summary>

- `deploy` fetches, returns to the configured branch, resets hard to origin branch and runs `clean -fd`, then answers with a JSON result record.
- `pull` only moves forward and refuses when histories differ.
- `diff` returns a summary and untracked paths, not a full patch or a date-based report; `log ab 20` has no date filter.
- Commits use the server account's Git name and email; `push` uses its remote and keys. It never pretends to be the person who asked.

</details>
