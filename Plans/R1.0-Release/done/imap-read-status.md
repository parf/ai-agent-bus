# IMAP live read status

## Result

Delivered in **0.9.154**, source **`cedba648`**, verified on main.
The [IMAP guide](../../../docs/agents/imap.md#what-it-does) owns the calls;
its [access section](../../../docs/agents/imap.md#who-may-use-it) owns flag
permissions and failure/recovery behaviour.

List/search show live server read/unread alongside cached summaries; list can
filter unread before its limit. Default read marks Seen, no-mark read leaves it,
and mark sets/clears only Seen. Cached reads still use live server checks.
Writable selection is confined to flag-changing calls; sync, subscriptions and
publication remain EXAMINE/PEEK. No other flag or mailbox write is implemented.

## Verification

- Final `src/smoke.sh --normal`: **1214 passed, 0 failed, 46.572 s**, under the
  slow lock, released before deployment. The expanded IMAP suite inside the gate
  ran **191/0 in 37.041 s**. Restored owning Seen checks: **12/0 in 9.181 s**.
- **26 compiling assertion-red mutants**, source restored: stale live flags,
  unread/order, writable list/publish/poll, BODY without PEEK, epoch mismatch or
  unknown epoch accepted, default/cached/no-mark behaviour, idempotence, add vs
  clear, replacement/other flag, BAD/NO, external verification result, deadline
  classification, UID/call bounds, strict JSON booleans and write retry.
- A fixture explicitly applies implicit Seen for BODY[] in writable SELECT and
  refuses STORE under EXAMINE. It models tagged NO/BAD, post-STORE disconnect,
  concurrent external flag change, cached-body reuse, permanent flag permission
  and expunge. All account names and mail text are synthetic.
- Two initially error-only mutant checks were hardened into direct assertions;
  their final runs are assertion-red. Removing TimeoutError alone is ineffective
  because it subclasses OSError: the earned alarm mutant removes both catches.
  No timeout or build-error mutant received credit.
- Docs structure/index checks **5/0**; changed-document link checks **bad 0**.

The first full fixture regression was **190/0** before the last extra epoch test.
An over-count cache fixture initially expected the oldest body to survive LRU;
its mtimes were made explicit. The cache's existing 1,000-entry bound was
confirmed, so the review's suspected over-count list regression was withdrawn.

## Review

`#claude/ab2-review` closed the landed source, independently verifying main
ancestry, VERSION and runtime byte-identity with the reviewed `8c4e4657` delta
following `f5547efe`. No open finding. Scope: static runtime hunks in wire.py,
cache.py and imap.py. Tests, docs, agent.json, acceptance and agents.md were not
read, and no check was run by the reviewer; the numbers above are implementer
evidence. The code preserves parsed tagged BAD before imaplib's generic-error
conversion and classifies the whole STORE-through-verification interval.

## Deployment and remaining live proof

Manager-reported deployment: old package backed up under `agent-backup/imap-*`,
new package atomically swapped and byte-matched to the build, runner reloaded.
The three configured instances run; the accountless base template remains
disabled. A read-only `list 5` on each instance showed live `(cached; read)` or
`(cached; unread)` rows. Post-deploy **6/6**. The manager pushed; the implementer
performed no push, live mail operation, account-file edit or host deployment.

The manager did not call read or mark. Real Seen writes, persistent flag
permission/provider variants and PEEK in a writable provider session remain
unproven live until the owner authorises a write check. No owner mail contents
or addresses were added to tracked files or fixtures.

## Approved plan history

<details><summary>Plan and review refinements retained as history</summary>

- ⏳ B.24 IMAP read status ([RD146](../DECISIONS.md#recorded-decisions)); xAB2
  - Owner, 2026-10-29: "does list know read status? it needs to"; then: "option for read (not to set) - default read marks as read; + separate mark command"
  - `list` and `search` show each mail's current read/unread state from the server, not a cached guess; `list` can show unread mail only
  - `read <uid>` marks the mail read on the server by default; an option reads it without changing anything
  - `mark <uid> read|unread` changes only that flag
  - The only server write is the read (`\Seen`) flag: never delete, move, copy, expunge or other flags. Subscriptions and `publish` keep reading without marking
  - Plan first: xAB2 writes the method shapes and the server-state rules as a plan line for review before code. The mailbox stops being opened read-only only where a flag is changed
  - Plan for review (xAB2, 2026-10-29): `list [limit] [--unread]` (JSON `method:list`, optional strict boolean `unread`); `search <text>` keeps its local raw-content match; both display live server `read`/`unread` beside each cached UID. Status is fetched with bounded, UID-checked `UID FETCH ... (UID FLAGS)` batches on each call, never persisted in the cache; expunged UIDs are omitted, unknown/malformed status fails rather than guessing. Unread filtering precedes the row limit. `read <uid> [--no-mark]` (JSON optional strict boolean `mark_read`, default true) renders/bounds the cached or PEEK-fetched body first, then explicitly sets only `\Seen`; cached reads still update the live flag. `mark <uid> read|unread` (JSON `uid`, `state`) sets/clears only `\Seen` and returns a fixed confirmation. Parse/UID/options validation precedes any connection.
  - Session rule: all existing operations, list/search, sync, publication and subscription polls use EXAMINE/read-only. Only default-marking read and mark use SELECT/writable. Bodies always use BODY.PEEK, even in a writable session; flag writes are exactly one `UID STORE <integer> +FLAGS.SILENT (\Seen)` or `-FLAGS.SILENT (\Seen)`, with constant tokens, followed by live FLAGS verification. No CLOSE (implicit expunge), STORE replacement, other flag, delete/move/copy/append/expunge command, redirect or automatic write retry. Writable-select refusal fails closed; a post-send transport/timeout/unparseable failure reports flag outcome unknown and advises inspection, never claiming the body was delivered or the flag unchanged; parsed refusal remains definite.
  - Review refinements before code: cache identity is host/port/account/mailbox plus UIDVALIDITY; flag-changing calls SELECT exactly Configuration mailbox and refuse on a mismatching existing cache epoch before purge/re-fetch/STORE (sync or no-mark read may refresh it explicitly). SELECT must permit persistent `\Seen` via PERMANENTFLAGS `\Seen` or `\*`; otherwise default read/mark refuse with cannot-mark, without returning a body as marked. All cached read calls, including no-mark, still connect, EXAMINE/SELECT, validate epoch/existence as applicable; server unavailable fails rather than returning offline text. An expunged UID refuses read/mark, without storing. A lost bus reply can leave Seen set; no-mark avoids this side effect, and repeats are idempotent.
  - Live-state bounds and outcomes: at most 1,000 cached UIDs per call in batches of at most 100, within the existing delivered deadline/60 s cap; larger sets refuse, never silently truncate the live-status query. Ignore unsolicited FETCH for other UIDs, require matching UID/valid FLAGS for requested entries, omit expunged entries. UID input is strict decimal without signs/leading zeroes, in 1..4294967295. Read current Seen before STORE; if already requested, skip the write. Tagged STORE NO/BAD is definite refusal (flag unchanged); only post-send transport/timeout/unparseable reply is unknown. Verification returns the observed Seen state, even if another mail client changed it; never retry/re-store. Ordinary IMAP admission permits Seen changes too, stated plainly in the guide. Fixture emulates BODY[] implicitly setting Seen under writable SELECT, rejects STORE under EXAMINE, and records every command.
  - Proof and delivery: TLS fixture with mixed live flags, external flag changes between calls, cached default read/no-mark, unread filtering before limit, set/clear preserving all other flags, malformed/absent UID/FLAGS, writable refusal and post-STORE disconnect; subscription/publish/sync retain EXAMINE/PEEK and no STORE. Mutants for stale status, ignored unread/no-mark, wrong STORE verb/flag, write in publish/poll and cached read missing STORE; normal under slow, source review, coordinated fresh version/changelog, docs with end-user wording only (no plan IDs/status/links), IMAP-only enabled deployment and check-deploy. Fixtures use synthetic accounts/content only; no live mail contents or owner addresses enter tracked files.
  - Acceptance on the TLS fixture server: list shows read and unread as the server holds them; `read` sets the flag and the no-mark option leaves it; `mark` sets and clears it; no other flag changes, nothing is expunged; a subscription forward leaves a mail unread; owning mutants fail on assertions; normal gate; deploy enabled


</details>
