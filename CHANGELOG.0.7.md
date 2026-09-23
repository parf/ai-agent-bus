# Changelog 0.7

📌 **TL;DR:** Changes on the 0.7 development line, newest first, one or two
lines per version. 0.7 is an odd line: code may be broken until the
`STABLE - passed all tests and reviews` commit. The previous line is
[changelog 0.6](CHANGELOG.0.6.md#changelog-06).

## 0.7.10 — 2026-09-22

Groups are records of kind `group` whose allow list is their membership; any
User creates one; `@administrators` follows the daemon Owner; an inactive Group
grants nothing; database schema 4 drops the separate groups table.

## 0.7.9 — 2026-09-22

Users and records are active or inactive; an inactive one is no such entity,
its name reserved, readable only in the web face's `/inactive` view; paused,
banned and `disabled` are gone. A publication no recipient takes is refused.

## 0.7.8 — 2026-09-22

A secret must be an env file; a stored non-conforming secret or non-compact
configuration is ignored at load; a service's configuration is read through
its ACL, an agent's by the agent alone.

## 0.7.7 — 2026-09-22

Email, GitHub login and case-insensitive Twitter/X name are unique across
Users on add, edit, import and restore; Twitter/X names are syntax-checked.

## 0.7.6 — 2026-09-22

Credentials carry a User/Agent ID pair checked on every call; a transfer
rebinds and a removal deletes them in its own commit; stale pairs are ignored
and alerted; last use persists in batches.

## 0.7.5 — 2026-09-22

Agents are `#`-named and every record is owned by a User; Personal is valid on
every kind; a User hears only from Agents whose ACL admits it; incorrect stored
records are ignored and reported instead of deleted.

## 0.7.4 — 2026-09-22

The realm is optional: `alice` is a complete name, distinct from
`alice@srv1`, and setup's default Owner is the installer's bare account name.

## 0.7.3 — 2026-09-22

Every record and user carries a stable internal ID that is persisted, never
public and never reused, with its index rebuilt at every load.

## 0.7.2 — 2026-09-22

The daemon keeps three logs under `-log-dir`: an audit log of every
administrative action and entity edit, an error log copied to syslog, and a
debug log of requests that only the daemon Owner switches on.

## 0.7.1 — 2026-09-22

One exclusively held SQLite database replaces the JSON snapshot and the token
file; a management write commits only its entities before it is answered.

## 0.7.0 — 2026-09-22

Opens the constitution line. Adds the K.1 performance baseline benchmarks;
behaviour is unchanged from 0.6.17.
