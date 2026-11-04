# Earlier setup transitions

📌 **TL;DR:** Historical transition notes. Use current product documentation for supported operations.

### Earlier upgrade notes

<details>
<summary>History and compatibility cautions for older installations</summary>

These notes describe earlier release transitions, not a promise that the current daemon converts their old snapshots. Check the cited historical runbooks before restoring old state; current state validation remains authoritative.

#### Administrator name migration

Upgrading from the old administrative name migrates membership to the
[protected Administrator group](../../docs/01-identity-and-authority.md#groups).
Existing record Maintainer assignments and ACL references follow the rename.

- If an ordinary group already uses the destination name, it is preserved as `@administrators-legacy` (with a numeric suffix when necessary), and its existing references follow it.
- Neither existing groups nor unresolved record references are reused for that preserved name.
- Its members do not become Administrators.
- The former administrative name cannot be recreated; subsequent starts leave the migrated groups unchanged.

Upgrade the daemon and its clients together: the user directory's administrative
flag is now `administrator`, replacing `maintainer`. Record Maintainer
assignments retain their meaning.

**Downgrade:** an older daemon does not recognize the renamed group as granting
non-owner administrative standing. Its members still retain the access and
record maintenance granted through ordinary group references. Downgrading does
not restore the old name automatically.

#### Record kind upgrade

- From 0.6.0 `kind` is a closed set ([records](../../docs/03-records.md#record-kinds)).
- `generic`, `topic` and the `mode` field are gone, and **a stored record holding any of them is ignored at start and reported** by name, never converted ([what refuses the start](../../docs/constitution.md#what-refuses-the-start)).
- There is no compatibility obligation before 1.2, so nothing is migrated at load time.

- Upgrading a node that has run an earlier line therefore means editing its snapshot while the daemon is stopped: the procedure, the mapping and what it was verified against are in the [cutover runbook](R0.8-MVP/cutover.md#the-procedure).
- Two things to expect even where the kinds look right: a record that becomes 📡 must lose the queue settings every old record carried, and its inbox must be empty.

#### GitHub profile snapshot upgrade

- From 0.5.71, User profiles may retain public provider metadata and one bounded local PNG thumbnail.
- An older daemon ignores those unknown snapshot fields and drops them on its next snapshot write.
- Imported Person name and Email remain because they use the existing profile fields.
- The packaged rollback restores the matching earlier snapshot; do not run an older binary directly against a 0.5.71 snapshot if retaining the optional provider fields matters.

#### Maintainers-list upgrade

- From 0.5.62, a resource stores a list of Maintainers rather than one group.
- Existing string values load as a one-entry list; an empty string becomes an empty list.
- Every non-empty later answer and snapshot writes the array form; an empty list is omitted.
- Legacy noncanonical terms are preserved but remain inert until the Owner replaces the list.
- Registration never imports this authority from caller-supplied metadata.

Upgrade daemon and clients together. The new daemon accepts the legacy string
input during migration, but an older daemon cannot read the array form written
by 0.5.62 and fails to restore that snapshot rather than guessing authority.

#### Nested group upgrade

- From 0.5.57, an `@group` entry inside an ordinary group is a membership edge.
- Earlier snapshots may contain such entries as inert strings.
- Review existing group lists for stray `@`-names before upgrading: a group that exists now, or is populated later, grants its effective members every ACL and Maintainer permission carried by the containing group.

- Cycles terminate without granting anybody unless another path reaches them; unknown groups remain inert.
- `@administrators` stays direct-only and a snapshot that nests a group inside it is refused at startup.
- Personal agents still reject group ACL entries and Maintainer assignments, so this upgrade does not widen their assignment rules; 0.7 revises them through [Personal](../../docs/03-records.md#personal-and-shared).

#### Empty ACL upgrade

From 0.5.44, existing empty ACLs adopt the [restricted default](../../docs/02-access.md#acl);
there is no grandfathered open access. Unrelated callers lose both delivery
access and visibility: listings omit the record and lookup answers as hidden.

- Before upgrading, record Owners should explicitly grant any intended peer access, including access to reply inboxes.
- `*` opts into broad sharing; do not add it merely to silence a refusal.
- This release also stops implicit master access to empty-ACL resources.
- The daemon Owner can discover and manage those resources through separate [node-wide authority](../../docs/01-identity-and-authority.md#daemon-owner), but still cannot use their message interface without an ACL grant.

Fresh automatic registrations by faces and launchers also follow the default;
configured sharing survives [metadata re-registration](../../docs/01-identity-and-authority.md#registration).
The dedicated runner states grants with [its options](../../docs/06-dedicated-runner.md#script-agents).

#### Owner ACL and master removal

- From 0.5.74, `@owner` in an ACL means the record's direct Owner and the agents directly owned by that Owner.
- It is evaluated from current registry ownership and is never stored as a group.
- The `ab-claude`, `ab-codex`, `ab-opencode` and `ab-opencode1` launchers add it while preserving existing explicit grants; already-running launcher sessions acquire it when next restarted.

`@owner` has no stored-group form. Creating or nesting it is refused, and a
snapshot containing a group with that reserved name fails closed.

- The same release removes `--master`, `no_master` and the implicit master ACL grant.
- A daemon Owner still discovers and manages every resource through [node-wide authority](../../docs/01-identity-and-authority.md#daemon-owner), but message access now comes only from record authority or the record's ACL.

#### Daemon ownership upgrade

- `--owner user[@realm]` (or `AGENT_BUS_OWNER`) is required; the daemon no longer derives authority from the OS account name.
- On the first 0.5.55 start it seeds legacy state and is written into the snapshot.
- Later starts trust the stored Owner, so changing the flag does not transfer authority.
- Use the daemon-owner transfer API instead.

The daemon OS account's private socket answers as the current daemon Owner,
not the configured value: a transfer moves it at once, without a restart
([local socket](../../docs/02-access.md#local-socket)). A current snapshot whose stored
Owner is damaged fails startup.

**Downgrade:** an older daemon ignores the durable Owner fields and derives
authority from its configured `--owner` again. Resource ACLs and Administrator
membership remain stored, but daemon ownership reverts to that seed until the
current version returns.

</details>
