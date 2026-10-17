# R1 decisions

## Recorded decisions

Migrated 2026-09-13 and split from the old R1 on 2026-09-30. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| R1 Runners web face wraps runner APIs and links to existing agent pages | [Runners web face](runner/web-face.md#goal) | 2026-10-17 owner request; planned, not built |
| Replace Personal classification with record tags (proposed) | [record tags](tags.md#classification-and-filtering) | 2026-10-01 owner proposal |
| Retain Personal rules on the tag | [record tags](tags.md#classification-and-filtering) | 2026-10-01 owner clarification; resolves Q144 |
| Record tags use comma-only normalized labels; Personal rules retained | [automatic tags](tags.md#automatic-tags) | 2026-10-17 owner instruction |
| R1's scope is settled, and the old R1 splits into R1 and [R1.1](../R1.1/README.md#scope) (Q18) | [R1 scope](README.md#scope) | 2026-09-30 owner split; tools and exploration renumbered to R1.2 and R1.3 |
| The runner's children are agents, not services; each is read on its own inbox as itself | [many names into one inbox](runner-implementation.md#many-names-into-one-inbox) | 2026-10-16 owner review; reading revised 2026-10-17 by M.10 |
| Assigned roles keep the existing allow-list representation and current nested membership | [current roles](../../docs/03-records-agent.md#roles) | 2026-10-16 implementation choice under the start instruction; owner confirmation remains Q146/Q147 |
| Record-defined roles are R1's first topic (Q98) | [roles](roles.md#record-defined-roles) | 2026-09-22 owner decision; moved out of MVP; D249 |
| R1's roles ship as flat terms; composing groups with `& \| !` moved to R1.3 (Q138) | [roles](roles.md#record-defined-roles), [composed groups](../R1.3/composed-groups.md#group-expressions) | 2026-09-30 owner decision |
| Roles for Agents, generated roles and delivery | [roles are for Agents](roles.md#roles-are-for-agents) | 2026-09-30 owner decisions; naming revised 2026-10-16 |
| Opaque application roles with the existing shell-safe naming alphabet | [roles are for Agents](roles.md#roles-are-for-agents) | 2026-10-16 owner clarification; resolves Q148 |
| Generated role names cannot be assigned in ACLs; reject the save | [roles are for Agents](roles.md#roles-are-for-agents) | 2026-10-16 owner confirmation; resolves Q149 |
| Locks are held in memory and never stored | [shared locks](../../docs/01-identity-and-authority.md#shared-locks) | 2026-09-22 owner decision; built in 0.8.66; D64, D65, D66. Sets of interchangeable locks moved to [R2.0](../R2.0-Future/lock-sets.md#a-set-of-locks) on 2026-09-30 |
| A key-value store per record, set with `kv_set(record, name, value, how)`, `how` one of set, add, replace; its Owner, Maintainers and own Agent read and write it (Q107) | [key-value store](done/kv.md#per-record-storage) | 2026-09-22 owner instruction; tied to a record and its authority 2026-09-30 |
| Three kinds of value, string (the default), int and JSON, each its own table and namespace keyed by the record's internal ID, with get, set and delete each, plus `kv_int_inc` and `kv_json` | [key-value store](done/kv.md#per-record-storage) | 2026-09-30 owner decision; no timestamps. For owner review: a write commits before it is answered |
| A `json` value takes `set`, `unset`, `inc`, `push`, `unshift`, `shift`, `pop`, `add_to_set` and `remove_from_set` on a value's top-level keys, a list applied all or none | [JSON operations](../../docs/03-records-kv.md#json-operations) | 2026-09-30 owner decision; no conditional op, the shared lock covers that; top-level keys only, not JSON Pointer paths |
| MCP Resource and Resource Template become registry records | [Resource records](done/resources.md#resource-records) | 2026-09-23 owner instruction |
| A Resource is information: a card like a Service, one `resource` kind with a template flag, 📄 and 🧩 (Q110), carrying every MCP descriptor field (Q111) | [Resource records](done/resources.md#resource-records) | 2026-09-30 owner decisions, against the 2026-07-28 specification |
| A card's source is an Agent (a bus request) or an MCP Service (forwarded); the face fetches an `https://` card itself; the daemon serves no content (Q114, Q113) | [Resource records](../../docs/03-records-resource.md#what-a-resource-is) | 2026-09-30 owner decisions |
| Federation stays R1 scope, on hold | [federation](federation.md#chaining) | 2026-09-30 owner decision |
| Chaining | [definition](federation.md#chaining) | D80 |
| Modules | [definition](modules.md#modules) | D178 |
| Blocking lazy private-value properties in the Python client | [lazy private values](done/client-libraries.md#lazy-private-values) | Owner accepts blocking reads; other execution choices remain open |
| Java joins the client-library scope | [client libraries](done/client-libraries.md#scope) | Owner choice |
| Implement the reviewed clients against the current daemon contract | [review sequence](done/client-libraries.md#review-sequence) | Owner authorized implementation after the plan review ring |
| An agent's `processes` is a fixed count or a `min..max` range; the runner stops a process with `SIGTERM` and reloads it with `SIGHUP`, for every agent (Q150, Q12, Q156) | [runner § kinds of agents](runner.md#kinds-of-agents) | 2026-10-16 owner decision; on demand is `min` 0, through the runner's forwarding |
| An agent whose configuration says it serves itself is not registered by the runner; the runner starts it and restarts it when it dies (Q151) | [runner § the rule](runner.md#the-rule) | 2026-10-16 owner decision |
| A process above `min` idle for 5 minutes, by default, is stopped (Q154) | [runner § stop and reload](runner.md#stop-and-reload) | 2026-10-16 owner decision; the host may change it |
| Every managed agent is sandboxed by default, in a cgroup of the runner's delegated subtree (`Delegate=yes`), not through a user manager (Q157) | [runner § sandboxing](runner.md#sandboxing) | 2026-10-16 owner decision |
| A host overrides a shipped JSON file with its own file of the same name and JSON, per agent and per instance, its values winning (Q152) | [runner § overriding a shipped file](runner.md#overriding-a-shipped-file) | 2026-10-16 owner decision; merge and delivery proposed under Q155 |
| The runner's env files win over the record's bus secret: local over remote (Q153) | [runner § secrets](runner.md#secrets) | 2026-10-16 owner decision |
| The agent configuration options and their homes, a JSON Merge Patch override that changes only run options, and `agent.json`, `enabled`, `processes` and the host's `name` replacing `config.json`, `autostart`, `-N` and an author-given name (Q155) | [agent configuration](runner-config.md#where-an-option-lives) | 2026-10-16 owner decision |
| Bubblewrap applies what a cgroup does not: filesystem, privilege, a namespace per agent, a filtering proxy for named hosts, a sized `/tmp` and a soft work-directory limit (Q158) | [runner implementation § sandboxing](runner-implementation.md#sandboxing) | 2026-10-16 owner decision; not an account per agent |
| An edge runner with no daemon takes the bus's realm, `#runner/<edge>@<realm>` (Q15) | [where it runs](runner-implementation.md#where-it-runs) | 2026-10-16 owner decision |
| A configuration change waits for `reload`, which signals each changed agent's kept processes (Q19) | [stop and reload](runner.md#stop-and-reload) | 2026-10-16 owner decision |
| A backup is a runner verb, `backup` (Q33) | [backing it up](runner-implementation.md#backing-it-up) | 2026-10-16 owner decision |
| A pool member's stated hostname moves from R1.1 to R1 with pools (Q139) | [where a member says it is](runner-implementation.md#where-a-member-says-it-is) | 2026-10-16 owner decision |
| The runner is its person's agent `#runner/<person>`, never its person; its person owns every agent it serves, created with the token `register` answers; no new flag (M.10) | [who it runs as](runner-implementation.md#who-it-runs-as), [getting a token](../../docs/02-access.md#getting-a-token) | 2026-10-17 owner decisions; supersedes the runner's User below |
| The runner's system-call filter is a deny list compiled by `github.com/elastic/go-seccomp-bpf` (Q166) | [sandboxing](runner-implementation.md#sandboxing), [modules](../../src/MODULES.md#external-tools) | 2026-10-17 owner decision |
| A work directory's soft limit is `limits.disk`, default 1G, the agent stopped and reported by name when over (Q167) | [sandboxing](runner-implementation.md#sandboxing), [file formats](runner-config.md#file-formats) | 2026-10-17 owner decision |
| A person's runner is installed by `agent-bus-setup --runner install\|uninstall\|enable\|disable --for <person> [--purge]`, as `<person>-ab-runner` under one template unit, and setup installs no runner by default (M.11) | [a person's runner](../../docs/07-setup.md#a-persons-runner) | 2026-10-17 owner instruction; the option names are an implementation choice for owner review |
| A pool's member runners each read the pool name's own inbox as one of several readers; nothing forwards to a runner (Q159) | [one name on many hosts](runner-implementation.md#one-name-on-many-hosts) | 2026-10-17 owner decision; follows from M.10 |
| No serve-only grant: the runner serves what it created and what an Owner hands over (Q164) | [who it runs as](runner-implementation.md#who-it-runs-as) | 2026-10-17 owner decision; M.10 made it unneeded |
| The formats of `agent.json`, `agents.json` and the `status` answer | [file formats](runner-config.md#file-formats) | 2026-10-16 owner approval, for M.1 |
| The env files are dotenv, read by the runner's own parser (godotenv expands `$`), with comments, no expansion, and an empty `env.dist` value required | [file formats](runner-config.md#file-formats) | 2026-10-16 owner approval, for M.2 |
| What the runner does | [definition](runner-implementation.md#what-the-runner-does) | D211, D212 |
| Who it runs as | [definition](runner-implementation.md#who-it-runs-as) | D213 |
| Runner unit | [definition](runner-implementation.md#runner-unit) | D199, D200, D201 |
| Where it runs | [definition](runner-implementation.md#where-it-runs) | D209 |
| Reaching the runner | [definition](runner-implementation.md#reaching-the-runner) | D197, D198, D208 |
| What an instance is | [definition](runner-implementation.md#what-an-instance-is) | D129, D202, D207, D210 |
| The three env layers | [definition](runner-implementation.md#the-three-env-layers) | D203, D204 |
| What the child is told | [definition](runner-implementation.md#what-the-child-is-told) | D205, D206 |
| Long-lived agents | [definition](runner-implementation.md#long-lived-agents) | D120, D122 |
| A kept child may hold several messages in flight, opt-in with `--in-flight K`; answers match on topic and tag (Q13) | [long-lived agents](runner-implementation.md#long-lived-agents) | 2026-09-30 owner decision; for owner review: one at a time stays the default |
| One name on many hosts | [definition](runner-implementation.md#one-name-on-many-hosts) | D123, D124, D125, D128 |
| On demand | [definition](runner-implementation.md#on-demand) | |
| Message routing | [definition](runner-implementation.md#many-names-into-one-inbox) | |
| Additional script forms | [definition](runner-implementation.md#additional-script-forms) | D61, D118, D119, D121 |
| The list of what is installed | [definition](runner-implementation.md#the-list-of-what-is-installed) | D138, D139, D140 |
| What it comes after | [definition](runner-implementation.md#what-it-comes-after) | D141, D142, D143 |
| Backing it up | [definition](runner-implementation.md#backing-it-up) | D136, D137, D144 |

## Assigned roles implementation planning

| Date | Direction | Substance |
|---|---|---|
| 2026-10-16 | Prepare a checkbox plan and peer review, with efficient handling when no roles are assigned | [implementation checklist](roles-implementation.md#assigned-roles-implementation) |
| 2026-10-16 | Include role descriptions and usage in agent-edit mouseover help | [face checklist](roles-implementation.md#faces-and-persistence) |

## Backup encryption choice

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | User-key backup encryption | Reuse the user's existing key and an established tool; owner instruction | [runner § backing it up](runner-implementation.md#backing-it-up) |

## Distribution choice

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Published release distributions | Owner requests installation and container startup without a source build | [Release artifacts](distribution.md#release-artifacts), [container runtime](distribution.md#container-runtime) |

## Open

Unresolved choices live in [questions](QUESTIONS.md#open-questions).

## Superseded

| Earlier design | Replacement |
|---|---|
| Wider role alphabet proposed earlier on 2026-10-16 | [Existing shell-safe naming alphabet](roles.md#roles-are-for-agents), owner clarification on the same day |
| The implementation's role-specific length cap | [Revised naming policy](roles.md#roles-are-for-agents), 2026-10-16 |
| A Resource is an Agent in disguise, and the bus switches the read (Q112, 2026-09-23) | [Resource records](done/resources.md#resource-records): a card; its source, an Agent or an MCP Service, answers, 2026-09-30 |
| A kept process is sent `STOP` and `RELOAD` as messages it must understand, and a range needs `STOP` (Q150, 2026-10-16) | [Stop and reload](runner.md#stop-and-reload): `SIGTERM` and `SIGHUP` for every agent, the same day (Q156) |
| Locks in a Group, its members their ACL (2026-09-30, built 0.8.66) | [Locks on a record](../../docs/01-identity-and-authority.md#shared-locks), the same day |
| Global lock names in one default set, a set granted by its own ACL, and only the holder releasing | [Locks in a Group](done/locks.md#shared-locks), 2026-09-30 |
| On the bus the runner is a User, `agent-bus-runner`, owning `#runner@<host>` and every agent it registers, the deployer its Maintainer, an existing name transferred to it (2026-10-16) | [The runner as its person's agent](runner-implementation.md#who-it-runs-as) (M.10), 2026-10-17 |
| The runner reads one inbox: each agent forwards to it with `deliver_to`, read by `original_to` (Q162), serving only names its User owns (Q163) (2026-10-16) | [Each agent on its own inbox](runner-implementation.md#many-names-into-one-inbox) (M.10), 2026-10-17; the daemon's `original_to` filter stays built |
| Telegram, systemd and logwatch are built in R1, with the runner that keeps them; the rest of the catalogue stays R1.2 | [What moved, and what R1 adds](agents.md#what-moved-and-what-r1-adds) | 2026-10-17 owner instruction; the design stays the [catalogue](../R1.2/services.md#the-catalogue)'s |
| The telegram agent starts from `/rd/bin/telegram-post`, and its instance declares a default channel | [Telegram](agents.md#telegram) | 2026-10-17 owner instruction; the script's implicit first-chat default does not survive |
| The telegram token is the record's bus secret, and nothing reads `~/.codex/telegram.toml` | [Telegram](agents.md#telegram) | 2026-10-17 owner decision; the TOML is where a person copies the working token from once, and no part of the agent depends on it (Q169) |
| The systemd agent is one name with three role tiers, read-only without a role | [Systemd](agents.md#systemd) | 2026-10-17 owner instruction; it revises the catalogue's per-name rule, and the tier check sits inside the privileged process (Q168) |
| A privileged agent is standalone: its own root unit and install script, served by `agent-bus run`, never by the runner | [Systemd](agents.md#systemd) | 2026-10-17 owner decision; the runner's `NoNewPrivileges` and sandbox make it unable to keep one, so this is a second keeping model rather than a systemd exception (Q170) |
| `agent.json` may ask for less, never more | [Systemd](agents.md#systemd) | 2026-10-17 owner decision; the file ships inside the agent's own checkout, so a widening key would be a package granting itself privilege |
| Logwatch takes syslog beside files and journald, and forwards keyword matches to an agent | [Logwatch](agents.md#logwatch) | 2026-10-17 owner instruction; the ring and the verbatim default stay the [catalogue](../R1.2/services.md#reading-the-box)'s |

## History

Original wording and superseded choices are preserved in [decision history](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../R0.8-MVP/done/document-migration.md#decision-mapping).
