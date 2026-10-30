# Container image

📌 **TL;DR:** One image built from a release archive runs the bus in two
containers: the `daemon` role on a storage volume, and a `runner` role for one
person that reaches the daemon's port with its own token. People use the port
with tokens. The container is the sandbox boundary: an agent that asks the
runner for a sandbox is refused, never run unconfined. This implements the
[container contract](../../Plans/R1.0-Release/distribution.md#container-runtime).

| File | What it is |
|---|---|
| `Containerfile` | the image: AlmaLinux 10 minimal, `python3`, the release under `/usr/local/lib/agent-bus` as setup lays it out |
| `agent-bus-container` | the entry point: `daemon`, `runner`, or any command |
| `echo/` | the delivery example, a runner-served agent that answers with the message text |
| `acceptance.sh` | the image's acceptance check, rootless podman |

## Build

The image is built from a release archive that `src/package.sh` made, never
from a checkout. The archive's checksum and manifest are checked first.

```sh
src/package.sh tmp/dist
podman build --build-context release=tmp/dist -t agent-bus src/container
```

`docker buildx build` takes the same `--build-context`. With more than one
archive in the directory, pass `--build-arg ARCHIVE=agent-bus-<version>-linux-<arch>.tar.gz`.

## Run

| Role | Account | Volume | Needs |
|---|---|---|---|
| `daemon` | `agent-busd` (990) | `/var/lib/agent-bus/daemon`: the database, logs, TLS | `AGENT_BUS_OWNER=<user>@<realm>`; the hostname is the realm |
| `runner` | `agent-bus-runner` (991) | `/var/lib/agent-bus/runner`: inventory, settings, agent tokens, work | `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN` of `#runner/<person>` |

Each role starts as root only to give a fresh volume to its account, then runs
as that account with no capabilities. The daemon's and the runner's secrets
stay in separate containers.

```sh
podman network create agent-bus
podman run -d --name agent-busd --network agent-bus --hostname node -p 6767:6767 \
  -e AGENT_BUS_OWNER=owner@node -v agent-bus-daemon:/var/lib/agent-bus/daemon agent-bus daemon

# Inside the daemon container its own account is the Owner, on the local socket.
podman exec -u agent-busd agent-busd agent-bus register '#runner/owner' --allow @owner
podman exec -u agent-busd agent-busd agent-bus token '#runner/owner'   # the runner's token
podman exec -u agent-busd agent-busd agent-bus token                   # the Owner's token

podman run -d --name agent-bus-runner --network agent-bus \
  -e AGENT_BUS_ADDR=http://agent-busd:6767 -e AGENT_BUS_TOKEN=<runner token> \
  -v agent-bus-runner:/var/lib/agent-bus/runner agent-bus runner

AGENT_BUS_ADDR=http://localhost:6767 AGENT_BUS_TOKEN=<owner token> agent-bus call '#echo' hello
```

The answer comes from `#echo`, served by the runner container.

<details>
<summary>Persistence, access and agents</summary>

| Concern | Here |
|---|---|
| Persistence | The [storage](../../docs/07-setup.md#storage) is the daemon volume. A new container on the same volume keeps every record and credential; a container without it starts a new bus |
| Access | The port answers HTTP on `0.0.0.0:6767` (`AGENT_BUS_LISTEN` changes it). Put `cert.pem` and `key.pem` in the volume's `tls/` directory and it answers TLS too; clients then pin it with `AGENT_BUS_TLS_FINGERPRINT` ([TLS](../../docs/07-setup.md#tls)). The local socket serves only the daemon container's own processes ([access](../../docs/02-access.md#what-a-call-carries)) |
| Agents | Shared agent code is `/var/lib/agent-bus/agent.d`: the image holds `echo`; bind-mount a directory there for others ([runner configuration](../../docs/runner/configuration.md#where-settings-belong)). An agent that serves itself needs its token handed over as the runner says |
| Updates | Build a new image and replace the containers; the volumes stay |

</details>

## Sandboxing is off

The container is the boundary, and the image has no
[sandbox](../../docs/06-dedicated-runner.md#sandboxing) to give: the runner
serves an agent only when `runner/agents.json` says `"sandbox": "off"` for it.
On its first start the runner role writes that inventory for `echo`, and never
changes it after. An agent left at the default, or set to `"on"`, is refused
with "the sandbox is on and this host cannot give one", which
`agent-bus call '#runner/<person>' status` shows.

## Acceptance

`src/container/acceptance.sh <archive directory>` builds the image, runs both
roles on a private network and checks: a request/reply from a third container;
a recreated daemon on its volume keeps a registered service and the issued
tokens; an agent that asks for a sandbox is refused and gets no answer. It
removes everything it made. `MUTANT=daemon-entry|runner-entry|storage-outside|sandbox-downgrade`
breaks the entry point, and each must fail a check.
