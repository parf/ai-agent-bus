# PHP client

📌 **TL;DR:**
- **Problem:** a PHP script that should take part on the bus has to speak HTTP, match replies and avoid repeating an uncertain write.
- **Outcome:** `Bus` gives you send, call, serve, publish, key-value, locks and read-only settings; PHP 8.1+ with ext-curl, ext-json and ext-mbstring only.
- **Start:** copy [the example](#try-this-first); it needs a registered name, its token and a running bus.

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

## Why you'd want it

- 🧰 **Nothing to install.** Blocking calls over the unix socket or HTTP(S); load it with one `require` or Composer.
- 🔑 **Credentials stay out of your code.** The environment defaults match the CLI.
- 🛡️ **No silent repeats.** An uncertain write or consumption is never replayed.

## What you get

- ✅ **Call and serve:** `$bus->call($to, $body, new CallOptions(timeout: 10))` returns an `Answer`; `$bus->serve($handler)` answers messages one at a time.
- ✅ **Targets:** `$bus->record($name)` gives that record's `kv`, `locks`, `config()` and `secret()`; `$bus->me()->config()` reads your own Configuration.
- ✅ **Your own name works:** a call to yourself from inside a handler is answered by that handler.
- ✅ **Pools:** `ServeOptions(share: true)` makes several processes one serving pool.
- ✅ **Typed errors:** `Refusal` subclasses by HTTP status; `Transport`, `Timeout` and `UncertainOutcome` otherwise.

## Try this first

Serve one handler in a child process and call it (a disposable bus; `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN` and `AGENT_BUS_NAME` set for `#mailer@localhost`):

```php
require 'src/clients/php/src/autoload.php';
use AgentBus\{Bus, Message, ServeOptions, CallOptions, Answer};

$bus = Bus::connect(as: '#mailer@localhost');
$pid = pcntl_fork();
if ($pid === 0) {
    $bus->serve(fn(Message $m) => "sent to {$m->body}", new ServeOptions(topic: 'mail'));
    exit(0);
}
$o = $bus->call('#mailer@localhost', 'bob@example.com', new CallOptions(timeout: 10, topic: 'mail'));
if ($o instanceof Answer) { echo $o->message->body, "\n"; }
echo $bus->record('#mailer@localhost')->kv->inc('sent', 1), "\n";
posix_kill($pid, SIGTERM);
pcntl_waitpid($pid, $status);
```

```text
sent to bob@example.com
1
```

- ▶️ The [shared examples](../examples/README.md) run agents in every language together.

## Technical details

- **Package:** PHP 8.1 or later; the [PHP plan](../../../Plans/R1.0-Release/done/client-libraries/php.md) owns the interface; [authentication](../../../docs/02-access.md#what-a-call-carries) owns credentials; [MIT license](LICENSE.md).
- **Identity:** the credential's name calls. `as:` only checks it, throwing `IdentityMismatch` on the first call.
- **Secret:** `secret()` holds the bytes as delivered; `env()` is a single-line `NAME=value` reader, not the runner's dotenv grammar: a ` # note` stays in the value and a quoted value cannot span lines, so parse a PEM or multi-line value from `bytes()`.
- **Serving:** one message at a time per process; parallelism is more processes.
- **Self-calls:** outside `serve()` a call that takes its own request throws, since nothing here can answer it.
- **Uncertainty:** `UncertainOutcome` is never retried.
- **Not here:** administration, minting tokens, writing config or secrets, resource content, a pinned `https://` daemon.

## Checks

From the repository root:

```sh
src/clients/php/tests/daemon.sh php -d log_errors=0 -d display_errors=stderr src/clients/php/tests/run.php [section]
```

- `tests/daemon.sh` starts a throwaway daemon from the checkout's programs under `tmp/`; it never uses the live bus.
- Child processes cover serving, a shared pool, SIGTERM, a lock waiter and a server that hangs up mid-call.
- The [shared request/answer cases](../../testdata/client-conformance/README.md) are also run by the PHP check, as in every other language.
