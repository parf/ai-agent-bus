# Set up RAG over a folder

📌 **TL;DR:**
- **Problem:** you want an AI to search a folder of notes by keyword or meaning, and keep that search up to date as the notes change.
- **Outcome:** a [FileScan](filescan.md#what-it-does) instance watches the folder and feeds its own [RAG](rag.md#what-it-does) instance; every create, edit, rename and delete reaches RAG on its own.
- **Start:** follow [the steps](#the-steps) once as the node owner; then callers search RAG directly.

## Why you'd want it

- 🔍 **Notes nobody can find.** Now an AI searches them by keyword (free) or by meaning.
- 🔁 **An index that goes stale.** Now every edit is sent as it happens, with no cron and no manual sync.
- 🔑 **Private notes beside shared ones.** Now the folder gets its own RAG instance and its own access list.

## What you need

- A runner whose account can read the folder: the user-account runner for a folder under your home ([user runner](../runner/setup.md#a-persons-user-account-runner)).
- The bundled `#qdrant`, `#voyage` and `#ai` helpers, working.
- Owner rights on the bus. Every step below is an owner action.

## The steps

The example watches `/home/alice/notes` into `#rag/notes@node.example`, table `notes/pages`.

1. **Register the two agents**, each with only the callers it needs:

```sh
agent-bus register '#rag/notes@node.example' --kind=agent --allow='@owner,#filescan/notes@node.example,<who searches>'
agent-bus register '#filescan/notes@node.example' --kind=agent --allow='@owner'
```

2. **Create the collection** and its indexes, as in [Owner provisions the collection](rag.md#owner-provisions-the-collection), naming it `notes`.
3. **Grant the blocks to the new RAG only:** `#qdrant` with the `write` role, `#voyage` and `#ai`. Never grant `#voyage` to sessions: every grantee spends the same balance.

```sh
agent-bus manage '#qdrant@node.example' --add-to-set-allow='#rag/notes@node.example(write)'
agent-bus manage '#voyage@node.example' --add-to-set-allow='#rag/notes@node.example'
agent-bus manage '#ai@node.example' --add-to-set-allow='#rag/notes@node.example'
```

4. **Set both Configurations.** Put every exclude in place now: a file already sent stays in RAG when it is excluded later.

```sh
agent-bus config '#rag/notes@node.example' "$(cat src/agents/rag/config.json.dist)"   # with your block names and collection
agent-bus config '#filescan/notes@node.example' '{"dir":"/home/alice/notes","include":["*.md"],"exclude":["secrets/*","drafts/*"],"namespace":"notes","table":"pages"}'
```

5. **Give the table its labels.** RAG labels every text it stores and refuses a table without them:

```sh
agent-bus call '#rag/notes@node.example' '{"method":"prompts","action":"set","namespace":"notes","tableName":"pages","data":{"labels":{"howto":"A how-to","reference":"Reference material","other":"Anything else"},"instructions":"Choose the kind of this note."}}'
```

6. **Enable both in their runners, hand over the tokens, reload:**
   - `rag/notes` in the runner [inventory](../runner/configuration.md#inventory).
   - `filescan/notes`, with `"roots": ["/home/alice/notes"]`, in the inventory of the runner that can read the folder.
   - Each token goes to its runner once, as in [self-serving credential handoff](create-agent-prompt.md#self-serving-credential-handoff).
7. **Subscribe once.** The existing files go at once, and every later change follows on its own:

```sh
agent-bus call '#filescan/notes@node.example' 'sub add #rag/notes@node.example as rag notes pages'
```

## Check it

| Call | Done when |
|---|---|
| `agent-bus call '#filescan/notes@node.example' status` | `pending` is 0 and `skipped_for_rag` is empty |
| `agent-bus call '#rag/notes@node.example' '{"method":"count","namespace":"notes","tableName":"pages"}'` | `count` is the number of files sent |
| `agent-bus call '#rag/notes@node.example' '{"method":"search","namespace":"notes","tableName":"pages","query":"backup"}'` | hits name their `source` path |

## What it costs

- **Each stored file:** one Voyage embedding and one `#ai` labelling call. An edit pays both again; a touch that leaves the bytes unchanged pays nothing.
- **A search:** keyword is free; semantic and hybrid pay for one query embedding.

## When something is wrong

Answers name the cause; most name the fix.

| You see | Fix |
|---|---|
| `classification prompts are not configured for table 'pages'` | Step 5 |
| `required field … is missing` or `unknown field …` from RAG | Correct its Configuration (step 4) |
| `collection … is missing keyword index: …` | Create the named indexes (step 2) |
| `Qdrant grant missing: requires write role` | Step 3 |
| `voyage refused the request: …` (or `ai`, `qdrant`) | The block's own reason follows; a rate limit clears by itself and the file goes again |
| A file in `skipped_for_rag` | RAG's reason is beside it; the file goes again after the fix, behind fresh changes |
| `status` shows no files | The runner `roots` entry or `dir` is missing (step 6) |
| `paused` in `status` | The [mass-delete guard](filescan.md#the-mass-delete-guard) held a delete: check the folder, then `scan confirm:true` |

## Keep in mind

- **Exclude is not a read boundary:** the runner's sandbox mounts the whole `dir`, so keep secrets outside it, or accept that the agent can read what it never sends.
- **One RAG instance per trust domain:** anyone admitted to an instance can name any of its namespaces ([who may use it](rag.md#who-may-use-it)).
