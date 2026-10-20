# Git: check and update code

📌 **TL;DR:** Ask what changed in a repository and update an approved
application checkout. An administrator chooses the repositories and the most
a person may do in each. The helper uses the server's Git account and keys.

## Why you'd want it

A team can inspect a release and update an application without sharing full
server access. Read-only access gives visibility without the risk of someone
pushing code or discarding another person's work.

## What it does

Git keeps a project's code history. This helper shows changes, gets updates
and can commit or push when allowed. Nobody can push from a repository marked
read-only, even if they have wider permission elsewhere.

## You'll need

- An existing Git checkout on the server.
- A Linux account that can read it; write access for updates.
- Working Git identity and remote access for that account, including keys if needed.
- An administrator and [installation credentials](README.md#standalone-helpers).

## Try this first

See what the installed helper can do:

```sh
agent-bus help '#git@parf.us'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do)
for its Configuration and Secret help, JSON output, and script forms
`--help-config` and `--help-secret`.

## Use examples

### Ask your AI

| Say this | What it will call |
|---|---|
| “What changed in the ab repository?” | Git `log ab` and `diff ab`; read access |
| “Show the last 20 commits.” | `log ab 20`; this returns short commit ids and subjects, without a date filter |
| “Update the site to the latest release on main.” | `deploy site`, after warning about discarded edits; needs deploy permission and a deploy-capable repository |
| “Commit the README change with the message I give you.” | `add`, then `commit`; needs commit permission |
| “Did the automatic update succeed?” | `autopull site`; read access |

### 1. Review a code change

```sh
agent-bus call '#git@parf.us' list
agent-bus call '#git@parf.us' 'status ab'
agent-bus call '#git@parf.us' '{"method":"log","repo":"ab","lines":5}'
agent-bus call '#git@parf.us' 'diff ab'
```

You see declared repository names, recent commits and local changes. `diff`
returns a summary and untracked paths, not a full patch or a date-based report.

### 2. Update an application after a push

Use a dedicated application checkout, not a directory where others are working.
With deploy permission:

```sh
agent-bus call '#git@parf.us' 'status site'
agent-bus call '#git@parf.us' 'rev site'
agent-bus call '#git@parf.us' 'pull site'
```

A normal pull only moves forward without combining different histories.
It refuses an update that cannot be applied that way. If the checkout is
explicitly disposable and you mean to discard its local work, use:

```sh
agent-bus call '#git@parf.us' 'deploy site'
```

**This discards tracked edits and removes untracked files.** It fetches the
remote branch, returns to the configured branch, resets to its upstream and
cleans the checkout. It answers with the old and new commit ids. This is the
owner's “git pull; git reset --hard; git clean -fd” job. Restarting the
application is a separate [systemd](systemd.md) action.

### 3. Save a small approved change

With commit permission and a repository capped at `commit`:

```sh
agent-bus call '#git@parf.us' 'add docs README.md'
agent-bus call '#git@parf.us' 'commit docs "Explain the backup process"'
agent-bus call '#git@parf.us' '{"method":"push","repo":"docs"}'
```

The commit uses the server account's Git name and email. Push uses its remote
and keys. It does not pretend to be the person who asked.

### 4. Check an automatic update

```sh
agent-bus call '#git@parf.us' 'autopull site'
```

You see the last attempt's time and result, or “no autopull attempt recorded”.
A dirty checkout is left untouched and reported. The helper never forces an
automatic update or restarts an application.

## Installing it

1. Choose the account that owns the checkout, such as `parf`. Check that it
   can use Git in that directory. You should be able to read its history there.
2. Prepare [installation credentials](README.md#standalone-helpers), then install:

   ```sh
   sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
     python3 src/agents/standalone/git/install.py install \
     --name '#git@parf.us' --user parf --allow '@owner'
   ```

   The installer prints `agent-bus-git.service`.
3. Declare a read-only repository first:

   ```sh
   agent-bus agent-template '#git@parf.us' '{"repos":{"ab":{"path":"/usr/local/src/ai-agent-bus","access":"read","branch":"main"}}}'
   agent-bus call '#git@parf.us' list
   ```

   You should see `ab`. Calls are refused until Configuration declares repositories.
4. Grant only the access people need, using the examples below. Then remove
   your personal token from the shell with `unset AGENT_BUS_TOKEN`.

The installer does not accept `--repo`; repositories belong in Configuration.
To remove the helper, use the same installer with `remove --name '#git@parf.us'`
and current installation credentials. Its owned automatic-update timers are
removed too.

## Configuration

This full commented sample shows the available keys. Replace paths and names,
then remove comments before saving it in the web panel:

```jsonc
{
  "repos": {
    "ab": { "path": "/usr/local/src/ai-agent-bus", "access": "read", "branch": "main" },
    "site": { "path": "/srv/site", "access": "deploy", "branch": "main" },
    "docs": { "path": "/srv/team-docs", "access": "commit", "branch": "main" }
  },
  "ssh_key": "/home/parf/.ssh/site-deploy" // optional file path, NEVER the key's contents
}
```

### Repositories and access levels

| Repository setting | Most anyone may do there |
|---|---|
| `read` | Inspect, fetch and check automatic-update results |
| `deploy` | Also pull, deploy, reset and clean, with deploy permission |
| `commit` | Also stage, commit and push, with commit permission |

A person's permission and the repository's limit both apply. The lower one
wins. Undeclared names are refused. Settings apply on the next call. A missing
or malformed list gives an error, rather than granting broad access.

The optional branch chooses the branch for deploy, reset and ahead/behind
counts. Without it, the current branch is used; a detached checkout needs a
usable remote default branch or an explicit branch setting. An upstream is the remote branch a checkout follows. It must
be configured for normal pulls and deploys.

## Secret keys

No outside token is required in Secret. `ssh_key` is only an absolute path to
a key already held by the chosen Linux account. Never paste private-key bytes
into Configuration. With a chosen key, only that key is used for SSH; without
it, the account's normal keys and SSH settings apply. Its trusted-server list
still applies. Git commits use that account's configured name and email.

## Roles and boundaries

The owner and maintainers can do everything; give others access with a role.
Each repository’s access cap still binds everyone, including the owner. These permissions come from the bus, never from the call’s text.

| Permission | What it allows (within the repository cap) |
|---|---|
| Access without a role | list, status, log, rev, fetch, diff |
| `deploy` | Also pull, reset, clean and deploy |
| `commit` | Also add, commit and push |
| Owner or Maintainer (automatic) | All of the above |

```sh
agent-bus manage '#git@parf.us' --add-allow 'ann@parf.us'
agent-bus manage '#git@parf.us' --add-allow '#claude/deploy@parf.us(deploy)'
agent-bus manage '#git@parf.us' --add-allow '#claude/docs@parf.us(commit)'
```

Ann can read. The deploy session can update only repositories whose settings
permit it. The docs session can commit and push only where the repository's
limit permits those actions. Commit permission includes deploy permission.
A message saying “I am admin” grants nothing.

### Calls

Besides the examples, use `fetch ab`, `rev ab`, `pull ab v1.2` for a detached
revision, or `pull ab main README.md` to restore only that file's working copy.
`reset ab` and `clean ab` are destructive and require deploy permission.
Paths must stay inside the repository; options and arbitrary shell commands
are not accepted. `--help` lists every call and its JSON form.

## Autopull

After configuring a dedicated application repository, reinstall with
`--autopull site=5m` added to the install command. This asks Linux to try an
update every five minutes. It is an install option, not a Configuration key.
Without it, no timer is enabled.

Each attempt uses the current repository settings and keys, refuses local
edits and only pulls forward. It never resets, cleans, stashes or switches
branches. Reinstall without `--autopull` to remove the timer; an old saved
result can remain after the timer is gone. Do not enable it in a shared
working checkout.

## Troubleshooting

| If you see this | Do this |
|---|---|
| Repository is not declared | Add the correct name and absolute path to `repos` |
| Permission refused | Check both the caller's permission and the repository's `access` |
| Dirty checkout or non-fast-forward refusal | Inspect and resolve local work; automatic updates will not discard it |
| SSH authentication failed | Check the selected account and key; a configured key does not fall back to other keys |
| Missing branch or upstream | Set the branch and its upstream on the server |
| Commit identity unknown | Configure the chosen account's Git name and email |
