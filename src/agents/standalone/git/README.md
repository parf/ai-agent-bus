# Git agent

📌 **TL;DR:** B.8 inspects and synchronizes repositories declared at install.
It runs standalone under an explicitly chosen account; delivered roles control
writes. Commits use that server's git identity, never the bus caller's.

## Calls

Text is `method repo [arguments]`; `list` needs no repo. Quote paths or commit
messages containing spaces. JSON uses exactly the keys shown below, with
optional fields omitted. No roles, author, git options or arbitrary commands
are accepted in a body. Calls run one at a time per repository in this instance.

| Text | JSON | Answer |
|---|---|---|
| `list` | `{"method":"list"}` | Declared repository names |
| `status ab` | `{"method":"status","repo":"ab"}` | Branch and local/untracked changes |
| `log ab [N]` | `{"method":"log","repo":"ab","lines":20}` | Recent one-line commits; N is 1..1000, default 20 |
| `rev ab` | `{"method":"rev","repo":"ab"}` | HEAD commit and branch, or detached |
| `autopull ab` | `{"method":"autopull","repo":"ab"}` | Last automatic attempt: time, exit status and output or failure; no role required |
| `fetch ab` | `{"method":"fetch","repo":"ab"}` | Fetch, then ahead/behind counts against the installed branch's upstream |
| `diff ab` | `{"method":"diff","repo":"ab"}` | Local changes against HEAD as a stat, plus untracked paths |
| `pull ab` | `{"method":"pull","repo":"ab"}` | Fetch and fast-forward-only pull; divergence or conflicting edits fail |
| `pull ab rev` | `{"method":"pull","repo":"ab","rev":"v1.0"}` | Fetch and detached checkout of the revision |
| `pull ab rev file` | `{"method":"pull","repo":"ab","rev":"main","file":"README.md"}` | Restore that file's working copy only; HEAD and other files stay unchanged |
| `deploy ab` | `{"method":"deploy","repo":"ab"}` | Return to installed branch's upstream, discarding tracked edits and untracked files; old and new HEAD |
| `reset ab` | `{"method":"reset","repo":"ab"}` | Hard reset to the installed branch's currently fetched upstream |
| `clean ab` | `{"method":"clean","repo":"ab"}` | `git clean -fd`: remove untracked files/directories, keeping ignored files |
| `add ab path…` | `{"method":"add","repo":"ab","paths":["README.md"]}` | Stage literal paths |
| `commit ab "message"` | `{"method":"commit","repo":"ab","message":"Update README"}` | Commit staged changes using server git configuration |
| `push ab` | `{"method":"push","repo":"ab"}` | Push according to the repository's configured remote/branch |
| `help` | Text `help` or `agent-bus call '#git@host' --help` | Purpose, allowed methods, arguments, answers and examples, filtered by caller tier |

The owner's “pull; reset --hard; clean -fd” is implemented by `deploy` as fetch,
return to the installed branch, hard reset to its upstream, then clean; this
works even when local edits would make an ordinary pull refuse. `deploy` is
intentionally destructive and requires operate; standalone reset/clean require
admin. Each git command is bounded to 60 seconds; timeout stops its private process
group, including SSH children. Errors stop the sequence,
so a failed multi-step operation may leave earlier steps applied. Inspect
status before retrying. Returned stdout/stderr is capped at 256 KiB each.

Text answers are git output or `ok`, with failures prefixed `error:`. JSON
answers include `status` and command `stdout`/`stderr`, with `error` on failure
and `truncated` when capped. `rev` adds `head`/`branch`; `fetch` adds
`ahead`/`behind`; `deploy` adds `old_head`/`new_head`; `list` adds `repos`.

## Repositories and access levels

Pending (№B.10); the current tiers below hold until it lands. Owner,
2026-10-19: the Configuration lists the git roots and each one's access level,
which caps it; the caller's roles from the ACL say what the caller may do; even
the best role cannot go over the repository's cap.

```json
{ "repos": { "ab": { "path": "/usr/local/src/ai-agent-bus", "access": "deploy" } },
  "ssh_key": "/home/parf/.ssh/deploy_ab" }
```

| Level | Methods | Caller's role |
|---|---|---|
| `read` | list, status, log, rev, fetch, diff, autopull | none |
| `deploy` | pull (all forms, including one file at a revision), reset, clean, deploy | `deploy` |
| `commit` | add, commit, push | `commit` |

