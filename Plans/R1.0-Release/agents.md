# Agents for the runner

📌 **TL;DR:** The owner asked on 2026-10-17 for more agents the runner keeps,
and named three: telegram, systemd and logwatch. All three already exist as
entries in the R1.2 catalogue, so what moves to R1 is their build, not their
design. This file holds only what R1 adds to them, and the proposals still
waiting for a yes.

The shape every one of them follows — one name per grant, the daemon applying
the ACL, no daemon change — stays in the
[bundled agents catalogue](../R1.2/services.md#rules-they-all-obey). The
[tasks](TODO.md#bundled-agents) carry the acceptance. The rest of the
catalogue stays [R1.2](../R1.2/README.md#scope).

## What moved, and what R1 adds

| Agent | Catalogue entry | What R1 adds |
|---|---|---|
| telegram | [people and the world outside](../R1.2/services.md#people-and-the-world-outside) | where the bot token comes from, and a named destination always |
| systemd | [acting on the box](../R1.2/services.md#acting-on-the-box) | role tiers over one instance, instead of a name per verb ([Q168](QUESTIONS.md#agents-for-the-runner)) |
| logwatch | [reading the box](../R1.2/services.md#reading-the-box) | syslog as a third source, and keyword alerts forwarded to an agent |

These are the first catalogue entries built against the runner rather than by
hand, so each also answers whether the [runner](runner.md#runner) is enough to
keep an ordinary agent. A wave that needs a format first proposes it for
approval ([working rules](../../CLAUDE.md#working-rules)).

## Telegram

One template, one instance per account, the Slack agent's two halves: calls out
and a subscription forwarding a chat to an agent, 📮 queue or 📣 pubsub.

| | |
|---|---|
| Where the token lives | the record's bus secret, the one source it reads, as Slack's does ([rules](../R1.2/services.md#rules-they-all-obey)). `/rd/bin/telegram-post` and its `~/.codex/telegram.toml` are where the working token and the alias tables are **today**, and are what the first instance is seeded from — not a file read at run time ([Q169](QUESTIONS.md#agents-for-the-runner)) |
| A destination is always named | the existing script sends to the first configured direct chat when none is given. On the bus that default is removed: an omitted destination is a refusal. A convenience that picks a person when a group was meant is the one mistake a chat agent cannot take back |
| Aliases resolve, they do not authorise | `#group` and `@user` are lookup keys onto a chat id, the same rule the locator answers to ([people](../R1.2/people.md#how-to-reach-a-person)). An unknown alias is a refusal, never a fallback |
| More methods | beyond post and read, the API surface is the owner's to pick; the catalogue's rule is that a caller naming a provider wants that provider's own shape, not a normalised one |

## Systemd

`list` · `status` · `start` · `stop` · `restart` · `reload` · `enable` ·
`disable`, over the unit set its instance declares.

| Tier | Reaches |
|---|---|
| no role | `list` and `status` only — nothing that acts |
| operate (proposed name) | and `start`, `restart`, `reload` |
| admin | and `stop`, `enable`, `disable` |

| | |
|---|---|
| The grant is a name **and** a unit set | `#systemd/web@srv1` restarting nginx and not `sshd` is the same rule the globs are for in logwatch. A unit outside the instance's set is refused at every tier, admin included |
| The tiers are daemon-asserted roles | assigned in the allow list and carried on the message ([roles](../../docs/03-records-agent.md#roles)), not a field in the call. A flag in the body would be a privilege the caller granted itself |
| It revises a catalogue rule | the catalogue says danger is per name and never a flag, and that this agent's verbs are "each a grant of its own". Three tiers on one name is a second reading of that, after the Slack agent's ([one agent, both ways](../R1.2/slack/README.md#one-agent-both-ways)). Whether it is the right one is [Q168](QUESTIONS.md#agents-for-the-runner) |
| Reaching systemd at all | the sandbox gives each agent its own namespaces and no network ([what you grant the runner](../../docs/runner/security.md)), and nothing the daemon runs may gain privilege by exec ([process boundary](../../docs/08-processes.md#the-rule)). How this instance talks to the system manager — and under whose authority — is [Q170](QUESTIONS.md#agents-for-the-runner), and it is the one that decides whether this agent is buildable as described |

## Logwatch

The [ring-buffer service](../R1.2/services.md#reading-the-box) the catalogue
already adapts, with two additions.

| | |
|---|---|
| Three kinds of source, one agent | files by glob, journald units, and syslog. All three are an instance's configuration, the way journald already is — a source is not a second agent |
| The poll is the common one, and it is verbatim | last N lines, unfiltered, is the default; a filter is asked for. A filter silently hides the error nobody thought to grep for, which is the point the original makes |
| Alerts are the subscription half | a kept filter forwards matching lines to an agent, 📮 queue or 📣 pubsub, the shape the Slack agent's subscriptions already have. Its own ring stays the answer to *what just happened* |
| Bounded by construction | a log that floods cannot take the host, the instinct the queues are already built on ([overflow](../../docs/04-messaging.md#overflow)) |

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

Three, all the owner's, canonical in
[questions](QUESTIONS.md#agents-for-the-runner): Q168 whether systemd's tiers
stand against the catalogue's per-name rule, Q169 whether the TOML is also the
run-time token source, Q170 how a sandboxed agent reaches the system manager
at all.

The tier name for `start`/`restart`/`reload` is the owner's to pick; `operate`
is this file's proposal and the alphabet is the roles one, lowercase letters,
digits and `_` ([roles](../../docs/03-records-agent.md#roles)).

## Dependencies

| Agent | Needs |
|---|---|
| telegram, logwatch | the kept process form and its bounds (M.3), and M.9 for an agent that serves itself; both hold state between messages |
| systemd | [Q170](QUESTIONS.md#agents-for-the-runner) answered, and whatever M.4 settles about what an agent may reach |
| all three | the runner keeping an agent it did not write — M.1 and M.2, built; M.11 for the install |
