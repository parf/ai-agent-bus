# CLAUDE.md

All repository conventions live here. `AGENTS.md` only points here.

## What this repo is

agent-bus V2 connects AI agents and services through a registry, broker and faces.
Current development is MVP; [scope and status](Plans/MVP/README.md#scope) distinguish built code from pending requirements.
Legacy-V1 lives at `/rd/service/agent-bus/`, with its design at `/rd/vhosts/realty/Plans/PRF-25/`. Read those when a legacy fact is needed; this repository builds V2.

## Document map

| Home | Owns |
|---|---|
| [Overview](docs/00-overview.md#document-ownership) | Current MVP topic ownership |
| [Glossary](docs/glossary.md#names) | Current naming and links to definitions |
| [Decisions](docs/decisions.md#settled) | Current decision names and links |
| [Plans](Plans/README.md#stages) | Stage status and release scope navigation |
| [MVP](Plans/MVP/README.md#scope) | Active stage knowledge; current contracts remain in docs |
| [R1](Plans/R1/README.md#scope), [R1.1](Plans/R1.1/README.md#scope), [R1.2](Plans/R1.2/README.md#scope) | Future release knowledge and plans |
| [Future](Plans/Future/README.md#topics) | Generic undecided or unassigned ideas |
| `Plans/done/` and each plan's `done/` | Historical completion evidence, never current contracts |

## Working rules

- **No data models, schemas or wire formats until the owner asks.** Do not invent tables of fields, JSON shapes or endpoint lists during planning.
- **Current scope.** `docs/` describes the whole MVP scope, with built and pending explicit. A pending requirement is not an implementation claim.
- **Future scope.** Future design belongs in `Plans/<release>/README.md` or its topic files. Use `Plans/Future/` when the release or idea is undecided. Do not assign an idea to the current release merely because it was mentioned now.
- **Canonical home.** Every value has one owning section. Any other document may restate its claim in one sentence with a section link. If changing the value would require editing the summary, remove the value. The root README CLI example block is the sole example exception.
- **Plan lifecycle.** Follow `/rd/vhosts/realty/Plans/README.md`: README and topic files hold stable knowledge; TODO holds unfinished work, dependencies and falsifiable acceptance; DONE holds concise results; FUTURE holds follow-up; QUESTIONS holds only unresolved choices; DECISIONS holds dated decision names, brief rationale and links. Move completed task detail to `done/`, completed stages to `Plans/done/`. Never renumber task IDs.
- **Promote on acceptance.** When future scope becomes current, move its accepted substance to the owning current doc, update built/pending status and links, and leave plan history pointing to that home. Do not maintain competing copies.
- **Decisions need two edits.** Put substance in its owning current doc or future plan topic, and add a named link in `docs/decisions.md` for current scope or the owning plan's `DECISIONS.md` for future scope. A row names the decision; it does not restate values. Remove its resolved question at the same time.
- **Revising decisions.** Change the substance, record the replacement, and move the old decision reference to superseded history. Do not reopen a settled decision without an owner reason; an implementation gap is pending work, not a new decision.
- **Question IDs.** One namespace across all plans, including settled questions. Check decision records and Git history before allocating an ID; never reuse one merely because its question was removed.
- **Open choices.** The owning plan's QUESTIONS file is canonical. A topic points to the question rather than copying it. Preserve unresolved contradictions as questions; never silently pick a mechanism.
- **Section links.** Cross-references target the defining section, with short labels. Prefer plain-word headings and stable anchors. Historical snapshots are labeled history and do not override active knowledge.

## Verification

Code changes require `src/smoke.sh --slow` green; it runs vet and race tests. The fast subset is edit-loop feedback only. Break each behavioral fix and watch its named check fail; reproduce review findings before accepting them. For a documentation-only change, check internal paths and anchors, scope/status consistency and question/decision migration; no version bump or runtime test rerun is required solely for prose edits.

### Mutation first, then belief

**A green check is evidence of nothing until it has been seen to fail**, and a
mutant credited by a build failure takes no credit — it must compile and fail on
an assertion. Every shape below was actually found in this repository:

| Shape | The check that had it |
|---|---|
| taking the sender's word for a delivery | `ab_reply` said it replied; the routing was nonsense |
| counting nothing | a doubled delivery passed a check that only looked at the last one |
| grepping output that is non-empty either way | `$(cmd; echo -n nothing)` contains *nothing* whatever `cmd` did |
| grepping a word the success answer also contains | a refused receipt and an accepted one both say `receipt` |
| exercising a different path than the one it names | a *waiting reader* check whose inbox still held a message never blocked, so it measured the queued path twice |
| leaning on the refusal to end the command | `start` exits when refused and runs forever when not, so removing the refusal hung the check instead of failing it — bound anything whose success does not return |
| signalling the wrong process | `f() { ...; } &` backgrounds a subshell, so the service never got the signal |
| asking a dead process what it left behind | an orphan is reparented to init the moment its parent dies, so `pgrep -P <that parent>` is empty however badly it left — take the pids *before* the kill |
| matching a class name in the inline stylesheet | every page carries the stylesheet, so assert the element form |
| asserting an identity is absent from a listing | the signed-in name is in the page header, so assert the listing's own row |
| a refusal aimed at a subject that does not exist | `/subscribe` was refused for the *caller* being unregistered, so the check passed whichever field the daemon read |
| a presence check against a paginated listing | the page holds 25 rows, so it passed or failed on where the name sorted — narrow it with `?q=<name>` |

**Naming a shape does not remove it.** Sweep for repeats as part of the fix, not
as a later tidy. Every mutation is measured against `--slow`: a mutant that
survives because its check was skipped is the worst kind of green. `src/smoke.sh`
is read by byte offset while it runs — never edit it mid-run.

## Writing conventions

- Small files, main ideas only, tables over prose; terse English.
- Current docs open with a `📌 **TL;DR:**` paragraph: the essence in two to four lines, never a single clause. Lead sections with a short summary; put supporting detail in closed `<details>` / `<summary>` blocks and remove duplication.
- Use R1, R1.1 and R1.2 for stages; Legacy-V1 for the NATS system.
- Beyond the TL;DR marker, no glyph by default. Use question, conflict, failure, blocked, cancelled, deferred, partial, done, handed-off and superseded glyphs only when they add information. One glyph per cell.

## Design boundaries

Read the owning section before changing a boundary:

| Boundary | Home |
|---|---|
| Layering and dependency choice | [modules](docs/10-modules.md#the-rule), [external tools](docs/10-modules.md#external-tools) |
| Authentication and identity | [access](docs/02-access.md#what-a-call-carries), [names](docs/01-identity-and-roles.md#names) |
| Credential lifetime in every release | [token lifetime](docs/02-access.md#token-lifetime) |
| Visibility and use | [ACL](docs/02-access.md#acl) |
| Private configuration | [configuration](docs/03-records.md#configuring-a-template) |
| What an external service is and is not | [services](docs/06-services.md#what-a-service-is) |
| Body trust and persistence | [trust boundary](docs/02-access.md#trust-boundary), [durability](docs/04-messaging.md#durability) |
| Process privilege and exec | [process boundary](docs/11-processes.md#the-rule) |

## Licensing

The root [license](LICENSE.md#polyform-noncommercial-license-100) owns the project's terms. Keep package license metadata aligned with it and include the license in distributions. Dependencies retain their own licenses and required notices.

## Versioning

| Rule | |
|---|---|
| Shared SemVer | Every program and face shares one `MAJOR.MINOR.PATCH`; never version the daemon, runner, CLI or MCP independently |
| Canonical value | `src/internal/version/VERSION`, embedded by Go and read by TypeScript; no other current-version literals |
| MVP | `0.5.x`, then `0.6.x` from the record-kind enum, both released; `0.7.x` from the constitution work, which starts at `0.7.0`. Bump PATCH on every good development step — each feature or piece of progress, many small bumps rather than few — and on a shipped behaviour fix. A shared change still gets one bump |
| Stable | An even minor (`0.6`, `0.8`) is a feature line. An odd minor (`0.7`, `0.9`) is major development whose code may be broken; when it passes all tests and reviews, its commit subject is exactly `STABLE - passed all tests and reviews`, and the next even line begins |
| Later releases | Before stability, MINOR advances the release line. From major one, breaking changes bump MAJOR, compatible features MINOR, fixes PATCH; reset lower components when advancing a higher one |
| No behaviour change | Docs, tests and refactoring alone do not require a bump |
| Changelog | Every version bump includes a one- or two-line version summary in the current line's changelog — `CHANGELOG.0.7.md` from `0.7.0` — newest version on top. One file per release line; historical release numbers stay in theirs and are never rewritten to match the current version |
| Build evidence | Shipped Go binaries must carry [setup § build information](docs/09-setup.md#build-information) and expose it through the version query. Use the documented build script; never ship an unstamped development build |

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

**Nothing throwaway enters the tracked namespace.** All of the below is in
`.gitignore`; the point is to use it rather than to rely on it.

| Where | What |
|---|---|
| `tmp/` | temporary data — output, logs, dumps, a file written to be read once |
| `tmp/scripts/` | one-off scripts: a sweep, a checker, something driving a test by hand |
| `*.local`, `*.local.*` | a throwaway that has to sit beside the thing it belongs to — `config.local.json` next to `config.json` |
| `local/` | the same, when there are enough of them to want a directory |

Do not write scratch into `/tmp` either: it is shared with every other
process on the box, it is not cleaned up on any schedule anybody controls,
and a name collision there is somebody else's afternoon.

## Other agent configs

An OpenAI Codex config exists at `~/.codex/config.toml`. To import user-level
items (MCP servers, slash commands, subagents, skills, instructions), reply
`/import` to see what is importable, then `/import --yes=<digest>` to apply it.
