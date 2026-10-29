# Webhook bridge

📌 **TL;DR:** Receive HTTP POST bodies at fixed paths and forward them to a
configured queue or agent. Bus callers can post to named, configured outbound
URLs. Basic and signature checks are optional per inbound route; deploy behind
TLS and keep the record owner-only until its outbound authority is intended.

## Why you'd want it

Connect an HTTP event source to a queue, or let an admitted bus caller notify
one fixed HTTP endpoint without receiving its credentials.

## What it does

A kept, sandboxed Python process runs a bounded listener and answers `post` and
`routes`. Incoming bodies are forwarded as their original UTF-8 text: no JSON
reserialization, wrapper or HTTP headers. The receiver must treat them as
**untrusted external content**, despite delivery under the webhook's bus name.
Event-type headers such as `X-GitHub-Event` are deliberately not forwarded.

One accepted HTTP request makes one bus send. HTTP 202 means the bus accepted
it, not that its destination processed it. An uncertain send returns 503 with
an unknown-outcome message; there is no automatic resend. Repeating a request
can duplicate an event. There is no persistent or in-memory delivery dedupe:
Basic and GitHub signatures permit replay; Stripe signatures can be replayed
within the freshness window. TLS protects transit, not a sender's saved copy.

Outbound calls make one POST with the body unchanged, with no redirects,
proxy environment or retries. Upstream status and UTF-8 body are returned even
for non-2xx; credential masking may reformat a JSON response. Requests/replies
are bounded; a deadline or lost response is an unknown outcome, not proof the
remote service did nothing. A reported connect/certificate failure is not sent;
a helper killed before reporting is conservatively unknown, even if DNS was
still pending. There are no per-call URL/header overrides.

## You'll need

A runner with networking and bubblewrap, the owner's chosen listen address,
and a TLS reverse proxy for external ingress. The default binds only IPv4
loopback. Outbound-only configurations open no listener. Configure the fixed
receiving queue/agent's ACL to admit the webhook's own name.

## Try this first

Copy [config.json.dist](../../src/agents/webhook/config.json.dist), replace its
fictional targets/URL and set it as the whole Configuration value. Leave the
package disabled until the targets, authentication and reverse proxy are ready.
Use packaged `help-json` offline to inspect the call grammar without a listener.

## Use examples

```sh
agent-bus call '#webhook@node.example' routes
agent-bus call '#webhook@node.example' 'post notify {"event":"ready"}'
agent-bus call '#webhook@node.example' '{"method":"post","route":"notify","body":"hello"}'
```

For a local synthetic ingress check after enabling your chosen instance:

```sh
curl --fail-with-body -X POST http://127.0.0.1:8788/hooks/github \
  -H 'Content-Type: application/json' --data-binary @event.json
```

