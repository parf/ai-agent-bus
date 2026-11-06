<!-- AB: #ab-ceo -->
# RAG questions

📌 **TL;DR:** open owner choices for the [RAG app](README.md). Answer in place; resolved items move to the plan's decisions.

- ✅ Q360 → all four extras (`count`, `similar`, `complete`, `classify`); owner, 2026-11-06: "yes - implement this all; mdhouse is just one of many possible rag-system-users; we need to cover most common use cases out of the box". [Decision](../DECISIONS.md#recorded-decisions), [R.8](TODO.md)


## Large files

Choices for [large Markdown files](large-files.md#large-markdown-files-in-rag).

- ✅ Q368 → FileScan sends a bigfile reference and RAG downloads it, checking its SHA-256; owner, 2026-11-06 ~13:57: "all recommended options, go". [Decision](../DECISIONS.md#recorded-decisions), [plan](large-files.md)

- ✅ Q369 → Whoever adds an `as rag` subscription also adds the RAG instance to that FileScan instance's allow list, with no role label; owner, 2026-11-06 ~13:57: "all recommended options, go". [Decision](../DECISIONS.md#recorded-decisions), [plan](large-files.md)

- ✅ Q370 → Keep `bigfile_ttl` (1 hour by default); an expired reference is refused by name, and FileScan uploads again on its next attempt; owner, 2026-11-06 ~13:57: "all recommended options, go". [Decision](../DECISIONS.md#recorded-decisions), [plan](large-files.md)

- ✅ Q371 → 1 MiB per file, about 256 chunks; owner, 2026-11-06 ~13:57: "all recommended options, go". [Decision](../DECISIONS.md#recorded-decisions), [plan](large-files.md)
