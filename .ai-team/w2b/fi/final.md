
# Issues — Nov 1, 2026 · 30215f6..570d0064

Range review, `30215f6..570d0064` (1536 commits), 16 read-only reviewers in two passes, each finding re-read at HEAD. J1 is first because it lets an injected message reach the user's files without a prompt; `Plans/subsystems.md` gained `WRITING.md` in row N (it matched no row). The T-24 version-rule branches are not on main and are not reviewed here.

- 🔴 J1 `src/launchers/launcher.ts:452` the pre-allowed `mcp__plugin_agent-bus_agent-bus__*` covers `ab_bigfile_put/get`, which take model-chosen local paths with no prompt
  - Evidence: `launcher.ts:452` passes `--allowedTools mcp__plugin_agent-bus_agent-bus__*`; `mcp/bus.ts:151-167` `bigPut` reads any `input_path` the user can read, `bigGet` renames peer bytes over any `output_path` (no path restriction); tools at `mcp/server.ts:211-214`; no earlier tool in the wildcard touched the filesystem
  - Impact: an injected peer message (bodies and bytes are untrusted) can make a session upload `~/.ssh/*` to a readable record or overwrite `~/.claude/settings.json`, `~/.bashrc` or a hook, with no approval, in every `ab-claude` session
  > 💡👾 List the other tools by name so the four bigfile tools ask for approval, or confine both paths to the cwd or a named directory and refuse symlinks.

## Registry core

- 🟠 A1 `src/internal/core/big.go:70` `bigStore` runs the bigfile reopen (disk walk, RemoveAll, DB scan) holding the global `b.mu`
  - Evidence: `bigStore` calls `b.bigReopen()` between `b.mu.Lock()` and `b.unlock()`; `agent-busd/bigkeys.go` makes it `openBigKeys` → `bigfile.Open` (`refuseLinks`, `RemoveAll(tmp)`, `reconcile` with `BigRows()`, `bigfile.go:86-146`); `pruneBigOrphans` also deletes files under the lock
  - Impact: with a broken bigfile directory, a bigfile call (once a minute at most) on a slow or hung disk stalls every registry call
  > 💡👾 Take `bigTried` and the callback under the lock, run the reopen unlocked, re-lock to install the store and prune.
- 🟠 A2 `src/internal/core/transfer.go:362` import preflight reads every existing record's KV values under `b.mu`, then uses only credentials
  - Evidence: `ImportRecords` passes all `b.records` IDs to `ExportState`; `sqlite/transfer.go:6-30` selects every KV table per id; `state.KV` is read only at line 122 (export), import uses `state.Credentials` (366-372)
  - Impact: `agent-bus-admin import`, even `--dry-run`, stalls every bus call while all stored KV loads, and copies every private KV value into the daemon heap; cost grows with total KV
  > 💡👾 Pass `nil` ids at line 362, or add a credentials-only read to `TransferStore`.
- ⚪ A3 `src/internal/core/bus.go:1893` a second unfiltered reader refused with `ErrTwoReads` still has its host listed as a reader
  - Evidence: probe (deleted): first reader from `real` waits, second from `stray` gets `ErrTwoReads`, `Lookup` lists `[real, stray]`; `noteHost` and `defer touchHost` run before the return (`bus.go:1893-1896` vs `:1937-1943`); tests cover only `ErrNotAllow`
  - Impact: a stray CLI reading a runner's inbox from another host shows as a pool member for 2 minutes; routing unaffected
  > 💡👾 Note the host only once the read is accepted, and add an `ErrTwoReads` case to `TestARefusedReadersHostIsNotListed`.
- ⚪ A4 `src/internal/core/snapshot.go:598` the alert text hardcodes "65536-byte" instead of `maxConfigBytes`
  - Evidence: `bus.go:2205` defines `maxConfigBytes`; `configSize` formats it into its own message
  - Impact: none today; wrong limit in the alert if the constant changes
  > 💡👾 Format `maxConfigBytes` into the message.

## Authentication and keys

