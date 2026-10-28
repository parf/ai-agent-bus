# R.3 host commands awaiting owner approval

No command below has run. RD130 approved the network choice, **not host apply**.
The source is reviewed and gated separately. The operator builds the committed
setup with `src/build.sh`; this neither installs nor restarts a service.

## Operator entry points

```sh
sudo src/agent-bus-setup --qdrant install --for parf --dry-run
sudo src/agent-bus-setup --qdrant install --for parf
sudo src/acceptance/qdrant.sh
sudo src/agent-bus-setup --qdrant snapshot
src/check-deploy.sh
```

The acceptance wrapper invokes exactly `src/agent-bus-setup --qdrant check`.
**Check restarts the live Qdrant unit briefly. It refuses non-fixture collections;
this is commissioning before RAG data, never a production health check. It also
refuses existing owned backups, so a fixture cannot prune a real backup.**
Reboot acceptance remains pending until the owner schedules it. Uninstall or any
additional root/systemd/Podman action requires separate approval.

## Files and internal work

| Path or resource | Operation |
|---|---|
| `/run/agent-bus-qdrant.lock` | Root 0600 exclusive operation lock |
| `/run/agent-bus-qdrant-validate-<random>/` | Temporary public Quadlets; generator dry-run; removed on return |
| `/etc/agent-bus/qdrant/` | Root 0700; canonical `config.yaml` root 0600; generate/preserve one hex API key |
| `/var/lib/agent-bus/parf-ab-runner/qdrant/` | Runner-owned 0700; `env` runner-owned 0600 with matching `QDRANT_API_KEY` |
| `/etc/containers/systemd/agent-bus-qdrant.container`, `.volume` | Root 0644 public managed Quadlets; conflicting files refused |
| `/var/lib/agent-bus/qdrant.backup/` | Root 0700; root 0600 atomic snapshot files, commissioning/cleanup journals, temporary import tar; seven owned completed backups retained |
| `agent-bus-qdrant` | Labelled Podman volume/container; UID/GID 1000:1000; preserved on uninstall |
| `agent-bus-qdrant-config` | Mounted config secret; metadata labels include component and SHA-256 of canonical config; no secret env |

Host networking is literal `Network=host`. Qdrant binds `127.0.0.1:6333`,
gRPC is off, telemetry is off. The restore fixture uses the same config/secret
with HTTP overridden to 16333. Both use non-root UID/GID, no capabilities,
no new privileges, read-only rootfs, bounded tmpfs and resource limits.
The exact public Quadlets are printed by dry-run; no key bytes are printed/read.

## Child command inventory

The setup uses fixed argv, no shell. `IMAGE` below means exactly:

```text
docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2
```

`RUN` is this invocation's random 32-character hex marker, recorded privately;
`CONFIG_SHA256` is the canonical config's digest, not its contents.
`RENDER_DIR` is the private random directory above. These are the only generated
substitutions; the approved networking has no alternate branch.

Preflight/inspection and generator (no persistent changes):

```sh
podman --version
env QUADLET_UNIT_DIRS=RENDER_DIR /usr/lib/systemd/system-generators/podman-system-generator --dryrun
podman container exists agent-bus-qdrant
podman container inspect --format '{{index .Config.Labels "io.agent-bus.component"}}' agent-bus-qdrant
podman volume exists agent-bus-qdrant
podman volume inspect --format '{{index .Labels "io.agent-bus.component"}}' agent-bus-qdrant
podman secret exists agent-bus-qdrant-config
podman secret inspect agent-bus-qdrant-config
```

Installation mutations:

```sh
podman pull IMAGE
podman secret create --label io.agent-bus.component=qdrant --label io.agent-bus.config-sha256=CONFIG_SHA256 agent-bus-qdrant-config /etc/agent-bus/qdrant/config.yaml
systemctl daemon-reload
systemctl start agent-bus-qdrant.service
systemctl is-active agent-bus-qdrant.service
```

Quadlet creates the named volume with `UID=1000`, `GID=1000` and
`Label=io.agent-bus.component=qdrant`. It starts only the pinned container,
with `Pull=never`; no generated-service `enable`, prune, firewall or sysctl write.

