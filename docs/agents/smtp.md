# SMTP: send mail through your account

📌 **TL;DR:** Submit one email through the account configured for this helper.
TLS verifies the server's certificate. A grant lets a caller send as that
account; start owner-only and restrict recipients when sharing it.

## Why you'd want it

People and AI sessions can send reports without receiving your mail password.
Separate instances keep work and personal accounts apart.

## What it does

`send` submits one plain-text message, optionally with an HTML alternative,
to up to 20 unique envelope recipients across to/cc/bcc (duplicates submit once). No attachments in this version.
It never retries, and it never renders HTML or fetches embedded resources.

Supports app passwords with AUTH PLAIN and Google OAuth2 XOAUTH2.
IMAP reading and its local cache are a separate helper.

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner), Python 3 and network access.
- A provider account permitting SMTP AUTH PLAIN or Gmail XOAUTH2 over verified TLS.
- For Gmail, an app password created in the account's security settings after
  enabling 2-Step Verification; an ordinary account password is not sufficient.
  See [Google's app-password instructions](https://support.google.com/accounts/answer/185833).

## Try this first

```sh
agent-bus help '#smtp/work@node.example'
```

Help needs no credentials. Installation on the development node leaves the
package disabled until the owner supplies an account; examples below send
real mail when an enabled account is configured.

## Use examples

```sh
agent-bus call '#smtp/work@node.example' '{"method":"send","to":["reader@example.com"],"subject":"Build report","text":"Build passed."}'
agent-bus call '#smtp/work@node.example' '{"method":"send","to":["reader@example.com"],"bcc":["archive@example.com"],"subject":"Report","text":"Plain report","html":"<p>Report</p>"}'
```

Addresses are single bare ASCII addresses, without display names or quoted local parts; domains must be ASCII, with
punycode accepted. Local parts use letters, digits, dots, underscore, plus,
apostrophe or hyphen. Only subject, text, optional html and the
recipient arrays are accepted. Callers cannot supply From, Reply-To, Sender,
Message-ID or arbitrary headers. Bcc travels only in the SMTP envelope.
The configured sender supplies both From and MAIL FROM. Date is set when
building the mail; EHLO uses the sender domain. MIME encoding keeps all outgoing
bytes 7-bit, including Russian text; recipients decode the original text.

## Installing it

Package with `src/agents/smtp/package.sh <agent.d/smtp>`, then configure an
instance in the [runner inventory](../runner/configuration.md#inventory).
The script uses Python's standard library and the runner's network sandbox.
No daemon, runner binary or service restart is needed for a package refresh.

## Configuration

| Key | Meaning |
|---|---|
| `sender` | Required bare ASCII sender address |
| `host`, `port` | Defaults smtp.gmail.com and 587 |
| `mode` | Explicit starttls (default), tls (implicit TLS, usually port 465), or plain |
| `auth` | password (AUTH PLAIN, default) or xoauth2 (Google OAuth refresh) |
| `recipients` | Optional array of exact addresses or domains; omitted/empty admits any recipient |
| `cafile` | Optional public CA trust file accessible inside the sandbox; omitted uses system trust |
| `allow_plaintext` | Default false; explicit true permits plain mode only to a loopback address and allows port 25 |

STARTTLS must be advertised, complete successfully, and be followed by a new
EHLO before authentication. TLS is at least 1.2 with certificate and hostname
verification. A TLS failure never activates plain mode. Gmail supports
[SMTP TLS and app-password/OAuth authentication](https://developers.google.com/workspace/gmail/imap/imap-smtp).

Allowlist domains match the entire domain, case-insensitively; they include
no subdomains implicitly. Exact addresses preserve local-part case:
`Bob@Example.com` matches `Bob@example.com`, but not `bob@example.com` or
`Bob@example.com.evil.test`. Every to/cc/bcc recipient is checked before connecting.

## Secret keys

Put `SMTP_USERNAME` and the credentials for the chosen auth mode in this
instance's private host file:
`<host>/smtp/<instance>/env`, mode 0600, owned by the runner account.
See [where settings belong](../runner/configuration.md#where-settings-belong).
Keep package-wide `smtp/env` free of account credentials when instances use
different accounts. Never put credentials in Configuration, a call or bus Secret.
`env.dist` declares names only; packages contain no private values.

For `auth=password`, supply `SMTP_PASSWORD`. For `auth=xoauth2`, supply
`SMTP_CLIENT_ID`, `SMTP_CLIENT_SECRET` and `SMTP_REFRESH_TOKEN`. Obtain the
refresh token through Google's consent flow with offline access and the
`https://mail.google.com/` scope; see [Google's XOAUTH2 guide](https://developers.google.com/workspace/gmail/imap/xoauth2-protocol)
and [offline access](https://developers.google.com/identity/protocols/oauth2/web-server#offline).
The agent has no browser or consent flow. Gmail username must name the
authenticated account.

Each per-message process refreshes once through the fixed verified HTTPS
endpoint `https://oauth2.googleapis.com/token`. There is no configurable token
URL, redirect or retry. The access token lives only in this process's memory;
no token is written into Configuration or the cache. An `invalid_grant` answer
means renew the account's consent/refresh token. An XOAUTH2 auth refusal
completes the protocol's empty challenge response and returns a fixed refusal,
never Google's echoed challenge text.

## Who may use it

Default ACL is `@owner`. A bare grant means “send as this mailbox”, without a
volume limit beyond the provider's quotas. There is no per-call sender override
or hidden rate policy. Set a recipient allowlist before granting it broadly.

## Troubleshooting

Replies are JSON with a fixed outcome below; `unknown` also carries a `note`
warning against automatic retry:

| Outcome | What to do |
|---|---|
| accepted | Server accepted DATA; acceptance text/queue id is returned when supplied. This does not promise inbox delivery. |
| partial | DATA was accepted for the named accepted recipients; refused recipients have their codes. Do not resend to everyone. |
| refused | A definite SMTP refusal, including authentication/MAIL FROM failure or final DATA 4xx/5xx; code/reason are returned with credentials masked. |
| not_sent | Validation, missing credentials or a failure before DATA; no submission took place. |
| unknown | A disconnect, timeout or invalid reply after DATA began; the server may have accepted the message. Inspect before retrying. |

Each call submits once. Message-ID is stable for the delivered bus message id,
which the daemon assigns, and uses the configured sender's domain. A new call
has a new id; no receiving server is required to deduplicate by Message-ID.

Encoded mail is limited to 1 MiB, a subject to 998 UTF-8 bytes, each SMTP reply
line to 4 KiB and a whole reply to 64 KiB. The complete operation, including
DNS/TLS/auth and a trickling server, is bounded by the delivered deadline and
60 seconds. Credentials and protocol traces are never logged. Tests use local
TLS fixtures only; the real Gmail check awaits explicit owner approval.
