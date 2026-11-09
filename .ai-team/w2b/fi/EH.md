- 🟠 E1 `src/internal/api/server.go:322` a fixed 30 s body deadline cuts any bigfile put slower than ~1.1 MB/s (cap 32 MiB)
  - Evidence: `bodyDeadline` sets one absolute `SetReadDeadline(now+bodyWait)` cleared only at EOF; probe of that code: 1 KB every 0.4 s against a 1 s deadline gave `i/o timeout` after 3 KB; `bigPut` reads inside it, `badBody` returns 408
  - Impact: a 32 MiB put over TCP/TLS from a remote or slow host fails with 408; `Dial` (server.go:44, `Timeout: 2 * time.Minute`) also caps a whole put or get at 2 min; local sockets unaffected
  > 💡👾 Exempt PUT /bigfile from the absolute deadline or extend it per Read; give bigfile calls a client without the 2 min `Timeout`.
- ⚪ E2 `src/internal/api/server.go:812` `GET /ls` with a bad `tag` answers 400 via `answer()`, so the refusal is never counted
  - Evidence: only `answer` is called, not `s.refuse`, the sole caller of `bus.Refuse`; `docs/05-discovery.md:319` says Refused counts every endpoint refusal, malformed included
  - Impact: a node whose clients send bad `--tag` shows zero "malformed" refusals on status and dashboard
  > 💡👾 Use `s.refuse(w, 400, "malformed", ...)` in `ls`.
- ⚪ E3 `src/internal/api/server.go:268` `HEAD /consume` takes a message the client never sees
  - Evidence: Go `ServeMux` matches `GET` patterns for HEAD; probe: `http.Head` on a `"GET /consume"` mux returned 200 and ran the handler; `requestQuery` already maps HEAD to GET
  - Impact: a health check probing with HEAD and a valid token drops one message per probe; rare, authenticated only
  > 💡👾 Refuse HEAD (405) at the top of `consume`.

Checked and sound (E, H): decodeRequest/checkMembers, requestQuery dup checks, auth order and credential extraction, codes table, routeFailure, consume wait/peerGone/tcpPeerClosed, splitTransport/tlsClient pin, export/import owner-only, bigPut drain, lock args; locks FIFO hand-off, DropRecord, help OPTIONS vs only() flags, --help exit 0, parseBigTTL, bigGet SHA check, parent_watch, migrateBigDir, openDatabase, version.Text, ManText, DispatchAsk, redactWriter, secretEnv (no scratch daemon probes).
