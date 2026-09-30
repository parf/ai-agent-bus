# Roles

Status: proposed, not built. Record-defined roles are R1 work, and the first R1
topic after 0.7: the owner moved them out of MVP on 2026-09-22. The expression
syntax and role-transport design below remain proposals; accepting the
capability does not adopt these specific representations.

- **Groups** compose from groups with `& | !` (`@eng & !@contractors`) when
  AUTH is on. Basic flat groups and maintainers are now
  [required MVP](../../docs/01-identity-and-roles.md#groups).
  The expression engine comes with AUTH: `& | !` on the authorization path
  is where a precedence bug grants silently.
- **Roles** — *what a principal may do*: record-defined strings (`admin`,
  `read-only`, …) written in parentheses after the term
  ([sigils](../R1.1/identity.md#sigils)), assigned with the same expression
  pattern as groups. The daemon stores and resolves them; **it never
  interprets** them — the one place a role goes is the answer to an agent
  asking who its caller is, so there is no code path here that could.
  Maintainer stays the reserved management role, and a role edit must never
  let a Maintainer remove or replace another Maintainer.
- Access and authority stay two layers. One expression engine.
