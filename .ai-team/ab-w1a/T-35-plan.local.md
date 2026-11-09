# T-35 plan: tab titles for the ab-* launchers that start with a glyph

📌 **TL;DR:** plan only, no code until #ab-ceo approves. The launcher's title becomes
`<glyph> [state] <label>`, which replaces `Claude(ai-team/…)`. The label comes from the start
script, the bus name, or the model through a new `ab_tab_title` tool. Step 1 covers the glyph, the
label, the tool and the tmux window name, and can land on its own. Step 2 adds the ❓/🟥 markers,
driven by Claude HTTP hooks, Codex App Server notifications and OpenCode server events.

Source: Linear PRF-67, read on 2026-11-05, without its claude-watch parts, plus #ab-lead1's scope and
the owner's amendments 1 and 2. Researched read-only against the installed claude 2.1.319,
codex-cli 0.166.0, opencode v2.0.30, kitty 0.47.1 and tmux 3.8.

## What exists today

| Fact | Where |
|---|---|
| Title is `Claude(<session name or last two dirs>)` | `src/launchers/terminal.ts:17-20` |
| The title is set with OSC 0, kitty `set-tab-title` (argv) and Konsole `setTitle`. On restore the kitty tab title is set to `""` | `terminal.ts:44-71` |
| The control strips only C0/C1 (`safe`). Bidi controls are not stripped and there is no cap | `terminal.ts:15` |
| The account comes from the launcher-only flags `-2`, `-3` and `-4`, which suffix `CLAUDE_CONFIG_DIR` | `launcher.ts:75-78, 173-183` |
| A loopback control server with a bearer token. Its address and token reach the TUI and the MCP face in the env | `control.ts`, `launcher.ts:476, 534, 606` |
| `ab_rename` reaches the launcher through that control server | `src/mcp/server.ts:500-503` |
| The launcher holds a Codex App Server client and knows `turn/started` and `turn/completed` | `src/mcp/codex.ts:482-493` |
| The OpenCode client holds no event stream ("volatile by contract") | `src/mcp/opencode.ts:12, 119` |
| The launcher writes no Claude settings. Team windows already pass `--settings '{"fastMode":true}'` | `.ai-team/ab-dvp-v2-config.json` |
| `start-team` opens kitty tabs with `--tab-title CEO`, and the launcher then overwrites that title and clears it on exit | `.ai-team/start-agents.sh:30, 43`, `terminal.ts:51, 63` |
| No tmux handling. Team windows run under tmux from #tmux-sessions, and the kitty tab shows the tmux session | `.ai-team/start-agents.sh:43` |

## Step 1: glyph, label, `ab_tab_title`, tmux name

### Title form

`<glyph> <label>`, and in step 2 `<glyph> <state> <label>`. The order puts the part that tells tabs
apart before any truncation. No runtime word appears.

| Runtime | Glyph |
|---|---|
| Claude account 1 | ✴️ |
| Claude `-2` | 🔶 |
| Claude `-3` | 🌟 |
| Claude `-4` | owner item O1 |
| Codex | ❇️ |
| OpenCode, OpenCode1 | 👾 |

Tab colours stay as they are.

**(e) Account with no new config.** The `-N` flag the launcher already parses decides it. Without the
flag, the launcher reads a `CLAUDE_CONFIG_DIR` basename of the form `.claude<N>`, the shape the flag
itself produces. Anything else is account 1.

### Label

Precedence:

1. the label set through `ab_tab_title`, for the life of this launcher process;
2. `AB_TAB_TITLE`, used verbatim after cleaning;
3. the distinguishing part of the bus name: `#ab-w1b` gives `ab-w1b`, and `#claude/home@node` gives
   `home`. Remove `#`, then the `<template>/` prefix, then `@realm`;
4. with no bus, the session name, else the last directory component.

