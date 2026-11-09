# CLAUDE.md

All repository conventions live here. `AGENTS.md` only points here.

## What this repo is

agent-bus V2 connects AI agents and services through a registry, broker and faces.
R1 targets a clean installation. Its current scope is in [the release plan](Plans/R1.0-Release/README.md#scope); earlier development records are historical evidence.
Legacy-V1 lives at `/rd/service/agent-bus/`, with its design at `/rd/vhosts/realty/Plans/PRF-25/`. Read those when a legacy fact is needed; this repository builds V2.

## Document map

| Home | Owns |
|---|---|
| [Overview](docs/00-overview.md#guides) | User documentation and current reference |
| [Glossary](docs/glossary.md#names) | Current naming and links to definitions |
| [Decisions](docs/decisions.md#settled) | Current decision names and links |
| [Plans](Plans/README.md#stages) | Stage status and release scope navigation |
| [R0.8](Plans/done/R0.8-MVP/README.md#scope), its [web face](Plans/done/R0.8-MVP/web/README.md#scope) | Released history; current contracts remain in docs |
| [R1](Plans/R1.0-Release/README.md#scope), [R1.1](Plans/R1.1/README.md#scope), [R1.2](Plans/R1.2/README.md#scope), [R1.3](Plans/R1.3/README.md#scope) | Future release knowledge and plans |
| [R2.0](Plans/R2.0-Future/README.md#topics) | Generic undecided or unassigned ideas |
| `Plans/done/` and each plan's `done/` | Historical completion evidence, never current contracts |

## Working rules

- **No data models, schemas or wire formats until the owner asks.** Do not invent tables of fields, JSON shapes or endpoint lists during planning.
- **One canonical interface.** R1 accepts only its current field shapes, command names, flags and layouts. Retired forms fail explicitly; remove their decoders, aliases, converters and fallback producers from scripts, packages, fixtures and docs together. No prerelease installation or data-conversion compatibility is required. Current atomic operations and failure cleanup are not an excuse to retain obsolete input formats.
- **Current scope.** `docs/` describes shipped behavior and useful current limits, without release-stage narration, build status, roadmap or internal evidence. Keep history and unshipped scope in Plans, and contributor mechanics in contributor docs.
- **Future scope.** Future design belongs in `Plans/<release>/README.md` or its topic files. Use `Plans/R2.0-Future/` when the release or idea is undecided. Do not assign an idea to the current release merely because it was mentioned now.
- **Canonical home.** Every value has one owning section. Any other document may restate its claim in one sentence with a section link. If changing the value would require editing the summary, remove the value. The root README CLI example block is the sole example exception.
- **Plan lifecycle.** Follow `/rd/vhosts/realty/Plans/README.md`: README and topic files hold stable knowledge; TODO holds unfinished work, dependencies and falsifiable acceptance; DONE holds concise results; FUTURE holds follow-up; QUESTIONS holds only unresolved choices; DECISIONS holds dated decision names, brief rationale and links. Move completed task detail to `done/`, completed stages to `Plans/done/`. Never renumber task IDs.
- **Promote on acceptance.** When future scope becomes current, move its accepted substance to the owning current doc, update the owning plan status and links, and leave plan history pointing to that home. Do not add built/pending tracking to user docs. Do not maintain competing copies.
- **Decisions need two edits.** Put substance in its owning current doc or future plan topic, and add a named link in `docs/decisions.md` for current scope or the owning plan's `DECISIONS.md` for future scope. A row names the decision; it does not restate values. Remove its resolved question at the same time.
- **Revising decisions.** Change the substance, record the replacement, and move the old decision row to [the decision archive](Plans/done/decisions-archive.md#decision-archive); the archive is history, never a current contract. Do not reopen a settled decision without an owner reason; an implementation gap is pending work, not a new decision.
- **Question IDs.** One namespace across all plans, including settled questions. Check decision records and Git history before allocating an ID; never reuse one merely because its question was removed.
- **Open choices.** The owning plan's QUESTIONS file is canonical. A topic points to the question rather than copying it. Preserve unresolved contradictions as questions; never silently pick a mechanism.
- **Section links.** Cross-references target the defining section, with short labels. Prefer plain-word headings and stable anchors. Historical snapshots are labeled history and do not override active knowledge.

## Verification

| Rule | |
|---|---|
| Landing | `src/smoke.sh --normal` green; no flag (`--fast`) is NOT A LANDING GATE; `--slow` for huge changes and `--slow all` for a release ([sets](.claude/docs/testing.md#gate-sets)) |
| Docs only | `src/check-links.py`; no runtime gate and no version bump for prose |
| Lock | Before a normal, slow or mutant run, take the `slow` lock on `smoke-gate@parf.us` first, alone, under your own bus name via MCP `ab_lock`, never the `parf` CLI; `parf` holds only for `#ab-gate` and the owner. A hold you did not just take is not yours: wait; never extend, release or run under it ([the gate lock](.claude/docs/testing.md#the-gate-lock)) |
| Codex | Codex workers never run normal or slow smoke; they run only focused tests and mutants ([who runs what](.claude/docs/testing.md#who-runs-what)) |

Selection, quarantine, agent shards and host limits: [testing](.claude/docs/testing.md#testing-and-the-gate).

### Mutation first, then belief

- A green check proves nothing until it has been seen to fail; a mutant must compile and fail on an assertion. Reproduce review findings before accepting them.
- A new check, or a fix in a ⚠️ subsystem, gets a mutant; any other behaviour fix fails before and passes after ([proof table and shapes](.claude/docs/testing.md#mutation-first-then-belief)).
- `src/smoke.sh` is read by byte offset while it runs — never edit it mid-run.

## Writing conventions

- Write long CLI options as `--name` or `--name=value`; reserve a single dash for one-letter options such as `-o`.

- Small files, main ideas only, tables over prose; terse English.
- Doc style is owned by [WRITING.md](.claude/docs/WRITING.md#how-we-write-docs): order, voice, form, examples and the page checklist, for every doc, `agent.json` description, method help and web page intro. Every current doc opens with a `📌 **TL;DR:**` of two to four lines.
- Use R0.8, R1, R1.1, R1.2, R1.3 and R2.0 for stages; Legacy-V1 for the NATS system.
- Beyond the TL;DR marker, no glyph by default. Use question, conflict, failure, blocked, cancelled, deferred, partial, done, handed-off and superseded glyphs only when they add information. One glyph per cell.

## Design boundaries

Read the owning section before changing a boundary:

| Boundary | Home |
|---|---|
| Layering and dependency choice | [modules](src/MODULES.md#the-rule), [external tools](src/MODULES.md#external-tools) |
| Authentication and identity | [access](docs/02-access.md#what-a-call-carries), [names](docs/01-identity-and-authority.md#names) |
| Credential lifetime in every release | [token lifetime](docs/02-access.md#token-lifetime) |
| Visibility and use | [ACL](docs/02-access.md#acl) |
| Record configuration | [configuration](docs/03-records-agent.md#configuring-a-template) |
| What an external service is and is not | [services](docs/03-records-service.md#what-a-service-is) |
| Body trust and persistence | [trust boundary](docs/02-access.md#trust-boundary), [durability](docs/04-messaging.md#durability) |
| Process privilege and exec | [process boundary](docs/08-processes.md#the-rule) |

## Licensing

The root [license](LICENSE.md#polyform-strict-license-100) owns the project's terms, except `src/clients/`: every client library, its examples and fixture are [MIT](src/clients/LICENSE.md#mit-license), one license for all of them. Keep package license metadata aligned with whichever applies and include that license in distributions. Dependencies retain their own licenses and required notices.

## Versioning

| Rule | |
|---|---|
| Shared SemVer | Every program and face shares one `MAJOR.MINOR.PATCH`; never version the daemon, runner, CLI or MCP independently. Each bundled agent is the exception |
| Agent versions | Each bundled agent has its own SemVer, from `1.0.0`: `version` and `released` (the UTC date of its last bump) in its `agent.json`, and its own `CHANGELOG.md` beside it, newest first. `#ab-ceo` raises an agent's PATCH once at deploy, only when that agent changed since its last bump ([Team v2 roles](Plans/Team.v2/README.md#roles)); a landing never bumps it. `src/agents/VERSION` and `src/agents/CHANGELOG.md` are the pack's internal bookkeeping, never shown to users. Bundled Python clients stay on the bus version |
| Canonical value | `src/internal/version/VERSION`, embedded by Go and read by TypeScript, and each agent's `agent.json` `version` for that agent; `src/agents/VERSION` is bookkeeping; no other current-version literals |
| PATCH | Rises once per deploy, never per landing: `#ab-ceo` raises it ([Team v2 roles](Plans/Team.v2/README.md#roles)); nobody else edits the version |
| Deploy | The development node runs from the checkout through `sudo src/git-install.sh`, which links every program and launcher into `/usr/local/bin` and restarts the daemon and every active root-account `agent-bus-runner@*`. Commit first, then deploy only the changed component using [minimal deployment](.claude/docs/deployment.md#deploy-only-the-changed-component). `release.sh` is only for a node running a fixed release; on this one it would replace the links ([setup § install](docs/07-setup.md#install)) |

After deployment, run `src/check-deploy.sh` as an allowed caller. Its optional target (for example `'#debug@<node>'`) overrides the default derived from the realm in `agent-bus status`, falling back to the host name. Any failed or missing selftest fails the post-deploy check. See [debug](docs/agents/debug.md#check-the-bus-in-a-minute).

Lines, the R1 release step, app subprojects, changelog and push forms, and build evidence: [versioning detail](.claude/docs/versioning.md#versioning-detail).


## Git

Commit subjects are a single terse imperative-ish sentence describing the
decision or edit, no prefixes or tags (`Topics are first-class records registered
like services`, `Static-token principals do not sign; signatures only where a key
exists`). One decision per commit.

**Commit as you go.** After every substantial feature, and whenever a piece of
work is complete — not once at the end of a session. Work that is finished but
uncommitted is work nobody else can see and a rebase can lose.

**Other agents are working in this repo at the same time.** So:

- **Stage what you changed, by path.** `git add <paths>` — never `git add -A`,
  `git add .` or `git commit -a`, each of which sweeps up somebody else's
  half-finished work and commits it under your message.
- **Never undo work you did not do.** No `git reset`, no `git checkout --`, no
  `git stash`, no reverting, on files you did not edit. Modified or untracked
  files you do not recognise are another worker mid-task, not mess to clean.
- **Check `git status` before committing** and again after: anything left
  modified that you did not touch is *correct*, and is not a thing to fix.
- A commit of somebody else's may land between yours. Pull and rebase; never
  force-push.

## Scratch files

**Nothing throwaway enters the tracked namespace.**

| Where | What |
|---|---|
| `tmp/` | temporary data — output, logs, dumps, a file written to be read once |
| `tmp/scripts/` | one-off scripts: a sweep, a checker, something driving a test by hand |
| `*.local`, `*.local.*` | a throwaway that has to sit beside the thing it belongs to — `config.local.json` next to `config.json` |
| `local/` | the same, when there are enough of them to want a directory |

- When your commit lands or your review or report is sent, remove your worktree and your `tmp/` files.
- Never write scratch into `/tmp`. Rust builds only in the shared `tmp/cargo-target`.
- `df -h /` before a full gate. Remove only what is yours.
- Why, the full cleanup table and the mktemp exception: [scratch](.claude/docs/scratch.md#scratch-files-and-cleanup).
