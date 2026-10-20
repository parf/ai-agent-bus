# Start a set of sessions

📌 **TL;DR:** Start a configured set of Claude Code, Codex or OpenCode sessions
in detached tmux terminals. Each uses the person's own account and the existing
bus launchers. A collision fails that entry while the other entries continue.

## Calls

```sh
agent-bus help '#sessions@parf.us'
agent-bus call '#sessions@parf.us' --wait 2m start
# Equivalent JSON body:
agent-bus call '#sessions@parf.us' --wait 2m '{"method":"start"}'
```

`start` takes no arguments and reports one line per configured session.
“Started” means tmux accepted the detached launch, not that the runtime has
finished signing in. An existing tmux session or live bus name is refused;
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
    {"name":"#claude/billing", "runtime":"claude", "cwd":"/home/parf/work/billing", "args":[], "tags":["review"]},
    {"name":"#codex/billing", "runtime":"codex", "cwd":"/home/parf/work/billing", "args":[], "tags":[]}
  ]
}
```

`tag` is optional; `sessions` defaults to an empty list. Only these keys and
the shown session keys are accepted. Names are canonical lowercase bus agent
names, with an optional realm. Working directories must exist and be absolute.
Runtimes are exactly `claude`, `codex`, and `opencode`. `args` and `tags` are optional.
[Record tags](../03-records.md#tags) are normalized; a set tag and session tags
combine and reach the launcher through `--tag`. Each entry is limited to 64
arguments (4096 characters each) and 32 tags; a set has at most 32 entries.
Each subprocess is limited to five seconds. Invalid Configuration starts nothing.

The tmux name removes the leading `#` and replaces punctuation other than
`-` and `_` with `-`: `#claude/billing@parf.us` becomes `claude-billing-parf-us`.
Duplicate names after this conversion are refused. The argv is fixed, with no
shell string; runtime arguments retain their exact values.

## Installing it

Requires tmux, Python 3, agent-bus, the selected `ab-*` launchers and runtime
logins for the installing person. This runs outside the runner sandbox:

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  src/agents/standalone/sessions/install.py install \
  --name '#sessions@parf.us' --user parf --allow '@owner'
```

Set the installer connection as described by the [standalone starter](../../src/agents/standalone/starter.md#interface).
The generated service is `agent-bus-sessions.service`. Install with an empty
set first, then let its owner supply the sessions. Configuration paths, arguments
and executable selection are trusted owner settings. Do not install it as root.

The starter checks live names through the person's local bus socket. Tmux children
start with a clean environment and the configured `AGENT_BUS_NAME`; they never
inherit this agent's bus credential, address, Configuration or delivered roles.
They use the person's own local connection and credentials. Runtime terminal
settings and the person's standard home/display/SSH-agent paths remain available.
Starting a set does not grant access to its sessions. The installed ACL is `@owner`.

To inspect a session, use tmux directly, for example
`tmux attach -t claude-billing`; the names remain reserved as described by
[launcher session names](../06-launchers.md#session-names).
