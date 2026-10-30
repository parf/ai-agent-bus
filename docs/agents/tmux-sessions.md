# Tmux sessions: keep your terminal programs running

📌 **TL;DR:** Keeps a fixed set of terminal programs — AI sessions, monitors,
shells — running in one tmux session (a terminal that stays open after you
log out). One call brings back whatever is missing after a reboot or crash.
The programs run as you, outside the runner sandbox.

## Why you'd want it

**The problem.** After a reboot or a crash, someone has to log in and restart
each AI session and monitor by hand, in the right folder with the right options.

**The idea.** Write the list once: window name, folder, command. The helper starts
what is missing, stops or restarts what you name, and tells you what is really
running. Each program is a tab in the tmux status bar, and you can attach to look.

**When to use it**

- You run several long-lived AI sessions or monitors on one machine.
- They should come back with one call, from anywhere on the bus.
- Someone (or an AI) should restart a stuck one without a host login.

**When not to:** system services (use [systemd](systemd.md#what-it-does)), or
programs that should restart by themselves when they crash (it has no watchdog).

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| Team AI sessions after a reboot | `start` | Every configured session back in its folder |
| A stuck AI session | `restart` that window | A fresh session, others untouched |
| Morning health check | Ask for status | `ok: 5/5 windows up`, or which ones are down |
| Quiet night | `stop` the monitor | It gets ten seconds to save, then closes |
| A strict reviewer window | Add a [reviewer](#reviewer-window) to the list | A read-only Claude reviewer always ready |

## What it does

| Call | Does |
|---|---|
| `start [<name>...]` | Starts missing configured windows; running ones answer `already running`. No names means all. |
| `stop [<name>...]` | Asks the program to stop, allows ten seconds to save, closes the window. Missing ones answer `already stopped`. |
| `restart [<name>...]` | Stop, then start. |
| `list` | Each configured window: up or not, PID, uptime, folder; extra windows are marked `unmanaged`. |
| status ask | `ok: N/N windows up`; `degraded` when some are up; `failed` when none are, or the session is absent. |

Only configured windows are ever stopped; other windows and sessions are
untouched. There is no background watchdog: a later failure shows in `list`,
status or tmux.

<details><summary>Technical details: start and stop</summary>

- JSON requests use `{"method":"start","windows":["monitor"]}`; omit `windows` for all.
- Unknown names fail before any action and list the valid ones.
- Stop sends SIGTERM to the window's process group.
- Start creates the session with its first missing window, then adds the rest,
  observing each pane for two seconds before reporting `started`; an earlier
  exit answers `failed` with the output tail and removes the window.
- Closing a session's last window ends that session.
- No bus identity or reader is inferred from a command.
- One user-manager scope is used when creating the session; added windows use
  that tmux server. Restarting the keeper does not stop its windows.

</details>

## You'll need

Linux, Python 3 with pidfd support, tmux and the person's systemd user manager
(lingering enabled). Each command's own programs and logins must already work.

## Try this first

```sh
agent-bus help '#tmux-sessions@node.example'
agent-bus call '#tmux-sessions@node.example' list
agent-bus call '#tmux-sessions@node.example' --status
```

## Use examples

| I want to… | Command |
|---|---|
| Bring everything back after a reboot | `agent-bus call '#tmux-sessions@node.example' --wait=2m start` |
| Start just two windows | `agent-bus call '#tmux-sessions@node.example' 'start monitor shell'` |
| Restart a stuck session | `agent-bus call '#tmux-sessions@node.example' --wait=2m 'restart shell'` |
| Stop a monitor | `agent-bus call '#tmux-sessions@node.example' 'stop monitor'` |
| Look at the windows myself | `tmux attach -t <session>` (the configured `session`, default `tmux-sessions`) |

## Installing it

It belongs in the person's [user-account runner](../07-setup.md#a-persons-user-account-runner), which runs as them; an administrator installs that once (`sudo /usr/local/bin/agent-bus-setup --runner install --user-runner --for <person>`) and the person adds this package to it. The older way, its own Linux service per person, is below.

Running the main record and an instance in one runner takes sibling inventory keys, with the base key disabled ([how](../07-setup.md#a-persons-user-account-runner)).

<details><summary>Step by step</summary>

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  src/agents/standalone/tmux-sessions/install.py install \
  --name '#tmux-sessions@node.example' --user ann --allow '@owner'
```

Connection settings follow the [standalone interface](../../src/agents/standalone/starter.md#interface).
The service is `agent-bus-tmux-sessions.service`; it can stay unconfigured. A
second instance, such as `#tmux-sessions/billing@node.example`, installs the
same way with its own record, service
(`agent-bus-tmux-sessions-billing.service`), Configuration and default session
`billing`. Writing Configuration never starts a window; call `start` afterwards.

</details>

### Reviewer window

A window can launch a strict Claude reviewer with settings and a quoted deny
list; use the [read-only reviewer how-to](../user/ai-cli.md#read-only-reviewer-session).

## Configuration

The list of windows: a name, a folder and a command for each. Commands are
trusted settings written by the Owner or Maintainers; validation checks folders
but never runs commands.

<details><summary>Example and keys</summary>

Start from the shipped [config.json.dist](../../src/agents/standalone/tmux-sessions/config.json.dist)
(Claude, Codex, an env-file string and `htop`) and replace `/home/me/...`.

```json
{
  "session": "billing",
  "windows": [
    {"name":"monitor", "cwd":"/home/me/billing", "command":"htop"},
    {"name":"shell", "cwd":"/home/me/billing", "env":"A=1\nB=2", "command":"printf '%s %s\\n' \"$A\" \"$B\"; exec /bin/sh"},
    {"name":"claude", "cwd":"/home/me/billing", "env":{"AGENT_BUS_NAME":"#claude/billing"}, "command":"ab-claude --restart --tag billing"}
  ]
}
```

| Key | Meaning |
|---|---|
| `session` | tmux session name, 1–32 of `A-Za-z0-9-_`; default the instance part, or `tmux-sessions` |
| `state_home` | Existing absolute directory given to every window as `XDG_STATE_HOME`; default `/var/lib/agent-bus-tmux-sessions[-<instance>]` when that is a directory owned by the person (the old standalone unit's state, so launchers keep their locks and sessions after the move to the [user-account runner](../07-setup.md#a-persons-user-account-runner)), else inherited |
| `windows` | Up to 32 entries; default empty |
| `windows[].name` | Required, unique; same rule as `session`; case kept |
| `windows[].cwd` | Required existing absolute directory; `$HOME` is not expanded |
| `windows[].command` | Required one-line shell string, run by `/bin/sh -c`; quoting, `$HOME` and operators work |
| `windows[].env` | Optional [env-file](../runner/configuration.md#environment-files) string or string object, applied last |

Unknown keys are refused; put launcher flags in `command` and a bus name, if
wanted, in `AGENT_BUS_NAME`.

</details>

<details><summary>Technical details: environment and limits</summary>

- Commands start with a stripped environment: no starter credential,
  Configuration, address or delivered roles. Base: the person's home, shell,
  user-manager PATH, terminal and locale settings, `state_home`, then `env`
  (which may override PATH and `XDG_STATE_HOME`). The `state_home` default is
  checked when Configuration is read; a window gets only the path string. Absent TERM is `xterm-256color`; umask `0022`.
- `command`: at most 4096 UTF-8 bytes, without NUL, CR or newline.
- `env`: names match `[A-Za-z_][A-Za-z0-9_]*`; at most 64 final keys and 4096
  UTF-8 bytes per value; NUL refused. Env-file text is capped at 512 KiB and
  follows the shared parser: comments, quoting, reference refusal,
  line-numbered errors, later duplicates win. Both forms share one validation.
- Refused old keys: `sessions`, `runtime`, `args`, `tags`, `tag`, `window` and
  bus-name fields. The agent neither interprets `AGENT_BUS_NAME` nor waits for
  its reader.

</details>

## Secret keys

None. `AGENT_BUS_TOKEN` and `AB_ROLE_*` are refused in `env`: Configuration is
not a Secret. Do not put credentials in commands or `env`.

## Who may use it

Only the owner at first (`@owner`). Whoever the ACL admits may call every
method, including stop and restart; there are no extra tiers.

## Warnings and alerts

`#tmux-sessions` has its own Linux service, so the [runner entries](README.md#warnings-and-alerts) do not apply; it reports these itself. The page that shows them is in work.

| Level | When | What to do |
|---|---|---|
| alert | A window's command exited at start: `start` answers `failed` and removes the window | Fix its command or folder, then start it again |
| alert | Configuration is invalid (not JSON, unknown keys, a missing folder) | Fix the key the entry names |
| alert | `tmux` is missing, or the session cannot be created | Install tmux; check the person's user manager |
| warning | A window could not be stopped (`failed: could not stop window`) | Look with `tmux attach`; stop it by hand |
| warning | The status ask finds some windows down (`degraded`) | `list`, then `start` them |

## Troubleshooting

| If you see | Do this |
|---|---|
| `failed` with an output tail after `start` | The command exited at once; its window was removed. Fix it and start again. |
| `degraded` or `failed` from the status ask | `list` names missing or dead windows; start them again. |
| `unknown window; valid windows: …` | Use a configured window name. |
| A window is up but its program is not working | Up proves only that the program is alive, not signed in or ready; `tmux attach -t <session>` and look. |
