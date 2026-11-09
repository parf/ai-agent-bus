# T-32 fact sheet for the "3 ways to run agents" plan (W1B, 2026-11-02 13:05; main 863c7a0d + T-31c)

Facts only; nothing here edits a doc, Plans/subsystems.md or src/smoke-map.json.

## 1. Owning-doc anchors (all verified to exist)

**CLI agent (`agent-bus run`; there is no `agent-bus serve` verb in docs)**
- docs/user/dedicated-runner.md: #the-one-liner (`agent-bus run '#name' script`), #how-your-script-hears-the-message (args | json), #what-the-caller-gets-back, #more-than-one-at-a-time, #watching-and-stopping-it, #common-trip-ups, #sandboxing, #who-it-runs-as. Asks (`help`, `status`, `mcp`… reach the script as AGENT_BUS_ASK with an empty body) are in its collapsed "Asks" details at line ~87.
- docs/06-dedicated-runner.md (owner of the rules): #script-agents (table: args/json forms, one command, success/silent/nonzero/deadline/receipt/stop rows), #keeping-a-supervised-registration, #stopping-it-and-reading-what-it-said, #what-the-child-is-told (env: AGENT_BUS_* ; the token is removed from the script's env), #who-it-runs-as, #sandboxing.
- docs/dedicated-runner/agent-json.md#fields (the `agent-bus run -` JSON declaration).
- docs/user/cli.md (register/send/ask); docs/constitution.md#agent-self-description (help, status, asks).

**Runner (autostart, Configuration/Secret, confinement)**
- docs/runner/README.md#one-way-to-start, #how-it-works, #choose-your-starting-point.
- docs/runner/getting-started.md: worked example `hello-guide` (sudo install of a 3-line sh script + agent.json, reload, call). It is the runner's EXISTING worked example; a runner page should extend/replace it, not copy it.
- docs/runner/agents.md#package-a-script, #give-one-package-several-configurations, #message-context-and-roles.
- docs/runner/configuration.md: #where-settings-belong (Configuration vs Secret), #record-configuration, #agent-directories, #package-options, #package-help (native help, observed status), #agents-that-serve-themselves, #inventory (enable = autostart), #limits (memory, cpu, processes, files, io_weight, disk, with defaults), #environment-files, #apply-and-check.
- docs/runner/security.md: #isolation-today (#network-reach, #visible-files-and-writes, #processes-and-limits, #with-the-managed-sandbox-off, #the-separate-agent-bus-run-sandbox), #what-the-code-is-told-and-what-it-may-trust, #secrets, #who-may-call-an-agent.
- docs/runner/operations.md#deploying-an-agent, #installing-changing-and-removing-an-agent, #not-configured, #reading-the-logs, #stopping-and-restarting.

**Clients / protocol**
- src/clients/README.md#what-you-get, #try-this-first, #checks, #a-sends-wait; per-language READMEs (python/typescript/go/rust/java/php) each with "Try this first" and (since T-31b/c) executed event examples.
- docs/09-daemon-api.md#the-client-contract (#send-and-consume, #key-value, #locks, #bigfiles, #status-and-identity, #configuration), #checking-permission-to-send, #client-capability-matrix, #stability-and-compatibility.
- docs/04-messaging.md#request-and-reply, #reply-routing, #receipts, #one-reader-per-inbox; docs/02-access.md#getting-a-token.

**Events/state used by an "advanced" example**: docs/03-records-kv.md#rules, #suggested-use, #bigfiles; docs/01-identity-and-authority.md#shared-locks; docs/agents/bus.md#agent-logs (own-log write/read, landed with T-29), docs/agents/subscriptions.md.

## 2. What already runs, and under which check