- 🟠 B1 `src/internal/tokencmd/token.go:233` `prove` signs the namespace and nonce the server sends, not `protocol.SigNamespace` — a hostile endpoint gets a signature oracle
  - Evidence: `/enrol` reply is unmarshalled into `ask{Name,Nonce,Namespace}`, then `keyproof.Sign(key, ask.Namespace, ask.Nonce)` runs `ssh-keygen -Y sign -n <namespace>` unchecked (`keyproof.go:23-30`); only the client trusts the reply (daemon sends and verifies the constant: `api/server.go:746`, `sshkeygen.go:44`)
  - Impact: `agent-bus token --key` against a wrong, MITM or `http://` node gives it a valid sshsig (any namespace, any message) from the user's key, usable outside the bus; rare
  > 💡👾 Ignore the server's namespace and sign `protocol.SigNamespace`; refuse a reply whose namespace differs and an empty nonce.
- 🟠 B2 `src/internal/tokencmd/token.go:109,121` with `--key`, `--rotate` is silently dropped, the old token stays valid
  - Evidence: `issue` calls `prove(name, key)` when key != ""; `prove` has no rotate parameter and the `/enrol` bodies carry none; usage lists both options as independent; `agent-bus token x --key=f --rotate` exits 0 and prints the unchanged token (`tokens.go:132-148`)
  - Impact: someone rotating a leaked credential by key proof gets the existing token back, exit 0, and believes it is revoked
  > 💡👾 Refuse `--rotate` together with `--key`, or send rotate through `/enrol`.
- 🟠 B3 `src/internal/tokencmd/token.go:171` `--key` takes the next argument as its file, so `--key --rotate` or `--key alice` drops the flag or name
  - Evidence: scratch test on `parse`: `[--key --rotate alice]` gave rotate=false, key="--rotate"; `[--key alice]` gave name="", key="alice"; help documents only `--key=file`
  - Impact: `agent-bus token --key --rotate` silently loses the rotate; `--key #bot` reads the name as a path
  > 💡👾 Accept only `--key=file` plus bare `--key`, or have the space form skip a next argument that starts with `-` or is a valid name.
- 🔵 B4 `src/internal/auth/import.go:30` `CommitImport` refuses a token swap between two imported names against the live index
  - Evidence: conflict check compares each imported token to live `t.who` for any other name, without excluding names this import also replaces; no test case
  - Impact: rare; an explicit import that reassigns a token between names fails atomically with "conflicts with <other>"
  > 💡👾 Skip `t.who` hits whose owner is also a key in `values`, since `discardLocked` removes them first.

## Storage and logs

- 🟠 C1 `src/internal/store/sqlite/sqlite.go:561` importing an empty-string KV value aborts the whole import with `NOT NULL constraint failed`
  - Evidence: probe: `KVEdit` stored an empty string, `ExportState` returned it with `Bytes` nil; `ImportKV` of it gave `NOT NULL constraint failed: kv.value (1299)`; `kvArg` (`kv.go:42`) maps nil to `[]byte{}`, the import path does not; `core/transfer.go:485` accepts any length up to `KVMaxValue`
  - Impact: an export holding one empty-string KV value cannot be imported, whole or in part; `kv_json` values also land as BLOB where `kvArg` writes TEXT
  > 💡👾 Use `kvArg(entry.Kind, entry.Value)` in the `ImportKV` loop and add an empty-string and a JSON case to `transfer_test.go`.
- ⚪ C2 `src/internal/store/bigfile/bigfile.go:294` `Sweep` drops the error from `expire`, and `BigRecords` (`:576`) drops `BigRows` errors
  - Evidence: `Sweep` returns nothing, its only caller `cmd/agent-busd/bigkeys.go:80` cannot see it; `rows, _ := s.idx.BigRows()`
  - Impact: on a database fault, expired bigfiles stay and the start-up orphan check (`core/big.go:194`) sees no records, silently; rare
  > 💡👾 Make both return the error and log it at the call sites.
- ⚪ C3 `src/internal/store/bigfile/bigfile.go:279,302` every bigfile call runs a full-table expiry transaction and `usage` scans every row, under the global `s.mu`
  - Evidence: `BigPut/BigGet/BigList/BigDelete` call `expire` (Begin, SELECT, DELETE on `expires <= ?`, no index on `expires`, `sqlite.go:54`); `BigPut` loads all rows via `BigRows()`; one connection (`SetMaxOpenConns(1)`, `sqlite.go:130`); not timed
  - Impact: each bigfile read queues behind and delays other daemon commits; small while caps hold (100 names per record, 1 GiB total)
  > 💡👾 Skip the sweep when the in-memory `min(expires)` is in the future; leave expiry to `Sweep`'s timer.
