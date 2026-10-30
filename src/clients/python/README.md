# Python client

📌 **TL;DR:**
- **Problem:** a script that should take part on the bus has to speak HTTP, match replies and avoid repeating an uncertain write.
- **Outcome:** `Bus` gives you send, call, serve, key-value, locks and read-only settings with no dependencies beyond the standard library.
- **Start:** copy [the example](#try-this-first); it needs a name, a token and a running bus.

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

## Why you'd want it

- 🧰 **Nothing to install.** Blocking calls and bounded handler threads, with no dependencies.
- 🔑 **Credentials stay out of your code.** The environment defaults match the CLI.
- 🛡️ **No silent repeats.** An uncertain write or consumption is never replayed.

## What you get

- ✅ **Call and serve:** `bus.call(to, body, timeout=...)` returns a reply; `bus.serve(handler, workers=...)` answers messages with a bounded pool.
- ✅ **Targets:** `bus.record(name)` gives that record's `kv` and `locks`; `bus.config` reads the agent's Configuration.
- ✅ **Your own name works:** a call to yourself is answered by your own handler.
- ✅ **Safe secrets:** representations are redacted; `reveal()` returns bytes only when you ask.

## Try this first

Serve one handler and call it from the same process (a disposable bus, token in `AGENT_BUS_TOKEN`):

```python
import threading
from agent_bus import Bus, CallReply

stop = threading.Event()
with Bus("#worker@localhost") as bus:
    server = threading.Thread(target=bus.serve, args=(lambda m: "pong " + m.body,), kwargs={"workers": 2, "stop": stop})
    server.start()
    answer = bus.call("#worker@localhost", "hello", timeout=5)
    if isinstance(answer, CallReply):
        print(answer.message.body)
    print(bus.record("#worker@localhost").kv.inc("attempts", by=1))
    stop.set()
    server.join()
```

```text
pong hello
1
```

- ▶️ The [shared examples](../examples/README.md) run agents in every language together.

## Technical details

- **Package:** Python 3.11 or later, standard library only; the [API plan](../../../Plans/R1.0-Release/done/client-libraries/python.md#surface) owns the interface; [authentication](../../../docs/02-access.md#what-a-call-carries) owns credentials; the version follows the repository VERSION; [MIT license](LICENSE.md).
- **Connecting:** `Bus(name)` verifies the credential at the first authenticated operation; constructing it neither connects nor registers. Unix sockets, HTTP and HTTPS work, and `AGENT_BUS_TLS_FINGERPRINT` pins the peer certificate before credentials are sent.
- **Private values:** cached until `refresh()`; copies already returned survive revocation; no setters; secret representations are redacted, and `reveal()` returns bytes (do not log them).
- **Dispatch:** one dispatcher owns call and serve polling, and raw consume is exclusive with those modes. Call callbacks run on the calling thread; handlers run on bounded threads.
- **Serve:** it skips requests whose caller deadline has passed before acknowledgment, stops intake, drains to its deadline and reports unfinished handlers. Python cannot forcibly stop a running handler thread.
- **Replies:** each client has its own call-tag prefix to discard late replies, including legacy replies without `re`, without retaining completed calls. Normal replies include their request's `re`.
- **Locks and uncertainty:** lock helpers have no fencing token and their expiry is a conservative estimate; an uncertain write or consumption is never replayed automatically.
- **Self-calls:** they use the same serving handler. The unmarked request is never its own answer, even if it arrives before the send response; it waits for a free handler slot, and an already-taken self request joins graceful drain. A handler making a blocking call to itself needs another available worker. Legacy replies with a different message ID remain accepted.

## Checks

From the repository root:

```sh
(cd src && go build -o ../tmp/client-fixture ./clients/fixture)
PYTHONPATH=src/clients/python python3 -m unittest discover -s src/clients/python/tests -v
```

- Tests launch an isolated real daemon fixture, plus fault and TLS HTTP servers.
- They need Go and OpenSSL, and never use the live bus.
- Build a wheel locally with `TMPDIR="$PWD/tmp" python3 -m pip wheel --no-deps --no-build-isolation --wheel-dir tmp/python-wheels src/clients/python`.
- The [shared request/answer cases](../../testdata/client-conformance/README.md) are also run by the Python check, as in every other language.
