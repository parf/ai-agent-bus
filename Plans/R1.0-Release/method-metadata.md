# Method metadata

Status: **built** in 0.9.32–0.9.34 (RT9.1 loader and runner-answered asks, RT9.2 bundled metadata, RT9.3 web display). The current contract is [agent self-description](../../docs/constitution.md#agent-self-description) and [package help](../../docs/runner/configuration.md#package-help); this page keeps the design discussion as history. MCP tool generation remains
[R1.2](../R1.2/TODO.md#dependencies). The record description remains the short
listing summary; the daemon carries asks and never interprets package metadata.

## Settled so far

Owner, 2026-10-19: the description lives with the agent's code and changes
with it; the daemon neither stores nor interprets it. MCP `tools/list` is too
heavy to be the source.

### Secrets and environment: `env.dist`

The package's existing `env.dist` (no leading dot, so `ls` shows it) is the
description of the agent's environment variables, as a commented
`.env.example` is elsewhere. Rules:

- the comment lines directly above a key describe it, and faces show them (web hints, `help`, docs);
- an empty value declares a required key, and a value is a non-secret default, as the runner already reads it;
- no `# secret` marker: the recommendation is that `env.dist` lists secrets and settings go in Configuration, but each agent's author decides.

```dotenv
# Bot token from @BotFather (/newbot). Required.
# See docs/agents/telegram.md
TELEGRAM_BOT_TOKEN=
```

### Methods and Configuration keys

Owner, 2026-10-19: fields, each with a description and an **optional** type, on
every method and on every argument; kept in the package's `agent.json`, next to
`env.dist`.

```json
{
  "description": "Telegram bot on the bus: post, read, reply routing",
  "license": "PolyForm-Noncommercial-1.0.0",
  "methods": {
    "post": {
      "help": "Send a message to a chat",
      "args": [
        { "name": "channel", "help": "#name, chat id or @user; the default chat if omitted", "optional": true },
        { "name": "format",  "type": ["text", "markdown"], "help": "How the text is rendered", "optional": true },
        { "name": "text",    "help": "Message text, to the end of the line", "rest": true }
      ],
      "returns": "chat and message id"
    },
    "restart": { "help": "Restart a unit", "args": [ { "name": "unit", "help": "Exact unit name" } ], "role": "operate", "destructive": true }
  },
  "config": {
    "default": { "type": "string", "help": "Chat used when a post names none", "example": "@parf" },
    "units":   { "type": "list", "help": "Units this instance may manage" }
  }
}
```

| Field | Meaning |
|---|---|
| `help` | the description, on a method, an argument and a Configuration key |
| `type` | optional; `string` when absent; one of `string`, `int`, `bool`, `list`, `json`; **an array is an enum**: the allowed values |
| `optional`, `rest` | may be left out; runs to the end of the line |
| `returns`, `destructive` | optional, per method |
| `role` | optional, one role or a nonempty array; any one grants the method. Owner and Maintainer always grant it; no inferred hierarchy |
| `example` | optional, per Configuration key |

The agent as a whole carries two more fields (owner, 2026-10-19):

| Field | Meaning |
|---|---|
| `description` | what the agent is for, in a sentence or two; already required in every `agent.json` and registered as the record's description |
| `license` | the agent's license as an SPDX identifier, e.g. `MIT` or `PolyForm-Noncommercial-1.0.0`; shown by `--help` |
| `version` | the agent's own version; a bundled agent leaves it out and shows the bus's shared version, since [every program shares one version](../../CLAUDE.md#versioning); a third-party agent sets its own |
| `author` | who wrote it: a name, optionally with an email |
| `homepage` | where its docs or code live, a URL |

A Configuration key the description does not name is warned about, not refused.

### How a caller gets it

| Ask | Answers |
|---|---|
| `--help` | the text description for a person or an AI, as today; generated from `agent.json`. It **must say** which of the extra asks below the agent answers |
| `--help-config` | the Configuration keys: name, type, help, example, default |
| `--help-secret` | the `env.dist` keys: names, their comments, which are required; never a value |
| `--help-json` | the whole description as data for programs such as the web face: the agent fields, methods, Configuration keys and the `env.dist` names, help and required flags; never a value. This is the special help query for machines; the three above stay text |

The daemon only carries the ask, as it does `--help` today. Who answers
(owner, 2026-10-19):

| The agent has | Who answers `--help`, `--help-config`, `--help-secret`, `--help-json` |
|---|---|
| a package with `agent.json` / `env.dist`, served by the runner or by `agent-bus run` | **the runner itself**, generated from those files, without running the script; the author writes no help code |
| only its own program (a standalone process with no package the runner reads) | the agent, which must answer **at least `--help`**; `--help-config`, `--help-secret` and `--help-json` are optional, and its `--help` says whether it answers them |

The new asks are `help-config`, `help-secret` and `help-json` on the wire, beside `help` and `mcp` ([agent self-description](../../docs/constitution.md#agent-self-description)), as the CLI flags are named (owner, 2026-10-19).

Still to decide: whether MCP `tools/list` is generated from this now or later.
