# SMS sender

[User guide](../../../docs/agents/sms.md). Runner-executed stdlib Python;
stdin is the delivered envelope, stdout is one JSON result. Help comes from
agent.json/env.dist. The declared `send` method and bare request forms share
the same one-attempt handler.

TELNYX_API_URL defaults to HTTPS api.telnyx.com/v2/messages, using only the private
TELNYX_API_KEY host env key. The network declaration names that provider host; runner host lists are
currently coarse enablement, not a firewall. The fixed transport never
contacts another host or follows redirects.
The real host requires delivered live-runner identity. Tests override the URL
with a loopback fake and block every external socket. No redirects, callbacks, history or retries. A timeout, 5xx, redirect or malformed
success is unknown. A 4xx is failed; a valid 2xx with a canonical UUID is accepted.
An invalid request is rejected before transport. The alarm bounds DNS and the
whole request before the message deadline, at most twenty seconds.

Logs whitelist caller name, last four destination digits, status and canonical
provider id. No exception/provider text is logged or returned. Fake HTTP tests
pin normalization, send count on timeout, response shape and credential secrecy.

Package with package.sh <agent.d/sms-telnyx>. It runs outside the checkout with only
Python stdlib and the runner; no bus client is needed; the packaged shared metadata helper answers asks.
