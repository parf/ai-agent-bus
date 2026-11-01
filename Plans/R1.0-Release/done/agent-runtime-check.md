# Agent runtime method diagnostics and selection

## Result

Every exact method-set comparison in `src/acceptance/agent-runtime.py` names the agent, sorted expected and actual sets, and missing/extra methods. The normal setup check exports a real package, removes RAG's `get` and adds `unexpected_runtime_fixture`, then asserts all four diagnostic fields.

Selection already promotes all agent changes through the general L tag; it was not IMAP-only on current main. That rule and `smoke-map.json` remain unchanged, as agreed with the lead. New assertions cover every source agent manifest, including RAG: `go_unit` is normal and the reason is `changed: <manifest>`. A throwaway local Git fixture checks the actual selector output contains `L: normal — changed: src/agents/rag/agent.json` and `check go_unit: normal`. The normal Go race suite includes `TestBundledAgentsRunOutsideCheckout` and `TestBuildShipsAgentRuntimeAndSetupKeepsItRunnable`; its fast runner-reload check is not claimed as their replacement.

The T-25 pre-ready run was fast-only; the general selector already selected the normal runtime tests. This change pins that behavior rather than adding a redundant selector rule.

## Proof

- Removing the diagnostic text compiles and fails `TestAgentRuntimeMethodMismatchNamesAgentAndBothSets`: `method mismatch lacks "rag methods: expected="`, reproducing the empty assertion report (test body 1.668 s).
- Dropping RAG's explicit path-to-L mapping compiles and fails both new selection assertions: `unmapped path` instead of `changed: src/agents/rag/agent.json`. The conservative unmapped-path fallback still runs normal checks; the failure proves the explicit mapping and displayed reason, not the removal of that fallback.
- Both mutants ran only their owning checks under the slow lock. Source restored and lock released immediately; no build error or exception-only failure is credited.
- Restored selector suite: 18/0 in 0.157 s. Restored setup diagnostic plus both exported/installed runtime tests: PASS in 10.289 s. Links and diff checks pass. No shipped behavior or version change.
