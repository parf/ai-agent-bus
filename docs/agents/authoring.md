# Describe an agent clearly

📌 **TL;DR:** Help should let a caller choose a method and make a correct call
without reading the implementation. Author it once in `agent.json` and
`env.dist`; generated text stays short, while `help-json` retains the full data.

## Checklist: writing an agent's description and help

The description appears beside the name in `ls`, the web list, the MCP catalogue
and help. Repeating the name or kind makes people read it twice and costs every
AI caller tokens on every listing, while the useful purpose goes unsaid.
Start with a verb, say what it does and any notable limit or scope, and aim for
under 80 characters. Omit the name, kind, “bus”, “agent”, project and repository;
`homepage` already supplies the repository link.

| Bad | Good |
|---|---|
| telegram: puts a Telegram bot on the bus — post, read, subscriptions | Post and read chats; route your replies to the right session |
| systemd: manages services on the bus | Inspect units and logs; restart units within configured scope |
| server-info: server facts agent | Read host facts: network, memory, disks, load and TLS certificates |

| Check | Why |
|---|---|
| Method `help` starts with a verb, fits one line and says what the call returns | A caller needs the outcome, not the implementation |
| Drop a repeated verb when every method is a read | Twenty identical “Show” prefixes add no information |
| Put common arguments in top-level `arguments`; reference them by `name` in each method | Repeating the same paragraph hides the methods |
| Give arguments types, enum choices, defaults and limits | Callers should not discover the accepted values by failing |
| Use method `text` for the actual text syntax; mark JSON-only arguments `json_only` | A JSON field is not necessarily a positional text argument |
| Add 2–4 method `example` calls with real values that can be copied | Abstract placeholders do not prove that the grammar is usable |
| Put common methods first in the manifest | Generated help follows the author's order |
| Declare `role` and `destructive`; keep enforcing them in the handler | Callers must see permission requirements and destructive effects before calling |
| Give each Configuration key `help` and an `example` | Operators need to know what the setting changes |
| In `env.dist` comments, explain what a key is and where to obtain it; an empty declaration means required | Secret help must explain setup without revealing a value |
| Leave credentials, internal paths and obvious words out of help | Help is public to permitted callers, and incidental details obscure the contract |

Text help leaves `help` itself out of the method list, groups shared arguments
and examples, and ends with compact version/license/homepage and further-help
links. Full identity fields and JSON-only arguments remain in `help-json`.
See [package help](../runner/configuration.md#package-help) for validation and
[Secret handling](../runner/security.md#secrets) for the value boundary.
