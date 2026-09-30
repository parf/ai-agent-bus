# Install agent-bus

📌 **TL;DR:** For now agent-bus installs from git. Clone it under
`/usr/local/src`, build it, run setup once, and link the programs into
`/usr/local/bin` with `git-install.sh`. After that, an update is a pull, a
build and a restart.

## Prerequisites

- Linux with systemd, and `sudo`
- git, Go with gcc, and Bun at `/usr/bin/bun`

## Install

```sh
sudo mkdir -p /usr/local/src/ai-agent-bus && sudo chown "$USER" /usr/local/src/ai-agent-bus
git clone https://github.com/parf/ai-agent-bus /usr/local/src/ai-agent-bus
cd /usr/local/src/ai-agent-bus
bun install --cwd src/mcp
src/build.sh
sudo src/agent-bus-setup --exec "$PWD/src/agent-busd"
sudo src/git-install.sh --no-build
```

The checkout must live outside `/home`: the daemon runs as its own account and
cannot see a home directory.

## Check

```sh
agent-bus status
systemctl is-active agent-busd agent-bus-web
```

The web panel is at <http://127.0.0.1:6780/>.

## Try it

```sh
printf '#!/bin/sh\necho "hi: $1"\n' > ~/hi.sh && chmod +x ~/hi.sh
agent-bus start hi --algo=args ~/hi.sh      # in one terminal
agent-bus call '#hi' --wait 10s ping        # in another: hi: ping
```

## Update

```sh
cd /usr/local/src/ai-agent-bus
git pull && bun install --cwd src/mcp && src/build.sh
sudo systemctl restart agent-busd agent-bus-web
```

<details>
<summary>More setup options</summary>

| | |
|---|---|
| SSH key | Setup makes your `~/.ssh/id_ed25519.pub` the first user's key. Without one, run `ssh-keygen -t ed25519` first, or add it later with `agent-bus-admin user add` |
| TLS | `sudo src/agent-bus-setup --exec "$PWD/src/agent-busd" --tls self-signed`, or `--tls files --tls-cert … --tls-key …` ([setup § TLS](../docs/09-setup.md#tls)) |
| Sample data | `sudo agent-bus-setup --samples` adds sample users, agents, services, queues, topics and groups; `--remove-samples` takes them away |
| Preview | `--dry-run` says what setup would do; `--print-unit` prints the daemon unit. Neither changes anything |

</details>

## From a release archive

<details>
<summary>Install and upgrade from a packaged release</summary>

```sh
sha256sum -c agent-bus-*.tar.gz.sha256
mkdir agent-bus-install
tar -xzf agent-bus-*.tar.gz -C agent-bus-install --strip-components=1
sudo ./agent-bus-install/agent-bus-setup --owner "$USER@$(hostname -s)"
```

To upgrade, unpack the new archive the same way and run its
`agent-bus-setup --upgrade`. It keeps the node's configuration and TLS, backs up
the state, and rolls back by itself if the new release fails to start. After an
interrupted upgrade, run `--recover`, then `--upgrade` again
([setup § install](../docs/09-setup.md#install)).

</details>
