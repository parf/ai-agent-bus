# Changelog 0.8

📌 **TL;DR:** Changes on the 0.8 feature line, newest first, one or two lines
per version. 0.8 is an even line: it starts from the stable 0.7.20. The
previous line is [changelog 0.7](Plans/CHANGELOG.0.7.md#changelog-07).

## 0.8.9 — 2026-09-23

Web page review: the section navigation wraps on a phone, an empty shared list
points to its Personal records, and Diagnostics, Account, Group and Overview layouts fit.

## 0.8.8 — 2026-09-23

The web Users page lists Users only, like the other list pages: one toolbar,
Contact, Agents and Last used columns, an empty-state card; leftover names move
to Diagnostics, shown only while one exists, and `/users?kind=other` redirects there.

## 0.8.7 — 2026-09-23

The signed-out page says what agent-bus is, shows the project's picture and
links the repository and its author; pages behind the gate are unchanged.

## 0.8.6 — 2026-09-23

A group may be named `@<owner>/<name>[@realm]`, reserved to that User; a
Personal group must be, and a prefixed group is never transferred.

## 0.8.5 — 2026-09-23

Every web settings form offers every field its kind may change: a group's
description, Personal, Maintainers, secret and Danger Zone; an agent's secret.

## 0.8.4 — 2026-09-23

The web Channels section is two: 📮 Queues and 📣 PubSub, each with its own list,
registration and settings; old `/channels` addresses redirect to the right one.

## 0.8.3 — 2026-09-23

`agent-bus ls -h` has a LAST USED column and a lookup carries `last_used` too;
groups are marked 👥 like every kind; usage lines show the realm as optional.

## 0.8.2 — 2026-09-23

Push stops when the daemon refuses the credential or the read, instead of
asking again every two seconds for as long as the session lives.

## 0.8.2 — 2026-09-23

A listing carries each name's `last_used` and puts the most recently used
first; `ab_ls` says how long ago.

## 0.8.1 — 2026-09-23

`consume --topic` or `--tag` alone selects on that field only; a topic filter
no longer misses every tagged message on its topic.

## 0.8.0 — 2026-09-23

Opens the 0.8 line. A launcher prints a runtime adapter's "attached" status in
green, not in the red it keeps for failures.
