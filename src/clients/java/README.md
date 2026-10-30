# agent-bus Java client

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

📌 **TL;DR:** A blocking, thread-safe Java 21 client for the agent-bus daemon.
It reaches the daemon over its unix socket, HTTP or pinned HTTPS, and covers
send, call, a bounded `serve`, publish, a record's key-value store and locks,
and read-only config and secret. Its one dependency is jackson-core. The design
and its reasons are the [Java plan](../../../Plans/R1.0-Release/done/client-libraries/java.md).

```java
try (Bus bus = Bus.connect(Options.Connect.env().expectName("#indexer@team"))) {
    Record jobs = bus.record("jobs@team");
    try (Server s = bus.serve((m, ctx) -> {
            try (Locks.Lease l = jobs.locks().acquire("shard-" + m.body(), Duration.ofSeconds(30), Duration.ofSeconds(5))) {
                return Server.Response.reply("indexed " + jobs.kv().inc("indexed", 1));
            }
        }, Options.Serve.workers(16).topic("index"))) {
        s.awaitTermination();
    }
}
```

| | |
|---|---|
| Who calls | the credential's name; `expectName` only checks it, at connect |
| A target | `bus.record(name)`: `kv()`, `locks()`, `config()`, `secret()`, `lookup()`, `manage()`, `unregister()` |
| Secret | `secret()` holds the bytes as delivered; `env()` is a single-line `NAME=value` reader, not the runner's dotenv grammar: a ` # note` stays in the value and a quoted value cannot span lines, so parse a PEM or multi-line value from `bytes()` |
| Calls | each runs its own filtered wait, which the daemon serves ahead of any unfiltered reader; `serve` and `consume` hand it any stray reply. One budget bounds the whole exchange |
| Serving | at most `workers` handlers on virtual threads, never a message taken ahead; a stop drains up to `drain`, then cancels and interrupts |
| Calling yourself | answered by this client's own server: the call hands its own request to a free handler, never returns it as the answer; a handler waiting on such a call needs another worker |
| Transport | HTTP/1.1 written here, one connection per request, never retried: a library that re-sends a request it thinks idempotent would repeat a consume |
| Errors | `RefusalException` subclasses by HTTP status only; `TransportException`, `TimeoutException`, `UncertainOutcomeException` (never retried) |
| Not here | administration, minting tokens, writing config or secrets, resource content |

<details>
<summary>Build and tests</summary>

`tests/run.sh` compiles with the JDK's own compiler module
(`java -m jdk.compiler/...`, so no `javac` binary is needed). It takes
jackson-core 2.22.3 from Maven Central once, checks its SHA-256 and caches it
in the repository's `tmp/java-deps`, or uses `JACKSON_CORE_JAR`. It then runs
the tests against a throwaway daemon, with TLS, from `tests/daemon.sh`:

```sh
src/clients/java/tests/run.sh [section]
```

It compiles with `-source 21 -target 21`, not `--release 21`. A JRE-based JDK
may lack `ct.sym`, and then use of an API newer than Java 21 is not caught
here. A JDK with `ct.sym`, or Maven through `pom.xml`, checks it.

</details>
