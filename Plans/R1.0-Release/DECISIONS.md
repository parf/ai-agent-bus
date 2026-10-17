# R1 decisions

## Recorded decisions

`RD` labels the original unnumbered decision-index rows; it allocates no Q IDs.

Migrated 2026-09-13 and split from the old R1 on 2026-09-30. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

- 📌 RD82 Plans adopt the mdhouse item profile without losing plan knowledge
  - Why: owner instruction to commit the profile and convert R1 documents
  - Date: 2026-10-17
  - Home: [plan markup](../MARKUP.md#an-item)

- ✅ RD1 R1 Runners web face wraps runner APIs and links to existing agent pages
  - Earlier rows: 2026-10-17 owner request; planned, not built
  - Home: [Runners web face](runner/web-face.md#goal)

- 🔵 RD2 Replace Personal classification with record tags (proposed)
  - Earlier rows: 2026-10-01 owner proposal
  - Home: [record tags](tags.md#classification-and-filtering)

- ✅ RD3 Retain Personal rules on the tag
  - Earlier rows: 2026-10-01 owner clarification; resolves №Q144
  - Home: [record tags](tags.md#classification-and-filtering)

- ✅ RD4 Record tags use comma-only normalized labels; Personal rules retained
  - Earlier rows: 2026-10-17 owner instruction
  - Home: [automatic tags](tags.md#automatic-tags)

- ✅ RD5 R1's scope is settled, and the old R1 splits into R1 and [R1.1](../R1.1/README.md#scope) (№Q18)
  - Earlier rows: 2026-09-30 owner split; tools and exploration renumbered to R1.2 and R1.3
  - Home: [R1 scope](README.md#scope)

- ✅ RD6 The runner's children are agents, not services; each is read on its own inbox as itself
  - Earlier rows: 2026-10-16 owner review; reading revised 2026-10-17 by [№M.10](TODO.md#runner-waves)
  - Home: [many names into one inbox](runner-implementation.md#many-names-into-one-inbox)

- ⚠️ RD7 Assigned roles keep the existing allow-list representation and current nested membership
  - Earlier rows: 2026-10-16 implementation choice under the start instruction; owner confirmation remains [№Q146/Q147](QUESTIONS.md#open-questions)
  - Home: [current roles](../../docs/03-records-agent.md#roles)

- ✅ RD8 Record-defined roles are R1's first topic (№Q98)
  - Earlier rows: 2026-09-22 owner decision; moved out of MVP; [№D249](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [roles](roles.md#record-defined-roles)

- ✅ RD9 R1's roles ship as flat terms; composing groups with `& | !` moved to R1.3 (№Q138)
  - Earlier rows: 2026-09-30 owner decision
  - Home: [roles](roles.md#record-defined-roles), [composed groups](../R1.3/composed-groups.md#group-expressions)

- ✅ RD10 Roles for Agents, generated roles and delivery
  - Earlier rows: 2026-09-30 owner decisions; naming revised 2026-10-16
  - Home: [roles are for Agents](roles.md#roles-are-for-agents)

- ✅ RD11 Opaque application roles with the existing shell-safe naming alphabet
  - Earlier rows: 2026-10-16 owner clarification; resolves №Q148
  - Home: [roles are for Agents](roles.md#roles-are-for-agents)

- ✅ RD12 Generated role names cannot be assigned in ACLs; reject the save
  - Earlier rows: 2026-10-16 owner confirmation; resolves №Q149
  - Home: [roles are for Agents](roles.md#roles-are-for-agents)

- ✅ RD13 Locks are held in memory and never stored
  - Earlier rows: 2026-09-22 owner decision; built in 0.8.66; [№D64](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D65](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D66](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Sets of interchangeable locks moved to [R2.0](../R2.0-Future/lock-sets.md#a-set-of-locks) on 2026-09-30
  - Home: [shared locks](../../docs/01-identity-and-authority.md#shared-locks)

- ✅ RD14 A key-value store per record, set with `kv_set(record, name, value, how)`, `how` one of set, add, replace; its Owner, Maintainers and own Agent read and write it (№Q107)
  - Earlier rows: 2026-09-22 owner instruction; tied to a record and its authority 2026-09-30
  - Home: [key-value store](done/kv.md#per-record-storage)

- ✅ RD15 Three kinds of value, string (the default), int and JSON, each its own table and namespace keyed by the record's internal ID, with get, set and delete each, plus `kv_int_inc` and `kv_json`
  - Earlier rows: 2026-09-30 owner decision; no timestamps. For owner review: a write commits before it is answered
  - Home: [key-value store](done/kv.md#per-record-storage)

- ✅ RD16 A `json` value takes `set`, `unset`, `inc`, `push`, `unshift`, `shift`, `pop`, `add_to_set` and `remove_from_set` on a value's top-level keys, a list applied all or none
  - Earlier rows: 2026-09-30 owner decision; no conditional op, the shared lock covers that; top-level keys only, not JSON Pointer paths
  - Home: [JSON operations](../../docs/03-records-kv.md#json-operations)

- ✅ RD17 MCP Resource and Resource Template become registry records
  - Earlier rows: 2026-09-23 owner instruction
  - Home: [Resource records](done/resources.md#resource-records)

- ✅ RD18 A Resource is information: a card like a Service, one `resource` kind with a template flag, 📄 and 🧩 (№Q110), carrying every MCP descriptor field (№Q111)
  - Earlier rows: 2026-09-30 owner decisions, against the 2026-07-28 specification
  - Home: [Resource records](done/resources.md#resource-records)

- ✅ RD19 A card's source is an Agent (a bus request) or an MCP Service (forwarded); the face fetches an `https://` card itself; the daemon serves no content (№Q114, №Q113)
  - Earlier rows: 2026-09-30 owner decisions
  - Home: [Resource records](../../docs/03-records-resource.md#what-a-resource-is)

- ✅ RD20 Federation stays R1 scope, on hold
  - Earlier rows: 2026-09-30 owner decision
  - Home: [federation](federation.md#chaining)

- ✅ RD21 Chaining
  - Earlier rows: [№D80](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](federation.md#chaining)

- ✅ RD22 Modules
  - Earlier rows: [№D178](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](modules.md#modules)

- ✅ RD23 Blocking lazy private-value properties in the Python client
  - Earlier rows: Owner accepts blocking reads; other execution choices remain open
  - Home: [lazy private values](done/client-libraries.md#lazy-private-values)

- ✅ RD24 Java joins the client-library scope
  - Earlier rows: Owner choice
  - Home: [client libraries](done/client-libraries.md#scope)

- ✅ RD25 Implement the reviewed clients against the current daemon contract
  - Earlier rows: Owner authorized implementation after the plan review ring
  - Home: [review sequence](done/client-libraries.md#review-sequence)

- ✅ RD26 An agent's `processes` is a fixed count or a `min..max` range; the runner stops a process with `SIGTERM` and reloads it with `SIGHUP`, for every agent (№Q150, №Q12, №Q156)
  - Earlier rows: 2026-10-16 owner decision; on demand is `min` 0, through the runner's forwarding
  - Home: [runner § kinds of agents](runner.md#kinds-of-agents)

- ✅ RD27 An agent whose configuration says it serves itself is not registered by the runner; the runner starts it and restarts it when it dies (№Q151)
  - Earlier rows: 2026-10-16 owner decision
  - Home: [runner § the rule](runner.md#the-rule)

- ✅ RD28 A process above `min` idle for 5 minutes, by default, is stopped (№Q154)
  - Earlier rows: 2026-10-16 owner decision; the host may change it
  - Home: [runner § stop and reload](runner.md#stop-and-reload)

- ✅ RD29 Every managed agent is sandboxed by default, in a cgroup of the runner's delegated subtree (`Delegate=yes`), not through a user manager (№Q157)
  - Earlier rows: 2026-10-16 owner decision
  - Home: [runner § sandboxing](runner.md#sandboxing)

- ✅ RD30 A host overrides a shipped JSON file with its own file of the same name and JSON, per agent and per instance, its values winning (№Q152)
  - Earlier rows: 2026-10-16 owner decision; merge and delivery proposed under №Q155
  - Home: [runner § overriding a shipped file](runner.md#overriding-a-shipped-file)

- ✅ RD31 The runner's env files win over the record's bus secret: local over remote (№Q153)
  - Earlier rows: 2026-10-16 owner decision
  - Home: [runner § secrets](runner.md#secrets)

- ✅ RD32 The agent configuration options and their homes, a JSON Merge Patch override that changes only run options, and `agent.json`, `enabled`, `processes` and the host's `name` replacing `config.json`, `autostart`, `-N` and an author-given name (№Q155)
  - Earlier rows: 2026-10-16 owner decision
  - Home: [agent configuration](runner-config.md#where-an-option-lives)

- ✅ RD33 Bubblewrap applies what a cgroup does not: filesystem, privilege, a namespace per agent, a filtering proxy for named hosts, a sized `/tmp` and a soft work-directory limit (№Q158)
  - Earlier rows: 2026-10-16 owner decision; not an account per agent
  - Home: [runner implementation § sandboxing](runner-implementation.md#sandboxing)

- ✅ RD34 An edge runner with no daemon takes the bus's realm, `#runner/<edge>@<realm>` (№Q15)
  - Earlier rows: 2026-10-16 owner decision
  - Home: [where it runs](runner-implementation.md#where-it-runs)

- ✅ RD35 A configuration change waits for `reload`, which signals each changed agent's kept processes (№Q19)
  - Earlier rows: 2026-10-16 owner decision
  - Home: [stop and reload](runner.md#stop-and-reload)

- ✅ RD36 A backup is a runner verb, `backup` (№Q33)
  - Earlier rows: 2026-10-16 owner decision
  - Home: [backing it up](runner-implementation.md#backing-it-up)

- ✅ RD37 A pool member's stated hostname moves from R1.1 to R1 with pools (№Q139)
  - Earlier rows: 2026-10-16 owner decision
  - Home: [where a member says it is](runner-implementation.md#where-a-member-says-it-is)

- ✅ RD38 The runner is its person's agent `#runner/<person>`, never its person; its person owns every agent it serves, created with the token `register` answers; no new flag ([№M.10](TODO.md#runner-waves))
  - Earlier rows: 2026-10-17 owner decisions; supersedes the runner's User below
  - Home: [who it runs as](runner-implementation.md#who-it-runs-as), [getting a token](../../docs/02-access.md#getting-a-token)

- ✅ RD39 The runner's system-call filter is a deny list compiled by `github.com/elastic/go-seccomp-bpf` (№Q166)
  - Earlier rows: 2026-10-17 owner decision
  - Home: [sandboxing](runner-implementation.md#sandboxing), [modules](../../src/MODULES.md#external-tools)

- ✅ RD40 A work directory's soft limit is `limits.disk`, default 1G, the agent stopped and reported by name when over (№Q167)
  - Earlier rows: 2026-10-17 owner decision
  - Home: [sandboxing](runner-implementation.md#sandboxing), [file formats](runner-config.md#file-formats)

- ⚠️ RD41 A person's runner is installed by `agent-bus-setup --runner install|uninstall|enable|disable --for <person> [--purge]`, as `<person>-ab-runner` under one template unit, and setup installs no runner by default ([№M.11](TODO.md#runner-waves))
  - Earlier rows: 2026-10-17 owner instruction; the option names are an implementation choice for owner review
  - Home: [a person's runner](../../docs/07-setup.md#a-persons-runner)

- ✅ RD42 A pool's member runners each read the pool name's own inbox as one of several readers; nothing forwards to a runner (№Q159)
  - Earlier rows: 2026-10-17 owner decision; follows from [№M.10](TODO.md#runner-waves)
  - Home: [one name on many hosts](runner-implementation.md#one-name-on-many-hosts)

- ✅ RD86 A kept process's frames carry each message's bus ID, which its answer echoes; topic and tag stay the caller's labels (№Q160)
  - Earlier rows: 2026-10-17 owner decision, "agree" to the recommended option; amends [№Q13](#recorded-decisions)
  - Home: [long-lived agents](runner-implementation.md#long-lived-agents)
  - Question: With `in_flight` above 1, answers cannot be matched by topic and tag, which are the caller's labels and need not be unique. Does each frame carry the message ID, echoed on its answer, amending №Q13?
  - (x) Echo the existing bus message ID in each frame and answer; keep topic/tag as caller labels 🌟
  - 🚫 Echo a runner-assigned per-process frame number — a second identity beside the ID, with lifetime rules of its own
  - 🚫 Keep `in_flight` at 1; use more processes for throughput — gives up the settled `in_flight` (№Q13)
  > 💬 👤parf propose ideas
  >
  > 💬👾 Three ways, the first recommended. **a.** Each frame the runner writes carries the message's own bus ID, already unique and already travelling with it, and the process echoes it on its answer; topic and tag stay the caller's labels (amends №Q13). **b.** The runner numbers the frames it gives one process, 1, 2, 3, and the answer echoes the number: shorter, but a second identity beside the ID. **c.** No `in_flight` above 1: one message per kept process at a time, answers in order, more processes for throughput. Say a, b or c.
  >
  > ✅ 💡👾 Recommend echoing the existing bus message ID 🌟: it already identifies the message and works when answers finish out of order. A per-process number is the runner-up ⭐, but creates another identity and needs lifetime rules. Keep topic and tag unchanged; the exact frame syntax remains runner-format work after the choice.
  >
  > 💬 👤parf agree

- ✅ RD83 A changed env file or bus secret restarts the agent's kept processes, each draining as a stop does and bounded by its deadline; override files and bus `config` keep `SIGHUP` (№Q161)
  - Earlier rows: 2026-10-17 owner decision, "yes - restart"; amends [№Q19](#recorded-decisions)
  - Home: [stop and reload](runner.md#stop-and-reload)
  - Question: An env file change cannot reach a running process by `SIGHUP`: env is set only at exec. Does a changed env (or secret) restart the agent's kept processes after their work drains, while override files and bus `config` keep `SIGHUP`, amending №Q19?
  > 💬 👤parf yes - restart

- ✅ RD84 A runner is `#runner/<instance>@<host>`, the instance its person's name by default or an edge's name: one slot (№Q165)
  - Earlier rows: 2026-10-17 owner decision, "runner/$user_or_edge@host — user is just a suggestion"
  - Home: [who it runs as](runner-implementation.md#who-it-runs-as), [where it runs](runner-implementation.md#where-it-runs)
  - Question: A person's runner is `#runner/<person>@<realm>` (№M.10), and an edge runner `#runner/<edge>@<realm>` (№Q15): one slot, two meanings. Which names which, or do they compose (for example `#runner/<person>-<edge>`)?
  - 🚫 Compose the two, as `#runner/<person>-<edge>` — the owner kept one slot, named by whoever installs the runner
  > 💬 👤parf runner/$user_or_edge@host   - user is just a suggestion

- ✅ RD85 A multiple-tag query is comma-delimited and selects the records carrying every tag listed (№Q145)
  - Earlier rows: 2026-10-17 owner decision, "comma delimited; treated as and", closed the same day
  - Details: a single tag is the request that exists today, so nothing built changes meaning; the query itself is not built
  - Home: [record tags](../../docs/03-records.md#tags)

- ✅ RD87 Roles keep nested group membership, resolved iteratively with each group processed once (№Q147)
  - Earlier rows: 2026-10-17 owner decision, "groups should be resolved with algo (avoid recursion, mark processed groups)", then "agree"
  - Details: nested membership and processed-group marking are built in every walk; only role assignment walks iteratively, the rest is pending ([№RGW1](TODO.md#group-walk-acceptance)). [№RD9](#recorded-decisions)'s flat terms concern group expressions, not membership
  - Home: [groups](../../docs/01-identity-and-authority.md#groups), current decision [nested group resolution](../../docs/decisions.md#settled)

- ✅ RD43 No serve-only grant: the runner serves what it created and what an Owner hands over (№Q164)
  - Earlier rows: 2026-10-17 owner decision; [№M.10](TODO.md#runner-waves) made it unneeded
  - Home: [who it runs as](runner-implementation.md#who-it-runs-as)

- ✅ RD44 The formats of `agent.json`, `agents.json` and the `status` answer
  - Earlier rows: 2026-10-16 owner approval, for [№M.1](TODO.md#runner-waves)
  - Home: [file formats](runner-config.md#file-formats)

- ✅ RD88 The runner's env files are in the bus's one env-file syntax, the official dotenv format, read by the parser the daemon checks a secret with (№Q171–№Q173)
  - Earlier rows: 2026-10-17 owner decisions, "basic shell syntax as all env files do", "universal", no `$` expansion and no command substitution, then "lets implement official dotenv spec"; no legacy form until release
  - Home: [env-file syntax](../../docs/03-records-service.md#env-file-syntax), [file formats](runner-config.md#file-formats)

- ✅ RD46 What the runner does
  - Earlier rows: [№D211](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D212](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#what-the-runner-does)

- ✅ RD47 Who it runs as
  - Earlier rows: [№D213](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#who-it-runs-as)

- ✅ RD48 Runner unit
  - Earlier rows: [№D199](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D200](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D201](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#runner-unit)

- ✅ RD49 Where it runs
  - Earlier rows: [№D209](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#where-it-runs)

- ✅ RD50 Reaching the runner
  - Earlier rows: [№D197](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D198](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D208](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#reaching-the-runner)

- ✅ RD51 What an instance is
  - Earlier rows: [№D129](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D202](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D207](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D210](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#what-an-instance-is)

- ✅ RD52 The three env layers
  - Earlier rows: [№D203](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D204](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#the-three-env-layers)

- ✅ RD53 What the child is told
  - Earlier rows: [№D205](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D206](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#what-the-child-is-told)

- ✅ RD54 Long-lived agents
  - Earlier rows: [№D120](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D122](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#long-lived-agents)

- ✅ RD55 A kept child may hold several messages in flight, opt-in with `--in-flight K`; answers match on topic and tag (№Q13)
  - Earlier rows: 2026-09-30 owner decision; for owner review: one at a time stays the default
  - Home: [long-lived agents](runner-implementation.md#long-lived-agents)

- ✅ RD56 One name on many hosts
  - Earlier rows: [№D123](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D124](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D125](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D128](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#one-name-on-many-hosts)

- ✅ RD57 On demand
  - Home: [definition](runner-implementation.md#on-demand)

- ✅ RD58 Message routing
  - Home: [definition](runner-implementation.md#many-names-into-one-inbox)

- ✅ RD59 Additional script forms
  - Earlier rows: [№D61](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D118](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D119](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D121](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#additional-script-forms)

- ✅ RD60 The list of what is installed
  - Earlier rows: [№D138](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D139](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D140](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#the-list-of-what-is-installed)

- ✅ RD61 What it comes after
  - Earlier rows: [№D141](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D142](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D143](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#what-it-comes-after)

- ✅ RD62 Backing it up
  - Earlier rows: [№D136](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D137](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite), [№D144](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](runner-implementation.md#backing-it-up)

## Assigned roles implementation planning

- ✅ RD63 Prepare a checkbox plan and peer review, with efficient handling when no roles are assigned
  - Date: 2026-10-16
  - Home: [implementation checklist](roles-implementation.md#assigned-roles-implementation)

- ✅ RD64 Include role descriptions and usage in agent-edit mouseover help
  - Date: 2026-10-16
  - Home: [face checklist](roles-implementation.md#faces-and-persistence)

## Backup encryption choice

- ✅ RD65 User-key backup encryption
  - Why: Reuse the user's existing key and an established tool; owner instruction
  - Date: 2026-09-13
  - Home: [runner § backing it up](runner-implementation.md#backing-it-up)

## Distribution choice

- ✅ RD66 Published release distributions
  - Why: Owner requests installation and container startup without a source build
  - Date: 2026-09-13
  - Home: [Release artifacts](distribution.md#release-artifacts), [container runtime](distribution.md#container-runtime)

## Open

Unresolved choices live in [questions](QUESTIONS.md#open-questions).

## Superseded

- 🚫 RD45 The env files are dotenv, read by the runner's own parser, `\n` and `\t` decoded in double quotes and spaces allowed in an unquoted value (2026-10-16)
  - Replaced by: [the bus's env-file syntax](../../docs/03-records-service.md#env-file-syntax) (RD88, №Q171), 2026-10-17

- 🚫 RD67 Wider role alphabet proposed earlier on 2026-10-16
  - Replaced by: [Existing shell-safe naming alphabet](roles.md#roles-are-for-agents), owner clarification on the same day

- 🚫 RD68 The implementation's role-specific length cap
  - Replaced by: [Revised naming policy](roles.md#roles-are-for-agents), 2026-10-16

- 🚫 RD69 A Resource is an Agent in disguise, and the bus switches the read (№Q112, 2026-09-23)
  - Replaced by: [Resource records](done/resources.md#resource-records): a card; its source, an Agent or an MCP Service, answers, 2026-09-30

- 🚫 RD70 A kept process is sent `STOP` and `RELOAD` as messages it must understand, and a range needs `STOP` (№Q150, 2026-10-16)
  - Replaced by: [Stop and reload](runner.md#stop-and-reload): `SIGTERM` and `SIGHUP` for every agent, the same day (№Q156)

- 🚫 RD71 Locks in a Group, its members their ACL (2026-09-30, built 0.8.66)
  - Replaced by: [Locks on a record](../../docs/01-identity-and-authority.md#shared-locks), the same day

- 🚫 RD72 Global lock names in one default set, a set granted by its own ACL, and only the holder releasing
  - Replaced by: [Locks in a Group](done/locks.md#shared-locks), 2026-09-30

- 🚫 RD73 On the bus the runner is a User, `agent-bus-runner`, owning `#runner@<host>` and every agent it registers, the deployer its Maintainer, an existing name transferred to it (2026-10-16)
  - Replaced by: [The runner as its person's agent](runner-implementation.md#who-it-runs-as) ([№M.10](TODO.md#runner-waves)), 2026-10-17

- 🚫 RD74 The runner reads one inbox: each agent forwards to it with `deliver_to`, read by `original_to` (№Q162), serving only names its User owns (№Q163) (2026-10-16)
  - Replaced by: [Each agent on its own inbox](runner-implementation.md#many-names-into-one-inbox) ([№M.10](TODO.md#runner-waves)), 2026-10-17; the daemon's `original_to` filter stays built

- 🚫 RD75 Telegram, systemd and logwatch are built in R1, with the runner that keeps them; the rest of the catalogue stays R1.2
  - Replaced by: [What moved, and what R1 adds](agents.md#what-moved-and-what-r1-adds)
  - Earlier rows: 2026-10-17 owner instruction; the design stays the [catalogue](../R1.2/services.md#the-catalogue)'s

- 🚫 RD76 The telegram agent starts from `/rd/bin/telegram-post`, and its instance declares a default channel
  - Replaced by: [Telegram](agents.md#telegram)
  - Earlier rows: 2026-10-17 owner instruction; the script's implicit first-chat default does not survive

- 🚫 RD77 The telegram token is the record's bus secret, and nothing reads `~/.codex/telegram.toml`
  - Replaced by: [Telegram](agents.md#telegram)
  - Earlier rows: 2026-10-17 owner decision; the TOML is where a person copies the working token from once, and no part of the agent depends on it (№Q169)

- 🚫 RD78 The systemd agent is one name with three role tiers, read-only without a role
  - Replaced by: [Systemd](agents.md#systemd)
  - Earlier rows: 2026-10-17 owner instruction; it revises the catalogue's per-name rule, and the tier check sits inside the privileged process (№Q168)

- 🚫 RD79 A privileged agent is standalone: its own root unit and install script, served by `agent-bus run`, never by the runner
  - Replaced by: [Systemd](agents.md#systemd)
  - Earlier rows: 2026-10-17 owner decision; the runner's `NoNewPrivileges` and sandbox make it unable to keep one, so this is a second keeping model rather than a systemd exception (№Q170)

- 🚫 RD80 `agent.json` may ask for less, never more
  - Replaced by: [Systemd](agents.md#systemd)
  - Earlier rows: 2026-10-17 owner decision; the file ships inside the agent's own checkout, so a widening key would be a package granting itself privilege

- 🚫 RD81 Logwatch takes syslog beside files and journald, and forwards keyword matches to an agent
  - Replaced by: [Logwatch](agents.md#logwatch)
  - Earlier rows: 2026-10-17 owner instruction; the ring and the verbatim default stay the [catalogue](../R1.2/services.md#reading-the-box)'s

## History

Original wording and superseded choices are preserved in [decision history](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../R0.8-MVP/done/document-migration.md#decision-mapping).
