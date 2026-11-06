<!-- AB: #ab-ceo -->
# RAG questions

📌 **TL;DR:** open owner choices for the [RAG app](README.md). Answer in place; resolved items move to the plan's decisions.

- ✅ Q360 → all four extras (`count`, `similar`, `complete`, `classify`); owner, 2026-11-06: "yes - implement this all; mdhouse is just one of many possible rag-system-users; we need to cover most common use cases out of the box". [Decision](../DECISIONS.md#recorded-decisions), [R.8](TODO.md)


## Large files

Choices for [large Markdown files](large-files.md#large-markdown-files-in-rag).

- ❓ Q368 How RAG gets the bytes of a file over 32 KiB
  - Context: FileScan already hands a file of up to 32 MiB to a queue as a bigfile on its own record; RAG today never opens a path and takes a source only as data ([trust boundary](../../../docs/agents/rag.md#search-and-source-evidence))
  - ( ) FileScan sends a bigfile reference and RAG downloads it, checking its SHA-256 🌟
  - ( ) RAG reads the host file itself, given a path under a folder it is allowed to read ⭐
  > 💡👾 The bigfile reference: RAG keeps never opening a path, needs no read access to the owner's folders, and the hash it checks is the one it confirms.

- ❓ Q369 How RAG is admitted to a FileScan instance's bigfiles
  - Context: a bigfile is fetched by whoever the record's allow list admits; a term with no role label grants no FileScan role, so RAG could download but not use FileScan's methods
  - ( ) Whoever adds an `as rag` subscription also adds the RAG instance to that FileScan instance's allow list, with no role label 🌟
  - ( ) FileScan writes the bigfile to the RAG instance's own record, which needs FileScan as a Maintainer there ⭐
  > 💡👾 The allow-list term: read-only, one per FileScan instance, and it gives RAG nothing beyond the download.

- ❓ Q370 How long a reference made for RAG lives
  - Context: RAG downloads at the start of the call, so the lifetime covers a file's wait while pending, not RAG's processing; FileScan keeps the key until RAG confirms the file
  - ( ) Keep `bigfile_ttl` (1 hour by default); an expired reference is refused by name, and FileScan uploads again on its next attempt 🌟
  - ( ) Raise the default for `as rag` to 24 hours, so a long backlog never re-uploads ⭐
  > 💡👾 Keep the hour: a re-upload is a local copy, never a paid call, and short keys keep a record's 100-key limit free.

- ❓ Q371 The largest file RAG indexes, and so the most one file can cost
  - Context: each 4 KiB chunk is one paid `#voyage` embedding on first sync; a re-sync pays only for changed chunks; a file over the cap stays skipped and named in `status`
  - ( ) 1 MiB per file, about 256 chunks 🌟
  - ( ) 4 MiB per file, about 1024 chunks ⭐
  - ( ) 32 MiB, FileScan's bigfile limit
  > 💡👾 1 MiB: it covers mdhouse's five large files with room to spare, and keeps a file's first sync to a few hundred embeddings.
