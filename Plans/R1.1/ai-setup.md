# Web Face AI setup chat

📌 **TL;DR:** Help people install AgentBus and configure agents through a conversation in the Web Face. The conversation is backed by an `ab-*` session and reuses the Markdown setup workflow. This is R1.1 work, not a release claim.

## Web Face chat

Owner, 2026-11-03: T-34 moves from R1 to R1.1; T-33 stays in R1.

- Inspect the owner's mdhouse chat integration before design or implementation.
- Back the ongoing conversation with an `ab-*` session and show questions, progress and results across turns.
- Reuse the [Markdown setup workflow](../../AI-SETUP.md#how-to-start) and agent guides, rather than defining another installation contract.
- Keep the existing plain Web Face style. Session lifecycle, placement and integration follow the mdhouse read.

## The backing session

The owner's words, 2026-11-03 (EST):

| Time | Quote |
|---|---|
| 19:31 | "todo R1.1 - agent-bus-ai-cli - start tmux session with codex/claude/opencode with ab-stuff knowledge; with configured AB skills" / "same session is integrated into agent-bus-web as AI chat - per page or generic" |
| 19:36 | "T34 and new AIC.1 are similar - agent-bus-ai-cli - is that ab-* session (just more details added, that some init content added to it)" |
| 19:38 | "web face - is agent bus bundled web interface for that session." / "that session will be bundled in user-runner. (because it needs user access)" / "most likely it will be started on demand by runner" |
| 19:40 | "and actual command will be "agent-bus cli" (== runner cmd + tmux attach)" |
| 19:41 | "yes, and when web needs it it will act same way; runner run + ab call" |
| 19:42 | "but internally they share same tmux session;" |

- The command is `agent-bus cli` (19:40; the request's working name was `agent-bus-ai-cli`). It asks the runner for the backing `ab-*` session, a tmux session running Codex, Claude Code or OpenCode, then attaches to it with tmux.
- The Web Face reaches the session the same way: it asks the runner for the session, then sends it a bus call instead of attaching. Internally both use the same tmux session, never one for the CLI and another for the Web Face.
- The session starts with initial content: knowledge of the bus and the configured AgentBus skills.
- The Web Face chat talks to that same session, either about the current page or as a generic chat. The Web Face is the bus's bundled web interface for that session.
- The session is bundled in the user-account runner, because it needs the user's access. The runner most likely starts it on demand; the plan confirms this.
- What the initial content holds, and how a page chat passes its page, are for the plan; they are not decided here.

## Acceptance

[T-34](TODO.md#web-face-ai-chat) owns execution and acceptance. No session, provider or publication operation is authorized by this plan.
