# SMS sender

[User guide](../../../docs/agents/sms.md). Runner-executed stdlib Python;
stdin is the delivered envelope, stdout is one JSON result. Help comes from
agent.json/env.dist; the body is the request, so methods is empty.

The fixed transport is HTTPS api.telnyx.com/v2/messages, using only the private
TELNYX_API_KEY host env key. The network declaration names that provider host; runner host lists are
currently coarse enablement, not a firewall. The fixed transport never
contacts another host or follows redirects.
No redirects, callbacks, history or retries. A timeout, 5xx, redirect or malformed
success is unknown. A 4xx is failed; a valid 2xx with a canonical UUID is accepted.
An invalid request is rejected before transport. The alarm bounds DNS and the
whole request before the message deadline, at most twenty seconds.

Logs whitelist caller name, last four destination digits, status and canonical
provider id. No exception/provider text is logged or returned. Fake HTTP tests
pin normalization, send count on timeout, response shape and credential secrecy.

Package with package.sh <agent.d/sms>. It runs outside the checkout with only
Python stdlib and the runner; no bus client or shared helper is imported.