After startup and during check (inspection only):

```sh
podman container inspect --format '{{.State.Pid}}' agent-bus-qdrant
ss -H -ltnp
sysctl -n net.ipv4.conf.all.route_localnet
ip -json address show
podman container inspect agent-bus-qdrant
```

The full inspect is captured in bounded memory only for a no-key assertion and
never printed. Listener inspection requires exactly the Qdrant PID's loopback
HTTP listener. Internal 400 ms connects probe each non-loopback IPv4/IPv6
address, including scoped link-local addresses, and require refusal.

Commissioning fixture mutations and checks:

```sh
podman container exists agent-bus-qdrant-restore-check
podman volume exists agent-bus-qdrant-restore-check
systemctl restart agent-bus-qdrant.service
podman volume create --uid=1000 --gid=1000 --label agent-bus.fixture=r3 --label agent-bus.run=RUN agent-bus-qdrant-restore-check
podman volume import agent-bus-qdrant-restore-check /var/lib/agent-bus/qdrant.backup/r3-acceptance.tar
podman run --detach --pull=never --name agent-bus-qdrant-restore-check --label agent-bus.fixture=r3 --label agent-bus.run=RUN --user=1000:1000 --read-only --cap-drop=ALL --security-opt=no-new-privileges --tmpfs=/tmp:rw,nosuid,nodev,noexec,size=64m --pids-limit=512 --memory=2g --cpus=2 --network=host --volume agent-bus-qdrant-restore-check:/qdrant/storage --secret agent-bus-qdrant-config,type=mount,uid=1000,gid=1000,mode=0400 --env QDRANT__SERVICE__HTTP_PORT=16333 --env QDRANT_INIT_FILE_PATH=/qdrant/storage/.qdrant-initialized --entrypoint /qdrant/qdrant IMAGE --config-path /run/secrets/agent-bus-qdrant-config --storage-snapshot /qdrant/storage/restore/r3-acceptance.snapshot
podman exec agent-bus-qdrant-restore-check stat -c %u:%g /qdrant/storage/restore/r3-acceptance.snapshot
podman container inspect --format '{{.State.Pid}}' agent-bus-qdrant-restore-check
podman container inspect agent-bus-qdrant-restore-check
podman container inspect --format '{{.Id}} {{index .Config.Labels "agent-bus.fixture"}} {{index .Config.Labels "agent-bus.run"}}' agent-bus-qdrant-restore-check
podman volume inspect --format '{{.Name}} {{index .Labels "agent-bus.fixture"}} {{index .Labels "agent-bus.run"}}' agent-bus-qdrant-restore-check
podman rm --force --time=2 agent-bus-qdrant-restore-check
podman volume rm agent-bus-qdrant-restore-check
```

Cleanup verifies this run's labels and captured container identity before removal.
It never adopts existing fixture resources. Failed starts can leave partial
containers; only matching run-labelled partials are cleaned. Failure is reported.
No backup directory is bind-mounted or relabelled.

## HTTP and backup effects

Only authenticated loopback requests carry the key, never argv or logs. Setup
checks no-key `/collections` refusal. Commissioning creates/deletes only the
synthetic `ab_r3_acceptance` collection and its two marker/vector points.
`check`/`snapshot` use full-snapshot POST/download/list/delete operations, cap
JSON at 1 MiB, snapshot at 2 GiB and creation/download at 300 seconds. Install
and check have 900-second overall budgets; snapshot and fixture cleanup each
have a separate 60-second allowance.
Free-space checks use statfs, not an extra root command.

After checksum verification, bounded tar headers and `config.json` identify
exact child collection snapshots. A root-private retry journal precedes their
DELETEs; each child must appear exactly in its collection's snapshot list.
Full snapshot is deleted last. A failed/unverified download removes the known
full snapshot and prints a bounded before/after name delta as a hint only. A
root-private journal written before POST blocks all further snapshot creation
until owner inspection and explicit clearance; it never guesses.
The local backup is fsynced/renamed atomically; pruning only owned backup names.
Q200 tracks revisiting the starting budgets before production RAG use.
