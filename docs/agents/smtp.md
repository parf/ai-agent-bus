# SMTP: send mail from your account

📌 **TL;DR:**
- **Problem:** reports and alerts should arrive by mail, but handing your mail password to every script or AI session is risky.
- **Outcome:** people, scripts and AI assistants send an email from one of your mail accounts without seeing its password; each call sends one message once, and you decide who may send, and to whom.
- **Start:** ask what the helper can do: `agent-bus help '#smtp/work@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **Every script needs the mail password.** Now it stays in one private file next to this helper; callers only get "send as this mailbox".
- 📧 **A report must reach a person.** Now a script or AI mails a report, summary or alert in one call.
- 🧱 **A script could mail anyone.** Now you limit the recipients to your own domain or a list.
- 🗂️ **Work and personal mail mix.** Now one helper per account keeps them apart.

## What it does

- ✅ **Send one message:** a subject, plain text, optional HTML, to up to 20 recipients across to, cc and bcc (`send`).
- ✅ **Nightly build report:** the team gets "Build passed" each morning.
- ✅ **AI review summary:** ask your AI to mail you the result.
- ✅ **Server alert:** a monitor sends an alert mail, only to your own domain.
- ✅ **Weekly status:** a readable HTML report with a plain-text copy.
- ✅ **From is always the configured account;** callers cannot change it or add other headers.
- ⚠️ **Each call submits once and never retries;** HTML is never rendered and nothing is fetched.
- 🚫 **Not for:** reading mail (use [IMAP](imap.md#what-it-does)), attachments (not supported), or bulk mailing lists.

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner), Python 3 and network access.
- A mail account that allows SMTP sign-in; for Gmail an [app password](https://support.google.com/accounts/answer/185833) (needs 2-Step Verification), since the normal password does not work.

## Try this first

- Ask the helper what it can do (help needs no credentials; the examples below send real mail):

```sh
agent-bus help '#smtp/work@node.example'
```

<details>
<summary>See it on a real node (ran today)</summary>

```sh
agent-bus help '#smtp/gmail@parf.us'
```

```text
Send mail once through a configured account with verified TLS
[destructive] send <to> <subject> <text> [cc] [bcc] [html]: Submit once; return accepted, partial, refused or unknown delivery outcome
  to (json): Nonempty array of bare ASCII addresses; at most 20 unique across to/cc/bcc
  subject (string): Single line, at most 998 UTF-8 bytes
  text (string): Plain text; total encoded mail at most 1 MiB
  cc (json): Additional visible recipients
  bcc (json): Envelope recipients, omitted from headers
  html (string): HTML alternative; never fetched or rendered here
...
```

- The output is shortened (the line marked `...`) and is from the development node on 2026-10-30.
- No mail was sent.

</details>

## Use examples

- **Mail the team a build report:** `agent-bus call '#smtp/work@node.example' '{"method":"send","to":["team@example.com"],"subject":"Build report","text":"Build passed."}'`
- **Send HTML with an archive copy:** `agent-bus call '#smtp/work@node.example' '{"method":"send","to":["ann@example.com"],"bcc":["archive@example.com"],"subject":"Report","text":"Plain report","html":"<p>Report</p>"}'`
- **Both commands send real mail:** run them only when you mean to.

## Installing it

- An administrator packages it once and adds one instance per mail account.

### Connect a Gmail account

- Gmail works with an **app password** (needs 2-Step Verification); your normal Google password is refused.
- One app password can serve both [`#imap`](imap.md#connect-a-gmail-account) and `#smtp`.
- Example: `your.name@gmail.com`, runner of `ann`.

1. Create the app password at <https://myaccount.google.com/apppasswords>.
2. Put the account in its private file `/var/lib/agent-bus/ann-ab-runner/smtp/gmail/env` (owned by `ann-ab-runner`, mode 0600), quoting the password:

   ```sh
   SMTP_USERNAME='your.name@gmail.com'
   SMTP_PASSWORD='abcdefghijklmnop'
   ```

