# Your first managed agent

📌 **TL;DR:** Install a small script, ask the runner to reload, then grant your
bus user access and call it. The runner runs the script for each message;
you do not need to leave a terminal open.

- Use a host with AgentBus installed and `agent-bus-runner` active.
- You need host administration access to install code under the runner's account.
- You also need the agent's Owner or daemon Owner to grant caller access.
- Install bubblewrap and enable unprivileged user namespaces; the runner
  refuses an agent when its default sandbox cannot be created.
- Read the [current isolation limits](security.md#isolation-today).

## Set the names

This walkthrough uses a new name, `#hello-guide@<realm>`. Replace the values
below with the daemon's realm and your exact bus user name. Choose another
agent name if this one is already in use.

```sh
REALM=your-bus-realm
CALLER=your-bus-user
RUNNER="#runner@$REALM"
AGENT="#hello-guide@$REALM"
systemctl status agent-bus-runner
```

If the unit is missing, follow [installation](../07-setup.md#install).
The commands below use the standard installation paths.
They describe the current shared runner account. Per-person runners are
[planned](../../Plans/R1.0-Release/runner-implementation.md#who-it-runs-as);
their setup is not available yet.

## Install the script

```sh
sudo install -d -o agent-bus-runner -g agent-bus-runner /var/lib/agent-bus/agent.d/hello-guide
printf '#!/bin/sh\nprintf "hello, %%s\\n" "$1"\n' | sudo tee /var/lib/agent-bus/agent.d/hello-guide/hello.sh >/dev/null
sudo chmod 755 /var/lib/agent-bus/agent.d/hello-guide/hello.sh
printf '{"command":"./hello.sh","form":"args"}\n' | sudo tee /var/lib/agent-bus/agent.d/hello-guide/agent.json >/dev/null
```

The script takes the message body as one argument and prints the answer.
No inventory entry is needed for this example: an installed directory is
enabled by default. Its directory name supplies the agent's default name.

## Load it and grant access

Run these as the runner's account, using its local account socket rather than
any bus credentials inherited from your terminal:

```sh
sudo -u agent-bus-runner env -u AGENT_BUS_ADDR -u AGENT_BUS_TOKEN -u AGENT_BUS_NAME agent-bus call "$RUNNER" --wait 15s reload
sudo -u agent-bus-runner env -u AGENT_BUS_ADDR -u AGENT_BUS_TOKEN -u AGENT_BUS_NAME agent-bus manage "$AGENT" --add-allow "$CALLER" --add-maintainers "$CALLER"
sudo -u agent-bus-runner env -u AGENT_BUS_ADDR -u AGENT_BUS_TOKEN -u AGENT_BUS_NAME agent-bus call "$RUNNER" --wait 15s status
```

Find your agent's row in `status`: `idle` means it is ready for work.
If it says `refused`, follow its reason before continuing.
The access step is required: the runner owns the new record, and a fresh
record does not admit your user automatically. Adding your user as a Maintainer
also lets you manage this record later.

## Call it

From a terminal authenticated as the bus user named in `CALLER`:

```sh
agent-bus call "$AGENT" --wait 10s world
```

Expect an acknowledgment followed by an answer from your agent whose body is
`hello, world`. The script's stdout is the answer; diagnostics written to
stderr appear in `journalctl -u agent-bus-runner`.

If your caller is another agent, grant that agent's name instead.
If the call is refused or times out, use [troubleshooting](operations.md#when-a-call-does-not-come-back).

## Finish the example

To leave the code installed but stop serving it, add
`"hello-guide": {"enabled": false}` to the existing
`/var/lib/agent-bus/runner/agents.json` object, preserving its other entries,
then call `reload` as above. Do not replace an existing inventory with the example.

For removal, follow [operations](operations.md#installing-changing-and-removing-an-agent).
Stopping service does not by itself delete its bus record.
