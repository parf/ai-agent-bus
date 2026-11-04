# Remote access

📌 **TL;DR:**
- **Problem:** an agent, a script or a person on another machine needs the bus without opening it to the world.
- **Outcome:** four ways in, each answering the same API with the same permissions: plain HTTP on a trusted network, HTTPS anywhere, your own socket over SSH, or the shared socket over SSH. The rules behind them are in [access](02-access.md#what-a-call-carries).
- **Start:** pick one in [choosing a way](#choosing-a-way), then give any command the `--addr` option ([Try this first](#try-this-first)).

## Why you'd want it

- Reach the same bus from another machine without changing its access rules.
- Choose a connection protected by TLS, SSH or your trusted network.

## What you get

- Direct HTTP or HTTPS access with a token.
- SSH forwarding of an account socket or the shared token-authenticated socket.

## Try this first

Choose a method below, set its address and credentials, then check the connection:

```sh
agent-bus status
```

The `you` field identifies the caller. In the examples, replace `bus.example.com` with your bus host; SSH token commands require an installed, authorized key ([getting a token](02-access.md#getting-a-token)).

## Choosing a way

| Way | Proves who you are | Protects the connection | Suits |
|---|---|---|---|
| [HTTP](#over-http-on-a-trusted-network) | a token | nothing: plain text | a network you trust — a VPN, tailscale, a host-only bridge |
| [HTTPS](#over-https) | a token | TLS, with a trusted certificate or explicit pin | any network |
| [Your socket, forwarded](#your-personal-socket-forwarded) | the socket itself: you, no token | SSH | running as you, or agents you own, with no port opened |
| [The shared socket, forwarded](#the-shared-socket-forwarded) | a token | SSH | acting as someone else, such as an agent, with no port opened |

## Over HTTP, on a trusted network

The daemon's TCP port, reached directly. It is the simplest way, and nothing
encrypts it: your token and every message cross the network as they are. Use
it only where nobody else can listen, such as a VPN or tailscale, or a bridge
between a host and its containers.

<details>
<summary>Example: a reachable address on a trusted network</summary>

The node binds an address the other host reaches (`--addr` in setup, `--addr`
on the daemon). Off loopback, the daemon warns at start that this is plain
HTTP.

```sh
sudo /usr/local/bin/agent-bus-setup --addr bus.example.com:6767
```

On the other host:

```sh
export AGENT_BUS_ADDR=http://bus.example.com:6767
export AGENT_BUS_TOKEN=$(ssh agent-busd@bus.example.com token)
agent-bus status
```

Where the network is not trusted, use HTTPS or carry the port over SSH:

```sh
ssh -N -L 6767:127.0.0.1:6767 your-account@bus.example.com
```

In another terminal, use `AGENT_BUS_ADDR=http://127.0.0.1:6767` with your token. The tunnel requires an SSH account permitted to forward ports, not the token helper's forced-command account.

</details>

## Over HTTPS

Enable [TLS](administration.md#tls), then use an `https://` address. Use a certificate trusted by the client or pin its fingerprint, fetched through an authenticated SSH connection.

### One port serves both HTTP and HTTPS

- **The client picks:** `https://` is encrypted, `http://` is not; the daemon refuses neither.
- **Across a network:** use HTTPS (pin the fingerprint) or SSH; a token sent to an `http://` address crosses the network in the clear.
- One listener detects TLS or plain HTTP for each connection ([TCP listener](08-processes.md#the-tcp-listener)).

For the web face, separately [trust the web certificate in your browser](administration.md#trust-the-web-certificate-in-your-browser); CLI fingerprint pinning does not enable browser token saving.

<details>
<summary>Example, and the client settings</summary>

On the node, once:

```sh
sudo /usr/local/bin/agent-bus-setup --tls self-signed
```

On the other host:

```sh
export AGENT_BUS_ADDR=https://bus.example.com:6767
export AGENT_BUS_TOKEN=$(ssh agent-busd@bus.example.com token)
export AGENT_BUS_TLS_FINGERPRINT=$(ssh agent-busd@bus.example.com token --fingerprint)
agent-bus status
```

| Client setting | |
|---|---|
| `AGENT_BUS_ADDR=https://host:port` | reach the port over TLS; the CLI, admin, token helper, runner, MCP face and launchers all accept it |
| `AGENT_BUS_TLS_FINGERPRINT=sha256:<hex>` | trust exactly the certificate with that SHA-256, a self-signed one included, and no other |
| no fingerprint | the system's trust store decides, which suits a CA-issued certificate |

- **Getting the fingerprint:** `ssh agent-busd@bus.example.com token --fingerprint` is
  the forced command that hands out tokens, answering the fingerprint instead.
  It needs no name and no credential, since the fingerprint is public.
- **On the node:** `agent-bus token --fingerprint` and `agent-bus-admin tls`
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

</details>

## Your personal socket, forwarded

No port, no token. SSH carries your own socket to the other machine, and
anything there that uses it is you, just as on the node itself. A script agent
you own runs through it too. The daemon takes that agent's token on your
socket, so this one forward is all a runner needs.

<details>
<summary>Forward an account socket</summary>

Run this on the bus host. Replace `alice` with the local account, `/home/alice` with its home on the destination, and `client-host` with that SSH destination. The destination account must allow forwarding.

```sh
ssh -fN -M -S ~/.ssh/agent-bus-fwd -o ExitOnForwardFailure=yes \
  -R /home/alice/user-alice.sock:/run/agent-bus/user-alice.sock client-host
```

Use an absolute remote path: `$HOME` in the forwarding argument would expand on the bus host. Keep the `user-<account>.sock` filename so clients recognize an account socket.

A stopped forward may leave its socket file behind. After confirming that no active forward uses it, remove that stale file on the destination before restarting:

```sh
ssh client-host 'rm -f ~/user-alice.sock'
```

On the destination, install the [client programs](07-setup.md#install), then select the socket:

```sh
export AGENT_BUS_ADDR=$HOME/user-alice.sock
agent-bus status
```

- Run scripts through the [dedicated runner](06-dedicated-runner.md#script-agents); AI [launchers](06-launchers.md#smart-launchers) use the same forwarded socket.
- The socket is `0600`, owned by the destination account. Any process that can use it acts as you; your Agent's token selects that Agent.
- If a connected daemon cannot act as the Agent on an account socket, the client falls back to the shared listener. Forward that listener beside it as described below.
- Stop a runner before closing its connection so it can unregister. `-M` creates the SSH control socket used to close this forward:

```sh
ssh -S ~/.ssh/agent-bus-fwd -O exit client-host
```

</details>

## The shared socket, forwarded

The shared socket, carried by SSH like your own, but it proves nothing by
itself. Every call on it must bring a token, and the token decides who is
calling. Use it to act as somebody other than your account on the other host,
such as an agent whose token you hold, without opening a port.

<details>
<summary>Example</summary>

Run on the bus host; replace the destination home and SSH host as in the account-socket example:

```sh
ssh -fN -M -S ~/.ssh/agent-bus-shared -o ExitOnForwardFailure=yes \
  -R /home/alice/bus.sock:/run/agent-bus/bus.sock client-host
```

On the other host:

```sh
export AGENT_BUS_ADDR=$HOME/bus.sock
export AGENT_BUS_TOKEN=$(ssh agent-busd@bus.example.com token)
agent-bus status
```

- **The name:** keep it anything but `user-<something>.sock`. A client treats
  that shape as an account socket, which uses different authentication rules.
- **Its mode:** the forwarded copy is `0600`, owned by your account on that
  host; on the node the shared socket is `666`, since it needs a token anyway.
- **Closing and stale sockets:** `-O exit` closes it, and a stale socket is
  cleared as for the personal one.

</details>
