# Local Qdrant

📌 **TL;DR:** Setup manages a separate, keyed Qdrant container for RAG. It binds
only IPv4 loopback and retains its data and keys on uninstall. Host changes need
the owner's approval; commissioning acceptance is separate from the normal gate.

## Install

Build the committed setup binary with [stamped build information](07-setup.md#build-information).
On a systemd host with rootful Podman and Quadlet, the approved operator runs:

```sh
sudo src/agent-bus-setup --qdrant install --for parf --dry-run
sudo src/agent-bus-setup --qdrant install --for parf
```

`--for` selects an **existing** person's runner. Dry-run prints public Quadlets
and runs the installed generator in a temporary render directory; it reads no
key and installs no unit. Installation refuses conflicting files/resources,
preserves an existing matching key, pulls the digest-pinned image, reloads
systemd and starts only `agent-bus-qdrant.service`. It does not restart the bus.
A partial failed installation is reported; retry preserves its matching files.

| Boundary | Setup supplies |
|---|---|
| Image | Qdrant v1.20.0 pinned by registry digest in `internal/qdrant.Image`; pulls only during explicit install |
| Listener | Host network; `service.host: 127.0.0.1`, HTTP 6333, gRPC off; no published ports |
| Process | UID/GID 1000:1000; no capabilities; no new privileges; read-only rootfs; 64 MiB `/tmp` tmpfs |
| Storage | Named volume `agent-bus-qdrant`, owned 1000:1000; data, snapshots, temporary files and init marker under `/qdrant/storage` |
| Limits | 2 GiB memory, two CPUs, 512 tasks; initial values to revisit before indexing |
| Boot | Quadlet `WantedBy=multi-user.target`; do not enable its generated service |

Host networking also gives a compromised container access to the host's other
loopback services. The owner accepted that trade-off in
[RD130](../Plans/R1.0-Release/DECISIONS.md#recorded-decisions).

## Credentials

The root-only `/etc/agent-bus/qdrant/config.yaml` holds a randomly generated
64-character hex key. Podman mounts it as secret `agent-bus-qdrant-config`,
mode 0400 owned 1000:1000. It is absent from argv and container environment.
The runner gets the same key as `QDRANT_API_KEY` in
`/var/lib/agent-bus/<runner account>/qdrant/env`, runner-owned 0600. This creates
no bus record; R.4 installs the proxy separately.

Setup compares the two files without printing values and checks captured
container inspect output for leaks. Privileged `podman secret inspect
--showsecret` is outside that guarantee and is never invoked. Conflicting
keys, configuration or resource labels are refused; there is no silent rotation.

## Commissioning

**This briefly restarts Qdrant. Never run it against production RAG data.**
Obtain approval for these commands separately:

```sh
sudo src/acceptance/qdrant.sh
sudo src/agent-bus-setup --qdrant snapshot
src/check-deploy.sh
```

The helper invokes `agent-bus-setup --qdrant check`. It refuses existing owned backups and non-fixture
collections before writes or restart. It writes synthetic points, restarts only
Qdrant, checks persistence and no-key refusal, then restores a verified full
snapshot into a fresh labelled volume/container at 127.0.0.1:16333. It imports a
single tar entry instead of binding or relabelling the backup directory, so
SELinux remains enabled. It verifies vectors, payload, imported ownership,
read-only-rootfs recovery, generator output and actual listeners. Every host
non-loopback IPv4/IPv6 address is probed; none may accept the API connection.
`route_localnet` is reported, never changed.

Cleanup removes only this run's labelled fixture resources, synthetic collection
and acceptance files. It refuses pre-existing fixture resources and keeps a
private commissioning journal if cleanup fails. A lone old acceptance collection
is removable only when both its points match that journal. Cleanup failures are
errors. An actual reboot check remains pending until an owner-scheduled reboot.

## Backups

`--qdrant snapshot` works with real collections and does not restart Qdrant.
It requires 30 GiB free and the snapshot budget plus 1 GiB spare, limits the
creation/download to 300 seconds and each full snapshot to 2 GiB, verifies Qdrant's
SHA-256, then fsyncs and atomically publishes a root-owned 0600 UTC-named backup
in `/var/lib/agent-bus/qdrant.backup/`. It retains the latest seven owned backup
names and preserves unrelated files. Requests neither retry nor follow redirects.
[Q200](../Plans/R1.0-Release/QUESTIONS.md#rag-service) tracks revisiting these
starting budgets before production indexing.

Full snapshots create collection snapshots too. Setup reads only headers and
the bounded manifest from the checksum-verified tar, records their names in a
private cleanup journal, checks the collection snapshot lists, deletes exactly
those children, then deletes the full server snapshot. Interrupted cleanup is
retried before a new snapshot. Before POST, setup writes a private unverified journal with snapshot-list hints.
An unverified download deletes the known full snapshot, prints a bounded list-delta
hint and blocks another POST until the owner inspects and explicitly clears the
journal. A definite HTTP 4xx refusal or failed dial clears that marker automatically;
an uncertain timeout/5xx outcome keeps it. After inspection, the owner may approve
`sudo src/agent-bus-setup --qdrant clear-unverified`: it prints fresh list-delta
hints and removes only the marker, never server snapshots. Verified child cleanup has a separate 60-second deadline. Install and
check each have a 900-second overall deadline; fixture cleanup gets 60 seconds. It never
infers deletion authority from a before/after list difference.

## Uninstall

With separate owner approval:

```sh
sudo src/agent-bus-setup --qdrant uninstall
```

This stops the owned unit, removes only matching managed Quadlets and reloads
systemd. It keeps the named data volume, mounted secret, root configuration,
runner key and backups. There is no purge operation.
