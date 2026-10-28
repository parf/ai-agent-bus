# Qdrant

📌 **TL;DR:** Run Qdrant wherever you choose; it is an external store, not a bus
component. The `#qdrant` proxy (R.4, pending) will use the URL you configure and
add the API key from its runner environment.

## Run a store

Choose the host, volume, port mapping and lifecycle yourself. This one example
maps HTTP to local port 6333; replace the mapping for your deployment:

```sh
install -d -m 0700 "$HOME/.config/qdrant"
# Create $HOME/.config/qdrant/env with mode 0600 and this assignment:
# QDRANT__SERVICE__API_KEY=<your randomly generated key>
podman run -d --name qdrant \
  --env-file "$HOME/.config/qdrant/env" \
  -v qdrant-data:/qdrant/storage -p 127.0.0.1:6333:6333 \
  docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2
```

The env file must be 0600; anyone who can run Podman as this user can read the
key through container inspection. The named volume retains the data. The bus installs no container, chooses no
mapping and imposes no location, resource or backup policy. Reboot startup and
backups are the operator's responsibility. A user Quadlet is one option for
reboot startup; it is local deployment configuration, not a bus installer feature.

## Point the proxy at it

**R.4 is pending.** Its Configuration will select the Qdrant URL, such as
`http://127.0.0.1:6333` for the mapping above or an operator-chosen remote HTTPS
URL. The runner's `QDRANT_API_KEY` must match the store's API key. Put it in the
[host environment layer](../runner/configuration.md#environment-files), never
in a call body or the URL. A remote store works the same way; no bus code may
assume localhost. The proxy adds the key and passes through Qdrant's API.

See the [RAG block plan](../../Plans/R1.0-Release/rag/TODO.md#steps) for status.
