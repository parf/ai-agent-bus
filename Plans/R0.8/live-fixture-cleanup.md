# Historical live fixture cleanup

📌 **TL;DR:** Three smoke names remain as active live identities without stored
credentials. They came from historical fixtures and hold no authority beyond
their own records, and removing them needs the Owner's explicit authority
decision.

## Measured state

**Resolved 2026-09-23:** after the 0.7 reinstall none of these names exists on the
live node; the record below is history.

Read-only inspection on 2026-09-17 found:

| Name | Profile and record | Authority | Stored entry points |
|---|---|---|---|
| `plain@srv1` | Active profile; self-owned Agent record from 2026-09-16 11:22 EDT | None beyond its own record | No installed SSH key or durable token |
| `piped@srv1` | Active profile; self-owned Agent record from 2026-09-16 11:22 EDT | None beyond its own record | No installed SSH key or durable token |
| `chief@srv1` | Active profile; self-owned Agent record from 2026-09-16 11:22 EDT | Direct member of `@administrators` | No installed SSH key or durable token |

`@administrators-legacy` is empty. No state was changed during this review.

## Provenance

The former smoke `adm` helper set an isolated `AGENT_BUS_HOME` but did not set
`AGENT_BUS_ADDR`. Its temporary keys stayed in scratch storage, while
`agent-bus-admin` discovered the production Owner socket. Each `user add`
therefore called production `/user`; the `--admin` case also updated the
production Administrator group. Those writes create exactly the durable profile,
self-owned record and direct Administrator membership observed above.

The helper is now bound to the disposable daemon and uses distinct
`smoke-admin-*` names. The [migration evidence](done/administrator-names.md#live-verification-and-harness-correction)
records that correction and its acceptance checks.

## Cleanup boundary

Removing `chief@srv1` from `@administrators`, changing any of the three profile
states, or retiring their self-owned records changes live authority or identity.
The Owner must choose those actions. The absence of current stored credentials
does not authorize silent cleanup.
