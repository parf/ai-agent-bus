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

Checked and sound (L): webhook inbound (Basic before body, signature presets, bounds, header refusal, push validation, masking, no redirects), fetch policy (denylist, pinned dial, redirect re-authorisation, header strip), autopull, IMAP wire and SMTP guards, telegram redaction, envfile.py vs envfile.ts (4000-case fuzz, 0 mismatches), voyage provider_http redaction, eventlog, sms-telnyx endpoint guard, package-tree.sh rollback. Not checked in depth: slack forwarding, rag, logwatch, tmux-sessions, cron/at, folder_watch.
