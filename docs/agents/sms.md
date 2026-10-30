# SMS: send a text message to a phone

📌 **TL;DR:** Lets you or your AI send one ordinary text message (SMS) to a
phone, through the provider Telnyx. The provider password stays on the server.
"Accepted" means Telnyx took the message, not that the phone got it.

## Why you'd want it

**The problem.** Someone needs to know now, but they are away from their
computer and not in any chat app.

**The idea.** One call sends one text message. Your AI can do it without ever
seeing the provider password, and every send is logged with who asked.

**When to use it**

- A short, urgent note to a person on the move.
- An on-call alert that must reach a phone.
- Any phone, no app needed on the other end.

**When not to:** a conversation or a reply is needed (use [Telegram](telegram.md)
or [Slack](slack.md)), long or formatted text, or many messages: each one costs money.

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| On-call alert | Send to the nickname `oncall` | The on-call phone gets the alert |
| Job finished | Ask the AI to text you when the backup ends | A text when it is done |
| Schedule change | Text a colleague "meeting moved to 3" | They know without opening a laptop |
| Deploy note | Send "deploy finished" to the team lead | A short confirmation |

## What it does

| Ability | In plain words |
|---|---|
| `send` | Send one text to a phone number or a saved nickname |

Any bus caller may send, and the caller's name is logged. It tries once: no
automatic retry, and no delivery report yet.

<details><summary>Technical details: call forms and answer</summary>

- Method `send <to> <text>`, or JSON `{"method":"send","to":"+12025550142","text":"Hello"}`.
  Bare `{to,text}` and `<to> <text>` remain supported, including the send-sms
  wrapper; all forms use the same one-attempt handler.
- Use JSON when a formatted number contains spaces. Text is 1–1600 characters.
- A US format such as `202-555-0100` becomes `+12025550100`.
- The JSON answer has `status`, `to`, `from` and `provider_message_id`.
- One provider attempt per call; it never follows a redirect or retries a timeout.
- `agent-bus ls --tag=messaging` finds messaging helpers; call SMS by its exact
  name, since they accept different calls.

</details>

## You'll need

- A bus account.
- For setup: an administrator and a Telnyx account with a sending number.
- The Telnyx API key, kept in a private file on the server.

## Try this first

```sh
agent-bus help '#sms-telnyx@node.example'
```

It shows the request format, sender settings and the key's name, never its value;
see [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

## Use examples

Each command sends a real SMS; run it only when you mean to.

| I want to… | Command |
|---|---|
| Alert on-call | `agent-bus call '#sms-telnyx@node.example' 'send oncall The deploy finished.'` |
| Send exact text | `agent-bus call '#sms-telnyx@node.example' '{"method":"send","to":"oncall","text":"Meeting at 3."}'` |
| Text a number with dashes | `agent-bus call '#sms-telnyx@node.example' '{"to":"202-555-0100","text":"Hello."}'` |

If the result is unknown or the call times out, **do not send again**: the
first message may already be on its way.

## Installing it

An administrator does this once: install the package, put the Telnyx key in a
private file on the server, set the sender number.

<details><summary>Step by step (runner account <code>ann-ab-runner</code>)</summary>

1. Package it on the runner host (see [runner helpers](README.md#runner-helpers)):

   ```sh
   sudo src/agents/sms-telnyx/package.sh /var/lib/agent-bus/agent.d/sms-telnyx
   ```

2. Write one `TELNYX_API_KEY` line to `/var/lib/agent-bus/ann-ab-runner/sms-telnyx/env`,
   mode `0600`, owned by the runner account
   ([where settings belong](../runner/configuration.md#where-settings-belong)).
3. Reload, set the sender and nicknames, and open it to all callers:

   ```sh
   agent-bus call '#runner/ann@node.example' reload
   agent-bus config '#sms-telnyx@node.example' '{"from":"+12025550100","aliases":{"oncall":"+12025550142"}}'
   agent-bus manage '#sms-telnyx@node.example' --add-to-set-allow '*'
   ```

   The daemon keeps the allow list across reloads. Without the key the runner refuses calls.

</details>

## Configuration

Your sender number and optional nicknames for people you text often.

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `from` | Your Telnyx sender number, with country code |
| `aliases` | Optional recipient nicknames, e.g. `{"oncall": "+12025550142"}` |

`TELNYX_API_URL` defaults to `https://api.telnyx.com/v2/messages` in `env.dist`;
`TELNYX_API_KEY= # required` declares the key. The live entry accepts only the
Telnyx host. Fake-provider tests use a loopback URL and cannot reach a real provider.

</details>

## Secret keys

`TELNYX_API_KEY` lives only in the private host file; there is no bus Secret.
Help shows key names, never the value.

## Who may use it

Any bus caller may send. Only the owner and maintainers may change the sender,
nicknames or allow list. Logs hold the caller, the last four digits of the
number, status and message id; never the text or the key.

<details><summary>Restrict it to the owner</summary>

```sh
agent-bus manage '#sms-telnyx@node.example' --add-to-set-allow '@owner'
agent-bus manage '#sms-telnyx@node.example' --remove-allow '*'
```

</details>

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#sms-telnyx` reports these. The page that shows them is in work.

| Level | When | What to do |
|---|---|---|
| alert | `TELNYX_API_KEY` is missing, or Telnyx rejects the key | Set the key in the host file, then reload the runner |
| alert | `from` is missing or not a valid number | Set `from` in Configuration |
| warning | A send's outcome is `unknown` (timeout, server error, redirect) | Check the provider portal before anyone sends again |
| warning | Telnyx refused one message (`failed`) | Check the recipient and the provider account |

## Troubleshooting

| If you see | Do this |
|---|---|
| `accepted` | Telnyx accepted it; wait for the recipient to confirm |
| `unknown`, a bus timeout or no usable answer | Do not send again; it may already have been sent |
| `failed` | Telnyx refused it; the administrator checks the provider account |
| `invalid` | Check sender, recipient and 1–1600 character text; no provider call was made |
| `needs TELNYX_API_KEY` | The administrator sets the host file and reloads |
