# AI: ask a language model without sharing its password

📌 **TL;DR:** One shared door to an AI model (like the one behind ChatGPT) for
everyone on the bus. You send a question, you get the answer back. The account
password stays with the administrator, who also decides who may ask and which
model answers. Nothing is remembered between questions.

## Why you'd want it

**The problem.** Using a paid AI model needs an account key. Copying that key
into every script and every AI session means it leaks, and nobody knows who
spends the money.

**The idea.** The key stays in one locked place on the server. Callers get a
door, not the key: they can ask questions, and the administrator controls who
may open the door and can close it any time.

**When to use it**

- A script or bot needs a short AI answer: explain, summarise, translate.
- Several people or AI sessions should share one model account.
- You need a fixed choice with probabilities ("billing, bug or spam?") — see [typed decisions](#typed-decisions).

**When not to:** long conversations with memory (it keeps none), live
word-by-word output (no streaming), or text that must never reach an outside AI
provider (the provider sees every prompt).

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| Explain an error | Paste the error message | A plain-English explanation |
| Nightly report summary | A script sends the build log | A three-line summary for the team |
| Sort incoming tickets | Send the ticket with your categories | The category with its probability |
| Translate a notice | Send the text and target language | The translation |
| Feed a bigger app | [RAG](rag.md#what-it-does) calls it to sort texts | Topics for each stored text |

## What it does

| Call | You send | You get |
|---|---|---|
| `ask` (or just your text) | A question, or a full chat request | The answer text, or the provider's full reply |
| `decide` | A question with fixed choices | Each choice with its probability |

Only callers on the allowed list can use it. Each call stands alone: send the
context again next time.

<details><summary>Technical details: request forms</summary>

- `ask` accepts a bare prompt, `ask <prompt>`, `{"method":"ask","prompt":"Hello"}`, `{"method":"ask","prompt":{"messages":[...]}}` or a chat-completions JSON object. Prompt forms return assistant text; chat-body forms return the provider's JSON, including choices and usage.
- A JSON request may set `"model"` for that request only. Streaming is not supported.

</details>

## Typed decisions

`decide` asks a decision model, such as Jev, a question with fixed answers and
returns how likely each answer is. It uses the [OpenRouter Decisions API](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request).

```sh
agent-bus call '#ai@node.example' 'decide {"model":"typesafe/jev-1.14","state":"Tea","questions":{"drink":{"type":"choice","instructions":"Which drink?","criteria":{"tea":"Tea","coffee":"Coffee"}}}}'
```

<details><summary>Technical details: <code>decide</code></summary>

- Only text after `decide ` that starts with `{` is a decision; “decide whether tea beats coffee” stays an ordinary prompt.
- JSON method form is exactly `{"method":"decide","body":"<raw JSON string>"}`. A non-string `body` or extra method fields are refused.
- Put the decision model in the body; the chat default is never added, and a missing model is left to the provider to refuse.
- The raw string is validated as a non-streaming JSON object, then sent byte for byte, including whitespace and duplicate keys.
- The full provider JSON comes back, including every Choice probability. Values are equal after JSON parsing; decimal spelling and arbitrary precision are not preserved. Non-standard NaN/Infinity are refused.
- `decide` uses the scheme, host and port of the trusted `base_url`, discards its path and posts to `/api/alpha/decisions`. `ask` appends `/chat/completions` to the configured base path.
- Both methods spend the same key under the same ACL, with the same limits, deadline, masking, no-redirect and no-retry rules. Logs hold the caller, a bounded model ID (or `invalid`), byte sizes and status, never the request, response or key.

</details>

## You'll need

- An account with an AI provider that speaks the common OpenAI format, such as OpenRouter: its key and a model name.
- A bus runner with Python 3 and network access.

## Try this first

```sh
agent-bus help '#ai@node.example'
```

It shows what the helper does, its settings and the key's name — never the key.

<details><summary>More help forms</summary>

`agent-bus help '#ai@node.example' config` and `... secret` show one part.
Scripts can use `call ... --help-config`, `--help-secret` or `--help-json`.

</details>

## Use examples

| I want to… | Command |
|---|---|
| Ask one question | `agent-bus call '#ai@node.example' --wait=60s 'Explain a database backup in one sentence.'` |
| Send a full chat request | `agent-bus call '#ai@node.example' --wait=60s '{"messages":[{"role":"user","content":"Say hello in French."}],"temperature":0.2}'` |
| Use another model once | add `"model":"openai/gpt-4.1-mini"` to the JSON |
| Get probabilities for fixed choices | a `decide` call, [above](#typed-decisions) |
| See the default model | `agent-bus help '#ai@node.example' config` |
| Ask from my AI session | say “Ask the AI helper to explain this error.” |

## Installing it

An administrator does this once: install the package, put the provider key in
the runner's private file, choose who may call and the default model.

<details><summary>Step by step</summary>

1. Package it: `sudo src/agents/ai/package.sh /var/lib/agent-bus/agent.d/ai` (gives `ai.py`, `agent.json`, `env.dist`, `LICENSE.md`).
2. Enable `"ai": {"enabled": true, "sandbox": "on"}` in the [runner inventory](../runner/configuration.md#inventory) and reload the runner. Without a key the runner reports `needs OPENROUTER_API_KEY` and does not start it.
3. Choose callers: `agent-bus manage '#ai@node.example' --add-to-set-allow '*'` (or a narrower list).
4. Set the default model: `agent-bus config '#ai@node.example' '{"model":"openai/gpt-4.1-mini"}'`.
5. The host administrator puts `OPENROUTER_API_KEY` in the agent's private [host environment file](../runner/configuration.md#environment-files), then reloads the runner.

The runner must be configured before the agent answers help or model calls.

</details>

## Configuration

Which model answers by default, and which provider address to use.

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `model` | Default chat model when a call names none, e.g. `openai/gpt-4.1-mini`; not used by `decide` |
| `base_url` | OpenAI-compatible API root, default `https://openrouter.ai/api/v1` |

Only the administrator should change `base_url`: it receives the provider key.
Redirects are not followed. The shipped network declaration permits only `openrouter.ai`.

</details>

## Secret keys

`OPENROUTER_API_KEY`, only in the runner's private host file. Never in
Configuration, bus Secret, a call body or a command argument.

## Who may use it

Only callers on the agent's allowed list; it alone decides who spends the
provider balance. There are no roles or per-caller limits. The provider still
sees each prompt and may keep it under its own policy.

<details><summary>Technical details: narrowing access and limits</summary>

The list is stored by the daemon and survives a runner reload. To narrow `*` to the owner:

```sh
agent-bus manage '#ai@node.example' --add-to-set-allow '@owner'
agent-bus manage '#ai@node.example' --remove-allow '*'
```

- The caller's wait bounds a request, up to the package's 60-second timeout.
- Requests and answers are at most 1 MiB each.
- Logs record the caller's bus name, model, byte sizes and status.

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
