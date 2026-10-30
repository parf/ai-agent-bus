# The bus on the bus

📌 **TL;DR:** Landed as `bfeb5f58` (0.9.224): the settled read-only help/status/version/ls set is complete; management methods remain future scope.
This records the acceptance on main on 2026-10-30; it is completion evidence.

## Acceptance map

| Acceptance | Test on main |
|---|---|
| The built-in bus record answers help and the RT23 status ask through ordinary messages, including the AI face route | `src/internal/api/daemon_agent_test.go:TestDaemonAgentReadOnlyMethodsAndAsks` calls the production daemon answer for every initial method and ask; `TestDaemonAgentReplyToAndLiteralAskBoundary` pins routing/literal ask handling. The MCP transport is exercised separately by `src/mcp/launcher-runtime-fixture.ts` (`ab_send`) through `src/launchers/rename-smoke.ts`; this is component coverage, not a new live AI-session run. |
| Text stays default; JSON is direct API status/identity/list/lookup data without a fence; empty list is an array | `src/internal/api/daemon_agent_json_test.go:TestBusStatusJSONIsTheDaemonsStatusForTheCaller`, `TestBusVersionJSONIsTheNodeIdentity`, `TestBusLsJSONKeepsTheCallersVisibility`, `TestBusLsJSONWithoutOptionsListsOnlyAgentsWithReaders` parse the result and compare with the caller-authorized API. |
| No new powers: caller-scoped selection/lookup, unsupported methods refused, help stays text and describes only supported JSON options | `daemon_agent_test.go:TestDaemonAgentListsOnlyTheOriginalCallersRecords`, `TestDaemonAgentReadOnlyMethodsAndAsks`; `daemon_agent_json_test.go:TestBusJSONOptionIsOnlyOnTheReadMethods`. The final record keeps `body_formats: ["text"]`; JSON is a method option, correcting the old task state sentence archived below. |
| General totals are node snapshots, privileged Owner-inactive details remain gated for non-Owner callers and active Administrators | `TestBusStatusJSONIsTheDaemonsStatusForTheCaller`, `TestBusStatusJSONForAnAdministratorWhoIsNotTheOwner`, `TestBusStatusJSONCountsKindsReadersAndCurrentVersusCumulative` in `daemon_agent_json_test.go` distinguish hidden records, caller visibility and privileged fields. |
| All record kinds and inactive state; filtered waits versus unique agents, busy handlers; current queue versus cumulative dropped/expired; status remains ok | `TestBusStatusJSONCountsKindsReadersAndCurrentVersusCumulative` creates service/resource/group/MCP-tagged and inactive records, filtered polls and a busy handler, forces nonzero counters, drains the queue and checks text/status asks before and after. |
| Owning format/selection/count/disclosure mutants | The landed RT24 commits record the assertion-red format/Owner-caller/selection/empty-list mutations; the subsequent count-zeroing and degraded-on-backlog assertions are in `TestBusStatusJSONCountsKindsReadersAndCurrentVersusCumulative`. No new mutant run is claimed for this documentation edit. |

## Verification

Each named check and its assertions were inspected on main. This close-out changes no code and adds no test result or mutant claim; the owning implementation evidence records its runs. Changed links and whitespace are checked for this edit.

## Archived task detail

The following is the pre-close-out task text, including its historical pending statements; it is not current status.

- ⏳ RT24 The bus on the bus: the daemon answers as an agent of its own, so a person manages it from an AI CLI with ordinary bus messages (owner, 2026-10-26: "bus itself should be on bus; provide list of methods: help, status, …; so I can manage it from my AI CLIs")
  - Answers the [RT23](../TODO.md#bundled-agents) asks like any agent: `help` with its capabilities line, `status` (`ok: 82 records, 13 readers`, `degraded: …`), `help-json`
  - Methods map one-to-one onto existing daemon calls, never new powers: for example `status`, `version`, `ls`, `manage`, `debug-log on|off`, `realm` (RT22). Each runs with the caller's own authority, checked at the door as today; a method the caller may not use is refused, and help leaves it out
  - Settled (owner, 2026-10-26: "name it #bus; start with help, status, version, ls"): the record is `#bus@<realm>`; the first set is `help`, `status`, `version` and `ls`, all read-only. Management methods come later, one by one
  - Settled by `#ab-dir`, 2026-10-30: text remains the default; `status --json`, `version --json` and `ls … --json` return the existing API result without a new schema, preamble or Markdown fence. `help` stays text; `help-json` describes the options; ask `status` stays the RT23 line. Version JSON is `/identity`; status JSON is the full caller-authorized `/status` result; listing/lookup retain their current selection, visibility and fields, with `[]` for an empty list.
  - Counts stay the existing `StatusFor` node snapshot: records are active/live records of all kinds (not the caller's visible records); readers are outstanding polls including filtered waits, not unique agents or healthy workers. A busy handler need not be waiting. Uptime is this process's; queued is current depth, dropped/expired are cumulative since start. Owner-inactive detail remains absent except for the daemon Owner/active Administrators, never fabricated as zero for others.
  - `ok` means that this authenticated status request was served. No `degraded` branch until an existing observable current fault is named; zero readers, backlog or historical counters alone are not a new health score.
  - Remaining acceptance: preserve text/status asks; JSON parses directly and empty listing is `[]`; callers with different listing visibility see the same general node totals, with privileged fields still gated; fixtures distinguish all record kinds/inactive state, filtered polls/unique agents, busy handlers and current/cumulative counters. Owning mutants for format wrapping, broadened listings, incorrect counts and privileged disclosure must fail on assertions. API adapter/tests and bus guide only; no core change.
  - State 2026-10-30, branch `ab-w2b/RT24`: `status --json`, `version --json` and `ls [name] … --json` answer with the daemon's own `/status`, `/identity`, `/ls` and `/lookup` results (shared helpers, no new shape); `help` and extra arguments are refused; `help-json` declares the option and `json` as a body format. A name given to `ls --json` answers that record as `/lookup` does, a list as an array, `[]` when empty. Checks: `TestBus*JSON*` in `internal/api/daemon_agent_json_test.go`; mutants in the branch's commit.
  - Acceptance: from an AI CLI session, `ab_send` with ask `help` to the bus record lists its methods; `status` answers in the RT23 form; a method beyond the caller's authority is refused exactly as the CLI refuses it
