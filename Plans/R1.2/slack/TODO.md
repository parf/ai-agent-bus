# TODO: Slack agent

## Objective

Build the [Slack agent](README.md#slack-agent) as `src/agents/slack/`.

## Process

1. OpenCode (`#opencode/oc-ab`) and Codex (`#codex/ab-dvp`) review this TODO
   before any code.
2. OpenCode implements, task by task, under [CLAUDE.md](../../../CLAUDE.md#verification):
   - stage by path;
   - each mutant seen to fail;
   - bump and changelog;
   - `smoke.sh --slow` green;
   - commit as it goes.
3. Codex reviews each task's code and copies `#claude/ab-dvp-b`.
4. An open choice goes to [questions](../QUESTIONS.md#open-questions) for the owner, never picked silently.
5. A live deploy with real tokens is the owner's step.

## Tasks

| Task | Done when; mutation that must fail |
|---|---|
| S.1 Skeleton and secrets | Registers, serves `help`, reads its tokens from the bus secret; with no `SLACK_BOT_TOKEN` it answers `error: SLACK_BOT_TOKEN is not set`. Mutant: tokens read from the process env pass the secret-only test |
| S.2 Command parsing | Text and JSON forms parse to the same call, and an unknown or malformed call answers `help`. Table test over every method; mutant: drop a JSON alias |
| S.3 post and read | Against a fake Slack HTTP server (`SLACK_API_URL`): post sends the right `chat.postMessage` and answers the permalink; email resolves to a DM; read answers N messages oldest first; `ok:false` becomes an `error:` answer. Mutant: the email path skipped |
| S.3b Directory and message tools | channels, users, user, channel, thread, react, edit, delete, join, whoami against the fake Slack: each sends its one API method with the right arguments and answers the documented fields; `#name` and `@name` resolve through the cached list and refresh once on a miss; pagination (`cursor`) is followed for lists. Mutants: no pagination; no refresh on miss |
| S.4 Subscriptions in KV | add, list, edit and delete round-trip through the agent's KV (real fixture daemon); a bad regex is refused at add; edit changes only the named fields; a caller without `owner`/`maintainer` is refused add/edit/delete and allowed list. Mutant: the role check removed |
| S.5 Forwarding | A fake Socket Mode server feeds events: a matching event reaches the target, with topic/tag/body as specified; a filtered-out one does not; its own post is dropped; a pubsub target fans out; a refused target records `last_error`. Mutants: filter ignored; self-drop removed |
| S.6 Reconnect | The fake socket drops: it reconnects with backoff, and the watchdog refreshes a silent socket (bounded test). Mutant: no reconnect fails a bounded check |
| S.7 Docs and running | README: install, the bus secret, the Slack app scopes (`chat:write`, `channels:history`, `groups:history`, `channels:read`, `groups:read`, `users:read`, `users:read.email`, `reactions:write`, `channels:join`, Socket Mode with `connections:write`), and running it by hand or as a user unit until M.9; `agent.json` loads with the runner's strict loader (`TestTheShippedAgentsLoad`); the agent's tests run in `smoke.sh --slow` as a shard |

