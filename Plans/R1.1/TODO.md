# TODO R1.1

## Objective

Prepare the [proposed scope](README.md#scope). Started: items marked ⏳ are in progress; there are no implementation waves committed yet.

Moved to R1 on 2026-11-11, shipped in 1.0.x: FileScan, DBSync, T-35 tab titles and `call --body` ([R1 TODO](../R1.0-Release/TODO.md#moved-from-r11)).

## Next step

Storage scope is assigned; its implementation follows the 0.7 prerequisites.
Other proposed work needs scope confirmation and resolution of
[questions](QUESTIONS.md#open-questions). Existing decisions describe targets, not completed code.

## Dependencies

| Candidate work | Must precede it |
|---|---|
| ☐ [Additional storage backends](storage.md#backends) | SQLite persistence contract and exclusive database access implemented in 0.7 |
| ☐ [Peer registry](registry.md#registry-sync) | Record authenticity and clock decisions ([Q5](../R1.3/QUESTIONS.md#Q5), moved to R1.3 on 2026-11-06; [Q367](../R1.3/QUESTIONS.md#Q367) asks whether the registry follows) |
| ☐ [Optional dashboard extensions](discovery.md#dashboard-extensions) | Required MVP dashboard acceptance; the AUTH, health, stats, federation and runner data each additional view reports |
| ☐ Visitor IP on web-face edits — the face forwards the visitor's address and the audit log records it (Q128) | a trusted way for the face to state it, since the daemon sees only its socket |

Name implementation waves and falsifiable acceptance after those choices. Peer-registry acceptance must exercise distinct nodes.

## Windows CLI client

Owner, 2026-10-30; plan in [Windows CLI client](windows-cli.md#steps-and-estimate), owner answers in [decisions](DECISIONS.md#recorded-decisions), open [Q268](QUESTIONS.md#open-questions). MAC.1 has landed.

| ID | Deliverable | Acceptance and mutation |
|---|---|---|
| ☐ WIN.1 | The `agent-bus` CLI builds for Windows amd64 and reaches a remote daemon over pinned HTTPS ([plan](windows-cli.md#what-you-get)); daemon, runner, sandbox and launchers stay out | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/agent-bus` succeeds in the normal `go_checks` shard, and untagging one Linux-only file fails it. On a `windows-latest` CI job: `agent-bus --version` prints the version and a `build_info:` stamp; a request and a reply complete against an in-process pinned-TLS test daemon; a wrong fingerprint is refused with nothing sent; a missing `AGENT_BUS_ADDR` says it is required; a left-out subcommand exits non-zero naming itself. The Linux and macOS builds and their normal checks are unchanged. Mutants: drop the pin check and the wrong-fingerprint check fails; send the reply-context cache to `$HOME/.cache` on Windows and the cache-location test fails |
| ☐ WIN.1b | The same request and reply against a real daemon, from a Windows machine | Prerequisite: a Windows server the owner provides when it is due (ask `#ab-ceo` through the lead), and a reachable isolated daemon with TLS and a token; not part of WIN.1's acceptance |
| ☐ WIN.2 | `package.sh` writes `agent-bus-client-<version>-windows-x86_64.zip`; `release-verify` and the npm wrapper take it | the archive holds `agent-bus.exe` and no node program; `release-verify` passes it with the build stamp read from the binary, and fails an unstamped one; the npm platform package carries those checked bytes and passes its installed CLI checks, without a signing step |
| ☐ WIN.3 | `ab-claude`, `ab-codex` and `ab-opencode` start or resume a named session on Windows with the bus tools, from one Go module meant for every OS, no bun ([launchers](windows-cli.md#launchers-on-windows)) | On `windows-latest`: the module's tests pass; a binding lock taken twice is refused on the second; a stopped runtime leaves no child; `--take-over` is refused with its reason; `ab-claude` starts a fake Claude and the face lists and sends through a pinned-TLS test daemon. The same tests pass on Linux and macOS with no `GOOS` branch in the shared packages (a grep check of the shared packages for `runtime.GOOS`). Mutants: treat a held lock as free and the double-lock test fails; skip the process-tree stop and the child test fails; put a `runtime.GOOS` branch in a shared package and the grep check fails. Real runtimes on the Windows server follow in WIN.1b |

## Storage acceptance

Owner-assigned, pending. Preserve the task ID carried from 0.7.

| Task | Done when; mutation that must fail |
|---|---|
| ☐ K.1.2 Configure backend selection and add MySQL/PostgreSQL adapters | Implement the [backend contract](storage.md#backends). Run shared storage checks against real SQLite, MySQL and PostgreSQL, covering uniqueness, transactions, restart, failed commits, statistics batching, exclusive access across competing processes/hosts, backup/restore and performance. Ignoring backend selection, bypassing a transaction or allowing two daemons to serve the same database fails the corresponding check |

## MCP agents

Owner request, 2026-10-17; proposed, pending [Q178](QUESTIONS.md#open-questions).

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| ☐ K.46 | An agent answers the body `--mcp` with an MCP `tools/list` result, and is tagged `mcp`; `agent-bus call <to> --mcp` sends it ([MCP agents](mcp-agents.md#mcp-agents)). `#slack` answers it, limited to what the caller may do | Q178; [K.42](../done/R0.8-MVP/TODO.md#agent-self-description) | a valid answer adds the tag, an invalid or missing one does not; `GET /ls?tag=mcp` lists the agent; Slack's tools validate against the MCP schema. A tag added without a valid answer, or Slack's admin tools shown to a plain caller, fails the check |

## MCP call

Owner, 2026-11-17: "we need to add ab_call method to mcp"; "we should recommend to use ab_call - can reduce/simplify traffic"; "make it preferable (edit mcp help)". A typical model sends with `ab_send`, then waits with a filtered `ab_consume`: two tool calls and an invented topic and tag.

- ✅ T-54 `ab_call`: one MCP tool that sends and waits for the correlated answer, the face's form of [`agent-bus call`](../../docs/04-messaging.md#request-and-reply)
  - State: landed 2026-11-17 (`4a235c550`), normal gate green 1339/0; six mutants (no reply hold, no wait, ack as answer, done ignored, no cap, fixed tag) each fail an assertion; under push the real-daemon check gets the answer after an ack and push shows neither. Sessions see it after a restart
  - Arguments: `to`, `text` or `ask`, optional `topic`, `wait` (default `30s`, at most `5m`); the face makes a unique tag, and a topic when none is given, so the daemon's reply hold always applies
  - Send: like the CLI's `callTarget`, the send carries `wait` and `reply_to` naming this session with that topic and tag, so a push reader on the same inbox cannot take the answer
  - Wait: the filtered loop shared with `ab_consume` (an `ack` keeps waiting on what is left of the deadline; a session ask is answered and skipped), in slices of at most the daemon's 60 s
  - Answers: the reply; `done` says the agent finished with no answer, do not resend; a timeout or abort after the send says the message was accepted, do not resend, and names the topic and tag: under push the late answer arrives on its own, otherwise `ab_consume` with them collects it
  - Preferred: the face instructions and the `ab_send` and `ab_consume` descriptions recommend `ab_call` for any request that expects an answer, including `ask help`; `ab_send` stays for one-way messages, `ab_consume` for unsolicited ones; the user guide's recipes use `ab_call`
  - Runtimes: `ab-codex` raises the agent-bus server's tool timeout above the 5 m cap (Codex defaults to 60 s); OpenCode's limit is checked and set the same way if it has one
  - Acceptance, against the face fixture:
    - an answer is returned in one tool call; an `ack` then the answer returns the answer; `done` ends the wait early with its own words
    - with an unfiltered reader live on the same inbox, the answer still reaches `ab_call`; dropping `reply_to` or `wait` from the send fails this check on an assertion
    - two calls in flight whose answers arrive in reverse order each get their own
    - a timeout names the topic and tag, and a later `ab_consume` with them returns the answer
    - a wait over `5m` or not a duration is refused before anything is sent

## MCP latest revision

> **Waiting for Codex and OpenCode.** Postponed from R0.8 on 2026-10-16. The
> MCP face stays on 2025-11-25 until Codex and OpenCode speak MCP 2026-07-28,
> and Claude Code delivers channels on it. On 2026-10-16 Codex 0.163.0 sent
> only a 2025-06-18 `initialize`, OpenCode 2.0.30 only 2025-11-25; Claude Code
> 2.1.302 tries 2026-07-28 first. Recheck on each client release
> ([measurements](../../docs/constitution.md#external-protocols)).

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| ☐ K.37 | The MCP face speaks only the latest MCP specification, under the [2025-11-25 exception](../../docs/constitution.md#external-protocols) until then (Q143). The v2 SDK (`@modelcontextprotocol/server` 2.2.0) serves 2026-07-28 and refuses a 2025 `initialize` with -32022 | Claude Code delivering channels on 2026-07-28; Codex and OpenCode speaking it | the face and its smoke checks negotiate only 2026-07-28; a 2025 `initialize` is refused; push reaches Claude Code on that revision. Pinning the old SDK or accepting an old revision fails the check. Recheck the clients on each release |

## Backups

Owner-moved from R1 on 2026-10-30; task IDs and acceptance are preserved.

- ⏳ RT13 [Backups](backups.md#backing-it-up)
  - Details: decided: the runner's `backup` verb

## Backup acceptance

- ⏳ RBK1 Backup acceptance
  - Acceptance: The [backup contract](backups.md#backing-it-up) is settled, invoked as the runner's `backup` verb. Implementation must restore an archive with the intended user's key and refuse an unrelated key. Replace encryption with plaintext output and the format/decryption check must fail; encrypt to the wrong recipient and the intended-user restore must fail. Confirm backup creation works with only the public key available.
  - Mutant: Replace encryption with plaintext output and the format/decryption check must fail; encrypt to the wrong recipient and the intended-user restore must fail.

## Runner extensions

Moved by [Q299](../R1.0-Release/DECISIONS.md#recorded-decisions), owner, 2026-11-01.

- ⏳ M.6b Edge runners as `#runner/<edge>@<realm>`; [naming](runner.md#edge-runners), settled [Q165 / RD84](../R1.0-Release/DECISIONS.md#recorded-decisions).
  - Acceptance: an edge runner registers as `#runner/<edge>@<realm>`.
  - M.6a sharing and hostname reporting stay in [R1](../R1.0-Release/TODO.md#runner-waves).

- ⏳ M.7 Named hosts (audit 2026-10-30: the network is on or off, nothing filters hosts)
  - M.7a the mechanism is chosen and its format approved (owner question); M.7b the proxy and the no-route bypass check (L)
  - Builds: the filtering proxy for `needs` hosts
  - Acceptance: a named host is reached and another refused; bypassing the proxy has no route
  - Mutant: set with the mechanism (M.7a): a request to a host outside the list reaches it, and the no-route bypass check passes without the proxy.
  - Home: [named hosts](runner.md#named-hosts); [open policy](QUESTIONS.md#runner-extensions).

- 💡 RT19 A root runner that replaces the standalone starter (owner, [Q336](QUESTIONS.md#runner-extensions)): [approach](root-runner.md#what-the-starter-is-today) for approval, [open choices](QUESTIONS.md#root-runner); no format chosen and nothing built.

## Runner web additions

Proposals to prioritize after the minimum. RT11 RW10–RW17, moved by Q299, and RW4 and RW8, moved by [Q311](../R1.0-Release/DECISIONS.md#recorded-decisions); [topic](runner-web.md#useful-additions). R1 minimum and RW19/RW21/RW27 remain in [R1](../R1.0-Release/runner/web-face.md#acceptance-checklist).

- ⏳ RW10 Search and filter agents by name, tag, state and refusal reason; count
  - Details: before pagination and only over the caller-visible set.

- ⏳ RW11 Show running work versus capacity, queued work, last failure and last
  - Details: successful status refresh when the runner supplies those observations.

- ⏳ RW12 Show effective sandbox/resource limits and warnings when protection is
  - Details: disabled, without implying settings the running version does not enforce.

- ⏳ RW13 Explain configuration precedence and whether changes need reload or a
  - Details: process restart; provide a deliberate restart action when supported.

- ⏳ RW15 Offer bounded, permission-checked logs and diagnostics when available,
  - Details: with secret redaction and a clear unavailable state.

- ⏳ RW16 Add agents from installed packages/templates, then install/update/remove
  - Details: packages when the runner supports remote management. Keep removal and interruption behind consequence confirmations.

- ⏳ RW17 Recheck status after an action and show the resulting agent state; refresh
  - Details: without changing a separate agent editor or overwriting a newer action result.

- ⏳ RW4 Keep the runner's last observation and show it as stale when status fails (low priority)
  - Owner, 2026-11-02 (Q311): "ideally just ignore; our values are ~ good enough for most cases". Moved from R1; today a failed status shows the runner with its agents "not known".
  - Details: the web face keeps nothing between requests, so this needs a small cache; the full text and tests are in [the R1 checklist](../R1.0-Release/runner/web-face.md#minimum-checklist).

- ⏳ RW8 Mark which settings are saved but not yet applied (low priority)
  - Owner, 2026-11-02 (Q311): "ideally just ignore; our values are ~ good enough for most cases". Moved from R1; today the page explains that a saved change takes effect at the next runner reload.
  - Details: needs a per-setting comparison between the record and the running process; the full text and tests are in [the R1 checklist](../R1.0-Release/runner/web-face.md#minimum-checklist).

## Agent capabilities

- ☐ RT23 Agent capabilities: every agent answers `help` (minimum); callers learn which further asks an agent answers before sending them (owner, 2026-10-26)
  - Plus an optional `status` an agent may declare; the web shows it and may offer actions based on it (owner, 2026-10-26)
  - Exposed to bus MCP users too: `status` as an `ab_send` ask, and the capabilities line read before other asks (owner, 2026-10-26)
  - Q191 settled: status only on the agent’s own page; no mass polling or status in `ab_ls`; the web offers restart on `failed` only ([policy](agent-capabilities.md#proposal), [decision](DECISIONS.md#recorded-decisions)).
  - Acceptance: the web record page and doctor send no ask an agent did not declare; an agent that answers only `help` costs one round trip and never a timeout
  - Remaining: document and implement capability discovery (constitution § agent self-description, authoring checklist, create-agent prompt), updating every bundled and standalone agent and the MCP face. The settled status policy is not reopened.
  - Home: [proposal](agent-capabilities.md#proposal).

## Node cleanup

- ⏳ B.15 Node cleanup with a lasting bound (built: `src/maintenance/node-cleanup.py` and its weekly timer ran on 2026-11-01; the one-week acceptance below is still open): [proposal and observation](node-cleanup.md#lasting-bounds), [RD110](../R1.0-Release/DECISIONS.md#recorded-decisions).
  - Acceptance: after one week of normal work, none of the Go build cache, shared Rust target, abrt crash dumps, old release versions, logs, backups or test `/tmp` leftovers has grown past its stated bound, and the disk report shows it. Review the stale-session list and back up the daemon before removing records; observation itself authorizes no timer or deletion.

## macOS sandbox

- ☐ MAC.3 Sandbox: [proposal](macos.md#mac3-sandbox); deferred, built only if the owner asks.
  - Acceptance: mirrors the Linux profiles; a mutant that lets a denied access through fails the owning assertion.

## Calendar

Owner, 2026-11-03 11:04 EST: "todo - add "Calendar agent, add/edit/delete/subscribe/smart-query-with-cache"; 11:09: "in R1.1". Not planned: no plan, provider or acceptance yet, and no code until the owner approves a plan.

- ☐ CAL.1 Calendar agent: add, edit and delete events, subscribe to changes, and answer smart queries from a cache
  - Next: a plan in `calendar.md` naming the provider, what the cache holds and how it stays fresh, with steps, an estimate and falsifiable acceptance against a fake provider

Owner, 2026-11-10: "add to todo - apple & google calendar integration agents". Not planned; the same plan decides whether these are separate agents or providers behind CAL.1.

- ☐ CAL.2 Google Calendar integration agent
  - Next: in the `calendar.md` plan, the Google account authorization and how the token is stored and renewed, change notification against polling, and acceptance against a fake Google Calendar API
- ☐ CAL.3 Apple Calendar integration agent
  - Next: in the `calendar.md` plan, iCloud CalDAV with an app-specific password, how changes are noticed, and acceptance against a fake CalDAV server

## Alexa

- ☐ AL.1 `#alexa`: control Alexa devices from the bus by wrapping alexacli by codenamejason (Go CLI; repository link to be confirmed) (owner, 2026-11-01: "for 1.1 - alexa integration")
  - Builds: a runner agent around the alexacli Go binary: `devices` (list), and `say <text>` mapped to `alexacli voice-command`, so "turn on the kitchen light" works as text; Alexa+ text conversation where alexacli supports it; its direct smart-home API is still a work in progress upstream
  - Depends: an owner plan for approval first (methods, Configuration, which account); alexacli talks to Amazon through an unofficial API, so the Amazon session is the record's Secret and the agent must say when Amazon breaks it
  - Risks to settle in the plan: device commands act in the physical home, so every acting method is `[destructive]` and the allow list stays owner-only by default; credentials and refresh; Linux packaging of the binary (Homebrew on macOS)

## Agent images

- ✅ IMG.1 Every agent has an optional image, shown on the web (owner, 2026-11-07); plan: [agent images](agent-images.md#rules); owner answers 2026-11-11 ([№Q395](resolved.md#Q395)–[№Q398](resolved.md#Q398)); [№Q400](resolved.md#Q400) one house style
  - Builds: `image` in `agent.json` with its hash in help, validated by type, size, shape and path at `agent_pack`, at package load and at serve; the `image` ask, answered by the runner from the package beside `help` for a runner-served agent and by the shared helpers for a self-hosted one; one web route that reads a bundled agent's file from the installed pack first and otherwise asks, caching by hash; the field and how to make an image in the agent guide and create-agent prompt; 24 bundled images after the owner OKs the model and cost.
  - Done: `c8128571` (field, checks, help hash, the `image` ask, runner and helpers, web route and header), then the TypeScript agents, packaging, list thumbnails and the 24 bundled pictures made with `openai/gpt-5.4-image-2` (about $0.21). Live once deployed: the daemon (the ask), the web face, runners and agent packages.
  - Acceptance: a mutant per refusal, each failing an assertion that names it (absolute path, `..`, a symlink, SVG renamed `.png`, a 257 KiB file, a non-square image, a PNG and an AVIF header each claiming 60 000×60 000); the web serves a bundled agent's image from disk with no ask (a fixture agent that fails if asked); with the file absent from disk it asks, and a runner-served fixture agent's script never runs for the answer; an unchanged hash is not fetched twice and a changed one is; an offline agent with a cached image still shows it, one never fetched shows the placeholder; a maximal 256 KiB image fits the reply; the web serves with `nosniff`, a CSP and the sniffed type, and a viewer the agent refuses gets the placeholder; no test makes a paid call.

## Agent versions

- ✅ AV.1 Every agent has its own version and release date (owner, 2026-11-06); plan: [agent versions](agent-versions.md#rules); owner answers settled ([№Q393](resolved.md#Q393), [№Q394](resolved.md#Q394))
  - Builds: `version` and `released` in every bundled `agent.json`; a `CHANGELOG.md` per agent; `deploy-bump.sh` bumps each changed agent once per deploy and writes the pair; `--help`, `--help-json` and the web page show both; the pack fallback and `AGENT_BUS_BUILD_AGENTS_VERSION` go; [the doc edits](agent-versions.md#doc-edits).
  - Done: shipped in 1.0.54. Every bundled agent carries `version` and `released` in its `agent.json` and its own `CHANGELOG.md`; `.ai-team/deploy-bump.sh` bumps each changed agent once per deploy; help and the web show both (team 2 `ac7bf256`); [versioning](../../CLAUDE.md#versioning) and the `#ab-ceo` [role](../Team.v2/README.md#roles) state the rule.
  - Seeding: the first per-agent bump writes `version` and `released` into every `agent.json` at once, in one `#ab-ceo` deploy step, in the same deploy window as the code that adds the check ([first bump](agent-versions.md#first-bump)).
  - Acceptance: (a) `agent_pack` fails when any agent under `src/agents` lacks `version` or `released`: two mutants, one deleting `version` from one `agent.json` and one deleting `released`, each failing the assertion that names its field; (b) `deploy-bump.sh` sets `released` to the bump's UTC date for every agent it bumps and leaves an unbumped agent's pair alone; (c) the trigger, with two agents A and B: a change only to a `src/agents/shared` file that A declares in `src/smoke-map.json` bumps A and not B; a shared file neither declares bumps neither; a change to the bundled Python client bumps exactly the agents that bundle it; mutant: drop the shared-path lookup, and (c) fails on an assertion; (d) an agent whose `released` is not `YYYY-MM-DD` fails; `--help-json` of two bundled agents reports different versions after a deploy that changed one; an outside agent without them reports neither; a repackage leaves the pair unchanged; the Bun package tests assert the bundle's own pair; `grep -r "support/agents/VERSION"` finds only the pack-marker readers listed in the plan.

## Agents split

- ⏳ AS.1 Split the daemon and the bundled agents: the bundled agents pack moves to its own repository with its own SemVer (owner, 2026-11-02: "split Daemon and Agents - separate repo & semver for bundled agents pack")
  - Before: one shared version for every program and agent ([versioning](../../CLAUDE.md#versioning)); agents live under `src/agents/` and ship with the daemon
  - State: the repository question is settled by [Q331](resolved.md#Q331): one repository. The version part of Q331 is revised by [AV.1](#agent-versions): each agent has its own SemVer, the pack's is bookkeeping. Built:
    - The pack's own `src/agents/VERSION` from 1.0.0 and `src/agents/CHANGELOG.md` (`ee19a656`).
    - Agent-only selection: a change only under `src/agents/` runs that agent's checks and the pack contract (`aeef6364`). The gate gives such a landing an agents-only receipt (J117, in the untracked `local/gate.sh`).
    - The pack's PATCH is raised once at deploy by `#ab-ceo` through `deploy-bump.sh --agents-summary`, never by a landing (T-39 `abc512b3`, usage `d45471cd`). Readers: `#ab-w1b`; earlier deltas `#ab-w1b` and `#ab-qa1`.
  - Remaining:
    - ✅ The pack declares the daemon protocols it supports in `src/agents/protocol.json` (the two identity fields, equal to what the bundled clients speak), shipped in every package and the release layout; the runner refuses a pack agent the daemon cannot serve when it loads and reloads, by name, and never checks an agent of the user's own. Each client library's own check at connect remains the second line.
    - Whether a pack-only change gets its own image tag ([Q341](QUESTIONS.md#container-image-tags)).
  - Superseded by Q331: "its own repository" in the row and in its acceptance. Superseded by AV.1: "its own SemVer" for the pack, and the built pack version above, is deploy bookkeeping only.
  - Depends: an owner-approved plan first: what stays in this repository (daemon, runner, CLI, MCP, clients), which protocol versions the pack declares it works with ([client contract](../../docs/09-daemon-api.md#the-client-contract)), and how install, packaging and the smoke gate change
  - Acceptance: the pack builds, tests and releases from its own repository at its own version; a daemon release with no agent change bumps no agent version; the pack refuses a daemon whose protocol it does not support

## IMAP large transfer

- ⏸ B.31 IMAP large transfer — **deferred (owner, 2026-11-01): parked behind [B.32](../R1.0-Release/TODO.md#bundled-agents)**; no code. [Plan](imap-large-transfer.md#help-text); questions [Q281](QUESTIONS.md#imap-large-transfer)–Q287 (Q285 settled).
  - Builds: `part` in windows, `upload begin|put|status|abort` and `append --upload`, so mail and attachments up to 25 MB pass the 256 KiB answer and 1 MiB request caps; code in `src/agents/imap` only
  - Depends: [B.30](../R1.0-Release/TODO.md#bundled-agents) steps 2 and 4
  - Acceptance: [the plan's acceptance](imap-large-transfer.md#acceptance-for-the-todo-item)
  - Mutants: [the plan's list](imap-large-transfer.md#acceptance-for-the-todo-item)
  - Whether bigfile's IMAP part/append (`1a95ec19`) supersedes B.31 is the owner's later call.

## Web Face AI chat

- ☐ T-34 Web Face AI chat backed by an `ab-*` session; moved from R1 by the owner, 2026-11-03.
  - Home: [Web Face chat](ai-setup.md#web-face-chat), [the backing session](ai-setup.md#the-backing-session).
  - Session: one tmux session running Codex, Claude Code or OpenCode, with bus knowledge and the configured AgentBus skills, bundled in the user-account runner, which most likely starts it on demand. `agent-bus cli` runs the runner command, then tmux attach; the Web Face runs the runner command, then a bus call, into the same tmux session. The chat is per page or generic (owner, 2026-11-03 19:31–19:42 EST, quoted in [the backing session](ai-setup.md#the-backing-session)).
  - Depends: T-33's Markdown workflow; inspect the owner's mdhouse implementation before design/build.
  - Acceptance: a real conversation reaches its backing session, asks for required setup choices, performs the chosen configuration flow and shows progress/results across turns.
  - Acceptance: an email or Telegram setup journey reuses T-33; an unavailable session or unfinished step is shown plainly rather than reported as success.
  - Acceptance: `agent-bus cli` has the runner start a tmux session for each of the three runtimes and attaches to it; a second `agent-bus cli` attaches to the same session, and that session answers a bus question and lists the configured AgentBus skills; a page chat answers about the page it was opened from, and a generic chat reaches the same session; with no session running, the first Web Face chat has the runner start it; a question asked in the Web Face shows in the attached tmux session, so the runner holds one session, not two.

## AI agent methods

- ☐ T-36 The [AI agent](../../docs/agents/ai.md) passes more of its provider's API through (owner, 2026-11-04: "if API have it - we provide it; our idea - transparent proxy"; 2026-11-05: "delay image only", then images too).
  - Builds, step 1: `models`, `model <id>`, `balance`, `key`, `embed`, and each `ask` reporting its tokens and cost. A method the provider lacks is refused with that reason.
  - Builds, step 2 (owner, 2026-11-05: "maybe we can do all"): image generation, with the image models listing and per-endpoint parameters and prices; every image goes to [bigfile](../../docs/03-records-kv.md#bigfiles) and the answer names it.
  - Status: step 1 landed at `db97076d` (1.0.2), step 2a at `c722581c` (1.0.4), step 2c at `2b5afd82` (1.0.5): `models` answers a table of context length, price per million tokens and input kinds, a filter word or `free`, `--json`, and the default in its last line. Step 2b landed at `1bdb1a34` (1.0.11), deployed: `#ai` serves itself as two kept processes, and `image` puts each picture into its own bigfile record for 24 hours and names it; with no image model configured it is refused before any provider call ([Q333](DECISIONS.md)). The image deadline landed at `2700a811` (1.0.13): `image` gets a 90 s ceiling, 75 s of it for generation, and a shorter `--wait` is refused by name.
  - Remaining: the 1.0.13 `#ai` package deploy by `#ab-ceo` (package, runner reload, `check-deploy`; no live paid `image` call); a working provider key from the owner (the deployed one answers 401).
  - Acceptance: every method decoded from recorded provider answers, no live paid call in tests; `embed` caps its batch below the bus message limit and refuses an over-limit request by name; the provider key never appears in an answer.
