# Your first managed agent

📌 **TL;DR:** Give the runner your runner agent's token, install a small
script, ask the runner to reload, and call it. The runner is your agent and
every agent it serves is yours; it runs the script for each message, so you do
not need to leave a terminal open.

- Use a host with AgentBus installed.
- You need host administration access to install code under the runner's account.
- Install bubblewrap and enable unprivileged user namespaces; the runner
  refuses an agent when its default sandbox cannot be created.
- Read the [current isolation limits](security.md#isolation-today).

## Set the names

This walkthrough uses a new name, `#hello-guide@<realm>`. Replace the values
below with the daemon's realm and your exact bus user name. Choose another
agent name if this one is already in use.

```sh
REALM=your-bus-realm
ME=your-bus-user
RUNNER="#runner/$ME@$REALM"
AGENT="#hello-guide@$REALM"
```

If the unit is missing, follow [installation](../07-setup.md#install).
The commands below use the standard installation paths and the one shared
`agent-bus-runner` account, so a host runs one person's runner until the
setup verbs of [M.11](../../Plans/R1.0-Release/TODO.md#managed-runner-waves)
give each person their own.

## Start your runner

The runner is your agent, never you: create it once, then hand the runner its
token. It refuses to start with your own.

```sh
agent-bus register "$RUNNER" --kind agent --allow "$ME"
agent-bus-token "$RUNNER" | sed 's/^/AGENT_BUS_TOKEN=/' | sudo -u agent-bus-runner tee /var/lib/agent-bus/runner/runner.env >/dev/null
sudo chmod 600 /var/lib/agent-bus/runner/runner.env
sudo systemctl restart agent-bus-runner
systemctl status agent-bus-runner
```

## Install the script

```sh
sudo install -d -o agent-bus-runner -g agent-bus-runner /var/lib/agent-bus/agent.d/hello-guide
printf '#!/bin/sh\nprintf "hello, %%s\\n" "$1"\n' | sudo tee /var/lib/agent-bus/agent.d/hello-guide/hello.sh >/dev/null
sudo chmod 755 /var/lib/agent-bus/agent.d/hello-guide/hello.sh
printf '{"command":"./hello.sh","form":"args"}\n' | sudo tee /var/lib/agent-bus/agent.d/hello-guide/agent.json >/dev/null
sudo chown agent-bus-runner:agent-bus-runner /var/lib/agent-bus/agent.d/hello-guide/hello.sh /var/lib/agent-bus/agent.d/hello-guide/agent.json
```

The script takes the message body as one argument and prints the answer.
No inventory entry is needed for this example: an installed directory is
enabled by default. Its directory name supplies the agent's default name.

## Load it and grant access

As yourself:

```sh
agent-bus call "$RUNNER" --wait 15s reload
agent-bus call "$RUNNER" --wait 15s status
```

Find your agent's row in `status`: `idle` means it is ready for work.
If it says `refused`, follow its reason before continuing.
Also check the runner's reported version before relying on its
[sandbox protections](security.md#isolation-today); an older running process
does not gain them merely because newer files are installed.
The runner created `$AGENT` for you: you own it, and a fresh record admits its
Owner. To let somebody else call it, widen its ACL:
`agent-bus manage "$AGENT" --add-allow <their name>`.

## Call it

From a terminal authenticated as `$ME`:

```sh
agent-bus call "$AGENT" --wait 10s world
```

This script call returns an acknowledgment followed by an answer from your agent
whose body is `hello, world`. The runner's own `status` and `reload` calls
return an answer without an acknowledgment. The script's stdout is the answer;
diagnostics written to
stderr appear in `journalctl -u agent-bus-runner`.

If the call is refused or times out, use [troubleshooting](operations.md#when-a-call-does-not-come-back).

## Finish the example

To leave the code installed but stop serving it, add
`"hello-guide": {"enabled": false}` to the existing
`/var/lib/agent-bus/runner/agents.json` object, preserving its other entries,
then call `reload` as above. Do not replace an existing inventory with the example.

For removal, follow [operations](operations.md#installing-changing-and-removing-an-agent).
Stopping service does not by itself delete its bus record.
