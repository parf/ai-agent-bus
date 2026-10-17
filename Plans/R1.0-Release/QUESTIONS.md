# R1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan. [№Q13](DECISIONS.md#recorded-decisions) and [№Q107](DECISIONS.md#recorded-decisions) were settled on 2026-09-30, [№Q12](DECISIONS.md#recorded-decisions), [№Q15](DECISIONS.md#recorded-decisions), [№Q19](DECISIONS.md#recorded-decisions), [№Q33](DECISIONS.md#recorded-decisions), [№Q139](DECISIONS.md#recorded-decisions), Q150–Q158, [№Q162](DECISIONS.md#superseded) and [№Q163](DECISIONS.md#superseded) on 2026-10-16, [№Q159](DECISIONS.md#recorded-decisions), [№Q164](DECISIONS.md#recorded-decisions) and Q166–Q170 on 2026-10-17 ([decisions](DECISIONS.md#recorded-decisions)); [№Q140](DECISIONS.md#recorded-decisions) was withdrawn the same day, since locks no longer use a Group's membership.

- ❓ Q145 Multiple-tag query matching
  - Settled by: owner, with R1
  - Home: [record tags](#record-tags)
  > 💬 👤parf comma delimited; treated as "and"

- ❓ Q20 A chaining namespace and an agent template both want the `/`
  - Settled by: owner, with chaining
  - Home: [federation § chaining](federation.md#chaining)
  > 💬 👤parf elaborate
  >
  > 💬 👤parf come with ideas

- ❓ Q17 How `protocol` is specified across client languages
  - Settled by: owner, with data models
  - Home: [client libraries](done/client-libraries.md#scope)
  > 💬 👤parf come with ideas.
  > for now we support what we know

- ❓ Q137 Whether R1's chaining is a lookup-only fallthrough without AUTH, signed generations and peer sync, or waits for R1.1
  - Settled by: owner, when federation comes off hold
  - Home: [federation](federation.md#chaining)
  > 💬 👤parf move federation for R1.2

- ❓ Q146 Confirm the implemented assigned-role representation, duplicate actor grants and applicable record kinds
  - Settled by: owner
  - Home: [assigned roles](#assigned-roles)
  > 💬 👤parf elaborate

- ❓ Q160 With `in_flight` above 1, answers cannot be matched by topic and tag, which are the caller's labels and need not be unique. Does each frame carry the message ID, echoed on its answer, amending [№Q13](DECISIONS.md#recorded-decisions)?
  - Settled by: owner, before [№M.3](TODO.md#runner-waves)
  - Home: [long-lived agents](runner-implementation.md#long-lived-agents)
  > 💬 👤parf propose ideas

- ❓ Q161 An env file change cannot reach a running process by `SIGHUP`: env is set only at exec. Does a changed env (or secret) restart the agent's kept processes after their work drains, while override files and bus `config` keep `SIGHUP`, amending [№Q19](DECISIONS.md#recorded-decisions)?
  - Settled by: owner, before [№M.5](TODO.md#runner-waves)
  - Home: [stop and reload](runner.md#stop-and-reload)
  > 💬 👤parf yes - restart

- ❓ Q165 A person's runner is `#runner/<person>@<realm>` ([№M.10](TODO.md#runner-waves)), and an edge runner `#runner/<edge>@<realm>` ([№Q15](DECISIONS.md#recorded-decisions)): one slot, two meanings. Which names which, or do they compose (for example `#runner/<person>-<edge>`)?
  - Settled by: owner, before [№M.6](TODO.md#runner-waves)
  - Home: [who it runs as](runner-implementation.md#who-it-runs-as), [where it runs](runner-implementation.md#where-it-runs)
  > 💬 👤parf runner/$user_or_edge@host   - user is just a suggestion

- ❓ Q147 Confirm retaining current nested-group behavior for roles instead of the older flat-groups wording
  - Settled by: owner
  - Home: [assigned roles](#assigned-roles)
  > 💬 👤parf elaborate
  >
  > 💬 👤parf groups should be resolved with algo (avoid recursion, mark processed groups)

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


## Modules context

**How `protocol` is specified across client languages** — a document, a shared
schema, or a generator? Nothing can be reimplemented consistently until this is
answered, and it is the gate on the client libraries. *Settled by:* owner, when
data models are taken up.
