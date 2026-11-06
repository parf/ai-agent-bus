# AI: ask a language model without sharing its password

📌 **TL;DR:**
- **Problem:** scripts and AI sessions need model access without each holding the account key.
- **Outcome:** everyone on the bus asks one shared door and gets the answer; the key stays on the server and the administrator decides who may ask.
- **Start:** see what the helper offers: `agent-bus help '#ai@node.example'` ([Try this first](#try-this-first)).

## What it does

- ✅ **Ask:** send a question or a full chat request, get the answer text or the provider's full reply (`ask`, or just your text). A text answer ends with its tokens and cost.
- ✅ **Decide:** send a question with fixed choices, get each choice with its probability (`decide`, see [typed decisions](#typed-decisions)).
- ✅ **Embed:** send a JSON text or list of texts, get one vector per text (`embed`, see [embeddings](#embeddings)).
- ✅ **Models:** list the provider's models as a table of context length, price per million tokens and input kinds, with your default marked (`models`; `models --images` gives image models and their price per image or megapixel), or see one model's context length, prices and input kinds for each provider that serves it (`model <id>`; for an image model, its parameters and prices).
- ✅ **Account:** see the remaining credit (`balance`) or the key's spend limit, remaining credit and rate limit, never the key itself (`key`). Only the Owner and Maintainers may ask.
- ✅ **Same as the provider:** every method goes to the configured provider and answers what it says. A provider without one of these methods gets `provider has no <method>`.
- 🚫 **Images:** `image` checks the request and answers `image generation is not available yet`; no provider call is made, since `#ai` has nowhere to store the pictures.
- 🚫 **Not for:** long conversations with memory (each call stands alone: send the context again), live word-by-word output (no streaming), or text that must never reach an outside AI provider (the provider sees every prompt).

## Typed decisions

- **What:** `decide` asks a decision model, such as Jev, a question with fixed answers and returns how likely each answer is.
- **Provider:** it uses the [OpenRouter Decisions API](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request).

```sh
agent-bus call '#ai@node.example' 'decide {"model":"typesafe/jev-1.14","state":"Tea","questions":{"drink":{"type":"choice","instructions":"Which drink?","criteria":{"tea":"Tea","coffee":"Coffee"}}}}'
```

## Embeddings

- **What:** `embed` sends texts to the provider's `/embeddings` and answers `{"model":...,"vectors":[[...],...]}`, one vector per text, in order.
- **Form:** `embed "one text"`, `embed ["first","second"]` or `embed {"input":[...],"model":"..."}`. Other text after `embed` is an ordinary prompt.
- **Model:** set `embed_model` in Configuration, or put `"model"` in the request object.
- **Size:** one answer, with its reply envelope, holds at most 1 MiB, so a large batch is refused as `N items × D dims > max M`. Split it and send again.
- **Cost:** the agent learns a model's vector size from its first answer. So the first over-limit request to a new model costs one provider call; later ones are refused before any call.

```sh
agent-bus call '#ai@node.example' 'embed ["first text","second text"]'
```

## You'll need

- An account with an AI provider that speaks the common OpenAI format, such as OpenRouter: its key and a model name.
- A bus runner with Python 3 and network access.

## Try this first

Use the complete name of your [enabled instance](#installing-it) in the commands below.

- See what the helper does, its settings and the key's name (never the key):

```sh
agent-bus help '#ai@node.example'
```

<details>
<summary>More help forms</summary>

- `agent-bus help '#ai@node.example' config` and `... secret` show one part.
- Scripts can use `call ... --help-config`, `--help-secret` or `--help-json`.

</details>

## Use examples

- **Ask one question:** `agent-bus call '#ai@node.example' --wait=60s 'Explain a database backup in one sentence.'`
- **Send a full chat request:** `agent-bus call '#ai@node.example' --wait=60s '{"messages":[{"role":"user","content":"Say hello in French."}],"temperature":0.2}'`
- **Use another model once:** add `"model":"openai/gpt-4.1-mini"` to the JSON.
- **Pick a model:** `agent-bus call '#ai@node.example' 'models claude'` (or `models free`), then `agent-bus call '#ai@node.example' 'model openai/gpt-4.1-mini'` for its prices per provider.
- **Find an image model:** `agent-bus call '#ai@node.example' 'models --images'`, then `model <id>` for its parameters and prices.
- **Embed texts:** `agent-bus call '#ai@node.example' 'embed ["first text","second text"]'`.
- **Check the account:** `agent-bus call '#ai@node.example' balance` or `... key` (Owner and Maintainers).
- **Get probabilities for fixed choices:** a `decide` call, [above](#typed-decisions).
- **Ask from my AI session:** say "Ask the AI helper to explain this error."

## Installing it

1. Use the AI package included with the bus installation.
2. Put `OPENROUTER_API_KEY` in the instance's private [host environment file](../runner/configuration.md#environment-files), owned by the runner account with mode 0600.
3. Set `enabled:true` for `ai` in the [runner inventory](../runner/configuration.md#inventory), preserving other entries, then reload the runner.
4. Set the default model: `agent-bus config '#ai@node.example' '{"model":"openai/gpt-4.1-mini"}'`.
5. Grant access only to callers who may spend the provider balance.

## Configuration

- `model` is the default chat model when a call names none, for example `openai/gpt-4.1-mini`; it is not used by `decide`. `models` marks it.
- `embed_model` is the default embedding model for `embed`, for example `openai/text-embedding-3-small`.
- `image_model` is the default image model, for example `openai/gpt-image-2`; `models --images` marks it.
- `base_url` is the OpenAI-compatible API root; default: `https://openrouter.ai/api/v1`. Only a trusted administrator should change it: it receives the provider key. Redirects are not followed.

## Secret keys

- **`OPENROUTER_API_KEY`,** only in the runner's private host file.
- **Never** in Configuration, bus Secret, a call body or a command argument.

## Who may use it

- **Only callers on the agent's allowed list;** it alone decides who spends the provider balance.
- **`balance` and `key` are for the Owner and Maintainers only.** Anyone else gets `<method> needs the owner or maintainer role`, and no provider call is made.
- **No other roles or per-caller limits.**
- **The provider sees each prompt** and may keep it under its own policy.

<details>
<summary>Narrowing access and limits</summary>

The list is stored by the daemon and survives a runner reload. To narrow an existing `*` grant to the owner and their agents:

```sh
agent-bus manage '#ai@node.example' --add-to-set-allow '@owner'
agent-bus manage '#ai@node.example' --remove-allow '*'
```

- The caller's wait bounds a request, up to the package's 60-second timeout.
- Requests and answers are at most 1 MiB each.

</details>

## Troubleshooting

| If you see… | Do this |
|---|---|
| `needs OPENROUTER_API_KEY`, or `OPENROUTER_API_KEY is not set` when run directly | Ask the host administrator to set the host file, then reload |
| `the provider refused the configured key (401: …)` or `(403: …)` | Ask the administrator to check `OPENROUTER_API_KEY` and the provider account |
| `the provider refused for lack of credit (402: …)` | Ask the administrator to add credit |
| A bus permission refusal | Ask the owner to add you to the allowed list |
| `set Configuration model or provide a request model` | Set a default model or put `model` in the JSON; no provider call happens until then |
| Invalid request or Configuration | Check the model, API root and JSON; do not request streaming |
| Provider request timed out | Shorten the request or wait longer, at most 60 seconds |
| Answer exceeds the size bound | Ask the model for a shorter answer |
| `provider has no <method>` | The configured provider lacks that method; use OpenRouter, or skip it |
| `N items × D dims > max M` | Send at most M texts per `embed` call |
| `balance needs the owner or maintainer role` | Ask the Owner; only the Owner and Maintainers see the account |

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#ai` reports these.

| Level | When | What to do |
|---|---|---|
| alert | The provider refuses the key or the account (401, 402 or 403) | Ask the administrator to check `OPENROUTER_API_KEY` and the provider account |
| alert | The provider cannot be reached (`provider connection failed`) | Check the runner's network and `base_url` |
| warning | The provider rate-limits a call (429) or fails it (5xx) | Wait; if it repeats, check the provider's status and quota |
| warning | A provider call timed out | Shorten requests; repeated timeouts point at the provider |
| warning | A call names no model and Configuration has none | Set `model` in Configuration |

## Technical details

<details>
<summary>Request forms</summary>

- `ask` accepts a bare prompt, `ask <prompt>`, `{"method":"ask","prompt":"Hello"}`, `{"method":"ask","prompt":{"messages":[...]}}` or a chat-completions JSON object.
- Prompt forms return assistant text; chat-body forms return the provider's JSON, including choices and usage.
- A JSON request may set `"model"` for that request only; streaming is not supported.
- A text answer ends with `[tokens <in> in, <out> out; cost <n> USD]` when the provider reports usage.
- `models [--images] [--json] [<word>]`, `balance` and `key` are the whole body; `models <word>` is always a filter, never a prompt: it keeps rows whose ID or name contains the word, or only free models for `free` (a price of exactly zero, or a `:free` ID); a price the provider gives as NaN or infinite shows as `-`, an image model shows its cheapest price per image, else per megapixel, and control characters in a cell are shown escaped; `--json` returns the provider's full records instead of the table, and the table's last line names the default (`default: unset`, or `(not listed)` when the listing lacks it); a configured variant such as `:nitro` marks its base model, and a canonical slug marks its model; `model <id>` takes one model ID; `embed` takes a JSON string, list or `{"input":...}` object, and `image` a JSON object; other text after any of them is a prompt. JSON forms: `{"method":"models","images":false,"json":false,"filter":"..."}` (each field optional), `{"method":"model","id":"..."}`, `{"method":"embed","input":...,"model":"..."}`.

</details>

<details>
<summary>Lookups and embeddings</summary>

- `models` reads `GET /models` (`--images`: `GET /images/models`), `model <id>` reads `GET /models/<id>/endpoints` and, when that is a 404, `GET /images/models/<id>/endpoints`, `balance` reads `GET /credits`, `key` reads `GET /key`, and `embed` posts to `/embeddings`, each under the configured `base_url`.
- A 401 or 403 from any method names the refused key, and a 402 the missing credit, with the provider's text masked.
- `image` is for the Owner and Maintainers. Like `decide`, only a JSON object is a request: `image {"prompt":"..."}` or `{"method":"image","prompt":"..."}`; "image recognition is…" stays an ordinary prompt for every caller. It takes a prompt, `model`, `n` from 1 to 4, `aspect_ratio`, `resolution`, `size`, `output_format` (png, jpeg, webp, svg or gif), `quality`, `background`, `seed`, and up to 16 `input_references` as https or `data:image/...;base64,` URLs, within 1 MiB. A request that passes these checks is not yet sent.
- `model <id>` lists every endpoint with its own context length and prices; they are never merged.
- `key` answers only the limit fields; the provider's key label is dropped, since it repeats part of the key.
- `embed` allows at most floor((1048576 − O) / (25·D + 2)) texts for a D-dimension model. 1048576 bytes is the most the daemon reads of the answer envelope the runner sends back. O is that envelope around an empty answer: the reply route, its topic and tag, and the body's escaping. Each number takes at most 24 characters plus a comma. A batch over the cap gets only the refusal, never a cut answer. Known vector sizes are kept in the agent's work directory, at most 256 models.
- `model <id>` takes `author/slug` or `author/slug:variant`; an empty, `.` or `..` part is refused before any call.

</details>

<details>
<summary><code>decide</code></summary>

- Only text after `decide ` that starts with `{` is a decision; "decide whether tea beats coffee" stays an ordinary prompt.
- The JSON method form is exactly `{"method":"decide","body":"<raw JSON string>"}`; a non-string `body` or extra method fields are refused.
- Put the decision model in the body; the chat default is never added, and a missing model is left to the provider to refuse.
- The raw string is validated as a non-streaming JSON object, then sent byte for byte, including whitespace and duplicate keys.
- The full provider JSON comes back, including every Choice probability; values are equal after JSON parsing, decimal spelling and arbitrary precision are not preserved, and non-standard NaN and Infinity are refused.
- `decide` uses the scheme, host and port of the trusted `base_url`, discards its path and posts to `/api/alpha/decisions`; `ask` appends `/chat/completions` to the configured base path.
- Every method spends the same key under the same ACL, with the same limits, deadline, masking, no-redirect and no-retry rules.
- Logs hold the caller, a bounded model ID (or `invalid`), byte sizes and status, never the request, response or key.

</details>
