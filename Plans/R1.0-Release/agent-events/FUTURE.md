# Agent event follow-ups

📌 **TL;DR:** follow-ups from the B.28/B.29 review. None widens the accepted
one-attempt subscription delivery rule or adds replay/idempotency guarantees.

## Subscription failure classification

T-2 QA low, 2026-10-30: reproduce any distinction lost when transport or
lookup failures become `admission_unavailable`; improve the fixed warning code
if evidence warrants it. A transient unavailable check still drops that match
by the accepted [delivery rule](../subscriptions.md#one-behaviour); never add
retry or hold state as a cosmetic repair.

## Repeated caller requests

T-2 QA low, 2026-10-30: a 200-message seen cache cannot promise replay
protection after eviction. The new eventlog adapter has no such cache; it
consumes each bus request once and treats each accepted `write` as an
occurrence, including a folded occurrence. If callers need request
idempotency across reconnect/restart, obtain an explicit contract and durable
bound before adding it. Do not describe a bounded memory cache as persistent
deduplication.
