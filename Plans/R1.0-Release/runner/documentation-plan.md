# Runner documentation plan

The managed runner needs its own guide: it is an external program that uses
AgentBus, not a daemon subsystem. Explain why someone would use it, what they
can do today, and how to get an answer from their first agent before describing
configuration choices.

Separate means a program boundary: the runner remains in this repository,
uses its shared version, and ships through the same setup and release.

## Reader goals

- Run a script as an agent without writing bus integration.
- Install code once and configure separate instances without editing it.
- Know whether an agent is available, why it was refused, and where to look next.
- Understand who may call or manage it and where private settings belong.
- Choose the managed runner, a foreground script, or an agent that serves itself.

## Proposed home

Keep the user guide together in **`docs/runner/`**, with its own entry point.
Link to it from the repository README, setup guide and user-guide index.
Keep implementation plans, unfinished features and unresolved choices under
`Plans/R1.0-Release/`; they are not operating instructions.
Re-scope `docs/user/runner.md` to foreground scripts and
`docs/06-runner-role.md` to the foreground/launcher contracts in the same
change that introduces this guide. Those pages link here for managed use;
they do not retain copies of managed-runner instructions.

| Page | Reader's question | Owns |
|---|---|---|
| `README.md` | Why use the runner, and is it right for me? | Benefits, short capability/status summary, distinction from the bus and foreground command, reading paths |
| `getting-started.md` | How do I run my first agent and get an answer? | Prerequisites, installation links, one minimal script, configuration, reload, call, expected result, cleanup |
| `agents.md` | How do I package code and run separate instances? | Author-supplied files, message input and answers, templates, instance naming, links to configuration reference |
| `configuration.md` | What do I configure, and where? | Code versus host settings, inventory, environment and secret precedence, required variables; supported options and exact syntax in closed details |
| `operations.md` | How do I check it, change it, and recover? | Status, journal, code updates, reload-driven removal, shutdown, failures and troubleshooting; only available controls |
| `security.md` | What access am I granting, and what protection do I have? | Caller versus operator permissions, ownership, credentials, secret handling, actual isolation and its current limits |

Split a reference page out only when its detail would make these pages hard
to scan. No empty pages for features that have not shipped.

## Page shape

- Open with a short summary of the outcome and why it matters.
- Follow with a few bullets containing the main points or choices.
- Give the shortest useful procedure and its expected result.
- Keep necessary cautions beside the affected step, especially current isolation limits.
- Put optional examples and syntax tables in closed `<details>` blocks.
- Move lengthy reference material to a linked file; do not hide an entire guide in one expand.
- Describe names, files, permissions and outcomes users control. Keep read-loop
  scheduling, token-fetch machinery, cgroup construction, APIs and implementation
  wave IDs out of the user journey.

## Diagrams

- Overview: caller → AgentBus → runner → script, with the answer returning
  under the agent's name. Show separate program boundaries, not internal plumbing.
- Configuration: one code package → two configured instances → two agent names.
- Add a lifecycle diagram only when the supported lifecycle needs one; do not
  diagram planned restart or pool behaviour as if it already works.

Use small Mermaid diagrams with user-facing labels. Each must answer a question
more clearly than a few bullets would.

## Source reconciliation

Read the three runner plan pages, runner decisions/questions/waves, foreground
contract and user guide, and runner-related setup/process/messaging sections.
Check executable behaviour and named tests before turning any claim into an
instruction.

Current discrepancies already found:

- The runner overview says the managed program is not built, while setup and
  the wave list record the program, unit and one-message serving as built.
- The configuration introduction says file formats are unwritten, above approved
  file formats and an implementation that reads them.
- Superseded fallback routing is already marked as history; keep it out of the
  user guide rather than treating it as current behaviour.
- Foreground scripts and AI-session launchers occupy the same runner documents;
  neither should define the managed runner's scope.
- Managed sandboxing is planned; today's managed execution is unsandboxed.
  The new guide must say that before asking someone to run third-party code.
- Accepted configuration keys do not necessarily activate a supported feature.
  Separate supported behaviour from reserved or pending options.
- Current foreground documentation wrongly implies setup installs no managed
  program or unit. Correct that claim when adding the new entry point.

The capability reference must distinguish **enforced**, **refused**, and
**accepted but not implemented**, checked against the code at drafting time.
In particular, accepting `sandbox: on` or resource limits does not currently
enforce them. A process maximum currently controls concurrency; a minimum
does not yet keep processes running. Do not present either as supervision.

Managed scripts return stdout as the answer; stderr goes to the runner's
unit journal. `agent-bus logs` is for foreground runs. Explain how to identify
the managed unit without relying on ambiguous process titles. Track title
ambiguity separately as a code finding, not a documentation change.

## Work and review

- [x] Read the runner documentation, related contracts and current entry points.
- [x] Propose the audience, directory, page ownership and diagram subjects.
- [x] Review the layout with Claude `#claude/ab-dvp@parf.us` before drafting.
- [ ] Agree a built/pending matrix from code and tests, without resolving owner questions.
- [ ] Draft the overview and first successful call; have Claude follow the instructions
  against an isolated fixture, never by reloading the live runner.
- [ ] Draft packaging, configuration, operations and security pages.
- [ ] Promote built material from plans into its single owning guide section;
  leave future design in plans and link it to the guide.
- [ ] Update old entry points and inbound links without breaking useful existing anchors.
- [ ] Have Claude review accuracy, user focus, duplication and missing journeys.
- [ ] Check relative links, anchors and every documented command in an isolated fixture.
- [ ] Commit the documentation by exact paths; do not push.

Completion means a reader can choose the runner, get a real answer, configure
another instance and diagnose a refusal without reading a design plan or source.
The guide makes no promises about features that are only planned.
