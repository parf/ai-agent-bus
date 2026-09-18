# Runtime owner ACL and master removal

📌 **TL;DR:** 0.5.74 adds contextual `@owner` sharing for one direct ownership
cohort and removes the former master access layer.

## Result

`@owner` in a resource ACL admits the direct Owner plus registered Services and
Agents whose direct Owner is the same principal. The lookup is dynamic and one
step: ownership transfer changes the cohort immediately, a service owned by a
service does not walk to a person, and Channels are not members. The term grants
ordinary ACL access only; it never grants management or Maintainer authority.

`@owner` is runtime syntax rather than stored group state. Group creation,
group nesting and Maintainer assignment reject it. Personal services reject it
with every other non-direct-Service ACL entry. A damaged snapshot containing a
stored `@owner` group fails closed. No compatibility migration exists because
the name had no supported earlier group or ACL meaning.

The `ab-claude`, `ab-codex` and `ab-opencode` registration paths merge
`@owner` into the agent record without erasing explicit grants. Existing live
sessions gain it on their next launcher start; restarting only the daemon does
not rewrite their records.

The daemon has no master ACL layer. The startup flag, record and management
fields, CLI/runner options, core map and implicit grant are gone. The daemon
Owner retains node-wide management, discovery, Recent and unfiltered refusal
history through Owner authority, while use of a resource still requires its
resource authority or ACL.

## Checks

Core tests cover the direct Owner, Service and Agent cohort; exclusion of
Channels, indirect ownership, foreign principals and the daemon Owner's
message use; dynamic ownership transfer; access-without-management; Personal,
group and Maintainer refusal; and damaged stored-group state. API, CLI, runner,
MCP and WEB tests cover the removed master surface, `@owner` registration,
plain textarea help and absence from group editing.

Fast smoke passes **489/0**. Fourteen targeted mutations each fail their named
check: removing or widening the cohort, walking ownership twice, granting
management, admitting the term into stored groups, nested membership,
Maintainers or Personal records, trusting impossible stored group state,
restoring daemon-Owner message use, accepting the removed runner field,
erasing explicit launcher grants, overwriting those grants in the transport
fixture, and removing WEB help. Documentation
validation checks **172 files** and **2,905 local links** with zero errors.

The corrected byte-frozen slow smoke passes **610/0**, including vet, race,
all three installed launcher paths and coordinated rename. All **225** frozen
source and harness hashes remain unchanged through the run.

The first slow run finished **609/1** and receives no acceptance credit: its
installed-launcher check found that the message-flow fixture replaced the real
launcher ACL with `*` after registration. The fixture now merges `*` without
erasing `@owner` or any explicit grant; the corrected candidate is re-frozen
and rerun. The final installed-launcher checks observe `@owner` on Claude,
Codex and OpenCode records after their real fixture exchanges.

## Limits

`@owner` changes only the ACL on the record that names it. A caller's reply
inbox remains governed by that inbox's own ACL. The term does not create a
group listing, grant a credential, infer a human owner through a chain or alter
Personal classification.
