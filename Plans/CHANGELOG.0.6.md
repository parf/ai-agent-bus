# Changelog 0.6

📌 **TL;DR:** Archived changes **0.6.0–0.6.17**, 2026-09-18–2026-09-22, grouped by area. Earlier: [0.5](CHANGELOG.0.5.md#changelog-05); next: [0.7](CHANGELOG.0.7.md#changelog-07).

## Grouped changes

| Area | Versions | Summary |
|---|---|---|
| Record model | 0.6.0–0.6.6, 0.6.9 | Replaced delivery mode with the closed `user`, `agent`, `queue`, `pubsub`, `service` kinds. External services became address/protocol cards with no inbox, readers, backlog or queue policy; agents represented processes behind bus names. Registration, edits and restore shared validation; the interim `Inbox` display label returned to `Agent`. |
| Breaking names | 0.6.6–0.6.7 | `service-template` became `agent-template`; record `topic` became `channel` in CLI verbs, publish options and subscription fields; `reply_to.service` became `reply_to.name`. Old spellings were refused rather than aliased. Message send/consume/reply kept `topic` as a filter label. |
| Channel delivery | 0.6.1, 0.6.7, 0.6.15 | Counted disabled-recipient drops and made published copies carry the channel name as their topic. PubSub gained separate publish ACL and manager-controlled Deliver-To lists: Group recipients expanded at publish time, and recipients could remove themselves but not subscribe themselves. |
| Private service values | 0.6.8, 0.6.13 | Added separately read/written service secrets, exposed a digest rather than content in ordinary answers and kept submitted secrets out of redisplayed forms. This line's opaque-secret model was replaced by ENV validation in 0.7. |
| Web administration | 0.6.2–0.6.4, 0.6.10–0.6.16 | Added distinct Agents, external Services, Queues/PubSub and Personal views; unified create/edit forms and kind-specific fields. A Maintainer's save did not clear controls it was not offered. Live node counts were separated from uptime/call windows and Owner/administrative labels made consistent. |
| Launcher accounts | 0.6.17 | Added `ab-claude -2`, `-3` and `-4` configuration homes for separate Claude logins on one machine. |
