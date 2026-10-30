# Agents your team can use

📌 **TL;DR:**
- **Problem:** your AI assistants and teammates need mail, chat, servers, code and data, and handing out the passwords or a server login is risky.
- **Outcome:** ready-made helpers do the job under their own name; an administrator installs one once and chooses who may use it, and people and AIs call it by name and never see its passwords.
- **Start:** pick a job in [Choose an agent](#choose-an-agent), then ask the helper what it can do: `agent-bus help '#<helper>'`.

## Why you want these helpers

- 🔑 **Passwords spread everywhere.** Now they stay with the helper; callers only get its answers.
- 🧱 **A login gives too much.** Now you grant one helper, or one role on it, per person or AI.
- 🧩 **Each tool is its own island.** Now an AI reads from one helper, decides with another and posts through a third.
- 🗂️ **One account for everybody.** Now each team gets its own instance with its own settings and access.

## What you get

- ✅ **One name per job:** `#systemd`, `#smtp/work`; callers ask the helper, not the server.
- ✅ **Keys stay put:** provider keys and passwords live with the helper.
- ✅ **Access per job:** grant a person or AI one helper, or one role on it, rather than a host login.
- ✅ **Helpers combine:** [incoming events](#incoming-events-go-to-a-queue-or-an-agent) flow into queues or AI sessions, as in the [mail help desk](#example-a-mail-help-desk).
- ✅ **Many instances:** [run as many as you need](#run-as-many-instances-as-you-need).

## Run as many instances as you need

- Most helpers can run as **many instances**, side by side (`#bus` is the exception).
- Each instance is its own agent on the bus, `#<helper>/<name>`, with **its own settings, its own passwords and its own access list**.
- Give a person or an AI exactly the instance it needs and nothing else.

| Instances | Why |
|---|---|
| `#smtp/support`, `#smtp/billing` | One per mailbox; the support AI can send only as support |
| `#db/reports`, `#db/admin` | A read-only account for reports, a full one for the DBA |
| `#fetch/docs` | Limited to your documentation sites; an AI granted only this one cannot browse elsewhere |
| `#cron/team`, `#cron/ops` | One per trust group, so a destination allows only the group it trusts |
| `#systemd/web` | Only the web units, for the web team |

Adding an instance is a settings change, not new code: for a runner helper see [several configurations](../runner/agents.md#give-one-package-several-configurations); a standalone helper is installed again under another `--name`.

## Incoming events go to a queue or an agent

Helpers that receive something from outside, such as a chat message, a log line or a mail, hand it to a destination you choose. That destination is either a
**queue** (an [inbox](../04-messaging.md#inbox-queues) that keeps messages until
someone reads them) or an **agent** directly, which may be an AI session.

| Source | Forwards |
|---|---|
| [Slack](slack.md#what-it-does), [Telegram](telegram.md#reply-routing) | Replies, tags and matching channel messages to sessions |
| [Logwatch](logwatch.md#what-it-does) | Matching log lines to subscribers |
| [IMAP](imap.md#what-it-does) | Future-mail subscriptions or one manual `publish` to its configured target ([who may subscribe](imap.md#who-may-use-it)) |
| [Fetch](fetch.md#what-it-does) | A page as readable text or Markdown, or a raw `curl` request, scoped by instance site lists |
| [Webhook](webhook.md#what-it-does) | An HTTP POST body to a configured queue or agent |
| [Cron](cron.md#what-it-does) | A scheduled or delayed message to a queue or agent |
| [At](at.md#what-it-does) | One deferred message to a queue or agent |

A queue suits work that may wait or is shared by several readers. An agent suits work one helper or AI should handle right away.

Subscriptions in every helper are meant to work one way: who may add one, what a refused copy does, what `sub list` shows ([subscriptions § the shared behaviour](subscriptions.md#the-shared-behaviour)).

### Example: a mail help desk

Questions from customers come in by mail. A help system written as an Obsidian vault (a folder of Markdown pages) answers most of them, and an AI that cannot do anything else sorts the mail and replies.

| Step | Helper |
|---|---|
| 1. Fetch new mail | [`#imap`](imap.md#what-it-does) subscription to the `support` queue |
| 2. Find the answer in the help pages | The vault in Git, read through [`#git`](git.md#what-it-does), or stored in [`#rag`](rag.md#what-it-does) to search by meaning |
| 3. Decide: answer, route or escalate | A sandboxed AI session that has only these grants, or an [`#ai` typed decision](ai.md#typed-decisions) |
| 4. Reply to the customer | [`#smtp`](smtp.md#what-it-does) `send`, with a recipient allowlist if wanted |
| 5. Alert a person when it matters | [`#slack`](slack.md#what-it-does) `post` or [`#sms-telnyx`](sms.md#what-it-does) |

- "Sandboxed" here means the AI holds grants for these few helpers and nothing else, so it can read mail and reply, but cannot touch servers, code or other accounts.
- Mail content is untrusted: `#imap` marks every published mail as external content, not instructions.

A subscription forwards each new mail by itself; nothing is forwarded until someone adds one, and the queue's access list must allow `#imap`.

## How to pick one

- **What job?** Pick the helper named for it below; each page opens with when to use it and when not to.
- **Who installs it?** The table under [Choose an agent](#choose-an-agent) says whether it needs an administrator, a runner or nothing at all.
- **Who may call it?** Most start owner-only; the Owner grants people and AIs.
- **One for everybody, or one each?** Run a separate [instance](#run-as-many-instances-as-you-need) per account, team or site list, each with its own settings and access.

## What a guide leaves out

- **Build status, versions, ids and plan links:** none; a guide is in the present tense and says nothing outside the release, as [WRITING.md](../../WRITING.md#how-we-write-docs) sets out.

## Bundled agents

One line each, from the description its `help` prints.

<!-- bundled-agents: generated by src/agents/agent_docs.py --write -->
- [`#ai`](ai.md) — Ask a model or submit a typed decision without sharing its provider key
- [`#at`](at.md) — Send one message to a queue or an agent later: in 30m, or at a time
- [`#bus`](bus.md) — The daemon on the bus; read-only methods use your authority
- [`#cron`](cron.md) — Send a message to a queue or an agent on a cron schedule or once later
- [`#db`](db.md) — Run one SQL statement against a MySQL, PostgreSQL or ClickHouse database; read is enforced by the database
- [`#debug`](debug.md) — Check delivery and runner health; diagnose failures with bounded probes
- [`#eventlog`](eventlog.md) — Named notice, warning and alert logs with independent rings and once-only subscriptions
- [`#fetch`](fetch.md) — Fetch pages or make scoped HTTP requests without a browser
- [`#git`](git.md) — Inspect, synchronize and deploy repositories within configured access levels
- [`#imap`](imap.md) — Read a private mail cache, manage Seen status and subscribe to future mail
- [`#logwatch`](logwatch.md) — Read and search host logs; mark a point and see what came after
- [`#qdrant`](qdrant.md) — Call the configured store within your read, write or admin role
- [`#rag`](rag.md) — Store, classify and embed records; manage their prompts
- [`#server-info`](server-info.md) — Read host facts: network, memory, disks, load and TLS certificates
- [`#slack`](slack.md) — Post and read messages; route replies; forward selected conversations
- [`#sms-telnyx`](sms.md) — Send one SMS through Telnyx; the body is {to,text} or \<to> \<text>; never retry unknown results
- [`#smtp`](smtp.md) — Send mail once through a configured account with verified TLS
- [`#systemd`](systemd.md) — Inspect units and logs; start, stop and restart units within configured scope
- [`#telegram`](telegram.md) — Post and read chats; route your replies to the right session
- [`#tmux-sessions`](tmux-sessions.md) — Keep a tmux session of named windows running: start, stop, restart and list them
- [`#voyage`](voyage.md) — Embed text or pass a request through Voyage without sharing its provider key
- [`#webhook`](webhook.md) — Receive authenticated events and post to configured webhook routes
<!-- /bundled-agents -->

## Choose an agent

An **agent** is a small helper with a name, such as `#systemd@node.example`. The **runner** keeps some helpers running; others have their own Linux service.

| Agent | How it runs | Who sets it up / who uses it |
|---|---|---|
| [Bus](bus.md) | Inside the daemon; nothing to install | Any bus caller, with their own authority |
| [Slack](slack.md) | Runner; Slack app and tokens | Admin installs; people and AIs use it |
| [Telegram](telegram.md) | Runner; a Telegram bot token | Admin installs; listed people and AIs use it |
| [Systemd](systemd.md) | Own service; root to manage units | Admin installs, or a [dedicated instance](systemd.md#dedicated-instances) per team; permitted operators and AIs |
| [Git](git.md) | Own service; an account that reads the code | Admin installs; permitted developers and AIs |
| [Server-info](server-info.md) | Own service; normally no root | Admin installs; permitted people and AIs read it |
| [Logwatch](logwatch.md) | Own service; an account that reads the chosen logs | Admin installs; permitted people and AIs read it |
| [Tmux sessions](tmux-sessions.md) | Own service, as the person | The person configures and starts the set |
| [AI](ai.md) | Runner, one process per call | Admin installs; any bus caller may use it |
| [IMAP](imap.md) | Runner, serves itself with a private cache | Owner-only at first; a grant exposes that account's cached mail |
| [Fetch](fetch.md) | Runner, stateless HTTP(S) reader and caller | Owner-only by default; private access requires a matching exact entry |
| [Webhook](webhook.md) | Runner, kept HTTP listener and outbound calls | Owner-only by default; every grantee can use configured outbound routes |
| [SMTP](smtp.md) | Runner, one process per call | Owner grants send-as-mailbox; optional recipient allowlist |
| [Voyage](voyage.md) | Runner, one process per call | Owner grants callers |
| [Database](db.md) | Runner, one process per call | Owner grants callers; the database enforces read |
| [Qdrant](qdrant.md) | Runner, one process per call; an [operator-run store](qdrant.md#run-a-store) | Owner grants read, write or admin |
| [RAG](rag.md) | Runner, kept process | Owner-only at first; all namespaces share its ACL |
| [Telnyx SMS](sms.md) | Runner, one process per call | Admin installs; any bus caller may send |
| [Cron](cron.md) | Runner, kept process | Owner grants callers; a destination receives only if its access list allows cron |
| [Eventlog](eventlog.md) | Runner, kept process with private alert state | Direct read/write roles; write also permits pull and clean |
| [At](at.md) | Runner, kept process | Owner grants callers; a destination receives only if its access list allows `#at` |
| [Debug](debug.md) | Runner, kept process | Admin installs; owner-only diagnostics |

## Use examples

Your Claude Code, Codex or OpenCode session must be connected to the bus, and can only use the helpers and actions it is granted.

| Say this to your AI | What it calls |
|---|---|
| "Read new mail, classify each one and post the urgent ones to Slack." | IMAP `sync` and `read`, then an [`#ai` typed decision](ai.md#typed-decisions), then Slack `post` |
| "Check failed services on all nodes." | `call --tagged systemd failed` |
| "Show the logs for the broken web service, then ask me before restarting it." | Systemd `logs`, then `restart` with restart permission |
| "Check memory and disk before we update the app." | Server-info `memory` and `disks`, then permitted Git calls |
| "What changed in the repository since yesterday?" | Git `log` and `diff`; no write permission needed |
| "Mail the team the nightly build report." | SMTP `send` |
| "Post to Telegram that the deploy finished." | Telegram `post`; you can answer from your phone |

Writing your own agent? Use the [prompt for your coding AI](create-agent-prompt.md#prompt-to-paste) and the [description and help checklist](authoring.md#checklist-writing-an-agents-description-and-help).

## Technical details

### Before you install

An administrator does this part. The bus and CLI must already be [installed](../07-setup.md#install). Pages use example names such as `node.example`; replace them with your server and account.

<details><summary>Running from a checkout or a release archive</summary>

- Run commands from the checkout root.
- From a release archive, replace `src/agents/` with `/usr/local/lib/agent-bus/current/agents/`;
- Slack and Telegram are bundled there, so copy their folders to the runner's shared code directory instead of running `package.sh` (see [installed layout](../07-setup.md#install)).

</details>

### Runner helpers

- Helpers marked "Runner" above need [a person's runner](../07-setup.md#a-persons-runner) (a background program that keeps a person's helpers running), such as `#runner/ann@node.example` running as `ann-ab-runner`.
- Its shared code directory is `/var/lib/agent-bus/agent.d`; its private settings are in `/var/lib/agent-bus/ann-ab-runner`.
- Installing helper code needs sudo.

### Standalone helpers

Systemd, git, server-info, logwatch and tmux-sessions have their own Linux services. Their installers need sudo and your current bus credentials; they save only the helper's own credential, never yours.

<details><summary>Getting your credentials for an installer</summary>

| Where you run the command | Fingerprint | Your token |
|---|---|---|
| Another computer | `ssh agent-busd@<node> token --fingerprint` | `ssh agent-busd@<node> token` |
| The bus server itself | `agent-bus token --fingerprint` | `agent-bus token` |

On the server, `agent-bus token` uses your login socket's identity; without one, pass the name, such as `agent-bus token ann`. Then:

```sh
export AGENT_BUS_ADDR=https://bus.example.com:6767
export AGENT_BUS_TLS_FINGERPRINT='sha256:…' # paste the fingerprint
read -rsp 'Your bus token: ' AGENT_BUS_TOKEN; printf '\n'
export AGENT_BUS_TOKEN
```

- On the bus server itself, the plain HTTP address the bus listens on (such as `http://127.0.0.1:6767`) works without a fingerprint; the token is still needed.
- Unix socket paths are not accepted; see [remote access](../02-access-remote.md#over-https).

Run `unset AGENT_BUS_TOKEN` afterwards. Removal needs your credentials again; see [installer lifecycle](../../src/agents/standalone/starter.md#lifecycle).

</details>

### Settings and passwords

**Configuration** holds ordinary settings: destinations, people, allowed units.
**Secret** keys are outside passwords such as bot tokens; never put them in
Configuration. Each agent page lists its keys and where they go; the two secret stores, bus Secret and the runner's private files, are compared in [runner security](../runner/security.md#secrets).

For [systemd](systemd.md#who-may-use-it) and [Git](git.md#who-may-use-it), the Owner and Maintainers can do everything; give others a role. Configured unit and repository limits still apply to everyone.

### Warnings and alerts

Each agent keeps a short record of what went wrong, at three levels, for its Owner and Maintainers to see on its web page; callers on its access list see none of it and get their own error answers.

| Level | Means | Shown | Every runner agent gets |
|---|---|---|---|
| alert | It failed, or cannot do its job; act now | Loud, kept longest | An exit nobody asked for, a kill by signal or memory or process limit, a failed start, a restart loop |
| warning | Something went wrong, but it still does its job | Quieter | Each line it writes to its error output |
| notice | Ordinary news | Folded away | Started, stopped |

Old entries expire. Agents with their own Linux service, and `#bus`, get no runner entries. Each agent's page lists its own under **Warnings and alerts**.
