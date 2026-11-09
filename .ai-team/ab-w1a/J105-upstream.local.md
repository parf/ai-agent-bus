# Claude Code 2.1.319: a plugin's stdio MCP server is never started (2.1.318 starts it)

**Version:** 2.1.319 (native, linux x86_64). Last good version: 2.1.318.

**Summary:** with the same config dir, working directory and arguments, 2.1.318 starts the stdio MCP server declared in a local plugin's `plugin.json`. 2.1.319 never starts it, and the `--debug` log has no line at all for that server. The plugin's commands still load. Another plugin's MCP server (`chrome-devtools-mcp`, `npx` command) starts on both versions.

## The plugin

Installed from a local marketplace (`agent-bus@agent-bus-local`). `installed_plugins.json` records `"version": "unknown"`, and the cache path ends in `/unknown`. Its `plugin.json`:

```json
{
  "name": "agent-bus",
  "mcpServers": {
    "agent-bus": {
      "command": "bun",
      "args": ["${CLAUDE_PLUGIN_ROOT}/channel.ts"],
      "env": {"AGENT_BUS_PUSH": "claude", "AGENT_BUS_RUNTIME": "claude"}
    }
  },
  "channels": [{"server": "agent-bus"}]
}
```

These features set it apart from the plugin that still works. Any of them may be the trigger:
- no `version`;
- `${CLAUDE_PLUGIN_ROOT}` in `args`;
- an `env` block;
- a `channels` entry.

## Reproduce

1. Run interactively with `DISABLE_AUTOUPDATER=1 <versions>/2.1.319 --debug-file <dir>/debug.txt` in a scratch directory.
2. Wait about 10 s, then quit.
3. Repeat with `<versions>/2.1.318`.

## Evidence (debug logs, same account and arguments)

| | 2.1.318 | 2.1.319 |
|---|---|---|
| `Loaded 2 commands from plugin agent-bus` | yes | yes |
| `MCP server "plugin:agent-bus:agent-bus": Starting connection …` | yes (line 284) | **absent** |
| any line naming `plugin:agent-bus:agent-bus` | 6 | **0** |
| `MCP server "plugin:chrome-devtools-mcp:chrome-devtools": Starting connection` | yes | yes |

On 2.1.318 the server is spawned. In this scratch probe it exits by design ("start this session with ab-claude"), which still proves Claude ran it. On 2.1.319 nothing is spawned and nothing is logged, so the session silently has no tools and no channel from this plugin.

## Impact

Sessions that rely on the plugin's channel stop receiving messages without any error. A session already running is affected after an auto-update once it reconnects its servers.

## Workaround

Pin 2.1.318 and set `DISABLE_AUTOUPDATER=1`.

Logs: `tmp/j105-c/debug.txt` (2.1.319) and `tmp/j105-e/debug.txt` (2.1.318); they contain no tokens. The upload, and any settings-key bisect, are still to do.