- **One mechanism: the `AB_TAB_TITLE` env, no flag.** Both start paths already set env per window:
  #tmux-sessions config `env` and kitty `launch --env`. A flag would duplicate it (owner item O2).
- **Start-script flow.** `ab-dvp-v2-config.json` gains `"AB_TAB_TITLE": "DIR"` per window (local file,
  not tracked). `start-team` CEO tab: `--env AB_TAB_TITLE=CEO`. mdhouse `start-agents.sh` does the
  same; that file is outside this repo, so I list it for the owner.
- **On exit the kitty title is put back, not cleared.** The launcher reads its tab's title at start
  (`kitty @ ls --match window_id:N`, already argv) and restores it on exit, so `CEO` survives the
  session. If the read fails, it falls back to today's `""`.
- **Strip from the child env.** `AB_TAB_TITLE` is removed from the runtime env, so a nested launcher
  never inherits the outer label.

### `ab_tab_title [text]`, the MCP tool

- **Path.** New control endpoint `POST /label {label?}`, behind the existing bearer token, the same
  way `ab_rename` reaches `/rename`.
- **Answer.** `{current, recommended}`. With `text` it sets the label and answers the same.
  `recommended` is `AB_TAB_TITLE`, else the bus-name part.
- **Same for all runtimes.** It lives in the MCP face, which all three use.
- **Without a launcher** (no `AGENT_BUS_CONTROL_ADDR`) it refuses: "needs an ab-* launcher".
- **Authority scope.** The endpoint changes only the in-memory terminal label. It never touches the
  bus name, the identity, the session title or the binding, and the docs say so. Two readers review it
  anyway (launcher control).
- **Instructions.** The face's instructions tell the model its current label and the recommended one,
  and invite a short update when its task changes (`W1B T-35`). They reach the face as
  `AGENT_BUS_TAB_LABEL` and `AGENT_BUS_TAB_RECOMMENDED` in `faceEnv`.
- **Restarts.** The label survives a TUI restart (`--restart` keeps the launcher process), but not a
  launcher restart: no new state file, and the recommended label returns. Owner item O3 asks whether
  that is right.

### Cleaning a label (owner amendment 2, the security part)

One function, `cleanLabel`, in `terminal.ts`. Every label source goes through it: the env, the bus
name, the session name and the tool.

- remove ESC, BEL, all C0 (`\x00-\x1f`), DEL and C1 (`\x80-\x9f`);
- remove the bidi controls U+200E, U+200F, U+202A–U+202E, U+2066–U+2069 and U+061C;
- collapse whitespace and trim;
- cap at 24 graphemes (`Intl.Segmenter`, granularity `grapheme`), never by bytes;
- an empty result falls back to the next source; the tool refuses it with 400.

tmux gets the label only as an argv value (`spawnSync("tmux", ["rename-window", "-t", pane, name])`),
never through a shell. Risk R3 covers tmux expanding `#[…]` inside a name.

### tmux window name

- **When.** Under `$TMUX` and `$TMUX_PANE` with an active terminal, the window is renamed
  `<WIN> <glyph> [state]`, where `<WIN>` is its name before the launcher started.
- **What is saved.** The launcher records two window user options. `@ab_title_orig` holds the
  original name and `@ab_title_owner` holds `<pid>:<pane>`. It also records whether
  `automatic-rename` was on.
- **Exit.** It renames the window back, puts `automatic-rename` back, and unsets both options.
- **(d) Crash or kill -9.** No trap runs, so the decorated name stays. The next launcher in that window
  sees an `@ab_title_owner` whose pid is dead (or is not a launcher, per `/proc/<pid>/cmdline`). It
  first restores `@ab_title_orig`, then takes ownership. The name is cleaned up on the next start,
  which I accept: a window nobody relaunches keeps a stale name until it is renamed by hand.
