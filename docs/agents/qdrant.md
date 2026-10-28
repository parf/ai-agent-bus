# Qdrant

📌 **TL;DR:** Call an operator-configured Qdrant store through the bus without
sharing its API key. Read, write and admin roles control the entire store;
admitted callers without an application role get write.

## Why you'd want it

Let an app or AI session query and update a Qdrant store using its bus identity,
while the operator keeps the store's API key in the runner environment.

## What it does

`request` sends one HTTP call to the configured store and returns its JSON or
UTF-8 text. It preserves API paths and raw JSON bodies within the caller's tier,
including administrative APIs for admin. It adds no RAG operations or schema.

## You'll need

A running Qdrant store, its URL and API key, Python 3, a runner with network
access, and an ACL grant to the proxy. The store can be local or remote.

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

## Try this first

```sh
agent-bus help '#qdrant@node.example'
agent-bus call '#qdrant@node.example' 'request GET /collections'
```

The first describes the proxy. The second lists the store's collections and
requires at least read. See the [block plan](../../Plans/R1.0-Release/rag/TODO.md#steps)
for implementation status.

## Use examples

```sh
agent-bus call '#qdrant@node.example' 'request GET /collections'
agent-bus call '#qdrant@node.example' 'request PUT /collections/example/points?wait=true {"points":[{"id":1,"vector":[0.1,0.2]}]}'
agent-bus call '#qdrant@node.example' '{"method":"request","verb":"POST","path":"/collections/example/points/count","body":"{\"exact\":true}"}'
```

Use uppercase GET, POST, PUT, PATCH, DELETE, HEAD or OPTIONS. The text path
ends at the first space; percent-encode spaces in query values. JSON calls use
a string `body`, preserving its raw UTF-8 JSON bytes. GET/HEAD/OPTIONS accept no
body. Paths stay within Qdrant's API roots; full URLs, authorities, traversal,
encoded separators, fragments, semicolons, controls and raw spaces are refused.
An operator base-path prefix is preserved. Responses are JSON or UTF-8 text
selected by Content-Type; binary responses are not supported. HEAD/204 return
HTTP status and redacted header metadata. HTTP errors retain status and body.

The example collection must already exist; creating it requires admin.

## Installing it

Build `src/agents/qdrant/package.sh <agent.d/qdrant>`, then enable the package in
the [runner inventory](../runner/configuration.md#inventory). Set its record's
Configuration and host key as below. Help works before either is configured.
The package uses stdlib Python and `needs.network:["*"]`; today's runner grants
the whole network, not a hostname firewall. The operator manages the store.

## Configuration

The only key is required `url`, with no default:

```json
{"url":"https://store.example/qdrant"}
```

Any operator-chosen HTTP(S) base URL is accepted, including an optional
reverse-proxy prefix. For the local mapping above, choose
`http://127.0.0.1:6333` explicitly; the proxy assumes no localhost endpoint.
HTTPS verifies certificates. **Remote plain HTTP sends the API key unencrypted**;
a non-loopback HTTP call logs one notice per invocation. URLs must not contain
credentials, query strings or fragments.

## Secret keys

Put matching `QDRANT_API_KEY` in the runner's private host `qdrant/env`,
runner-owned0600; it is declared required in `env.dist`. Use the
[host environment layer](../runner/configuration.md#environment-files), never
Configuration, the URL, a call body or argv. Help shows the variable name only.
The operator's container env file uses `QDRANT__SERVICE__API_KEY` for the same
value; anyone controlling that Podman user can read it through inspection.

## Who may use it

Default ACL is `@owner`. The [delivered roles](../03-records-agent.md#roles)
apply to the entire store, not a collection, namespace or record filter:

| Role | What it permits |
|---|---|
| `read` | List/inspect collections; retrieve, search, query, scroll, facet and count points |
| `write` | Read, plus upsert/update/delete points, vectors and payloads |
| `admin` | Everything, including collection/index/alias changes, snapshots/recovery, cluster and unknown API operations |

An ACL-admitted caller without an application role defaults to **write**.
Explicit `read` restricts it; explicit `write` or `admin` wins over `read`.
Generated `owner` and `maintainer` count as admin. Unknown labels add no tier.
For example, grant `@team(read)` for reading or `#rag(write)` for point changes.
A bare `#rag` grant also gets write. An explicit read plus a bare grant stays
read. Body-claimed roles and AB_ROLE-looking text confer no authority; the
daemon's envelope roles win over environment claims.

A read caller can read every collection (including cross-collection query
lookup). A write caller can change points in every collection. Granting admin
is full-store administration, including deleting any collection and making
Qdrant fetch recovery URLs or access paths on the **store host**. The proxy
passes those administrative operations through; it adds no store sandbox.
The table below uses exact method/path matches. A collection segment is not checked for
existence; keyword-like names do not shift the route. Unmatched paths require
admin; paths outside the API roots are refused even for admin.

### HTTP classification

`c` is one collection segment. Query strings do not change the tier. Point ids
are numeric or UUIDs. No prefix, substring or case-folded route matching is used.

| Method | Path | Needed |
|---|---|---|
| GET, HEAD | `/collections`, `/collections/{c}`, `/collections/{c}/exists`, `/collections/{c}/points/{id}` | read |
| POST | `/collections/{c}/points`, `/collections/{c}/facet` | read |
| POST | `/collections/{c}/points/scroll`, `/count`, `/query` (suffixes under `points`) | read |
| POST | `/collections/{c}/points/query/{batch,groups}` | read |
| POST | `/collections/{c}/points/search` and `search/{batch,groups,matrix/pairs,matrix/offsets}` | read |
| POST | `/collections/{c}/points/recommend` and `recommend/{batch,groups}`; `discover` and `discover/batch` | read |
| PUT | `/collections/{c}/points`, `/collections/{c}/points/vectors` | write |
| POST | `/collections/{c}/points/{delete,batch}`, `/collections/{c}/points/vectors/delete` | write |
| POST, PUT | `/collections/{c}/points/payload` | write |
| POST | `/collections/{c}/points/payload/{delete,clear}` | write |
| Every other method/path pair | Any unmatched API operation, even GET; collection/index/alias/snapshot/cluster/service operations | admin |

## Troubleshooting

A role refusal names the minimum tier required. Check the caller's delivered
roles; an explicit read grant restricts the no-role write default. A missing-key
answer means the private host env layer needs configuration. Connection failures
before sending say not sent; after sending, a lost write response is uncertain.

Each call makes one request, without redirects, retries or compression. Request
and answer caps are 1 MiB; the whole call is at most 60 seconds and ends before
the envelope deadline. A lost or incomplete write response says the outcome is unknown: do not
automatically replay it. A connection failure before sending says not sent.
Key echoes in JSON, text, error bodies and exposed headers are masked. Logs
contain only verb/path without query, byte counts and status, never request or
response bodies. Required-role refusals name the needed tier before connecting.
