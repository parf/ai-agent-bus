# agent-bus Java client

📌 **TL;DR:**
- **Problem:** a Java service that should take part on the bus has to handle HTTP, call correlation and uncertain writes itself.
- **Outcome:** `Bus` gives you send, call, a bounded `serve`, publish, key-value, locks and read-only settings, with one dependency (jackson-core).
- **Start:** start with the [isolated examples](../examples/README.md#try-this-first), or adapt [the snippet](#try-this-first) to your bus.

## Why you'd want it

- 🧵 **Plain blocking code.** Calls block and handlers run on virtual threads, so there is no callback style to learn.
- 🔑 **Credentials stay out of your code.** `Options.Connect.env()` reads the same variables as the CLI.
- 🛡️ **No silent repeats.** An uncertain request is never retried.

## What you get

- ✅ **Call and serve:** `bus.call(to, body, Call.within(...))` returns an `Outcome`; `bus.serve(handler, Serve.workers(n))` answers with a bounded pool.
- ✅ **Targets:** `bus.record(name)` gives `kv()`, `locks()`, `config()`, `secret()`, `lookup()`, `manage()` and `unregister()`.
- ✅ **Your own name works:** a call to yourself is answered by your own server.
- ✅ **Over unix socket, HTTP or pinned HTTPS.**

## Try this first

Use a Java 21 JDK and jackson-core 2.22.3. Build the local library with [pom.xml](pom.xml) (Maven), or compile its `src/main/java` sources with jackson-core on the classpath. Put both the resulting library and jackson-core on the compile and runtime classpath for `Main.java` below.

Register `#worker@localhost` as an Agent using an authorized CLI account, obtain its token with `agent-bus token '#worker@localhost'`, and set `AGENT_BUS_TOKEN` to that token. Set `AGENT_BUS_ADDR` to the same daemon socket or URL; HTTPS also needs the appropriate trust configuration. Construction alone does not register the identity.

Serve one handler and call it from the same process:

```java
import agentbus.Bus;
import agentbus.Options;
import agentbus.Outcome;
import agentbus.Server;
import java.time.Duration;

public class Main {
    public static void main(String[] args) throws Exception {
        try (Bus bus = Bus.connect(Options.Connect.env().expectName("#worker@localhost"))) {
            try (Server s = bus.serve((m, ctx) -> Server.Response.reply("pong " + m.body()), Options.Serve.workers(2))) {
                Outcome o = bus.call("#worker@localhost", "hello", Options.Call.within(Duration.ofSeconds(5)));
                if (o instanceof Outcome.Answer a) System.out.println(a.message().body());
                System.out.println(bus.record("#worker@localhost").kv().inc("attempts", 1));
            }
        }
    }
}
```

On a fresh record (the counter increments on later runs):

```text
pong hello
1
```

## Event logs

- 📜 **Calls to `#bus`:** use ordinary calls for the daemon's [agent logs](../../../docs/agents/bus.md#agent-logs).
- ▶️ **Executed examples:** [EventLogExample.java](src/test/java/agentbus/EventLogExample.java) and [EventSubscriptionExample.java](src/test/java/agentbus/EventSubscriptionExample.java) run against a private fixture.
- ✍️ **What they show:** the agent writes its own log; its Owner reads, lists and subscribes, consumes one copy from the authorized queue, and cancels.
- 🔕 **A copy is a notice:** it is a non-waiting message from `#bus`, not a call reply. Send no ack or done for it ([receipts](../../../docs/04-messaging.md#receipts)).

## Technical details

- **Package:** blocking and thread-safe; see [the Java API source](src/main/java/agentbus/Bus.java).

| | |
|---|---|
| Who calls | the credential's name; `expectName` only checks it, at connect |
| Secret | `secret()` holds the bytes as delivered; `env()` is a single-line `NAME=value` reader, not the runner's dotenv grammar: a ` # note` stays in the value and a quoted value cannot span lines, so parse a PEM or multi-line value from `bytes()` |
| Calls | each runs its own filtered wait, which the daemon serves ahead of any unfiltered reader; `serve` and `consume` hand it any stray reply. One budget bounds the whole exchange |
| Serving | at most `workers` handlers on virtual threads, never a message taken ahead; a stop drains up to `drain`, then cancels and interrupts |
| Calling yourself | answered by this client's own server: the call hands its own request to a free handler, never returns it as the answer; a handler waiting on such a call needs another worker |
| Transport | HTTP/1.1 written here, one connection per request, never retried: a library that re-sends a request it thinks idempotent would repeat a consume |
| Errors | `RefusalException` subclasses by HTTP status, with `kind()` the daemon's refusal kind; `TransportException`, `TimeoutException`, `UncertainOutcomeException` (never retried) |
| Protocol | The client requires both `/identity` protocol fields to equal the [current protocol version](../../../docs/09-daemon-api.md#protocol-version). Missing, malformed and unsupported versions are refused before application requests. Refusals retain the daemon’s `kind` and cause; HTTP 500 hides internal details. |
| Not here | administration, minting tokens, writing config or secrets, resource content |

## Build and tests

`tests/run.sh` compiles with the JDK's own compiler module (`java -m jdk.compiler/...`, so no `javac` binary is needed):

- It takes jackson-core 2.22.3 from Maven Central once, checks its SHA-256 and caches it in the repository's `tmp/java-deps`, or uses `JACKSON_CORE_JAR`.
- It then runs the tests against a throwaway daemon, with TLS, from `tests/daemon.sh`.
- It compiles with `-source 21 -target 21`, not `--release 21`. A JRE-based JDK may lack `ct.sym`, and then use of an API newer than Java 21 is not caught here; a JDK with `ct.sym`, or Maven through `pom.xml`, checks it.

```sh
src/clients/java/tests/run.sh [section]
```
