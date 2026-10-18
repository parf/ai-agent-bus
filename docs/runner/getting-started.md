# Your first managed agent

📌 **TL;DR:** Add a script to your `agent.d`, reload your runner, and call
the agent. The runner starts the script for each message and returns its output;
you do not need to keep a terminal open.

The runner is included with AgentBus. If yours is not installed yet, use
[setup](../07-setup.md#a-persons-runner). The host needs the
[sandbox prerequisites](security.md#isolation-today).

## Set the names

Replace `your-bus-user` with your plain bus user name, such as `alice`.
This example uses `hello-guide`; choose another name if it is already in use.

```sh
ME=your-bus-user
HOST=$(hostname)
RUNNER="#runner/$ME@$HOST"
AGENT="#hello-guide@$HOST"
MINE=/var/lib/agent-bus/runners/$ME
```

## Install the script

Run as yourself. Setup gives you write access to your `agent.d`:

```sh
mkdir -p "$MINE/agent.d/hello-guide"
printf '#!/bin/sh\nprintf "hello, %%s\\n" "$1"\n' >"$MINE/agent.d/hello-guide/hello.sh"
chmod 755 "$MINE/agent.d/hello-guide/hello.sh"
printf '{"command":"./hello.sh","form":"args"}\n' >"$MINE/agent.d/hello-guide/agent.json"
```

The script receives the message body as one argument and prints the answer.
The directory name supplies the agent name; no inventory entry is needed.

## Load it and grant access

Reload your runner and check that the agent is `idle`:

```sh
agent-bus call "$RUNNER" --wait 15s reload
agent-bus call "$RUNNER" --wait 15s status
```

The runner registers `$AGENT` for you. You own it and can call it immediately.
To let someone else call it, use
`agent-bus manage "$AGENT" --add-allow <their name>`.

If status says `refused`, follow its reason before continuing.

## Call it

From a terminal authenticated as `$ME`:

```sh
agent-bus call "$AGENT" --wait 10s world
```

You receive an acknowledgment, then the answer `hello, world`.
The script's stdout becomes the answer; stderr goes to
`journalctl -u "agent-bus-runner@$ME"`.

For failures, see [troubleshooting](operations.md#when-a-call-does-not-come-back).

## Finish the example

To disable or remove the agent, follow
[operations](operations.md#installing-changing-and-removing-an-agent).
