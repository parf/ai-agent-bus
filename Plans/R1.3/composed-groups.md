# Composed groups

Status: exploratory, unscheduled. Moved from [R1 roles](../R1.0-Release/roles.md#record-defined-roles)
on 2026-09-30 (owner). Basic flat groups and Maintainers are
[MVP](../../docs/01-identity-and-roles.md#groups) and stay as they are.

## Group expressions

**Groups compose from groups with `& | !`** — `@eng & !@contractors` — when
[AUTH](../R1.1/auth.md#bundle) is on.

| | |
|---|---|
| What it adds | an ACL, Maintainer or role term that is an expression over groups rather than one group, so "engineering but no contractors" is one term instead of a group kept in step by hand |
| Why after AUTH | `& \| !` on the authorization path is where a precedence bug grants silently: one engine, written once with AUTH, rather than one per face |
| What R1 ships instead | flat terms: a User, an Agent, a Group, nested Groups by membership — and [roles](../R1.0-Release/roles.md#record-defined-roles) assigned to those terms, without expressions (Q138) |
| The sigils | the `@` already tells a group from a user in an expression ([R1.1 sigils](../R1.1/identity.md#sigils)) |
