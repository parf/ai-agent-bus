# Mail delivery evidence

## B.6 App-password SMTP

Landed 2026-10-28 in `9bfe0926`, shared version **0.9.134**. The later XOAUTH2 increment is below; the owner-approved real-provider check
remains pending. B.5 IMAP is separate. Current use is documented in the
[SMTP guide](../../../docs/agents/smtp.md#what-it-does).

| Check | Result |
|---|---|
| Final normal gate | 1213 passed, 0 failed, 48.703 seconds |
| Focused SMTP fixture suite | 18 passed; loopback TLS only |
| Runner account isolation | Two distinct instance credentials, package-wide file without account credentials; race check passed |
| Runtime packaging | Three setup/export checks passed, including execution outside checkout |
| Docs | RT32 five checks passed; touched-file linkcheck bad 0 |
| Owning mutants | 30 Python and one runner-env omission compiled and failed assertions; restored |
| Installed component | Seven package files byte-match committed source; env.dist has no values |
| Activation | Inventory disabled before package promotion/reload; runner status reports smtp disabled |
| Credential-free help | Installed help-json reports .134, send, docs URL and no credential defaults |
| Deployment check | 6/6 |

The mutants cover recipients/allowlist/domain boundaries, single submission,
TLS verification and upgrade/AUTH ordering, refusal/unknown outcomes, redaction,
From/bcc/CRLF, recipient/message/reply limits, DNS-wide deadline, 7-bit Russian
MIME, Date/EHLO, username preservation, duplicate keys/recipients, body controls,
metadata and account isolation. Initially surviving domain-prefix, alarm and
duplicate-key mutants exposed weak checks; those checks were corrected and the
mutants rerun assertion-red. No surviving or error-only mutant earns credit.

Reviewer `#claude/ab2-review` closed `9bfe0926` after confirming ancestry,
version and identical smtp.py/mail.py blobs to reviewed `86a24101`. Scope:
static source read only, including agent.json/env.dist initially; no test,
docs, wiring, gate or host command execution by the reviewer. Runtime evidence
above is the implementer's.

Deployment changed only the SMTP package and its disabled inventory entry.
No account credentials were read or written, no real SMTP/Gmail connection or
mail submission occurred, and no binary/service restart or push occurred.
Real-provider TLS/AUTH remains unproven until the owner authorizes the Gmail
read/send check.

## B.6 XOAUTH2

Landed 2026-10-28 in `fef18f79`, shared version **0.9.135**. SMTP implementation
is built; live Google/Gmail verification still waits for the owner's account
and explicit approval. Current authentication and limits live in the
[SMTP guide](../../../docs/agents/smtp.md#secret-keys).

| Check | Result |
|---|---|
| Final normal gate | 1213 passed, 0 failed, 49.235 seconds |
| SMTP and OAuth fixtures | 28 passed, loopback TLS only; no Google endpoint contacted |
| Runner account isolation | Two race checks passed, including OAuth vs password accounts |
| Owning mutants | 15 Python, one runner-env omission, one setup required-helper omission; all compiled and failed assertions; restored |
| Deployed component | Eight SMTP files byte-match source; disabled inventory preserved |
| Credential-free help | .135, password/XOAUTH2 modes, five name-only env declarations with no private defaults |
| Deployment check | 6/6 |

OAuth mutants cover grant/path/TLS, duplicate refresh, missing fields, response
byte/length/encoding/type/Bearer bounds, redirect refusal, access-token
redaction, the empty 334 challenge response and authentication refusal. An
initial encoded-response mutant produced a test KeyError, earning no credit;
the check was changed to an explicit assertion and rerun assertion-red.

Reviewer `#claude/ab2-review` closed `fef18f79`, verifying main ancestry,
version and identical smtp.py to reviewed `4475edd3`; the only later helper
hunk gives non-string token_type a named OAuth response error. Review scope:
static mail_oauth.py read plus smtp.py/env.dist/agent.json delta and that hunk;
no tests, docs, wiring or commands run. Runtime evidence above is ours.

No credentials/account Configuration were supplied or read, no real mail,
Google token exchange or Gmail login occurred, and no binary/service restart
or push occurred. Real Google error shapes and Gmail's 334 behaviour remain
unproven until the owner-approved check. Only the SMTP package was refreshed.

## Live app-password send

2026-10-28, owner-approved: instances `#smtp/{difive,eprofit,scienteco,realmo}`
were created from the owner's Thunderbird accounts (host `mail.eprofit.com`,
port 465 TLS; port 587 gave no banner from this host). One message from
`#smtp/eprofit` to the owner's own address was `accepted` and the owner
confirmed receipt. Real TLS and AUTH PLAIN are proven; the Gmail XOAUTH2
check is still pending. Two earlier attempts were refused with 535 because an
unquoted `#` in the password ended the env value, as the
[quoting rules](../../../docs/runner/configuration.md#where-settings-belong)
specify. The runner reads host env files at reload.

## Approved SMTP plan history

The following is the pre-implementation B.6 plan, retained as history. Current
calls and authentication rules are owned by the guide; unfinished IMAP scope
remains in B.5.

<details>
<summary>Reviewed B.6 plan and acceptance</summary>

- ⏳ B.6 SMTP client
  - App-password increment built and installed disabled: [0.9.134 evidence](mail.md#b6-app-password-smtp); XOAUTH2 and the owner-approved real-provider check remain.
  - Builds: the R1 [SMTP client](../agents.md#mail-clients), kept by the runner
  - Rules: [mail clients](../agents.md#mail-clients): STARTTLS and implicit TLS with verified certificates, Gmail with an app password or OAuth2
  - Acceptance: against a recording SMTP server over STARTTLS and implicit TLS, send the requested message to its intended recipient exactly once; a server refusal is reported without exposing credentials
  - Mutant: change the recipient, submit twice, or report a refused submission as successful; each must fail its corresponding check
  - Mutant (IMAP cache): refetch known UIDs, or keep the cache across a `UIDVALIDITY` change; a check must fail
  - Mutant (both): skip certificate verification, or fall back to plain text without the opt-out; a check must fail
  - Gmail: one live read and one live send to the owner's own address, only with the owner's approval
  - Plan (2026-10-28; review required before code): Python stdlib `#smtp`, runner per-message script, default ACL `@owner`; one send method plus metadata help. Send plain-text mail with optional HTML, to/cc/bcc, subject and a configured sender; no attachments in this step. Validate recipient/header syntax and refuse CR/LF injection before connecting. Limit 20 recipients and 1 MiB encoded mail. Keep bcc out of the transmitted headers. Submit once, never retry; report partial recipient refusals and distinguish a definite refusal from an unknown outcome after DATA, so callers do not resend blindly.
  - Review pins (2026-10-28): existing runner per-instance host env files separate accounts without new prefixes or env.dist pattern support: `<host>/smtp/<instance>/env` and `<host>/imap/<instance>/env` override package-wide defaults. Keep account credentials out of the package-wide file when instances differ; test two instances with distinct values and no cross-read. See [host files](../../../docs/runner/configuration.md#where-settings-belong).
  - Relay scope: a grant means “send as this mailbox”; default ACL stays @owner. Optional Configuration recipient allowlist accepts exact ASCII addresses/domains, checked against every to/cc/bcc envelope recipient before connecting. Default is unrestricted recipients for admitted callers; no hidden rate policy or retry. Provider quotas still apply. Test an outside recipient refused with zero connection/submission.
  - Message construction: EmailMessage with SMTP policy, canonical CRLF bytes and no bare CR/LF; reject CR/LF/NUL in all headers and recipient inputs. Each recipient is one bare ASCII addr-spec, no display names, angle brackets, whitespace, multiple addresses or SMTPUTF8/IDNA ambiguity. Sender and From come only from Configuration; no caller From/Reply-To/Sender/arbitrary headers. Message-ID uses a digest of the delivered bus message id and configured sender domain (same delivery id is stable; a new call id is a new send). Mutants cover header injection, bare-LF dot smuggling, caller sender, bcc exposure and 21 recipients.
  - TLS/AUTH: explicit starttls/tls mode, TLS >=1.2, verified default context with hostname checking; EHLO again after STARTTLS, then re-check AUTH capability. No AUTH before required STARTTLS; unsupported selected mechanism is refused. Password uses an explicit supported mechanism, OAuth uses XOAUTH2. On XOAUTH2 334 failure send the required empty response and report a bounded redacted refusal. The local plaintext opt-out never activates from an upstream failure. Tests/mutants pin these transitions.
  - Outcome answers: accepted (server acceptance text/queue id when supplied), definite refusal (including final DATA 4xx/5xx), partial recipients (name accepted/refused recipients), or unknown after DATA may have reached the server (disconnect/timeout). Never claim accepted before final success and never automatically resubmit. Document that Message-ID is not a promise that another server deduplicates.
  - Shared mail authentication/TLS code, separate from provider_http: verified implicit IMAP TLS and SMTP implicit TLS/STARTTLS; require STARTTLS before AUTH and never downgrade on failure. Gmail defaults, app password or XOAUTH2 chosen explicitly. OAuth refresh uses Google's verified HTTPS token endpoint, bounded response, no redirects/retry, and an in-memory expiring access token. Per-message SMTP refreshes once per send; fixture token endpoints are injected by tests only, never a production Configuration redirect for refresh tokens. Refresh errors name a bounded error class without echoing the body. No browser/OAuth consent flow in the agent: the operator obtains the refresh token. Credentials, including OAuth client secret/refresh token, come only from each instance's host runner env, not Configuration or the bus Secret store. env.dist declares names and authentication alternatives; errors/logs redact passwords and tokens, including server echoes. The existing explicit plaintext opt-out is local-server-only and never automatic.
  - Bounds: each operation respects the delivered deadline, capped at 60 seconds, including DNS/connect/TLS/auth/token refresh and protocol I/O, with an overall SIGALRM deadline as well as socket timeouts; bounded response lines, literal bytes and OAuth response (64 KiB). Cancel/close on timeout; no debug protocol traces. Keep upstream refusals as answers and follow the client's receipt rules.
  - Proof: recording TLS IMAP/SMTP and OAuth fixtures on loopback only; SMTP tests both 587-style STARTTLS and 465-style implicit TLS, app-password and XOAUTH2, exact envelope recipients and one DATA submission. Wrong CA/hostname, absent STARTTLS, echoed secrets, partial refusal, disconnect after DATA and deadlines have assertions. Mutants change recipient, duplicate submission, suppress refusal, disable certificate verification, permit unintended plaintext, leak a secret or ignore a bound. No real mail, Gmail connection or provider token request in any test.
  - Delivery order: review this plan with `#claude/ab2-review`; first land shared mail support and app-password SMTP, then XOAUTH2 as its own tested SMTP increment, then IMAP/cache as a separate fresh bumped landing. Coordinate package/setup/acceptance/smoke files with xAB after its R6 release. Each final tree gets owning-test mutants, `src/smoke.sh --normal` under the smoke-gate lock, changelog and a fresh shared patch. Add RT32-structured imap/smtp guides, index rows, manifest docs URLs and copyable fixture-safe examples. Package only changed agents; install disabled with no account or mail connection, verify package contents contain no secrets and help works without credentials, then check-deploy. A later owner-approved Gmail check remains pending; no push.


</details>

## B.5 IMAP

Landed 2026-10-28 in `d32d429d`, shared version **0.9.136**, after rebasing
onto `65bfd9a2`. Source was actually fast-forwarded and main ancestry checked.
[The guide](../../../docs/agents/imap.md#what-it-does) owns current calls,
cache rules, credentials and external publication; [RD136](../DECISIONS.md#recorded-decisions)
settled the publication format.

| Check | Result |
|---|---|
| Final normal gate | 1214 passed, 0 failed, 48.936 seconds |
| Focused IMAP suite | 41 cache, parser, TLS/protocol and exported-runtime checks passed |
| Owning mutants | 47 Python plus Go kept-reader and required-helper omissions; all 49 compiled and failed named assertions, then restored |
| Loader/runtime | Exported package selects one self-serving sandboxed reader; a real fixture bus receives exact warning + raw HTML/instruction text from that identity |
| Dependencies | Three package/setup/export checks passed, including outside-checkout use |
| Docs | RT32 five checks passed; touched-file linkcheck bad 0 |
| Deployed package | Twenty IMAP/client/helper/license files byte-match committed .136 |
| Activation | Plain agent and three configured instances disabled; sandbox on, owner-only ACLs, target unset; reload complete |
| Credential-free help | Installed .136 help-json loads methods/docs and name-only declarations without an account |
| Setup tool | Stamped .136 from d32d429d; only the changed setup binary promoted |
| Deployment check | 6/6, including after setup promotion |

Mutants cover UIDVALIDITY/account purge, no repeated UID fetch, byte/count/LRU
bounds, no-follow and private modes, root symlink refusal, flock, corruption
repair, literal/line/aggregate caps before allocation, short literals, PEEK and
read-only selection, UID cross-checks, UIDNEXT fallback, safe size differences,
bad-UID progress, tainted-stream progress, persisted retry counts/caps, transient
refusal recovery, non-touching bounded list/search, warning/publication bounds,
fixed destination and unknown-field refusal, secret masking, status before
untrusted content, reply classification/delivery errors, credential-free help,
whole-operation alarm, verified TLS, local-only plaintext, modified UTF-7 and
bounded discovery. Every credited mutation ran only its owning test.

Reviewer `#claude/ab2-review` closed `d32d429d`, checking main ancestry, .136
and identical imap.py/cache.py/wire.py to reviewed `133d7e9e`. Scope: static
read of those sources/deltas, agent.json head and env.dist; no test, docs,
packaging, gate or host command execution by the reviewer. Runtime evidence
above is ours.

The owner approved preparing the instances but left live enable/sync to the
manager. No implementer live IMAP/Gmail connection, account env-file read/write,
SMTP change, daemon/runner/service restart, container change or push occurred.
Own bus credentials were handed to the runner privately, never in argv or logs.
The manager separately reported successful read-only LOGIN/EXAMINE on the
accounts before deployment; that is not proof of this agent's live protocol
behaviour. The manager will enable one instance and test a small sync with
target unset. Sync never publishes; only explicit publish can send a message.

## Approved IMAP plan history

This was the pre-implementation B.5 plan. Review added bounded per-UID failure
progress, safe literal-size discrepancies, self-repair, UIDNEXT fallback,
non-touching scans and one persisted pending-UID retry cap. Current behaviour
is owned by the guide above.

<details>
<summary>Reviewed B.5 plan and acceptance</summary>

- ⏳ B.5 IMAP client
  - Builds: the R1 [IMAP client](../agents.md#mail-clients), kept by the runner
  - Rules: [mail clients](../agents.md#mail-clients): TLS with verified certificates, Gmail with an app password or OAuth2
  - Cache: [IMAP cache](../agents.md#mail-clients); a second read of an unchanged mailbox fetches no message again; a `UIDVALIDITY` change refetches; the bound holds
  - Acceptance: against a fixture IMAP server over TLS, read the configured mailbox and publish the expected message to its configured bus channel; a server refusal is reported without exposing credentials
  - Mutant: omit the fetched message or report a refused read as successful; each must fail its corresponding check

  - Plan (2026-10-28; review required before code): Python stdlib `#imap`, one account/mailbox per instance, one runner-kept serves-itself process. Its own bus credential enables explicit publication to the fixed configured target; default ACL `@owner`. No polling or automatic forwarding in this step. Methods: sync, list, search, read by UID, and publish by UID; help follows metadata. Calls select the mailbox read-only and use `BODY.PEEK[]`, never marking mail seen.
  - Publication format: [RD136](../DECISIONS.md#recorded-decisions): one plain external-content warning line followed by bounded raw RFC 822 text; no application JSON format. Mail is untrusted external content, including prompt injection; never execute/render HTML or fetch remote images. Publication must identify the content as external rather than instructions, within the existing cap.
  - IMAP review pins: quote mailbox names safely, encode modified UTF-7, refuse CR/LF/NUL, and accept only integer UIDs; search is a local bounded cache search, never raw server criteria. Use bounded UID range discovery and fetch RFC822.SIZE before BODY.PEEK; override imaplib read paths to refuse excessive literal/aggregate bytes before allocation. Test injection strings, Unicode labels, oversized declared literals and every bound.
  - Cache implementation: private files under `AGENT_BUS_WORK`; identity includes server, account and mailbox, then UIDVALIDITY and UID. Check UIDVALIDITY before returning cached mail. Fetch a cached UID zero additional times, including after process restart; evicted mail may be fetched again. A changed identity/UIDVALIDITY discards the old cache. Directories 0700/files 0600, dir-fd/O_NOFOLLOW access, bounded index parsing and rebuild on corruption. Atomic writes and flock across process generations; interrupted writes are discarded, never treated as complete mail. Read/publish re-check UID existence and refuse expunged mail; identify cached results. Sync advances oldest-first through a bounded UID window and reports its cursor/backlog, never immediately re-fetching an evicted earlier window. Cache is sensitive and removed with the instance, not included in a package.
  - Bounds: 64 MiB total cache by default (including index and temporary writes), configurable 4–512 MiB; evict oldest-accessed entries before reserving a write. At most 1,000 cached messages, 2 MiB raw per message, 100 new messages per sync, 100 list/search results and 256 KiB per answer/publication. Oversized mail is reported without caching a partial body. Bound IMAP literal lengths and accumulated protocol responses before allocation, not only after parsing; UID discovery uses bounded pages rather than an unbounded SEARCH ALL response. Cache search is explicitly local, with sync progress reported.
  - Proof: TLS fixture counts FETCH per UID across repeat reads/restart, changes UIDVALIDITY, exceeds each bound, interrupts a write and returns protocol refusals. A real bus fixture publishes one expected cached message only to the configured destination. Mutants remove cached-UID reuse, validity invalidation, byte/literal bounds, publication or refusal reporting, disable certificate verification or allow unintended plaintext; each owning assertion must fail.


</details>
