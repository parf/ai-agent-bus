# Server-info: read a server's facts

📌 **TL;DR:** Ask about network addresses, memory, disk space and other server
facts without logging into the machine. The normal installation only reads
information and runs under an account without administrator powers.

## Why you'd want it

Get the facts you need during an incident without waiting for someone to
open a server session. Your AI can check several likely causes and bring
back a short report for the team.

## What it does

This helper runs a fixed set of read-only Linux commands. It answers one
section, such as `network`, or all available sections. Only people on its
access list can ask; it has no command for changing the machine.

## You'll need

- Linux, Python 3, systemd and the bus CLI.
- The Linux tools for the sections you want, such as `ip`, `free` and `lsblk`; `openssl` for `tls`.
- An administrator and [installation credentials](README.md#standalone-helpers).

## Try this first

See what the installed helper can do:

```sh
agent-bus help '#server-info@parf.us'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do)
for its Configuration and Secret help, JSON output, and script forms
`--help-config` and `--help-secret`.

## Use examples

### Ask your AI

| Say this | What it will call |
|---|---|
| “Is this server running out of memory?” | Server-info `memory` and `pressure` |
| “Show its network addresses and routes.” | `network` |
| “Check disk space and free inodes before the deploy.” | `disks` and `inodes`; inodes count available file entries |
| “Give me a quick report on this machine.” | `all`, then summarize the available sections |

### 1. Check a connection problem

```sh
agent-bus call '#server-info@parf.us' network
```

You see network interfaces, addresses and routes. A real host normally has
more than the local-only `lo` interface. This is the host's view, not a
private runner network.

### 2. Check space before updating

```sh
agent-bus call '#server-info@parf.us' disks
agent-bus call '#server-info@parf.us' inodes
```

You see disk sizes, mountpoints and free space, followed by free file entries.
A full disk or exhausted file entries can explain a failed write.

### 3. Collect facts for an incident report

```sh
agent-bus call '#server-info@parf.us' '{"method":"all"}'
```

The answer contains each section's command, output, error text and exit code.
A missing tool is reported; it is not silently treated as a healthy check.

## Installing it

1. Prepare [your installation credentials](README.md#standalone-helpers).
   You should have the bus address and your current token available to the installer.
2. Install the normal, unprivileged helper:

   ```sh
   sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
     src/agents/standalone/server-info/install.sh install \
     --name '#server-info@parf.us' --allow '@owner'
   ```

   It prints `agent-bus-server-info.service`. Its account is normally
   `ab-server-info`; the installer may use `nobody` if account creation is unavailable.
3. Check it:

   ```sh
   agent-bus call '#server-info@parf.us' network
   agent-bus call '#server-info@parf.us' --help
   ```

   You should see the host network and the supported sections.
4. Remove your personal token from the shell with `unset AGENT_BUS_TOKEN`.
   The helper keeps only its own credential.

To remove it, run the same installer with `remove --name '#server-info@parf.us'`
and current installation credentials. The Linux account is kept. An unchanged
reinstall does not restart the helper.

## Configuration

No agent settings are needed. Its full configuration is simply:

```jsonc
{} // no settings; remove this comment if saving JSON
```

It does not take a list of commands or execute commands from a question.
To include disk-health and hardware-sensor sections, the administrator must
explicitly reinstall with `--user root`. That is an installation choice,
not a Configuration setting. Install the required host tools separately.

## Secret keys

None. Do not save a person's bus password in Secret or Configuration. The
installer handles the helper's own bus credential.

## Who may use it

```sh
agent-bus manage '#server-info@parf.us' --add-allow 'ann@parf.us'
```

Ann can now read its facts. No extra action permission exists because this
helper does not change the server. Facts such as addresses and signed-in
users can still be private, so keep the access list small.

### Calls

| Section | What you see |
|---|---|
| `system`, `os`, `cpu`, `virt` | Kernel, OS, processor and virtualization facts |
| `network`, `dns`, `ports` | Interfaces/routes, DNS settings and listening TCP ports |
| `memory`, `load`, `pressure` | Memory use, load and resource waiting |
| `disks`, `inodes` | Disk inventory, space and available file entries |
| `uptime`, `time`, `users` | Time running, clock settings and signed-in users |
| `units` | Failed systemd units |
| `tls` | Public bus certificate SHA-256 pin, subject/SAN names, expiry date and days left; token-helper pin for comparison |
| `health`, `sensors` | Disk-health and sensor readings, only when installed as root |
| `all` | Every available section |

Text accepts one section name. JSON accepts exactly `{"method":"network"}`
or another section name; extra arguments are refused. A command can run for
up to ten seconds.

## TLS certificate

`agent-bus call '#server-info@parf.us' tls` reads only the public
`/etc/agent-bus/tls/cert.pem`, using `openssl x509`, and compares its pin through
`agent-bus token --fingerprint`. No private key is read. Missing OpenSSL answers
`unavailable: openssl not installed`; other read failures retain their diagnostic.
In JSON, the first successful `tls` command row adds `data` with `pin`,
`token_pin`, `subject`, `names`, `expires_at` (UTC) and fractional `days_left`.
[Debug doctor](debug.md#methods-and-limits) uses these facts over the bus to flag fewer
than 30 days left or a pin mismatch.

## Troubleshooting

| If you see this | Do this |
|---|---|
| A command is missing | Ask the administrator to install that host tool |
| Empty `health` or `sensors` | Normal without root: those sections are absent, even when requested |
| Permission refused by the bus | Ask the helper's owner to add you to its access list |
| A command returns an error | Read its error and exit code; the helper reports the failure rather than fixing it |
| The Linux service does not start | Check `systemctl status agent-bus-server-info`; confirm the account can execute the checkout and CLI |