- ⚪ C4 `src/internal/store/bigmem/bigmem.go:1-3` stale package comment; bigmem plus `MemoryIndex` copy the bigfile limit logic outside production
  - Evidence: the comment says the file holder "follows in the same release" though `bigfile` landed; only `core/big_test.go` and `clients/fixture/main.go:38` use bigmem; `bigmem.BigPut` repeats `bigfile.go:381-401`
  - Impact: the client conformance fixture always reports `Held: "memory"`; a bigfile limit change will not show in client tests
  > 💡👾 Point the fixture at `bigfile.Open` with `NewMemoryIndex`, delete `bigmem`, or fix the comment and mark it test-only.

## Parsers and wire

- ⚪ D1 `src/internal/core/transfer.go:152` import `--rename-realm` checks realms via `ParseName("fixture@"+r)`, not `ValidRealm`, so it caps them at 56 chars, not 62
  - Evidence: probe: a 60-char realm gives `ValidRealm` nil, `ParseName("fixture@"+r)` "is 68 characters: at most 64"; `rewriteBackup` returns "invalid realm mapping"; `protocol.ValidRealm` (`name.go:208`) is used in `bus_settings.go:57,119`
  - Impact: an owner whose realm is 57–62 chars cannot map a backup to or from it; rare, loud refusal
  > 💡👾 Validate each realm in `rewriteBackup` with `protocol.ValidRealm`.
- ⚪ D2 `src/internal/protocol/tags.go:41` the HTTP tag filter accepts empty segments the CLI refuses: `?tag=,` lists nothing, `?tag=a,,b` reads as `a,b`
  - Evidence: `NormalizeTags` skips `word == ""`; `TagFilter(",")` gives no labels and `none=false`, so `listRecords` matches nothing with 200; `launchers/local.test.ts:512` makes the CLI refuse `review,,billing`; `03-records.md:238` says an empty value is refused by name
  - Impact: a raw API or #bus caller with a stray comma gets a silent empty or wrong listing; low frequency
  > 💡👾 In `TagFilter`, refuse an empty segment, and add the case to `ls_tag_test.go`.
- ⚪ D3 `src/internal/protocol/tags.go:67` `SyncTags` discards `NormalizeTags`' error, so one bad tag silently empties the record's tags
  - Evidence: probe: `Record{Tags:["ok","bad tag"]}.SyncTags()` leaves `Tags == []`; callers `bus.go:594`, `manage.go:652`, `snapshot.go:305`, `transfer.go:286` validate first; `bus.go:776`, `manage.go:937`, `snapshot.go:313`, `daemon_agent.go:22` also drop the error, after validation
  - Impact: none today; a future unvalidated caller loses all tags with no error ("config errors fail loudly")
  > 💡👾 Make `SyncTags` keep the input tags on error, or return the error.

## HTTP API

- 🟠 E1 `src/internal/api/server.go:322` a fixed 30 s body deadline cuts any bigfile put slower than ~1.1 MB/s (cap 32 MiB)
  - Evidence: `bodyDeadline` sets one absolute `SetReadDeadline(now+bodyWait)` cleared only at EOF; probe of that code: 1 KB every 0.4 s against a 1 s deadline gave `i/o timeout` after 3 KB; `bigPut` reads inside it, `badBody` returns 408
  - Impact: a 32 MiB put over TCP/TLS from a remote or slow host fails with 408; `Dial` (`server.go:44`, `Timeout: 2 * time.Minute`) also caps a whole put or get at 2 min; local sockets unaffected
  > 💡👾 Exempt PUT /bigfile from the absolute deadline or extend it per Read; give bigfile calls a client without the 2 min `Timeout`.
- ⚪ E2 `src/internal/api/server.go:812` `GET /ls` with a bad `tag` answers 400 via `answer()`, so the refusal is never counted
  - Evidence: only `answer` is called, not `s.refuse`, the sole caller of `bus.Refuse`; `docs/05-discovery.md:319` says Refused counts every endpoint refusal, malformed included
  - Impact: a node whose clients send bad `--tag` shows zero "malformed" refusals on status and dashboard
  > 💡👾 Use `s.refuse(w, 400, "malformed", ...)` in `ls`.
