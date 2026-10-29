# SMTP: send mail from your account

📌 **TL;DR:** Lets people, scripts and AI assistants send an email from one of
your mail accounts without ever seeing its password. Each call sends one
message once. You decide who may send, and to whom.

## Why you'd want it

**The problem.** Reports and alerts should arrive by mail, but handing your mail
password to every script or AI session is risky.

**The idea.** The password stays in one private file next to this helper.
Callers only get "send as this mailbox", and you can limit the recipients.

**When to use it**

- A script or AI should mail a report, summary or alert.
- Several people or sessions share one sending account.
- You want work and personal accounts kept apart (one helper each).

**When not to:** reading mail (use [IMAP](imap.md#what-it-does)), attachments
(not supported), or bulk mailing lists.

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| Nightly build report | A build script calls `send` | The team gets "Build passed" each morning |
| AI review summary | Ask your AI to mail you the result | The summary in your inbox |
| Server alert | A monitor calls `send` | An alert mail, only to your own domain |
| Weekly status | A timer sends a short HTML report | A readable report with a plain-text copy |

## What it does

| Call | Does |
|---|---|
| `send` | Sends one message: subject, plain text, optional HTML, to up to 20 recipients (to/cc/bcc) |

The From address is always the configured account; callers cannot change it or
add other headers. No attachments. Each call submits once and **never retries**;
HTML is never rendered and nothing is fetched.

<details><summary>Technical details: message rules and limits</summary>

- Fields: subject, text, optional html, to/cc/bcc. No From, Reply-To, Sender,
  Message-ID or other headers. Duplicate recipients submit once; Bcc stays in
  the SMTP envelope only.
- Addresses: one bare ASCII address each, no display name or quoted local part;
  local parts use letters, digits, `.`, `_`, `+`, `'` or `-`; punycode domains.
- The sender supplies From and MAIL FROM; EHLO uses its domain; Date is set at
  build. MIME keeps outgoing bytes 7-bit, including Russian text.
- Auth: AUTH PLAIN (app passwords) or Google OAuth2 XOAUTH2.
- Message-ID is stable per delivered bus message id, on the sender's domain;
  servers need not deduplicate by it.
- Mail up to 1 MiB, subject 998 UTF-8 bytes, reply line 4 KiB, whole reply
  64 KiB. The whole operation, including DNS/TLS/auth, is bounded by the
  delivered deadline and 60 seconds.

</details>

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner), Python 3 and network access.
- A mail account that allows SMTP sign-in. For Gmail, an
  [app password](https://support.google.com/accounts/answer/185833) (needs
  2-Step Verification); the normal password does not work.

## Try this first

```sh
agent-bus help '#smtp/work@node.example'
```

Help needs no credentials. The examples below send real mail.

## Use examples

| I want to… | Command |
|---|---|
| Mail the team a build report | `agent-bus call '#smtp/work@node.example' '{"method":"send","to":["team@example.com"],"subject":"Build report","text":"Build passed."}'` |
| Send HTML with an archive copy | `agent-bus call '#smtp/work@node.example' '{"method":"send","to":["ann@example.com"],"bcc":["archive@example.com"],"subject":"Report","text":"Plain report","html":"<p>Report</p>"}'` |

## Installing it

An administrator packages it once and adds one instance per mail account.

<details><summary>Step by step</summary>

1. Package with `src/agents/smtp/package.sh <agent.d/smtp>`.
2. Add an instance to the [runner inventory](../runner/configuration.md#inventory).
3. Put the account's keys in its private host file ([Secret keys](#secret-keys)).

It uses only Python's standard library and the runner's network sandbox; a
package refresh needs no restart.

</details>

## Configuration

The sender address and mail server, plus an optional recipient allowlist. Every
recipient is checked before connecting; a TLS failure never falls back to plain.

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `sender` | Required bare ASCII sender address |
| `host`, `port` | Defaults smtp.gmail.com and 587 |
| `mode` | starttls (default), tls (implicit TLS, usually port 465), or plain |
| `auth` | password (AUTH PLAIN, default) or xoauth2 (Google OAuth refresh) |
| `recipients` | Optional array of exact addresses or domains; omitted/empty admits any recipient |
| `cafile` | Optional public CA trust file readable inside the sandbox; omitted uses system trust |
| `allow_plaintext` | Default false; true permits plain mode only to a loopback address and allows port 25 |

</details>

<details><summary>Technical details: TLS and allowlist matching</summary>

- TLS 1.2 or later with certificate and hostname checks; STARTTLS must be
  advertised and succeed, then EHLO again before auth
  ([Gmail SMTP](https://developers.google.com/workspace/gmail/imap/imap-smtp)).
- A domain entry matches that whole domain, case-insensitively, no subdomains.
  An address entry keeps local-part case: `Bob@Example.com` matches
  `Bob@example.com`, not `bob@example.com` or `Bob@example.com.evil.test`.

</details>

## Secret keys

The account's sign-in lives only in this instance's private host file
`<host>/smtp/<instance>/env` (mode 0600, owned by the runner account; see
[where settings belong](../runner/configuration.md#where-settings-belong)) —
never in Configuration, a call or bus Secret. Credentials are never logged.

<details><summary>Keys</summary>

| Key | When |
|---|---|
| `SMTP_USERNAME` | Always; for Gmail, the authenticated account |
| `SMTP_PASSWORD` | `auth=password` |
| `SMTP_CLIENT_ID`, `SMTP_CLIENT_SECRET`, `SMTP_REFRESH_TOKEN` | `auth=xoauth2` |

- Keep package-wide `smtp/env` free of account credentials when instances use
  different accounts. `env.dist` declares names only. Protocol traces are never logged.
- Get the refresh token yourself through Google's consent flow with offline
  access and scope `https://mail.google.com/`
  ([XOAUTH2](https://developers.google.com/workspace/gmail/imap/xoauth2-protocol),
  [offline access](https://developers.google.com/identity/protocols/oauth2/web-server#offline));
  the agent has no browser.
- Each call refreshes once at the fixed `https://oauth2.googleapis.com/token`,
  no redirect or retry; the access token stays in that process's memory.
- An XOAUTH2 refusal returns a fixed text, never Google's echoed challenge.

</details>

## Who may use it

Only the owner at first (`@owner`). A grant means "send as this mailbox", with
no volume limit beyond the provider's quotas. Set `recipients` before granting
it broadly.

## Troubleshooting

| Answer | Meaning and what to do |
|---|---|
| accepted | Server accepted the message (queue id if given); not a promise of inbox delivery. |
| partial | Accepted for the named recipients; refused ones carry codes. Do not resend to everyone. |
| refused | Definite SMTP refusal (auth, MAIL FROM or final DATA 4xx/5xx); code and reason, credentials masked. |
| not_sent | Bad request, missing credentials or a failure before sending; nothing was sent. |
| unknown | Connection lost while sending; the mail may have gone out. Check before retrying (the answer says so too). |
| `invalid_grant` | Renew the account's consent and refresh token. |
