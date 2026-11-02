# Your service under the runner: the time service as packages

📌 **TL;DR:**
- **Problem:** a script run from your terminal stops with the terminal, and nothing starts it again, keeps it up or applies a host's limits to it.
- **Outcome:** the same time scripts and the Python process as packages the runner starts on demand, keeps up, confines and limits.
- **Start:** copy the packages into the runner's folder and reload it ([Try this first](#try-this-first)).

## Why you'd want it

- 🖥️ **Your terminal is the service manager.** Now the runner starts the script when a message arrives, and keeps the process up.
- 🔑 **Settings and keys sit beside the script.** Now Configuration and Secret belong to the agent's record and reach it at its next start.
- 🧱 **A script you run by hand is as open as your account unless you ask otherwise.** Now by default it sees its own package read-only, a writable work folder and the network it asked for.

## What you get

- ✅ **The same files:** the [simple and advanced scripts](write-agent-cli.md#try-this-first) and the [Python process](write-agent-process.md#try-this-first) run unchanged; only a package description, `agent.json`, is added beside each.
- ✅ **Help from the package:** the runner answers a simple package's help from its description; the advanced one sets `"help":"native"` so its own script shapes the answer from live Configuration.
- ✅ **A process that comes back:** the kept process serves itself, and the runner restarts it if it dies.
- ✅ **Confinement and limits you can read:** the shard reads the package views and the limits from a live agent at `slow`, and pushes a limit once to see it held.
- 🔑 **Per-message scripts hold no bus credential;** only the process that serves itself is handed its own token ([what runs as what](../runner/security.md#what-runs-as-what)).
- 🚫 **A package folder is code:** copying one in is a host change, so it takes the rights the folder needs.

## Try this first

- The packages are the example folders: each holds the script and an `agent.json` (for the fields, see [package options](../runner/configuration.md#package-options) and [package help](../runner/configuration.md#package-help)). `AGENTS` is your runner's agent folder, `CLIENTS` the clients folder of your checkout, `HOST` the runner's host folder, `RUNNER` its name from `agent-bus ls`, and `REALM` your realm. Put the three packages in place; the process carries the Python client with it:

<!-- run: runner -->
```sh
cp -R time-simple time-advanced time-process "$AGENTS/"
cp -R "$CLIENTS/python/agent_bus" "$AGENTS/time-process/agent_bus"
```

- Register the process's name and hand its token over once, then reload the runner. It starts the packages and the process; wait until both answer to `agent-bus ls`:

<!-- run: runner -->
```sh
agent-bus register "#time-process@$REALM" --kind agent >/dev/null
agent-bus token "#time-process@$REALM" > "$HOST/tokens/%23time-process@$REALM"
agent-bus call "$RUNNER" reload >/dev/null
until agent-bus ls "#time-simple@$REALM" >/dev/null 2>&1; do sleep 1; done
until agent-bus ls "#time-process@$REALM" | grep -q '"readers":1'; do sleep 1; done
```

- Call the simple service:

<!-- run: runner -->
```sh
agent-bus call "#time-simple@$REALM" epoch | jq -r .body
```

- Ask the advanced one what it does. Its help comes from its own script:

<!-- run: runner -->
```sh
agent-bus call "#time-advanced@$REALM" --help | jq -r .body
```

<!-- output -->
```text
time [now|iso|epoch] - the current UTC time, now in the configured format; history - the last five reports
```

- Give the advanced service and the process their format and key, and reload; both read them at their next start:

<!-- run: runner -->
```sh
STAMP_KEY=a-private-stamp-key
for a in time-advanced time-process; do
  agent-bus config "#$a@$REALM" '{"format":"epoch"}' >/dev/null
  agent-bus secret "#$a@$REALM" "TIME_STAMP_KEY=$STAMP_KEY" >/dev/null
done
agent-bus call "$RUNNER" reload >/dev/null
until agent-bus ls "#time-process@$REALM" | grep -q '"readers":1'; do sleep 1; done
```

- Ask each for the time in the configured format:

<!-- run: runner -->
```sh
agent-bus call "#time-advanced@$REALM" now | jq -r .body
agent-bus call "#time-process@$REALM" now | jq -r .body
```

## Technical details

- 📦 **Packages and the inventory:** where packages live, the options and the inventory entry that limits one: [configuration](../runner/configuration.md#inventory).
- 🧱 **Confinement:** the views, the network rule and the limits are in [isolation today](../runner/security.md#isolation-today); this page copies none of them.
- 🔬 **Where they are checked:** `src/acceptance/agent-guide/guide.sh` at `slow` runs these packages on a private runner and bus, reads the package, work and outside views, the network answer and the live cgroup limits, kills the kept process to see it return, and enforces the CPU share once. The runner needs a systemd scope for that, so the normal level checks only the packages' static facts and the packaged help rule through `agent-bus run --package-dir`.
- 🪪 **Serving itself:** [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves) say what is handed over and how a stop behaves.
- 🧭 **The checklist:** [what a good agent does](../10-writing-an-agent.md).
