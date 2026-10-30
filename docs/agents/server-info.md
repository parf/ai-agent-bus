# Server-info: a server's vital signs

📌 **TL;DR:**
- **Problem:** in an incident the first questions are simple (is the disk full? is memory gone?), but answering them waits for someone with a server login.
- **Outcome:** you or your AI get the facts at once, read-only, and can write one short report.
- **Start:** ask what the helper can do: `agent-bus help '#server-info@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **Only people with a login can look.** Now anyone on the access list asks over the bus.
- ⏱️ **The usual suspects take time to check.** Now one call returns disks, memory, network and more.
- 🛡️ **A check must not change anything.** Now it only reads, with a fixed set of harmless commands.
- 📜 **A certificate expires unnoticed.** Now you ask when it ends and how many days are left.

## What it does

- ✅ **Disk trouble:** free space and free file slots per disk (`disks`, `inodes`).
- ✅ **Slow or killed service:** memory use and how much programs wait (`memory`, `pressure`, `load`).
- ✅ **Peers cannot connect:** addresses, routes, DNS and listening ports (`network`, `dns`, `ports`).
- ✅ **Certificate check:** expiry date and days left (`tls`).
- ✅ **Incident report:** every section in one answer (`all`).
- ✅ **Honest answers:** each answer shows the command, its output and any error; a missing tool is reported, never treated as healthy.
- 🚫 **Not for:** fixing anything (it cannot), live monitoring and graphs, or application logs (use [logwatch](logwatch.md#what-it-does)).

## You'll need

- Linux, Python 3, systemd and the bus on the server.
- The usual tools for the sections you want (`ip`, `free`, `lsblk`; `openssl` for `tls`).
- An administrator to install it.

## Try this first

- Ask the helper what it can do:

```sh
agent-bus help '#server-info@node.example'
```

- More on this: [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

## Use examples

- **Check disk space:** ask for `disks`.
- **Check memory:** ask for `memory`.
- **See addresses and routes:** ask for `network`.
- **Check the certificate:** ask for `tls`.
- **Get everything:** ask for `all`.

## Installing it

- An administrator installs it once as a Linux service under a plain account.

<details>
<summary>Step by step</summary>

With [installation credentials](README.md#standalone-helpers):

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  src/agents/standalone/server-info/install.sh install \
  --name '#server-info@node.example' --allow '@owner'
unset AGENT_BUS_TOKEN
```

- The installer prints `agent-bus-server-info.service`; it runs as `ab-server-info`, or `nobody` if that account cannot be created.
- Disk-health and sensor sections need an explicit reinstall with `--user root` (an install choice, not a setting) and their host tools.
- Remove: the same installer, `remove --name '#server-info@node.example'`, with current credentials; the Linux account is kept.
- An unchanged reinstall does not restart it.

</details>

## Configuration

- **None:** it takes no settings (`{}`).

## Secret keys

- **None.**
- The installer handles the helper's own bus credential; never save a person's token in Secret or Configuration.

## Who may use it

- **Only people on its access list;** there are no extra permissions because it changes nothing.
- **Keep the list small:** addresses and signed-in users can still be private.
- Example: `agent-bus manage '#server-info@node.example' --add-allow 'ann@node.example'`.

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

- **What:** `tls` reads only the bus's public certificate, never its private key.
- **You get:** its names, expiry and days left, and whether its fingerprint matches the one clients use.
- **Used by:** [debug doctor](debug.md#what-it-does) warns at fewer than 30 days or a mismatch.

<details>
<summary>Technical details: tls</summary>

- Reads `/etc/agent-bus/tls/cert.pem` with `openssl x509` and compares its SHA-256 pin with `agent-bus token --fingerprint`.
- Without OpenSSL it answers `unavailable: openssl not installed`; other read failures keep their diagnostic.
- In JSON the first successful `tls` row adds `data` with `pin`, `token_pin`, `subject`, `names` (subject/SAN), `expires_at` (UTC) and fractional `days_left`.

</details>

## Troubleshooting

| If you see | Do this |
|---|---|
| A command is missing | Install that host tool |
| Empty `health` or `sensors` | Normal without root; those sections are absent |
| Refused by the bus | Ask the owner to add you to the access list |
| A command error | Read its error and exit code; the helper reports, never fixes |
| The service does not start | `systemctl status agent-bus-server-info`; check the account can run the checkout and CLI |

## Warnings and alerts

`#server-info` has its own Linux service, so the [runner entries](README.md#warnings-and-alerts) do not apply; it reports these itself. No alerts of its own: it reports facts and never judges them; [doctor](debug.md#what-it-does) does.

| Level | When | What to do |
|---|---|---|
| warning | A host tool a section needs is missing | Install it, or ignore that section |
| warning | A section's command failed or timed out | Read its error in the answer |
| warning | `tls` cannot read the bus certificate | Make `/etc/agent-bus/tls/cert.pem` readable by its account |

## Technical details

<details>
<summary>How it runs</summary>

- A fixed set of harmless read-only commands runs under a plain account (`ab-server-info`, or `nobody` if that account cannot be created).
- It never runs a command taken from a question.
- Disk-health and sensor sections need an explicit reinstall as root and their host tools.
- A missing tool is reported with the section, never treated as healthy.

</details>

<details>
<summary>Rules for calls</summary>

- Text takes one section name; JSON takes exactly `{"method":"<section>"}`; extra arguments are refused.
- Each command may run up to ten seconds; the answer carries its exit code.
- `network` is the host's view, not a runner's private network.

</details>
