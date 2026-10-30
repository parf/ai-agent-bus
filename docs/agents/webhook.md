# Webhook bridge

📌 **TL;DR:**
- **Problem:** web services announce events by calling a web address, and your assistants and servers cannot hear them; or a caller must notify one web endpoint without holding its credentials.
- **Outcome:** an HTTP post becomes a bus message, a hosting service's push reaches every server that follows the branch, and an admitted caller posts to a named, configured address.
- **Start:** keep the package disabled until targets, authentication and the reverse proxy are ready: [Try this first](#try-this-first).

## Why you'd want it

- 📥 **An event source speaks HTTP.** Now each post lands in an inbox or reaches an assistant.
- 🚀 **A push must update many servers.** Now a GitLab or GitHub push becomes one new-code event for every server that follows the branch.
- 🔑 **A caller must not hold another service's credentials.** Now it posts to a named route and the Basic credential stays in the helper.
- 🛡️ **Anyone could post.** Now each inbound route can require Basic or a signature check.

## What it does

- ✅ **Receive:** HTTP POST bodies at fixed paths are forwarded to a configured queue or agent, as their original UTF-8 text (no wrapper, no HTTP headers).
- ✅ **Publish new code:** a GitLab or GitHub push, or a pipeline's deploy request, becomes a new-code event on a channel ([below](#publishing-new-code)).
- ✅ **Post out:** bus callers post to named, configured outbound URLs, never choosing the target (`post`, `routes`).
- ✅ **Check the sender:** Basic and signature checks are optional per inbound route.
- ⚠️ **202 means the bus accepted it,** not that the destination processed it; an uncertain send returns 503 and nothing resends it.
- ⚠️ **Received bodies are untrusted external content,** even though they arrive under the webhook's bus name.
- 🔒 **Deploy behind TLS** and keep the record owner-only until its outbound authority is intended.

## You'll need

- A runner with networking and bubblewrap.
- The owner's chosen listen address, and a TLS reverse proxy for external ingress.
- The fixed receiving queue or agent's access list admitting the webhook's own name.
- The default binds only IPv4 loopback; outbound-only configurations open no listener.

## Try this first

- Copy [config.json.dist](../../src/agents/webhook/config.json.dist), replace its fictional targets and URL, and set it as the whole Configuration value.
- Leave the package disabled until the targets, authentication and reverse proxy are ready.
- Use the packaged `help-json` offline to inspect the call grammar without a listener.

<details>
<summary>See what an unconfigured helper says (ran today)</summary>

```sh
agent-bus call '#webhook@parf.us' routes | tail -1 | jq -r .body
```

```text
error: Configuration, required Secret or listener unavailable
```

- On the development node the helper is running without routes; the output is from 2026-10-30.
- Step by step: [Installing it](#installing-it).

</details>

## Use examples

- **List the routes:** `agent-bus call '#webhook@node.example' routes`
- **Post to a named route:** `agent-bus call '#webhook@node.example' 'post notify {"event":"ready"}'`
- **The same as JSON:** `agent-bus call '#webhook@node.example' '{"method":"post","route":"notify","body":"hello"}'`
- **Check an inbound route locally** after enabling your chosen instance:

```sh
curl --fail-with-body -X POST http://127.0.0.1:8788/hooks/github \
  -H 'Content-Type: application/json' --data-binary @event.json
```

- A configured Basic or signature check must also be supplied; this unsigned example is refused by the shipped GitHub route.
- Real GitHub and Stripe deliveries are verified as described under [Secret keys](#secret-keys).

## Installing it

- Build `src/agents/webhook/package.sh <agent.d/webhook>`.
- Register the selected name with `--allow='@owner'` and privately hand its token to the runner, as for [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves).
- Merge an initially disabled entry into the [runner inventory](../runner/configuration.md#inventory), preserving other agents.
- Configure targets and Secret, then reload and enable it when you are ready to expose it.
- No daemon or web change is needed.

<details>
<summary>The listen address</summary>

- `WEBHOOK_LISTEN` is a public host environment setting, numeric IPv4:port, default `127.0.0.1:8788`.
- A non-loopback address exposes the listener to anyone who can reach that interface; an unauthenticated route admits every sender.
- TLS belongs to the reverse proxy; do not forward arbitrary paths or weaken its request-size, connection and timeout limits.

</details>

## Configuration

- **Set the entire raw JSON value:** a **root JSON list**, not `{"routes":[...]}`.
- **Command line:** `agent-bus config '#webhook@node.example' - < webhook-config.json`.
- **Web:** the settings Configuration field is a raw JSON textarea, not a key/value form; paste the list itself.
- **`(root)`** is a documentation label in native help, not an input property.
- **At most 64 routes;** names are unique ASCII letters, digits, underscore or hyphen, 1–64 characters; extra or duplicate keys and mixed inbound and outbound fields refuse.

<details>
<summary>Route fields</summary>

| Entry field | Meaning |
|---|---|
| `name` | Required unique name, also used by outbound `post` |
| `path` | Inbound only, required: literal HTTP path |
| `target` | Inbound: the queue or agent that gets the body unchanged; or use `publish` |
| `publish` | Inbound, instead of `target`: a pubsub name that gets a [new-code event](#publishing-new-code) |
| `event` | With `publish`, required: `gitlab-push`, `github-push` or `deploy` |
| `branches` | With `publish`, required: 1–16 branch names; any other branch publishes nothing |
| `url` | Outbound only, required fixed HTTP(S) URL; no userinfo, fragment or controls |
| `basic` | Optional `{ "user":"sender", "password":"PASSWORD_KEY" }`; password is a Secret key reference |
| `signature` | Inbound only, optional `{ "preset":"github", "secret":"KEY" }`, `gitlab` or `stripe` |
| `max_bytes` | Integer 1–262144, default 262144, bounding incoming and outgoing request bodies |
| `rate`, `burst` | Inbound only: token refill per minute (1–600, default 60), bucket capacity (1–100, default 10) |
| `content_type` | Outbound only: `application/json` (default) or `text/plain` |

</details>

## Publishing new code

- **What:** a route with `publish` reads the payload, which stays untrusted, and publishes one strict event instead of the body.
- **Who gets it:** every name on the channel's Deliver-To list, such as each `#git` that follows the branch.

| `event` | Accepted request | Published |
|---|---|---|
| `gitlab-push` | `X-Gitlab-Event: Push Hook`; `ref`, `after`, `project.path_with_namespace` | `{"v":1,"type":"push","remote":"realmo/realty","branch":"main","commit":"<40 hex>"}` |
| `github-push` | `X-GitHub-Event: push`; `ref`, `after`, `repository.full_name` | the same |
| `deploy` | a pipeline's JSON with exactly `remote`, `branch`, `commit` (7–40 hex) and an optional `id` (1–64 of `A-Za-z0-9._-`) | `{"v":1,"type":"deploy",…,"id":"pipeline-81234"}` |

| Request | Answer |
|---|---|
| mapped and published | 202 `Accepted` |
| another branch, a tag push, a deleted branch, a ping or another event type | 202 `Ignored`, nothing published |
| a malformed payload or a field outside the table | 400, nothing published |
| failed Basic, token or signature | the generic 404, nothing published |

- **At start** the webhook checks every `publish` name: it must be a pubsub that the webhook's own name may publish to, by the channel's ACL; otherwise the whole listener stays closed and `status` names the route.
- **Grant it** with, for example, `agent-bus manage autopull-cmd@node.example --add-to-set-allow='#webhook@node.example'`.
- **A large GitHub push** can exceed the default 256 KiB cap; such a request is refused, not truncated.

```json
{"name":"gitlab","path":"/hooks/gitlab","publish":"autopull-cmd@node.example",
 "event":"gitlab-push","branches":["main"],"signature":{"preset":"gitlab","secret":"GITLAB_TOKEN"}}
```

## Secret keys

- **Basic passwords and HMAC keys** are **bus Secret env-file entries**, not host runner env variables; Configuration holds only their names.
- **Set them:** `agent-bus secret '#webhook@node.example' - < private-webhook.env`.
- **The private file** uses the shared [env-file rules](../runner/configuration.md#environment-files); its key names might be `HOOK_PASSWORD`, `GITHUB_KEY`, `GITLAB_TOKEN` and `STRIPE_KEY`.
- **Never put values** into commands, Configuration, examples, logs or reports.
- **A missing or invalid referenced key disables the whole listener;** other routes are not partly enabled; each signature route uses one UTF-8 key, with no overlapping-key rotation.
- **Both Basic and signature must pass** if both are configured.

<details>
<summary>How each check works</summary>

- [GitHub validation](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries) checks `X-Hub-Signature-256` against the exact raw body with HMAC-SHA256 and a constant-time comparison; the prefix is exactly `sha256=` and hex is 64 digits.
- [Stripe's verifier](https://github.com/stripe/stripe-go/blob/master/webhook/client.go) uses `Stripe-Signature` timestamp-dot-raw-body HMAC-SHA256.
- The `gitlab` preset compares the one `X-Gitlab-Token` header with the key in constant time; GitLab sends the token itself, so keep the route behind TLS.
- Stripe-style checks permit one decimal timestamp and at most eight `v1` candidates, ignore unknown schemes, and require absolute clock distance at most 300 seconds.

</details>

## Who may use it

- **Targets are fixed in Configuration** by the Owner or Maintainers, never chosen by a caller, so no per-caller send check applies.
- **The bus access list alone decides outbound access,** default `@owner`.
- **Every grantee can post** arbitrary permitted bodies through every configured outbound route using its stored Basic credential; use separate instances for separate authority; there are no hidden per-route roles.
- **Incoming HTTP authorization** is the route's Basic or signature policy, independent of the bus caller list.
- **Destinations grant the webhook name,** not the original HTTP sender.

## Troubleshooting

| If you see | Do this |
|---|---|
| A 404 | An unknown path or failed authentication: inspect your configured path and credentials locally without logging them |
| A 429 | The route's post-authentication bucket is empty |
| A 503 | Busy admission, a refused bus target, or an explicitly unknown outcome; do not blindly retry an unknown send unless the receiver can tolerate duplicates |
| Replies to forwarded events | They are not new commands and do not start answer loops |
| `status` failed or degraded | Missing Configuration or Secret, or a bind conflict is failed; unknown outcomes make it degraded; no listener opens for invalid settings |

- `status` reports the actual listener and configuration state and accepted, refused and unknown counts; `routes` adds fixed-size counters per configured route.

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#webhook` reports these.

| Level | When | What to do |
|---|---|---|
| alert | The listener stays closed: invalid Configuration, a missing Secret key, or a `publish` name it may not publish to | Fix the route `status` names, then reload the runner |
| alert | Another program holds `WEBHOOK_LISTEN` | Free the address or choose another |
| alert | A route's `target` refuses the webhook or no longer exists | Allow the webhook on the target, or fix the route |
| warning | A send's outcome was unknown (HTTP 503) | Check the destination; the sender may repeat it |
| warning | Requests were refused by the rate bucket (429) or dropped at the admission limit | Raise `rate` or `burst`, or look for a flood at the proxy |
| warning | Repeated failed authentication on a route | Check the sender's credentials; it may be a probe |
| warning | Basic credentials go to a plain-HTTP outbound URL outside loopback | Use HTTPS |

## Technical details

<details>
<summary>How it runs and what it forwards</summary>

- A kept, sandboxed Python process runs a bounded listener and answers `post` and `routes`.
- Incoming bodies are forwarded as their original UTF-8 text: no JSON reserialization, wrapper or HTTP headers; event-type headers such as `X-GitHub-Event` are deliberately not forwarded.
- One accepted HTTP request makes one bus send; there is no automatic resend, so repeating a request can duplicate an event.
- There is no persistent or in-memory delivery dedupe: Basic and GitHub signatures permit replay; Stripe signatures can be replayed within the freshness window; TLS protects transit, not a sender's saved copy.
- Empty authenticated bodies are accepted as empty messages; queued events use the destination record's own retention and bound; each delivery checks the target's current kind, with no cache.
- A `target` must be a queue or agent and a `publish` name a pubsub; anything else refuses.
- Configuration changes apply on runner reload; a Secret change requires restarting the kept process.

</details>

<details>
<summary>Inbound requests and limits</summary>

- Inbound paths use `/` and ASCII letters, digits and `._~-`, at most 256 characters; queries, percent aliases, backslashes, double slashes and dot segments refuse; a trailing slash is a distinct path.
- Only POST with one numeric Content-Length is accepted; Transfer-Encoding, Expect, obs-fold, duplicate critical headers, absolute-form targets and pipelining refuse; every response closes its connection.
- Unknown paths and failed authentication share a generic 404 with no challenge.
- Basic is checked before reading the body; signature and freshness validation then precede the delivery token; bad authentication never drains a delivery bucket.
- Eight HTTP workers and at most six pre-authentication requests bound admission; the first byte has one second, headers two seconds in total and body reads five seconds.
- At the pre-authentication cap, the oldest unfinished connection is closed to admit a faster request; if all eight workers are occupied, excess connections are dropped without writing on the accept loop; the reverse proxy must handle network floods.
- Only ten log lines per minute plus a bounded summary are emitted, with names, bytes and status, never bodies, credentials, signatures or URLs; counters and buckets reset at restart.
- No SO_REUSEPORT is used; an overlapping listener on the same address fails visibly rather than splitting traffic.

</details>

<details>
<summary>Outbound calls</summary>

- One POST with the body unchanged, with no redirects, proxy environment or retries.
- Upstream status and UTF-8 body are returned even for non-2xx; credential masking may reformat a JSON response.
- A fixed helper process, private pipes and a ten-second whole DNS, connect and read deadline (or the caller's earlier deadline); it is killed and reaped on timeout or excess output.
- stdout and stderr share a two-MiB pipe cap, and upstream body reads stop at 256 KiB; bus credentials, roles and arbitrary host variables are removed from its environment.
- HTTPS verifies certificates; Basic credentials on configured plain HTTP outside loopback travel unencrypted, and one fixed startup notice records that choice without its values.
- Loopback and private configured URLs are permitted: Configuration, not the caller, chooses the authority; there are no per-call URL or header overrides.
- A deadline or lost response is an unknown outcome, not proof the remote service did nothing; a reported connect or certificate failure is not sent; a helper killed before reporting is conservatively unknown, even if DNS was still pending.

</details>
