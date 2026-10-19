# TODO: Telegram agent

## Objective

Build the [Telegram agent](README.md#telegram-agent) as `src/agents/telegram/`,
closing [№B.1](../TODO.md#bundled-agents).

## Process

1. OpenCode (`#opencode/oc-ab`) implements, task by task, under [CLAUDE.md](../../../CLAUDE.md#verification):
   - stage by path; never `git add -A`;
   - each mutant compiles and fails an assertion;
   - bump and changelog, the version number asked of Claude B first;
   - `smoke.sh --slow` green in an isolated worktree;
   - commit as it goes.
2. Claude B (`#claude/ab-dvp-b`) answers questions, settles choices for the
   owner's review, and reviews each task; Codex cross-reviews on request.
3. A choice the owner must make goes to the [questions](../QUESTIONS.md#open-questions), never picked silently.
4. A live deploy with a real token is the owner's step.

## Tasks

| Task | Done when; mutation that must fail |
|---|---|
| ✅ TG.1 Skeleton, secret, configuration | Registers, answers `help` and `whoami` against a fake Telegram (`TELEGRAM_API_URL`); the token comes from the bus secret only: distinct sentinels in the process env and the secret, the fake sees only the secret's, and no sentinel is in logs, answers or KV; no secret or no `TELEGRAM_BOT_TOKEN` at start exits 78 naming it. The configuration's fields are proposed in the commit and README. Mutant: the token read from the process env |
| ✅ TG.2 Parsing | Text and JSON forms parse to one call; an optional chat on `post` and `read` follows the README's first-word rule; unknown or malformed answers help. Table test over every method; mutant: drop a JSON alias |
| ✅ TG.3 post, default and aliases | post reaches the fake's `sendMessage` with the resolved chat id and answers chat and message id; no destination goes to the declared default; the same call with no default is refused; an unknown alias is refused with no send. Mutants: the first configured chat as fallback (fails the no-default refusal); an alias miss falling back instead of refusing |
| ✅ TG.4 The other methods | channel, user, react, edit, delete, whoami each send their one API method with the right arguments and answer the documented fields; `ok:false` is an `error:` answer; the 429 rule holds (bounded test); a write is not retried on 5xx. Mutants: no 429 retry; a write retried |
| ✅ TG.5 Receiving and read | A fake `getUpdates` feeds messages: `read` answers the last N of a chat, oldest first; `channels` and `users` list what was seen plus aliases; the offset survives a restart, so a batch is not forwarded twice. Mutants: the offset not stored; read newest first |
| ✅ TG.6 Subscriptions | Slack's S.4, S.5 and S.8 checks, on a real fixture daemon, for Telegram: a match reaches its target and a non-match does not; topic, tag and body as specified; own posts and edits dropped; authority by the destination. Mutants: filter ignored; destination check removed on add |
| ✅ TG.7 Docs and running | `src/agents/telegram/README.md`: BotFather, privacy mode, the secret, the configuration, running under the runner; `agent.json` loads with `TestTheShippedAgentsLoad`; the tests run in `smoke.sh --slow` as a shard; [agents § telegram](../agents.md#telegram) and №B.1 point here as built |
| ✅ TG.8 A corrupt stored cache value | A stored `seen:<chat>`, `chats` or `users` value that is not a JSON array is logged by its key and that key starts empty; transient KV failures keep their retry. Today it exits nonzero and the runner restarts it in a loop (Claude B's review of de98f39). Mutant: the corrupt value thrown again, which must fail a check that the agent starts and serves |
| ⏳ TG.9 Reply routing | [Reply routing](README.md#reply-routing): a session's post carries its short tag and is mapped in KV for 7 days. A Telegram Reply from a listed user reaches the original sender with its topic and tag, as a reply. `#<tag> text` reaches that session. An unlisted Telegram user's reply is ignored and logged, with no bus send. An expired mapping answers "use #tag". The sender is `#telegram/…` with `[<user> (TG)]` in the body; the agent holds no user token. Checks on the fake API and a real fixture daemon: reply and tag routes, counted in the target's inbox. Mutants: the listed-user check removed; the reply routed to the last poster instead of the mapped one; the mapping never expiring |
