# Container image

📌 **TL;DR:** One image built from a release archive runs the bus in two
containers: the `daemon` role on a storage volume, and a `runner` role for one
person that reaches the daemon's port with its own token. People use the port
with tokens. The container is the sandbox boundary: an agent that asks the
runner for a sandbox is refused, never run unconfined. See [sandboxing](#sandboxing-is-off) before running agents.

| File | What it is |
|---|---|
| `Containerfile` | the image: AlmaLinux 10 minimal, `python3`, the release under `/usr/local/lib/agent-bus` as setup lays it out |
| `agent-bus-container` | the entry point: `daemon`, `runner`, or any command |
| `echo/` | the delivery example, a runner-served agent that answers with the message text |
| `acceptance.sh` | the image's acceptance check, rootless podman |

## Build

Install a container engine (Podman or Docker Buildx) and the bus client on the host.
Download a full Linux release archive and its `SHA256SUMS` into `tmp/dist`.
Use this checkout for the Containerfile and entry point; the runtime comes from
the archive. The build checks the selected checksum, then its inner manifest:

- `SHA256SUMS` takes precedence; a missing selected line fails even if a sidecar exists.
- A local `src/package.sh` directory supplies `<archive>.sha256`, used only when `SHA256SUMS` is absent.
- Verification and unpacking happen in a builder stage; only the unpacked tree enters the final image, without release-input archive layers.

```sh
podman build --build-context release=tmp/dist -t agent-bus src/container
```

`docker buildx build` takes the same `--build-context`. With more than one
archive in the directory, pass `--build-arg ARCHIVE=agent-bus-<version>-linux-<arch>.tar.gz`.
Use that explicit selection with a downloaded GitHub release, whose directory
contains several platforms and `SHA256SUMS`, without per-archive sidecars.
For contributor-built archives, run `src/package.sh tmp/dist` first. Complete
release-set verification is described in the [maintainer deployment guide](../../.claude/docs/deployment.md#build-information).

## Run

| Role | Account | Volume | Needs |
|---|---|---|---|
| `daemon` | `agent-busd` (990) | `/var/lib/agent-bus/daemon`: the database, logs, TLS | `AGENT_BUS_OWNER=<user>@<realm>`; the hostname is the realm, saved at the first start |
| `runner` | `agent-bus-runner` (991) | `/var/lib/agent-bus/runner`: inventory, settings, agent tokens, work | `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN` of `#runner/<person>` |

Each role starts as root only to give a fresh volume to its account, then runs
as that account with no capabilities and no way to gain privilege (no new privileges, empty bounding set), so the image's setuid binaries stay inert. A directory the account owns is set to mode 0700, whatever it was; an empty volume is also given to the account first. A bind mount owned by anyone else must already be usable by the account and not writable by its group or others, or the start is refused by name and the directory is left as it was; one that is usable keeps its owner and mode. A container started as the account itself (`--user 990`) follows the same rules and cannot change an owner, so it refuses a directory it cannot use. Under rootless podman a bind mount with data needs `podman unshare chown 990:990 <dir>` (991 for the runner) first, or use a named volume. The daemon's and the runner's secrets
stay in separate containers.

```sh
podman network create agent-bus
podman run -d --name agent-busd --network agent-bus --hostname node -p 6767:6767 \
  -e AGENT_BUS_OWNER=owner@node -v agent-bus-daemon:/var/lib/agent-bus/daemon agent-bus daemon

# Inside the daemon container its own account is the Owner, on the local socket.
podman exec -u agent-busd agent-busd agent-bus register '#runner/owner' --allow @owner
RUNNER_TOKEN=$(podman exec -u agent-busd agent-busd agent-bus token '#runner/owner')
OWNER_TOKEN=$(podman exec -u agent-busd agent-busd agent-bus token)

podman run -d --name agent-bus-runner --network agent-bus \
  -e AGENT_BUS_ADDR=http://agent-busd:6767 -e AGENT_BUS_TOKEN="$RUNNER_TOKEN" \
  -v agent-bus-runner:/var/lib/agent-bus/runner agent-bus runner

AGENT_BUS_ADDR=http://localhost:6767 AGENT_BUS_TOKEN="$OWNER_TOKEN" agent-bus call '#echo' hello
```

The first start of the daemon saves the hostname as the realm and the `AGENT_BUS_OWNER` as the Owner, both for good: pass `--hostname` then (a first start on the default container-id hostname is refused), and a later `AGENT_BUS_OWNER` is read but changes nothing. The answer comes from `#echo`, served by the runner container.

<details>
<summary>Persistence, access and agents</summary>

| Concern | Here |
|---|---|
| Persistence | The [storage](../../docs/administration.md#storage) is the daemon volume. A new container on the same volume keeps every record and credential; a container without it starts a new bus |
| Access | The port answers HTTP on `0.0.0.0:6767` (`AGENT_BUS_LISTEN` changes it). Put `cert.pem` and `key.pem` in the volume's `tls/` directory and it answers TLS too; clients then pin it with `AGENT_BUS_TLS_FINGERPRINT` ([TLS](../../docs/administration.md#tls)). The local socket serves only the daemon container's own processes ([access](../../docs/02-access.md#what-a-call-carries)) |
| Agents | Shared agent code is `/var/lib/agent-bus/agent.d`: the image holds `echo`; bind-mount a directory there for others; the runner's first-start inventory lists the agents present then, so a directory without `echo` lists no `#echo` ([runner configuration](../../docs/runner/configuration.md#where-settings-belong)). An agent that serves itself needs its token handed over as the runner says |
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

Every agent runs as the runner's own account, `agent-bus-runner` (uid 991). It
can read whatever that account can: the runner's token in the runner's
environment (`/proc/1/environ`), and every other agent's token and settings under
`/var/lib/agent-bus/runner`. Run in this image only agents you would trust with
the runner's identity.

## Acceptance

`src/container/acceptance.sh <archive directory>` builds the image, runs both
roles on a private network and checks: a request/reply from a third container;
a recreated daemon on its volume keeps a registered service and the issued
tokens; an agent that asks for a sandbox is refused and gets no answer. It
removes the fixture containers, network and volumes; build caches remain. `MUTANT=daemon-entry|runner-entry|storage-outside|sandbox-downgrade`
breaks the entry point, and each must fail a check.

`src/container/acceptance.sh --image=<reference> <version>` runs the same
checks on a published image already pulled into the local store; a mutant is a
layer over that image's own entry point. The published image stays in the store.

The build uses `--pull=never` and needs the pinned base image in the local
store: `podman pull` the `FROM` reference in the `Containerfile` once. Without
it, or without podman, the smoke check fails with that reason. Its package install reaches the
AlmaLinux mirrors, so an offline host with no cached layers fails the build.
