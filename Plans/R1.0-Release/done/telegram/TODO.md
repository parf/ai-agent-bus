# TODO: Telegram agent

History: completed TG.1–TG.10 checklist. Use the [current Telegram guide](../../../../docs/agents/telegram.md) for behavior.

## Objective

Build the [Telegram agent](README.md#telegram-agent) as `src/agents/telegram/`,
closing [№B.1](../../TODO.md#bundled-agents).

## Process

1. OpenCode (`#opencode/oc-ab`) implements, task by task, under [CLAUDE.md](../../../../CLAUDE.md#verification):
   - stage by path; never `git add -A`;
   - each mutant compiles and fails an assertion;
   - bump and changelog, the version number asked of Claude B first;
   - `smoke.sh` green in an isolated worktree;
   - commit as it goes.
2. Claude B (`#claude/ab-dvp-b`) answers questions, settles choices for the
   owner's review, and reviews each task; Codex cross-reviews on request.
3. A choice the owner must make goes to the [questions](../../QUESTIONS.md#open-questions), never picked silently.
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
| ✅ TG.7 Docs and running | `src/agents/telegram/README.md`: BotFather, privacy mode, the secret, the configuration, running under the runner; `agent.json` loads with `TestTheShippedAgentsLoad`; the tests run in `smoke.sh --slow` as a shard; [agents § telegram](../../agents.md#telegram) and №B.1 point here as built |
| ✅ TG.8 A corrupt stored cache value | A stored `seen:<chat>`, `chats` or `users` value that is not a JSON array is logged by its key and that key starts empty; transient KV failures keep their retry. Today it exits nonzero and the runner restarts it in a loop (Claude B's review of de98f39). Mutant: the corrupt value thrown again, which must fail a check that the agent starts and serves |
| ✅ TG.9 Reply routing | [Reply routing](README.md#reply-routing): a session's post carries its short tag and is mapped in KV for 7 days. A Telegram Reply from a listed user reaches the original sender with its topic and tag, as a reply. `#<tag> text` reaches that session. An unlisted Telegram user's reply is answered as unknown and logged, with no bus send. An expired mapping answers "use #tag". A message from a listed user that no reply, tag, subscription or default route matches is answered in that chat, saying it was not understood and listing the known tags; it is never dropped silently. The sender is `#telegram/…` with `[<user> (TG)]` in the body; the agent holds no user token. Checks on the fake API and a real fixture daemon: reply and tag routes, counted in the target's inbox. Mutants: an unroutable message dropped silently; the listed-user check removed; the reply routed to the last poster instead of the mapped one; the mapping never expiring |
| ✅ TG.10 Per-chat routing | [Per-chat routing](README.md#per-chat-routing): a chat's `route` wins over the global one; with `tags: false` a `#word` in that chat is not routed by tag; with `anyone: true` an unlisted member's mention reaches the chat's route, with no "unknown" answer, while the same sender in another chat is still refused; a chat with no entry behaves as TG.9. In a channel (the bot an admin), a post goes to that channel's `route`, and a session can post to the channel; its linked discussion group is configured as a group. Checks on the fake API and a real fixture daemon. Mutants: the chat route ignored; tags still detected with `tags: false`; `anyone` applied to every chat |
