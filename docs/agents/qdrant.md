# Qdrant: a shared store for meaning fingerprints

📌 **TL;DR:** A door to a Qdrant store — a database that finds things by meaning
(it keeps *vectors*: number lists from an embedding service). Apps and AI
sessions read and write it under their own bus name; the store's password stays
on the server. Each caller gets read, write or admin rights for the whole store.

## Why you'd want it

**The problem.** A vector store needs an API key. Giving it to every app means
anyone with it can also delete everything, and there is no record of who did what.

**The idea.** The key stays on the server. Each caller gets a door with a lock
level: **read** looks, **write** changes stored items, **admin** may change or
delete whole collections. The owner sets the level per caller.

**When to use it**

- An app such as [RAG](rag.md#what-it-does) needs to store and find texts by meaning.
- Several apps or sessions share one store, with different rights.
- You want an AI session to search a store without holding its key.

**When not to:** ordinary rows and numbers (use a [database](db.md#what-it-does)),
or separating teams inside one store (rights cover the whole store, not one
collection).

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| Back a RAG app | Grant `#rag` write | RAG stores and reads its texts |
| Let an AI search | Grant the session read | Search without the store key |
| Check what is stored | List collections or count points | Sizes and names |
| Admin maintenance | Grant one admin | Create collections and indexes |

## What it does

| Call | Does |
|---|---|
| `request` | Sends one Qdrant API request (for example list, search, store) and returns the answer |

It passes requests through within the caller's rights and adds nothing of its
own: no RAG logic, no schema. The operator runs the store itself ([below](#run-a-store)).

<details><summary>Technical details: request rules and limits</summary>

- Verbs: uppercase GET, POST, PUT, PATCH, DELETE, HEAD or OPTIONS. GET/HEAD/OPTIONS take no body.
- The text path ends at the first space; percent-encode spaces in query values. JSON calls use a string `body`, preserving raw UTF-8 JSON bytes.
- Paths stay within Qdrant's API roots; full URLs, authorities, traversal, encoded separators, fragments, semicolons, controls and raw spaces are refused. An operator base-path prefix is preserved.
- Responses are JSON or UTF-8 text by Content-Type; binary is not supported. HEAD/204 return status and redacted header metadata. HTTP errors keep status and body.
- One request per call, no redirects, retries or compression. Request and answer caps are 1 MiB; the call takes at most 60 seconds and ends before the envelope deadline.
- A connection failure before sending says not sent. A lost or incomplete write response says the outcome is unknown: do not replay it automatically.
- Key echoes in JSON, text, error bodies and exposed headers are masked. Logs hold verb/path without query, byte counts and status, never bodies.

</details>

## You'll need

A running Qdrant store (local or remote), its address and API key, a runner
with Python 3 and network access, and the owner's grant to call the proxy.

## Run a store

The operator chooses where and how the store runs; the bus installs none. One
example with Podman, listening only on this machine:

<details><summary>Example: run Qdrant in a container</summary>

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
key through container inspection. The named volume keeps the data. The bus
chooses no mapping and imposes no location, resource or backup policy; reboot
startup (a user Quadlet is one option) and backups are the operator's.

</details>

## Try this first

```sh
agent-bus call '#qdrant@node.example' 'request GET /collections'
```

Lists the store's collections; needs at least read. Status of the work is in
the [block plan](../../Plans/R1.0-Release/rag/TODO.md#steps).

## Use examples

| I want to… | Command |
|---|---|
| See what collections exist | `agent-bus call '#qdrant@node.example' 'request GET /collections'` |
| Store a point (write) | `agent-bus call '#qdrant@node.example' 'request PUT /collections/example/points?wait=true {"points":[{"id":1,"vector":[0.1,0.2]}]}'` |
| Count points from a script | `agent-bus call '#qdrant@node.example' '{"method":"request","verb":"POST","path":"/collections/example/points/count","body":"{\"exact\":true}"}'` |
| Set up a collection for RAG | admin only; see [RAG provisioning](rag.md#owner-provisions-the-collection) |

The example collection must already exist; creating one needs admin.

## Installing it

An administrator installs the package, sets the store address and puts the key
in the runner's private file. Help works before either is set.

<details><summary>Step by step</summary>

Build `src/agents/qdrant/package.sh <agent.d/qdrant>`, enable it in the
[runner inventory](../runner/configuration.md#inventory), then set Configuration
and the host key as below. The package uses standard-library Python and
`needs.network:["*"]`; today's runner grants the whole network, not a hostname
firewall.

</details>

## Configuration

Only the store's address, `url`, required. **Plain `http://` to a remote store
sends the key unencrypted.**

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `url` | Required Qdrant HTTP(S) base URL, no default, e.g. `https://store.example/qdrant` |

A reverse-proxy prefix is allowed. For the local store above, set
`http://127.0.0.1:6333`; no localhost endpoint is assumed. HTTPS verifies
certificates. A non-loopback HTTP call logs one notice per call. URLs must not
contain credentials, query strings or fragments.

</details>

## Secret keys

`QDRANT_API_KEY`, required, only in the runner's private host `qdrant/env`
([environment files](../runner/configuration.md#environment-files)). Never in
Configuration, the URL, a call body or argv; help shows only its name. It is the
same value as the store's `QDRANT__SERVICE__API_KEY`.

## Who may use it

Only the owner by default. Rights cover the **whole store**, not one collection:

| Role | What it permits |
|---|---|
| `read` | List/inspect collections; retrieve, search, query, scroll, facet and count points |
| `write` | Read, plus upsert/update/delete points, vectors and payloads |
| `admin` | Everything, including collection/index/alias changes, snapshots/recovery, cluster and unknown API operations |

A caller admitted with no role gets **write**; give `read` explicitly to limit
it, for example `@team(read)`. Admin can delete any collection and make the store
fetch files or URLs on the store's own host.

<details><summary>Technical details: role rules and request classification</summary>

- Roles are the [delivered roles](../03-records-agent.md#roles). A bare `#rag` grant gets write; `#rag(write)` is explicit.
- Explicit `read` restricts; explicit `write` or `admin` wins over `read`. An explicit read plus a bare grant stays read.
- Generated `owner` and `maintainer` count as admin. Unknown labels add no tier.
- Body-claimed roles and AB_ROLE-looking text confer nothing; the daemon's envelope roles win over environment claims.
- A read caller can read every collection (including cross-collection query lookup); a write caller can change points in every collection. The proxy passes admin operations through and adds no store sandbox.
- Unmatched paths need admin; paths outside the API roots are refused even for admin. Required-role refusals name the needed tier before connecting.
- `c` is one collection segment, not checked for existence; keyword-like names do not shift the route. Query strings do not change the tier. Point ids are numeric or UUIDs. Matches are exact: no prefix, substring or case-folded matching.

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

</details>

## Troubleshooting

| If you see… | Do this |
|---|---|
| A refusal naming the needed role | Check the caller's roles; an explicit read grant overrides the write default |
| A missing-key answer | Set `QDRANT_API_KEY` in the host env file |
| Connection failure, not sent | Check `url` and the store; nothing was sent |
| The write outcome is unknown | Check the store before repeating; nothing retries automatically |
