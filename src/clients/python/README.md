# Python client

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

Synchronous, thread-safe client for the existing daemon API. Python 3.11 or
later; standard library only. Package version follows the repository VERSION.
The [API plan](../../../Plans/R1.0-Release/done/client-libraries/python.md#surface)
owns the interface; [authentication](../../../docs/02-access.md#what-a-call-carries)
owns credentials. Licensed under the [MIT license](LICENSE.md).

```python
from agent_bus import Bus, CallReply

with Bus("#worker@team", token=worker_token) as agent:
    config = agent.config                 # lazy, blocking, copied from cache
    jobs = agent.record("jobs@team")      # target; caller stays the same
    jobs.kv.inc("attempts", by=1)
    with jobs.locks.acquire("build", ttl=30):
        answer = agent.call("#builder@team", "build", timeout=10)
        if isinstance(answer, CallReply):
            print(answer.message.body)
    agent.serve(handler, workers=4, stop=stop_event)
```

`Bus(name)` verifies the actual credential at the first authenticated operation;
constructing it neither connects nor registers. Environment defaults match the
CLI. Unix sockets, HTTP and HTTPS are supported; `AGENT_BUS_TLS_FINGERPRINT` pins
the peer certificate before sending credentials.

Private values are cached until `refresh()`. Copies already returned survive
revocation. No setters are exposed. Secret representations are redacted;
`reveal()` explicitly returns bytes. Do not log revealed values.

One dispatcher owns call/serve polling. Raw consume is exclusive with those
modes. Call callbacks run on the calling thread; handlers run on bounded threads.
Serve skips requests whose caller deadline has already passed before acknowledgment.
It stops intake, drains to its deadline and reports unfinished handlers;
Python cannot forcibly stop a running handler thread. Each client uses its own
call-tag prefix to discard late replies, including legacy replies without `re`,
without retaining completed calls. Normal replies include their request's `re`.
Lock helpers have no fencing token. Their expiry is a conservative estimate;
an uncertain write or consumption is never replayed automatically.

## Checks

From the repository root:

```sh
(cd src && go build -o ../tmp/client-fixture ./clients/fixture)
PYTHONPATH=src/clients/python python3 -m unittest discover -s src/clients/python/tests -v
```

Tests launch an isolated real daemon fixture, plus fault and TLS HTTP servers.
They need Go and OpenSSL, and never use the live bus. Build a wheel locally with
`TMPDIR="$PWD/tmp" python3 -m pip wheel --no-deps --no-build-isolation --wheel-dir tmp/python-wheels src/clients/python`.
