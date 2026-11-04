# Voyage: meaning fingerprints for your texts

📌 **TL;DR:**
- **Problem:** finding texts by meaning, not exact words, needs an embedding service and a paid account key; handing the key to every app and AI session spreads it around and hides who spends the money.
- **Outcome:** callers get a "meaning fingerprint" for any text, and can re-order search results by relevance; the key stays on the server and the owner decides who may use it.
- **Start:** see what the helper offers: `agent-bus help '#voyage@node.example'` ([Try this first](#try-this-first)).

## What it does

- ✅ **Fingerprint a text:** numbers that put similar texts close together, called an embedding (`embed`).
- ✅ **Rerank:** put the best few results of a list first; any other Voyage request works the same way (`request`).
- ✅ **Search notes by meaning:** fingerprint each note, then the question, and compare.
- ✅ **Find duplicate tickets:** two matching texts give close fingerprints.
- ✅ **Power a RAG app:** [`#rag`](rag.md#what-it-does) calls it for every stored text.
- ✅ **One request per call:** exactly one request to Voyage, never retried.
- 🚫 **Not for:** exact word or id lookups (a database is cheaper), or text that must never leave your servers (Voyage receives it).

## You'll need

- A Voyage API key.
- A [person's runner](../runner/setup.md#a-persons-runner) with Python 3.
- The agent's owner to let you call it.

## Try this first

Use the complete name of your [enabled instance](#installing-it) in the commands below.

- Ask the helper what it can do (help works before a key is set; real calls need it):

```sh
agent-bus help '#voyage@node.example'
```

## Use examples

- **Fingerprint a document:** ask for `embed` and the text.
- **Fingerprint a search question:** send `{"method":"embed","text":"...","input_type":"query"}`.
- **Rerank search hits:** ask for `request /v1/rerank` with a JSON body of the query and documents.
- **Let RAG use it:** grant `#rag` on this agent's access list ([RAG installing](rag.md#installing-it)).

## Installing it

- Voyage is included in the installed agent bundle; no build or package copy is needed.
- Put `VOYAGE_API_KEY` in this instance's [private host env file](../runner/configuration.md#environment-files), owned by the runner account with mode 0600.
- Enable `voyage` (or a named instance) in the [runner inventory](../runner/configuration.md#inventory), preserving other entries, then reload the runner.
- Ask for help to check the instance; an embedding request uses the provider account and its balance.

## Configuration

`model` selects the default embedding model; its default is `voyage-3.5`.

## Secret keys

- **`VOYAGE_API_KEY`,** required, only in the private host `voyage/env` file ([environment files](../runner/configuration.md#environment-files)).
- **Never** in Configuration or bus Secret.

## Who may use it

- **Only the owner at first.**
- **Grant others explicitly:** every caller spends the provider balance.

## Troubleshooting

| If you see… | Do this |
|---|---|
| `error: <status> <provider message>` (including 429) | The provider refused; check the key, quota or request |
| A timeout | The outcome is uncertain; do not repeat it automatically |

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#voyage` reports these.

| Level | When | What to do |
|---|---|---|
| alert | `VOYAGE_API_KEY` is missing, or Voyage rejects it (401 or 403) | Set the key in the host `voyage/env`, then reload the runner |
| alert | Voyage cannot be reached | Check the runner's network |
| warning | Voyage rate-limits a call (429) or fails it (5xx) | Wait; check the quota |
| warning | A call timed out: its outcome is unknown | Do not repeat it automatically |

## Technical details

<details>
<summary>Requests and limits</summary>

- One POST per call to the fixed origin `https://api.voyageai.com`, with verified TLS, returning the provider's JSON (embeddings and usage for `embed`).
- A JSON `embed` may set `model` and `input_type` (`query`, `document` or null); model IDs contain at most 128 letters, digits, `.`, `_`, `/` or `-`.
- To embed text starting with `{`, prefix it with `embed `; an unprefixed `{` starts a JSON call.
- `request <path> <JSON>` sends the JSON bytes unchanged; the JSON form is `{"method":"request","path":"/v1/rerank","body":"<raw JSON>"}`, where `body` is a string so its bytes survive.
- Paths start `/v1/` and contain only slash-separated API names (letters, digits, `_`, `-`); URLs, queries, encoded paths and traversal are refused.
- No retries, redirects or streaming; request and answer limits are 1 MiB; the whole request, including DNS and a trickling response, expires before the message deadline, capped at 60 seconds.
- Provider echoes of the key are masked in JSON keys, values and text; logs hold only the model, byte counts and status, never prompts, answers or the key.

</details>
