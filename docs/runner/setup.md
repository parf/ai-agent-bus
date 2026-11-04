# Install a runner

📌 **TL;DR:** Choose a separate service account for ordinary helpers, or your login account for agents that must use your files and sessions.

## A person's runner

- 📌 A [runner](README.md) is a person's, and setup installs one only when asked, for one person at a time: `sudo /usr/local/bin/agent-bus-setup --runner install|uninstall|enable|disable [--for <person>]`.
- Every verb looks first and does only what is missing, so a second run changes nothing, and `--dry-run` prints the plan.

| For person `alice` | What it is |
|---|---|
| `alice-ab-runner` | a system account of its own, `nologin`, so her runner's secrets are apart from everybody else's |
| `/var/lib/agent-bus/agent.d/` | shared agents' code, root-owned, 755, readable by every runner account |
| `/var/lib/agent-bus/alice-ab-runner/` | `alice-ab-runner`'s alone, 700: its token in `runner.env`, the tokens of the agents it created, their work directories |
| `#runner/alice@<realm>` | her agent on the bus (the bus realm; a runner already registered under any realm keeps that record, two such records refuse the install, and a realm too long for the 64-character name refuses it too), allowing `@owner`; the runner is started with its token, never with hers ([runner identity](security.md#what-runs-as-what)) |
| `agent-bus-runner@alice.service` | an instance of one template, `/etc/systemd/system/agent-bus-runner@.service`: her account, only its private directory writable, `Delegate=yes`, no capability, `Wants=`/`After=` the daemon, never `Requires=`, and no start while `runner.env` is missing |

| Verb | Leaves the host with |
|---|---|
| `install` | everything above, started; `--for` defaults to whoever ran `sudo`, and must be an active bus User |
| `disable` | the unit stopped and disabled, every file kept |
| `enable` | the unit enabled and started; refused when nothing is installed |
| `uninstall` | no account, unit instance, `/var/lib/agent-bus/alice-ab-runner/` or `#runner/alice@<host>`, and no template once the last runner goes; shared `agent.d/` always stays, even with `--purge`, and the agents it created stay hers |

The runner accepts `--agents <dir>` (default `/var/lib/agent-bus/agent.d`)
and required `--host <dir>` for its private directory.

Install and enable refuse an existing runner record unless it is an Agent
owned by the intended person. A lookup failure other than a missing record
aborts the plan, including uninstall, before local recovery files are removed.

- The runner reaches the bus through the shared socket with its token, so no account is mapped for it.
- `runner.env` is setup's alone to write and holds one line, `AGENT_BUS_TOKEN=<token>`, a token being hex digits: systemd reads it as the unit's `EnvironmentFile`, setup reads it with the bus's [env-file parser](../03-records-service.md#env-file-syntax), and that one line means the same to both, so it is no second syntax to keep in step.
- A node with no runner still serves every face.

## A person's user-account runner

- 📌 Some agents must act **as the person**: start their tmux sessions, use their logins, their user manager and their files.
- The system runner above deliberately cannot, since it runs under its own account.
- For those few agents a person gets a second runner, `#runner/<person>-user@<realm>`, running as them under their own systemd user manager.
- Install it only through setup; everything else stays in the system runner.

| Use | When the agent |
|---|---|
| the system runner, `#runner/<person>@<host>` | only needs the bus and what its package declares; the default for every agent |
| the user-account runner, `#runner/<person>-user@<host>` | must act as the person: tmux sessions ([`#tmux-sessions`](../agents/tmux-sessions.md#installing-it)), their CLI logins, their home |

- `sudo /usr/local/bin/agent-bus-setup --runner install|uninstall|enable|disable --user-runner [--for <person>] [--dry-run]`, and `--runner refresh --user-runner` for every installed one.
- `--for` defaults to whoever ran `sudo`.
- Like the system runner, every verb looks first and does only what is missing, so a second run changes nothing.

| For person `alice` | What it is |
|---|---|
| `alice` | her own Linux login account (uid at least `UID_MIN`, a login shell); root, system and `nologin` accounts are refused. The bus keeps no uid, so the same name is the whole binding, and root runs the install |
| `/etc/systemd/user/agent-bus-runner.service` | one template for every person, root-owned: `ExecStart=/usr/local/bin/agent-bus-runner`, `Delegate=yes`, `KillMode=mixed`, a bounded restart, and no sandbox directives, because its agents act in her whole context; each agent's own sandbox stays the control |
| `~/.local/state/agent-bus/runner/` | hers, 0700: `runner.env` with the runner agent's token (never hers), the inventory, the agents' tokens |
| `~/.local/share/agent-bus/agent.d/` | hers, 0700: only the packages she puts there; the shared `agent.d` is never visible to it |
| `/var/lib/agent-bus/users/alice/files` | hers, 0700: the place to point `AGENT_BUS_BIGFILE_DIR` for her sessions ([bigfiles](../03-records-kv.md#bigfiles)); `users/` stays root's, 0755 |
| `#runner/alice-user@<host>` | her agent on the bus, allowing `@owner` |
| linger | enabled, so the runner starts at boot without a login; uninstall leaves it as found |

- Her files folder is the one place under `/var/lib/agent-bus` that is hers. A new folder is made under a random name in `users/`, which only root can write, given its owner and mode, and renamed into place without replacing anything, so nothing she does can reach it before it is hers; setup never changes the owner of a folder that was already there. One that is already hers with another mode (the setuid, setgid and sticky bits count) is set to 0700 and setup says so; one with another group is left and said; a folder of another owner, a link, or a file in its place is refused; the contents are never touched. A leftover staging directory of an interrupted run (root-owned, empty, older than a minute, in the state root or `users/`) is removed at the next run. Nothing sets `AGENT_BUS_BIGFILE_DIR` for her: that stays unset until she (or a launcher she configures) names it. This is Linux only; the macOS setup has no per-person runner, so it makes no such folder.
- A person with no user-account runner makes the same folders by hand, with the same owner and mode (`users/` stays root's, 0755): `sudo install -d -m 0755 /var/lib/agent-bus/users` then `sudo install -d -m 0700 -o <person> -g <group> /var/lib/agent-bus/users/<person> /var/lib/agent-bus/users/<person>/files`. A later setup run accepts them as they are.
- Root never opens a path under her home: every observation and change there is made by setup re-executed as her (`setpriv`, no new privileges, no inherited environment), so a link she plants leads only where she can already go.
- Setup asks her user manager only about `agent-bus-runner.service`; it never stops her user manager or a session scope.

**Her secrets are readable by her own processes**, her AI sessions included:
the runner's token and its agents' tokens and env files live in her home.
Give it only agents she would trust with her login.

- `git-install.sh` and `--upgrade` refresh every installed user-account runner one by one: the template is rewritten when it differs and each runner restarts.
- She sees her runner's agents restart (a call in flight to one of them fails once); her tmux sessions do not.
- The template goes with the last runner the registry knows of, so a unit enabled by hand without a record loses it then.
- Her tmux sessions are untouched: each lives in its own `ab-session-<name>.scope` beside the runner in her user manager, outside the runner's cgroup, so a reload, a stop, a crash or a refresh leaves every window and pid as it was.

<details><summary>Add an agent to it</summary>

Enable the bundled agent in the person's inventory, preserving other entries, then reload the runner:

```json
{"tmux-sessions":{"enabled":true,"sandbox":"off"}}
```

```sh
agent-bus call '#runner/alice-user@node.example' reload
```

The inventory is `~/.local/state/agent-bus/runner/agents.json`. A bundled agent needs explicit `enabled:true`.

For several instances, use sibling keys such as `tmux-sessions/main` and `tmux-sessions/billing`, and disable the base `tmux-sessions` entry. Set `name` on a sibling when it should keep the base record name. Parent and child work directories cannot run together.

Existing agents keep their name, Configuration and access list: hand their token to the runner as described under [identity handover](security.md#what-runs-as-what) before stopping any separate service. Keep the existing `state_home` when moving [tmux sessions](../agents/tmux-sessions.md#configuration).

</details>
