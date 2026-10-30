# Daemon configuration (RT22)

## Approved plan

Approved by the owner on 2026-10-20 ("RT22 ok").

The bus setup is the Configuration of the built-in [`#bus`](../../docs/agents/bus.md#what-it-does)
record, edited on the ordinary agent edit page like any agent's (owner,
2026-10-29: "this page is just the setup for the bus agent"). It holds the
default realm, following [RD108](DECISIONS.md#recorded-decisions), and the
[agent-event](agent-events/README.md#levels) ring sizes and TTLs. `#bus`
declares its keys through `help-config`, and the daemon validates a save and
applies it without a restart. Only the daemon Owner writes it; a Maintainer of
`#bus` cannot. The web's left navigation has its own link straight to the
`#bus` Configuration (owner, 2026-10-29). This is a plan, not built behaviour.
An explicit realm remains valid; a realm-less name remains a complete identity
under the [existing name rules](../../docs/01-identity-and-authority.md#names).

## Settings on the first page

| Setting | Proposed behaviour |
|---|---|
| Agent events | Ring size and TTL per level ([RD156](DECISIONS.md#recorded-decisions)); a change applies to new entries and trims rings at once. |
| Realm | The first editable setting. Default to the daemon's host name on first initialization; persist the chosen value, including a shorter Owner-selected name on hosts with long names. Use the existing realm grammar; show validation before saving. Never silently truncate. |
| Host, version and effective listeners | Read-only context from existing daemon facts, so the Owner knows which bus is being edited. The host is not the realm. |
| Other settings | Keep network/socket addresses, database and TLS paths, account mappings, syslog and startup/resource options in their current owning interfaces. No general host-file editor or restart button. Link the existing debug-log control rather than duplicating it. |

Authorize both page access and the daemon write as the current daemon Owner;
Administrator, Maintainer and body-claimed roles do not grant it. The web uses
the visitor's credential. Persist in daemon-owned storage, include it in backup
and restore, and audit the actor plus the old and new realm. No daemon restart
is needed. Existing host configuration cannot silently overwrite a saved realm.

## Defaults and host tags

Publish the effective realm in daemon identity/status. Launchers and installers
read it when constructing a **new automatic** agent name. Explicit names,
saved session bindings and names without a realm keep their existing meaning.
If the daemon cannot answer, refuse automatic naming instead of inventing a
hostname realm; explicit naming retains its existing connection checks.

Host tags describe the process's actual host, including a remote standalone
agent's host, rather than deriving location from its realm. Preserve custom
and runtime tags. Spelling (owner, 2026-10-20: "short host tag"): the first label of
the normalized host name (`parf` for `parf.us`). Do not retag existing
records during a realm save.

## Changing the realm

Treat the setting as the default for future automatic names, **not a rename**.
Every existing record name, token, ACL term, route, subscription and saved
session binding stays unchanged and reachable by its old spelling. There is
no alias, bulk rewrite or automatic collision suffix for an explicit name.
Existing old-realm records become explicit legacy names, which RD108 permits.

Before saving, show the old and new default and a short warning: “Existing
names stay as they are; only new automatic names use this realm.” Existing
launchers adopt the new default on a new launch, without renaming a resumed
session. An Owner who wants to migrate old names uses the normal explicit
registration/rename tools and updates peers separately.

## Delivery and acceptance

1. Approve this scope and host-tag spelling; then implement persistence and
   Owner-only read/write checks in the daemon, followed by the web page.
2. Wire new automatic names and host tags into launchers/installers; document
   which processes need a new launch to observe the new default.
3. Check Owner versus Administrator/Agent access, validation, restart and
   backup persistence, concurrent saves, new automatic names, explicit foreign
   realms and resumed sessions. A realm change must leave existing names,
   credentials and delivery unchanged. Use owning authority mutants and the
   normal landing gate; no code is authorized by this plan alone.
