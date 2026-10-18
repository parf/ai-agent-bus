# agent-bus PHP client

📌 **TL;DR:** A synchronous PHP 8.1+ client for the agent-bus daemon over its
unix socket or HTTP(S): send, call, serve, publish, a record's key-value store
and locks, and read-only config and secret. It has no dependency beyond
ext-curl, ext-json and ext-mbstring. The design and its reasons are the
[PHP plan](../../../Plans/R1.0-Release/done/client-libraries/php.md).

```php
require 'src/clients/php/src/autoload.php';   // or Composer: agent-bus/client
use AgentBus\{Bus, Message, ServeOptions, CallOptions, Answer};

$bus = Bus::connect(as: '#mailer@team');      // AGENT_BUS_ADDR / _TOKEN / _NAME by default
$cfg = $bus->me()->config();
$bus->serve(fn(Message $m) => "sent to {$m->body}", new ServeOptions(topic: 'mail'));

$o = $bus->call('#mailer@team', 'bob@example.com', new CallOptions(timeout: 10, topic: 'mail'));
if ($o instanceof Answer) { echo $o->message->body; }
```

| | |
|---|---|
| Who calls | the credential's name. `as:` only checks it, throwing `IdentityMismatch` on the first call |
| A target | `$bus->record($name)`: `kv`, `locks`, `config()`, `secret()`, `lookup()`, `manage()`, `unregister()` |
| Secret | `secret()` holds the bytes as delivered; `env()` is a single-line `NAME=value` reader, not the runner's dotenv grammar: a ` # note` stays in the value and a quoted value cannot span lines, so parse a PEM or multi-line value from `bytes()` |
| Serving | one message at a time per process. Parallelism is more processes: `ServeOptions(share: true)` makes them a pool |
| Errors | `Refusal` subclasses by HTTP status only; `Transport`, `Timeout`, `UncertainOutcome` (never retried) |
| Not here | administration, minting tokens, writing config or secrets, resource content, a pinned `https://` daemon |

<details>
<summary>Tests</summary>

`tests/daemon.sh` starts a throwaway daemon from the checkout's programs, under
the repository's `tmp/`, and runs a command against it:

```sh
src/clients/php/tests/daemon.sh php src/clients/php/tests/run.php [section]
```

Child processes cover serving, a shared pool, SIGTERM, a lock waiter and a
server that hangs up mid-call.

</details>
