# agent-bus Java client

📌 **TL;DR:**
- **Problem:** a Java service that should take part on the bus has to handle HTTP, call correlation and uncertain writes itself.
- **Outcome:** `Bus` gives you send, call, a bounded `serve`, publish, key-value, locks and read-only settings, with one dependency (jackson-core).
- **Start:** copy [the example](#try-this-first); it needs a name, a token and a running bus.

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

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

Serve one handler and call it from the same process (a disposable bus, token in `AGENT_BUS_TOKEN`):

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

```text
pong hello
1
```

- ▶️ The [shared examples](../examples/README.md) run agents in every language together.

## Technical details

- **Package:** Java 21, blocking and thread-safe; the design and its reasons are the [Java plan](../../../Plans/R1.0-Release/done/client-libraries/java.md).

| | |
|---|---|
| Who calls | the credential's name; `expectName` only checks it, at connect |
| Secret | `secret()` holds the bytes as delivered; `env()` is a single-line `NAME=value` reader, not the runner's dotenv grammar: a ` # note` stays in the value and a quoted value cannot span lines, so parse a PEM or multi-line value from `bytes()` |
| Calls | each runs its own filtered wait, which the daemon serves ahead of any unfiltered reader; `serve` and `consume` hand it any stray reply. One budget bounds the whole exchange |
| Serving | at most `workers` handlers on virtual threads, never a message taken ahead; a stop drains up to `drain`, then cancels and interrupts |
| Calling yourself | answered by this client's own server: the call hands its own request to a free handler, never returns it as the answer; a handler waiting on such a call needs another worker |
| Transport | HTTP/1.1 written here, one connection per request, never retried: a library that re-sends a request it thinks idempotent would repeat a consume |
| Errors | `RefusalException` subclasses by HTTP status only; `TransportException`, `TimeoutException`, `UncertainOutcomeException` (never retried) |
| Not here | administration, minting tokens, writing config or secrets, resource content |

## Build and tests

`tests/run.sh` compiles with the JDK's own compiler module (`java -m jdk.compiler/...`, so no `javac` binary is needed):

- It takes jackson-core 2.22.3 from Maven Central once, checks its SHA-256 and caches it in the repository's `tmp/java-deps`, or uses `JACKSON_CORE_JAR`.
- It then runs the tests against a throwaway daemon, with TLS, from `tests/daemon.sh`.
- It compiles with `-source 21 -target 21`, not `--release 21`. A JRE-based JDK may lack `ct.sym`, and then use of an API newer than Java 21 is not caught here; a JDK with `ct.sym`, or Maven through `pom.xml`, checks it.

```sh
src/clients/java/tests/run.sh [section]
```
