# IMAP folders

📌 **TL;DR:**
- **Plan only; no code until the owner approves** ([B.30](TODO.md#bundled-agents), questions Q269–Q275).
- **Scope:** `folders`, a per-call `--folder` on sync, list, search, read and mark, a destructive `move`, `delete` as a move to Trash, and optionally a folder on `sub add`.
- **Safety:** the agent today never deletes, moves or expunges. `move` is the first destructive call, so it ships behind its own switch, off by default.
- **Order:** read-only first (`folders`, `--folder`), then `move`/`delete`, then subscriptions.

## Help text

What `help` prints for the new and changed calls, line for line, in the format of today's [help](../../docs/agents/imap.md#what-it-does). Lines not shown are unchanged except that "No delete, move or other flags" in `read` becomes "No other flags".

```text
folders [--counts]: List the account's folders: name, role such as Trash or Sent, and with --counts the message and unseen counts. Names are the ones every --folder takes. No mail is read.
sync [limit] [--folder <name>]: Fetch mail not yet cached, newest first; default 100. Return progress and whether more remains; use list to see cached mail. --folder picks the folder; default is the configured mailbox, then INBOX. A forwarding-state error blocks sync until rebaseline; read, list and search keep working.
list [limit] [--unread] [--folder <name>]: See what is cached: Date, From and Subject with live read or unread status, newest UID first; default 20. --unread filters before the limit. --folder picks the folder; default is the configured mailbox, then INBOX. JSON unread is a boolean. Date is as sent, unverified.
search <text...> [--folder <name>]: Find cached mail by text: up to 100 Date, From and Subject summaries whose raw mail matches, with live read or unread status. --folder picks the folder; default is the configured mailbox, then INBOX. The match runs locally; the server does no text search.
[destructive] read <uid> [--no-mark] [--folder <name>]: Get one raw mail and mark it Seen by default, cached or not. --no-mark (JSON mark_read:false) leaves it unchanged. --folder names the folder the uid is in; default is the configured mailbox, then INBOX. Every read needs a live server; a lost reply may leave the mail marked. No other flags.
[destructive] mark <uid> read|unread [--folder <name>]: Set or clear Seen on the server and get the observed read or unread state. Ordinary IMAP admission permits this. --folder names the folder the uid is in; default is the configured mailbox, then INBOX. An unknown write outcome is not retried.
[destructive] move <uid> <dest> [--folder <src>]: Move one mail from --folder (default: the configured mailbox, then INBOX) to the folder dest, which must exist. The uid is then gone from the source; the answer gives its new uid when the server says it. Refused unless the instance allows moving. Nothing else in the folder is removed.
[destructive] delete <uid> [--folder <name>]: Move one mail from --folder (default: the configured mailbox, then INBOX) to the Trash folder. It is not erased; empty Trash from your mail client. Refused unless the instance allows moving, or inside Trash itself.
sub add <target> [/regex/] [--folder <name>]: Forward mail that arrives after this add to a destination; the old backlog is never forwarded. --folder picks the folder to watch; default is the configured mailbox, then INBOX. A missing or empty filter matches ALL mail from this account. The author's permission to send is checked at add and on each forward, and the receiver must also admit this IMAP instance. At most 20 subscriptions per creator, 100 in total.
```

The `sub add` folder is optional (step 4, [Q274](QUESTIONS.md#imap-folders)); the other lines ship in steps 1–3.

JSON spellings: `{"method":"move","uid":7,"dest":"Archive","folder":"INBOX"}`, `{"method":"delete","uid":7,"folder":"INBOX"}`, `folder` on the others; `folders` takes `"counts":true`.

## Scope

The [IMAP guide](../../docs/agents/imap.md#what-it-does) owns today's calls; one instance reads one configured `mailbox`. The code is `src/agents/imap` (the request said `standalone/imap`; there is no such path).

| Part | Ships | Notes |
|---|---|---|
| `folders` | read-only | names, flags, hierarchy delimiter, counts when cheap |
| `--folder <name>` (JSON `folder`) | read-only | on `sync`, `list`, `search`, `read`, `mark`, `delete`, `move` |
| `move <uid> <dest> [--folder <src>]` | **destructive** | the source mail leaves the source folder |
| `delete <uid> [--folder <name>]` | **destructive** | a move to Trash, never a hard expunge |
| `sub add <target> [/regex/] [--folder <name>]` | optional, last | see [order of work](#order-of-work) |

Not in scope: creating, renaming or deleting folders, `copy` as its own call, hard expunge, flags other than Seen (and `\Deleted` inside the COPY fallback).

## Commands and options

| Call | Meaning |
|---|---|
| `folders [--counts]` | one row per selectable folder: decoded name, special-use role if the server names one, and with `--counts` the message and unseen counts |
| `--folder <name>` | the folder this call acts on; default is the configured `mailbox`, then `INBOX` |
| `move <uid> <dest>` | the UID is in `--folder`, else the default; `dest` must be an existing, selectable folder, never the source |
| `delete <uid>` | `move` to the Trash folder; refused when the source is the Trash folder |

- Names are the decoded names `folders` prints; the agent encodes modified UTF-7 and quotes as `mailbox` does today. `INBOX` matches case-insensitively, any other name exactly.
- A `--folder` that is not in the server's LIST is refused by name before any SELECT. A `\Noselect` folder is refused.
- Counts: `LIST-STATUS` when the server advertises it, else one bounded `STATUS` per folder, capped (recommendation: 50 folders, within the call deadline); over the cap the listing says counts were skipped. Without `--counts` no STATUS is sent.
- Every call still runs read-only (EXAMINE) except `mark`/default `read`, which open a writable SELECT of exactly the named folder as today, and `move`/`delete`, which open a writable SELECT of the source only.

## Cache and UIDs per folder

- The cache identity already includes the mailbox (server, port, account, mailbox) and the epoch includes UIDVALIDITY. Adding `--folder` therefore means: one cache directory per folder identity, UIDs never shared between folders.
- **Migration:** the existing single cache is exactly the cache of the configured `mailbox`. It is adopted in place as that folder's cache; no re-fetch and no rewrite. A check proves a pre-change cache directory is read as the default folder's and a second folder starts empty beside it.
- One byte budget (`cache_bytes`) covers all folders together; eviction stays least-recently-used across them (Q270).
- A UID is only meaningful with its folder: read, mark and move check UIDVALIDITY of that folder before using a cached copy, as today.
- Subscriptions and forwarding progress stay on the configured `mailbox` unless the last step ships; a per-call `--folder` never moves the poller.
- `publish` stays on the configured mailbox; the owner did not list it ([Q275](QUESTIONS.md#imap-folders)).
- Mail copied or moved into a folder gets a new UID there; it is a new arrival for a subscription on that folder, as the guide already says for the configured mailbox.

## Move and delete

### Order of operations for `move`

| Server offers | Method |
|---|---|
| `MOVE` (RFC 6851) | `UID MOVE <uid> <dest>`; the server removes the source copy itself |
| no `MOVE`, with `UIDPLUS` (RFC 4315) | `UID COPY`, check the answer names the new UID, `UID STORE +FLAGS.SILENT (\Deleted)` on that UID only, then `UID EXPUNGE <uid>` on that UID only |
| neither `MOVE` nor `UIDPLUS` | refused before any command that changes the mailbox (Q271) |

- **Never a bare `EXPUNGE`, and never `CLOSE` on a writable selection**: both drop every `\Deleted` mail in the folder, including mail another client flagged. `UID EXPUNGE` removes only the named UID.
- Failure in the COPY path: a failed COPY changes nothing; a COPY that succeeded and a flag or expunge that failed is reported as "copied, source kept" with the new UID, never retried. A connection loss after sending is reported unknown, as the guide says for marks.
- Success removes the source UID from that folder's cache and reports the destination UID when the server gives it (`COPYUID`); the destination cache fills on its next `sync`.
- The call is idempotent only in the refusal: a UID already gone is refused as expunged, as `read` does today.

### Trash and `delete`

- **Recommendation: `delete` moves to Trash** ([Q272](QUESTIONS.md#imap-folders)), not flag-and-expunge: recoverable, no `\Deleted` flag left for other clients to purge, and Gmail needs it (flag-and-expunge only removes a label there).

- The Trash folder is the one LIST marks `\Trash` (SPECIAL-USE, RFC 6154).
- Fallback when no folder carries it: a new optional Configuration key naming the Trash folder (Q272). With neither, `delete` is refused, saying to set it. Folder-name guessing is not done.
- Gmail: `[Gmail]/Trash` carries `\Trash`; moving a mail out of a label folder into Trash removes it from all labels, and in `[Gmail]/All Mail` it is the same move. The guide says this once.
- `delete` from the Trash folder is refused: there is no hard expunge by default. A separate owner-approved switch is left to [FUTURE](FUTURE.md#imap-folders-follow-up) if ever wanted.
- A server that auto-purges Trash is the owner's server policy, not this agent's.

### Authority

- **Recommendation: a Configuration switch, off by default** ([Q273](QUESTIONS.md#imap-folders)): a caller given read access must not gain the power to remove mail.

- Today a grant lets a caller read mail and change Seen only ([who may use it](../../docs/agents/imap.md#who-may-use-it)). `move` and `delete` lose mail from a folder, so they need a stronger gate than the access list alone: a Configuration switch, off by default, with `move` and `delete` refused by name while it is off (Q273). The name of the key is chosen at build time.
- The web and `help` mark both calls `[destructive]`. Mail text never grants this: a body is data.
- No call carries the credential; the destination folder is a plain name the caller supplies, validated against LIST.

## Docs touched when built

| Doc | Change |
|---|---|
| [IMAP guide](../../docs/agents/imap.md#what-it-does) | new calls and examples; the claims "only the read flag can change" and "no delete, move, copy, expunge or flag other than Seen" become "unless move is enabled"; the cache key line names the folder |
| `src/agents/imap/help.golden.json` | the new calls |
| [Subscriptions](../../docs/agents/subscriptions.md#what-each-helper-does-today) | only if the last step ships |

## Order of work

| Step | Does | Who | ETA after approval |
|---|---|---|---|
| 1 | wire `LIST` (+ `LIST-STATUS`, `STATUS` cap, SPECIAL-USE), `folders`, name validation | Team 2 worker | half a day |
| 2 | `--folder` on sync, list, search, read, mark; per-folder cache adoption and migration check | same worker | half a day |
| 3 | `move`, `delete`, the switch; the fixture models MOVE, no MOVE, UIDPLUS, no UIDPLUS, no `\Trash`, a foreign `\Deleted` mail | same worker; QA2 reviews | one day, mutants take the `slow` lock |
| 4 | folder on `sub add` (optional): the poller watches more than one folder | later, only if the owner asks (Q274) | one day |

Each step is its own commit and its own landing; step 3 does not land before 1 and 2.

## Acceptance (for the TODO item)

- `folders` lists the fixture's folders with roles; counts appear with `--counts` and a server without `LIST-STATUS` is capped and says so.
- `list`, `search`, `read` and `mark` with `--folder` act on that folder only; the same UID in two folders is two different mails; UIDVALIDITY of each is checked separately.
- A cache made before the change serves the default folder with no fetch, and a second folder starts empty.
- `move` with MOVE sends exactly one `UID MOVE`; with UIDPLUS only it sends `UID COPY`, a `UID STORE` and a `UID EXPUNGE` for that UID alone; with neither it sends nothing that changes the mailbox. A mail another client flagged `\Deleted` in the source folder survives every path.
- `delete` lands in the `\Trash` folder, or the configured one; with neither it is refused; from Trash it is refused.
- With the switch off, `move` and `delete` are refused by name and the fixture sees no write command.
- **Mutants** (compiling, assertion-red): bare `EXPUNGE` instead of `UID EXPUNGE`; `CLOSE` after the flag; the switch ignored; a folder name not checked against LIST; the cache key without the folder; UIDVALIDITY of the wrong folder; the source UID kept in the cache after a move; Trash from a guessed name; `delete` allowed inside Trash.