- ⚪ E3 `src/internal/api/server.go:268` `HEAD /consume` takes a message the client never sees
  - Evidence: Go `ServeMux` matches `GET` patterns for HEAD; probe: `http.Head` on a `"GET /consume"` mux returned 200 and ran the handler; `requestQuery` already maps HEAD to GET
  - Impact: a health check probing with HEAD and a valid token drops one message per probe; rare, authenticated only
  > 💡👾 Refuse HEAD (405) at the top of `consume`.

## Runner and process boundary

- 🟠 F1 `src/cmd/agent-bus-runner/install.go:47` install and remove hold `reloadMu` across a 2-minute git fetch and a 2-minute `package.sh` build, which stalls the runner's single inbox loop
  - Evidence: `installPackage` takes `r.reloadMu` across `fetchDeploy` and the build; `control()` (`control.go:20`), `readFile`/`writeFile`/create/delete (`files.go:211,295,329`), `reload()` and `reloadStatus` (`reload_ready.go:18`) take the same mutex synchronously in `handle()`, which `run()` calls inline (`main.go:186-206`); only install, remove and reload run in goroutines; read, not run
  - Impact: a slow or hung git remote or build during an owner `install` freezes `status`, `check-deploy` and every control call, `stop` and `disable` included (the emergency-stop path), for up to ~4 minutes with no sign of why
  > 💡👾 Fetch and build into staging before taking `reloadMu`; hold it only for the checks, rename, inventory write and `apply`; or answer controls in goroutines like `install`.
- 🟠 F2 `src/cmd/agent-bus-runner/overlay.go:41` every load wipes `runner/.merged/<key>`, so an install staging check destroys the live agent's mounted override files
  - Evidence: `install.go:~134` calls `loadSources` on the staging dir, which calls `shippedOverlays` with the staged `agentDir`, and that does `os.RemoveAll(merged)` for the live key first; scratch test (removed): after a live merge, `shippedOverlays` on a staged package lacking the shipped file returned "does not ship" and the live `Overlay.Src` file no longer existed
  - Impact: `install --replace` of an agent with `runner/<agent>/<file>.json` overrides, when the package changes or drops that file, fails the staging check and leaves the live merged file gone; the next per-message start or crash restart fails `--ro-bind` on a missing source
  > 💡👾 Build merged files into a staging dir and rename into place only on success; have the install staging load skip overlay writes (digest only).
- 🟠 F3 `src/cmd/agent-bus-runner/jsonl.go:215` an answer line over ~1 MiB ends `answers()` silently; the child is neither read nor killed until a deadline
  - Evidence: `answers()` stops at the Scanner's `ErrTooLong`, never checks `lines.Err()`, logs nothing; `loop()` blocks in `cmd.Wait()` on the live child, which stays in `children`; scratch test (removed): a jsonl child printing `id <2 MB line>`, `st.do` with a 3 s context returned only at 3.000 s with deadline exceeded; the one-shot path cuts with a notice
  - Impact: one oversized answer from a kept jsonl agent (an LLM or RAG answer) loses that message and every other message in flight on the process; later ones wait for their deadline with no diagnostic
  > 💡👾 On a non-nil `lines.Err()` log it and `st.kill(c)`, or read the rest of the line and drop it, as the one-shot path does.
- ⚪ F4 `src/cmd/agent-bus-runner/install.go:106` the bundled `package.sh` build runs with the runner's full environment, including `AGENT_BUS_TOKEN`
  - Evidence: `exec.CommandContext(buildCtx, "/bin/sh", script, staged)` leaves `cmd.Env` nil; the comment trusts the host's packager, true for a release bundle, but in a git checkout `package.sh` is whatever the tree holds; the timeout kills only the shell, not its children
  - Impact: a dev node running install on a checkout hands the runner credential (management authority) to any build step, bun or npm lifecycle scripts included
  > 💡👾 Run the packager with a minimal env (PATH, HOME=staging temp) and a process group killed on timeout.
- ⚪ F5 `src/cmd/agent-bus-runner/overlay.go:79` the override merge round-trips shipped JSON through `any`, so integers above 2^53 change and key order is lost
  - Evidence: probe (removed): shipped `{"id":9007199254740993,"u":"a<b&c"}` plus override `{"k":1}` gives `{"id":9007199254740992,"k":1,"u":"a<b&c"}`; a shipped `1e400` is refused as "not JSON"
  - Impact: a package shipping a large integer in an overridable JSON file gets a silently different value once the host overrides any key; rare
  > 💡👾 Decode with `json.Decoder.UseNumber()` and `SetEscapeHTML(false)` before merging.