A configured Basic or signature check must also be supplied; this unsigned
example is refused by the shipped GitHub route. Real GitHub/Stripe deliveries
are verified as described under [Secret keys](#secret-keys).

## Installing it

Build `src/agents/webhook/package.sh <agent.d/webhook>`. Register the selected
name with `--allow='@owner'` and privately hand its token to the runner as for
[agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves).
Merge an initially disabled entry into the [runner inventory](../runner/configuration.md#inventory),
preserving other agents. Configure targets and Secret, then reload/enable only
when the owner approves exposure. No daemon or web change is needed.

`WEBHOOK_LISTEN` is a public host environment setting, numeric IPv4:port,
default `127.0.0.1:8788`. A non-loopback address exposes the listener to anyone
who can reach that interface; an unauthenticated route admits every sender.
TLS belongs to the reverse proxy. Do not forward arbitrary paths or weaken
its request-size, connection and timeout limits.

## Configuration

Set the entire raw JSON value, for example:

```sh
agent-bus config '#webhook@node.example' - < webhook-config.json
```

The web settings Configuration field is also a raw JSON textarea, not a
key/value form. Paste the list itself; do not turn the informational `(root)`
help label into a saved property.

The value is a **root JSON list**, not `{"routes":[...]}`. Native help labels
that value `(root)` as documentation; `(root)` is not an input property.
At most 64 routes; names are unique ASCII letters/digits/underscore/hyphen,
1–64 characters. Extra/duplicate keys and mixed inbound/outbound fields refuse.

| Entry field | Meaning |
|---|---|
| `name` | Required unique name, also used by outbound `post` |
| `path`, `target` | Inbound only, both required: literal HTTP path and queue/agent name |
| `url` | Outbound only, required fixed HTTP(S) URL; no userinfo, fragment or controls |
| `basic` | Optional `{ "user":"sender", "password":"PASSWORD_KEY" }`; password is a Secret key reference |
| `signature` | Inbound only, optional `{ "preset":"github", "secret":"KEY" }` or `stripe` |
| `max_bytes` | Integer 1–262144, default 262144, bounding incoming/outgoing request bodies |
| `rate`, `burst` | Inbound only: token refill per minute (1–600, default 60), bucket capacity (1–100, default 10) |
| `content_type` | Outbound only: `application/json` (default) or `text/plain` |

Inbound paths use `/` and ASCII letters/digits/`._~-`, at most 256 characters.
Queries, percent aliases, backslashes, double slashes and dot segments refuse;
a trailing slash is a distinct path. Only POST with one numeric Content-Length
is accepted. Transfer-Encoding, Expect, obs-fold, duplicate critical headers,
absolute-form targets and pipelining refuse. Every response closes its connection.
Unknown paths and failed authentication share a generic 404 with no challenge.

Basic is checked before reading the body; signature/freshness validation then
precedes the delivery token. Bad authentication never drains a delivery bucket.
Eight HTTP workers and at most six pre-authentication requests bound admission;
the first byte has one second, headers two seconds in total and body reads five seconds.
At the pre-authentication cap, the oldest unfinished connection is closed to
admit a faster request; if all eight workers are occupied, excess connections
are dropped without writing on the accept loop; the reverse proxy must handle network floods. Only ten
log lines/minute plus a bounded summary are emitted, with names/bytes/status,
never bodies, credentials, signatures or URLs. Counters and buckets reset at restart.

Empty authenticated bodies are accepted as empty messages. Queued events use
the destination record's own retention/bound; this bridge invents no different
expiry policy. Each delivery checks the target's current kind, with no cache. Pubsub, service,
resource, user or group targets refuse. Configuration changes apply on runner
reload; a Secret change requires restarting the kept process.

Outbound uses a fixed helper process, private pipes and a ten-second whole
DNS/connect/read deadline (or the caller's earlier deadline). It is killed and
reaped on timeout or excess output; stdout/stderr share a two-MiB pipe cap,
and upstream body reads stop at 256 KiB. Bus credentials, roles and arbitrary
host variables are removed from its environment. HTTPS verifies certificates.
Basic credentials on configured plain HTTP outside loopback travel unencrypted;
one fixed startup notice records that choice without its values.
Loopback/private configured URLs are permitted: Configuration, not the caller,
chooses the authority. No SO_REUSEPORT is used; an overlapping listener on the
same address fails visibly rather than splitting traffic.

## Secret keys

Basic passwords and HMAC keys are **bus Secret env-file entries**, not host
runner env variables. Configuration holds only their names. For example:

```sh
agent-bus secret '#webhook@node.example' - < private-webhook.env
```

The private file uses the shared [env-file rules](../runner/configuration.md#environment-files).
Its key names might be `HOOK_PASSWORD`, `GITHUB_KEY` and `STRIPE_KEY`; never put
values into commands, Configuration, examples, logs or reports. Missing/invalid
referenced keys disable the whole listener; other routes are not partly enabled.
Each signature route uses one UTF-8 key, with no overlapping-key rotation.

[GitHub validation](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries)
checks `X-Hub-Signature-256` against the exact raw body with HMAC-SHA256 and a
constant-time comparison. The prefix is exactly `sha256=` and hex is 64 digits.
[Stripe's verifier](https://github.com/stripe/stripe-go/blob/master/webhook/client.go)
uses `Stripe-Signature` timestamp-dot-raw-body HMAC-SHA256. This preset permits
one decimal timestamp and at most eight `v1` candidates, ignores unknown
schemes, and requires absolute clock distance at most 300 seconds. Both Basic
and signature must pass if both are configured.

## Who may use it

The bus ACL alone decides outbound access, default `@owner`. **Every grantee can
post arbitrary permitted bodies through every configured outbound route using
its stored Basic credential.** Use separate instances for separate authority;
there are no hidden per-route roles. Incoming HTTP authorization is the route's
Basic/signature policy, independent of the bus caller ACL. Destinations grant
the webhook name, not the original HTTP sender.

## Troubleshooting

`status` reports actual listener/configuration state and accepted/refused/unknown
counts; `routes` adds fixed-size counters per configured route. Missing Configuration/Secret or a bind conflict is failed; unknown
outcomes make status degraded. No listener is opened for invalid settings.
A 404 can mean unknown path or failed authentication; inspect your configured
path and credentials locally without logging them. A 429 means the route's
post-authentication bucket is empty. A 503 may be busy admission, a refused
bus target, or an explicitly unknown outcome; do not blindly retry an unknown
send unless the receiver can tolerate duplicates. Unsolicited replies to
forwarded events are not new commands and do not start answer loops.
