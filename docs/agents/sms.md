# Telnyx SMS

📌 **TL;DR:** Send one text message from the bus. The provider key stays on the
server. An `accepted` answer means Telnyx accepted it, not that the phone received it.

## Why you'd want it

Send a short update to someone who is away from their computer. Your AI can
send the message without seeing the provider password.

## What it does

The helper sends one SMS through Telnyx. It accepts a phone number or a saved
nickname, such as `oncall`. Any bus caller may use it; the caller's name is logged
with each request.

## You'll need

- A bus account.
- For setup: an administrator and a Telnyx account with a sending number.
- The Telnyx API key: a password for the provider, kept in a private host file.

## Ask your AI

| Say this after deciding to send | What it calls |
|---|---|
| “Send oncall an SMS: the deploy finished.” | `#sms-telnyx` with recipient alias `oncall` and that text |
| “Text +12025550100 that the meeting moved to 3.” | `#sms-telnyx` with that number and text |
| “Show me how SMS is configured.” | `agent-bus help '#sms-telnyx@parf.us' config`; no key is shown |

## First call

```sh
agent-bus help '#sms-telnyx@parf.us'
agent-bus help '#sms-telnyx@parf.us' config
agent-bus help '#sms-telnyx@parf.us' secret
```

You should see the request format, sender settings and the key's name.

## Install

1. Package it on the runner host:

   ```sh
   sudo src/agents/sms-telnyx/package.sh /var/lib/agent-bus/agent.d/sms-telnyx
   ```

   You should see `sms.py`, `agent.json` and `env.dist` in that folder.
2. The administrator puts one `TELNYX_API_KEY` line into
   `/var/lib/agent-bus/parf-ab-runner/sms-telnyx/env`, mode `0600`, owned by
   `parf-ab-runner`. Do not add an extra `runner/` directory beneath the home.
   You should have a private file readable by the runner account.
3. Reload and set the public settings, replacing the sample sender number
   with your assigned Telnyx number:

   ```sh
   agent-bus call '#runner/parf@parf.us' reload
   agent-bus agent-template '#sms-telnyx@parf.us' '{"from":"+12025550100","aliases":{"oncall":"+12025550142"}}'
   agent-bus manage '#sms-telnyx@parf.us' --add-to-set-allow '*'
   ```

   You should see successful updates. The allow list is saved by the daemon;
   a runner reload keeps it. Without the provider key the runner refuses calls.

## Configuration and Secret keys

This example is commented to explain it. Remove comments before saving it:

```jsonc
{
  "from": "+12025550100", // Your Telnyx sender number, with country code
  "aliases": {"oncall": "+12025550142"} // Optional recipient nicknames
}
```

`TELNYX_API_URL` defaults to `https://api.telnyx.com/v2/messages` in env.dist;
`TELNYX_API_KEY= # required` declares the private key requirement.
The live entry accepts only the Telnyx host. Fake-provider tests explicitly use
a loopback URL and cannot connect to a real provider.
`TELNYX_API_KEY` belongs only in the host env file. There is no bus Secret to
set. Help reads `agent.json` and `env.dist`; `--help-config` and `--help-secret`
show settings and key names, never the key value.

## Use examples

Run a send only when you intend to send a real SMS.

| Goal | Command | Result |
|---|---|---|
| Send a short update | `agent-bus call '#sms-telnyx@parf.us' 'oncall The deploy finished.'` | One SMS to the configured alias |
| Keep exact text in JSON | `printf '%s' '{"to":"oncall","text":"Meeting at 3."}' \| agent-bus call '#sms-telnyx@parf.us' -` | One SMS with that text |
| Use a US phone format | `printf '%s' '{"to":"202-555-0100","text":"Hello."}' \| agent-bus call '#sms-telnyx@parf.us' -` | Number becomes `+12025550100` |

The package is `sms-telnyx`; its `sms` tag also supports `agent-bus call --tagged sms`.
The body is `{to,text}` or `<to> <text>`; there is no method name. Use JSON
when a formatted number contains spaces. Text must have 1–1600 characters.
The JSON answer has `status`, `to`, `from` and `provider_message_id`.

## Access and results

Any bus caller may send. Only the owner and maintainers may change the sender,
aliases or access list. To restrict access later:

```sh
agent-bus manage '#sms-telnyx@parf.us' --add-to-set-allow '@owner'
agent-bus manage '#sms-telnyx@parf.us' --remove-allow '*'
```

The helper makes at most one provider attempt per call. It never follows a
redirect or retries a timeout. There are no delivery callbacks yet.
Logs contain the caller, the destination's last four digits, status and message
id. They contain neither message text nor the provider key.

## If something goes wrong

| If you see | Do this |
|---|---|
| `accepted` | Report provider acceptance; wait for the recipient to confirm delivery. |
| `unknown`, a bus timeout or no usable answer | Do not send again. The first message may already have been sent. |
| `failed` | Telnyx refused the attempt. Ask the administrator to check the provider account. |
| `invalid` | Check the sender, recipient and 1–1600 character text. No provider call was made. |
| `needs TELNYX_API_KEY` | Ask the administrator to set the host file and reload. |
