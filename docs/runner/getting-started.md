# Your first managed agent

📌 **TL;DR:** Have your runner installed, put a small script in your
`agent.d`, ask the runner to reload, and call it. The runner is your agent and
every agent it serves is yours; it runs the script for each message, so you do
not need to leave a terminal open.

- Use a host with AgentBus installed.
- Installing your runner needs host administration access once; installing
  code afterwards does not.
- Install bubblewrap and enable unprivileged user namespaces; the runner
  refuses an agent when its default sandbox cannot be created.
- Read the [current isolation limits](security.md#isolation-today).

## Set the names

This walkthrough uses a new name, `#hello-guide@<host>`. Replace `ME` with
your exact bus user name, a plain one such as `alice`. Choose another agent
name if this one is already in use.

```sh
ME=your-bus-user
HOST=$(hostname)
RUNNER="#runner/$ME@$HOST"
AGENT="#hello-guide@$HOST"
MINE=/var/lib/agent-bus/runners/$ME
```

If AgentBus itself is missing, follow [installation](../07-setup.md#install).

## Install your runner

Once, by an administrator ([a person's runner](../07-setup.md#a-persons-runner)):

```sh
sudo agent-bus-setup --runner install --for "$ME"
systemctl status "agent-bus-runner@$ME"
```

That makes your runner's own account, your `$MINE/agent.d`, and your agent
`$RUNNER`, whose token the runner is started with; it never holds yours.

## Install the script

Your `agent.d` is yours to write:

```sh
mkdir -p "$MINE/agent.d/hello-guide"
printf '#!/bin/sh\nprintf "hello, %%s\\n" "$1"\n' >"$MINE/agent.d/hello-guide/hello.sh"
chmod 755 "$MINE/agent.d/hello-guide/hello.sh"
printf '{"command":"./hello.sh","form":"args"}\n' >"$MINE/agent.d/hello-guide/agent.json"
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
stderr appear in `journalctl -u "agent-bus-runner@$ME"`.

If the call is refused or times out, use [troubleshooting](operations.md#when-a-call-does-not-come-back).

## Finish the example

To leave the code installed but stop serving it, add
`"hello-guide": {"enabled": false}` to the existing
`$MINE/runner/agents.json` object (the runner account's, so `sudo`), preserving its other entries,
then call `reload` as above. Do not replace an existing inventory with the example.

For removal, follow [operations](operations.md#installing-changing-and-removing-an-agent).
Stopping service does not by itself delete its bus record.
