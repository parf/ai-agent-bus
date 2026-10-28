# Mail delivery evidence

## B.6 App-password SMTP

Landed 2026-10-28 in `9bfe0926`, shared version **0.9.134**. B.6 remains
open for the separate XOAUTH2 increment and owner-approved real-provider check;
B.5 IMAP is separate. Current use is documented in the
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