| Thing | Runs? | Where |
|---|---|---|
| src/clients/examples (codestats PHP/Java, go, python, ts, rust: workers, demo.sh) | yes, all together on a throwaway daemon | `src/clients/examples/run.sh` (daemon via php/tests/daemon.sh); check `clients/check.sh examples` = examples/tests/entrypoints.py + go vet/test in examples/go + run.sh; smoke shard `client_examples` (normal only; no fast entry) |
| Event-log examples in all six clients (T-31a/b/c) | yes, output asserted | each client's own conformance check, shards client_go/python/rust/typescript/php/java, against the Go fixture |
| docs/runner/getting-started.md `hello-guide`, docs/user/dedicated-runner.md hello script, `agent-bus run --help` blocks | NO check runs them; hand-written prose (one block is labelled "ran today") |
| A time-like service | none exists in the repo. Closest: the `hello` script (prints "hello, <arg>"), bundled `#debug` hello method, src/agents/standalone/server-info |
| Confined script agent through `agent-bus run` / runner | yes, internals only: smoke shards `script_confined` (smoke.sh:1924), `script_inbox`, `script_done`, `supervisor`, and `core_3` (tags FHL; paths runner, sandbox, cgroup, script, debug agent); they start `agent-bus-runner --agents … --host …` themselves (smoke.sh ~1400/1411/1580) |
| Throwaway-bus helpers | `src/clients/php/tests/daemon.sh` and `java/tests/daemon.sh` (real agent-busd on own socket/port/db under tmp/, needs built `src/agent-busd`, `src/agent-bus` or CLIENT_BIN; gives AB_TEST_ADDR/OWNER/OWNER_TOKEN); `src/clients/fixture` (Go, in-process core + #bus reader, prints addr+tokens; NO runner). There is no ready helper that starts a runner on a throwaway bus outside smoke.sh. |

## 3. Doc-ownership rule for the new pages (docs/00-overview.md#document-ownership)

- Rule: each value has one owning section; other pages link. Table rows that matter: Dedicated runner (06-dedicated-runner.md, `agent-bus run`), Runner (runner/README.md: packaging, configuration, operations, security), **User guides (docs/user/README.md: "What to type: the command-line tools, running an agent, the daemon, and agent sessions; the *why* stays in the topics above")**, Create an agent (agents/create-agent-prompt.md, an AI-prompt aid), Bundled agent guides (agents/README.md: ready-made helpers, choosing/installing/using them; NOT authoring), Glossary, Decisions. docs/agents/authoring.md is the help-writing checklist (under docs/agents but not a table row).
- Consequence for placement: worked "what to type" pages fit **docs/user/** (listed in docs/user/README.md and in the docs/00 "User guides" row), not docs/agents/ (bundled helpers) and not docs/runner/ (the Runner row owns packaging/config/ops/security, and getting-started already holds its worked example). A runner page under docs/user/ would link runner/* for every value; if instead it extends runner/getting-started.md, no new file is needed for the runner way (say which). The client way's examples live in src/clients/*/README.md + src/clients/examples; the user page links them.

## 4. Subsystem letters, smoke map, registration

- Letters (Plans/subsystems.md, first match wins): **H** = src/cmd/, src/internal/ (daemon, CLI `agent-bus` is src/cmd/agent-bus); **F** = src/cmd/agent-bus-runner/, src/internal/sandbox|cgroup|script/ (⚠️ two reviewers); **K** = src/clients/; **L** = src/agents/; **N** = docs/, README.md, WRITING.md, CHANGELOG, LICENSE (docs); **O** = Plans/; **M** = src/smoke*, src/acceptance, src/ (catch-all).
- reverse deps (smoke-map.json): F→HL, H→FIJKL, K→JL, N→(none), O→(none).
- Selection (src/smoke-select.py): path→letter; a check runs if one of its tags is promoted AND (the row has no `paths`, or the tag is "broad", or a changed path starts with one of the row's `paths`). Any `src/smoke*` change, go.mod/lock or unmapped path promotes everything. **No existing check carries tag N**, so a docs-only change selects nothing ("no executable checks for changed paths: all fast"). To make guide docs select the shard, the new check needs tags that include N plus `paths` for the guide pages and example dir (as agent_eventlog does with "L" + paths); F, H and K are covered by listing them in `tags` (without `paths`, or with the runner/clients paths).
- How an existing shard is registered (client_examples as the model): (1) name in the SHARDS array, src/smoke.sh lines ~52-57; (2) a function `client_examples() { client_check examples; }` at smoke.sh:4075 (body gated by `normal`; the level is set by the selector); (3) `checks.client_examples = {"tags":"GK"}` in src/smoke-map.json (`paths` optional); (4) optional `fast.<name>` = explicit argv allowlist run at fast level (e.g. client_go's `go test -run ^TestOptionsNeverPrintTheToken$`); a fast candidate over 15 s fails. smoke_select_test.py / smoke_fast_test.py may pin the map shape. smoke.sh and smoke-map.json are shared files: propose, never edit (lead does).
- Existing normal-level cost reference: `check.sh examples` runs inside `client_check` with `timeout 180`.
