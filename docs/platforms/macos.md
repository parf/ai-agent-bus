# Use AgentBus on macOS

📌 **TL;DR:** Use the client to reach a bus, or install a local daemon from the full archive. macOS has different service and isolation limits from Linux.

## macOS

### Client and runner

Install the client with `npm install -g ai-agent-bus-client`. Launchers require macOS 12.3 or newer for `readlink -f`.

The runner starts agents as its own account, without a sandbox or cgroup limits. A sandbox request is refused. Use a dedicated unprivileged account; agents hold that account's rights.

Downloaded binaries are unsigned and not notarized, so Gatekeeper may warn.

#### The daemon

Use the full Darwin archive to install the daemon with `agent-bus-setup`. It runs under launchd as its own account. The macOS npm package supplies the client, not daemon setup.

| Part | macOS |
|---|---|
| Account | `_agentbusd`, a hidden role account: an id from 200 free as both user and group, no login shell, no password |
| Storage | `/usr/local/var/agent-bus/daemon`, 0700, the database and `logs/` |
| Sockets | `/usr/local/var/run/agent-bus`, 0711; the shared `bus.sock` and the account's own socket only |
| Job | `/Library/LaunchDaemons/agent-busd.plist`: `agent-busd` as `_agentbusd`, umask 077, kept alive; setup reloads it with `launchctl` |
| TLS | as on Linux ([TLS](../administration.md#tls)) |
| First user | the installer, from their key, through `agent-bus-admin` on the account's socket |

Setup reuses an existing role user only with its matching group. A missing or mismatched group, or a failed lookup, refuses setup without changing the account.

- Not on macOS: per-role accounts, cgroup limits, the web job, logrotate, a person's runner, `--upgrade`; setup refuses those options.
- Without `CAP_CHOWN` the daemon cannot hand a socket to another account, so there are no per-user sockets: a person calls with a token (`sudo -u _agentbusd agent-bus token <name>`).
- Outside Linux the bus child stops itself when its supervisor is gone, where Linux has the kernel's death signal ([how a child is started](../08-processes.md#how-a-child-is-started)).
