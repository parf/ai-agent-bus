# Voyage: meaning fingerprints for your texts

📌 **TL;DR:** Turns a text into a "meaning fingerprint" (an *embedding*: numbers
that put similar texts close together), and can re-order search results by
relevance. It uses the Voyage AI service; its key stays on the server, and the
owner decides who may use it.

## Why you'd want it

**The problem.** Finding texts by meaning, not exact words, needs an embedding
service, and that needs a paid account key. Handing the key to every app and AI
session spreads it around and hides who spends the money.

**The idea.** The key stays in one locked place on the server. Callers get a
door, not the key; the owner's access list decides who may spend the balance.

**When to use it**

- You build search by meaning, or an app such as [RAG](rag.md#what-it-does).
- You want the best few results from a list put first (*reranking*).
- Several people or apps should share one Voyage account.

**When not to:** exact word or id lookups (a database is cheaper), or text that
must never leave your servers (Voyage receives it).

**Possible use cases**

| Use case | What goes in | What you get |
|---|---|---|
| Search notes by meaning | Each note, then the question | Fingerprints to compare |
| Rerank search hits | A question and 20 candidate answers | The candidates ordered by relevance |
| Find duplicate tickets | Two tickets' texts | Fingerprints close together when they match |
| Power a RAG app | `#rag` calls it for every stored text | Stored fingerprints |

## What it does

| Call | Does |
|---|---|
| `embed` | Turns a text into a meaning fingerprint |
| `request` | Sends any other Voyage request, such as rerank, and returns the answer |

Each call makes exactly one request to Voyage, never retried.

<details><summary>Technical details: requests and limits</summary>

- One POST per call to the fixed origin `https://api.voyageai.com`, with verified TLS, returning the provider's JSON (embeddings and usage for `embed`).
- A JSON `embed` may set `model` and `input_type` (`query`, `document` or null). Model IDs contain at most 128 letters, digits, `.`, `_`, `/` or `-`. To embed text starting with `{`, prefix it with `embed `; unprefixed `{` starts a JSON call.
- `request <path> <JSON>` sends the JSON bytes unchanged. JSON form: `{"method":"request","path":"/v1/rerank","body":"<raw JSON>"}`; `body` is a string so its bytes survive.
- Paths start `/v1/` and contain only slash-separated API names (letters, digits, `_`, `-`). URLs, queries, encoded paths and traversal are refused.
- No retries, redirects or streaming. Request and answer limits are 1 MiB. The whole request, including DNS and a trickling response, expires before the message deadline, capped at 60 seconds.
- Provider echoes of the key are masked in JSON keys, values and text. Logs hold only the model, byte counts and status; never prompts, answers or the key.

</details>

## You'll need

- A Voyage API key.
- A [person's runner](../07-setup.md#a-persons-runner) with Python 3.
- The agent's owner to let you call it.

## Try this first

```sh
agent-bus help '#voyage@node.example'
```

Help works before a key is set; real calls need it.

## Use examples

| I want to… | Command |
|---|---|
| Fingerprint a document | `agent-bus call '#voyage@node.example' 'embed A document about tea'` |
| Fingerprint a search question | `agent-bus call '#voyage@node.example' '{"method":"embed","text":"tea","input_type":"query"}'` |
| Rerank search hits | `agent-bus call '#voyage@node.example' 'request /v1/rerank {"model":"rerank-2.5","query":"tea","documents":["tea","coffee"]}'` |
| Let RAG use it | grant `#rag` on this agent's access list ([RAG installing](rag.md#installing-it)) |

## Installing it

An administrator installs the package and puts the key in the runner's private file.

<details><summary>Step by step</summary>

Build with `src/agents/voyage/package.sh <agent.d/voyage>`, then enable it in the
[runner inventory](../runner/configuration.md#inventory). It uses Python's
standard library; its network sandbox allows only `api.voyageai.com`.

</details>

## Configuration

Only the default model.

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `model` | Default embedding model, default `voyage-3.5` |

</details>

## Secret keys

`VOYAGE_API_KEY`, required, only in the private host `voyage/env` file
([environment files](../runner/configuration.md#environment-files)). Never in
Configuration or bus Secret.

## Who may use it

Only the owner at first. Grant others explicitly: every caller spends the
provider balance.

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#voyage` reports these. The page that shows them is in work.

| Level | When | What to do |
|---|---|---|
| alert | `VOYAGE_API_KEY` is missing, or Voyage rejects it (401 or 403) | Set the key in the host `voyage/env`, then reload the runner |
| alert | Voyage cannot be reached | Check the runner's network |
| warning | Voyage rate-limits a call (429) or fails it (5xx) | Wait; check the quota |
| warning | A call timed out: its outcome is unknown | Do not repeat it automatically |

## Troubleshooting

| If you see… | Do this |
|---|---|
| `error: <status> <provider message>` (including 429) | The provider refused; check the key, quota or request |
| A timeout | The outcome is uncertain; do not repeat it automatically |
