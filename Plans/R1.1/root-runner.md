# Root runner

📌 **TL;DR:** RT19 builds a runner that runs as root and starts each host-access agent as the Linux account the host chose, and it **replaces** the standalone starter (owner, Q336). This is the approach for the owner to approve: what the starter is today, what moves and what is dropped, how installs fail rather than convert, what setup, `git-install.sh` and `check-deploy` change, how it is tested and when. Nothing here is built and no format is chosen; the open choices are [Q361–Q364](QUESTIONS.md#root-runner).

Owner, 2026-10-19: "root runner; allow to run services under specific user; or under root (optional drop permissions)". Owner, Q336: the root runner replaces the starter; both a root runner and a person's runner need a default policy that hides other agents' secret files; protecting a person's own env files from that person is wanted if possible; a root runner cannot be protected from root. The proposal that started it is [runner.md § Root runner](runner.md#root-runner).

## What the starter is today

| | Today |
|---|---|
| Files | `src/agents/standalone/install.py` (install and remove), [`starter.md`](../../src/agents/standalone/starter.md#standalone-starter), one wrapper per agent (`systemd`, `logwatch`, `git`, `tmux-sessions` `install.py`; `server-info` `install.sh`), and the agents' code under `standalone/`. The release ships it as `agents/standalone/` |
| Who calls it | An administrator, by hand, with `sudo` and their own credential, following each agent's page. Not setup, not `git-install.sh`, not `check-deploy`; only its tests call it otherwise |
| What an install writes | One systemd unit per agent, `agent-bus-<name>.service`, and one private env file under `/etc/agent-bus/standalone/` holding the agent's own token. It registers the name with the caller's credential once, asks `agent-bus token` for the agent's own, and keeps nothing of the caller's |
| What the unit runs | `agent-bus run --sandbox off --stay-registered` around the script (one process per message), or the executable itself for an agent that serves its own inbox (`logwatch`). No sandbox, no cgroup limits, no private network |
| What it runs as | A dedicated system account `ab-<agent>` (nologin, no home) unless `--user` names another: root for `systemd` and optionally `logwatch` and `server-info`, the project owner for `git`, the person for `tmux-sessions` (whose page now sends it to a person's user-account runner) |
| Lifecycle | Install is serialized per name and idempotent; removal needs the remover's current credential and keeps the account, so a UID is never reused. A deploy does not restart these units: `git-install.sh` restarts the daemon and the runners only |

Five agents use it today: `systemd`, `logwatch`, `git`, `server-info`, `tmux-sessions`. Each of them reaches the host through its Linux account; none needs the daemon's powers.

## What moves and what is dropped

| | Decision proposed |
|---|---|
| Moves into the root runner | Starting each of those agents; giving each its own bus record and token (the runner already creates the record and keeps the token for every agent it serves); per-message and kept processes (`serves: itself` already covers `logwatch`); the runner's message deadlines, status, `reload`, stop and logs, which the starter never had |
| New in the runner | The account each agent runs as, and for root agents the capabilities kept; both set **only in a root-owned host file the runner reads**, never in a bus record or call, so no bus caller, Owner included, can raise an agent's privilege (settled 2026-10-19, [process boundary](../../docs/08-processes.md#the-rule)) |
| Default policy (Q336) | Every agent runs in its own account with the runner's sandbox on, so another agent's env files, tokens and work directory are not readable or visible to it. An agent runs unsandboxed or keeps a capability only when the host file says so, by name. Details and limits: [Q362](QUESTIONS.md#root-runner) |
| Agents become ordinary pack agents | Their manifests gain the runner's `command`, `serves` and `needs`, and they leave `standalone/` for `src/agents/<name>/` like the rest of the pack. Their agent pages, `package.sh`, setup's package lists and the smoke map follow |
| Dropped | `install.py`, `starter.md`, the per-agent installers, `/etc/agent-bus/standalone/`, the per-agent units, the unsandboxed `agent-bus run` wrapper and the `--user` and `--script-arg` installer options. Removed together with their docs and tests, per [one canonical interface](../../CLAUDE.md#working-rules), with no alias or converter |
| Unchanged | A person's runner and its account `<person>-ab-runner`; the user-account runner; the dedicated `agent-bus run`; agent code and behaviour |

## Existing installs

No prerelease installation or data conversion is kept ([one canonical interface](../../CLAUDE.md#working-rules)). A node with starter units fails by name, once, with the steps to retire them; it is never half-migrated:

- **Setup and the root runner refuse** while a starter unit or `/etc/agent-bus/standalone/` entry exists, naming each one (`standalone starter unit present: agent-bus-<name>.service`) and the three commands that retire it (disable and stop the unit, unregister the name with the owner's credential, remove the unit and env file). The retired starter is not kept to do it.
- **A bus record that exists without the runner's token is refused by name**, as for any agent today, until its token is handed over once ([what runs as what](../../docs/runner/security.md#what-runs-as-what)). That already blocks a same-name install over a live starter record.
- **Accounts the starter made (`ab-*`) are left in place.** Whether the root runner reuses them or makes its own is [Q361](QUESTIONS.md#root-runner).
- **The development node** retires its own starter units in the build step that lands the removal, and `check-deploy` fails if any remains.

## What setup, git-install.sh and check-deploy change

| | Change |
|---|---|
| `agent-bus-setup` | A root form of the runner install beside the person's: one system unit for the root runner, the root-owned host file and directory, and the refusal above. Its verbs mirror the person's runner (install, uninstall, enable, disable), planned and idempotent. Its name on the bus is [Q364](QUESTIONS.md#root-runner) |
| `git-install.sh` | It already restarts every active system runner after a deploy, so the root runner and its unit template refresh with them; the starter units, which it never touched, disappear. Nothing else |
| `check-deploy` | Each root-runner agent is checked as any agent is (its `selftest`), plus two named failures: an agent refused or not running as its host-file account in the runner's `status`, and any starter unit still on the node |
| Release layout | `agents/standalone/` leaves the release and package lists; the pack's own `protocol.json` check already applies to the moved agents |

## Tests and mutants

The runner is a ⚠️ subsystem, install and process privilege: each boundary below gets a mutant that must compile and fail on a named assertion, and two readers.

| Boundary or refusal | Test | Mutant |
|---|---|---|
| The host file is the only source of an agent's account and capabilities | A bus record or call carrying an account or capability changes nothing; a host file that is not root-owned, or is group- or world-writable, or is a link, refuses the whole load | read the account from the record; accept a writable or non-root file; follow a link |
| The account switch | The child runs with exactly the host file's uid, gid and groups, no inherited supplementary groups, no new privileges | skip the gid or the groups; keep root's groups; allow new privileges |
| Capabilities | A root agent holds only the listed ones; none is added by the environment or the manifest | keep all; drop after the exec; honour the manifest's own list |
| Default isolation | Under two accounts, one agent cannot read, list or open another's env files, token store or work directory | share a work directory; mount the runner's settings; widen a mode |
| Explicit refusals | An unknown, system-less or uid-0 account without the explicit root form; a sandbox-off entry that is not named; two agents resolving to one work directory; a starter unit or record without a token | accept each |
| Starter retirement | Setup and the root runner refuse with the named unit; `check-deploy` fails on a leftover | skip the scan; match the wrong path |
| Reload | A host-file change between two reloads is seen by the second, and a refused entry never starts | cache the file; start before the refusal |

Real account switching needs root, so it runs in the podman slow shard, which takes the smoke-gate lock; the unit level uses a switch seam the tests replace. No paid or live calls anywhere.

## Docs touched

[Processes and privileges § The rule](../../docs/08-processes.md#the-rule) and its process table (a root runner, and who sets its privilege); the runner pages ([setup](../../docs/runner/setup.md#a-persons-runner), with the root form beside the person's, [security](../../docs/runner/security.md#what-runs-as-what), [configuration](../../docs/runner/configuration.md)); the five agent pages, whose install sections become runner configuration; `src/agents/standalone/starter.md` and its README, removed with the code; [deployment](../../.claude/docs/deployment.md) for `git-install.sh` and `check-deploy`; this topic moves to `done/` when built.

## Steps and clock time

R1.1 is closed: the build starts when the owner opens it, so the times below are agent wall-clock from that start, with readers included.

| Step | What | Estimate |
|---|---|---|
| 1 | Host file, account switch and capability drop in the runner, with the seam and its mutants | 2 h |
| 2 | Default isolation across accounts and the refusals, including the root-only podman test | 2 h |
| 3 | Setup's root form, the unit and the starter refusal | 1.5 h |
| 4 | The five agents become pack agents, and the starter and its docs and tests are removed together | 2 h |
| 5 | `check-deploy`, `git-install.sh`, the dev node's own retirement and the docs | 1.5 h |

About nine hours in all, in that order; each step is its own branch with two readers. This topic itself, with the questions, is the only thing due now.

## Questions

Owner choices for RT19, in one place with the plan (owner, 2026-11-06: "create dedicated readme/todo plan for this section - use combined document"). №Q336 settled the design: the root runner replaces the starter.

- ❓ Q361 Which Linux accounts the root runner starts agents as, and who creates them
  - Context: the starter makes a dedicated `ab-<agent>` account per agent (nologin, no home) and keeps it after removal; the root runner needs an account per agent too ([what moves](#what-moves-and-what-is-dropped)). Blocks steps 3 and 4
  - ( ) Both: setup creates `ab-<agent>` accounts by default, and the host file may name any existing account instead 🌟
  - ( ) Setup creates `ab-<agent>` accounts as the starter does, and the host file names them ⭐
  - ( ) The host file names any existing account; setup never creates one
  > 💡👾 Both: today's dedicated-account default stays, and `git` can still run as the project owner. Leftover `ab-*` accounts are reused only if they still pass the nologin and no-home test.

- ❓ Q362 What the default policy hides, and which capabilities a root agent can keep
  - Context: the owner asked for a default that protects other agents' secret files; a root agent runs with capabilities dropped except those the host file lists ([default policy](#what-moves-and-what-is-dropped)). Blocks steps 1 and 2
  - ( ) One account per agent plus the runner's sandbox, no capability kept unless listed, and a short never-list of capabilities that cannot be listed 🌟
  - ( ) The same without the never-list; the host owner's word is final ⭐
  > 💡👾 With the never-list: the host owner still decides; the list only stops a typo from letting a root agent escape its own sandbox. The owner names the list.

- ❓ Q363 Whether a person's own env files can be protected from that person
  - Context: a person's runner and a user-account runner run as the person, so the person can read what the runner reads; a root runner cannot be protected from root. Blocks nothing in RT19
  - ( ) Not protected, stated plainly in the security page; bus Secret and Configuration records are the documented way to keep a secret from the person 🌟
  - ( ) Not protected, stated plainly; nothing more ⭐
  - ( ) A root-owned file only a root helper reads, for system runners only
  > 💡👾 State it plainly and point to bus Secrets: a root helper inside a person's runner is what this design avoids.

- ❓ Q364 The root runner's name on the bus
  - Context: a person's runner is `#runner/<person>@<realm>` and an edge's `#runner/<edge>@<realm>` ([№Q165](../R1.0-Release/DECISIONS.md#recorded-decisions)); the root runner needs a name that is neither. Blocks step 3
  - ( ) `#runner/root@<realm>` 🌟
  - ( ) `#runner/<host>@<realm>` ⭐
  - ( ) The node owner's choice at install, refused if it matches a person or an edge
  > 💡👾 `root`: it reads as what it is, and a person or edge named `root` is already refused by the plain-name rule.

- ❓ Q365 Open RT19's build now, or later
  - Context: R1.1 is closed to new scope ("missing parts and fixing bugs"). The [steps](#steps-and-clock-time) take about 9 agent-hours and touch install (⚠️)
  - ( ) Later: build when R1.1 opens or a person runner on this node needs it 🌟
  - ( ) Now, as an exception, after №Q361–№Q364 are answered ⭐
  > 💡👾 Later: this node has one owner account, so the starter works today.

Not blocking: [Q276–Q278](QUESTIONS.md#runner-hardening), the runner-hardening questions moving to R1.2; the secret-file hiding here rests on one account per agent.
