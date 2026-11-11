# agent-bus: why you want it and what it gets you (T-8, draft for owner review)

Node-local, not tracked. Written by #ab-w2b against main 0.9.172. Every line is a benefit true at HEAD with a docs link as evidence; anything not built is marked **pending**. Problem first, outcome second, the component (bus, runner, agent) is only the "how", and an internal fact appears as a one-line proof next to its link.

## A. The problems it solves

| Your team's problem | What you get | How (and proof) |
|---|---|---|
| **Your AI sessions cannot talk to each other, to scripts or to you.** Each one is an island you copy-paste between. | One AI asks another for a review and waits for the verdict; a message to a busy or sleeping session is kept until it is back; your sessions can be reached from another machine. | Every session, script and person gets a name and a queue. Proof: [inbox queues](../../docs/04-messaging.md#inbox-queues), [request and reply](../../docs/04-messaging.md#request-and-reply), [push into live Claude Code, Codex and opencode sessions](../../docs/06-launchers.md#smart-launchers), [remote access](../../docs/02-access-remote.md#choosing-a-way) |
| **You cannot safely give an agent a password or a shell.** | The AI uses mail, chat, databases, servers and models without ever holding the key; you choose, per name, who may use what. | A helper holds the secret; callers only get answers. Proof: [access lists](../../docs/02-access.md#acl), [what the helpers are](../../docs/agents/README.md#bundled-agents) |
| **Work waits until somebody remembers.** | "Check the logs at nine every weekday" and "is the deploy green in 30 minutes?" wake an AI session by themselves, exactly once. | `#cron` and `#at`. Proof: [cron](../../docs/agents/cron.md#what-it-does), [at](../../docs/agents/at.md#what-it-does) |
| **You find out about breakage late.** | A server error reaches an on-call AI session, which investigates and tells you on Telegram, Slack or by text. | Log forwarding plus chat/SMS helpers (the session is the link). Proof: [logwatch](../../docs/agents/logwatch.md#what-it-does), [Telegram](../../docs/agents/telegram.md#what-it-does), [SMS](../../docs/agents/sms.md#what-it-does) |
| **Deploying means logging in to every server.** | One push updates every server that follows the branch, each runs its own reload script and reports one result line. | Webhook, `#git` autopull. Proof: [autopull](../../docs/agents/git-autopull.md#how-it-works). **Pending on this node:** channel setup and the owner's switch; no real hosting service wired yet |
| **Mail, chat and phone are walled off from your AI.** | An AI triages a mailbox, replies, posts to a team channel, answers when someone replies in a Slack or Telegram thread. | Proof: [IMAP](../../docs/agents/imap.md#what-it-does) (mail labelled untrusted), [SMTP](../../docs/agents/smtp.md#what-it-does), [Slack](../../docs/agents/slack.md#what-it-does), [help-desk walk-through](../../docs/agents/README.md#example-a-mail-help-desk) |
| **Your AI needs data and the web, but not your credentials.** | Read-only SQL on MySQL, PostgreSQL, ClickHouse; a web page as text; a model call; all through keys the AI never sees. | Proof: [db](../../Plans/R1.0-Release/dbsync.md#what-the-db-agent-offers-and-what-dbsync-does-not-assume), [fetch](../../docs/agents/fetch.md#what-it-does), [ai](../../docs/agents/ai.md#what-it-does) |
| **Your handy scripts cannot be shared safely.** | A script becomes a named team tool with no messaging code, its passwords kept apart, confined to its own folder, restartable from a web page. | The runner. Proof: [runner](../../docs/runner/README.md), [isolation](../../docs/runner/security.md#isolation-today), [controls](../../docs/runner/operations.md#controlling-one-agent), [instances](../../docs/runner/agents.md#give-one-package-several-configurations) |
| **A reboot kills your long-running AI sessions.** | One call brings back the set of terminal windows (AI sessions, monitors). | Proof: [tmux sessions](../../docs/agents/tmux-sessions.md#what-it-does) |
| **Several agents step on each other.** | Claim a task with a named lock, note progress in a shared store, announce a change to everyone on a channel. | Proof: [locks](../../docs/01-identity-and-authority.md#shared-locks), [store](../../docs/03-records-kv.md#key-value-store), [channels](../../docs/03-records-channel.md#the-two-channel-kinds) |
| **You cannot see what is going on.** | One web page shows what needs attention (refusals, full queues, lost messages) and manages names and access. | Proof: [dashboard](../../docs/05-discovery.md#dashboard) |
| **Pending:** installing it takes work today. | One-command install on a clean Linux host, container image. | [simple install](../../docs/07-setup.md#install), waiting for its first signed release |

## B. The pieces (the "how")

### The bus

**What it is for.** Your AI sessions, scripts, services and people reach each other by name, with a say over who may call whom, from one program instead of a stack of servers ([overview](../../docs/00-overview.md#why-youd-want-it)).

| You get | Evidence |
|---|---|
| Every agent, script, person and session has a name and an inbox; a message waits there until its reader is back, so a restart loses nothing queued | [inbox queues](../../docs/04-messaging.md#inbox-queues), [durability](../../docs/04-messaging.md#durability) |
| Send and forget, or send and wait for the answer; the sender learns "received" and "done" separately | [request and reply](../../docs/04-messaging.md#request-and-reply), [receipts](../../docs/04-messaging.md#receipts) |
| Your live Claude Code, Codex and opencode sessions get messages pushed into the running session, so one AI can ask another | [session launchers](../../docs/06-launchers.md#smart-launchers) |
| AI tools see who is on the bus and can list, send, wait for and answer messages, with no custom code | [MCP tools](../../docs/05-discovery.md#mcp-minimum) |
| You choose who may send to whom, per name; passwords stay with the helper that holds them, never with the caller | [access lists](../../docs/02-access.md#acl) |
| One message to one reader (a queue) or a copy to everyone subscribed (a channel) | [queues and channels](../../docs/03-records-channel.md#the-two-channel-kinds) |
| Shared notes and counters per name, and named locks, so several agents can split work without a database | [key-value store](../../docs/03-records-kv.md#key-value-store), [locks](../../docs/01-identity-and-authority.md#shared-locks) |
| Reach it from another machine over HTTPS, or through SSH with no open port | [remote access](../../docs/02-access-remote.md#choosing-a-way) |
| One web page shows what needs attention (refusals, full queues, lost messages) and lets you manage names and access | [dashboard](../../docs/05-discovery.md#dashboard) |
| Python, TypeScript, Go, Rust, Java and PHP libraries, all MIT, can call, serve and publish | [client libraries](../../src/clients/README.md) |
| Encrypted export and import of your records, credentials and settings | [backups](../../docs/user/backup.md) |
| **Pending:** one-command install on a clean Linux host (waits for its first signed release); container image | [simple install](../../docs/07-setup.md#install) |

### The runner

**What it is for.** A script becomes a named helper without learning the bus, and its passwords and reach are controlled by the host, not by the script ([runner](../../docs/runner/README.md)).

| You get | Evidence |
|---|---|
| Drop a script in a folder, reload, and call it by name; it gets the message, you get its output, no message loop to write | [first managed agent](../../docs/runner/getting-started.md) |
| Each helper runs confined: it cannot see other helpers, cannot write outside its own work folder, has memory and process limits, and holds no bus credential | [isolation](../../docs/runner/security.md#isolation-today) |
| Passwords live in a separate secret and settings in a visible configuration; the script finds both in its environment | [configuration](../../docs/runner/configuration.md), [dedicated runner secret](../../docs/06-dedicated-runner.md#script-agents) |
| One package, several instances with their own settings, secrets and access lists (`#smtp/support`, `#db/reports`) | [instances](../../docs/runner/agents.md#give-one-package-several-configurations) |
| Start, stop, restart, enable and disable each helper, and see why one was refused, from the CLI or the web page | [controlling one agent](../../docs/runner/operations.md#controlling-one-agent) |
| A helper that must stay running (a mail reader, a chat connection) is kept alive and restarted with back-off | [agents that serve themselves](../../docs/runner/configuration.md#agents-that-serve-themselves) |
| Any one script can also be a helper without the runner: `agent-bus run` | [dedicated runner](../../docs/06-dedicated-runner.md#script-agents) |
| **Pending:** pools that grow and shrink with load, long-lived processes that take many messages, installing helpers over the bus with no shell, named-host network filtering | [what is not available yet](../../docs/runner/operations.md#why-an-agent-was-refused) |

### Bundled agents, as building blocks

Each is a ready helper you switch on and grant by name ([choose an agent](../../docs/agents/README.md#choose-an-agent)). "Gives" is what you can do without logging in anywhere.

| Agent | Gives you |
|---|---|
| [`#slack`](../../docs/agents/slack.md#what-it-does), [`#telegram`](../../docs/agents/telegram.md#what-it-does) | Your AI posts to and reads chats; your reply in a thread goes back to the right AI session; matching messages are forwarded to a session |
| [`#sms-telnyx`](../../docs/agents/sms.md#what-it-does) | One text message to a phone, once |
| [`#imap`](../../docs/agents/imap.md#what-it-does) / [`#smtp`](../../docs/agents/smtp.md#what-it-does) | An AI reads, searches and triages one mailbox, or sends mail as it, without the password; mail is labelled untrusted |
| [`#cron`](../../docs/agents/cron.md#what-it-does), [`#at`](../../docs/agents/at.md#what-it-does) | A message sent on a schedule or once later; wake an AI session at nine on weekdays |
| [`#webhook`](../../docs/agents/webhook.md#what-it-does) | An HTTP post, or a GitLab/GitHub push, becomes a bus message |
| [`#logwatch`](../../docs/agents/logwatch.md#what-it-does) | Ask what a server logged, or have new errors sent to an inbox |
| [`#server-info`](../../docs/agents/server-info.md#what-it-does), [`#systemd`](../../docs/agents/systemd.md#what-it-does) | Disk, memory, certificate expiry; unit status, logs, restart within a limit |
| [`#git`](../../docs/agents/git.md#what-it-does) | See, update and deploy code on a server; update every server on a push ([autopull](../../docs/agents/git-autopull.md)) |
| [`#db`](../../Plans/R1.0-Release/dbsync.md#what-the-db-agent-offers-and-what-dbsync-does-not-assume) | Read-only SQL for an AI on MySQL, PostgreSQL, ClickHouse; the database enforces read |
| [`#fetch`](../../docs/agents/fetch.md#what-it-does) | A web page as text, or a scoped HTTP request |
| [`#ai`](../../docs/agents/ai.md#what-it-does) | A shared door to a model; the key stays with the administrator |
| [`#tmux-sessions`](../../docs/agents/tmux-sessions.md#what-it-does) | A set of terminal windows (AI sessions, monitors) restored after a reboot with one call |
| [`#debug`](../../docs/agents/debug.md#what-it-does), [`#bus`](../../docs/agents/bus.md#what-it-does) | One call says whether the bus and its helpers work |
| [`#rag`](../../docs/agents/rag.md#what-it-does), [`#voyage`](../../docs/agents/voyage.md#what-it-does), [`#qdrant`](../../docs/agents/qdrant.md#what-it-does) | Store documents and sort them; **pending:** search by meaning |


## C. Combinations that work at HEAD

(Verified from the guides and the agents' own checks; no single end-to-end run of each chain on a live node. Each hop needs the destination's access list to admit the sender, and the helpers are enabled by the owner.)

1. **Server error to your phone.** `#logwatch` `sub add` an on-call AI session on `ERROR` ([logwatch use cases](../../docs/agents/logwatch.md#what-it-does)); the session investigates with `#systemd` and `#server-info`, then tells you through `#telegram` or `#sms-telnyx`. Checks: logwatch's own tests cover `sub add` and alert delivery; the session reaches helpers by MCP.
2. **A scheduled AI task.** `#cron` `sub add 0 9 * * 1-5 #claude/ops@… check the overnight logs` wakes a live session every weekday ([cron](../../docs/agents/cron.md#what-it-does)); `#at` `in 30m …` does the same once ([at](../../docs/agents/at.md#what-it-does)). The session reads with `#logwatch` or `#db` and posts the result with `#slack`.
3. **Push to every server.** A GitLab or GitHub push posted to `#webhook` is published to the `autopull-cmd` channel; each server's `#git` updates its copy, runs its own after-update script and reports one result line on `autopull-log` ([autopull](../../docs/agents/git-autopull.md#how-it-works), [webhook](../../docs/agents/webhook.md#what-it-does)). **Pending on this node:** the channels' setup and the switch from the old autopull are the owner's; no real hosting service has been wired.
4. **A mail help desk.** `#imap` `sub add` a support queue; a sandboxed AI session reads it, looks up answers in `#git` or `#rag`, replies by `#smtp` and alerts a person by `#slack` or `#sms-telnyx` ([the walk-through](../../docs/agents/README.md#example-a-mail-help-desk)). Pending: `#rag` search by meaning.


## D. What you can do with it

Scenarios from an outsider pass, kept only where every piece is built at HEAD (checked against the guides); "via a session" means a live AI session sits in the middle and calls the helpers, because a helper's alert is a message, not a command to another helper.

**AI sessions working together**

1. Your Claude Code session asks your running Codex session to review a diff and waits for the verdict. [sessions](../../docs/06-launchers.md#smart-launchers), [request and reply](../../docs/04-messaging.md#request-and-reply)
2. One session announces "shared types changed" on a channel and every subscribed session hears it. [channels](../../docs/03-records-channel.md#the-two-channel-kinds), [sessions](../../docs/06-launchers.md#smart-launchers)
3. Sessions claim a task with a named lock and note progress in the shared store so two never take the same one. [locks](../../docs/01-identity-and-authority.md#shared-locks), [store](../../docs/03-records-kv.md#key-value-store)
4. Send a task to a session on another machine at night, close the laptop, read the answer in the morning. [durability](../../docs/04-messaging.md#durability), [remote access](../../docs/02-access-remote.md#choosing-a-way)
5. A Go reviewer and a Python worker exchange messages with no shared code. [clients](../../src/clients/README.md)

**Timed and event-driven work**

6. Wake a coding session every weekday at 6 with "run the tests and report failures"; or once, in 30 minutes, "is the deploy still green?". [cron](../../docs/agents/cron.md#what-it-does), [at](../../docs/agents/at.md#what-it-does)
7. A server logs an error; an on-call session is woken, checks units and disks, and messages you on Telegram or by SMS (via a session). [logwatch](../../docs/agents/logwatch.md#what-it-does), [systemd](../../docs/agents/systemd.md#what-it-does), [Telegram](../../docs/agents/telegram.md#what-it-does), [SMS](../../docs/agents/sms.md#what-it-does)
8. Every morning a session checks certificate expiry on your hosts and posts to Slack only when one is near (via a session). [cron](../../docs/agents/cron.md#what-it-does), [server-info](../../docs/agents/server-info.md#what-it-does), [Slack](../../docs/agents/slack.md#what-it-does)
9. Push to main and every server that follows the branch updates itself, runs its own reload script and reports one result line. [autopull](../../docs/agents/git-autopull.md#how-it-works) **Pending on this node:** channel setup and the owner's switch; no real hosting service wired yet.
10. A CI webhook reports a failed build to a session that reads the logs and suggests a fix in a Slack thread (via a session). [webhook](../../docs/agents/webhook.md#what-it-does), [logwatch](../../docs/agents/logwatch.md#what-it-does), [Slack](../../docs/agents/slack.md#what-it-does)
11. A new mail arrives in the support box; a sandboxed session reads it as untrusted text, drafts the reply and sends it by SMTP once you approve. [help desk walk-through](../../docs/agents/README.md#example-a-mail-help-desk), [IMAP](../../docs/agents/imap.md#what-it-does), [SMTP](../../docs/agents/smtp.md#what-it-does)

**Reaching things without handing out keys**

12. A coding agent answers "how many orders failed yesterday" with read-only SQL and never holds the password. [db](../../Plans/R1.0-Release/dbsync.md#what-the-db-agent-offers-and-what-dbsync-does-not-assume)
13. An agent reads a vendor's docs page through the fetch helper, limited to the sites you list. [fetch](../../docs/agents/fetch.md#what-it-does)
14. Message "status" to the bot in Slack from your phone; the session that gets it answers with disk, memory and failed units. [Slack](../../docs/agents/slack.md#what-it-does), [server-info](../../docs/agents/server-info.md#what-it-does)
15. A long-running AI session sits in a tmux window the bus brings back after a reboot, and you reach it from Telegram. [tmux sessions](../../docs/agents/tmux-sessions.md#what-it-does), [Telegram](../../docs/agents/telegram.md#reply-routing)

**Running your own scripts**

16. Five monitoring scripts run as helpers with secrets kept apart from the code, each confined, and you restart one from the web page. [runner](../../docs/runner/README.md), [isolation](../../docs/runner/security.md#isolation-today), [controls](../../docs/runner/operations.md#controlling-one-agent)
17. One backup-check script runs as six instances, one per host, each with its own settings. [instances](../../docs/runner/agents.md#give-one-package-several-configurations)
18. A PHP page, a Java service and a Rust logger pass messages through one bus. [clients](../../src/clients/README.md)

**Left out as not true at HEAD:** a runner script that calls another helper (scripts hold no bus credential, so a session or a kept helper does it); a panel view of stored counters; "unread messages fall back to Telegram"; a bus self-check that pages by itself (an answer goes back to the caller, not onward); search by meaning in `#rag` (pending).
