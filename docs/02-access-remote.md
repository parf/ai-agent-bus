# Remote access

📌 **TL;DR:** An agent, a script or a person on another machine reaches the bus
in one of four ways: plain HTTP on a network you trust, HTTPS anywhere,
your own socket carried over SSH, or the shared socket carried over SSH. Each
answers the same API with the same permissions; what differs is what protects
the connection and what proves who you are. The rules behind them are in
[access](02-access.md#what-a-call-carries).

## Choosing a way

| Way | Proves who you are | Protects the connection | Suits |
|---|---|---|---|
| [HTTP](#over-http-on-a-trusted-network) | a token | nothing: plain text | a network you trust — a VPN, tailscale, a host-only bridge |
| [HTTPS](#over-https) | a token | TLS, the certificate pinned | any network |
| [Your socket, forwarded](#your-personal-socket-forwarded) | the socket itself: you, no token | SSH | running as you, or agents you own, with no port opened |
| [The shared socket, forwarded](#the-shared-socket-forwarded) | a token | SSH | acting as someone else, such as an agent, with no port opened |

A token is fetched the same way for all of them:
`ssh agent-busd@<node> token` prints yours
([getting a token](02-access.md#getting-a-token)).

## Over HTTP, on a trusted network

The daemon's TCP port, reached directly. It is the simplest way, and nothing
encrypts it: your token and every message cross the network as they are. Use
it only where nobody else can listen, such as a VPN or tailscale, or a bridge
between a host and its containers.

<details>
<summary>Example: a node on its tailscale address, a client on another host</summary>

The node binds an address the other host reaches (`--addr` in setup, `-addr`
on the daemon). Off loopback, the daemon warns at start that this is plain
HTTP.

```sh
sudo agent-bus-setup --addr 100.121.175.32:6767        # on the node
```

On the other host:

```sh
export AGENT_BUS_ADDR=http://100.121.175.32:6767
export AGENT_BUS_TOKEN=$(ssh agent-busd@100.121.175.32 token)
agent-bus status                                       # "you": you
agent-bus start "#agent1@$(hostname)" --algo=args ~/hi.sh --descr "greets"
```

A realm after `@` is part of the agent's name, not where it runs. The daemon
never learns the host, so name the agent after it, as `$(hostname)` does here.

Where the network is not trusted, carry the port over SSH instead:
`ssh -L 6767:127.0.0.1:6767 agent-busd@<node>` and
`AGENT_BUS_ADDR=http://127.0.0.1:6767` on your side. Or use HTTPS.

</details>

## Over HTTPS

The same port, encrypted. With TLS turned on, the daemon answers TLS and plain
HTTP on one port, so older clients keep working while new ones use `https://`.
A self-signed certificate is trusted by its fingerprint, which you fetch once
over SSH; a client that meets any other certificate refuses to talk. Setup
turns it on ([setup § TLS](09-setup.md#tls)).

<details>
<summary>Example, and the client settings</summary>

On the node, once:

```sh
sudo agent-bus-setup --tls self-signed
```

On the other host:

```sh
export AGENT_BUS_ADDR=https://100.121.175.32:6767
export AGENT_BUS_TOKEN=$(ssh agent-busd@100.121.175.32 token)
export AGENT_BUS_TLS_FINGERPRINT=$(ssh agent-busd@100.121.175.32 token --fingerprint)
agent-bus status
agent-bus start "#agent1@$(hostname)" --algo=args ~/hi.sh --descr "greets"
```

| Client setting | |
|---|---|
| `AGENT_BUS_ADDR=https://host:port` | reach the port over TLS; the CLI, admin, token helper, runner, MCP face and launchers all accept it |
| `AGENT_BUS_TLS_FINGERPRINT=sha256:<hex>` | trust exactly the certificate with that SHA-256, a self-signed one included, and no other |
| no fingerprint | the system's trust store decides, which suits a CA-issued certificate |

- **Getting the fingerprint:** `ssh agent-busd@<node> token --fingerprint` is
  the forced command that hands out tokens, answering the fingerprint instead.
  It needs no name and no credential, since the fingerprint is public.
- **On the node:** `agent-bus-token --fingerprint` and `agent-bus-admin tls`
  read it from the certificate alone, never the key. Setup prints it when it
  finishes.
- **A wrong pin:** the call is refused, and the error names the certificate
  that answered. Nothing retries over plain HTTP.
- **No pin:** a self-signed certificate is refused as untrusted.
- **Renewal:** a certificate replaced later has a new fingerprint, so clients
  fetch it again the same way.
- **The TypeScript faces** check the certificate on one TLS connection, then
  trust exactly that certificate for every call. Bun's `fetch` cannot compare
  a fingerprint itself.
- **The listener:** [processes § the TCP listener](11-processes.md#the-tcp-listener).

</details>

## Your personal socket, forwarded

No port, no token. SSH carries your own socket to the other machine, and
anything there that uses it is you, just as on the node itself. A script agent
you own runs through it too. The daemon takes that agent's token on your
socket, so this one forward is all a runner needs.

<details>
<summary>Example: the forward, and a script agent run from the other host</summary>

One remote forward carries your socket there. Write the remote path for the
remote home, since `$HOME` would expand on your side. Keep the
`user-<something>.sock` shape, which is how a client recognises an identity
socket:

```sh
ssh -fN -M -S ~/.ssh/agent-bus-fwd -o ExitOnForwardFailure=yes \
  -R /home/parf/user-parf.sock:/run/agent-bus/user-parf.sock rdvp
```

A previous forward leaves the remote socket behind, and a new one over it would
not bind. With `ExitOnForwardFailure` it refuses to start rather than
half-working, so clear the old one first:

```sh
ssh rdvp 'rm -f ~/user-parf.sock'
```

On the other host, the [release archive](09-setup.md#install) provides
`agent-bus`, and one variable names your socket for every command below:

```sh
export AGENT_BUS_ADDR=$HOME/user-parf.sock
agent-bus status        # answers "you" as you
```

The script needs an absolute path, because the runner starts scripts in their
own work directory; the plain name gains its `#` on its own:

```sh
printf '#!/bin/sh\necho "hello $1"\n' > ~/hi.sh && chmod +x ~/hi.sh
agent-bus start "sample-hello@$(hostname)" --algo=args ~/hi.sh --allow '*' --descr greets
agent-bus call "#sample-hello@$(hostname)" --wait 10s world    # call takes the # as written
# hello world
agent-bus stop "sample-hello@$(hostname)"      # a deliberate stop unregisters it
```

- **When done:** `ssh -S ~/.ssh/agent-bus-fwd -O exit rdvp` closes the forward.
  `-M` is what makes that control socket, since without it only a
  `ControlMaster` setting in your SSH config would. Stop a runner before you
  close its forward, so it can still unregister itself.
- **Who else is you:** while the forward is up, anyone on that host who
  reaches the socket is you. It is `0600`, owned by your account there.
- **Launchers:** `ab-claude`, `ab-codex` and `ab-opencode` still switch to the
  shared socket beside yours, so forward `bus.sock` too, as for an older daemon.
- **An older daemon:** a runner also needs the shared socket beside its own,
  so add `-R /home/parf/bus.sock:/run/agent-bus/bus.sock` to the forward.
- **The rule:** [access § local socket](02-access.md#local-socket).

</details>

## The shared socket, forwarded

The shared socket, carried by SSH like your own, but it proves nothing by
itself. Every call on it must bring a token, and the token decides who is
calling. Use it to act as somebody other than your account on the other host,
such as an agent whose token you hold, without opening a port.

<details>
<summary>Example</summary>

```sh
ssh -fN -M -S ~/.ssh/agent-bus-shared -o ExitOnForwardFailure=yes \
  -R /home/parf/bus.sock:/run/agent-bus/bus.sock rdvp
```

On the other host:

```sh
export AGENT_BUS_ADDR=$HOME/bus.sock
agent-bus status                                   # "set AGENT_BUS_TOKEN": no token, no answer
AGENT_BUS_TOKEN=$(ssh agent-busd@<node> token) agent-bus status    # "you": the token's principal
```

- **The name:** keep it anything but `user-<something>.sock`. A client treats
  that shape as an identity socket and would not send the token.
- **Its mode:** the forwarded copy is `0600`, owned by your account on that
  host; on the node the shared socket is `666`, since it needs a token anyway.
- **Closing and stale sockets:** `-O exit` closes it, and a stale socket is
  cleared as for the personal one.

</details>
