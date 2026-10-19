# Method metadata

Status: proposed, not built. The MVP carries an agent's method information in
its [description](../../docs/03-records-agent.md#agent-templates) —
one sentence, written by whoever registered it, and the only thing a listing or
the MCP catalog can show. That is enough to recognise an agent and not enough
to call one: a caller cannot learn a verb's arguments from it, nothing marks the
verbs that destroy something, and a face has nothing to generate documentation
from.

R1 replaces it with something **stated**, the same shape as
[where a member says it is](runner-implementation.md#where-a-member-says-it-is):
supplied by the registrant, kept raw, and not a value the daemon interprets or
routes on. What it has to answer:

| | |
|---|---|
| **which verbs there are** | an agent's callable methods, each described on its own, so a catalog lists them instead of one sentence about the whole agent |
| **which of them destroy something** | the hint a face needs before it calls one on a person's behalf |
| **generated, not maintained twice** | the faces build their catalog from what the agent stated, rather than from prose somebody keeps in step by hand |

The representation, and whether the daemon checks its shape at all, is release
design work after scope confirmation. One candidate, proposed by the owner on
2026-10-17: an MCP `tools/list` result, answered to the body `--mcp`
([MCP agents](../R1.1/mcp-agents.md#mcp-agents)). The description field stays either way: an
agent that states nothing keeps exactly today's behavior.

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
| `returns`, `role`, `destructive` | optional, per method |
| `example` | optional, per Configuration key |

The agent as a whole carries two more fields (owner, 2026-10-19):

| Field | Meaning |
|---|---|
| `description` | what the agent is for, in a sentence or two; already required in every `agent.json` and registered as the record's description |
| `license` | the agent's license as an SPDX identifier, e.g. `MIT` or `PolyForm-Noncommercial-1.0.0`; shown by `--help` |

A Configuration key the description does not name is warned about, not refused.

### How a caller gets it

| Ask | Answers |
|---|---|
| `--help` | the text description for a person or an AI, as today; generated from `agent.json`. It **must say** which of the two below the agent answers |
| the same help ask in the agent's own format | an agent that speaks JSON or MessagePack answers the description in that format, as data |
| `--help-config` | the Configuration keys: name, type, help, example, default |
| `--help-secret` | the `env.dist` keys: names, their comments, which are required; never a value |

The daemon only carries the ask, as it does `--help` today. Who answers
(owner, 2026-10-19):

| The agent has | Who answers `--help`, `--help-config`, `--help-secret` |
|---|---|
| a package with `agent.json` / `env.dist`, served by the runner or by `agent-bus run` | **the runner itself**, generated from those files, without running the script; the author writes no help code |
| only its own program (a standalone process with no package the runner reads) | the agent, which must answer **at least `--help`**; `--help-config` and `--help-secret` are optional, and its `--help` says whether it answers them |

Still to decide: the wire names of the two new asks, alongside `help` and `mcp` ([agent self-description](../../docs/constitution.md#agent-self-description)); whether MCP `tools/list` is generated from this now or later.