- ⚪ F6 `src/cmd/agent-bus-runner/reload_ready.go:50` the reload wait polls `/lookup` every 20 ms per pending agent for up to 15 s
  - Evidence: loop `reload_ready.go:36-70` issues one authenticated `GET /lookup` per pending name per 20 ms tick, no backoff
  - Impact: a reload of many kept agents with one slow starter sends ~50×N daemon requests a second for 15 s; bounded but noisy
  > 💡👾 Back off the tick (20 ms doubling to 250 ms), or look up all pending names in one call.
- ⚪ F7 `src/cmd/agent-bus-setup/runner_install.go:332`, `src/cmd/agent-bus-setup/user_runner.go:747`, `docs/runner/getting-started.md:36` the registered runner `descr` omits install and remove
  - Evidence: the runner's refusal text (`main.go:561`) and `docs/runner/operations.md:122-125` list `install` and `remove`; the descr strings and the `ls` example stop at "create, delete"
  - Impact: `agent-bus ls` and `help` discovery for a runner do not mention the deploy calls; cosmetic
  > 💡👾 Add install and remove to both descr strings and the getting-started example.

## Setup, install and release

- 🟠 G1 `src/agents/db/requirements.txt:3` db pins carry hashes for one wheel (cp314, manylinux x86_64), so `package.sh` can fail on other Pythons and CPUs
  - Evidence: the file's own comment says the hashes cover the cp314 manylinux x86_64 `psycopg-binary` wheel only; `package.sh` runs `pip install --only-binary=:all: --require-hashes`; `build.sh:54` calls `agents/package.sh` for every archive; `release.yml:16` builds on `ubuntu-24.04` and `ubuntu-24.04-arm`; pip not run (needs the index)
  - Impact: a tagged release likely fails at "Archives" on a runner whose Python is not 3.14 or whose CPU is aarch64; any such node cannot build the db agent
  > 💡👾 Pin hashes for each wheel the matrix uses (cp312/cp314 × x86_64/aarch64) or fetch wheels per target; add a `package.sh` check against a non-cp314 pin file.
- ⚪ G2 `src/cmd/agent-bus-setup/main.go:353,531,537,760,834` root setup still runs `systemctl`, `getent`, `ldd` by bare name, though `user_runner.go` moved to absolute `tool()` paths
  - Evidence: c1129422 says "the installer's PATH is never consulted" and adds `tool()`/`toolDirs`; other `exec.Command` calls remain at `main.go:353,531,537,760,834`, `runner_install.go:410-411`, `web.go:51,192,243,340`
  - Impact: a wrong sudo `secure_path` still resolves system tools through PATH; the same root process has two lookup rules
  > 💡👾 Use `tool()` for every root `exec` in setup, or document that only the person-facing calls are pinned.
- ⚪ G3 `src/cmd/agent-bus-setup/user_runner.go:638` an error from the registry listing is swallowed, so `uninstall` can delete the shared user template
  - Evidence: `observeUser` reads `if people, err := userRecords(...); err == nil { ... st.others++ }`; on error `others` stays 0; `userPlan` (line 349) removes `/etc/systemd/user/agent-bus-runner.service` when `st.others == 0`; every other registry call in `observeUser` returns its error
  - Impact: rare; when it fires, one person's uninstall removes the template every other person's runner uses, and they fail at their next restart
  > 💡👾 Return the `userRecords` error from `observeUser`, as the other lookups do.
- ⚪ G4 `src/cmd/agent-bus-setup/main.go:46`, `src/maintenance/agent-bus.logrotate:1` two logrotate rules for one log set, and setup rewrites the node's own
  - Evidence: setup's `logrotateConf` (weekly, 8 archives) is written to `/etc/logrotate.d/agent-bus` by `installLogrotate` (`main.go:410`) on every fresh install and `--reinstall`, which `git-install.sh --reinstall` calls; the dev rule (daily, maxsize 10M, rotate 3) is a tracked file no script installs, `docs/07-setup.md` has the operator copy it by hand; the live file is the dev rule today
  - Impact: a reinstall on the dev node silently drops the 10 MiB early rotation and the 3-archive bound; logs grow for a week and the docs' disk-bounds table is wrong
  > 💡👾 Have `git-install.sh --reinstall` reinstall the maintenance rule, or give setup one rule both paths read.
