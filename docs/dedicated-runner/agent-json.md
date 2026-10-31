# The agent JSON of `agent-bus run`

📌 **TL;DR:** `agent-bus run -` reads one JSON object from stdin that describes
the agent to serve: its name and script, plus optional settings that match
`run`'s per-agent flags. `--stay-registered` is a lifecycle flag only; it has no JSON field. Only `name` and `script` are required. An unknown
field is refused by name, and a flag given on the command line wins over the
same field. This is the [dedicated runner](../06-dedicated-runner.md#script-agents)'s
form. The [runner](../runner/README.md)'s installed `agent.json` is a different file
([runner configuration](../runner/configuration.md)).

## An example

```json
{
  "name": "#hello@srv1",
  "script": "/home/you/bin/hello.sh",
  "descr": "greets you",
  "instances": 2,
  "allow": ["@owner"],
  "tags": ["demo"]
}
```

```sh
agent-bus run - < hello.json        # serve it
agent-bus run -5 - < hello.json     # the same, five at a time
```

## Fields

| Field | Type | Default | Meaning | Flag |
|---|---|---|---|---|
| `name` | string | required | the 👾 agent's name; a missing leading `#` is added | first argument |
| `script` | string | required | one shell command line; an absolute path, or a bare command found on `PATH` (a relative path is refused) | second argument |
| `algo` | `"json"` or `"args"` | `"args"` | how a message reaches the script: the envelope as JSON on stdin, or the body as `$1` ([script agents](../06-dedicated-runner.md#script-agents)) | `--algo` |
| `no_asks` | boolean | false | answer unsupported for asks metadata cannot answer, without starting the script | `--no-asks` |
| `package_dir` | string | none | absolute directory containing `agent.json` and optional `env.dist`; the dedicated runner answers help asks from [package metadata](../runner/configuration.md#package-help) | `--package-dir` |
| `descr` | string | none | what `agent-bus ls` and the MCP catalogue show | `--descr` |
| `instances` | number | `1` | at most this many script processes at once | `-N` |
| `allow` | array of names | the [restricted default](../02-access.md#acl) | who may send to it, e.g. `["@owner"]` or `["*"]` | `--allow` |
| `personal` | boolean | `false` | register the record as [Personal](../03-records.md#personal-and-shared) | `--personal` |
| `sandbox` | `"on"` or `"off"` | off | apply the [dedicated isolation rules](../runner/security.md#isolation-today) | `--sandbox` |
| `network` | boolean | `false` | a sandboxed script that needs the network says so | `--network` |
| `roles` | array of strings | none | the [roles](../03-records-agent.md#roles) the script understands, registered on the agent, informational | `--roles` |
| `tags` | array of strings | none | explicit [record labels](../03-records.md#tags); `run` adds none of its own | `--tags` |

## Rules

| Rule | |
|---|---|
| Read only with `-` | stdin is read only when `run` is given `-`; JSON piped without it is refused |
| Strict | a field not in the table above is refused by name, so a typo cannot pass |
| Flags win | `--algo`, `--descr`, `-N`, `--sandbox`, `--allow`, `--personal`, `--network`, `--tags`, `--roles` and `--package-dir` given beside `-` replace the JSON's value |
| One object | one agent per `run`; serve several with several `run`s |
