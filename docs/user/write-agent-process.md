# A program of your own on the bus: the time service with the Python client

📌 **TL;DR:**
- **Problem:** an application that already runs wants to answer bus messages and keep progress, without a script launcher in between.
- **Outcome:** the same time service as a Python process that reads its own inbox, records progress in KV, uses a short lock and writes events to its own log.
- **Start:** register the name, start the program with its credential, and call it ([Try this first](#try-this-first)).

## Why you'd want it

- 🏠 **The logic already lives in your application.** Now it answers by name from inside it.
- 💾 **Memory is lost on restart.** Progress sits in KV; a lock coordinates the two workers while its lease remains valid.
- 🧾 **Operators want to know what happened.** Now each report writes an event to the agent's own log.

## What you get

- ✅ **One serve loop:** the client reads the inbox, runs your handler on bounded workers and sends each answer back to the right caller.
- ✅ **Help, status and the same body rules** as the [script version](write-agent-cli.md#the-advanced-service): the body must be one of the supported time or history commands.
- ✅ **Settings and a key:** Configuration for the format, Secret for the stamp key, read through the client and never echoed.
- ✅ **A bounded drain:** SIGTERM stops intake and allows five seconds for outstanding work; it does not forcibly cancel a Python handler.
- 🪪 **Your own credential:** unlike a per-message script, a process that serves itself holds its own token ([what runs as what](../runner/security.md#what-runs-as-what)).
- 🔁 **Run it under the runner** when it should stay up without you: [the runner page](write-agent-runner.md).
- A daemon disconnect can end `serve()` with an uncertain-consumption error. Supervise and restart the process; never automatically replay that consumption ([client uncertainty](../../src/clients/python/README.md#technical-details)).

## Try this first

- The program is [time_service.py](../../src/clients/examples/agent-guide/packages/time-process/time_service.py): save the whole file (it adds the imports, the help text and the `serve()` call) next to the client's `agent_bus` folder, which needs no installation ([clients](../../src/clients/README.md#what-you-get)). The [working part](#example-source) is reproduced below.

- Register the name; you are its Owner:

<!-- run: process -->
```sh
agent-bus register '#time-process' --kind agent
```

- Start it with its own credential, and leave it running:

<!-- run: process ready=#time-process -->
```sh
AGENT_BUS_NAME="#time-process" AGENT_BUS_TOKEN=$(agent-bus token '#time-process') python3 time_service.py
```

- In a second terminal, ask what it does, then set `format` (`iso` by default, or `epoch`) and the disposable example `TIME_STAMP_KEY`. This help example uses `jq -r .body` only to show the answer text:

<!-- run: process -->
```sh
STAMP_KEY=a-private-stamp-key
agent-bus call '#time-process' --help | jq -r .body
agent-bus config '#time-process' '{"format":"epoch"}' >/dev/null
agent-bus secret '#time-process' "TIME_STAMP_KEY=$STAMP_KEY" >/dev/null
```

<!-- output -->
```text
time [now|iso|epoch] - the current UTC time, now in the configured format; history - the last five reports
```

- Ask for the time in the configured format; the full JSON answer contains the time and its demonstration stamp:

<!-- run: process -->
```sh
agent-bus call '#time-process' now
```

- Read the progress the process kept in KV:

<!-- run: process -->
```sh
agent-bus kv get '#time-process' reports --int
```

<!-- output -->
```text
1
```

## Example source

<details>
<summary>Time service handler and serve loop</summary>

<!-- quote: src/clients/examples/agent-guide/packages/time-process/time_service.py process -->
```python
def serve():
    stop = threading.Event()
    signal.signal(signal.SIGTERM, lambda *_: stop.set())     # graceful stop: finish what is running, then leave
    with Bus() as bus:                                        # name, token and address come from the environment
        kv, locks = bus.kv, bus.locks

        def read(key, kind="string"):                         # a missing key is not an error here
            try:
                return kv.get(key, kind=kind)
            except Refusal as refused:
                if refused.kind == "not_found":
                    return None
                raise

        def settings():
            fmt = (bus.config or {}).get("format", "iso")     # Configuration: non-secret settings
            key = dict(line.split("=", 1) for line in bus.secret.reveal().decode().splitlines() if "=" in line)
            return fmt, key.get("TIME_STAMP_KEY", "")         # Secret: read, used, never echoed

        def report(word):
            fmt, key = settings()
            word = word or "now"
            if word == "now":
                word = fmt
            if word == "history":
                return read("history") or "no reports yet"
            now = datetime.datetime.now(datetime.timezone.utc)
            if word == "iso":
                text = now.strftime("%Y-%m-%dT%H:%M:%SZ")
            elif word == "epoch":
                text = str(int(now.timestamp()))
            else:
                return "unsupported format; ask help"        # the body is data, never echoed back
            stamp = hashlib.sha256((key + text).encode()).hexdigest()[:8]
            lease = locks.acquire("report", ttl=5, wait=2)    # one report is recorded at a time
            try:
                kv.inc("reports")
                kv.set("history", "\n".join([text] + (read("history") or "").splitlines()[:4]))
            finally:
                lease.release()
            bus.send("#bus", f"write {bus.name} notice reported {word}")   # an event in my own log; no answer is expected
            return f"{text} stamp={stamp}"

        def handle(message):
            if message.ask == "help":
                return HELP
            if message.ask == "status":
                fmt, _ = settings()
                return f"reports: {read('reports', 'int') or 0}; format: {fmt}"
            if message.ask:
                return "unsupported: " + message.ask
            return report((message.body or "").strip())

        bus.serve(handle, workers=2, stop=stop, drain_timeout=5)
```

</details>

## Limits of the example

- **No transaction or exactly-once result:** the count increment, history update, log event and answer are separate operations. A failure can leave only some of them completed; do not retry an uncertain call blindly.
- **A five-second lease:** the code neither extends the lock nor fences writes after expiry. A slow worker can overlap a later holder. Use a transaction or a design that tolerates partial updates for production state.
- **Narrow Secret parsing:** this example splits lines at `=` without applying the full environment-file grammar. Use a plain `TIME_STAMP_KEY=value` line here; quoted values, comments containing `=` and other syntax need a proper parser.
- **A demonstration stamp:** a short hash of a key and public time is not a signature. Keep the key disposable and use an established authentication construction for real verification.
- **Bounded work matters:** the client's drain timeout can raise while a Python worker is still running. Bound each handler's I/O and execution time; a supervisor must not assume every effect stopped at that timeout.

## Further reading

- 🧩 **The client:** calls, serving, KV, locks and the event write are in the [client libraries](../../src/clients/README.md#what-you-get); other languages use the [HTTP client contract](../09-daemon-api.md#the-client-contract).
- 🔁 **Under the runner:** `"serves": "itself"` keeps it running; its token is handed over once ([agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves)).
- 🛑 **Stopping:** what a stop waits for and what the author owns: [author stop responsibilities](../runner/configuration.md#author-stop-responsibilities).
- 🔐 **Secret and Configuration:** [the split](../03-records-agent.md#configuring-a-template); the Secret is never written to answers, status or logs.
- 📝 **Events:** the process writes only to its own log; reading it is a manager's right ([agent logs](../agents/bus.md#agent-logs)).
- 🧭 **Before using an agent in production:** [what a good agent does](../10-writing-an-agent.md).
