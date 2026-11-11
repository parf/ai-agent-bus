# T-35 launcher tab titles and the CLI reply body, completed

📌 **TL;DR:** T-35 shipped in 1.0.3 and 1.0.8; `agent-bus call --body` in 1.0.14. Current homes: [launchers](../../../docs/06-launchers.md), the CLI help.

Moved from the R1.1 TODO on 2026-11-11: built and shipped in 1.0.x, so R1 work. History, not a current contract.

## Launcher tab titles

- ✅ T-35 Glyph-led tab titles in the `ab-*` launchers, from Linear PRF-67 (owner, 2026-11-05: "good, - go").
  - Step 1: an agent glyph by runtime and account (✴️, 🔶, 🌟, 🔷 Claude accounts 1–4; ❇️ Codex; 👾 OpenCode), then the label; `AB_TAB_TITLE` sets the label, else the bus name's distinguishing part; an `ab_tab_title` MCP tool lets the model read the recommendation and set its own label; inside tmux the window is renamed too.
  - Step 2: a state marker after the glyph: ❓ the agent waits on a question or approval; 🟥 its last answer ends with the stop marker; cleared on new input or a new turn.
  - Home: [launchers](../../../docs/06-launchers.md).
  - Acceptance: labels are sanitised and capped; a model-set label never changes the bus name, identity or session title and resets to the recommendation on launcher restart; the start script's title and the tmux window name are restored on exit, nested launchers leave the outer name alone; Codex and OpenCode events are decoded from recorded fixtures; a runtime without a confirmed event source shows glyph and label only.
  - Done: step 1 `4fe73afc` (1.0.3), step 2 `6c6170c2` (1.0.8); readers W1B, QA1 (step 1) and W1B, W1C (step 2). Known limits: OpenCode shows ❓ only (no turn-end event without a model turn); a reused pid whose command line names a launcher keeps a dead owner's tmux claim; a multi-window kitty tab restores the title this window saved.

## CLI reply body


- ✅ `agent-bus call --body` prints only the reply body, without the JSON envelope (`e43309f1`, 1.0.14).
  - Acceptance: `agent-bus call "#time-simple@$REALM" epoch --body` produces the same output as `agent-bus call "#time-simple@$REALM" epoch | jq -r .body`.
