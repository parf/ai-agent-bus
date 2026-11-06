# TODO R1.1

## Objective

Prepare the [proposed scope](README.md#scope). Not started; there are no implementation waves committed yet.

## Next step

Storage scope is assigned; its implementation follows the 0.7 prerequisites.
Other proposed work needs scope confirmation and resolution of
[questions](QUESTIONS.md#open-questions). Existing decisions describe targets, not completed code.

## CLI reply body


- ☐ Add `agent-bus call --body` to print only the reply body, without the JSON envelope.
  - Acceptance: `agent-bus call "#time-simple@$REALM" epoch --body` produces the same output as `agent-bus call "#time-simple@$REALM" epoch | jq -r .body`.

## Dependencies

| Candidate work | Must precede it |
|---|---|
| ☐ [Additional storage backends](storage.md#backends) | SQLite persistence contract and exclusive database access implemented in 0.7 |
| ☐ Scoped credentials and encryption | [Settled lifetime policy](../../docs/02-access.md#token-lifetime); remaining key recovery and token grammar decisions |
| ☐ D.1–D.4 encryption carried from MVP | [Encryption acceptance](encryption-wave.md#d--the-bus-stops-reading-payloads); key lifecycle decisions |
| ☐ [Peer registry](registry.md#registry-sync) | Record authenticity and clock decisions ([Q5](QUESTIONS.md#open-questions)) |
| ☐ [Optional dashboard extensions](discovery.md#dashboard-extensions) | Required MVP dashboard acceptance; the AUTH, health, stats, federation and runner data each additional view reports |
| ☐ Visitor IP on web-face edits — the face forwards the visitor's address and the audit log records it (Q128) | a trusted way for the face to state it, since the daemon sees only its socket |
| ☐ AUTH authorization freshness | [Q35](QUESTIONS.md#authorization-refresh), independently of credential lifetime |

Name implementation waves and falsifiable acceptance after those choices. Crypto acceptance must test what the daemon cannot decrypt; peer-registry acceptance must exercise distinct nodes.

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

## Discord

Owner-moved from R1 on 2026-10-30 ("move discord to 1.1"); the task ID is preserved.

- ✅ B.27a Discord: approved methods, Configuration and gateway implemented on `ab-w1c/B27a`, fake-provider and private-bus acceptance; [guide](discord-user-guide.md#discord), [plan](discord.md#discord-agent-b27a), [evidence](../R1.0-Release/DONE.md#b27a-discord). Owner application/server choice [Q244](QUESTIONS.md#messaging-agents) remains before live deployment.
- ⏳ Ship Discord in R1.1 (B.27a follow-up): add `agents/discord/package.sh` back to `src/agents/package.sh` and the file list in `src/cmd/agent-bus-setup/package_install.go`, and turn the packager test that refuses a discord tree into one that requires it
  - Acceptance: the R1.1 archive holds the discord tree and setup accepts it; the check that the R1 archive holds none is replaced, not deleted

## Runner hardening

Owner-moved from R1 on 2026-11-01; M.14 and Q276–Q280 keep their IDs.

- ☐ M.14 Runner hardening — T-18b, **plan only; owner approval required before code**. [Proposal](runner-hardening.md#runner-hardening-proposal); [Q276–Q280](QUESTIONS.md#runner-hardening). The owner named runtime-file exposure, self-serving bus access and host sharing policy; live `jsonl` work-size checks and a finite CPU default are **added from T-18 review, owner to confirm**. No mechanism or implementation is selected.

| Boundary | Falsifiable acceptance after approval |
|---|---|
| 1. Runtime files | Put distinct readable markers in this agent's package, an unrelated sibling bundle under `/usr/local`, and an unrelated host directory. An approved representative agent still starts and reads its required runtime/code; unapproved sibling and unrelated markers cannot be read. Writing the runtime or package is refused, while own-work writes succeed. Repeat for per-message, `jsonl` and self-serving forms |
| 2. Self-serving bus access | A self-serving fixture receives and answers one correlated call and performs each approved supporting operation. Attempts at an unapproved operation and access to another account's socket fail without effect. Exercise the owner-approved IP behavior separately, including attempts to bypass any required restriction. An ordinary per-message child gains no bus access from this change |
| 3. Inbox sharing | Test host `share=false` and `share=true` on two readers: the first refuses a conflicting reader, the second admits sharing and handles every sent message once with no loss or duplicate. Exercise runner-issued reads and the owner-selected self-serving scope, then change the setting on reload and assert the approved transition, including in-flight work |
| 4. Work-size checks — **added from T-18 review, owner to confirm** | Keep a `jsonl` process alive, grow its work directory one byte past the approved limit, and assert the approved over-limit response within the approved deadline without a restart/reload trigger. Check work below the limit still succeeds and all processes in the pool count toward the same boundary |
| 5. CPU default — **added from T-18 review, owner to confirm** | Run a busy agent with no CPU override and measure that its whole process group stays within the approved finite default over the approved observation window/tolerance. Check explicit overrides and reload follow the approved policy; a second agent remains able to answer a bounded probe |

The question answers must state observable allowed/denied operations, bounds, transitions and compatibility requirements before implementation acceptance is final. They choose **policy**, not a technical mechanism. Tests use sandboxed fixtures and harmless markers; no real host secret or live account socket is read. Removing each implemented boundary must fail its owning assertion, rather than a setup or build failure. Until approval, no code, runtime experiment or rollout belongs to this item.

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

- 💡 RT19 A root runner: [proposal](runner.md#root-runner), [open choices](QUESTIONS.md#runner-extensions); no design approved.

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

## Filescan

Owner, 2026-11-01; plan in [filescan](filescan.md#steps-and-estimate), owner choices in [decisions](DECISIONS.md#recorded-decisions), open [Q305–Q310](QUESTIONS.md#filescan). Plan only: no code until the owner approves the plan.

- ☐ FS.1 `#filescan/<name>`: a runner agent that tracks new, changed, deleted and renamed files in one directory and hands them to subscribed agents, by polling, by inotify, or both ([modes](filescan.md#modes)); steps F0–F4 with an estimate of four to six hours of agent time
  - Depends: RT31 (user-account runner, landed); F0 adds a read-only bind of one directory to the runner
  - Acceptance, each against a fixture directory and a fake subscriber, never the live bus:
    - **Poll:** a created, changed, renamed and deleted file are each sent once; a `touch` with the same content sends nothing; a file made and removed between two scans sends nothing
    - **Watch:** a file written and closed is sent within one second of the close; a rename inside the tree is one `renamed` change; an overflow (forced by a tiny queue) triggers a rescan that finds the missed file; with the watch limit set to one, the agent watches that directory, polls the rest, and `status` says so
    - **Both:** the same file arriving by watch and by poll is sent once
    - **Confirm:** a subscriber that never replies keeps its changes pending and receives them again; a second subscriber is not held up; a file is marked tracked only after the ok reply
    - **State:** killed between the temporary file and the rename, the old state loads and no truncated file exists; at `state_max` the scan is refused with the `state full` text and the file is byte-identical
    - **One scan:** a second scan started during a walk is refused by the lock
    - **Mass delete:** with `dir` unmounted, missing, or emptied past `delete_pct`, no delete is sent on either path, `status` names the guard, and `scan confirm:true` releases it
    - **Authority:** `sub add` and each forward need the author's may-send, a target that does not admit `#filescan` is refused, and the per-creator and total caps hold, as in [IMAP](../../docs/agents/imap.md#who-may-use-it)
    - **Host:** `dir` through a symlink leading out, `/`, the home root and the runner state directory are refused; the agent cannot write inside `dir`; inotify events arrive for a path bound read-only into the sandbox
    - **RAG form:** `as rag` sends upsert and delete with the path as id (hash over 128 characters), a rename is delete then upsert, and nothing is tracked on a refusal
    - **First run:** `baseline:true` sends nothing for existing files; `max_per_scan` caps one scan and `status` shows the backlog
  - Mutants (each must compile and fail on an assertion): send deletes while the guard is held; skip the `dir` symlink and root refusals; write the state in place instead of rename; mark tracked before the reply; drop the cookie pairing; ignore the overflow; drop the lock; ignore `state_max`


## DBSync

Owner, 2026-11-02; plan in [dbsync](dbsync.md#steps-and-estimate), open [Q314–Q320](QUESTIONS.md#dbsync). Plan only: no code until the owner approves.

- ⛔ DS.1 `#dbsync/<name>`: a runner agent that tracks new and changed rows of one table by an `updated` column, asks through a read-only [DB agent](../../src/agents/db/guide.md#what-it-does), and hands the rows to subscribed agents; steps D0–D3 with an estimate of four to five hours of agent time
  - Depends: the DB agent (built); the owner's answers to [Q314–Q319](QUESTIONS.md#dbsync); [FS.1](#filescan) for the shared core if Q314 says so
  - Follow-up, outside R1.1: [Q320](QUESTIONS.md#dbsync), a counter mode for `updated`
  - Acceptance, each against the DB fixture and a fake subscriber, never the live bus:
    - **Cursor:** three rows sharing one `updated` value with `batch` of two are all sent, once each; a cursor on `updated` alone fails this
    - **Paging:** an uncut short page ends the poll; a full page, or a cut whose scan position advanced, triggers the next query at once from the last raw pair received; the page is `max_rows` minus `ceil(max_rows / 10)`, recomputed each poll; `max_rows` 1 is refused with an alert; `max_rows` lowered after a cursor is saved gives a smaller page and the same saved cursor
    - **Known rows ahead:** `max_rows` 2 (page 1), the first lookback row already delivered and a later row new: the poll pages past the known row, delivers the new row exactly once, raises no refusal, and does not read the first page twice
    - **Partial byte cut:** page 9, the DB agent returns 2 rows and a cut; the next query continues after the 2nd raw pair, also when both rows are already known, and nothing is read twice or skipped
    - **No progress:** a fixture row larger than the DB agent's byte cap returns a cut with no row; DBSync alerts and stops, the scan position does not move, and a second poll does not loop; a page of only known rows is not this case; removing the refusal makes this check hang or advance past the row and fail on an assertion
    - **Late commit:** a row committed after a newer one with an older `updated`, inside `lookback`, is sent once; outside it, it is not
    - **Type guard:** a text key, or an `updated` that is neither numeric nor timestamp, makes configuration load and `sub add` refuse with the named error that counts the failing columns (1 of 2, 2 of 2); a numeric key with a timestamp `updated` passes, and a numeric `updated` with a declared unit (epoch seconds or milliseconds) passes; a plain counter or version number is refused with the named error; any numeric key passes; a name outside the identifier pattern is refused; the statement text never carries a quote from a cursor value
    - **Lookback:** a stale cursor older than the floor starts at the cursor; a recent one starts at the floor by the database's clock, not the agent's (a skewed agent clock changes nothing)
    - **Read only:** every statement the agent sends is a read, and a write attempted through its tier is refused by the database
    - **Authority:** an author without may-send to the DB agent is refused at `sub add` and before a forward
    - **Resend:** a refused or unanswered send stays pending and is sent again; a confirmed one is not
    - Every ⚠️ part (the statement builder, state writes, authority) carries a mutant that fails on an assertion
## Alexa

- ☐ AL.1 `#alexa`: control Alexa devices from the bus by wrapping alexacli by codenamejason (Go CLI; repository link to be confirmed) (owner, 2026-11-01: "for 1.1 - alexa integration")
  - Builds: a runner agent around the alexacli Go binary: `devices` (list), and `say <text>` mapped to `alexacli voice-command`, so "turn on the kitchen light" works as text; Alexa+ text conversation where alexacli supports it; its direct smart-home API is still a work in progress upstream
  - Depends: an owner plan for approval first (methods, Configuration, which account); alexacli talks to Amazon through an unofficial API, so the Amazon session is the record's Secret and the agent must say when Amazon breaks it
  - Risks to settle in the plan: device commands act in the physical home, so every acting method is `[destructive]` and the allow list stays owner-only by default; credentials and refresh; Linux packaging of the binary (Homebrew on macOS)

## Agents split

- ☐ AS.1 Split the daemon and the bundled agents: the bundled agents pack moves to its own repository with its own SemVer (owner, 2026-11-02: "split Daemon and Agents - separate repo & semver for bundled agents pack")
  - Today: one shared version for every program and agent ([versioning](../../CLAUDE.md#versioning)); agents live under `src/agents/` and ship with the daemon
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
  - Home: [Web Face chat](ai-setup.md#web-face-chat).
  - Depends: T-33's Markdown workflow; inspect the owner's mdhouse implementation before design/build.
  - Acceptance: a real conversation reaches its backing session, asks for required setup choices, performs the chosen configuration flow and shows progress/results across turns.
  - Acceptance: an email or Telegram setup journey reuses T-33; an unavailable session or unfinished step is shown plainly rather than reported as success.

## Launcher tab titles

- ☐ T-35 Glyph-led tab titles in the `ab-*` launchers, from Linear PRF-67 (owner, 2026-11-05: "good, - go").
  - Step 1: an agent glyph by runtime and account (✴️, 🔶, 🌟, 🔷 Claude accounts 1–4; ❇️ Codex; 👾 OpenCode), then the label; `AB_TAB_TITLE` sets the label, else the bus name's distinguishing part; an `ab_tab_title` MCP tool lets the model read the recommendation and set its own label; inside tmux the window is renamed too.
  - Step 2: a state marker after the glyph: ❓ the agent waits on a question or approval; 🟥 its last answer ends with the stop marker; cleared on new input or a new turn.
  - Home: [launchers](../../docs/06-launchers.md).
  - Acceptance: labels are sanitised and capped; a model-set label never changes the bus name, identity or session title and resets to the recommendation on launcher restart; the start script's title and the tmux window name are restored on exit, nested launchers leave the outer name alone; Codex and OpenCode events are decoded from recorded fixtures; a runtime without a confirmed event source shows glyph and label only.
