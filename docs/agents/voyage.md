# Voyage: embeddings through the bus

📌 **TL;DR:** Embed text or send a JSON request to a Voyage endpoint without
sharing the provider key with callers. The runner keeps the key in its private
host environment; access and spending are controlled by this agent's ACL.

## Calls

```sh
agent-bus call '#voyage@node.example' 'embed A document about tea'
agent-bus call '#voyage@node.example' '{"method":"embed","text":"tea","input_type":"query"}'
agent-bus call '#voyage@node.example' 'request /v1/rerank {"model":"rerank-2.5","query":"tea","documents":["tea","coffee"]}'
```

`embed` uses Configuration's `model`, default `voyage-3.5`; a JSON call may
override `model` and set `input_type` to `query`, `document` or null.
Model IDs contain at most 128 letters, digits, `.`, `_`, `/` or `-`.
It returns the provider JSON, including embeddings and usage. To embed text
starting with `{`, prefix it with `embed `; unprefixed `{` starts a JSON call.

`request <path> <JSON>` sends the JSON bytes unchanged. The equivalent JSON
method is `{"method":"request","path":"/v1/rerank","body":"<raw JSON>"}`;
`body` is a string so formatting and bytes survive. Paths must start `/v1/`
and contain only slash-separated API names (letters, digits, `_`, `-`).
URLs, queries, encoded paths and traversal are refused. The origin is fixed to
`https://api.voyageai.com`, with verified TLS certificates.

## Install and configure

Build with `src/agents/voyage/package.sh <agent.d/voyage>`, then enable the
package in the [runner inventory](../runner/configuration.md#inventory).
Put `VOYAGE_API_KEY` in the private host `voyage/env` file; it is declared
required in `env.dist`. Never put the key in Configuration or bus Secret.
The package uses Python's standard library and the runner's network sandbox,
allowing only `api.voyageai.com`. Start with the normal owner-only ACL and
grant other callers explicitly.

Configuration accepts only `model`. Help asks work without a configured key.

## Safety and errors

There is one POST per call, no retries or redirects, and no streaming.
Request and answer limits are 1 MiB. The whole request, including DNS and a
trickling response, expires before the message deadline, with a 60-second cap.
A timeout is uncertain: do not automatically replay it.

Provider errors, including 429, return `error: <status> <provider message>`.
Provider echoes of the credential are masked in JSON keys, values and text.
Logs contain only the model, byte counts and status; never prompts, answers
or the credential. The key belongs only to the host environment.
