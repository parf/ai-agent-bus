# TODO R1.2

## Objective

Prepare the [tools stage](README.md#scope). Stage extensions are not started; completed catalogue entries are marked below, and remaining extensions wait on their R1 prerequisites.

## Next step

Resolve [catalogue and scope questions](QUESTIONS.md#open-questions), then select the first tools with the owner.

## Dependencies

| Candidate | Prerequisite | Acceptance to carry into its implementation task |
|---|---|---|
| ☑ [Slack agent](done/slack/TODO.md#tasks) | the TypeScript client; runner M.9 to be kept by the runner | built 0.8.137 ahead of the stage on owner request; deployed on the dev node with real tokens 0.8.156, 2026-10-17 |
| telegram, systemd, logwatch — moved to R1 | 2026-10-17, with the runner that keeps them | acceptance is [R1's](../R1.0-Release/TODO.md#bundled-agents); their design stays [here](services.md#the-catalogue) |
| ☐ Ordinary catalogue agents | Runner, ACL and configuration contracts | Deploy and call an agent using the published ordinary agent interface; remove a required interface capability and the exercise fails. Review the daemon diff to detect hidden special handling |
| ☐ [Desktop notifications](services.md#people-and-the-world-outside) | Ordinary catalogue agents | A call to `#notify@srv1` shows the notification on that host's own display; a caller outside the instance's ACL is refused by the daemon before delivery, and a send with no desktop session comes back as a refusal — an answer, not a no-reply |
| ☐ Agent credentials | R1 token scoping | A narrowed credential calls its target and is refused elsewhere; disable target enforcement and the negative check fails |
| ☐ Record expiry | Lifetime and peer clock decisions | An inactive ephemeral record expires while a served record and a kept control survive; removing expiry or the served-record guard fails the appropriate check |
| ☐ Catalogue image | [R1 distribution](../R1.0-Release/distribution.md#container-runtime) and selected catalogue agents | From a clean host, the supplied command boots the image and calls a bundled agent without editing a file; omit a required installed service or break the supplied defaults and the call fails |
| ☐ Contact routing | MVP profiles, contact visibility and identity linkage decisions | Reach the configured contact using its selected route; deny a caller outside its visibility. Bypass visibility and the negative check fails |
| ☐ [Declared record state](records.md#down-and-retired) | Nothing; the authority, the states and their codes are settled | A record declared down refuses a send with `409` and retired with `410`, each in its own words, and neither answers `404` or `500`. A retired name is held against a stranger registering it, and its owner or maintainers bring it back. Collapse the two answers into one code, let a retired name answer *no such name*, or let a stranger take it: each fails its own check. A backlog present when the state was declared is still there afterwards |
| ☐ [Nobody is reading](records.md#coming-back-in-a-moment-is-not-one-of-them) | Nothing; `reading` is already exact — but it holds only a read that accepts any message, so the **answer text** cannot be *nobody is reading*: a read restricted to a topic or tag is attached and takes what matches it. MVP withdrew that phrase from every face; the wording here depends on the [accepted all-reader count](../../docs/05-discovery.md#readers), still pending implementation | A send to a registered service with no reader is accepted, queued, and its answer says nobody is reading; the same send to a service with a reader attached does not say it. A `call` against a reader-less service stops instead of waiting out its deadline, and the message is still queued afterwards — a caller that gave up is not a message thrown away. Refuse the send instead of queuing it, report reading from anything but the readers actually blocked on that inbox, or let a `call` against a live service give up early: each fails its own check |
| ☐ [Agent runtimes](services.md#agent-runtimes) | Ordinary catalogue agents, and a name and token for each runtime | **Both directions, and each falsified alone.** Hermes or OpenClaw, configured against the MCP face, calls a bus service it is allowed and is refused one it is not — widen nothing and the refusal must stay. A bus service sends to `#hermes@srv1` and the agent's answer comes back to the sender. Point the runtime at a second name with fewer grants and the first exercise fails, which is what says the runtime is a principal rather than a door |

| ☐ MCP from agent metadata | R1's [agent method metadata](../R1.0-Release/done/method-metadata.md#settled-so-far) (RT9) | an agent's `tools/list` is generated from its `agent.json` methods, an enum becoming a JSON Schema `enum`; an MCP client calls a generated tool and gets the agent's answer (owner, 2026-10-19: "MCP later") |
The no-daemon-change criterion applies to catalogue entries; whether the whole stage can meet it is an [open scope conflict](QUESTIONS.md#open-questions).

## Go-only client

Owner, 2026-10-30: after the Windows launchers exist in Go ([WIN.3](../R1.1/TODO.md#windows-cli-client)), Linux and macOS move to them and bun leaves the client path. Design and cost: [Windows CLI client](../R1.1/windows-cli.md#r12-linux-and-macos-move-to-the-go-client).

| ID | Deliverable | Acceptance and mutation |
|---|---|---|
| ☐ GO.1 | The `ab-*` launchers and the Claude channel plugin on Linux and macOS run the Go launcher and MCP face; `agent-bus-ts` remains only for the web face; the npm client package, `package.sh` and `release-verify` ship and check the Go binaries | After WIN.3. On Linux and macOS the launcher tests and the launcher smoke run against the Go module and pass with the TypeScript launcher removed from the launcher path; a worker's window restarts by name and resumes its own session, as `launcher.ts` does today ([J34](../issues/2026-10/2026-10-30.md)); `ab-claude --version` and the channel push work from a release archive with no bun installed. A soak of a week on this team's own windows with no regression. Mutants: point a shim back at bun and the no-bun check fails; drop the resume-by-ID and the resume check fails |

## Federation

`RF` labels federation work moved from R1; it allocates no question IDs.

- ⏸️ RF1 Federation chaining across distinct nodes
  - State: moved from R1 on the owner's instruction, 2026-10-17 (№Q137); the earlier hold is not lifted by this move
  - Depends: the owner taking federation off hold; R1.1 signed AUTH generations, pinned peer keys and registry contracts; the namespace choice is settled (№Q20)
  - Acceptance: exercise discovery and calls across distinct daemon nodes, including local shadowing and unresolved fallthrough; a single-node exercise does not satisfy this task
  - Mutant: disable the upstream lookup and the cross-node call must fail
  - Home: [chaining](federation.md#chaining), [scope and namespace decisions](../R1.0-Release/DECISIONS.md#recorded-decisions)

## Discord

Owner-moved from R1 on 2026-10-30 ("move discord to 1.1"), then to R1.2 on 2026-11-05 ("discord => 1.2"); the task ID is preserved.

- ✅ B.27a Discord: approved methods, Configuration and gateway implemented on `ab-w1c/B27a`, fake-provider and private-bus acceptance; [guide](discord-user-guide.md#discord), [plan](discord.md#discord-agent-b27a), [evidence](../R1.0-Release/DONE.md#b27a-discord). Owner application/server choice [Q244](QUESTIONS.md#messaging-agents) remains before live deployment.
- ⏳ Ship Discord in R1.2 (B.27a follow-up): add `agents/discord/package.sh` back to `src/agents/package.sh` and the file list in `src/cmd/agent-bus-setup/package_install.go`, and turn the packager test that refuses a discord tree into one that requires it
  - Acceptance: the R1.2 archive holds the discord tree and setup accepts it; the check that the R1 archive holds none is replaced, not deleted

## Messaging agents

- ⏳ B.27b Google Chat, moved from [R1 B.27](../R1.0-Release/TODO.md#bundled-agents) (owner, 2026-11-01: "we need corp acct. cant use personal")
  - Builds: the [Google Chat plan](google-chat.md#proposed-methods-for-approval); technical choices decided
  - Depends: a corporate Google account and project ([Q247, Q248](QUESTIONS.md#messaging-agents))

## Runner hardening

Owner-moved from R1 to R1.1 on 2026-11-01, then to R1.2 on 2026-11-06; M.14 and Q276–Q280 keep their IDs.

- ⛔ M.14 Runner hardening — T-18b, **plan only; owner approval required before code**
  - Details: The owner named runtime-file exposure, self-serving bus access and host sharing policy; live `jsonl` work-size checks and a finite CPU default are **added from T-18 review, owner to confirm**. No mechanism or implementation is selected.
  - Depends: the five open policy questions, [Q276–Q280](runner-hardening.md#questions)
  - Home: [proposal](runner-hardening.md#runner-hardening-proposal)

| Boundary | Falsifiable acceptance after approval |
|---|---|
| 1. Runtime files | Put distinct readable markers in this agent's package, an unrelated sibling bundle under `/usr/local`, and an unrelated host directory. An approved representative agent still starts and reads its required runtime/code; unapproved sibling and unrelated markers cannot be read. Writing the runtime or package is refused, while own-work writes succeed. Repeat for per-message, `jsonl` and self-serving forms |
| 2. Self-serving bus access | A self-serving fixture receives and answers one correlated call and performs each approved supporting operation. Attempts at an unapproved operation and access to another account's socket fail without effect. Exercise the owner-approved IP behavior separately, including attempts to bypass any required restriction. An ordinary per-message child gains no bus access from this change |
| 3. Inbox sharing | Test host `share=false` and `share=true` on two readers: the first refuses a conflicting reader, the second admits sharing and handles every sent message once with no loss or duplicate. Exercise runner-issued reads and the owner-selected self-serving scope, then change the setting on reload and assert the approved transition, including in-flight work |
| 4. Work-size checks — **added from T-18 review, owner to confirm** | Keep a `jsonl` process alive, grow its work directory one byte past the approved limit, and assert the approved over-limit response within the approved deadline without a restart/reload trigger. Check work below the limit still succeeds and all processes in the pool count toward the same boundary |
| 5. CPU default — **added from T-18 review, owner to confirm** | Run a busy agent with no CPU override and measure that its whole process group stays within the approved finite default over the approved observation window/tolerance. Check explicit overrides and reload follow the approved policy; a second agent remains able to answer a bounded probe |

The question answers must state observable allowed/denied operations, bounds, transitions and compatibility requirements before implementation acceptance is final. They choose **policy**, not a technical mechanism. Tests use sandboxed fixtures and harmless markers; no real host secret or live account socket is read. Removing each implemented boundary must fail its owning assertion, rather than a setup or build failure. Until approval, no code, runtime experiment or rollout belongs to this item.
