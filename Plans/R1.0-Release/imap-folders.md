# IMAP folders

📌 **TL;DR:**
- **Approved 2026-10-31 12:05:** full IMAP mail automation, built by team 2 in five steps ([B.30](TODO.md#bundled-agents)); step 1 is urgent.
- **Scope:** folders, per-call `--folder`, flags and server-side search, MIME parts, copy/move/trash/delete/compact, folder and append management, subscriptions per folder.
- **Safety:** every call that removes, moves or writes mail sits behind a per-instance `allow_destructive` switch, off by default; read, mark and flag stay under ordinary admission.
- **Not included:** SIEVE, quota, IMAP ACL/shared, subscribe/unsubscribe.

## Help text

The owner's approved call list. `help` lines, the guide and the golden file are written from it when each step lands; this list is the contract until then.

**Common**

- `<uids>` is `123`, `123,130` or `100:200`, at most 1000 UIDs, with a per-UID result.
- `--folder <name>` (JSON `folder`) on every per-mail call; default is the configured mailbox, then INBOX.
- `allow_destructive` gates move, trash, delete, compact_trash, folder rename and delete, append and copy ([authority](#authority)). Read, mark and flag stay under ordinary admission.

**Step 1: folders and the per-folder cache**

- `folders [--counts]`: name and role (Inbox, Sent, Drafts, Trash, Junk, Archive); `--counts` adds total and unseen via STATUS.
- `--folder` on sync, list, search, read, mark and publish.
- Per-folder cache keyed by folder and UIDVALIDITY; the existing single-mailbox cache becomes the default folder's cache with no re-sync.

**Step 2: flags, search, headers, parts**

- `mark <uids> read|unread`; `flag <uids> add|remove <flag...>` (flagged, answered, draft, seen, a custom keyword; `\Deleted` refused).
- `list` gains a flags column and `--flag <f>`.
- `search` adds server-side IMAP SEARCH: `--from --to --subject --header Name:value --since --before --flag --no-flag --larger --smaller --limit`, combined with AND. Free text stays local.
- `headers <uids>` (no Seen).
- `parts <uid>`: the MIME tree with id, type, size, filename, inline or attachment.
- `part <uid> <part-id>`: decoded, base64 for binary, 25 MB cap, no Seen.

**Step 3: copy, move, trash, delete**

- `copy <uids> <dest>`.
- `move <uids> <dest>`: MOVE, else copy and remove those UIDs only.
- `trash <uids>`: to the folder with role Trash; refused if there is none.
- `delete <uids>`: permanent; refused without UIDPLUS.
- `compact_trash [--older <days>]`.
- The `allow_destructive` switch.

**Step 4: folders and append**

- `folder create <name>`.
- `folder rename <name> <new>` (INBOX refused).
- `folder delete <name> [--force]` (INBOX and role folders refused; a non-empty folder needs `--force`).
- `append <folder> [--flags f,..] [--date rfc3339]`: raw RFC 5322 body, 25 MB cap, replies with the new UID.

**Step 5: subscriptions**

- `sub add <target> [/regex/] [--folder]`; `sub list` has a folder column.
- `status` shows one line per synced or subscribed folder.
- IDLE where supported, else polling.

Not included: SIEVE, quota, IMAP ACL/shared, subscribe/unsubscribe.

## Scope

The [IMAP guide](../../docs/agents/imap.md#what-it-does) owns today's calls; one instance reads one configured `mailbox`. The code is `src/agents/imap`.

| Step | Ships | Class |
|---|---|---|
| 1 | `folders`, `--folder`, per-folder cache | read-only; **urgent** |
| 2 | `mark`, `flag`, flags in `list`, server-side `search`, `headers`, `parts`, `part` | read and ordinary-admission writes |
| 3 | `copy`, `move`, `trash`, `delete`, `compact_trash`, the switch | **destructive** |
| 4 | `folder create/rename/delete`, `append` | rename, delete and append **destructive**; create ungated |
| 5 | `sub add --folder`, folder column, per-folder `status`, IDLE | subscriptions |

## Rules for every call

- Names are the decoded names `folders` prints; the agent encodes modified UTF-7 and quotes as `mailbox` does today. `INBOX` matches case-insensitively, any other name exactly.
- A folder not in the server's LIST is refused by name before any SELECT. A `\Noselect` folder is refused.
- A `<uids>` set over 1000 is refused whole. Each UID gets its own result; one failure does not stop the rest, and nothing is retried after an unknown write outcome.
- Counts: `LIST-STATUS` when the server advertises it, else one bounded `STATUS` per folder, capped (recommendation: 50 folders, within the call deadline); over the cap the listing says counts were skipped. Without `--counts` no STATUS is sent.
- Reads open EXAMINE; `mark`, `flag` and the default `read` open a writable SELECT of exactly the named folder; destructive calls open a writable SELECT of the source only. `headers`, `part` and `parts` never set Seen (BODY.PEEK).
- Server-side `search` sends one IMAP SEARCH with the options ANDed; free text is matched locally against the cache as today.

## Cache and UIDs per folder

- The cache identity already includes the mailbox and the epoch includes UIDVALIDITY. Per folder means one cache directory per folder and UIDVALIDITY, UIDs never shared between folders.
- **Migration:** the existing single cache is exactly the cache of the configured `mailbox`. It is adopted in place as that folder's cache; no re-fetch and no rewrite. A check proves a pre-change cache directory is read as the default folder's and a second folder starts empty beside it.
- One byte budget (`cache_bytes`) covers all folders together, least-recently-used across them ([Q270](QUESTIONS.md#imap-folders) stays open).
- A UID is only meaningful with its folder: read, mark, flag and move check that folder's UIDVALIDITY before using a cached copy.
- Subscriptions and forwarding progress stay on the configured `mailbox` until step 5; a per-call `--folder` never moves the poller.
- Mail copied, moved or appended into a folder gets a new UID there and is a new arrival for a subscription on that folder.

## Move, trash and delete

### Order of operations for `move`

| Server offers | Method |
|---|---|
| `MOVE` (RFC 6851) | `UID MOVE <uids> <dest>`; the server removes the source copies |
| no `MOVE`, with `UIDPLUS` (RFC 4315) | `UID COPY`, check the answer names the new UIDs, `UID STORE +FLAGS.SILENT (\Deleted)` on those UIDs only, then `UID EXPUNGE` of exactly those UIDs |
| neither | `move` is refused before any command that changes the mailbox ([Q271](DECISIONS.md#recorded-decisions), `#ab-dir`): it must remove only the named UIDs, which needs `UID EXPUNGE` |

- **Never a bare `EXPUNGE`, and never `CLOSE` on a writable selection**: both drop every `\Deleted` mail in the folder, including mail another client flagged.
- A failed COPY changes nothing; a COPY that succeeded and a flag or expunge that failed is reported per UID as "copied, source kept" with the new UID, never retried.
- Success removes the source UIDs from that folder's cache and reports destination UIDs when the server gives them (`COPYUID`).

### Trash and permanent delete

- **`trash`** moves to the folder LIST marks `\Trash` (SPECIAL-USE, RFC 6154); refused if none. Gmail: `[Gmail]/Trash` carries it, and moving a mail there removes it from all labels.
- **`delete`** is permanent: `UID STORE +FLAGS.SILENT (\Deleted)` then `UID EXPUNGE` of exactly the named UIDs, refused without UIDPLUS. It is separate from `trash`; neither calls the other ([Q272](DECISIONS.md#recorded-decisions)). On Gmail, expunge in a label folder only removes the label; the guide says so once.
- **`compact_trash [--older <days>]`** expunges the Trash folder: all of it, or mail older than the given days by internal date. The same `UID EXPUNGE` rule applies to the selected UIDs.
- Folder-name guessing is not done.

### Authority

- **`allow_destructive`** is a per-instance Configuration switch, off by default ([Q273](DECISIONS.md#recorded-decisions)). While off, the calls it gates are refused by name and the fixture sees no write command.
- Gated: move, trash, delete, compact_trash, folder rename and delete, append, copy. Not gated: read, mark, flag (ordinary admission; today a grant lets a caller change Seen only, [who may use it](../../docs/agents/imap.md#who-may-use-it)).
- The web and `help` mark gated calls `[destructive]`. Mail text never grants this: a body is data. No call carries the credential.

## Folders and append (step 4)

- `folder create` makes the folder; `folder rename` refuses INBOX; `folder delete` refuses INBOX and any role folder, and a non-empty folder needs `--force`.
- `append` takes the raw RFC 5322 body (cap 25 MB, larger refused before any command), optional `--flags` and `--date`, and replies with the new UID (APPENDUID when the server gives it).

## Subscriptions (step 5)

`sub add --folder` ([Q274](DECISIONS.md#recorded-decisions)): the poller watches each subscribed folder with its own forwarding progress. `sub list` gains a folder column, `status` one line per synced or subscribed folder; IDLE where the server offers it, else polling. Subscriptions without a folder keep the configured mailbox.

## Docs touched when built

| Doc | Change |
|---|---|
| [IMAP guide](../../docs/agents/imap.md#what-it-does) | new calls and examples; the claims "only the read flag can change" and "no delete, move, copy, expunge or flag other than Seen" become "unless `allow_destructive` is on"; the cache key line names the folder |
| `src/agents/imap/help.golden.json` | the new calls |
| [Subscriptions](../../docs/agents/subscriptions.md#what-each-helper-does-today) | step 5 |

## Order of work

| Step | Does | Who |
|---|---|---|
| 1 | LIST (+ LIST-STATUS, STATUS cap, SPECIAL-USE), `folders`, name validation, `--folder`, per-folder cache and migration check | Team 2 worker, **urgent** |
| 2 | flags, `mark`/`flag`, server-side search, `headers`, `parts`, `part` | same worker |
| 3 | copy, move, trash, delete, compact_trash, the switch; the fixture models MOVE, no MOVE, UIDPLUS, no UIDPLUS, no `\Trash`, a foreign `\Deleted` mail | same worker; QA2 reviews; mutants take the `slow` lock |
| 4 | folder create/rename/delete, append | same worker |
| 5 | `sub add --folder`, folder column, per-folder status, IDLE | same worker |

Each step is its own commit and its own landing; step 3 does not land before 1 and 2.

## Acceptance (for the TODO item)

- `folders` lists the fixture's folders with roles; `--counts` adds total and unseen, and a server without `LIST-STATUS` is capped and says so.
- Every per-mail call with `--folder` acts on that folder only; the same UID in two folders is two different mails; each UIDVALIDITY is checked separately.
- A cache made before the change serves the default folder with no fetch, and a second folder starts empty.
- `<uids>` accepts `123`, `123,130`, `100:200`; 1001 is refused; each UID has its own result.
- `mark`, `flag` change only the named flags; `\Deleted` is refused; `headers` and `part` leave Seen unchanged; `part` over 25 MB is refused.
- Server-side `search` options are ANDed and sent as one SEARCH; free text never reaches the server.
- `move` with MOVE sends one `UID MOVE`; with UIDPLUS only it sends `UID COPY`, `UID STORE` and `UID EXPUNGE` for those UIDs alone; with neither it sends nothing that changes the mailbox. A mail another client flagged `\Deleted` survives every path.
- `trash` lands in the `\Trash` folder and is refused when there is none; `delete` is permanent for exactly the named UIDs and refused without UIDPLUS; `compact_trash` expunges Trash only, `--older` limited by date.
- With `allow_destructive` off, every gated call is refused by name and the fixture sees no write command; `mark` and `flag` still work.
- `folder rename/delete` refuse INBOX; `folder delete` refuses role folders and a non-empty folder without `--force`; `append` over 25 MB is refused and a good one returns the new UID.
- `sub add --folder` forwards only mail arriving in that folder; existing subscriptions are unchanged.
- **Mutants** (compiling, assertion-red): bare `EXPUNGE` instead of `UID EXPUNGE`; `CLOSE` after the flag; the switch ignored; a folder name not checked against LIST; the cache key without the folder; UIDVALIDITY of the wrong folder; the source UIDs kept in the cache after a move; Trash from a guessed name; `delete` without the UIDPLUS refusal; `compact_trash` expunging another folder; a UID set expanded past 1000; `\Deleted` accepted by `flag`; Seen set by `headers`.
