# Keep named tmux windows running

📌 **TL;DR:** `#tmux-sessions` keeps one configured tmux session of named command
windows. Start missing windows, stop or restart configured ones, and inspect
real state. Commands run as the installing person, outside the runner sandbox.

Each window shows as a tab in the tmux status bar.

## Calls

All methods use the record's ACL alone. Whoever the ACL admits may call any
method, including stop and restart; there are no extra role tiers or agent-side
Owner checks. The installer defaults to `@owner`; the Owner decides who to add.

```sh
agent-bus call '#tmux-sessions@demo' --wait=2m start
agent-bus call '#tmux-sessions@demo' 'start monitor shell'
agent-bus call '#tmux-sessions@demo' 'stop monitor'
agent-bus call '#tmux-sessions@demo' --wait=2m 'restart shell'
agent-bus call '#tmux-sessions@demo' list
agent-bus call '#tmux-sessions@demo' --status
agent-bus help '#tmux-sessions@demo'
tmux attach -t work
```

| Method | Result |
|---|---|
| `start [<name>...]` | Start only missing configured windows; present names answer `already running`. No names means every configured window. |
| `stop [<name>...]` | Send SIGTERM to each selected window's process group, allow up to ten seconds to save, then close its window. Missing windows answer `already stopped`. |
| `restart [<name>...]` | Stop, then start selected configured windows with the same checks. |
| `list` | Each configured window's presence, pane liveness, PID, uptime and cwd; extra windows are marked `unmanaged`. |
| status ask | `ok: N/N windows up`; `degraded` when some are up; `failed` when none are up or the session is absent. Missing or dead window names are listed. |

Unknown names fail before any action and list the valid configured names. Only
configured windows are stopped; other windows and sessions are never changed.
Closing a session's last window ends that session. JSON requests use
`{"method":"start","windows":["monitor"]}`; omit `windows` for all.

Start creates the session with its first missing window, then adds subsequent
windows. It observes the pane for two seconds before reporting `started`.
An immediate exit returns `failed` and the captured output tail; its dead
window is removed, so fixing the command allows a retry. No bus identity or
reader is inferred from a command. A later command failure is visible through
list, status or tmux; there is no background watchdog.

## Configuration

Start from the shipped [config.json.dist](../../src/agents/standalone/tmux-sessions/config.json.dist).
Its generic examples include Claude, Codex in a subdirectory, an env-file
string and a plain `htop` window. Replace `/home/me/...` with existing paths.
The parser checks directories, but never runs commands while validating.

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
| `session` | The one tmux session name: 1–32 ASCII letters, digits, `-` or `_`. Defaults to the agent's instance component, or `tmux-sessions` for the main record. |
| `windows` | At most 32 entries; defaults to empty. |
| `windows[].name` | Required window name, same character/length rule as session; unique within the set; case is preserved. |
| `windows[].cwd` | Required existing absolute directory. `$HOME` is not expanded here. |
| `windows[].command` | Required non-empty shell string, at most 4096 UTF-8 bytes, without NUL, CR or newline. It runs as `/bin/sh -c`; quoting, `$HOME` and shell operators work. |
| `windows[].env` | Optional string in the [shared env-file format](../runner/configuration.md#environment-files), or object of string keys and values. Applied after the stripped base and user-manager PATH, so an explicit PATH can override it. |

Unknown keys are refused, including the old `sessions`, `runtime`, `args`,
`tags`, `tag`, `window` and bus-name fields. Put launcher flags in `command`;
a bus name, if wanted, is an explicit `AGENT_BUS_NAME` environment setting.
The agent neither interprets it nor waits for its reader.

Environment names match `[A-Za-z_][A-Za-z0-9_]*`; at most 64 final keys and
4096 UTF-8 bytes per value are allowed. NUL is refused. Env-file text is capped
at 512 KiB and uses the same comments, quoting, reference refusal and
line-numbered errors as the Go parser, with later duplicate assignments winning.
Both forms share the same validation. `AGENT_BUS_TOKEN` and `AB_ROLE_*` are
refused: Configuration is not a Secret. Do not put credentials in commands or
environment settings.

The command starts with a stripped environment: no starter credential,
Configuration, address or delivered roles. It gets the person's home, shell,
user-manager PATH, normal terminal/locale paths, then explicit `env` settings.
An absent TERM defaults to `xterm-256color`; umask is `0022`. Commands are
trusted Configuration written by the record's Owner or Maintainers. Readiness
proves a pane is alive, not that its application is signed in or ready for work.

## Installing it

Requires Linux, Python 3 with pidfd support, tmux and the person's systemd user
manager (lingering enabled). Commands' own executables and logins must exist.
One user-manager scope is used when creating the tmux session; added windows
use that existing tmux server. Restarting the keeper does not stop its windows.

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  src/agents/standalone/tmux-sessions/install.py install \
  --name '#tmux-sessions@demo' --user alice --allow '@owner'
```

Supply the install connection using the [standalone interface](../../src/agents/standalone/starter.md#interface).
The main service is `agent-bus-tmux-sessions.service`. It can remain unconfigured.

A second instance has its own record, unit and Configuration:

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  src/agents/standalone/tmux-sessions/install.py install \
  --name '#tmux-sessions/billing@demo' --user alice --allow '@owner'
agent-bus config '#tmux-sessions/billing@demo' - < billing.json
agent-bus call '#tmux-sessions/billing@demo' --wait=2m start
tmux attach -t billing
```

Its default session name is `billing`, and its service is
`agent-bus-tmux-sessions-billing.service`. Writing its Configuration does not
configure the main record. Changing Configuration alone never starts a window.
