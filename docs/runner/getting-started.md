# Getting started: your first script answers by name

📌 **TL;DR:**
- **Problem:** you want to see a script become a shared tool before you trust the runner with real work.
- **Outcome:** in a few minutes a two-line script answers calls under its own name, with no terminal left open.
- **Start:** check your runner answers ([Try this first](#try-this-first)), then follow the four steps below.

## Why you'd want it

- ⏱️ **Trying a new tool takes an afternoon.** Now it takes four short steps.
- 🖥️ **A script stops when you close the terminal.** Now the runner starts it whenever a message arrives.
- 🧾 **Wrapping a script for messages means new code.** Now the script reads one argument and prints its answer.

## What you get

- ✅ **A named agent:** `#hello-guide@<realm>` answers `hello, world`.
- ✅ **You own it:** you can call it at once and decide who else may.
- ✅ **A pattern to reuse:** the same folder, reload and call work for every later script.
- ⚠️ **Shared folder:** installing into it needs `sudo` on the host.

## You'll need

- Your runner installed: [setup](setup.md#a-persons-runner).
- The host's [sandbox prerequisites](security.md#isolation-today).
- A terminal signed in to the bus as yourself.

## Try this first

Confirm your identity and the bus's realm:

```sh
agent-bus status
```

Find your runner in the agent list:

```sh
agent-bus ls --kind=agent
```

## Set the names

Use your bus user name and the `realm` from status. Check that `RUNNER` matches the runner name in the listing; the bus realm need not match the computer's hostname. Pick another agent name if `hello-guide` is taken.

```sh
ME=your-bus-user
REALM=your-bus-realm
RUNNER="#runner/$ME@$REALM"
AGENT="#hello-guide@$REALM"
AGENTS=/var/lib/agent-bus/agent.d
```

## Install the script

Install the code as root into the shared folder that every runner reads:

```sh
sudo mkdir -p "$AGENTS/hello-guide"
printf '#!/bin/sh\nprintf "hello, %%s\\n" "$1"\n' | sudo tee "$AGENTS/hello-guide/hello.sh" >/dev/null
sudo chmod 755 "$AGENTS/hello-guide/hello.sh"
printf '{"command":"./hello.sh","form":"args","asks":"none","description":"Greets the caller"}\n' | sudo tee "$AGENTS/hello-guide/agent.json" >/dev/null
```

- 📥 The script gets the message as one argument and prints the answer.
- 🏷️ The folder name becomes the agent name; nothing else to list.
- 📝 Every package needs a `description`; callers see it.
- 🙋 `"asks":"none"` says the script knows nothing of questions about itself: the runner answers `help` from the description and replies `unsupported: <ask>` to the rest, without starting it.

## Load it and grant access

Reload your runner, then check that `hello-guide` shows `idle`:

```sh
agent-bus call "$RUNNER" --wait 15s reload
agent-bus call "$RUNNER" --wait 15s status
```

- ✅ The runner registers `$AGENT` for you; you own it and can call it now.
- 🙋 Let someone else call it: `agent-bus manage "$AGENT" --add-allow <their name>`.
- ⚠️ If status says `refused`, fix the reason it names, then reload again.

## Call it

From a terminal signed in as `$ME`, call the new agent:

```sh
agent-bus call "$AGENT" --wait 10s world
```

- The answer is `hello, world`.
- Stuck? See [when a call does not come back](operations.md#when-a-call-does-not-come-back).



## Finish the example

- Disable or remove the agent: [operations](operations.md#installing-changing-and-removing-an-agent).
- Write a real one next: [Agents](agents.md).

## Technical details

<details>
<summary>What happens on each call</summary>

- One process starts per message and exits after answering.
- The script's printed output becomes the answer.
- Its error output goes to the runner's log: `journalctl -u "agent-bus-runner@$ME"`.
- The runner sends a "still working" acknowledgment only when the script has not answered within 2 seconds.

</details>
