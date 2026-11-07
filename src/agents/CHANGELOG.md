# Agent pack changelog

📌 **TL;DR:** Changes to the bundled agents in `src/agents/`, newest first.
The pack has its own SemVer in `src/agents/VERSION`, separate from the bus version.

## 1.0.5 — 2026-11-07

ai: models --images shows no price column; ai.md and help give the image_output unit via model <id>; free refused before any provider call (J127)

## 1.0.4 — 2026-11-06

FileScan: include matches the whole relative path, a bare-directory include is warned in status, **/ matches zero directories (J124, J125); bundled Python client: serve(on_reply) and call() refuses wait (J122)

## 1.0.3 — 2026-11-06

First published agent pack changeset: 1.0.0–1.0.3, squashed.

| Area | Changes |
|---|---|
| Pack | The bundled agents have their own version, apart from the bus; RAG keeps its own app version in its `agent.json`. Each agent declares the daemon protocol it speaks (`protocol.json`), and its method set comes from its `agent.json`. An agent change runs that agent's checks and the pack contract, not the daemon's. |
| FileScan | Reports a rename as one change, stores big files on request, confirms RAG upserts safely, runs a finite on-demand `sync` (bounded, resumable, removes files gone from the folder) and sends large Markdown files to RAG by bigfile reference. |
| RAG | 0.4.0 adds optional rerank; 0.4.1 adds count, similar, complete (a prefix index built by `keyword_index`) and classify (off unless configured); 0.4.2 upserts large Markdown files in chunks and answers a completeness `check`. |
| DBSync | Shares the fixed change-sync core with FileScan. |
