# Server-info agent

📌 **TL;DR:** `#server-info@<host>` reports the host's facts through fixed
commands. It runs outside the sandbox under its own systemd unit; the shared
[standalone starter](../starter.md#standalone-starter) also serves
agents such as systemd. The daemon's ACL controls who can ask.

## Calls

Send one section name, or `all`. Text requests receive text; JSON requests
use exactly one field, `{"method":"network"}`, and receive JSON. Additional
words or JSON fields are refused as answers, before any command runs.
`--help` through the bus lists the sections available to this instance.

```sh
agent-bus call '#server-info@host' network
agent-bus call '#server-info@host' '{"method":"all"}'
```

Each JSON section contains a list of command results: `command` (the fixed
argument vector), `stdout`, `stderr`, and `status` (exit code). Text has the
same output under section headings. A missing command or command refusal
is an answer, with its status, not a silent success. Each command has a
10-second bound. No caller-supplied argument reaches a command or shell.

## Sections

| Section | Host source |
|---|---|
| system | `uname -a` |
| network | `ip a`, `ip r` |
| memory | `free` |
| disks | `lsblk` with name, type, size, model, serial and mountpoints; `df -h` |
| uptime | `uptime` |
| load | `/proc/loadavg` |
| os | `/etc/os-release` |
| cpu | `lscpu` |
| inodes | `df -i` |
| pressure | `/proc/pressure/cpu`, `memory`, `io` |
| units | `systemctl --failed --no-pager` |
| time | `timedatectl --no-pager` |
| dns | `/etc/resolv.conf` |
| ports | `ss -tln`, without process names |
| users | `who` |
| virt | `systemd-detect-virt` |
| health (root only) | `smartctl -H` for each whole disk from the host's `lsblk` inventory |
| sensors (root only) | `sensors` |

Health and sensors are absent when the process is unprivileged, including
when requested individually: empty text or `{}`, not an error. Install the
host tools for the sections wanted; the agent does not install packages.

## Install and remove

Requires Linux, Python 3, systemd, and an installed `agent-bus`. Set
`AGENT_BUS_ADDR` to the bus's HTTP(S) endpoint and `AGENT_BUS_TOKEN` to the
launching account's credential. For pinned TLS also set
`AGENT_BUS_TLS_FINGERPRINT`. Keep those values out of command arguments.

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  ./src/agents/standalone/server-info/install.sh install \
  --name '#server-info@host' --allow '@owner'

sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  ./src/agents/standalone/server-info/install.sh remove --name '#server-info@host'
```

The default is the dedicated `ab-server-info` system account, with no login
or home directory; if its creation is refused, the starter uses `nobody`.
Removal retains the account. Add `--user root` explicitly
to include health and sensors. The unit account must be able to execute the agent and CLI at
their installed paths; the starter does not copy binaries.
The returned unit name is stable for the agent name. Rerunning an unchanged
installation keeps the same unit, name, credential file and running process.
See the [starter lifecycle](../starter.md#lifecycle) for credential
ownership, restart and removal behavior.

## Verification

The `agent_server_info` shard runs inside `src/smoke.sh --slow`. It checks
fixed commands and refusal before execution, privileged section availability,
real bus calls against the host network, install twice, restart, graceful
removal and removal after a killed runner. Generated units pass
`systemd-analyze verify`; tests use a recording systemd stand-in and a private
daemon, never install a unit on the development node.
