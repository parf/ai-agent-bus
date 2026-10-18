# R1 decisions

## Recorded decisions

`RD` labels the original unnumbered decision-index rows; it allocates no Q IDs.

Migrated 2026-09-13 and split from the old R1 on 2026-09-30. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

- ✅ RD98 The Telegram agent is the Slack agent's twin: its methods, keywords, forms and subscriptions wherever Telegram allows
  - Why: owner instruction, built by OpenCode with Claude B answering its questions
  - Date: 2026-10-18
  - Home: [Telegram agent](telegram/README.md#telegram-agent)

- ✅ RD97 IMAP and SMTP clients are included in R1
  - Why: owner instruction to add both clients to the agents plan
  - Date: 2026-10-17
  - Home: [mail clients](agents.md#mail-clients)

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

- ✅ RD7 Assigned roles keep the existing allow-list representation and current nested membership
  - Earlier rows: 2026-10-16 implementation choice under the start instruction; representation and duplicate-grant behavior confirmed 2026-10-17 as [№RD92](#recorded-decisions); nested membership as [№RD87](#recorded-decisions); applicability as [№RD95](#recorded-decisions)
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

- ✅ RD21 Chaining
  - Earlier rows: [№D80](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite)
  - Home: [definition](../R1.2/federation.md#chaining)

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

- ✅ RD96 A bundled agent is opt-in: listed in `status` as available, served only once the host's `agents.json` enables it; an agent in the person's own `agent.d` stays enabled by default (№Q175)
  - Earlier rows: 2026-10-17 owner decision, "1 - yes" in the session
  - Question: Is a bundled agent served as soon as it arrives (№M.12), or only once the host enables it in `agents.json`?
  - Home: [№M.12](TODO.md#runner-waves)
  - (x) Opt-in: a bundled agent is listed in `status` as available and served only after `agents.json` enables it; an agent in the person's own `agent.d` stays enabled by default 🌟
  - 🚫 Served at once, like a directory in `agent.d`; the host disables what it does not want — each upgrade could start an agent nobody asked for
  - 🚫 Opt-in per agent: its `agent.json` says whether it starts on arrival — the choice belongs to the host, not the agent's author
  > 💡👾 Recommend opt-in 🌟. Every person's runner would otherwise start every bundled agent the moment an upgrade brings it: the Slack agent with no secret restarting in a loop, the debug agent answering `env` to whoever its ACL admits. Copying an agent into your own `agent.d` stays the one-step way to run it. Served at once ⭐ is simpler, but each upgrade could then start something nobody asked for.
  >
  > 💬 👤parf (in the session) 1 - yes

- ✅ RD94 When `agent-bus run` puts the agent's bus secret in its script's environment, the secret wins over the shell's value of the same name (№Q174)
  - Earlier rows: 2026-10-17 owner decision, "agree" to the recommended option
  - Question: When `agent-bus run` puts the agent's bus secret in its script's environment (№DR.1), and the shell that started it already sets the same name, which value does the script get?
  - Home: [№DR.1](TODO.md#runner-waves)
  - (x) The secret wins: it is the agent's, set on purpose with `agent-bus secret`, and the shell's environment is whatever the terminal happened to hold 🌟
  - 🚫 The shell wins, local over remote, as №Q153 has the runner's host env files beat the bus secret — a shell's environment is not a decision made for the agent
  - 🚫 Refuse to start while both set one name — the secret is the deliberate value, so there is nothing to refuse
  > ✅ 💡👾 Recommend the secret wins 🌟. №Q153's local file is a host decision written for that agent; a shell's environment is not — an `IMAP_USER` left over in a terminal would silently send the agent to the wrong account. Keeping the shell's value is the runner-up ⭐ if starting by hand should be able to override the record for a test.
  >
  > 💬 👤parf agree

- ✅ RD95 Assigned roles sit on records with an ACL, excluding User records, Group membership and Maintainers (№Q146)
  - Why: owner selected the recommended option and agreed
  - Date: 2026-10-17
  - Home: [current roles](../../docs/03-records-agent.md#roles)
  - Question: Which record kinds may carry assigned roles?
  - Details: completes [№RD92](#recorded-decisions). The scope chosen is the one built, so nothing changes: `manage.go:176` refuses a decorated Group member or User grant, and `assigned_roles_test.go:73` a decorated Maintainer
  - (x) Permit assignments on ACL-bearing records, excluding User records, Group membership and Maintainers 🌟
  - 🚫 Restrict assignments to Agent records; queue/pubsub ACLs would no longer assign application roles ⭐ — owner selected the recommended alternative
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
  >
  > 💬👾 The first two checked aspects are recorded as [№RD92](DECISIONS.md#recorded-decisions). The third remains unchecked, so this item stays open only for which records may carry assignments. No implementation behavior changes while that choice remains open.
  >
  > ✅ 💡👾 Recommended: keep assignments on ACL-bearing records 🌟. This matches the current contract and lets a queue or pubsub channel grant the roles carried by a forwarded call; User records, membership and Maintainers remain excluded. The runner-up is Agent-only assignment ⭐: a narrower surface, but it removes queue/pubsub application-role grants and requires a contract and implementation change. Which scope should we keep?
  >
  > 💬 👤parf agree
  >
  > 💬👾 Recorded as №RD95; the scope was already the [current contract](../../docs/03-records-agent.md#roles), so no code changes.

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

- ✅ RD87 Roles keep nested group membership, resolved iteratively with each group processed once (№Q147)
  - Earlier rows: 2026-10-17 owner decision, "groups should be resolved with algo (avoid recursion, mark processed groups)", then "agree"
  - Details: nested membership and processed-group marking are built in every walk; only role assignment walks iteratively, the rest is pending ([№RGW1](TODO.md#group-walk-acceptance)). [№RD9](#recorded-decisions)'s flat terms concern group expressions, not membership
  - Home: [groups](../../docs/01-identity-and-authority.md#groups), current decision [nested group resolution](../../docs/decisions.md#settled)

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
  > ✅ 💡👾 Recommend confirming the existing nested-membership behavior 🌟 while retaining the explicit non-recursive traversal and processed-group tracking already requested. That handles cycles without changing who receives roles. The runner-up ⭐ is to leave membership policy open while keeping the algorithm requirement; it needs a concrete access example showing what should differ.
  >
  > 💬 👤parf agree
  >
  > 💬👾 Closed as [№RD87](DECISIONS.md#recorded-decisions), with a
  > correction to my turn above: "already met in code" holds for role
  > assignment only. All four group walks mark processed groups, but the ACL
  > check (`memberThrough`, `manage.go:287`), the directory index
  > (`reachThrough`, `users.go:829`) and publication expansion (`deliverTo`,
  > `deliverto.go:114`) still recurse. Who receives what is unchanged; making
  > those three iterative is now [№RGW1](TODO.md#group-walk-acceptance).

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

- ✅ RD89 Chaining uses the realm as its namespace; template identity keeps `/` (№Q20)
  - Why: owner selected option 1 and confirmed it
  - Date: 2026-10-17
  - Home: [chaining](../R1.2/federation.md#chaining)
  - Question: A chaining namespace and an agent template both want the `/`
  - (x) The realm already is the namespace — `#ci@team`, `#mail@company.upstream`; no grammar change, and `team/ci` is deleted from the plan 🌟
  - 🚫 Keep `team/ci` and give chaining its own separator from `.`, `_`, `-` ⭐ — owner selected the recommended alternative
  - 🚫 Drop namespaced ids entirely; resolution order decides which node answers — owner selected the recommended alternative
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
  >
  > 💬👾 Recorded as №RD89; the accepted substance is in [chaining](../R1.2/federation.md#chaining).

- ✅ RD90 Shared executable conformance cases specify the protocol supported by client libraries (№Q17)
  - Why: owner agreed to the recommended conformance-first specification; no new wire schema
  - Date: 2026-10-17
  - Home: [protocol conformance](modules.md#protocol-conformance)
  - Question: How `protocol` is specified across client languages
  - (x) A conformance suite as the specification — one shared fixture of request and answer pairs every client must reproduce 🌟
  - 🚫 A prose contract in `docs/`, one section per call ⭐ — owner selected the recommended alternative
  - 🚫 A schema and a generator (JSON Schema or an IDL) — owner selected the recommended alternative
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
  >
  > 💬👾 Recorded as №RD90; the accepted substance is in [protocol conformance](modules.md#protocol-conformance).

- ✅ RD91 Federation moves from R1 to R1.2 (№Q137)
  - Why: owner moved the topic; R1 no longer carries chaining or its later-stage prerequisites
  - Date: 2026-10-17
  - Home: [R1.2 federation](../R1.2/federation.md#chaining)
  - Question: Whether R1's chaining is a lookup-only fallthrough without AUTH, signed generations and peer sync, or waits for R1.1
  > 💬 👤parf move federation for R1.2
  >
  > 💬👾 Understood: federation leaves R1 for R1.2, which answers the question
  > by removing it — R1 has no chaining, so it needs nothing from R1.1. What
  > moves with it: the topic file, its Dependencies item, the acceptance line
  > that federation must be exercised across distinct nodes, and №Q20, since
  > the `/` conflict only bites once chaining exists. R1's scope row and the
  > Next step paragraph lose their federation clause. The R1 plan keeps one
  > line saying where it went, so the history stays followable.
  >
  > 💬👾 Recorded as №RD91; the accepted substance is in [R1.2 federation](../R1.2/federation.md#chaining).

- ✅ RD93 The systemd start/restart/reload application-role tier is named `operate` (№RQ1)
  - Why: owner accepted the proposed name
  - Date: 2026-10-17
  - Home: [systemd](agents.md#systemd)
  - Question: What should the systemd start/restart/reload role tier be named?
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
  >
  > 💬👾 Recorded as №RD93; the accepted substance is in [systemd](agents.md#systemd).

- ✅ RD92 The assigned-role representation and duplicate-grant behavior are owner-confirmed (№Q146)
  - Details: the first two checkboxes were selected; applicable record kinds remained unselected and are not treated as approval
  - Question: Confirm the implemented assigned-role representation, duplicate actor grants and applicable record kinds
  - [x] Keep role assignments in existing `allow` terms such as `bob(deploy)` and `@dev(deploy, read_only)`
  - [x] Keep separate same-actor grants and union their matching roles; remove a bare actor across its grants, or a decorated grant by its role set
  - Date: 2026-10-17
  - Home: [current roles](../../docs/03-records-agent.md#roles)
  - Completed by: [№RD95](#recorded-decisions), applicability to ACL-bearing records

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

- 🚫 RD20 Federation stays R1 scope, on hold
  - Earlier rows: 2026-09-30 owner decision
  - Replaced by: owner move to R1.2 on 2026-10-17, [№RD91](#recorded-decisions)
  - Home: [federation](../R1.2/federation.md#chaining)


## History

Original wording and superseded choices are preserved in [decision history](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../R0.8-MVP/done/document-migration.md#decision-mapping).
