# Install a runner

📌 **TL;DR:** Choose a separate service account for ordinary helpers, or your login account for agents that must use your files and sessions.

## A person's runner

Install a [runner](README.md) for the person invoking `sudo`:

```sh
sudo /usr/local/bin/agent-bus-setup --runner install
```

Use `--for <person>` for another active bus User. Add `--dry-run` to inspect the plan. Setup checks the existing installation and applies only missing or changed settings.

| For person `alice` | What it is |
|---|---|
| `alice-ab-runner` | a system account of its own, `nologin`, so her runner's secrets are apart from everybody else's |
| `/var/lib/agent-bus/agent.d/` | shared agents' code, root-owned, 755, readable by every runner account |
| `/var/lib/agent-bus/alice-ab-runner/` | `alice-ab-runner`'s alone, 700: its token in `runner.env`, the tokens of the agents it created, their work directories |
| `#runner/alice@<realm>` | her Agent, allowing `@owner`; the runner uses its own token ([runner identity](security.md#what-runs-as-what)) |
| `agent-bus-runner@alice.service` | systemd unit under the separate account; only its private directory is writable, with delegated cgroups and no capabilities. It starts the runner of the current release, so an upgrade moves it; setup run from a checkout starts the runner beside it. It waits for the daemon and requires `runner.env` |

Replace `install` with a lifecycle verb when needed:

| Verb | Leaves the host with |
|---|---|
| `install` | everything above, started |
| `disable` | the unit stopped and disabled, every file kept |
| `enable` | the unit enabled and started; refused when nothing is installed |
| `uninstall` | no account, unit instance, `/var/lib/agent-bus/alice-ab-runner/` or `#runner/alice@<realm>`, and no template once the last runner goes; shared `agent.d/` always stays, even with `--purge`, and the agents it created stay hers |

The runner accepts `--agents <dir>` (default `/var/lib/agent-bus/agent.d`)
and required `--host <dir>` for its private directory.

Setup preserves a single existing runner record across realms; multiple matching records or a name exceeding 64 characters are refused.
Install and enable require an existing runner record to be an Agent owned by the intended person. A lookup failure other than a missing record
aborts the plan, including uninstall, before local recovery files are removed.

- The runner reaches the bus through the shared socket with its token, so no account is mapped for it.
- Setup manages `runner.env`, which contains the runner's `AGENT_BUS_TOKEN`; do not edit it by hand.
- A node with no runner still serves every face.

## A person's user-account runner

Use a second runner for agents that need the person's tmux sessions, CLI logins or home. It runs as `#runner/<person>-user@<realm>` under that person's systemd user manager.

| Use | When the agent |
|---|---|
| the system runner, `#runner/<person>@<realm>` | only needs the bus and what its package declares; the default for every agent |
| the user-account runner, `#runner/<person>-user@<realm>` | must act as the person: tmux sessions ([`#tmux-sessions`](../agents/tmux-sessions.md#installing-it)), their CLI logins, their home |

Install it for the person invoking `sudo`:

```sh
sudo /usr/local/bin/agent-bus-setup --runner install --user-runner
```

The same lifecycle verbs, `--for` and `--dry-run` apply. `--runner refresh --user-runner` refreshes every installed user-account runner.

| For person `alice` | What it is |
|---|---|
| `alice` | her own Linux login account (uid at least `UID_MIN`, a login shell); root, system and `nologin` accounts are refused. The bus keeps no uid, so the same name is the whole binding, and root runs the install |
| `/etc/systemd/user/agent-bus-runner.service` | one template for every person, root-owned: `ExecStart=/usr/local/bin/agent-bus-runner`, `Delegate=yes`, `KillMode=mixed`, a bounded restart, and no sandbox directives, because its agents act in her whole context; each agent's own sandbox stays the control |
| `~/.local/state/agent-bus/runner/` | hers, 0700: `runner.env` with the runner agent's token (never hers), the inventory, the agents' tokens |
| `~/.local/share/agent-bus/agent.d/` | hers, 0700: only the packages she puts there; the shared `agent.d` is never visible to it |
| `/var/lib/agent-bus/users/alice/files` | hers, 0700: the place to point `AGENT_BUS_BIGFILE_DIR` for her sessions ([bigfiles](../03-records-kv.md#bigfiles)); `users/` stays root's, 0755 |
| `#runner/alice-user@<realm>` | her agent on the bus, allowing `@owner` |
| linger | enabled, so the runner starts at boot without a login; uninstall leaves it as found |

- Setup creates the files folder privately and refuses a link, file or folder owned by another account. It corrects permissions to 0700, reports a different group without changing it, and leaves contents untouched.
- Set `AGENT_BUS_BIGFILE_DIR` explicitly to use that folder; setup does not set it for the person. macOS has no per-person runner or matching files folder.
- A person with no user-account runner makes the same folders by hand, with the same owner and mode (`users/` stays root's, 0755): `sudo install -d -m 0755 /var/lib/agent-bus/users` then `sudo install -d -m 0700 -o <person> -g <group> /var/lib/agent-bus/users/<person> /var/lib/agent-bus/users/<person>/files`. A later setup run accepts them as they are.
- Root never opens a path under her home: every observation and change there is made by setup re-executed as her (`setpriv`, no new privileges, no inherited environment), so a link she plants leads only where she can already go.
- Setup asks her user manager only about `agent-bus-runner.service`; it never stops her user manager or a session scope.

**Her secrets are readable by her own processes**, her AI sessions included:
the runner's token and its agents' tokens and env files live in her home.
Give it only agents she would trust with her login.

- `--upgrade` refreshes installed user-account runners one by one. Their agents restart, and an in-flight call can fail; check its outcome before retrying.
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

The inventory is `~/.local/state/agent-bus/runner/agents.json`.

For several instances, use sibling keys such as `tmux-sessions/main` and `tmux-sessions/billing`, and disable the base `tmux-sessions` entry. Set `name` on a sibling when it should keep the base record name. Parent and child work directories cannot run together.

Existing agents keep their name, Configuration and access list: hand their token to the runner as described under [identity handover](security.md#what-runs-as-what) before stopping any separate service. Keep the existing `state_home` when moving [tmux sessions](../agents/tmux-sessions.md#configuration).

</details>
