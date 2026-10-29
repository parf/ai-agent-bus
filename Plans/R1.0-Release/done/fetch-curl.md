# B.23 scoped curl and fetch allowlists

## Result

**0.9.153**, source `3b6e6669`, verified fast-forward and ancestry on main. Current behavior lives in the [fetch guide](../../../docs/agents/fetch.md#what-it-does). Plain/JSON curl returns bounded lossless HTTP answers; instance lists scope every request/hop, with explicit exact-entry private grants. Existing get forms/extraction remain.

- **Final normal: 1218/0, 47.513 s**, after the final Proxy-* header guard. An earlier green run (50.501 s) predates that guard and is not the final proof.
- **79 focused checks**, 7.668 s; **59 compiling owning mutants assertion-red**, restored green; **fast 14**, 0.627 s. All credited mutants failed a named assertion, with no build/test error, timeout or hang credited.
- Go shipped-agent loader and independent setup/package/outside-checkout/executable/runtime contracts passed; the package still has the same **10 runtime files**. No new dependency or compiled program changed.
- Agent docs structure **5/0**, changed Markdown links **0 bad**. End-user guides/index preserve the instances section and contain no plan IDs, Plan links, build statuses or in-work labels for these features.

The final header sweep found Proxy-Connection missing from the explicit RFC proxy-auth/header list. The final prefix guard refuses every Proxy-* name; its mutation restores that gap and fails the header assertion. Additional fixtures prove numeric pinning, mixed-set refusal, exact private/unusual-port success versus wildcard/no-list refusal, canonical path match-and-send, forbidden redirects with zero extra hits, exact body/header/verb bytes, duplicate/raw/binary answers, 256 KiB plus large-header arithmetic, HEAD/TRACE/custom verbs, and one-hit uncertain POST outcomes. TLS uses disposable certificates and servers; private HTTPS rejects untrusted certificates normally. No real public URL or private service was exercised.

## Review

Read-only reviewer closed `3b6e6669`, verified it is an ancestor of main, VERSION 0.9.153 and byte-identical reviewed fetch source on main. No open finding. Scope: static production diffs of policy/transport/worker/fetch at `9aa04be8`, then deltas through `6b84c035` and `3b6e6669`; not a fresh full read of unchanged B.18 code. Reviewer did not read tests, guide, manifest, acceptance script or changelog and ran nothing. Counts above belong to the implementer.

## Disabled deployment

The manager backed up the old package under `/var/lib/agent-bus/agent-backup/fetch-*`, installed the new package atomically, retained explicit `enabled:false`/`sandbox:on`, reloaded and reported check-deploy **6/6**. The implementer independently verified all **10 installed files byte-match** the committed 0.9.153 package, fetch.py 0755/other files 0644, offline help version/get+curl/allow metadata, the explicit disabled inventory and actual runner disabled state, then ran check-deploy **6/6** again. No live Configuration/Secret write, owner-window or daemon/runner service restart by the implementer. The manager reports pushing the release; the implementer did not push.

Live public/private-service behavior remains separate from fixture evidence. The owner chooses any live private scope. This work enables no live fetch instance and makes no real service call. Own worktree/build/log scratch is removed after this evidence fold.

<details>
<summary>Compiling mutants and their owning assertion checks</summary>

| Mutant | Owning check |
|---|---|
| empty-list | `test_curl.Allowlist.test_omitted_empty_and_invalid_lists_fail_closed` |
| unlisted | `test_curl.Allowlist.test_exact_url_scope_and_default_port_case_idna` |
| wildcard-private | `test_curl.CurlHTTP.test_wildcard_and_no_list_never_reach_same_private_fixture` |
| exact-private-removed | `test_curl.CurlHTTP.test_real_exact_private_post_headers_body_status_and_raw_response` |
| url-path-scope | `test_curl.Allowlist.test_exact_url_scope_and_default_port_case_idna` |
| url-port-scope | `test_curl.Allowlist.test_exact_url_scope_and_default_port_case_idna` |
| url-scheme-scope | `test_curl.Allowlist.test_exact_url_scope_and_default_port_case_idna` |
| unanchored | `test_curl.Allowlist.test_wildcards_are_anchored_and_never_private` |
| canonical-send | `test_curl.Allowlist.test_path_scope_decodes_normalizes_then_sends_same_target` |
| encoded-unreserved | `test_curl.Allowlist.test_path_scope_decodes_normalizes_then_sends_same_target` |
| encoded-separator | `test_curl.Allowlist.test_path_scope_decodes_normalizes_then_sends_same_target` |
| encoded-percent | `test_curl.Allowlist.test_path_scope_decodes_normalizes_then_sends_same_target` |
| pattern-star-limit | `test_curl.Allowlist.test_invalid_patterns_and_literal_limits` |
| allow-count | `test_curl.Allowlist.test_omitted_empty_and_invalid_lists_fail_closed` |
| allow-total-bytes | `test_curl.Allowlist.test_omitted_empty_and_invalid_lists_fail_closed` |
| path-star-private | `test_curl.CurlHTTP.test_wildcard_and_no_list_never_reach_same_private_fixture` |
| unmatched-exact-grant | `test_curl.Allowlist.test_overlap_grants_only_the_matched_exact_entry` |
| mixed-exact-dns | `test_curl.Allowlist.test_exact_dns_trusts_all_private_but_refuses_mixed_answers` |
| zero-network | `test_curl.Allowlist.test_exact_dns_trusts_all_private_but_refuses_mixed_answers` |
| all-dns-answers | `test_policy.Policy.test_every_dns_answer_is_checked_before_selecting_one` |
| redirect-policy | `test_curl.CurlHTTP.test_unlisted_same_host_redirect_never_makes_second_request` |
| redirect-dns | `test_transport.Transport.test_same_host_redirect_rechecks_all_dns_answers` |
| second-resolution | `test_transport.Transport.test_actual_numeric_dial_never_resolves_a_second_time` |
| sent-before-connect | `test_curl.CurlHTTP.test_definite_first_connect_failure_is_not_unknown` |
| sent-never | `test_curl.CurlHTTP.test_oversize_post_answer_is_sent_unknown_exactly_once` |
| body-changed | `test_curl.CurlHTTP.test_real_exact_private_post_headers_body_status_and_raw_response` |
| verb-changed | `test_curl.CurlHTTP.test_real_exact_private_post_headers_body_status_and_raw_response` |
| caller-headers-dropped | `test_curl.CurlHTTP.test_real_exact_private_post_headers_body_status_and_raw_response` |
| host-header | `test_curl.Fields.test_request_header_smuggling_and_case_duplicates_refuse` |
| te-header | `test_curl.Fields.test_request_header_smuggling_and_case_duplicates_refuse` |
| length-header | `test_curl.Fields.test_request_header_smuggling_and_case_duplicates_refuse` |
| header-newline | `test_curl.Fields.test_request_header_smuggling_and_case_duplicates_refuse` |
| case-duplicate-header | `test_curl.Fields.test_request_header_smuggling_and_case_duplicates_refuse` |
| request-header-bytes | `test_curl.Fields.test_request_method_body_and_header_bounds` |
| request-body-cap | `test_curl.Fields.test_request_method_body_and_header_bounds` |
| canonical-base64 | `test_curl.Fields.test_request_method_body_and_header_bounds` |
| empty-scope-propagation | `test_curl.Fields.test_empty_allow_is_sent_to_worker_and_caller_cannot_override_scope` |
| caller-policy-field | `test_curl.Fields.test_request_method_body_and_header_bounds` |
| cross-origin-headers | `test_curl.CurlHTTP.test_origin_change_strips_every_caller_header` |
| origin-port | `test_curl.CurlHTTP.test_same_host_port_change_drops_caller_credentials` |
| cross-origin-replay | `test_curl.CurlHTTP.test_cross_origin_preserved_body_or_verb_is_not_replayed` |
| 303-content-header | `test_curl.CurlHTTP.test_post_303_becomes_get_and_removes_body_content_headers` |
| 303-body | `test_curl.CurlHTTP.test_post_303_becomes_get_and_removes_body_content_headers` |
| 302-nonpost | `test_curl.CurlHTTP.test_put_delete_and_custom_301_302_stay_same_origin` |
| worker-unknown | `test_curl.Fields.test_worker_preserves_unknown_outcome_class` |
| preflight-unknown | `test_curl.Fields.test_helper_preflight_and_popen_failures_are_definitely_not_sent` |
| timeout-definite | `test_curl.Fields.test_curl_helper_timeout_is_uncertain_but_preflight_is_not_sent` |
| helper-input-cap | `test_curl.Fields.test_helper_preflight_and_popen_failures_are_definitely_not_sent` |
| sent-error-definite | `test_curl.CurlHTTP.test_oversize_post_answer_is_sent_unknown_exactly_once` |
| raw-body-whitespace | `test_curl.Fields.test_binary_and_escaped_text_answers_round_trip_with_warning_and_cap` |
| duplicate-response-header | `test_curl.Fields.test_binary_and_escaped_text_answers_round_trip_with_warning_and_cap` |
| raw-status | `test_curl.Fields.test_binary_and_escaped_text_answers_round_trip_with_warning_and_cap` |
| raw-warning | `test_curl.Fields.test_binary_and_escaped_text_answers_round_trip_with_warning_and_cap` |
| curl-response-cap | `test_curl.CurlHTTP.test_oversize_post_answer_is_sent_unknown_exactly_once` |
| private-tls | `test_curl.CurlHTTP.test_private_https_requires_normal_ca_and_hostname_verification` |
| https-downgrade | `test_transport.Transport.test_tls_checks_ca_hostname_sni_and_refuses_downgrade` |
| query-wildcard | `test_curl.Allowlist.test_only_star_is_wildcard_and_query_is_literal` |
| actual-curl-cap | `test_curl.CurlHTTP.test_actual_close_framed_and_chunked_curl_reply_caps` |
| proxy-family | `test_curl.Fields.test_request_header_smuggling_and_case_duplicates_refuse` |

</details>

## Historical plan

The following is pre-implementation history. Current contracts belong only in the linked guide. Review pinned broad bare-host trust, mixed DNS refusal, definitely-not-sent helper preflight, uncertain post-write outcomes, and the final proxy-family refusal.

- ⏳ B.23 fetch: a `curl` method and a per-instance site allowlist ([RD144](../DECISIONS.md#recorded-decisions), [RD145](../DECISIONS.md#recorded-decisions)); xAB
  - Owner, 2026-10-29: "fetch should support curl too; also optional allowed sites/URLs (with `*`) list; this way we can limit some agent to specific sites"
  - `curl`: any HTTP method, caller headers and a body; the answer is the status, response headers and the raw body, opened with the same external-content warning line (RD142), no text extraction. `get` stays as it is
  - Allowlist: optional Configuration on an instance (`#fetch/<name>`); when set, every request and every redirect hop must match one entry, `*` as a wildcard in host or path; when absent, behaviour is today's public-only fetch. An agent is limited to sites by granting it one instance and not the others
  - Private destinations (RD145): only an entry naming an exact host, with no `*`, may reach a private, loopback or non-standard-port address; any `*` entry and the no-list default stay public-only on the existing ports
  - Still refused for every request: userinfo in the URL, HTTPS-to-HTTP downgrade, unbounded bodies, automatic retries; a caller header may not set `Host` or hop-by-hop headers. Redirects re-check the allowlist and addresses per hop
  - Secrets: no stored credentials in this item; a caller may send its own `Authorization` header, which is never carried to a redirect on another host
  - Plan first: xAB writes the method shape, header/body/size bounds and the allowlist matching rules as a plan line for review before code. No wire format is invented here
  - Acceptance: an instance with an allowlist refuses an unlisted host and an unlisted redirect hop; an exact private entry reaches a fixture server while the same address under a `*` entry refuses; `curl` POST with a header and body reaches a fixture and returns status, headers and raw body with the warning line; the no-list instance behaves as B.18; owning mutants fail on assertions; normal gate; deploy disabled
  - **B.23 plan for review (2026-10-29; before code):**
    - **Method/output:** keep all `get` forms and extraction unchanged apart from the optional instance policy. Add plain `curl <VERB> <url> [body...]` (no headers, body is the unchanged remainder after one separator; no shell/shlex interpretation) plus the full JSON form `{"method":"curl","verb":"POST","url":"https://api.example.org/v1/items","headers":{"Content-Type":"application/json"},"body":"{ \"x\": 1 }"}`. `verb` is required, a valid case-sensitive ASCII HTTP token <=32 bytes (not a fixed method enum); no shell or curl executable. Optional headers default `{}` and body defaults empty. `body` is UTF-8 text sent unchanged; optional `body_base64` instead carries arbitrary bytes (strict base64, mutually exclusive with body). Return the fixed external-content warning and blank line, then JSON `{status,headers:[[name,value],...],body,body_encoding}`: preserve duplicate response headers, no extraction/normalization of body bytes, `body_encoding` is utf-8 or base64. Use UTF-8 only when exact decoding and the complete answer fit; otherwise lossless base64. Non-2xx is an HTTP answer, not a local error. Request and raw response bodies <=256 KiB each; complete curl answer <=512 KiB including warning/headers/encoding overhead, no silent truncation. Existing get's 1 MiB source/512 KiB output caps stay. JSON call/envelope <=2 MiB (including escaped request text); helper input <=2 MiB and existing combined pipe cap stays 2 MiB. Pin these fields/types/examples in metadata before parser code.
    - **Configuration/matching:** one optional `allow` string-list, <=64 entries, <=4096 UTF-8 bytes per entry and <=64 KiB total; unknown fields/types refuse before network. Omitted allow means today's public-only policy; explicit `[]` denies all. Entries are a bare host (DNS name, canonical IPv4 or bracketed IPv6, without port/path) or a complete HTTP(S) URL. Bare host matches either scheme and all paths/queries; a URL entry matches its canonical scheme, effective port and entire path/query. Only literal `*` is wildcard, zero or more characters (host may span labels, path may span segments); all other characters, including `?` and brackets in paths, are literal. Anchor the whole match; lowercase/IDNA DNS host and canonical IP/default-port handling are shared by patterns and requests. Use bounded stdlib glob matching with literal escaping, <=16 stars/entry, no user regex. A host pattern `*.example.org` does not admit example.org or example.org.evil. Wildcards in scheme/port and URL userinfo/fragments/controls/zone ids refuse. Every initial request and every followed redirect must match before DNS/connect.
    - **Exact exceptions and path scope:** classify privilege from the matching ENTRY, never just from the requested hostname. A matching entry with NO literal `*` (a bare exact host or exact full URL) permits private/loopback/link-local addresses and ports 1..65535 for that scope; wildcard matches and no-list requests keep all public-address/port checks from B.18. If entries overlap, any matching exact entry grants its explicit scope. Numeric spelling ambiguity, invalid addresses/families/scope ids, unspecified/multicast/broadcast destinations, userinfo and protocol upgrades/tunnels remain refused. For allowlisted URL paths, validate percent escapes, decode unreserved bytes, normalize dot segments, reject encoded slash/backslash/percent/control and repeated separators, then MATCH AND SEND that same canonical target; query is literal unless a path star spans it. This prevents `/allowed/%2e%2e/admin` or double decoding from escaping a path scope. No-list get retains its existing URL behavior. All DNS answers are validated; exact exceptions allow valid private unicast answers, public-only matches reject the entire mixed set if any answer is forbidden. Numeric socket pinning, original Host/SNI and default TLS verification remain, including for private HTTPS.
    - **Headers/redirects:** at most64 caller headers/16 KiB total, <=8 KiB/value; valid ASCII header-token names, visible ASCII/tab values, no CR/LF/NUL or case-insensitive duplicates. Refuse Host, Connection and all hop-by-hop/proxy headers, Content-Length, Transfer-Encoding, Expect and Upgrade; compute framing/Host ourselves. No stored Secret/env credentials. For curl only, a caller may choose Content-Type/Accept-Encoding; return compressed/binary bytes without decompressing, still capped. Five redirect hops, full policy/address checks each time, no HTTPS downgrade. 303 changes to GET except HEAD; 301/302 change POST to GET; 307/308 preserve method/body. Drop body/content headers on a GET conversion. Across any origin change drop ALL caller headers (including Authorization/Cookie and custom API keys), and refuse a redirect that would repeat a body or non-GET/HEAD verb across origins. Never automatically retry a request after connect/write failure, nor open an upgraded protocol/tunnel. Document explicit redirect resubmission versus retry, and that admitted callers can use the instance's configured private scope.
    - **Shared safety/integration:** extend the existing policy/transport/worker/entrypoint rather than a second HTTP path. Keep hard child-process DNS/connect/read deadline, numeric pinning at every hop, bounded response headers/chunk metadata/trailers, strict framing, no proxy env, stripped helper env, private stdin and fixed argv; requests/headers/Authorization never appear in argv, stderr/logs or exception text. Raw replies deliberately may echo caller-supplied credentials, so do not redact/rewrite their bytes; they contain no agent-owned key. Content-free local errors and logs remain. Add config metadata and curl grammar/help/examples, guide and package contract tests. No new dependency or shared runtime file is planned; preserve the existing explicit runtime inventory unless a new file becomes necessary. Agent guides describe use only: no build status, plan IDs/links or approval prose; plans hold rollout/evidence.
    - **Review pins (ae2b35bf):** retain the owner's broad exact-host grant: a bare no-star host admits private unicast on ALL ports/paths for every ACL grantee; the guide must state that this exposes every reachable service on that host and recommend an exact full URL for a narrow grant. Trailing-dot/uppercase/IDNA DNS names canonicalize once to the same host; noncanonical numeric and IPv4-mapped/compatible literal aliases refuse rather than inheriting another host's exception. Response headers retain the existing 16 KiB/64-line cap, so even base64 of 256 KiB fits the 512 KiB answer cap. Caller-header refusals explicitly include TE, Trailer, Keep-Alive, Proxy-Authorization and Sec-WebSocket-*; CONNECT/tunnels and upgrades refuse, TRACE is an ordinary allowed HTTP method, and GET/HEAD may carry a bounded body (HEAD's response body is empty). For 301/302, PUT/DELETE/custom verbs remain unchanged on same-origin redirects; every origin change drops caller headers, and another same-host path still needs a matching allowlist entry. Private HTTPS uses normal system CA/hostname verification, with no insecure flag or caller-provided CA.
    - **Sent/uncertain outcomes:** explicitly connect before marking HTTP bytes as possibly sent. A definite first-hop validation/DNS/connect/TLS refusal is a content-free ordinary error. After ANY request starts, a lost reply, an oversized/framing-refused answer or a forbidden redirect is `unknown: request was sent; ...; do not retry automatically`, never a definite not-sent refusal. A helper deadline/failure without a trustworthy result is conservatively unknown for curl even if it could have stalled in DNS; get keeps its existing error behavior. A valid bounded upstream response, including 4xx/5xx, returns its status/headers/raw body. Tests count exactly one server hit for a too-large POST response and for timeout-after-write, and zero hits for definite pre-send refusal; no automatic replay.
    - **Checks, mutants, landing:** use disposable HTTP/TLS servers only. Cover exact private/localhost/unusual-port success versus same-address wildcard/no-list refusal; bare hosts/exact URLs/wildcard host+path, empty/invalid lists, suffix tricks, encoded traversal, overlap, query/default-port/IDNA canonicalization; zero connection/hit counts for unlisted first/redirect hops; public-to-private redirects require a new exact match; existing get suite unchanged. Curl fixture asserts verb, exact headers/body bytes, status/duplicate headers/body round-trip, binary/base64 and all byte limits, HEAD, non-2xx, redirects and cross-origin credential stripping/body refusal. Asks never parse or run HTTP. Owning compiling assertion-red mutants widen matching, turn a star match private, skip a hop/DNS check, re-resolve, leak redirect credentials, change body, accept hop headers, remove a bound or warning; restored focused suite then one normal gate under slow. Commit plan and get xAB2/ab2-review approval before code; source review before landing. Ask manager for next patch immediately before the gate/bump, rebase preserving peers, verify ff/ancestry and report numbers. Manager installs explicitly disabled and verifies check-deploy; no live/private-service/public URL call, Secret write, service/window restart or push by this work.

