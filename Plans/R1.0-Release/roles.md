# Roles

Status: **partly built.** How a role reaches an Agent, the generated `owner`
and `maintainer` roles and an Agent's supported list are built in 0.8.97; the
contract is [agents § roles](../../docs/03-records-agent.md#roles). Assigned
roles in allow lists ([role syntax](#role-syntax)) are pending, and composing
groups moved to [R1.3](../R1.3/composed-groups.md#group-expressions).

## Record-defined roles

Record-defined roles are R1's first topic: the owner moved them out of MVP on
2026-09-22.

- **Groups** stay flat, as [MVP](../../docs/01-identity-and-authority.md#groups)
  built them: composing groups with `& | !` moved to
  [R1.3](../R1.3/composed-groups.md#group-expressions) on 2026-09-30, so R1's
  roles ship without an expression engine (Q138).
- **Roles** — *what a principal may do*: record-defined strings (`admin`,
  `read_only`, …) written in parentheses after the term
  ([role syntax](#role-syntax)), assigned to flat terms. The daemon stores and resolves them; **it never
  interprets** them — the one place a role goes is the answer to an agent
  asking who its caller is, so there is no code path here that could.
  Maintainer stays the reserved management position, and a role edit must never
  let a Maintainer remove or replace another Maintainer.
- Access and authority stay two layers.

## Role syntax

**Roles go in parentheses after the term, and are left out when there are
none**: `parf@github(admin)`, `@dev(deploy, read_only)`, `*(guest)`,
`#batcher@srv1`. The terms themselves are the
[sigils](../R1.1/identity.md#sigils); a role is handed to the service exactly as
written — it is that service's own vocabulary, not ours.

## Roles are for Agents

*Built in 0.8.97, except assigned roles; the [contract](../../docs/03-records-agent.md#roles) is current. This section keeps the owner's decisions.*

**A role is what a caller holds toward an Agent, and the Agent is told it on
every message it serves** (owner, 2026-09-30). The daemon works the roles out
at delivery from the record's allow list; nothing a sender writes is a role.

| | |
|---|---|
| Names | lowercase letters, digits and `_`, used as written: `admin`, `read_only`. Nothing is converted, so a name with any other character is refused where it is assigned |
| On every message | the delivered envelope carries `roles: [...]`, for any reader — an MCP session, a long-lived child, a script |
| As environment | a process `agent-bus start` spawns for one message also gets `AB_ROLE_<NAME>=1` for each, the name upper-cased: `read_only` is `AB_ROLE_READ_ONLY=1`. A role not held is absent, and the runner clears every `AB_ROLE_*` it inherited, so none leaks into a child. A kept child (`--in-flight`) reads the envelope field |
| Generated | `owner` and `maintainer`: the caller is the Agent's Owner, or one of its Maintainers (through a Group too). The daemon adds them; both names are reserved and cannot be assigned |
| Combining | a caller matched by several terms holds all their roles: `bob(admin)` and `@dev(deploy)` with bob in `@dev` is `admin, deploy`; `*(guest)` adds `guest` for everyone |
| Forwarding | worked out once, at the record the sender addressed (`original_to`), and passed through every forward unchanged — a runner's service behind redirects sees the caller's roles, not the redirect's |
| Supported roles | the Agent's record carries a `roles` list of the roles it understands, which the managed runner registers from its config and anyone may set by hand. Informational for now: it tells the Owner and the administrators what to assign, and nothing is validated or refused against it |
| An Agent opts in | a role does nothing unless the Agent reads it. The daemon never interprets one ([above](#record-defined-roles)) |
| Sender-supplied | a `roles` field or an `AB_ROLE_*` a sender states is refused, never stored and ignored, as any field a sender does not write |

**`owner` and `maintainer` ship first.** They need no new syntax — the daemon
already knows a record's Owner and Maintainers — so a script can gate its
owner-only commands before the allow-list role syntax exists (owner,
2026-09-30).

