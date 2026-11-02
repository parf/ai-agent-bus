# Your script on the bus: the time service with `agent-bus run`

📌 **TL;DR:**
- **Problem:** a shell script answers only you; letting others call it by name means writing message code.
- **Outcome:** a six-line time service you start with one command and call by name, then the same service with settings, a key, help and state.
- **Start:** save the script below, run it, and call it from a second terminal ([Try this first](#try-this-first)).

## Why you'd want it

- 🙋 **Only you can run your script.** Now it has a name that allowed callers use from any host on the bus.
- 🧑‍💻 **Message code is a chore.** Now the script reads one argument and prints its answer.
- 🧪 **You want to see it work before installing anything.** Now one foreground command is the whole setup.

## What you get

- ✅ **A named agent in one command:** `agent-bus run` registers the name, reads the inbox and runs your script once per message.
- ✅ **Help without effects:** an ask such as help reaches the script as `AGENT_BUS_ASK`; the script answers it and does nothing else.
- ✅ **The advanced form:** [settings and a key](#the-advanced-service) kept out of the script, a little state, and a status answer.
- ✅ **Runs as the same service elsewhere:** the [runner](write-agent-runner.md) packages these two scripts unchanged; a [process of your own](write-agent-process.md) does the same work with a client library.
- 🚫 **You keep it alive:** the command runs in your terminal; close it and the agent is gone.

## Try this first

- Save [time-simple.sh](../../src/clients/examples/agent-guide/packages/time-simple/time-simple.sh) and make it executable (the file begins with `#!/bin/sh` and a comment; its working part is below), then start it in one terminal:

<!-- quote: src/clients/examples/agent-guide/packages/time-simple/time-simple.sh simple -->
```sh
case "${AGENT_BUS_ASK:-}" in
  "") ;;                                         # an ordinary request: carry on
  help) echo "time [iso|epoch] - the current UTC time"; exit 0 ;;
  *) echo "unsupported: $AGENT_BUS_ASK"; exit 0 ;;
esac
case "${1:-iso}" in
  iso) date -u +%Y-%m-%dT%H:%M:%SZ ;;
  epoch) date -u +%s ;;
  *) echo "unsupported format; ask help" ;;      # the body is data, never echoed back
esac
```

- Leave it running; it prints nothing until a message arrives:

<!-- run: cli-simple ready=#time-simple -->
```sh
agent-bus run '#time-simple' "$PWD/time-simple.sh"
```

- In a second terminal, ask for the time:

<!-- run: cli-simple -->
```sh
agent-bus call '#time-simple' iso | jq -r .body
```

- Ask what it does, and send a body that is not a format. The body is data: it is never echoed back and nothing in it runs:

<!-- run: cli-simple -->
```sh
agent-bus call '#time-simple' --help | jq -r .body
agent-bus call '#time-simple' 'rm -rf /' | jq -r .body
```

<!-- output -->
```text
time [iso|epoch] - the current UTC time
unsupported format; ask help
```

## The advanced service

- 🔑 **Settings and keys stay out of the script.** Configuration holds the format; the Secret holds a stamp key that the script uses and never prints.
- 🗂️ **State lives in the work folder:** a report count and the last five times, which a restart keeps with `--stay-registered`.
- 📖 **It shapes its own help and status,** so help can name the history word.
- Save [time-advanced.sh](../../src/clients/examples/agent-guide/packages/time-advanced/time-advanced.sh) the same way; its working part is below. Run it kept registered:

<!-- quote: src/clients/examples/agent-guide/packages/time-advanced/time-advanced.sh advanced -->
```sh
case "${AGENT_BUS_ASK:-}" in
  "") ;;
  help) echo "time [now|iso|epoch] - the current UTC time, now in the configured format; history - the last five reports"; exit 0 ;;
  status)
    echo "reports: $(cat "$AGENT_BUS_WORK/reports" 2>/dev/null || echo 0); format: $(printf '%s' "${AGENT_BUS_CONFIG:-}" | sed -n 's/.*"format":"\([a-z]*\)".*/\1/p' | grep . || echo iso)"
    exit 0 ;;
  *) echo "unsupported: $AGENT_BUS_ASK"; exit 0 ;;
esac
format=$(printf '%s' "${AGENT_BUS_CONFIG:-}" | sed -n 's/.*"format":"\([a-z]*\)".*/\1/p' | grep . || echo iso)
word=${1:-now}; [ "$word" = now ] && word=$format
case "$word" in
  history) cat "$AGENT_BUS_WORK/history" 2>/dev/null || echo "no reports yet"; exit 0 ;;
  iso) now=$(date -u +%Y-%m-%dT%H:%M:%SZ) ;;
  epoch) now=$(date -u +%s) ;;
  *) echo "unsupported format; ask help"; exit 0 ;;
esac
# Work folder: lasting state. The stamp proves the Secret was used, never shows it.
stamp=$(printf '%s%s' "${TIME_STAMP_KEY:-}" "$now" | sha256sum | cut -c1-8)
echo $(( $(cat "$AGENT_BUS_WORK/reports" 2>/dev/null || echo 0) + 1 )) >"$AGENT_BUS_WORK/reports"
{ echo "$now"; head -n 4 "$AGENT_BUS_WORK/history" 2>/dev/null || true; } >"$AGENT_BUS_WORK/history.new" && mv "$AGENT_BUS_WORK/history.new" "$AGENT_BUS_WORK/history"
echo "$now stamp=$stamp"
```

<!-- run: cli-advanced ready=#time-advanced -->
```sh
agent-bus run '#time-advanced' "$PWD/time-advanced.sh" --stay-registered
```

- Set the format and the key; the script reads both at its next message:

<!-- run: cli-advanced -->
```sh
STAMP_KEY=a-private-stamp-key
agent-bus config '#time-advanced' '{"format":"epoch"}' >/dev/null
agent-bus secret '#time-advanced' "TIME_STAMP_KEY=$STAMP_KEY" >/dev/null
```

- Ask for the status; nothing has been reported yet:

<!-- run: cli-advanced -->
```sh
agent-bus call '#time-advanced' --status | jq -r .body
```

<!-- output -->
```text
reports: 0; format: epoch
```

- Ask for the time. The word `now` means the configured format, and the stamp proves the key was used without showing it:

<!-- run: cli-advanced -->
```sh
agent-bus call '#time-advanced' now | jq -r .body
```

## Technical details

- 📜 **Script rules:** how the CLI hands over the message, the ask and the work folder: [script agents](../06-dedicated-runner.md#script-agents).
- 🔐 **A per-message script holds no bus credential;** only a [process that serves itself](write-agent-process.md) does ([what runs as what](../runner/security.md#what-runs-as-what)).
- 🧩 **Settings and Secret:** the split and where each lives: [Configuration](../03-records-agent.md#configuring-a-template).
- ✅ **What the shard checks:** `src/acceptance/agent-guide/guide.sh` runs these exact files and the commands above; a quoted block that drifts from its source fails there.
- 🧭 **The checklist these examples follow:** [what a good agent does](../10-writing-an-agent.md).
