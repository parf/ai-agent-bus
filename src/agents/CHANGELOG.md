# Agent pack changelog

📌 **TL;DR:** Changes to the bundled agents in `src/agents/`, newest first.
The pack has its own SemVer in `src/agents/VERSION`, separate from the bus version.

## 1.0.0 — 2026-11-06

The pack's first version: the bundled agents are versioned separately from the bus. RAG keeps its own app version in its `agent.json`, apart from this one.
An agent change, FileScan and DBSync included, runs that agent's checks and the pack contract, and no longer re-tests the daemon.
