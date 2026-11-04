# Your service under the runner: the time service as packages

📌 **TL;DR:**
- **Problem:** a script run from your terminal stops with the terminal, and nothing starts it again, keeps it up or applies a host's limits to it.
- **Outcome:** the runner starts the time scripts on demand and supervises the Python process; Linux hosts can apply its sandbox and limits.
- **Start:** copy the packages into the runner's folder and reload it ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 Keep settings and keys separate from the package code, on the agent's record.
- 🧱 **A script you run by hand is as open as your account unless you ask otherwise.** On Linux, the default sandbox provides a read-only package and writable work folder. A network request grants the host's permitted network reach, not a host-name allowlist; see the [boundaries](../runner/security.md#isolation-today).

## What you get

- ✅ **The same files:** the [simple and advanced scripts](write-agent-cli.md#try-this-first) and the [Python process](write-agent-process.md#try-this-first) run unchanged; only a package description, `agent.json`, is added beside each.
- ✅ **Help from the package:** the runner answers a simple package's help from its description; the advanced one sets `"help":"native"` so its own script shapes the answer from live Configuration.
- ✅ **A process that comes back:** the kept process serves itself, and the runner restarts it if it dies.
- ✅ **Inspect confinement and limits:** the runner's status reports each agent's sandbox and limits.
- 🔑 **Per-message scripts hold no bus credential;** only the process that serves itself is handed its own token ([what runs as what](../runner/security.md#what-runs-as-what)).
- 🚫 **A package folder is code:** copying one in is a host change, so it takes the rights the folder needs.

## Try this first

- Use an [installed user-account runner](../runner/setup.md#a-persons-user-account-runner) you manage. Save the three [example package folders](../../src/clients/examples/agent-guide/packages) and the [Python client](../../src/clients/python/agent_bus); no bus build is needed. Run the copy commands from the folder containing `time-simple`, `time-advanced` and `time-process`.
- Set `AGENTS` to your runner's writable agent directory, `CLIENTS` to the downloaded clients directory, `HOST` to its host folder containing `agents.json` and `tokens/`, and `RUNNER` to its bus name. Set `REALM` to that runner's configured realm, not automatically the host name. These examples assume a nonempty realm.
- Merge `"enabled":true` entries for the three package names into the existing [inventory](../runner/configuration.md#inventory); preserve other entries. Keep `time-advanced` at one concurrent worker because its file updates are not transactional.
- Use unused package names and records for this walkthrough. Copy the packages and Python library, preserving executable permissions:

<!-- run: runner -->
```sh
cp -R time-simple time-advanced time-process "$AGENTS/"
cp -R "$CLIENTS/python/agent_bus" "$AGENTS/time-process/agent_bus"
```

- Register the process as its Owner and hand its token to the runner once. Keep the token directory private and readable by the runner account. Reload after the inventory and files are ready:

<!-- run: runner -->
```sh
agent-bus register "#time-process@$REALM" --kind agent >/dev/null
(umask 077; agent-bus token "#time-process@$REALM" > "$HOST/tokens/%23time-process@$REALM")
agent-bus call "$RUNNER" reload >/dev/null
agent-bus ls "#time-simple@$REALM"
agent-bus ls "#time-process@$REALM"
```

- If the expected records are absent, inspect runner status and logs before continuing. Avoid unbounded polling loops; set a deadline and stop on configuration or permission errors. A zero reader count can mean a busy worker, not failed startup.
- Call the simple service:

<!-- run: runner -->
```sh
agent-bus call "#time-simple@$REALM" epoch
```

- Ask the advanced one what it does. Here `jq -r .body` extracts its native help text for comparison:

<!-- run: runner -->
```sh
agent-bus call "#time-advanced@$REALM" --help | jq -r .body
```

<!-- output -->
```text
time [now|iso|epoch] - the current UTC time, now in the configured format; history - the last five reports
```

- Set `format` (`iso` by default, or `epoch`) and a disposable demonstration `TIME_STAMP_KEY`, then reload for the next start:

<!-- run: runner -->
```sh
STAMP_KEY=a-private-stamp-key
for a in time-advanced time-process; do
  agent-bus config "#$a@$REALM" '{"format":"epoch"}' >/dev/null
  agent-bus secret "#$a@$REALM" "TIME_STAMP_KEY=$STAMP_KEY" >/dev/null
done
agent-bus call "$RUNNER" reload >/dev/null
agent-bus ls "#time-process@$REALM"
```

- Ask each for the time in the configured format:

<!-- run: runner -->
```sh
agent-bus call "#time-advanced@$REALM" now
agent-bus call "#time-process@$REALM" now
```

## Further reading

- 📦 **Package metadata:** [options](../runner/configuration.md#package-options) and [help](../runner/configuration.md#package-help).
- 🪪 **Serving itself:** [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves) say what is handed over and how a stop behaves.
- 🧭 **Before using an agent in production:** [what a good agent does](../10-writing-an-agent.md).
