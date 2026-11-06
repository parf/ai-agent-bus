# Agent pack changelog

📌 **TL;DR:** Changes to the bundled agents in `src/agents/`, newest first.
The pack has its own SemVer in `src/agents/VERSION`, separate from the bus version.

## 1.0.3 — 2026-11-06

FileScan: finite on-demand `sync` (bounded, resumable, removes files gone from the folder) and `as rag` large files by bigfile reference; RAG 0.4.2: chunked large-file upsert, completeness `check` (T-48)

## 1.0.2 — 2026-11-06

RAG 0.4.1: count, similar, complete (prefix index via keyword_index) and classify (off unless configured); agent method sets come from each agent.json.

## 1.0.1 — 2026-11-06

Agents declare their daemon protocol (protocol.json); RAG 0.4.0 with optional rerank; FileScan renamed changes, bigfile opt-in and safer RAG confirmation; DBSync shares the fixed change-sync core.

## 1.0.0 — 2026-11-06

The pack's first version: the bundled agents are versioned separately from the bus. RAG keeps its own app version in its `agent.json`, apart from this one.
An agent change, FileScan and DBSync included, runs that agent's checks and the pack contract, and no longer re-tests the daemon.
