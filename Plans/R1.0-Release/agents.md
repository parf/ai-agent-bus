# Agents for the runner

📌 **TL;DR:** The owner asked on 2026-10-17 for more agents the runner keeps,
and named telegram, systemd and logwatch, then added IMAP and SMTP clients.
Their catalogue entries already exist in R1.2, so what moves to R1 is their
build, not their design. This file holds what R1 adds to them, and the
proposals still waiting for a yes.

The shape every one of them follows — one name per grant, the daemon applying
the ACL, no daemon change — stays in the
[bundled agents catalogue](../R1.2/services.md#rules-they-all-obey). The
[tasks](TODO.md#bundled-agents) carry the acceptance. The rest of the
catalogue stays [R1.2](../R1.2/README.md#scope).

## What moved, and what R1 adds

| Agent | Catalogue entry | What R1 adds |
|---|---|---|
| telegram | [people and the world outside](../R1.2/services.md#people-and-the-world-outside) | where the bot token comes from, and a default channel declared in the instance's configuration — **built**: [the Telegram agent](../../src/agents/telegram/README.md#the-telegram-agent) |
| systemd | [acting on the box](../R1.2/services.md#acting-on-the-box) | **built**: [systemd agent](../../src/agents/standalone/systemd/README.md#systemd-agent), with its own keeper |
| logwatch | [reading the box](../R1.2/services.md#reading-the-box) | syslog as a third source, and keyword alerts forwarded to an agent |
| server-info | [reading the box](../R1.2/services.md#server-info) | **built**: [server-info](../../src/agents/standalone/server-info/README.md#server-info-agent), with the shared standalone starter; the sandbox would hide the host it describes ([№B.7](TODO.md#bundled-agents)) |
| IMAP client | [mail](../R1.2/services.md#people-and-the-world-outside) | the inbound mail client is included in R1 |
| SMTP client | [mail](../R1.2/services.md#people-and-the-world-outside) | the outbound mail client is included in R1 |

Telegram and logwatch are the first catalogue entries built against the
[runner](runner.md#runner) rather than by hand, so each also answers whether it
is enough to keep an ordinary agent. Systemd answers the opposite question and
is kept outside it. A task that needs a format first proposes it for approval
([working rules](../../CLAUDE.md#working-rules)).

## Telegram

One template, one instance per account, the Slack agent's two halves: calls out
and a subscription forwarding a chat to an agent, 📮 queue or 📣 pubsub.

| | |
|---|---|
| Where the token lives | the record's bus secret, the one source it reads, as Slack's does ([rules](../R1.2/services.md#rules-they-all-obey)). `/rd/bin/telegram-post` and its `~/.codex/telegram.toml` are where a person **copies the working token from, once**, when seeding the first instance. Nothing in the agent reads that file and nothing may come to depend on it — the alias tables it holds today belong in the instance's configuration beside the default channel (owner decision, 2026-10-17) |
| The default channel is declared, not inferred | the instance's configuration names one, and a call without a destination goes there (owner decision, 2026-10-17). What does **not** survive is the existing script's implicit default — the first configured direct chat, which moves when the file is edited. A declared default is one decision per instance, visible at `reload`; an inferred one picks a person when a group was meant |
| Aliases resolve, they do not authorise | `#group` and `@user` are lookup keys onto a chat id, the same rule the locator answers to ([people](../R1.2/people.md#how-to-reach-a-person)). An unknown alias is a refusal, never a fallback |
| More methods | the Slack agent's set and keywords wherever Telegram allows (owner instruction, 2026-10-18): [the Telegram plan](telegram/README.md#methods) |

## Systemd

Built: [B.2 calls, scope and installation](../../src/agents/standalone/systemd/README.md#calls).

The [implemented methods and tiers](../../src/agents/standalone/systemd/README.md#calls)
apply only to the instance’s declared unit set.

**The runner does not keep this one.** It is standalone, as root, under a unit
the shared [standalone starter](../../src/agents/standalone/starter.md#standalone-starter) writes (built with №B.7), and the serve loop is the dedicated runner:
`agent-bus run` with the sandbox off, which is its default
([script agents](../../docs/06-dedicated-runner.md#script-agents)). Owner
decision, 2026-10-17.

| | |
|---|---|
| Why not the runner | the runner's unit sets `NoNewPrivileges=yes`, which every descendant inherits, and each agent is additionally in bubblewrap namespaces with every capability dropped — the runner refuses to start an agent on a host without it ([what you grant the runner](../../docs/runner/security.md)). No field could grant this agent what it needs there, so the answer is a different keeper, not a new option |
| `agent.json` may ask for less, never more | `limits`, `needs.writable` and `needs.network` only narrow, and the file ships inside the agent's own checkout — a key that widened would be a package granting itself privilege over its host. That is the general rule, not a systemd exception ([№Q170](DECISIONS.md#superseded), settled 2026-10-17 in [decisions](DECISIONS.md#recorded-decisions)) |
| The tiers are daemon-asserted roles | assigned in the allow list and carried on the message ([roles](../../docs/03-records-agent.md#roles)), never a field in the call: a flag in the body is a privilege the caller granted itself. The daemon applies the record's ACL before delivery, as for every agent |
| The tier check is inside a root process | which is what one name with three tiers means, and it is the accepted cost of not installing three units. It makes the ACL the real boundary: a caller who should never reach `stop` should not be in the allow list at all, and the tiers are the second line |
| The unit set is declared scope, not a host grant | `#systemd/web@srv1` naming nginx and not `sshd` is enforced by the agent, the same way its tiers are. It bounds mistakes, not an attacker who already has the process |
| It reaches the bus over IP | with its token in the unit's `EnvironmentFile`, the way the runner's `runner.env` holds one ([remote access](../../docs/02-access-remote.md#choosing-a-way)). No local socket is wanted and `root` need not be a mapped principal — and the same choice is what lets `#systemd@srv2` manage a host the daemon does not run on, the way logwatch reaches a second box by being a second name. A token in a root-readable file is as exposed as the host it manages, which is the bargain the runner's own unit already makes |

## Logwatch

The [ring-buffer service](../R1.2/services.md#reading-the-box) the catalogue
already adapts, with two additions.

| | |
|---|---|
| Three kinds of source, one agent | files by glob, journald units, and syslog. All three are an instance's configuration, the way journald already is — a source is not a second agent |
| The poll is the common one, and it is verbatim | last N lines, unfiltered, is the default; a filter is asked for. A filter silently hides the error nobody thought to grep for, which is the point the original makes |
| Alerts are the subscription half | a kept filter forwards matching lines to an agent, 📮 queue or 📣 pubsub, the shape the Slack agent's subscriptions already have. Its own ring stays the answer to *what just happened* |
| Bounded by construction | a log that floods cannot take the host, the instinct the queues are already built on ([overflow](../../docs/04-messaging.md#overflow)) |

## Mail clients

The owner added both clients to R1 on 2026-10-17: an **IMAP client** for reading
mail and an **SMTP client** for sending it. They are separate agents following
the [catalogue rules](../R1.2/services.md#rules-they-all-obey), kept by the runner.
Both are planned; neither is implemented by this scope addition. Their tasks
are [№B.5 and №B.6](TODO.md#bundled-agents).

## Still to approve

The owner asked for proposals. Most of what a new agent would do here is
already a composition of catalogue entries, which is evidence the catalogue is
right rather than short — a CI agent is `shell` with a declared command set, a
scheduled backup is `cron` calling the runner's `backup` verb. These are what
is left after taking those out, in the order worth building:

| | From | |
|---|---|---|
| **cron** | proposed, catalogue | *send this message later, or on a schedule.* Nothing on this bus starts anything on time: every agent is woken by a caller, and the runner has no clock either. It makes every other entry more useful and costs the daemon nothing. Listed under *acting on the box*, which is a misfiling — it acts on nothing and sends a message |
| **health** | owner, catalogue | `df`, `free`, `uptime` — the cheapest possible proof that the runner keeps an ordinary agent end to end, and worth building first for that alone rather than for what it reports |
| **files** | proposed, catalogue | read, write and list under one declared root. Almost everything else needs it, and the declared root is the grant |
| **probe** | proposed here | calls each agent a runner serves with its declared no-op and answers which replied. `status` says what is **installed**; after a `reload` nothing yet says what **answers**. With cron it is the check that a deploy worked, and it needs no new mechanism — only a no-op every agent already has to have ([method metadata](method-metadata.md#method-metadata)) |

## Open choices

None left. [№Q168](DECISIONS.md#superseded), [№Q169](DECISIONS.md#superseded) and [№Q170](DECISIONS.md#superseded) were all settled by the owner on 2026-10-17
([decisions](DECISIONS.md#recorded-decisions),
[questions](QUESTIONS.md#agents-for-the-runner)).

The owner accepted `operate` for the middle tier on 2026-10-17
([№RD93](DECISIONS.md#recorded-decisions)); no tier-naming question remains.

## Dependencies

| Agent | Needs |
|---|---|
| telegram, logwatch | the kept process form and its bounds ([№M.3](TODO.md#runner-waves)), and [№M.9](TODO.md#runner-waves) for an agent that serves itself; both hold state between messages |
| systemd | nothing in the runner's waves: it is a unit of its own running `agent-bus run`, which is built. Its install script and the tier names are the work |
| telegram, logwatch | the runner keeping an agent it did not write — [№M.1](TODO.md#runner-waves) and [№M.2](TODO.md#runner-waves), built; [№M.11](TODO.md#runner-waves) for the install |