- ⚪ G5 `src/maintenance/node-cleanup.py:110` the root run aborts when a bounded directory is absent, skipping every step after it
  - Evidence: `spool.iterdir()` (110), `releases.iterdir()` (118), `backup.iterdir()` (121) are unguarded; only `retain_record_backups` checks `exists()`; a missing `/var/spool/abrt` raises `FileNotFoundError` before the release, backup and log steps; the `finally` then runs `release ... check=True`, which can mask it; both directories exist here, so latent
  - Impact: a node without abrt, or without the daemon backup directory yet, never gets the weekly bounds applied; the timer's service reports a failed run
  > 💡👾 Guard each `iterdir()` with `exists()` as `retain_record_backups` does, or run each step in its own try block.
- 🟠 Q1 `src/container/Containerfile:34` the image unpacks the release as root keeping the builder's uid/gid and modes, so the release tree is not root-owned
  - Evidence: the Containerfile runs `tar -xzf "$archive"` with no `--no-same-owner`, and `src/package.sh:58,76` run `tar -czf` with no `--owner=0 --group=0 --numeric-owner`; GNU tar as root restores archive ownership by default (a probe archive with `--owner=1234` lists `1234/1234`); `install.sh` uses `--no-same-owner`; the `chmod -R go-w` covers only `agent.d`
  - Impact: every image build; `/usr/local/lib/agent-bus/releases/<v>` is owned by the builder uid, which is no account in the image, so a container run with that uid can replace the daemon and runner binaries; a builder umask of 077 would make them unreadable to uids 990 and 991
  > 💡👾 Pass `--no-same-owner` in the Containerfile, add `--owner=0 --group=0 --numeric-owner` in `package.sh`, then `chown -R root:root` and `chmod -R go-w` the release tree.

## MCP face and launchers

- ⚪ J2 `src/mcp/README.md:3` says "fifteen tools" and lists no `ab_bigfile_*` tool; the face has 19
  - Evidence: `tools.yaml` has 19 `ab_` entries, `mcp/smoke.ts:80` expects 19; no `ab_bigfile` hit in `src/mcp/README.md` or `docs/user/ai-cli.md:81-94`; same shape as closed J6
  - Impact: a user wiring the face by hand is told the wrong count and never learns the bigfile tools exist
  > 💡👾 Say "nineteen tools", add the `ab_bigfile_put/get/ls/rm` row to both docs, drop the count from the TL;DR.

## Client libraries

- ⚪ K1 `src/clients/go/bus.go:499` a negative `WithWait` or `WithTTL` is silently left out of the send; the other five clients refuse it
  - Evidence: `Send` writes `wait`/`ttl` only `if > 0`; Python `transport.py:22` and TS `transport.ts:26` refuse `< 0`; `clients/README.md` "A send's wait" says only zero means no wait
  - Impact: a Go caller with a computed remaining time gone negative sends with no deadline or lifetime; rare
  > 💡👾 Refuse `< 0` with an error in `Send`; drop only an exact zero.
- ⚪ K2 `src/clients/README.md:5` says the six libraries "behave the same way", but bigfiles exist only in Go, Python and TS
  - Evidence: `grep -ci bigfile` on READMEs: python 8, go 2, rust 0, php 0, java 0; `Plans/R1.0-Release/bigfile.md:4`; `may-send` (Python, TS) and `asks: 'only'` (TS) are also absent from the hub
  - Impact: a Rust, PHP or Java author expects `record.bigfile` and finds nothing
  > 💡👾 Add a one-line per-language support note for bigfile, may-send and asks-only to "What you get".

## Bundled agents

- 🟠 L1 `src/agents/db/db.py:149,185,235` MySQL, PostgreSQL and ClickHouse connect with no TLS and no warning, and Configuration `host` may be any address
  - Evidence: `run_mysql` has no `ssl`; `run_postgres` sets no `sslmode` (psycopg `prefer`, downgradable); `run_clickhouse` sends `X-ClickHouse-Key` over plain HTTP unless `secure` (default off); `docs/agents/db.md` says nothing about encryption; `imap/wire.py local_host` and `smtp/smtp.py:177-180` refuse plaintext off loopback, `qdrant/qdrant.py:157` warns
  - Impact: an instance pointed at a non-loopback database sends the password and every result row in clear, on every call
  > 💡👾 Add `sslmode=require` and an `ssl` option for MySQL, and refuse or warn on a non-loopback host without TLS, as `smtp` and `qdrant` do.
