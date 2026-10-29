# Describe an agent clearly

📌 **TL;DR:** Help should let a caller choose a method and make a correct call
without reading the implementation. Author it once in `agent.json` and
`env.dist`; generated text stays short, while `help-json` retains the full data.

Starting a package? Use the [create-agent prompt](create-agent-prompt.md#prompt-to-paste), then check the prose below.

## Checklist: writing an agent's description and help

The description appears beside the name in `ls`, the web list, the MCP catalogue
and help, so every AI caller pays for it on every listing. Start with a verb, say
what it does and any notable limit, and stay under 80 characters. Omit the name,
kind, "bus", "agent", project and repository (`homepage` links the repository).

| Bad | Good |
|---|---|
| telegram: puts a Telegram bot on the bus — post, read, subscriptions | Post and read chats; route your replies to the right session |
| systemd: manages services on the bus | Inspect units and logs; restart units within configured scope |
| server-info: server facts agent | Read host facts: network, memory, disks, load and TLS certificates |

| Check | Why |
|---|---|
| Method `help` starts with a verb, fits one line and says what the call returns | A caller needs the outcome, not the implementation |
| Drop a repeated verb when every method is a read | Twenty identical "Show" prefixes add nothing |
| Put common arguments in top-level `arguments`; reference them by `name` in each method | Repeating a paragraph hides the methods |
| Give arguments types, enum choices, defaults and limits | Callers should not learn accepted values by failing |
| Use method `text` for the text syntax; mark JSON-only arguments `json_only` | A JSON field is not necessarily a positional argument |
| Declare `body_formats` (`["text"]` by default); add `json` only when the handler accepts `{ "method": "name", ...args }` | Machine help describes accepted call bodies |
| Add 2–4 method `example` calls with real, copyable values | Placeholders do not prove the grammar works |
| Put common methods first in the manifest | Generated help follows the author's order |
| Declare `role` and `destructive`; keep enforcing them in the handler | Callers see permissions and destructive effects before calling |
| Dispatch every nonempty ask first, through the language metadata helper; unsupported asks reply `unsupported: <ask>`; only an empty ask reaches normal work | An ask must never run an ordinary operation |
| Set `"asks":"none"` for a script unaware of asks (`agent-bus run --no-asks` without a package) | Metadata answers what it can; other asks never start the script |
| Set `"help":"native"` and answer all four help asks when help depends on runtime Configuration | Generic metadata cannot apply configured caps or scope |
| Give each Configuration key `help` and an `example` | Operators need to know what a setting changes |
| In `env.dist` comments, say what a key is and where to get it; mark required keys with a trailing `# required`; unmarked keys are optional, and either kind may have a default | Secret help explains setup without revealing a value |
| Leave credentials, internal paths and obvious words out of help | Help is public to permitted callers |

Text help omits `help` itself from the method list, groups shared arguments and
examples, and ends with version/license/homepage links; full identity fields and
JSON-only arguments stay in `help-json`. See [package help](../runner/configuration.md#package-help)
for validation and [Secret handling](../runner/security.md#secrets) for the value boundary.

## Implementation recommendations

| Recommendation | Why |
|---|---|
| Start per-message unless retained state or connections require otherwise | Less lifecycle code to own |
| Use existing services and the smallest dependency and privilege set | No undocumented authority paths |
| Include a refusal; exercise destructive examples only in disposable fixtures | Prove scope without risking shared resources |
| Keep application errors consistent and credentials out of answers and logs | Each agent owns the secrecy of its replies |
