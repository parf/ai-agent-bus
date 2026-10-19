# MCP agents

📌 **TL;DR:** an agent can state its methods the way an MCP server states its
tools. Asked `--mcp`, it answers an MCP `tools/list` result. An agent that
answers one is tagged `mcp`, so a listing or a face finds every agent it can
call as tools. Proposed by the owner on 2026-10-17 for R1.1; not built.

## What an agent answers

The body `--mcp` asks for the agent's tools; `agent-bus call <to> --mcp` sends it, as `--help` is sent, and travels as [Q179](../done/R0.8-MVP/QUESTIONS.md#open-questions) decides. The answer is the `result` of an
MCP `tools/list`, under the MCP revision the [constitution](../../docs/constitution.md#external-protocols)
names: `{"tools": [...]}`. Each tool has `name`, `title`, `description` and
an `inputSchema`, a JSON Schema of its arguments:

```json
{"tools": [{
  "name": "search_properties",
  "title": "Search CRE Properties",
  "description": "Search commercial properties by location and filters. Returns up to `limit` matches sorted by relevance. Use get_property for full details on a single result.",
  "inputSchema": {"type": "object", "required": ["city"], "properties": {
    "city": {"type": "string", "description": "City name, e.g. 'Boston'"},
    "asset_type": {"type": "string", "enum": ["office", "retail", "industrial", "multifamily"]},
    "min_sqft": {"type": "integer", "minimum": 0},
    "limit": {"type": "integer", "default": 20, "maximum": 100}}}
}]}
```

The [self-description](../../docs/constitution.md#agent-self-description)
rules hold. `--help` is the text an AI reads, `--mcp` the schema a program
reads. Both cover only what the caller may do.

<details>
<summary>The tag, and what it changes</summary>

| | |
|---|---|
| **The `mcp` tag** | added automatically to an agent whose `--mcp` answer is a valid tools result with at least one tool — `{"tools": []}` is what an agent with none answers ([constitution](../../docs/constitution.md#agent-self-description)) — like the launchers' `claude` and the runner's `runner` ([tags](../../docs/03-records.md#tags)); who probes and when is [Q178](QUESTIONS.md#open-questions) |
| **Tags are not only the user's** | already true: `personal` and the launcher and runner labels are added by the bus. This adds one more label set by what an agent does, not by what someone typed |
| **Method metadata** | this is a candidate for R1's [method metadata](../R1.0-Release/method-metadata.md#method-metadata) representation (RT9). If accepted there, RT9 takes this shape and R1.1 keeps only the tag |

</details>
