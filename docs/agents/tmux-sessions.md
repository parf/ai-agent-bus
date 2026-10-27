# Start a tmux session set

📌 **TL;DR:** Start a configured set of Claude Code, Codex or OpenCode sessions
in detached tmux terminals. Each uses the person's own account and the existing
bus launchers. A collision fails that entry while the other entries continue.

## Calls

```sh
agent-bus help '#tmux-sessions@demo'
agent-bus call '#tmux-sessions@demo' --wait 2m start
# Equivalent JSON body:
agent-bus call '#tmux-sessions@demo' --wait 2m '{"method":"start"}'
```

`start` takes no arguments and reports one line per configured session.
“Started” means the tmux pane is still running and the bus name has attached a
reader. Each entry waits up to 20 seconds; an immediate exit or missing reader
reports `failed` with a short output tail. An exited session is removed after its output is captured, so it can be retried.
A still-running session without a reader remains available for inspection;
`start` never replaces a live session. An existing tmux session or live bus name is refused;
a launcher's later failure is visible in its tmux terminal. No sessions are
stopped or replaced. There are no list, status, stop or attach methods yet.
Claude uses the [approved channel plugin](../06-launchers.md#claude-channel-plugin).
Before the first `start`, a person opens `ab-claude` once in each configured
`cwd` and answers Claude Code’s ordinary workspace-trust prompt. It appears
once per new working directory. The starter does not pre-accept trust or
bypass that security prompt on the person’s behalf.

## Configuration

The owner edits this agent's [Configuration](../03-records-agent.md#configuring-a-template):

```json
{
  "tag": "billing",
  "sessions": [
    {"name":"#claude/billing", "runtime":"claude", "tmux":"AB", "cwd":"/home/alice/work/billing", "args":[], "tags":["review"]},
    {"name":"#codex/billing", "runtime":"codex", "cwd":"/home/alice/work/billing", "args":[], "tags":[]}
  ]
}
```

`tag` is optional; `sessions` defaults to an empty list. Only these keys and
the shown session keys are accepted. Names are canonical lowercase bus agent
names, with an optional realm. Working directories must exist and be absolute.
Runtimes are exactly `claude`, `codex`, and `opencode`. `tmux`, `args` and `tags` are optional.
[Record tags](../03-records.md#tags) are normalized; a set tag and session tags
combine and reach the launcher through `--tag`, alongside the automatic
`ai-cli` and runtime tags. Put a set-wide label in `tag`, not in session `args`. Each entry is limited to 64
arguments (4096 characters each) and 32 tags; a set has at most 32 entries.
Each subprocess is limited to five seconds; startup readiness has the separate
20-second deadline above. Invalid Configuration starts nothing.

An explicit `tmux` name preserves case and must contain 1–32 ASCII letters,
digits, `-` or `_`. Every resolved name must be unique in the set. It is used
unchanged for `has-session`, tmux `-s` and the user-manager unit name.

Without `tmux`, the name removes the leading `#` and replaces punctuation other than
`-` and `_` with `-`: `#claude/billing@parf.us` becomes `claude-billing-parf-us`.
Duplicate names after this conversion are refused. The argv is fixed, with no
shell string; runtime arguments retain their exact values.

## Installing it

Requires tmux, Python 3, systemd’s user manager (with lingering enabled),
agent-bus, the selected `ab-*` launchers and runtime
logins for the installing person. This runs outside the runner sandbox:

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  src/agents/standalone/tmux-sessions/install.py install \
  --name '#tmux-sessions@demo' --user alice --allow '@owner'
```

Set the installer connection as described by the [standalone starter](../../src/agents/standalone/starter.md#interface).
The generated service is `agent-bus-tmux-sessions.service`. Install with an empty
set first, then let its owner supply the sessions. Configuration paths, arguments
and executable selection are trusted owner settings. Do not install it as root.

The starter checks live names through the person's local bus socket. Tmux children
start with a clean environment and the configured `AGENT_BUS_NAME`; they never
inherit this agent's bus credential, address, Configuration or delivered roles.
They use the person's own local connection and credentials. Runtime lookup uses
the person's systemd user-manager PATH, so a system unit's minimal PATH does
not hide per-user installations. A missing terminal type defaults to
`xterm-256color`. Runtime terminal
settings and the person's standard home/display/SSH-agent paths remain available.
Starting a set does not grant access to its sessions. The installed ACL is `@owner`.

Each tmux launch runs in the person’s user-manager scope, outside this
service’s cgroup. Restarting, stopping or reinstalling this starter does not
stop its sessions. A session starts with umask `0022`, independently of the
starter or an existing tmux server’s umask.

To inspect a session, use tmux directly, for example
`tmux attach -t claude-billing`; the names remain reserved as described by
[launcher session names](../06-launchers.md#session-names).
