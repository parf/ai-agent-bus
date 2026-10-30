# Server-info: a server's vital signs

📌 **TL;DR:** Ask a server how it is doing (disk space, memory, network,
certificate expiry and more) without logging in. It only reads: it runs a
fixed set of harmless commands under a plain account and cannot change
anything.

## Why you'd want it

**The problem.** In an incident, the first questions are simple (is the disk
full? is memory gone?), but answering them waits for someone with a server
login.

**The idea.** A read-only helper on the server answers those questions over
the bus, so you or your AI get the facts at once and can write one short
report.

**When to use it**

- Something failed and you want the usual suspects checked fast.
- Before a deploy, to confirm there is room and nothing is already broken.
- To watch when the bus certificate expires.

**When not to:** fixing anything (it cannot), live monitoring and graphs, or
reading application logs (use [logwatch](logwatch.md#what-it-does)).

**Possible use cases**

| Use case | What you ask | What you get |
|---|---|---|
| "No space left" error | `disks`, `inodes` | Free space and free file slots per disk |
| Service slow or killed | `memory`, `pressure` | Memory use and how much programs wait |
| Peers cannot connect | `network`, `ports` | Addresses, routes, listening ports |
| Certificate check | `tls` | Expiry date and days left |
| Incident report | `all` | Every section in one answer |

## What it does

It answers one section, or `all`; the sections are listed under
[Calls](#calls). Each answer shows the command, its output and any error. A
missing tool is reported, never treated as healthy. It never runs a command
from a question.

## You'll need

- Linux, Python 3, systemd and the bus on the server.
- The usual tools for the sections you want (`ip`, `free`, `lsblk`; `openssl` for `tls`).
- An administrator to install it.

## Try this first

```sh
agent-bus help '#server-info@node.example'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

## Use examples

| I want to… | Command |
|---|---|
| Check disk space | `agent-bus call '#server-info@node.example' disks` |
| Check memory | `agent-bus call '#server-info@node.example' memory` |
| See addresses and routes | `agent-bus call '#server-info@node.example' network` |
| Check the certificate | `agent-bus call '#server-info@node.example' tls` |
| Get everything | `agent-bus call '#server-info@node.example' '{"method":"all"}'` |

<details><summary>Rules for calls</summary>

- Text takes one section name; JSON takes exactly `{"method":"<section>"}`; extra arguments are refused.
- Each command may run up to ten seconds; the answer carries its exit code.
- `network` is the host's view, not a runner's private network.

</details>

## Installing it

An administrator installs it once as a Linux service under a plain account.

<details><summary>Step by step</summary>

With [installation credentials](README.md#standalone-helpers):

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  src/agents/standalone/server-info/install.sh install \
  --name '#server-info@node.example' --allow '@owner'
agent-bus call '#server-info@node.example' network
unset AGENT_BUS_TOKEN
```

- The installer prints `agent-bus-server-info.service`. It runs as `ab-server-info`, or `nobody` if that account cannot be created.
- Disk-health and sensor sections need an explicit reinstall with `--user root` (an install choice, not a setting) and their host tools.
- Remove: same installer, `remove --name '#server-info@node.example'`, with current credentials; the Linux account is kept. An unchanged reinstall does not restart it.

</details>

## Configuration

None: it takes no settings (`{}`).

## Secret keys

None. The installer handles the helper's own bus credential; never save a
person's token in Secret or Configuration.

## Who may use it

Only people on its access list; there are no extra permissions because it
changes nothing. Addresses and signed-in users can still be private, so keep
the list small (`agent-bus manage '#server-info@node.example' --add-allow 'ann@node.example'`).

### Calls

| Section | What you see |
|---|---|
| `system`, `os`, `cpu`, `virt` | Kernel, OS, processor, virtualization |
| `network`, `dns`, `ports` | Addresses and routes, DNS settings, listening TCP ports |
| `memory`, `load`, `pressure` | Memory use, load, how much programs wait |
| `disks`, `inodes` | Disks and free space, free file slots |
| `uptime`, `time`, `users` | Uptime, clock, signed-in users |
| `units` | Failed services |
| `tls` | Bus [certificate facts](#tls-certificate) |
| `health`, `sensors` | Disk health and sensors, only when installed as root |
| `all` | Every available section |

## TLS certificate

`tls` reads only the bus's public certificate, never its private key, and
reports its names, expiry and days left, plus whether its fingerprint matches
the one clients use. [Debug doctor](debug.md#what-it-does) uses it to warn at
fewer than 30 days or a mismatch.

<details><summary>Technical details: tls</summary>

- Reads `/etc/agent-bus/tls/cert.pem` with `openssl x509` and compares its SHA-256 pin with `agent-bus token --fingerprint`.
- Without OpenSSL it answers `unavailable: openssl not installed`; other read failures keep their diagnostic.
- In JSON the first successful `tls` row adds `data` with `pin`, `token_pin`, `subject`, `names` (subject/SAN), `expires_at` (UTC) and fractional `days_left`.

</details>

## Warnings and alerts

`#server-info` has its own Linux service, so the [runner entries](README.md#warnings-and-alerts) do not apply; it reports these itself. The page that shows them is in work. No alerts of its own: it reports facts and never judges them; [doctor](debug.md#what-it-does) does.

| Level | When | What to do |
|---|---|---|
| warning | A host tool a section needs is missing | Install it, or ignore that section |
| warning | A section's command failed or timed out | Read its error in the answer |
| warning | `tls` cannot read the bus certificate | Make `/etc/agent-bus/tls/cert.pem` readable by its account |

## Troubleshooting

| If you see | Do this |
|---|---|
| A command is missing | Install that host tool |
| Empty `health` or `sensors` | Normal without root; those sections are absent |
| Refused by the bus | Ask the owner to add you to the access list |
| A command error | Read its error and exit code; the helper reports, never fixes |
| The service does not start | `systemctl status agent-bus-server-info`; check the account can run the checkout and CLI |
