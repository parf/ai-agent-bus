# Qdrant: a shared store for meaning fingerprints

📌 **TL;DR:**
- **Problem:** a store that finds things by meaning needs an API key, and giving it to every app lets anyone with it delete everything, with no record of who did what.
- **Outcome:** apps and AI sessions read and write the store under their own bus name; the key stays on the server and each caller gets read, write or admin rights.
- **Start:** list the store's collections: `agent-bus call '#qdrant@node.example' 'request GET /collections'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **One key opens everything.** Now the key stays on the server; each caller gets a door with a lock level.
- 🧱 **Anyone with the key can delete it all.** Now **read** looks, **write** changes stored items, **admin** changes or deletes whole collections.
- 🔍 **You cannot tell who did what.** Now every call is made under the caller's own bus name.
- 🤖 **An AI should search, not own the store.** Now it gets read and nothing more.

## What it does

- ✅ **Pass one request through:** list, search, store, count, within the caller's rights (`request`).
- ✅ **Back a RAG app:** grant `#rag` write, and it stores and reads its texts ([RAG](rag.md#what-it-does)).
- ✅ **Let an AI search:** grant the session read; it searches without the store key.
- ✅ **Check what is stored:** list collections or count points.
- ✅ **Admin maintenance:** grant one admin to create collections and indexes.
- ✅ **Adds nothing of its own:** no RAG logic and no schema; the operator runs the store itself ([below](#run-a-store)).
- 🚫 **Not for:** ordinary rows and numbers (use a [database](db.md#what-it-does)), or separating teams inside one store (rights cover the whole store, not one collection).

## You'll need

- A running Qdrant store (local or remote), its address and API key.
- A runner with Python 3 and network access.
- The owner's grant to call the proxy.

## Run a store

- The operator chooses where and how the store runs; the bus installs none.
- One example with Podman, listening only on this machine:

<details>
<summary>Example: run Qdrant in a container</summary>

```sh
install -d -m 0700 "$HOME/.config/qdrant"
# Create $HOME/.config/qdrant/env with mode 0600 and this assignment:
# QDRANT__SERVICE__API_KEY=<your randomly generated key>
podman run -d --name qdrant \
  --env-file "$HOME/.config/qdrant/env" \
  -v qdrant-data:/qdrant/storage -p 127.0.0.1:6333:6333 \
  docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2
```

- The env file must be 0600; anyone who can run Podman as this user can read the key through container inspection.
- The named volume keeps the data.
- The bus chooses no mapping and imposes no location, resource or backup policy.
- Reboot startup (a user Quadlet is one option) and backups are the operator's.

</details>

## Try this first

- List the store's collections (needs at least read):

```sh
agent-bus call '#qdrant@node.example' 'request GET /collections'
```

<details>
<summary>See it on a real node (ran today)</summary>

```sh
agent-bus call '#qdrant@parf.us' 'request GET /collections' | jq -r .body
```

```text
{"result": {"collections": []}, "status": "ok", "time": 1.9286e-05}
```

- The store on the development node holds no collections yet; the output is from 2026-10-30.

</details>

## Use examples

- **See what collections exist:** `agent-bus call '#qdrant@node.example' 'request GET /collections'`
- **Store a point (write):** `agent-bus call '#qdrant@node.example' 'request PUT /collections/example/points?wait=true {"points":[{"id":1,"vector":[0.1,0.2]}]}'`
- **Count points from a script:** `agent-bus call '#qdrant@node.example' '{"method":"request","verb":"POST","path":"/collections/example/points/count","body":"{\"exact\":true}"}'`
- **Set up a collection for RAG:** admin only; see [RAG provisioning](rag.md#owner-provisions-the-collection).
- **Note:** the example collection must already exist; creating one needs admin.

## Installing it

- An administrator installs the package, sets the store address and puts the key in the runner's private file.
- Help works before either is set.

<details>
<summary>Step by step</summary>

- Build `src/agents/qdrant/package.sh <agent.d/qdrant>`, enable it in the [runner inventory](../runner/configuration.md#inventory), then set Configuration and the host key as below.
- The package uses standard-library Python and `needs.network:["*"]`; the runner grants the whole network, not a hostname firewall.

</details>

## Configuration

- **Only the store's address,** `url`, required.
- **Plain `http://` to a remote store sends the key unencrypted.**

<details>
<summary>Keys</summary>

| Key | Meaning |
|---|---|
| `url` | Required Qdrant HTTP(S) base URL, no default, e.g. `https://store.example/qdrant` |

- A reverse-proxy prefix is allowed.
- For the local store above, set `http://127.0.0.1:6333`; no localhost endpoint is assumed.
- HTTPS verifies certificates; a non-loopback HTTP call logs one notice per call.
- URLs must not contain credentials, query strings or fragments.

</details>

## Secret keys

- **`QDRANT_API_KEY`,** required, only in the runner's private host `qdrant/env` ([environment files](../runner/configuration.md#environment-files)).
- **Never** in Configuration, the URL, a call body or argv; help shows only its name.
- It is the same value as the store's `QDRANT__SERVICE__API_KEY`.

## Who may use it

- **Only the owner by default.**
- **Rights cover the whole store,** not one collection:

| Role | What it permits |
|---|---|
| `read` | List and inspect collections; retrieve, search, query, scroll, facet and count points |
| `write` | Read, plus upsert, update and delete points, vectors and payloads |
| `admin` | Everything, including collection, index and alias changes, snapshots and recovery, cluster and unknown API operations |

- **A caller admitted with no role gets write;** give `read` explicitly to limit it, for example `@team(read)`.
- **Admin can delete any collection** and make the store fetch files or URLs on the store's own host.

## Troubleshooting

| If you see… | Do this |
|---|---|
| A refusal naming the needed role | Check the caller's roles; an explicit read grant overrides the write default |
| A missing-key answer | Set `QDRANT_API_KEY` in the host env file |
| Connection failure, not sent | Check `url` and the store; nothing was sent |
| The write outcome is unknown | Check the store before repeating; nothing retries automatically |

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#qdrant` reports these.

| Level | When | What to do |
|---|---|---|
| alert | `QDRANT_API_KEY` is missing, or the store refuses it (401 or 403) | Set the key in the host `qdrant/env`; it must equal the store's key |
| alert | The store cannot be reached at `url` | Check `url` and that the store is running |
| warning | `url` is plain HTTP to a remote store: the key travels unencrypted | Use HTTPS or a loopback address |
| warning | A write's outcome is unknown (lost or incomplete answer) | Check the store before the caller repeats it |

## Technical details

<details>
<summary>Request rules and limits</summary>

- Verbs: uppercase GET, POST, PUT, PATCH, DELETE, HEAD or OPTIONS; GET, HEAD and OPTIONS take no body.
- The text path ends at the first space; percent-encode spaces in query values; JSON calls use a string `body`, preserving raw UTF-8 JSON bytes.
- Paths stay within Qdrant's API roots; full URLs, authorities, traversal, encoded separators, fragments, semicolons, controls and raw spaces are refused; an operator base-path prefix is preserved.
- Responses are JSON or UTF-8 text by Content-Type; binary is not supported; HEAD and 204 return status and redacted header metadata; HTTP errors keep status and body.
- One request per call, no redirects, retries or compression; request and answer caps are 1 MiB; the call takes at most 60 seconds and ends before the envelope deadline.
- A connection failure before sending says not sent; a lost or incomplete write response says the outcome is unknown: do not replay it automatically.
- Key echoes in JSON, text, error bodies and exposed headers are masked; logs hold verb and path without query, byte counts and status, never bodies.

</details>

<details>
<summary>Role rules and request classification</summary>

- Roles are the [delivered roles](../03-records-agent.md#roles); a bare `#rag` grant gets write; `#rag(write)` is explicit.
- Explicit `read` restricts; explicit `write` or `admin` wins over `read`; an explicit read plus a bare grant stays read.
- Generated `owner` and `maintainer` count as admin; unknown labels add no tier.
- Body-claimed roles and AB_ROLE-looking text confer nothing; the daemon's envelope roles win over environment claims.
- A read caller can read every collection (including cross-collection query lookup); a write caller can change points in every collection; the proxy passes admin operations through and adds no store sandbox.
- Unmatched paths need admin; paths outside the API roots are refused even for admin; required-role refusals name the needed tier before connecting.
- `c` is one collection segment, not checked for existence; keyword-like names do not shift the route; query strings do not change the tier; point ids are numeric or UUIDs; matches are exact: no prefix, substring or case-folded matching.

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
| Every other method/path pair | Any unmatched API operation, even GET; collection, index, alias, snapshot, cluster and service operations | admin |

</details>