3. Add `"smtp/gmail": {"enabled": true, "sandbox": "on"}` to the [runner inventory](../runner/configuration.md#inventory) and reload the runner; it creates `#smtp/gmail@node.example`. Then set the sender (server defaults are already Gmail's, smtp.gmail.com:587 with STARTTLS):

   ```sh
   echo '{"sender":"your.name@gmail.com"}' | agent-bus config '#smtp/gmail@node.example' -
   ```

4. Check with a mail to yourself at another address: `agent-bus call '#smtp/gmail@node.example' '{"method":"send","to":["ann@example.com"],"subject":"Test","text":"Hello"}'`.
   - Gmail files mail you send to your own Gmail address under Sent, not the inbox.

<details>
<summary>Step by step</summary>

1. Package with `src/agents/smtp/package.sh <agent.d/smtp>`.
2. Add an instance to the [runner inventory](../runner/configuration.md#inventory).
3. Put the account's keys in its private host file ([Secret keys](#secret-keys)).

- It uses only Python's standard library and the runner's network sandbox; a package refresh needs no restart.

</details>

## Configuration

- **What:** the sender address and mail server, plus an optional recipient allowlist.
- **Every recipient is checked before connecting;** a TLS failure never falls back to plain.

<details>
<summary>Keys</summary>

| Key | Meaning |
|---|---|
| `sender` | Required bare ASCII sender address |
| `host`, `port` | Defaults smtp.gmail.com and 587 |
| `mode` | starttls (default), tls (implicit TLS, usually port 465), or plain |
| `auth` | password (AUTH PLAIN, default) or xoauth2 (Google OAuth refresh) |
| `recipients` | Optional array of exact addresses or domains; omitted or empty admits any recipient |
| `cafile` | Optional public CA trust file readable inside the sandbox; omitted uses system trust |
| `allow_plaintext` | Default false; true permits plain mode only to a loopback address and allows port 25 |

</details>

## Secret keys

- **The account's sign-in lives only** in this instance's private host file `<host>/smtp/<instance>/env` (mode 0600, owned by the runner account; see [where settings belong](../runner/configuration.md#where-settings-belong)).
- **Never** in Configuration, a call or bus Secret; credentials are never logged.

<details>
<summary>Keys</summary>

| Key | When |
|---|---|
| `SMTP_USERNAME` | Always; for Gmail, the authenticated account |
| `SMTP_PASSWORD` | `auth=password` |
| `SMTP_CLIENT_ID`, `SMTP_CLIENT_SECRET`, `SMTP_REFRESH_TOKEN` | `auth=xoauth2` |

- Keep package-wide `smtp/env` free of account credentials when instances use different accounts; `env.dist` declares names only; protocol traces are never logged.
- Get the refresh token yourself through Google's consent flow with offline access and scope `https://mail.google.com/` ([XOAUTH2](https://developers.google.com/workspace/gmail/imap/xoauth2-protocol), [offline access](https://developers.google.com/identity/protocols/oauth2/web-server#offline)); the agent has no browser.
- Each call refreshes once at the fixed `https://oauth2.googleapis.com/token`, no redirect or retry; the access token stays in that process's memory.
- An XOAUTH2 refusal returns a fixed text, never Google's echoed challenge.

</details>

## Who may use it

- **Only the owner at first** (`@owner`).
- **A grant means "send as this mailbox",** with no volume limit beyond the provider's quotas.
- **Set `recipients` before granting it broadly.**

## Troubleshooting

| Answer | Meaning and what to do |
|---|---|
| accepted | Server accepted the message (queue id if given); not a promise of inbox delivery. |
| partial | Accepted for the named recipients; refused ones carry codes. Do not resend to everyone. |
| refused | Definite SMTP refusal (auth, MAIL FROM or final DATA 4xx/5xx); code and reason, credentials masked. |
| not_sent | Bad request, missing credentials or a failure before sending; nothing was sent. |
| unknown | Connection lost while sending; the mail may have gone out. Check before retrying (the answer says so too). |
| `invalid_grant` | Renew the account's consent and refresh token. |

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#smtp` reports these.

| Level | When | What to do |
|---|---|---|
| alert | The server refuses the sign-in, or the OAuth refresh is rejected (`invalid_grant`) | Check the host file; renew the consent and refresh token |
| alert | The server cannot be reached, or TLS or STARTTLS fails | Check `host`, `port`, `mode` and `cafile` |
| alert | `sender` or a required sign-in key is missing or invalid | Fix Configuration or the host file |
| warning | A send's outcome is `unknown`: the connection was lost while sending | Check before anyone retries |
| warning | Some recipients were refused (`partial`), or the server refused a send for a reason other than sign-in | Read the codes; check the recipients and the quota |

## Technical details

<details>
<summary>Message rules and limits</summary>

- Fields: subject, text, optional html, to, cc, bcc; no From, Reply-To, Sender, Message-ID or other headers; duplicate recipients submit once; Bcc stays in the SMTP envelope only.
- Addresses: one bare ASCII address each, no display name or quoted local part; local parts use letters, digits, `.`, `_`, `+`, `'` or `-`; punycode domains.
- The sender supplies From and MAIL FROM; EHLO uses its domain; Date is set at build; MIME keeps outgoing bytes 7-bit, including Russian text.
- Auth: AUTH PLAIN (app passwords) or Google OAuth2 XOAUTH2.
- Message-ID is stable per delivered bus message id, on the sender's domain; servers need not deduplicate by it.
- Mail up to 1 MiB, subject 998 UTF-8 bytes, reply line 4 KiB, whole reply 64 KiB; the whole operation, including DNS, TLS and auth, is bounded by the delivered deadline and 60 seconds.

</details>

<details>
<summary>TLS and allowlist matching</summary>

- TLS 1.2 or later with certificate and hostname checks; STARTTLS must be advertised and succeed, then EHLO again before auth ([Gmail SMTP](https://developers.google.com/workspace/gmail/imap/imap-smtp)).
- A domain entry matches that whole domain, case-insensitively, no subdomains.
- An address entry keeps local-part case: `Bob@Example.com` matches `Bob@example.com`, not `bob@example.com` or `Bob@example.com.evil.test`.

</details>
