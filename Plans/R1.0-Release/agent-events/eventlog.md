# Eventlog agent

📌 **TL;DR:** the built `#eventlog` keeps named logs under each instance's
existing ACL. R1 agent logs share its model and verbs, while its deployed
engine and stored logs remain unchanged. A single daemon engine and compatible
front are the [post-1.0 transition](README.md#eventlog-front-and-existing-state).

## What it does

The [current agent guide](../../../docs/agents/eventlog.md#what-it-does) owns
write/read/list/pull/clean and subscription behavior. Named-log instances keep
their current roles; that does not grant a reader role on private agent logs.
Historical B.28 implementation evidence stays in [DONE](DONE.md#shared-rings).

## Rules

The one specification is the [shared log model](../../../docs/agents/eventlog.md#log-model).
It owns levels, ring defaults/bounds, TTL, ids, normalization and repeat folding.
The guide's [storage section](../../../docs/agents/eventlog.md#technical-details)
owns today's alert persistence. Agent-log retention options and their costs are
in the [daemon design](README.md#flood-and-persistence); no second defaults table
is maintained here.

## Same code

Today's engine is `src/agents/shared/event_rings.py`, exercised by
`src/agents/eventlog/test_rings.py` and `src/testdata/event-rings.json`.
The R1 Go engine must run the [shared model vectors](README.md#drift-guard).
This is temporary model/verb unification, not an assertion that Go imports
Python or that existing named logs have already moved into the daemon.
