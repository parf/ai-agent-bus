# Debug: check your runner

📌 **TL;DR:** This tiny helper says hello and shows the settings a runner
passes to a script. Use it to check an installation before connecting a real
bot. Private environment values are masked; bus-delivered fields remain visible.

## Why you'd want it

A quick hello can separate a runner problem from a bot setup problem.
That saves time when a new helper seems silent.

## What it does

`hello` returns a greeting. `env` shows the script's environment names, masking
all values except `AGENT_BUS_*` and `AB_ROLE_*`: named settings
such as who asked and which permissions the script received. Anyone allowed
to call debug can see those values, so only trusted maintainers should use it.

## You'll need

- A [person's runner](README.md#runner-helpers).
- Python 3 on the runner host.
- Permission to install code and manage the runner.

## Use examples

### Ask your AI

| Say this | What it will call |
|---|---|
| “Check that my runner can answer a simple question.” | Debug `hello smoke` |
| “Check which name and topic the runner passes to a script.” | Debug `env`; trusted access only |
| “Check whether my test permission reached the script.” | Debug `env`, then inspect the `AB_ROLE_…` setting |

### 1. Test the full round trip

```sh
agent-bus call '#debug@parf.us' 'hello smoke'
```

You should receive `hello, smoke`. The runner received your question, ran the
script and returned its answer.

### 2. Check the sender and topic

```sh
agent-bus call '#debug@parf.us' 'env' --topic setup-test
```

The answer is JSON, so the settings arrive as one line with `\n` in it. To see
one setting per line, print just the body with `jq -r`:

```sh
agent-bus call '#debug@parf.us' 'env' --topic setup-test | jq -r '.body'
```

```
AB_ROLE_MAINTAINER=1
AB_ROLE_OWNER=1
AGENT_BUS_FROM=parf
AGENT_BUS_TO=#debug@parf.us
AGENT_BUS_TOPIC=setup-test
...
```

Find `AGENT_BUS_FROM` and `AGENT_BUS_TOPIC=setup-test` in the answer. These
are settings passed by the runner; a caller cannot invent permission by
putting a role name in the message text.

### 3. Check a test permission

After the owner grants you a test permission:

```sh
agent-bus manage '#debug@parf.us' --add-allow 'ann@parf.us(test)'
```

Ann calls `env` and should see `AB_ROLE_TEST=1`. Remove that access when the
test is over, using `--remove-allow 'ann@parf.us(test)'` as the owner.

## Installing it

1. Copy its small package into the shared code directory:

   ```sh
   sudo mkdir -p /var/lib/agent-bus/agent.d/debug
   sudo cp src/agents/debug/agent.json src/agents/debug/debug.py /var/lib/agent-bus/agent.d/debug/
   ```

   The directory now contains the description and script.
2. Reload your runner:

   ```sh
   agent-bus call '#runner/parf@parf.us' reload
   ```

   Debug should appear as `idle`: it starts a script only when a question arrives.
   No manual bot registration or outside token is needed.
3. Try the greeting:

   ```sh
   agent-bus call '#debug@parf.us' 'hello smoke'
   ```

   You should see `hello, smoke`.

If this runner's inventory already disables debug, enable it with
`agent-bus call '#runner/parf@parf.us' 'enable debug'` first.

## Configuration

There are no debug-specific settings. The full sample is:

```jsonc
{} // no settings; remove the comment before saving JSON
```

Debug accepts text calls only, not a JSON method form. Send `hello name` or
`env`. A JSON-looking message is ordinary text and does not run a JSON command.

## Secret keys

None. Do not attach secrets to debug or put passwords in its environment.
Its `env` call shows names, but masks every value except `AGENT_BUS_*` and `AB_ROLE_*`.

## Who may use it

Leave the default access list restricted to the owner and maintainers.
For a trusted reader, the owner may use:

```sh
agent-bus manage '#debug@parf.us' --add-allow 'ann@parf.us'
```

This grants both the greeting and the environment view. There is no separate
permission for hiding some settings from that caller.

## Troubleshooting

| If you see this | Do this |
|---|---|
| `idle` in runner status | Normal: debug runs only for a question |
| No agent by that name | Check the runner status and whether debug is enabled |
| Script cannot run | Check Python 3 and that `debug.py` is executable |
| A setting contains a password | Restrict access, remove the secret and replace the exposed password |
| Unknown command | Use `hello <name>`, `env`, or ask `--help` |
