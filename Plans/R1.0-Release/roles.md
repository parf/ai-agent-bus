# Roles

Status: proposed, not built. The expression syntax and role-transport design
below remain proposals; accepting the capability does not adopt these specific
representations.

## Record-defined roles

Record-defined roles are R1's first topic: the owner moved them out of MVP on
2026-09-22.

- **Groups** stay flat, as [MVP](../../docs/01-identity-and-roles.md#groups)
  built them: composing groups with `& | !` moved to
  [R1.3](../R1.3/composed-groups.md#group-expressions) on 2026-09-30, so R1's
  roles ship without an expression engine (Q138).
- **Roles** — *what a principal may do*: record-defined strings (`admin`,
  `read-only`, …) written in parentheses after the term
  ([role syntax](#role-syntax)), assigned to flat terms. The daemon stores and resolves them; **it never
  interprets** them — the one place a role goes is the answer to an agent
  asking who its caller is, so there is no code path here that could.
  Maintainer stays the reserved management role, and a role edit must never
  let a Maintainer remove or replace another Maintainer.
- Access and authority stay two layers.

## Role syntax

**Roles go in parentheses after the term, and are left out when there are
none**: `parf@github(admin)`, `@dev(deploy, read-only)`, `*(guest)`,
`#batcher@srv1`. The terms themselves are the
[sigils](../R1.1/identity.md#sigils); a role is handed to the service exactly as
written — it is that service's own vocabulary, not ours.