Levels are cumulative: `commit` includes `deploy` and `read`. What a call may do
is the lower of the repository's `access` and the caller's role. A repository
not in `repos` does not exist for the agent; `--repo` at install goes away.
Configuration is read at each call, so a change applies to the next call.

| Example | `access` | Effect |
|---|---|---|
| a production checkout | `deploy` | an operator with `commit` still cannot push from it |
| a docs or config repo agents edit | `commit` | a `commit` agent pushes; a `deploy` caller only pulls |
| a shared checkout others work in | `read` | nobody can reset it from the bus |

## Roles and boundaries

| Delivered tier | Methods |
|---|---|
| none | list, status, log, rev, fetch, diff, autopull |
| operate | pull (all forms), deploy, add, commit, push |
| admin | all of the above, reset, clean |

Only `AB_ROLE_OPERATE=1` and `AB_ROLE_ADMIN=1` delivered by the bus count.
An undeclared repository is refused even for admin. Paths are relative literal
paths inside that repository: no options, absolute paths, `..`, `.git`, or
symlinks escaping it. Revisions are names/commit IDs with ordinary ancestry
suffixes, resolved to a commit ID before use; option-looking input is refused
before git runs. Git hooks and repository/server configuration are trusted
installation inputs. Automatic pulls are opt-in at installation; see [autopull](#autopull).

## Install and identity

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN python3 src/agents/standalone/git/install.py install \
  --name '#git@host' --user deploy --repo ab=/srv/ai-agent-bus \
  --allow '#operator@host(operate),#administrator@host(admin)'
```

Repeat `--repo name=/absolute/path` for each repository. It must be a working
repository on a branch; the installer records its canonical path and that
branch in the unit's fixed arguments, allowing deploy to return from a pinned
detached revision. An upstream must be configured for pull, reset, deploy and
fetch counts. Choose `--user` with access to the repositories, git identity and
SSH keys. The starter's dedicated default account usually has none of these.
Install twice is idempotent; removal uses `remove --name '#git@host'` with the
same starter directories and current caller credentials. The
[shared starter](../starter.md#standalone-starter) owns the unit and token lifecycle.

Keep secrets in Secret and config in Configuration; see
[configuration and its script environment](../../../../docs/03-records-agent.md#configuring-a-template).
Optional Configuration is `{"ssh_key":"/home/deploy/.ssh/git-key"}`: a readable
absolute **path**, never private-key bytes. When set, git's SSH uses only that
identity, no agent or default identities and no account SSH config. The
account's known-host trust still applies. With no key configured, its normal
SSH configuration and keys apply. The agent never sets git author/committer;
the server's git configuration supplies them. A new call sees updated Configuration.

```sh
agent-bus call '#git@host' 'status ab'
agent-bus call '#git@host' '{"method":"log","repo":"ab","lines":5}'
# After inspecting the changes, with operate:
agent-bus call '#git@host' 'deploy ab'
```

## Autopull

Add `--autopull ab=5m` to install for each declared repository that should
pull automatically. Intervals are positive systemd durations such as `30s`,
`5m` or `1h30m`, set at install time; reinstall to change them. With no
`--autopull`, no timer is enabled. This is not a Configuration key.

Each repository gets a systemd timer and a oneshot unit under the chosen
account, using the agent's own credential and current Configuration for its
SSH key. An attempt takes the same repository lock as calls, refuses dirty
trees, and uses the same ff-only pull. Divergence, local edits or a failed
fetch are logged in the oneshot unit's journal and saved atomically in the
agent's state directory. It never resets, cleans, stashes, switches branches
or restarts another service. `autopull ab` (or JSON method `autopull` with
`repo`) answers the last attempt and its failure; before any attempt it says
`no autopull attempt recorded`.

Reinstalling without a timer removes it, and removal stops and removes every
owned timer and oneshot. A preserved last result is historical evidence, not
proof that a timer is still enabled. Leave autopull disabled on checkouts
where people or other agents are working.

## Verification

The slow `agent_git` shard uses disposable local repositories, a private SSH
server/upstream, and the shared starter's private unit controller. It checks
actual delivered roles, install idempotence/removal, destructive-operation
results, fixed identity, configured-key authentication and default-key fallback.
Mutants remove option/scope/tier checks, trust body roles/author, ignore the key,
or restore more than the named file. No live repository is reset, cleaned or
pushed by the checks; live deployment verification uses list/status/log/rev/diff/autopull only.
A real private systemd-user timer pulls a fixture upstream commit, reports dirty-tree refusal without discarding edits, and is removed afterwards.
