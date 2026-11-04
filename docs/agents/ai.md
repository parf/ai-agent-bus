# AI: ask a language model without sharing its password

📌 **TL;DR:**
- **Problem:** scripts and AI sessions need model access without each holding the account key.
- **Outcome:** everyone on the bus asks one shared door and gets the answer; the key stays on the server and the administrator decides who may ask.
- **Start:** see what the helper offers: `agent-bus help '#ai@node.example'` ([Try this first](#try-this-first)).

## What it does

- ✅ **Ask:** send a question or a full chat request, get the answer text or the provider's full reply (`ask`, or just your text).
- ✅ **Decide:** send a question with fixed choices, get each choice with its probability (`decide`, see [typed decisions](#typed-decisions)).
- 🚫 **Not for:** long conversations with memory (each call stands alone: send the context again), live word-by-word output (no streaming), or text that must never reach an outside AI provider (the provider sees every prompt).

## Typed decisions

- **What:** `decide` asks a decision model, such as Jev, a question with fixed answers and returns how likely each answer is.
- **Provider:** it uses the [OpenRouter Decisions API](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request).

```sh
agent-bus call '#ai@node.example' 'decide {"model":"typesafe/jev-1.14","state":"Tea","questions":{"drink":{"type":"choice","instructions":"Which drink?","criteria":{"tea":"Tea","coffee":"Coffee"}}}}'
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
- **Get probabilities for fixed choices:** a `decide` call, [above](#typed-decisions).
- **Ask from my AI session:** say "Ask the AI helper to explain this error."

## Installing it

1. Use the AI package included with the bus installation.
2. Put `OPENROUTER_API_KEY` in the instance's private [host environment file](../runner/configuration.md#environment-files), owned by the runner account with mode 0600.
3. Set `enabled:true` for `ai` in the [runner inventory](../runner/configuration.md#inventory), preserving other entries, then reload the runner.
4. Set the default model: `agent-bus config '#ai@node.example' '{"model":"openai/gpt-4.1-mini"}'`.
5. Grant access only to callers who may spend the provider balance.

## Configuration

- `model` is the default chat model when a call names none, for example `openai/gpt-4.1-mini`; it is not used by `decide`.
- `base_url` is the OpenAI-compatible API root; default: `https://openrouter.ai/api/v1`. Only a trusted administrator should change it: it receives the provider key. Redirects are not followed.

## Secret keys

- **`OPENROUTER_API_KEY`,** only in the runner's private host file.
- **Never** in Configuration, bus Secret, a call body or a command argument.

## Who may use it

- **Only callers on the agent's allowed list;** it alone decides who spends the provider balance.
- **No roles or per-caller limits.**
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
| `error: 401` or `error: 403` from the provider | Ask the administrator to check the key and provider account |
| A bus permission refusal | Ask the owner to add you to the allowed list |
| `set Configuration model or provide a request model` | Set a default model or put `model` in the JSON; no provider call happens until then |
| Invalid request or Configuration | Check the model, API root and JSON; do not request streaming |
| Provider request timed out | Shorten the request or wait longer, at most 60 seconds |
| Answer exceeds the size bound | Ask the model for a shorter answer |

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

</details>

<details>
<summary><code>decide</code></summary>

- Only text after `decide ` that starts with `{` is a decision; "decide whether tea beats coffee" stays an ordinary prompt.
- The JSON method form is exactly `{"method":"decide","body":"<raw JSON string>"}`; a non-string `body` or extra method fields are refused.
- Put the decision model in the body; the chat default is never added, and a missing model is left to the provider to refuse.
- The raw string is validated as a non-streaming JSON object, then sent byte for byte, including whitespace and duplicate keys.
- The full provider JSON comes back, including every Choice probability; values are equal after JSON parsing, decimal spelling and arbitrary precision are not preserved, and non-standard NaN and Infinity are refused.
- `decide` uses the scheme, host and port of the trusted `base_url`, discards its path and posts to `/api/alpha/decisions`; `ask` appends `/chat/completions` to the configured base path.
- Both methods spend the same key under the same ACL, with the same limits, deadline, masking, no-redirect and no-retry rules.
- Logs hold the caller, a bounded model ID (or `invalid`), byte sizes and status, never the request, response or key.

</details>
