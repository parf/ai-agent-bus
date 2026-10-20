# AI: ask a model through the bus

📌 **TL;DR:** Ask a model from any permitted bus session without giving that
session the provider password. The administrator chooses a default model and
keeps the password on the runner's host. There is no saved conversation.

## Why you'd want it

Your team can use one model connection without copying its password into every
AI session. Each caller sees the answer, while the administrator controls who
can make calls and spend money.

## What it does

Send a question and get the model's text back. Send a chat-completions JSON
request and get the provider's JSON back. Only callers on the allowed list
can use it. This node allows any bus caller.

## You'll need

- A working bus and runner, with Python 3 and network access.
- An account with an OpenAI-compatible model provider, such as OpenRouter.
- A model ID from that provider and a provider API key (a password).

## Ask your AI

Your AI session must be connected to the bus and allowed to call this agent.

| Say this | What it calls |
|---|---|
| “Ask the AI helper to explain this error in plain English.” | `#ai` with a text question |
| “Send these chat messages to the AI helper and show its full JSON reply.” | `#ai` with a chat-completions JSON body |
| “What model does the AI helper use by default?” | `agent-bus help '#ai@parf.us' config`, then Configuration if allowed |

## Use examples

### 1. Check what the helper accepts

```sh
agent-bus help '#ai@parf.us'
agent-bus help '#ai@parf.us' config
agent-bus help '#ai@parf.us' secret
```

You see its purpose, model settings and the key's name. Help never shows a key.
Scripts can use `call ... --help-config`, `--help-secret` or `--help-json`.

### 2. Ask one question

```sh
agent-bus call '#ai@parf.us' --wait 60s 'Explain a database backup in one sentence.'
```

The reply body holds the assistant's text. The CLI displays it in the usual
bus reply. Send the context again on your next call; the helper keeps no history.

### 3. Send a full chat request

```sh
agent-bus call '#ai@parf.us' --wait 60s '{"messages":[{"role":"user","content":"Say hello in French."}],"temperature":0.2}'
```

The reply body holds the provider's JSON, including its choices and usage.
Add `"model":"openai/gpt-4.1-mini"` to select a model for just that request.
The body is the provider's request itself: do not wrap it in `{"method":...}`.
Streaming is not supported.

## Installing it

These examples use account `parf` and host `parf.us`. Change those for your node.

1. Package it from the checkout:

   ```sh
   sudo src/agents/ai/package.sh /var/lib/agent-bus/agent.d/ai
   ```

   The destination should contain `ai.py`, `agent.json`, `env.dist` and `LICENSE.md`.
2. Add this entry inside the existing `/var/lib/agent-bus/parf-ab-runner/agents.json`:

   ```json
   "ai": {"enabled": true, "sandbox": "on"}
   ```

   Keep the other entries. The helper will use `#ai@parf.us`.
3. Reload the runner and set the allowed list:

   ```sh
   agent-bus call '#runner/parf@parf.us' reload
   agent-bus manage '#ai@parf.us' --add-to-set-allow '*'
   ```

   You should see the package status. Without a provider key, the runner reports
   `needs OPENROUTER_API_KEY`; it will not start until the host file is set.
4. Set the non-secret default model:

   ```sh
   agent-bus agent-template '#ai@parf.us' '{"model":"openai/gpt-4.1-mini"}'
   ```

   You should see a successful Configuration update. Use a model your provider offers.
5. The host administrator puts `OPENROUTER_API_KEY` in
   `/var/lib/agent-bus/parf-ab-runner/ai/env`, using a private host editor
   and mode `0600`, owned by the runner account. Then run the reload command again.
   The next permitted call should return a model answer.

Keep that key only in the runner's host file. Never put it in Configuration,
bus Secret, a call body or a command argument. Help never reveals the key. The runner must be configured before it can
answer help or model calls.

## Configuration and Secret keys

This commented example explains every Configuration key. Remove comments before
sending JSON:

```jsonc
{
  "model": "openai/gpt-4.1-mini", // Default when the call has no model
  "base_url": "https://openrouter.ai/api/v1" // Optional compatible API root
}
```

Only the administrator should change the API root: it receives the provider key.
The proxy appends `/chat/completions`. It does not follow redirects.
`OPENROUTER_API_KEY` is the only private key; it belongs in the host file above.
There are no bus Secret keys for this helper.

## Roles and access

The helper has no extra roles or per-caller limits. Its allowed list alone
decides who can spend the provider balance. This node allows any bus caller
with `*`. That setting is stored by the daemon and survives runner reload.
To restrict it to the owner later:

```sh
agent-bus manage '#ai@parf.us' --add-to-set-allow '@owner'
agent-bus manage '#ai@parf.us' --remove-allow '*'
```

The caller's wait bounds a request, up to the package's 60-second timeout.
Requests and answers are limited to 1 MiB each. Logs record the caller's bus name, model,
byte sizes and status; the provider still receives the prompt and may retain it
under its own policies.

## If something goes wrong

| If you see… | Do this |
|---|---|
| `needs OPENROUTER_API_KEY`, or `OPENROUTER_API_KEY is not set` when run directly | Ask the host administrator to set the private host file, then reload. |
| `error: 401` or `error: 403` from the provider | Ask the administrator to check the key and provider account. |
| A bus permission refusal | Ask the owner to add you to this agent's allowed list. |
| `set Configuration model or provide a request model` | Set a default model before sending text, or include `model` in a JSON request. No provider call happens until a model is supplied. |
| Invalid request or Configuration | Check the model, API root and JSON. Do not request streaming. |
| Provider request timed out | Use a shorter request or a larger caller wait, at most 60 seconds. |
| Answer exceeds the size bound | Ask the model for a shorter answer. |
