# FileScan and DBSync, completed

📌 **TL;DR:** FS.1–FS.3 and DS.1 shipped in 1.0.33–1.0.48. Plans: [FileScan](../filescan.md#steps-and-estimate), [DBSync](../dbsync.md#steps-and-estimate). Current guides: [filescan](../../../docs/agents/filescan.md), [dbsync](../../../docs/agents/dbsync.md).

Moved from the R1.1 TODO on 2026-11-11: built and shipped in 1.0.x, so R1 work. History, not a current contract.

## Filescan

Owner, 2026-11-01; approved 2026-11-05 ([Q334](../../R1.1/resolved.md#Q334)); plan in [filescan](../filescan.md#steps-and-estimate), owner choices in [decisions](../DECISIONS.md#recorded-decisions), answers [Q305–Q310](../../R1.1/resolved.md#Q305). Staged, each stage landing alone: S1 the shared core, S2 FileScan, S3 [DBSync](#dbsync) ([stages](../filescan.md#stages)).

- ✅ FS.1 `#filescan/<name>`: a runner agent that tracks new, changed, deleted and renamed files in one directory and hands them to subscribed agents, by polling, by inotify, or both ([modes](../filescan.md#modes)); steps F0–F4 with an estimate of four to six hours of agent time
  - S1 built and landed: `src/agents/shared/change_sync.py` — state file (cap, atomic rename, one holder), subscriptions with authority over it, delivery that marks a change done only after the target confirms (a send of unknown outcome is recorded first, so a later delete is still sent), the poll timer; tests `test_change_sync.py`
  - State: built in 1.0.33 (`a25330d9`; S1 core `5337290a`); readers `#ab-w1a`, `#ab-w1c`. The rows marked not built below move to FS.2
  - S2 built (F0–F4) on the S1 core: runner `needs.dir` with the node owner's `roots` (default empty), `src/agents/filescan/` with poll, inotify, the mass-delete guard, the RAG form, the package, `docs/agents/filescan.md`, a smoke shard. **Not built:** the single `renamed` change from kernel cookie pairing (a rename is a delete plus a create, which is also RAG's form), the bigfile opt-in for a file over the inline limit (size and hash only), binding the directory by file descriptor (the runner re-resolves it at every launch instead)
  - Depends: RT31 (user-account runner, landed); the S1 shared core library ([Q314](../../R1.1/resolved.md#Q314)); F0 adds a read-only bind of one directory to the runner
  - Deploy: FileScan is a kept process (inotify) serving itself with its own token; the Owner registers its record and hands the token over once ([agents that serve themselves](../../../docs/runner/configuration.md#agents-that-serve-themselves)), and the plan names that step in the guide
  - Acceptance, each against a fixture directory and a fake subscriber, never the live bus:
    - **Poll:** a created, changed, renamed and deleted file are each sent once; a `touch` with the same content sends nothing; a file made and removed between two scans sends nothing
    - **Watch:** a file written and closed is sent within one second of the close; a rename inside the tree is one `renamed` change; an overflow (forced by a tiny queue) triggers a rescan that finds the missed file; with the watch limit set to one, the agent watches that directory, polls the rest, and `status` says so
    - **Both:** the same file arriving by watch and by poll is sent once
    - **Confirm:** a subscriber that never replies keeps its changes pending and receives them again; a second subscriber is not held up; a file is marked tracked only after the ok reply
    - **State:** killed between the temporary file and the rename, the old state loads and no truncated file exists; at `state_max` the scan is refused with the `state full` text and the file is byte-identical
    - **One scan:** a second scan started during a walk is refused by the lock
    - **Mass delete:** with `dir` unmounted, missing, or emptied past `delete_pct`, no delete is sent on either path, `status` names the guard, and `scan confirm:true` releases it
    - **Authority:** `sub add` and each forward need the author's may-send, a target that does not admit `#filescan` is refused, and the per-creator and total caps hold, as in [IMAP](../../../docs/agents/imap.md#who-may-use-it)
    - **Host:** `dir` through a symlink leading out, `/`, the home root and the runner state directory are refused; a system path outside any home is accepted ([Q308](../../R1.1/resolved.md#Q308)); the agent runs as the runner's per-agent user with no added privilege or sudo, `dir` is fixed by the record's Owner and no call changes it; the agent cannot write inside `dir`; inotify events arrive for a path bound read-only into the sandbox
    - **Content:** a file at or under the inline limit goes inline; above it, a subscriber that opted in gets a bigfile and any other gets size and hash with no contents; `as rag` skips it and `status` names it; no message ever carries a truncated prefix
    - **RAG form:** `as rag` sends upsert and delete with the path as id (hash over 128 characters), a rename is delete then upsert, and nothing is tracked on a refusal
    - **First run and replay:** by default the first scan and a subscriber added later are sent every existing file ([Q307](../../R1.1/resolved.md#Q307)); `replay:false` and `baseline:true` send nothing for existing files; `max_per_scan` caps one scan and `status` shows the backlog
  - Mutants (each must compile and fail on an assertion): send deletes while the guard is held; skip the `dir` symlink and root refusals; write the state in place instead of rename; mark tracked before the reply; drop the cookie pairing; ignore the overflow; drop the lock; ignore `state_max`


- ✅ FS.2 FileScan follow-ups not built in FS.1: the single `renamed` change from kernel cookie pairing, the bigfile opt-in for a file over the inline limit, and the RAG form sending the file path as `source`
  - State: ✅ the RAG form sends the file path as `source`, which RAG 0.3 requires (`d54821d5`), and confirms an upsert only by the stored record that echoes it (`0f5340c3`), both 1.0.48; the bigfile opt-in is built (`sub add … bigfile`, the file's SHA-256 as the bigfile name, `bigfile_ttl`); the single `renamed` change is built (kernel cookie pairing in the watch; a poll-only rename stays a delete plus a create, and `as rag` keeps delete then upsert, Q305)

- ✅ FS.3 On-demand sync and large files in RAG (T-48): `mode` `demand` and the `sync` call ([plan](../filescan.md#on-demand-sync)), RAG's completeness check before any resend on both paths, and a Markdown file over 32 KiB sent by [bigfile reference](../rag/large-files.md#the-upsert-by-reference)
  - State: landed `a254f562` (2026-11-06; readers `#ab-qa2`, `#ab-w2b`); guide [on-demand sync](../../../docs/agents/filescan.md#on-demand-sync), [when RAG's answer is lost](../../../docs/agents/filescan.md#when-rags-answer-is-lost), [large files in RAG](../../../docs/agents/filescan.md#large-files-in-rag); `change_sync.deliver` gains a `settle` hook and per-send call options
  - Acceptance, against a fake RAG (`test_sync.py`, `test_change_sync.py`): a capped or timed-out sync ends incomplete with counts; failed outranks capped; an unconfirmed send is unknown and goes again only with `resend:true`; a concurrent sync is refused busy; nothing stays running; marks resume per target and `reset` clears them; a removal counts only on ok or `not_found`, under the mass-delete guard; under the watch a lost answer goes again only when the check proves it not stored whole, at most three times per version, a count a restart keeps; a large file is only checked until its 30-minute deadline, across a restart too; files in flight past one batch never hold up a later file
  - Mutants: cap counted as success; unknown counted as success; an unconfirmed removal counted done; a watch resend without the check; the settle hook skipped in `change_sync`; the in-flight note not persisted
  - Limit: FileScan cannot read RAG's direct answer to a large upsert, so a refusal shows only through the check after the deadline; the client change that would read it is a follow-up

## DBSync

Owner, 2026-11-02; approved 2026-11-05 ([Q334](../../R1.1/resolved.md#Q334)); plan in [dbsync](../dbsync.md#steps-and-estimate), answers [Q314–Q320](../../R1.1/resolved.md#Q314). Stage S3, after the shared core and FileScan; MySQL and PostgreSQL first.

- ✅ DS.1 `#dbsync/<name>`: a runner agent that tracks new and changed rows of one table by an `updated` column, asks through a read-only [DB agent](../../../src/agents/db/guide.md#what-it-does), and hands the rows to subscribed agents; steps D0–D3 with an estimate of four to five hours of agent time
  - State: built in 1.0.37 (`23cf6ff3`), proven on the fake DB agent and the MariaDB and PostgreSQL fixtures; readers `#ab-w1c`, `#ab-w1b`. A live run waits for the owner: #db is not deployed on this node
  - Depends: the DB agent (built); the S1 shared core library from [FS.1](#filescan) ([Q314](../../R1.1/resolved.md#Q314))
  - Deploy: a kept process serving itself with its own token, handed over once like FileScan's; it holds no database credential, which stays in the read-only DB agent
  - Follow-ups, outside this stage: ClickHouse ([Q319](../../R1.1/resolved.md#Q319)), the key-diff way to notice deletes ([Q316](../../R1.1/resolved.md#Q316); soft delete is what S3 builds), and a counter mode for `updated` ([Q320](../../R1.1/resolved.md#Q320))
  - Acceptance, each against the DB fixture and a fake subscriber, never the live bus:
    - **Cursor:** three rows sharing one `updated` value with `batch` of two are all sent, once each; a cursor on `updated` alone fails this
    - **Paging:** an uncut short page ends the poll; a full page, or a cut whose scan position advanced, triggers the next query at once from the last raw pair received; the page is `max_rows` minus `ceil(max_rows / 10)`, recomputed each poll; `max_rows` 1 is refused with an alert; `max_rows` lowered after a cursor is saved gives a smaller page and the same saved cursor
    - **Known rows ahead:** `max_rows` 2 (page 1), the first lookback row already delivered and a later row new: the poll pages past the known row, delivers the new row exactly once, raises no refusal, and does not read the first page twice
    - **Partial byte cut:** page 9, the DB agent returns 2 rows and a cut; the next query continues after the 2nd raw pair, also when both rows are already known, and nothing is read twice or skipped
    - **No progress:** a fixture row larger than the DB agent's byte cap returns a cut with no row; DBSync alerts and stops, the scan position does not move, and a second poll does not loop; a page of only known rows is not this case; removing the refusal makes this check hang or advance past the row and fail on an assertion
    - **Late commit:** a row committed after a newer one with an older `updated`, inside `lookback`, is sent once; outside it, it is not
    - **Type guard:** a text key, or an `updated` that is neither numeric nor timestamp, makes configuration load and `sub add` refuse with the named error that counts the failing columns (1 of 2, 2 of 2); a numeric key with a timestamp `updated` passes, and a numeric `updated` with a declared unit (epoch seconds or milliseconds) passes; a plain counter or version number is refused with the named error; any numeric key passes; a name outside the identifier pattern is refused; the statement text never carries a quote from a cursor value
    - **Lookback:** a stale cursor older than the floor starts at the cursor; a recent one starts at the floor by the database's clock, not the agent's (a skewed agent clock changes nothing)
    - **Read only:** every statement the agent sends is a read, and a write attempted through its tier is refused by the database
    - **Authority:** an author without may-send to the DB agent is refused at `sub add` and before a forward
    - **Resend:** a refused or unanswered send stays pending and is sent again; a confirmed one is not
    - Every ⚠️ part (the statement builder, state writes, authority) carries a mutant that fails on an assertion