- **(h) Nested or shared windows.** The outer launcher sets `AGENT_BUS_TITLE_OWNER=<pid>` in the
  runtime env. An inner launcher that sees a live owner leaves the kitty tab, the OSC title and the
  tmux window alone. For tmux, a live `@ab_title_owner` on another pane also leaves the window name
  alone: two launchers in split panes do not fight over it, and the second one sets only its own
  pane's title.

### (5) Glyphs on a real kitty tab

- The title travels through OSC 0 and kitty `set-tab-title` as argv. Glyphs are above U+009F, so
  `cleanLabel` keeps them, including the U+FE0F variation selector.
- **Automatic check (unit).** Assert the exact bytes of the OSC sequence and of the kitty argv for
  `✴️ CEO`.
- **Real tab, once per step, by the worker.** The ab-dvp kitty has a socket
  (`unix:$XDG_RUNTIME_DIR/ab-dvp-kitty-$UID`); `kitty @` from a tool shell needs `--to`, per PRF-67.
  Run `kitty @ --to <socket> launch --type=tab --env AB_TAB_TITLE=T35-probe <launcher in plain mode>`,
  then check that `kitty @ --to <socket> ls | jq '.. | .title?'` holds `✴️ T35-probe` byte for byte.
  Close the tab and check that the original title came back.
- **Real tab, by the owner.** A glance at the rendered glyph, listed as an owner review item.

### Step 1 files

| File | Change |
|---|---|
| `src/launchers/terminal.ts` | glyph table, `cleanLabel`, title builder, kitty title save/restore, tmux name, owner guard |
| `src/launchers/launcher.ts` | account number, label precedence, `AB_TAB_TITLE` strip, owner env, `/label` wiring |
| `src/launchers/control.ts` | `POST /label` |
| `src/mcp/server.ts`, its descriptions, `instructions.yaml` | `ab_tab_title`, label lines in the instructions |
| `src/launchers/terminal.test.ts`, an MCP face test | tests below |
| `docs/06-launchers.md` | title rows at line 79 ("Title", "Exit", new "Label", "tmux", "Nesting") |
| the MCP tool reference, where `ab_rename` is documented | `ab_tab_title` row |
| the owning plan's TODO/DONE | item home: lead item L1 |

### Step 1 tests (f, g), no Codex or OpenCode needed

