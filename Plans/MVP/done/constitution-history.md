# Constitution history

📌 **TL;DR:** What the [constitution](../../../docs/constitution.md#project-constitution)
replaced when it was adopted for 0.7, and the owner clarifications it grew
from. History only: the constitution and the topic docs are the current
contract, and 0.7 reconciled the topic docs with it.

## What this replaced

Nothing above depends on this section; it exists so a reader of the older model
can see what changed.

| This model | Replaces |
|---|---|
| Users own everything, and an Agent's management authority never makes it an owner | agent-owned records, ownership chains and self-owned non-User records in the topic docs, whose presence there is not evidence that such objects exist |
| write-through ordering | the memory-before-persistence behavior of the 0.6 [persistence-failure contract](../../../docs/04-messaging.md#administrative-crash-recovery) |
| SQLite as the runtime store | the JSON dump, which the clean reinstall does not import |
| `active` and `inactive` | `banned`, and the Disabled switch: `active` is the former `disabled=false`, `inactive` the former `disabled=true` |
| an Agent's `#` inside its canonical name | unprefixed Agent names, which no installation carries into 0.7 |
| a 👥 `allow` holding its members | a separate `members` field |
| a validated env-file `secret` | the rule that `KEY=value` was only a caller convention |
| extended ACL syntax | nothing: existing terms stay valid and `*` is not widened |

### Clarified direction

Owner clarification, September 19, 2026, now owned by the sections beside it.

| Clarification | Owned by |
|---|---|
| Objects belong to Users; an Agent's management authority never makes it an owner | [registry record](../../../docs/constitution.md#-registry-record) |
| Group ownership and Maintainers extend daemon administration rather than replacing it | [group](../../../docs/constitution.md#-group) |
| Persistence follows a write-through cache model | [persistence](../../../docs/constitution.md#persistence-and-loading) |
| `banned` is gone, and record `active` replaces Disabled | [user](../../../docs/constitution.md#-user), [common fields](../../../docs/constitution.md#common-record-fields) |
| ACL syntax is extended, not replaced; the Agent marker is additive | [actor terms](../../../docs/constitution.md#actors-and-ascii-textarea-syntax) |


