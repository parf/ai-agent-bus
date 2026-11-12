# Agent pack changelog

📌 **TL;DR:** Changes to the bundled agents in `src/agents/`, newest first.
The pack has its own SemVer in `src/agents/VERSION`, separate from the bus version.

## 1.0.9 — 2026-11-12

Packaged agents ship the picture their agent.json names; agents bundling the Python client gain its schedule calls; #cron and #at are retired

## 1.0.8 — 2026-11-11

Every bundled agent gets a colorful picture of its own

## 1.0.7 — 2026-11-11

Every bundled agent ships its picture in one house style and answers the image ask (IMG.1)

## 1.0.6 — 2026-11-07

Each bundled agent now carries its own version (1.0.0; rag 0.4.2) and released date in agent.json and its own CHANGELOG.md; the shared metadata no longer falls back to the pack version (AV.1)

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
