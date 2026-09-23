# Interactive runtime isolation

📌 **TL;DR:** Codex and OpenCode pass concurrent native-TUI isolation exercises; Claude's channel prerequisite remains unproven in the fresh fixture profile.

## Scope

Tests only, against the 0.5.48 launchers and installed Codex 0.154.0,
OpenCode 1.18.30 and Claude Code 2.1.274 on this development host.
No live session, daemon, account configuration or provider credential changed.
This extends [endpoint checks](runtime-endpoint-auth.md#checks); it does not
close [H.9.5](../TODO.md#remaining-work) for every shipped runtime or satisfy
the separate fresh-host package gate.

## Checks

[Runtime harness](../../../src/acceptance/runtime-interactive.ts) starts two
launcher-owned native TUIs on PTYs, each in a new home and a working directory
containing spaces. Their bus, credentials, peers and service are disposable.
The actual second OS account is `agent-bus-runner`.

| Evidence | Measured result |
|---|---|
| `tmp/runtime-interactive/codex-final.log` | 23 checks, 0 failures; process exit 0 |
| `tmp/runtime-interactive/opencode-final.log` | 23 checks, 0 failures; process exit 0 |
| Both native keyboard turns | Each real TUI answers its own typed input through the local provider |
| Mid-exchange attack | Both pusher-triggered model requests held at the provider while the other account attempts native endpoint access, control read/write and credential-file reading |
| Native boundaries | Codex denies WebSocket connection; OpenCode denies read/write with 401; both control endpoints deny read/write with 401 |
| Real MCP | Tools declared to the model; allowed service listed, forbidden service hidden, direct forbidden send refused |
| Two addressed exchanges | Service receives one actual MCP request per session with correct sender, topic and tag; each peer receives its session's exact random service answer |
| TUI and sibling checks | Final rendered model text includes the random answer; neither sibling TUI nor sibling model input contains it |

The [model fixture](../../../src/acceptance/runtime-model-fixture.ts) chooses
only the next response/tool call. It never calls the bus or MCP itself. The
native runtime executes those calls and returns their actual results. The
fixture service generates its random answer **after** consuming the request;
the provider learns it only when the pusher delivers it as user input. MCP
`ab_send` returns that answer to the peer; its tool result is only acceptance.
Thus this exercises pusher input and MCP output separately. It proves no
model reasoning, instruction-following quality or external provider behavior.

Every observation has a bounded wait; expiry fails the run. Later reruns have
new evidence directories, not overwritten logs. Model traffic targets loopback;
this harness does not claim network-namespace confinement.

**0.7 rerun, 2026-09-23:** `tmp/rt-opencode3.log` 23 checks, 0 failed on the
upstream OpenCode 1.18.30 build; `tmp/scripts/mutate-opencode.py` caught the
`opencode-native-auth`, `opencode-mcp-tools` and `opencode-pusher` mutants. The
Fedora `opencode-cli-1.18.30-1.fc44` binary failed every prompt and is removed.

## Mutations

`tmp/runtime-interactive/mutate.py` rebuilds copied source into separate launcher
trees and runs the real interactive harness. `mutations.log`: **8/8 caught**,
all at their named failure. These are targeted runtime exercises, not full
smoke runs per mutant.

| Mutation | Named catch |
|---|---|
| Remove Codex native authentication | Second account connects during the exchange |
| Remove OpenCode server password | Second-account refusal assertion fails |
| Remove control bearer check | Mid-exchange control refusal assertion fails |
| Disable Codex MCP configuration | Runtime does not declare the real MCP tools |
| Disable OpenCode MCP configuration | Same declaration assertion fails |
| Drop Codex pusher delivery | Addressed model-turn wait expires |
| Drop OpenCode pusher delivery | Addressed model-turn wait expires |
| Replace random final screen text with a constant | Random-answer TUI assertion expires; harness-assertion mutation, not product code |

## Claude prerequisite

[Claude probe](../../../src/acceptance/claude-channel-probe.ts) uses a fresh
profile, literal dummy API key and local Messages endpoint. It seeds onboarding
and approval of that dummy key only, not feature flags or authentication state.
The native keyboard turn succeeds. The runtime then explicitly reports
`Channels are not currently available` and ignores development-channel
activation. The probe exits **1**, leaving interactive acceptance open.

`tmp/runtime-interactive/claude-channel-final.log` and `result.json` record
one local model request and the channel-unavailable warning. The earlier
`claude-third.log` records the separate attempted bus event: inbox reader
present, zero model requests, no reply. Its exploratory script exited 0 despite
those observations; that is **not** a passed acceptance run. The tracked probe
now fails explicitly on the warning.

The prerequisite is a supported channel-enabled Claude profile/configuration.
The observed warning applies to this fresh fixture-key/local-provider setup;
it does not establish the exact upstream gating cause or universal inability
to use a local provider. [Claude's channel requirements](https://code.claude.com/docs/en/channels)
and [development-channel rules](https://code.claude.com/docs/en/channels-reference)
describe availability separately from MCP tool loading.

History above: the fixture profile cannot pass.

## Claude channel checks

**Passed 2026-09-23.** [Live Claude gate](../../../src/acceptance/claude-channel-live.ts)
copies only a signed-in claude.ai login into a disposable profile and runs the
real model in a launcher-owned TUI (Claude Code 2.1.280, 0.8.0 launchers).

| Evidence | Measured result |
|---|---|
| `tmp/claude-live/run4` | 10 checks, 0 failed: channels available; the peer's bus message reaches the model as an `agent-bus` channel event with nobody typing; the model answers with `ab_reply`, correlated by tag and carrying the peer's random value; `agent-bus-runner` gets 401/401 from the control endpoint and cannot read the session's credential file |
| `tmp/scripts/mutate-claude.py` | Control bearer removed: the second-account refusal fails. Channel flag removed: no message reaches the model, and the reply wait expires |
| Live node, 0.8.0 | The owner's running `#claude/ab-dvp`, `#codex/…` and `#opencode/hi` sessions each answered a canary sent over the bus with `ab_reply`, correlated by tag |

## Harness corrections

- The first Codex run combined typing and Enter in one PTY write; native paste
  handling did not submit it. Its red log remains `concurrent-first.log`.
  The harness now waits for typed text, then sends Enter separately.
- Exact random peer-body comparison existed first; the screen check originally
  used a constant. OpenCode's review prompted the random-answer screen check,
  and the constant-screen mutant proves that new assertion fails independently.
- Final runs additionally check that the runtime declares the MCP tools and
  that the service receives the expected topic, not just tag and body.
- Claude's first two probes stopped at dummy-key confirmation. They are not
  channel-availability evidence; later probes passed that setup step.

## Repository verification

`tmp/runtime-interactive/slow-final.log:937`: **589 passed, 0 failed**,
with vet and race green (lines 10–11), process exit 0 and every file in the
source manifest unchanged. The separate interactive and mutation runs above
are not part of that smoke count. The script and byte-copy
`src/runtime-interactive-smoke.local.sh` share SHA-256
`d873f2bd046422220fb9f6fc0d96734ebd0f291548d6cdabab62a424b9054bd3`.
`tmp/runtime-interactive/source-frozen.json` covers tracked source plus the
three new acceptance files.

H.9.6 recovery, runtime resume/rename permutations, fresh-host installation,
Claude's co-exercise and the broader installed gates remain separate work.
