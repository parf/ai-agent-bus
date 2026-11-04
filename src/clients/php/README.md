# PHP client

📌 **TL;DR:**
- **Problem:** a PHP script that should take part on the bus has to speak HTTP, match replies and avoid repeating an uncertain write.
- **Outcome:** `Bus` gives you send, call, serve, publish, key-value, locks and read-only settings; PHP 8.1+ with ext-curl, ext-json and ext-mbstring only.
- **Start:** start with the [isolated examples](../examples/README.md#try-this-first), or adapt [the snippets](#try-this-first) to your bus.

## Why you'd want it

- 🧰 **Direct loading.** Blocking calls over the unix socket or HTTP(S); load it with one `require` or Composer.
- 🔑 **Credentials stay out of your code.** The environment defaults match the CLI.
- 🛡️ **No silent repeats.** An uncertain write or consumption is never replayed.

## What you get

- ✅ **Call and serve:** `$bus->call($to, $body, new CallOptions(timeout: 10))` returns an `Answer`; `$bus->serve($handler)` answers messages one at a time.
- ✅ **Targets:** `$bus->record($name)` gives that record's `kv`, `locks`, `config()` and `secret()`; `$bus->me()->config()` reads your own Configuration.
- ✅ **Your own name works:** a call to yourself from inside a handler is answered by that handler.
- ✅ **Pools:** `ServeOptions(share: true)` makes several processes one serving pool.
- ✅ **Typed errors:** `Refusal` subclasses by HTTP status; `Transport`, `Timeout` and `UncertainOutcome` otherwise.
- ✅ **Protocol:** `Refusal::kind()` is the daemon's refusal kind. The first call reads `GET /identity`: a daemon older than the client adds a line to `Bus::warnings()`, a client older than the daemon's `protocol_min` gets `DaemonTooNew`, a malformed answer `ProtocolError`.

## Try this first

Use PHP 8.1 or later with curl, json and mbstring extensions. Save the two files below in the checkout root; their `require` loads the local library. In separate shells, use the credentials for the indicated names. Grant `#caller@localhost` permission to send to `#mailer@localhost`.

Serve one handler under one name and call it from another (a disposable bus; both names registered, each process's `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN` and `AGENT_BUS_NAME` set for its own name):

```php
<?php
// serve.php, as #mailer@localhost
require 'src/clients/php/src/autoload.php';
use AgentBus\{Bus, Message, ServeOptions};

$bus = Bus::connect(as: '#mailer@localhost');
$bus->serve(fn(Message $m) => "sent to {$m->body}", new ServeOptions(topic: 'mail'));
```

```php
<?php
// call.php, as #caller@localhost
require 'src/clients/php/src/autoload.php';
use AgentBus\{Bus, CallOptions, Answer};

$bus = Bus::connect(as: '#caller@localhost');
$o = $bus->call('#mailer@localhost', 'bob@example.com', new CallOptions(timeout: 10, topic: 'mail'));
if ($o instanceof Answer) { echo $o->message->body, "\n"; }
echo $bus->record('#caller@localhost')->kv->inc('sent', 1), "\n";
```

Run `php serve.php` in the mailer shell, then `php call.php` in the caller shell; stop the server with Ctrl-C. On a fresh caller record the output is:

```text
sent to bob@example.com
1
```

## Event logs

- 📜 **Calls to `#bus`:** use ordinary calls for the daemon's [agent logs](../../../docs/agents/bus.md#agent-logs).
- ▶️ **Executed examples:** [event_log_example.php](tests/event_log_example.php) and [event_subscription_example.php](tests/event_subscription_example.php) run against a private fixture.
- ✍️ **What they show:** the agent writes its own log; its Owner reads, lists and subscribes, consumes one copy from the authorized queue, and cancels.
- 🔕 **A copy is a notice:** it is a non-waiting message from `#bus`, not a call reply. Send no ack or done for it ([receipts](../../../docs/04-messaging.md#receipts)).

## Technical details

- **Package:** see [the PHP API source](src/Bus.php); [authentication](../../../docs/02-access.md#what-a-call-carries) owns credentials; [MIT license](LICENSE.md).
- **Identity:** the credential's name calls. `as:` only checks it, throwing `IdentityMismatch` on the first call.
- **Secret:** `secret()` holds the bytes as delivered; `env()` is a single-line `NAME=value` reader, not the runner's dotenv grammar: a ` # note` stays in the value and a quoted value cannot span lines, so parse a PEM or multi-line value from `bytes()`.
- **Self-calls:** outside `serve()` a call that takes its own request throws, since nothing here can answer it.
- **Not here:** administration, minting tokens, writing config or secrets, resource content, a pinned `https://` daemon.

## Checks

From the repository root:

```sh
bash src/clients/check.sh php
```

- The check runs PHP with `-d log_errors=0 -d display_errors=stderr` and an empty `SASL_PATH`, so nothing reaches the host log. `tests/daemon.sh` starts a throwaway daemon from the checkout's programs under `tmp/`; it never uses the live bus.
- Child processes cover serving, a shared pool, SIGTERM, a lock waiter and a server that hangs up mid-call.
- The [shared request/answer cases](../../testdata/client-conformance/README.md) are also run by the PHP check, as in every other language.
