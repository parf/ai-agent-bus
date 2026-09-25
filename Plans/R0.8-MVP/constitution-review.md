# Constitution review, 2026-09-24

📌 **TL;DR:** A deep review of the [constitution](../../docs/constitution.md#project-constitution)
against 0.8.51 found most of it built as written. The exceptions are
thirteen code violations, two of them security gaps around reused names,
plus a set of doc statements that are stale, narrower than the code, or
self-contradictory. Fixes are [K.23–K.36](TODO.md#constitution-conformance);
open choices are [Q122–Q129](QUESTIONS.md#open-questions).

## Method

Five reviewers each took one area:
- persistence;
- logs and errors;
- users, tokens and authority;
- kinds, terms and fields;
- channels, groups and private values.

They checked every normative claim against the code and tests, and
reproduced the doubtful ones on disposable daemons, never the live node. The
most severe findings were rechecked in the code before this record was
written. A finding here is evidence of a gap, not a change: nothing was fixed
by the review.

## Code violations

Most severe first. The ID is the TODO row that fixes it.

| ID | Finding | Rule broken | Where |
|---|---|---|---|
| K.23 | A name freed by a record ignored at load can be registered by anyone, and inherits every ACL, Maintainer and Group-member reference other records still hold to it. Reproduced: a new `#lost@h` was admitted by `jobs@h` and became a member of `@crew@h` | a reused name inherits no authority | `core/commit.go:66-77`; `forgetName` runs only on unregister |
| K.24 | Ownership is stored and compared by User name, not `user_id`. A User recreated under a vanished User's name owned that User's ignored records after a restart, config included, and got a fresh token for them | `owner_id` is the stable `user_id` | `protocol/envelope.go:148`, every `r.Owner == name` check |
| K.25 | Waiting readers are refused before the commit, so a failed commit rolls the ACL back but the readers have already been refused | no reader observes a write before its commit | `core/manage.go:468`, `715-719`; `core/unregister.go:77` |
| K.26 | A User ignored at load for lacking its 👤 record is not marked ignored, so the ownerless-credential sweep deletes its credential | an ignored record keeps its credential | `core/snapshot.go:352-357`, `cmd/agent-busd/bus.go:130-139` |
| K.27 | A published copy keeps the expiry computed from the 📣's own TTL instead of taking its recipient's; a one-slot hop recomputes it. Reproduced: a copy outlived its recipient's 1 s TTL | each recipient is answered under its own destination's rules | `core/bus.go:1038`, `1266-1270` against `1107-1113` |
| K.28 | A 📣 accepts and stores `ttl`, `bound` and `overflow`, and gets a default `overflow`; every kind accepts `addr` and `protocol`, the 👤 record included | a field a kind cannot have is refused | `core/bus.go:272`, `336`, `398` |
| K.29 | An agent token row with an empty pair is bound silently at start, and re-issuing over an ignored mismatched row overwrites it unreported | the daemon MUST NOT repair the row | `core/credentials.go:81-84`, `auth/tokens.go` `mint` |
| K.30 | Users inside a Group on a 📣's `deliver_to` are skipped: no error, no log line, no drop | a User as a copy's recipient is an error, never discarded | `core/bus.go:1247-1252`; see Q126 |
| K.31 | `/group` stores duplicate members; `/manage` de-duplicates the same list | duplicate terms rejected or normalized deterministically | `core/manage.go:378-464` |
| K.32 | A User may be created with a template-form name such as `tmpl/eve`, and is issued a token | a User name is `user` or `user@team` | `core/users.go:607` |
| K.33 | Corrupt owner or administrator state at start goes to stderr only; token-store write failures and a failed debug-log open reach neither the error log nor syslog | a conceptual error is reported twice | `cmd/agent-busd/bus.go:78`, `auth/tokens.go:192`, `323`, `367` |
| K.34 | Registration drops fields it does not write, such as `status`, `maintainers`, `owner` and counters, instead of refusing them, and its reply echoes a caller-supplied `created_at` | a field is refused, never stored and ignored | `core/bus.go:470-475` |

## Doc corrections

Each row changes [constitution](../../docs/constitution.md#project-constitution)
text only, and is [K.35](TODO.md#constitution-conformance). Where the right
answer is open, the row names its question instead.

| Section | Correction |
|---|---|
| TL;DR | pending work is this plan's [conformance rows](TODO.md#constitution-conformance), not the finished 0.7 plan's |
| Persistence | the Durable row also holds activity days ([durability](../../docs/04-messaging.md#durability)) |
| Persistence | the "failed publish refuses everything" rule has no trigger in the lock-staging design: say so, or name the condition |
| Persistence | "shares an internal ID with an earlier one" means earlier by name order, and SQLite's schema already forbids it |
| Persistence | SIGHUP currently stops the daemon and leaves its sockets; state it or ignore the signal |
| Persistence | which incorrect rows refuse the start rather than being ignored: Q122 |
| Logs | setup, not the unit, creates `/var/log/agent-bus` (set-group-ID `adm`) |
| Errors | a refused administrative action is audited, as the audit table already says; the "appears nowhere else" sentence says otherwise |
| Registry record | "credential operations" are token and session operations; setting a secret is audited with its target only |
| Registry record | enrolment's audit actor is written as the signature, and startup seeding is not audited |
| User | no Ed25519 key is stored on a User, and a User's last use is derived from its credential |
| Daemon | no daemon description exists |
| Token | the stored fields are the current and previous token, issued, used, `user_id` and `agent_id`; no `updated_at` |
| Token | a User credential may be issued before it is bound; the unbound exception belongs here |
| Actors | terms are stored in canonical form (trimmed, lower-case); an ACL may hold a bare 📮 or 📣 name as a route source, so a bare name is a User or a channel source |
| Common fields | doc names against wire names: `description`/`descr`, `deliver_to`/`subs`, `updated_at`/`at`, `owner_id`/`owner` |
| Authority | the daemon Owner's override and the Administrators' Group edits belong in the table; a Maintainer may also remove and configure |
| Authority | whether Owner and Maintainers read private values and inactive records, and reach Users: Q125 |
| Channels | whose `dropped` a 📣 branch failing through a forwarding record counts: Q127 |
| Channels | a Group nested inside an active one and inactive is skipped without a log line; say "a listed Group" or log it |
| Group | an Administrator may make any Maintainer-level edit on a Group, not only membership |

## Untested claims

Built and working by reproduction, but no check fails when they break. They
are [K.36](TODO.md#constitution-conformance).

| Claim | Missing check |
|---|---|
| queue contents flush every minute | a daemon with a short `-flush-every`, killed after one interval, keeps the queue |
| error-log lines reach syslog at matching severity | a journal test with an injectable syslog writer |
| setup installs the logrotate configuration | a setup test on its content against a scratch root |
| a 👤 record whose owner names no User is ignored | a smoke check beside the agent case |
| an Administrator cannot reactivate another | the reactivation case beside the deactivation test |
| a reader is released when its record goes inactive | a core test: consume, deactivate, assert the release |
| human faces distinguish a configured route from an allowed one | a web contract test for both route states |

## Minor

Error texts that name the wrong cause:
- `ErrBusy` ("cannot unregister a busy inbox") also answers an occupied
  one-slot field, a duplicate `--add-allow` and removing a 👤 record;
- `ErrKind` prefixes field refusals with "unknown record kind";
- the bus child's load errors carry a `local accounts:` prefix;
- `SetSecret`'s comment still says the secret is never parsed.

The web face splits textarea terms on any whitespace, not one per line.

## What matches

These hold as written:
- write-through staging and rollback, one transaction per change, and the
  exclusive database lock;
- removal dropping every reference in one commit, and Users never removed;
- the kind enum, the `#` and `@` prefixes on every face, `--agent`, and `%23`;
- the reserved `@owner` and `@agent`, and the Personal rules;
- the inactive-record rule, and the flow outcomes;
- forwarding checks, `original_to`, and the ten-step limit;
- 📣 counters, private-value digests, and config compaction;
- Group prefixes, and the protected `@administrators`;
- the three logs, with no token, secret or body in any of them.
