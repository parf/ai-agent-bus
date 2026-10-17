# R1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan. [№Q13](DECISIONS.md#recorded-decisions) and [№Q107](DECISIONS.md#recorded-decisions) were settled on 2026-09-30, [№Q12](DECISIONS.md#recorded-decisions), [№Q15](DECISIONS.md#recorded-decisions), [№Q19](DECISIONS.md#recorded-decisions), [№Q33](DECISIONS.md#recorded-decisions), [№Q139](DECISIONS.md#recorded-decisions), Q150–Q158, [№Q162](DECISIONS.md#superseded) and [№Q163](DECISIONS.md#superseded) on 2026-10-16, [№Q145](#open-questions), [№Q159](DECISIONS.md#recorded-decisions), [№Q161](DECISIONS.md#recorded-decisions), [№Q164](DECISIONS.md#recorded-decisions), [№Q165](DECISIONS.md#recorded-decisions) and Q166–Q170 on 2026-10-17 ([decisions](DECISIONS.md#recorded-decisions)); [№Q140](DECISIONS.md#recorded-decisions) was withdrawn the same day, since locks no longer use a Group's membership.

- ✅ Q145 Multiple-tag query matching is comma-delimited and means "and"
  - Settled by: owner, 2026-10-17
  - Home: [record tags](#record-tags)
  > 💬 👤parf comma delimited; treated as "and"
  >
  > 💬👾 Understood: `tag=a,b` selects the records carrying **both**. Three
  > consequences I would carry into the task — an unknown tag gives an empty
  > result rather than an error; `no-tags` cannot combine with a real tag, since
  > a record either has no tags or has that one; and the existing single-tag
  > request is unchanged, so no page or client breaks. Acceptance: two tags
  > return only records holding both. Mutant: treat the list as *any* and that
  > check must fail.
  >
  > 💬 👤parf ok - close this
  >
  > 💬👾 Closed. `tag=a,b` selects records carrying every tag listed; a single
  > tag is the same request it is today, so nothing built changes meaning. The
  > query itself is not built — the current interface selects one label — so
  > the work is an R1 task carrying the acceptance above, and the contract
  > stays [record tags](../../docs/03-records.md#tags).

- ❓ Q20 A chaining namespace and an agent template both want the `/`
  - Settled by: owner, with chaining
  - Home: [federation § chaining](federation.md#chaining)
  - (x) The realm already is the namespace — `#ci@team`, `#mail@company.upstream`; no grammar change, and `team/ci` is deleted from the plan 🌟
  - ( ) Keep `team/ci` and give chaining its own separator from `.`, `_`, `-` ⭐
  - ( ) Drop namespaced ids entirely; resolution order decides which node answers
  > 💬 👤parf elaborate
  >
  > 💬 👤parf come with ideas
  >
  > 💬👾 Three ways out, and the first costs nothing. **1. The realm already is
  > the namespace.** [Names](../../docs/01-identity-and-authority.md#names) says
  > the realm "identifies a namespace, not the process's physical location", it
  > excludes `/` and `@`, and it allows `.` — so `#ci@team` and
  > `#mail@company.upstream` need no new separator and no grammar change. The
  > same section says the template is "part of the identity, not a lookup", and
  > a chaining namespace is a lookup concern by definition, so `team/ci` is in
  > the wrong slot by the document's own rule. **2.** Keep `team/ci` and give
  > chaining a separator from the free set (`.`, `_`, `-`): a grammar change,
  > and every written name migrates. **3.** Drop namespaced ids altogether —
  > resolution order already decides which node answers, so the name needs no
  > prefix. I would take 1 and delete the `team/ci` notation from the plan.
  >
  > 💬 👤parf 1
  >
  > ✅ 💡👾 Recommend the realm-as-namespace option 🌟. It uses the existing name grammar and keeps template identity distinct from lookup. A separate chaining separator is the runner-up ⭐ if realm naming cannot express the required scope; it would need a separately approved grammar change.
  >
  > 💬 👤parf agree


- ❓ Q17 How `protocol` is specified across client languages
  - Settled by: owner, with data models
  - Home: [client libraries](done/client-libraries.md#scope)
  - ( ) A conformance suite as the specification — one shared fixture of request and answer pairs every client must reproduce 🌟
  - ( ) A prose contract in `docs/`, one section per call ⭐
  - ( ) A schema and a generator (JSON Schema or an IDL)
  > 💬 👤parf come with ideas.
  > for now we support what we know
  >
  > 💬👾 Taking "support what we know" as the constraint, three forms, cheapest
  > first. **1. A conformance suite as the specification.** The six clients
  > exist; one shared fixture of request and answer pairs that every client must
  > reproduce defines the protocol by example, fails loudly the moment a client
  > drifts, and introduces no new language. **2. A prose contract** in `docs/`,
  > one section per call: it reads well, it cannot be executed, and it rots
  > without anyone noticing. **3. A schema and a generator** — JSON Schema or an
  > IDL — the only form that removes hand-written parsing, and also the most
  > work, and it fixes a wire format before one has been asked for. I would
  > start at 1, because it costs a fixture and a check and it is what makes 3
  > safe later: a generator is only trustworthy once something independent says
  > what its output must do.
  >
  > ✅ 💡👾 Recommend the conformance suite as the first authority 🌟, starting with the calls the six clients already support. It makes disagreement executable. A prose contract is the runner-up ⭐ for explaining those calls; it can accompany the tests without becoming a second wire authority. Defer a generator until its output can be checked against independent examples.
  >
  > 💬 👤parf agree


- ❓ Q137 Whether R1's chaining is a lookup-only fallthrough without AUTH, signed generations and peer sync, or waits for R1.1
  - Settled by: owner, when federation comes off hold
  - Home: [federation](federation.md#chaining)
  > 💬 👤parf move federation for R1.2
  >
  > 💬👾 Understood: federation leaves R1 for R1.2, which answers the question
  > by removing it — R1 has no chaining, so it needs nothing from R1.1. What
  > moves with it: the topic file, its Dependencies item, the acceptance line
  > that federation must be exercised across distinct nodes, and №Q20, since
  > the `/` conflict only bites once chaining exists. R1's scope row and the
  > Next step paragraph lose their federation clause. The R1 plan keeps one
  > line saying where it went, so the history stays followable.

- ❓ Q146 Confirm the implemented assigned-role representation, duplicate actor grants and applicable record kinds
  - Details: Confirm these aspects independently with the checkboxes. An unchecked aspect is not approval; name what should change in a reply.
  - Settled by: owner
  - Home: [assigned roles](#assigned-roles)
  - [x] Keep role assignments in existing `allow` terms such as `bob(deploy)` and `@dev(deploy, read_only)`
  - [x] Keep separate same-actor grants and union their matching roles; remove a bare actor across its grants, or a decorated grant by its role set
  - [ ] Permit assignments on ACL-bearing records, excluding User records, Group membership and Maintainers
  > 💬 👤parf elaborate
  >
  > 💬👾 What is built today, so there is something concrete to confirm or
  > reject. **Representation:** labels in parentheses after an allow-list term,
  > in the existing `allow` list and not a new field — `bob(admin)`,
  > `@dev(deploy, read_only)`, `*(guest)`. **Duplicate grants for one actor:**
  > `bob(deploy)` and `bob(read_only)` may both stand and their labels combine;
  > an identical repeated term is stored once; terms differing only in role
  > order can remain separate on addition, delivery deduplicates the labels,
  > and removal matches the whole role set. **Where they are allowed:** any
  > record with an ACL, including a queue or pubsub channel that forwards to an
  > Agent — not Group membership or the Maintainers list, which take actors
  > only, and not User records, which have no ACL. **Order on the message:**
  > generated roles first, then assigned, each label once, in first-match
  > order. **Reserved to the daemon:** `owner` and `maintainer` are generated,
  > and assigning either in an ACL rejects the whole save. Source:
  > [roles](../../docs/03-records-agent.md#roles).
  >
  > ✅ 💡👾 Recommend confirming all three aspects together. They match the implemented, tested behavior and preserve existing role-free ACLs without another stored representation. Generated `owner` and `maintainer` remain reserved. If one aspect is unsuitable, leave it unchecked and explain the desired behavior before changing the contract.
  >
  > 💬 👤parf agree


- ❓ Q160 With `in_flight` above 1, answers cannot be matched by topic and tag, which are the caller's labels and need not be unique. Does each frame carry the message ID, echoed on its answer, amending [№Q13](DECISIONS.md#recorded-decisions)?
  - Settled by: owner, before [№M.3](TODO.md#runner-waves)
  - Home: [long-lived agents](runner-implementation.md#long-lived-agents)
  - ( ) Echo the existing bus message ID in each frame and answer; keep topic/tag as caller labels 🌟
  - ( ) Echo a runner-assigned per-process frame number ⭐
  - ( ) Keep `in_flight` at 1; use more processes for throughput
  > 💬 👤parf propose ideas
  >
  > 💬👾 Three ways, the first recommended. **a.** Each frame the runner writes carries the message's own bus ID, already unique and already travelling with it, and the process echoes it on its answer; topic and tag stay the caller's labels (amends [№Q13](DECISIONS.md#recorded-decisions)). **b.** The runner numbers the frames it gives one process, 1, 2, 3, and the answer echoes the number: shorter, but a second identity beside the ID. **c.** No `in_flight` above 1: one message per kept process at a time, answers in order, more processes for throughput. Say a, b or c.
  >
  > ✅ 💡👾 Recommend echoing the existing bus message ID 🌟: it already identifies the message and works when answers finish out of order. A per-process number is the runner-up ⭐, but creates another identity and needs lifetime rules. Keep topic and tag unchanged; the exact frame syntax remains runner-format work after the choice.
  >
  > 💬 👤parf agree


- ❓ Q147 Confirm retaining current nested-group behavior for roles instead of the older flat-groups wording
  - Settled by: owner
  - Home: [assigned roles](#assigned-roles)
  - ( ) Confirm existing nested membership with iterative traversal, processed-group tracking and the protected direct-only Administrator boundary 🌟
  - ( ) Keep the iterative algorithm requirement, but revisit the nested-membership policy before confirming it ⭐
  > 💬 👤parf elaborate
  >
  > 💬 👤parf groups should be resolved with algo (avoid recursion, mark processed groups)
  >
  > 💬👾 Confirmed, and the requirement is already met in code rather than
  > only intended. `internal/core/assigned_roles.go:71-91` expands membership
  > with an explicit `todo` stack and a `visited` map — not recursion — and a
  > group already processed is skipped, which is what makes the documented
  > "including cycles" true instead of hopeful. The walk also stops at
  > `AdministratorsGroup`, so the protected group stays direct-only. So nested
  > membership is retained, the older flat-groups wording is retired, and
  > group expressions stay [R1.3](../R1.3/composed-groups.md#group-expressions).
  > Nothing changes in code; the plan wording is what moves.
  >
  > 💡👾 Recommend confirming the existing nested-membership behavior 🌟 while retaining the explicit non-recursive traversal and processed-group tracking already requested. That handles cycles without changing who receives roles. The runner-up ⭐ is to leave membership policy open while keeping the algorithm requirement; it needs a concrete access example showing what should differ.


## Record tags

Context for №Q145 above; the editable question remains under Open questions.

№Q145: if a future query selects multiple tags, should they match any or all?
The current interface selects one tag. Normalization, the synthetic `no-tags`
label and filtered counts are settled in [record tags](../../docs/03-records.md#tags).
*Settled by:* owner, with R1.

## Assigned roles

Context for №Q146 and №Q147 above.

№Q146: the implementation uses existing `allow` strings, preserves separate
same-actor grants, and supports assignments on records with ACLs, excluding
User records and Group membership. Owner confirmation of this representation
remains open; the implemented behavior is documented in
[roles](../../docs/03-records-agent.md#roles).

№Q147: the implementation preserves current nested-group membership, including
the protected Administrator boundary. Confirm the older plan's flat-groups
wording should be retired; group expressions remain R1.3 scope.

## Federation context


**A namespace and an agent template both want the `/`.** A name holds at
most one, and it already means *template* / *instance*
([identity § names](../../docs/01-identity-and-authority.md#names)), so `team/ci@realm` parses as
template `team`. Either a chaining namespace *is* the template part, or
chaining needs a separator of its own. *Settled by:* the owner, when chaining
is designed.

## Split context

**R1 topics written on R1.1 machinery.** The 2026-09-30 split kept these
topics in R1, but each was written against something now in R1.1, which ships
later. Chaining pins upstream keys and takes signed AUTH generations (№Q137).
(Pools' host field moved to R1, [№Q139](DECISIONS.md#recorded-decisions).) For chaining, either the R1 version is cut down to stand alone, or the
machinery moves to R1, or the topic waits for R1.1. *Settled by:* owner.

## Runner context

## Agents for the runner

`RQ` labels an existing owner choice without an allocated Q number; it does not allocate one.

Settled, all by the owner on 2026-10-17
([decisions](DECISIONS.md#recorded-decisions)): [№Q168](DECISIONS.md#superseded), the systemd agent is one
name with three tiers; [№Q169](DECISIONS.md#superseded), the telegram token comes from the record's bus
secret and nothing reads `~/.codex/telegram.toml`; [№Q170](DECISIONS.md#superseded), a privileged agent is
standalone and `agent.json` may ask for less but never more. What each agent
adds to its catalogue entry is in
[agents for the runner](agents.md#what-moved-and-what-r1-adds), and the
acceptance in [its tasks](TODO.md#bundled-agents). The tier name for
`start`/`restart`/`reload` is the one thing still the owner's, and `operate`
is the [proposal](agents.md#open-choices).

- ❓ RQ1 What should the systemd start/restart/reload role tier be named?
  - Details: The tier name for `start`/`restart`/`reload` remains the owner's choice; `operate` is the proposal. No Q number has been allocated.
  - Settled by: owner
  - Home: [open choices](agents.md#open-choices)
  > 💬 👤parf ok
  >
  > 💬👾 Reading that as yes to the proposal: the tier is named `operate`, so
  > systemd's three tiers are no role (`list`, `status`), `operate` (adding
  > `start`, `restart`, `reload`) and `admin` (adding `stop`, `enable`,
  > `disable`). It is a legal role name — lowercase ASCII letters only, within
  > [the alphabet](../../docs/03-records-agent.md#roles) — and it reaches a
  > caller as `AB_ROLE_OPERATE=1`. Say otherwise and I will change it; the
  > follow-up is to write the name into [agents](agents.md#systemd) and record
  > the decision, which I will do as its own commit rather than inside this
  > answering pass.


## Modules context

**How `protocol` is specified across client languages** — a document, a shared
schema, or a generator? Nothing can be reimplemented consistently until this is
answered, and it is the gate on the client libraries. *Settled by:* owner, when
data models are taken up.
