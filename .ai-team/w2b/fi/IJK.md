- 🔴 J1 `src/launchers/launcher.ts:452` the pre-allowed `mcp__plugin_agent-bus_agent-bus__*` covers `ab_bigfile_put/get`, which take model-chosen local paths with no prompt
  - Evidence: `launcher.ts:452` passes `--allowedTools mcp__plugin_agent-bus_agent-bus__*`; `mcp/bus.ts:151-167` `bigPut` reads any `input_path` the user can read, `bigGet` renames peer bytes over any `output_path` (no path restriction); tools at `mcp/server.ts:211-214`; no earlier tool in the wildcard touched the filesystem
  - Impact: an injected peer message (bodies and bytes are untrusted) can make a session upload `~/.ssh/*` to a readable record or overwrite `~/.claude/settings.json`, `~/.bashrc` or a hook, with no approval, in every `ab-claude` session
  > 💡👾 List the other tools by name so the four bigfile tools ask for approval, or confine both paths to the cwd or a named directory and refuse symlinks.
- ⚪ K1 `src/clients/go/bus.go:499` a negative `WithWait` or `WithTTL` is silently left out of the send; the other five clients refuse it
  - Evidence: `Send` writes `wait`/`ttl` only `if > 0`; Python `transport.py:22` and TS `transport.ts:26` refuse `< 0`; `clients/README.md` "A send's wait" says only zero means no wait
  - Impact: a Go caller with a computed remaining time gone negative sends with no deadline or lifetime; rare
  > 💡👾 Refuse `< 0` with an error in `Send`; drop only an exact zero.
- ⚪ J2 `src/mcp/README.md:3` says "fifteen tools" and lists no `ab_bigfile_*` tool; the face has 19
  - Evidence: `tools.yaml` has 19 `ab_` entries, `mcp/smoke.ts:80` expects 19; no `ab_bigfile` hit in `src/mcp/README.md` or `docs/user/ai-cli.md:81-94`; same shape as closed J6
  - Impact: a user wiring the face by hand is told the wrong count and never learns the bigfile tools exist
  > 💡👾 Say "nineteen tools", add the `ab_bigfile_put/get/ls/rm` row to both docs, drop the count from the TL;DR.
- ⚪ K2 `src/clients/README.md:5` says the six libraries "behave the same way", but bigfiles exist only in Go, Python and TS
  - Evidence: `grep -ci bigfile` on READMEs: python 8, go 2, rust 0, php 0, java 0; `Plans/R1.0-Release/bigfile.md:4`; `may-send` (Python, TS) and `asks: 'only'` (TS) are also absent from the hub
  - Impact: a Rust, PHP or Java author expects `record.bigfile` and finds nothing
  > 💡👾 Add a one-line per-language support note for bigfile, may-send and asks-only to "What you get".

Checked and sound (I, J, K): web store bigfile card, tag handling (InvalidTag 400), bus-setup page and nav gating, owned-record pagination, hostsText, realm prefill; Go client big.go, Python bigfile size/hash checks; MCP bigGet/bigPut checks, push.ts hold/supervise, launcher sessions.ts and plugin-steps.ts lock takeover, automaticBase; consume wait clamps; `bun test` src/web 206 pass.