| Test | Check, and how it is proven to fail |
|---|---|
| title builder | glyph per runtime and account, then state, then label; no runtime word |
| label precedence | tool > env > bus name > directory; bus name `#claude/home@node` gives `home` |
| injection | ESC, BEL, `\x9b`, OSC terminator, U+202E and `#[fg=red]` labels reach OSC, kitty argv and tmux argv clean. **Mutant: drop the stripping, and the test fails on an assertion** |
| grapheme cap | 30 emoji are cut to 24 graphemes, never split mid-sequence. Mutant: cap by `.length` |
| tmux | private server (`tmux -S <mktemp>/sock`, never the user's). Rename and restore; restore after a simulated kill -9 (stale owner pid); nested launcher leaves the name. **Mutant: remove the owner guard, and the nested test fails** |
| kitty restore | fake `kitty` on PATH records argv (the `terminal.test.ts` pattern): restore puts back the saved title, not `""`. Mutant: restore `""` |
| `/label` | 401 without the token; it never changes the binding or the bus name (both asserted unchanged after a set) |
| `ab_tab_title` | returns current and recommended; refuses without a launcher |

**Estimate for step 1:** about 60 min of agent time from approval. Glyph, label and kitty restore
take 15 min; `ab_tab_title`, the endpoint and the instructions 15 min; cleaning and the injection
tests 10 min; tmux with its fixture 15 min; docs 5 min. If approved at 21:00, step 1 is ready at
about 22:00.

## Step 2: the ❓/🟥 state markers

| State | Set | Cleared |
|---|---|---|
| ❓ | a question for the operator: permission, approval or elicitation | new input, a bus message handled, a new turn |
| 🟥 | the turn ended and the last answer ends with `🟥🟥🟥` | the same |
| (none) | no busy marker | |

All three runtimes feed one state machine in `terminal.ts`: `ask`, `stop`, `clear` → title.

### (c) Event APIs in the installed versions

| Runtime | ❓ | 🟥 | Clear | Proof |
|---|---|---|---|---|
| Claude 2.1.319 | `Notification` (matchers `permission_prompt`, `elicitation_dialog`), `PermissionRequest`, `Elicitation` | `Stop`, using its `last_assistant_message` | `UserPromptSubmit`, `PreToolUse` | strings in `~/.local/share/claude/versions/2.1.319`: `allowedHttpHookUrls`, `allowedEnvVars` ("Authorization": "Bearer $MY_TOKEN"), `last_assistant_message`, `transcript_path`, `notification_type`, `permission_prompt`, `elicitation_dialog`. `--settings <file-or-json>` is in `claude --help` |
| Codex 0.166.0 | `thread/status/changed`, whose `activeFlags` contain `waitingOnApproval` or `waitingOnUserInput` | `turn/completed`, plus the last `item/completed` agentMessage text | `turn/started`, empty `activeFlags` | `codex app-server generate-json-schema`: ServerNotification lists `thread/status/changed`, `turn/completed`, `item/completed`; `ThreadActiveFlag = [waitingOnApproval, waitingOnUserInput]` |
| OpenCode 2.0.30 | `permission.asked` | `session.idle` or `session.execution.succeeded`, then the last assistant message | `permission.replied`, `permission.rejected`, `session.execution.started` | binary `~/.opencode/bin/opencode`: `zt({type:"permission.asked"…})` at offset 133102929; `session.idle` at 133423214; `session.execution.{started,succeeded,failed,interrupted}`; `event.subscribe` → `GET /api/event` "native events … across all server locations. Volatile" |

- **Codex** observes only notifications. It never answers or intercepts an approval request; that
  stays the TUI's job.
- **OpenCode.** It is still unproven that `GET /api/event` carries `permission.asked` and
  `session.idle`: the V2Event union is built at runtime. Step 2 starts with a 5-minute live probe on a
  launcher-owned `opencode serve`. If the events are absent, OpenCode gets the glyph and label only,
  named as a gap in the docs, with no polling. The stream is volatile, so a reconnect starts with a
  clear state, never a guess.

### Claude hooks (a, b)

- **HTTP hooks, not command hooks.** `type: "http"` POSTs the payload to
  `${AGENT_BUS_CONTROL_ADDR}/state` with the header `Authorization: Bearer $AGENT_BUS_CONTROL_TOKEN`
  (`allowedEnvVars`). No process is spawned on each `PreToolUse`, so there is nothing to slow the
  agent.
- **(a) Fail safe.**
  - `timeout: 1` on every hook. Claude treats a timeout or a non-2xx answer as a non-blocking hook
    error.
  - `/state` always answers `204` with an empty body, so nothing can be read as a hook decision. A bad
    token gets 401, which is still non-blocking.
  - The handler only updates in-memory state. It never awaits I/O longer than the tail read.
- **(b) 🟥 detection.** `Stop` carries `last_assistant_message`; checking that it ends with
  `🟥🟥🟥` needs no file read. If the field is absent, the launcher reads only the last 64 KiB of
  `transcript_path` (a positioned read), and only when the path is under this account's
  `projects/` directory.
- **Settings merge.** The launcher builds one `--settings` JSON. A user `--settings` (inline JSON or a
  file path) is parsed and merged in, the user's keys winning except `hooks`, which are concatenated.
  This is needed because team windows already pass `--settings '{"fastMode":true}'`. An unparsable
  user value fails loudly at start.
  - 🚫 Hooks in the agent-bus plugin: they would fire in sessions without a launcher and need a plugin
    release.
- **Bus message handled.** A push the launcher delivers also sends `clear`.

### Step 2 files

`terminal.ts` (state), `control.ts` (`/state`), `launcher.ts` (settings merge, Codex/OpenCode wiring),
`src/mcp/codex.ts` (an `onState` callback in `#notification`, plus the last agentMessage text),
`src/mcp/opencode.ts` (an optional event reader, only if the probe passes), tests and fixtures,
`docs/06-launchers.md` (state rows and any gap).

### Step 2 tests (f, g): recorded fixtures only

| Test | Fixture | Mutant |
|---|---|---|
| state machine | ask → clear, stop → clear, a stop without the marker stays clear | treat any `Stop` as 🟥 |
| Claude `/state` | payloads recorded once from 2.1.319 with a dump-only hook, secrets removed | ignore `last_assistant_message` |
| tail read | 10 MB transcript with the marker in the last line; reads at most 64 KiB (counted through an injected reader) | read the whole file |
| hook fail-safety | bad token, garbage body, slow handler: always 204 or 401, empty body, within the timeout; the generated settings have `timeout ≤ 1` on every hook | answer 200 with JSON |
| settings merge | user inline JSON and file path are both kept; unparsable input fails | drop the user's keys |
| Codex | recorded `thread/status/changed`, `turn/completed` and `item/completed` NDJSON | drop the `activeFlags` check |
| OpenCode (if probed) | recorded SSE lines | drop `permission.replied` clearing |

**Estimate for step 2:** about 60 min of agent time from step 1's landing: the probe and fixture
recording 15 min, Claude hooks and the merge 20 min, Codex and OpenCode 15 min, docs 10 min. If step 1
lands at 22:30, step 2 is ready at about 23:30.

## Risks

| # | Risk | Handling |
|---|---|---|
| R1 | Claude ignores a second `--settings`, so the hooks or `fastMode` would be lost | the launcher merges into one; a test runs on the merge |
| R2 | An HTTP hook refuses when proxy, TLS or resolver settings differ (string seen in the binary), for example with `HTTPS_PROXY` set | add `NO_PROXY=127.0.0.1` to the TUI env; the probe in step 2 confirms |
| R3 | tmux expands `#[…]` or `#{…}` inside a window name | a fixture reads back `#W` for `#[fg=red]x`; if it is expanded, double `#` |
| R4 | A pid is reused after kill -9, so a stale owner looks alive | check `/proc/<pid>/cmdline` for the launcher too |
| R5 | OpenCode `/api/event` lacks the events | gap: glyph and label only, no polling |
| R6 | Konsole renders the variation-selector glyphs differently | display only; not in acceptance |
| R7 | A model sets a confusing label (say, another agent's name) | the label is the terminal only, never authority; it is capped and cleaned |

## Questions

Owner items (in the [Options and threads](../../Plans/rules/MARKUP.md#options-and-threads) form):

- ❓ **O1 — Glyph for Claude account 4 (`-4`).** The launcher supports four accounts; PRF-67 names three glyphs.
  - ( ) a new glyph, 🔷 (blue diamond, after 🔶) 🌟
  - ( ) the account-1 glyph ✴️ ⭐
  - ( ) no glyph, label only
- ❓ **O2 — How a start script names a tab.**
  - ( ) the `AB_TAB_TITLE` env only 🌟
  - ( ) env plus a `--tab-title` launcher flag ⭐
- ❓ **O3 — A label the model sets, after a launcher restart.**
  - ( ) dropped: the recommended label returns, and the model sets it again 🌟
  - ( ) kept in the session binding ⭐

Lead item:

- **L1 — Plan home for T-35's TODO/DONE.** Which plan file owns T-35 (R1 TODO or R1.1)?

## Split and order

1. Step 1 (one branch, 4–5 commits: glyph and label, kitty restore, `ab_tab_title`, tmux, docs). It
   can land alone.
2. Step 2 (a second branch after step 1 lands: state machine, Claude hooks, Codex, OpenCode or its
   gap, docs).
3. Two readers for each step.