- 🟠 L2 `src/agents/webhook/webhook.py:132-141` a transient bus error reading Configuration or Secret at start leaves the webhook failed until a manual reload
  - Evidence: `except (BusError, Invalid, UnicodeError): cfg, secret = None, b''` → `configuration(None)` raises `Invalid`, sets `problem`, opens no listener; `main` goes on to `bus.serve` and exits 0, so the runner never restarts it; `telegram.ts` exits 1 on a KV failure at start
  - Impact: a daemon hiccup (3 s `Bus(timeout=3)`) during runner start or reload disables every inbound route, visible only in `status` and `error:` replies
  > 💡👾 Let `BusError` from the config or secret read end `main` with exit 1; keep `Invalid` as a standing `failed:` status.
- ⚪ L3 `src/agents/discord/gateway.ts:24,64` reconnect backoff resets only on READY, never after a successful RESUME
  - Evidence: `again()` doubles `backoff` to 30 s; only READY sets it to 1000; `gateway.test.ts:11-28` covers RESUMED delivery, not backoff
  - Impact: after ~5 resumable drops every reconnect waits 30 s until a fresh IDENTIFY
  > 💡👾 Reset `backoff` to 1000 on `RESUMED` too and add a gateway test.
- ⚪ L4 `src/agents/voyage/voyage.py:25` bare-text `embed` of any text starting with `{` fails with a generic JSON error
  - Evidence: `request_bytes('{x} is a set', {})` raises `JSONDecodeError`, answered "invalid request, Configuration or provider response"; `embed {x} is a set` works
  - Impact: callers sending brace-led raw text (code, templates) get a misleading error; low frequency
  > 💡👾 On a JSON parse failure of a brace-led body fall back to the text form, or name the `embed ` prefix in the message.

## Current docs

- 🟠 N1 `docs/05-discovery.md:352` the `/identity` closed list says "nothing else", but the daemon also publishes `protocol` and `protocol_min`
  - Evidence: `protocol/node.go:12-13` and `api/identity.go:27` send both; `api/node_test.go:59` expects 8 fields; `docs/05-discovery.md:351-360` lists release, host, owner, uptime, calls, build; no text in docs/, Plans/ or README.md owns the two fields (the T-24 docs on `ab-w1b/T-24-D1` are not on main yet; drop this item if they land)
  - Impact: client authors and the unauthenticated-disclosure record have no canonical text for the two fields or when each rises
  > 💡👾 Add both to the 05-discovery "what is published" row and the 09-daemon-api `/identity` row, saying when each rises.
- ⚪ N2 `src/web/web-face/node.md:208` the fallback tile reads `status.services`, which the daemon no longer sends (removed in 0.9.264)
  - Evidence: CHANGELOG 0.9.264 lists "removed … status.services"; `src/web/pages/node.tsx:96-97` and `ctx.ts:28` render only `kinds`; no `status.services` use remains under `src/web`
  - Impact: a maintainer rebuilding the node page from this spec adds a dead fallback
  > 💡👾 Delete the "If `kinds` is empty, one tile Records = status.services" clause and say what the page shows when `kinds` is empty.
- ⚪ N3 `docs/04-messaging.md:273` the envelope `ask` row omits `status`, which Send accepts
  - Evidence: `bus.go:1373`, `ErrAsk` (`bus.go:48`) and `protocol/envelope.go:36` list it; `docs/constitution.md:280`, `docs/06-dedicated-runner.md:145` list it
  - Impact: a client builder reading the owning table is told `status` is refused
  > 💡👾 Add `status` to the ask list in that row.
- ⚪ N4 `docs/user/cli.md:155` says `--tag=<tag>` narrows to "one label", but the filter takes several and keeps records carrying all
  - Evidence: `docs/03-records.md:238` (canonical) describes comma-delimited and repeated `--tag` as AND, `no-tags` refused beside a real tag; `protocol/tags.go:35-63`, `api/server.go:821-838`
  - Impact: a CLI user does not learn `--tag=a,b` works, or that `no-tags` filters untagged records
  > 💡👾 Reword to "`--tag=a,b` keeps records with every listed label" and link the 03-records tags section.

## Plans

