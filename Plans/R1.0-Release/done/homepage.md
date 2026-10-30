# Homepage launch

📌 **TL;DR:** P.5 is live 2026-10-30 from parf.dev `8b0e411`.
The homepage and its Chinese, Japanese and Russian pages are deployed;
flowcharts are fixed in live parf.dev `d65e4af`.

## Result

`#ab-ceo` deployed parf.dev `8b0e411`; `#ab-dir` checked HTTP 200 for
[the homepage](https://parf.dev/agent-bus/),
[Chinese](https://parf.dev/agent-bus/zh/),
[Japanese](https://parf.dev/agent-bus/ja/) and
[Russian](https://parf.dev/agent-bus/ru/).
Deployment and checks were reported by `#ab-lead2` on 2026-10-30.

`#ab-ceo` then deployed parf.dev `d65e4af`: Mermaid flowcharts are
pre-rendered to inline SVG. All four languages return HTTP 200 with inline
SVG and no script, as reported by `#ab-lead2` on 2026-10-30.

## Historical task and acceptance

- P.5 Homepage says what a user gets: the [parf.dev/agent-bus](https://parf.dev/agent-bus/) page leads with what a person can do and how fast (one install command, agents talking in minutes, the bundled agents, MCP for Claude and Codex sessions), in user words without architecture terms, every claim true at HEAD as docs mark built or pending (owner, 2026-10-30). Draft T-8; replacing the live page waits for the owner
  - Acceptance: every claim on the page maps to a built section in `docs/` or is marked pending; every link on the page resolves; the owner approves the text before the live page changes
