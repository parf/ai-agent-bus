# Agent images: every agent has a picture

📌 **TL;DR:**
- **Problem:** the web agent page and every list show agents as bare names; a user cannot tell the mail agent from the chat agent at a glance.
- **Outcome:** an agent ships one image file in its own folder, named by `image` in its `agent.json`; the web reads it from disk when it can and otherwise asks the agent, and shows it in the page header and small in lists.
- **Start:** [the rules](#rules), then [how the web gets it](#how-the-web-gets-it). Task: [IMG.1](TODO.md#agent-images). Open: [Q400](QUESTIONS.md#agent-images).

Owner's ask, 2026-11-07: an optional `image` field and bundled images. Owner answers, 2026-11-11: the agent provides its own image ([№Q395](resolved.md#Q395), [№Q396](resolved.md#Q396)); nothing on the bus generates one.

## Rules

| Rule | Value |
|---|---|
| Field | optional `image` in `agent.json`: a file name inside the agent's own folder, suggested `agent.avif` or `agent.webp`; returned by `--help` and `--help-json` with the file's SHA-256, so the web caches by hash. Without one the web shows a letter-on-colour placeholder |
| Accepted | AVIF, WebP, PNG or JPEG, chosen by the first bytes, never by the extension; square; at most 256 KiB; 512×512 is the cap and the suggestion; never SVG, GIF or anything else. Nothing shipped scales an image (no image library in shared code, [external tools](../../src/MODULES.md#external-tools)): a file over the caps is refused, never resized |
| Refused | an absolute path, `..`, any symlink in the path, a name that is not a regular file, a wrong type, a non-square image, a file over the byte cap, a pixel count over the cap |
| Pixel cap | the byte cap alone does not bound the decoded size (a tiny PNG can claim 60 000×60 000), so width and height are read from the header without decoding (AVIF: its `ispe` property) and bounded by the 512×512 cap |
| Where it is checked | `agent_pack` for a bundled agent; the runner's package load for any package, refusing it by name as it does for a blank `description`; the web before it serves, from the bytes it got |
| Serving | one web route, record name only, never a path from a record; the type is sniffed again, `X-Content-Type-Options: nosniff`, a restrictive `Content-Security-Policy`, immutable cache keyed by the hash. The route reuses `secure()` in `server.ts` and adds none of the `statics` map's paths |
| Seeing it | the image shows to whoever may see the agent's page; a viewer the agent refuses gets the placeholder unless the web already holds that hash |
| Generation | none on the bus. The [good-agent guide](../../docs/10-writing-an-agent.md) and the create-agent prompt say how to make one (for example `#ai`'s `image`, open to whoever `#ai`'s ACL admits, [№Q397](resolved.md#Q397)) and where to put it |
| Bundled set | 24 images made once with the best image model `#ai` offers, reviewed by the owner, committed into each agent's folder as ordinary images, no mark ([№Q398](resolved.md#Q398)); sized to the caps when made, outside shipped code; the model and estimated cost go to the owner before any paid call |

## How the web gets it

Owner, 2026-11-11 ([№Q395](resolved.md#Q395)): disk first, then the agent.

| Order | Source | When |
|---|---|---|
| 1. Disk | the file in the installed bundled pack, found by the bundled agent's name under the pack directory the web already knows, never by a path from a record | a bundled agent installed on the web's host |
| 2. Ask, runner-served | an `image` ask the runner answers for the agents it serves, from the image named in the package's `agent.json`, validated at package load | an agent with no process of its own |
| 2. Ask, self-hosted | the same ask, answered by the shared helpers (Go, Python, TypeScript) from the agent's own `agent.json` | any agent that serves itself, bundled or not |

- **Caching:** the web keeps an image by its SHA-256 from help; a new hash fetches again; an unchanged one never asks.
- **Offline agent:** a cached image still shows; one never fetched shows the placeholder.
- **Answer size:** 256 KiB is about 342 KiB in base64, under the 1 MiB reply ceiling; IMG.1 tests a maximal image.
- **Nothing expires:** no bigfile, no key-value copy, no re-put timer.

## Size

| Item | Value |
|---|---|
| Package | a 512×512 WebP or AVIF is typically 15–60 KB; 24 add about 0.4–1.5 MB, at most 6 MB at the cap, against the 4.3 MB `src/agents` tree today |
| Container image | the same bytes added once; measured by `#ab-ceo` with `podman images` at the build |

## Readers and edits

| Where | Change |
|---|---|
| `src/internal/agentmeta/metadata.go` (`Manifest`), `help.go` | the `image` field and its hash; validate; return in `--help-json` and the text help |
| `src/agents/shared/metadata.py`, `metadata.ts`, the Go helper | validate at start; answer the `image` ask from `agent.json` |
| runner package load | the same validation, refusing by name; answer `image` for the agents it serves |
| `src/web/agent-metadata.tsx` and the lists | header image, lazy thumbnail, placeholder; one image route; disk first, then the ask; the hash cache |
| `src/agents/smoke.sh` `agent_pack` | every `image` named is valid; no agent names a file outside its folder |
| `docs/runner/configuration.md` (field table), `docs/10-writing-an-agent.md`, `docs/agents/create-agent-prompt.md` | the field, and how to make and place an image |

No schema beyond the named field and its hash.
