# Operations extensions

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Reload

Zero-downtime reload for `agent-busd` itself via socket inheritance
(`cloudflare/tableflip`-style); 2× RAM during the overlap.

## Runner unit

`/etc/systemd/system/agent-bus-runner.service` is the other half, and what it
does *not* say is most of it:

| The runner's unit says | So that |
|---|---|
| `User=agent-bus-runner` | it is the other secret domain, and cannot read the daemon's home |
| **which bus to use, defaulting to the local one** | the runner is a client, so the bus it serves is a setting rather than an assumption. A host with no daemon points it elsewhere and nothing else changes ([runner § where it runs](runner.md#where-it-runs)) |
| `WorkingDirectory` is its own home, and that alone is writable | `service.d` is a checkout it only reads, and the daemon's home is not on any path it has |
| `Restart=on-failure` | same reason, and it restarts *its own* children itself rather than leaving them to systemd |
| **no `AmbientCapabilities`** | the one capability on this host belongs to the supervisor, and the runner is not it |
| **against a local bus**: `Wants=agent-busd.service` and `After=` it | the two come up together and in the right order, which is what a local runner depends on. Not `Requires=`: that would stop the runner — and so every service it holds — whenever the bus is stopped, and a bus that is away is not a service that failed ([runner § where it runs](runner.md#where-it-runs)) |
| **against a remote bus**: neither | there is nothing on this host to order against, and the unit is the same file otherwise |

**The runner is a bus citizen like anyone else**, which has a consequence
worth stating: reaching the local bus over the socket means it is a *mapped
local account* like every other ([local users](../../docs/09-setup.md#local-users)), so the install
maps it and the daemon opens it a socket. Nothing about the runner is special
to the daemon — which is the whole claim of the split, made concrete.

⚠️ The runner's unit ships **with the runner**, in R1
([stages § R1](README.md#scope)) — there is no point writing a
unit for a program that is not installed. What exists today is everything it
will need: both accounts, the tree they own, and the runner's socket.
