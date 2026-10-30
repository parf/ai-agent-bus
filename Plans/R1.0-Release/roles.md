# Roles

Status: built ahead of R1 in 0.8.110. The current
[role contract](../../docs/03-records-agent.md#roles) owns syntax, generated and
assigned roles, propagation and script usage.

## Record-defined roles

Moved from MVP to R1 on 2026-09-22; implemented on the 0.8 line.
Roles use current nested groups; composing groups with expressions stays
[R1.3](../R1.3/composed-groups.md#group-expressions).

## Role syntax

The [current contract](../../docs/03-records-agent.md#roles) defines assignments
in the existing allow list; there is no new endpoint or storage schema.

## Roles are for Agents

The [current contract](../../docs/03-records-agent.md#roles) describes what an
Agent receives and how it uses roles. Application labels grant no bus authority.

Execution and review: [implementation checklist](done/roles-implementation.md#assigned-roles-implementation).