- ⚪ O1 `Plans/issues/done/2026-10/2026-10-20-constitution-check.md:28` a tracked file holds Russian text (owner replies quoted at lines 28, 41, 56)
  - Evidence: `grep -nP '[\x{0400}-\x{04FF}]'` matches those three lines; the only Cyrillic hit among changed docs, Plans, acceptance, README, WRITING, CLAUDE and .claude files
  - Impact: low; a closed archive file, but it breaks the English-only rule
  > 💡👾 Replace the quoted replies with an English paraphrase of the owner's answer.

Checked and sound:
- Core: reply holds, bigfile authority vs `kvStore`, `#bus` handling (`restoreDaemonAgent`, `mayMapPrincipal`, `pairFor`, `Authenticate`, `Register`, `Consume`), `configureBus` rollback and validation, record transfer Owner-only, iterative group walks, reply hold counters, asks-only reads, Restore tag migration; `go test ./internal/core` green, also under `-race`.
- Authentication: `CommitImport` ordering and rollback, `discardLocked`, forced-SSH entitlement, `--fingerprint` validation, `loginPrincipal` timeout and body cap, no token in error text, `Verify` pins the namespace, `ClientSocket` never falls back to /tmp.
- Storage: backup `Pack`/`Unpack`/`WriteFiles` bounds, `ExportState` single read transaction, schema 9 and migration 8, bigfile write/delete order and reconcile, journal `hostLog` gating, store lock order.
- Parsers: envfile `Parse`/`Describe` under a 300k-case fuzz, `Required` marker, NUL and `$` refusals, line-number-only refusals, `NormalizeTags` charset, `no-tags` sentinel, `ValidRealm`, `Ask` status, `NodeIdentity` field count.
- API and CLI: `decodeRequest`/`checkMembers`, `requestQuery` duplicate checks, auth order, `codes` table, `routeFailure`, `consume` wait and peer-gone requeue, `tlsClient` pin, export/import owner-only, `bigPut` drain, lock args, `locks.Store` FIFO, help OPTIONS against flags, `--help` exit 0, `bigGet` SHA check, `parent_watch`, `migrateBigDir`, `redactWriter`, `secretEnv`; no scratch-daemon probes.
- Runner: git deploy pin and URL check, `validateDeployTree`/`copyDeployTree` containment, publish and rollback order, keeper loop, `signalOne`, stream stop, `secretStderr` masking, `MaskValues`, bwrap `--ro-bind` order and non-bwrap refusal, systemd-run `--setenv=KEY` path, cgroup start and darwin no-op, `reloadStatus`, `orderDepends`, `mergePatch`.
- Install: `install.sh` hash and manifest checks and `--no-same-owner`, `release.yml` and `release-sign.sh`/`release-verify.sh`, Darwin cross-build with CGO off, `userCtlArgv` allowlist, `user_runner.go` privilege drop and token by stdin, `darwin.go` modes, `package_install.go` bundle contract, `agent-bus-admin` transfer sidecar, container entry `setpriv` drop and sandbox refusal, `node-cleanup.py` removal safety, npm `root-trust.cjs`/`setup.cjs`/`pack.py`.
- Web, MCP, clients: web store bigfile card, tag handling, bus-setup page, pagination, realm prefill; Go `big.go`, Python bigfile checks, MCP `bigGet`/`bigPut` checks, `push.ts` supervise, launcher `plugin-steps.ts` lock takeover, `automaticBase`, consume wait clamps; `bun test` in `src/web` (206 pass).
- Agents: webhook inbound (Basic before body, signature presets, bounds, masking, no redirects), fetch policy, autopull, IMAP wire and SMTP guards, telegram redaction, `envfile.py` vs `envfile.ts` (4000-case fuzz, 0 mismatches), voyage redaction, eventlog, sms-telnyx endpoint guard, `package-tree.sh` rollback. Not checked in depth: slack forwarding, rag, logwatch, tmux-sessions, cron/at, folder_watch.
- Docs and Plans: links and anchors in 256 changed Markdown files (0 broken), CHANGELOG head 0.9.264 matches VERSION, `.claude` and CLAUDE.md wording against Verification, `.gitignore`, acceptance `|| true` guards, removed 0.9.264 routes absent from docs, clients and MCP. Not checked: 35 acceptance files in full, smoke shard internals beyond selection.

Counts: 🔴 1 · 🟠 15 · ⚪ 27 · 🔵 1 (44).
