# Voyage: embeddings through the bus

📌 **TL;DR:** Embed text or send a JSON request to a Voyage endpoint without
sharing the provider key with callers. The runner keeps the key in its private
host environment; access and spending are controlled by this agent's ACL.

## Why you'd want it

Several people and AI sessions can use one Voyage account for embeddings and
reranking, while only the runner host holds the key. Who may spend the balance
is one access list on the bus.

## What it does

- `embed` turns text into vectors with Voyage's embeddings API.
- `request` POSTs an unchanged JSON body to any Voyage `/v1/` endpoint, such as rerank.
- It makes one POST per call to the fixed origin `https://api.voyageai.com`,
  with verified TLS certificates, and returns the provider's JSON.

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner) with Python 3.
- A Voyage API key, kept in a private host file.
- The agent's owner to grant callers.

## Try this first

```sh
agent-bus help '#voyage@node.example'
agent-bus call '#voyage@node.example' 'embed A document about tea'
```

Help asks work without a configured key; the call needs it.

## Use examples

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
URLs, queries, encoded paths and traversal are refused.

## Installing it

Build with `src/agents/voyage/package.sh <agent.d/voyage>`, then enable the
package in the [runner inventory](../runner/configuration.md#inventory).
The package uses Python's standard library and the runner's network sandbox,
allowing only `api.voyageai.com`.

## Configuration

Configuration accepts only `model`, the default embedding model (`voyage-3.5`).

## Secret keys

Put `VOYAGE_API_KEY` in the private host `voyage/env` file; it is declared
required in `env.dist`. Never put the key in Configuration or bus Secret.
The key belongs only to the host environment.

## Who may use it

Start with the normal owner-only ACL and grant other callers explicitly: every
caller spends the provider balance.

## Troubleshooting

There is one POST per call, no retries or redirects, and no streaming.
Request and answer limits are 1 MiB. The whole request, including DNS and a
trickling response, expires before the message deadline, with a 60-second cap.
A timeout is uncertain: do not automatically replay it.

Provider errors, including 429, return `error: <status> <provider message>`.
Provider echoes of the credential are masked in JSON keys, values and text.
Logs contain only the model, byte counts and status; never prompts, answers
or the credential.
