# Changelog 0.8

📌 **TL;DR:** Archived changes **0.8.0–0.8.243**, 2026-09-23–2026-10-19, grouped by area. Earlier: [0.7](CHANGELOG.0.7.md#changelog-07); next: [0.9](CHANGELOG.0.9.md#changelog-09). The groups retain significant transitions; they do not describe today's contract.

## Delivery and authority

- **0.8.1–0.8.6:** fixed independent topic/tag filtering and made owner-prefixed Personal Group names consistent.
- **0.8.21, 0.8.79–0.8.92:** restore reported invalid records/credentials; recreated names inherited no stale lists or Group membership. Schema 8 bound record ownership to User IDs. Reader cancellation followed a successful commit, not a refused write.
- **0.8.52:** PubSub dropped queue capacity/overflow/TTL settings; copies used their recipients' policies.
- **0.8.169–0.8.171:** forwarding was first deduplicated per name, then replaced by the final ten-channel traversal bound; an eleventh channel failed the whole send. Adding a Deliver-To recipient required management authority over its destination.
- **0.8.183, 0.8.187–0.8.189:** the in-flight recheck added at 0.8.183 was superseded at 0.8.189: authority was checked on admission/restart, not repeatedly during an admitted operation.

## Locks, resources and private values

- **0.8.66–0.8.74:** introduced expiring shared locks with wait, cancellation, extension and audited force release. Expiry woke waiters and a cancelled wait was never granted. Namespace/authority moved from Group membership to the owning record; `/holders` gained holder/expiry data.
- **0.8.70–0.8.74:** added Resource cards and MCP reads through an Agent, MCP Service or HTTPS source. Private values followed Owner/Maintainer/own-Agent authority; an ACL, Group membership or daemon office alone did not grant access.
- **0.8.77, 0.8.86, 0.8.94–0.8.96:** schema 7 added durable per-record string/int/JSON KV with atomic set, increment and JSON operations, followed by web store/value editing. JSON parsing preserved large values and rejected trailing input.

## Roles and clients

- **0.8.97–0.8.110:** daemon-generated owner/Maintainer and assigned application roles reached messages and scripts. Sender-claimed/reserved roles and unauthorized cross-owner role forwarding were refused.
- **0.8.100–0.8.101:** added Python, TypeScript, Go, Rust, PHP and Java clients with correlated calls, bounded serving, read-only private values, KV and locks; example workers/callers selected their own credentials.
- **0.8.168–0.8.180:** added capability asks and `/manages`; fixed alternate reply topics, consume/lock waits, cancellation, credential-safe diagnostics and large-integer handling in clients/MCP.
- **0.8.172, 0.8.195–0.8.200:** CLI parsing accepted piped integers and literal text after `--`, while refusing unknown/misplaced flags and missing values before daemon requests.

## AI launchers and MCP

- **0.8.0–0.8.31:** improved suspension/reconnect handling, kept queues through MCP-face recovery, reported runtime startup failures and printed exact resume commands after server loss. Socket discovery used the account UID; Codex TUI and server received matching enforced modes.
- **0.8.63–0.8.65, 0.8.100–0.8.104:** launchers and dedicated runs used personal sockets, refused relative scripts and retried interrupted links with backoff. OpenCode 2 gained its adapter while the 1.x launcher remained available; pushed Claude receipts rendered as ack/done.
- **0.8.178, 0.8.192:** stopped orphaned Codex pushers, found OpenCode root sessions beyond the first page and handled a plain Claude invocation without launcher identity with a bounded diagnostic exit.
- **0.8.233–0.8.239:** reserved saved-session identities and pinned explicit names, refusing another live reader without rewriting its record. Bindings, reply contexts, agent notes, lock PIDs and palette counters were published as private atomic snapshots; remaining cross-directory startup gaps were documented.

## Managed runner

- **0.8.114–0.8.124:** added strict package/inventory loading, status/reload, args/JSON serving, instances, layered ENV, bounded answers and `original_to` selection. Local launch errors were refused before registration, scripts received no Agent token and two starts could not both serve the same name.
- **0.8.126, 0.8.142–0.8.158:** added default bubblewrap confinement, per-agent cgroups, seccomp and disk limits, all failing closed. Agents were created for the runner's User and kept their own tokens; self-serving kept processes received the required private socket/token. Each person gained a separate runner account; token files used exclusive descriptor-owned writes.
- **0.8.177, 0.8.185–0.8.191:** launches used settings snapshots and actual launch jails; reload/shutdown waited for retiring keepers and bounded descendant stops. Sandboxed agents could not reach account sockets; overlapping/symlinked work paths were refused and disk checked before restarts.
- **0.8.193–0.8.205:** exit 78 represented missing/invalid configuration and stopped kept restarts until reload. A bus outage stayed a restartable failure, not “not configured”; a lost call answer was shown as an unknown outcome rather than a refusal.
- **0.8.203–0.8.225:** added bounded runtime observations, start/stop/restart, persistent enable/disable, host settings and instance creation/deletion. Owner/Maintainers controlled host files, author files were read-only, path traversal was refused and failed deletions could be retried. Root-owned shared code and account-named private state replaced the former layout, which setup refused without deleting operator data.

## Slack and Telegram

- **Slack, 0.8.125–0.8.139:** introduced validated text/JSON methods, aliases, directories, post/read/message operations and subscriptions. Tokens came from bus Secret; responses used one deadline, safe fixed errors, token redaction and bounded retries. Socket Mode acknowledged first, deduplicated and queued events, evaluated filters in a bounded worker and serialized counters/state so deletion stayed deleted.
- **Slack, 0.8.146–0.8.180, 0.8.194–0.8.199, 0.8.215:** packaged checkout-independent bundles, cached directories and configurable posting identity. Skipped failed channel lookups, bounded misses, refused unknown method keys and kept missing secrets distinct from bus outages.
- **Telegram, 0.8.197–0.8.218:** added configuration validation, help/whoami, aliases/default chats, post/read, channel/user lookup, reactions/edits/deletes, receive polling and destination-managed subscriptions. Bus Secret supplied the token; provider retries stayed inside deadlines, unknown keys/aliases were refused, and errors exposed no token/URL.
- **Telegram, 0.8.232–0.8.235:** committed received state before advancing the durable cursor, restored bounded history/directories, absorbed replay once and serialized subscription flushes. Flat agent/filter packages and explicit manifests supported restart-safe forwarding outside the checkout.

## Web console and activity

- **0.8.3–0.8.10, 0.8.15–0.8.16:** added consistent kind-specific lists/forms, Personal/inactive navigation, last-used sorting, a signed-out product page and responsive accessible layouts.
- **0.8.12, 0.8.41–0.8.46:** activity moved from checkpointed one-day rings to per-name calendar days kept for 400 days; a whole-day ledger prevented midnight/restart truncation and supported Day/Week/Month navigation.
- **0.8.32–0.8.51:** TypeScript web replaced the Go dashboard at 0.8.50 under its own account/unit. The former supervised web child and cgroup/bubblewrap model were removed; complete web files entered the release manifest. Cookie/session validation, escaping, asset-method and filesystem restrictions hardened the replacement. Versions 0.8.47–0.8.49 were skipped.
- **0.8.71–0.8.96, 0.8.111–0.8.148:** added Resources, lock/KV management, contextual field help and counted browse filters; updated forms to current identity, role and queue rules while keeping values byte-accurate.
- **0.8.162, 0.8.174, 0.8.196–0.8.231:** kept private editors empty rather than echoing stored values, preserved large integers/line breaks on unchanged saves and refused sign-in return paths to another host. Added runner information and per-agent management with safe action/override forms, clear access denial, meaningful exit reasons and recoverable call outcomes.

## Installation, transport and standalone agents

- **0.8.19–0.8.27, 0.8.51, 0.8.119, 0.8.164:** deployed committed, source-stamped releases; verified complete archives and manifest inventories; restored failed replacements and removed commands/units absent from rollback targets. Samples became idempotent without deleting unrelated records.
- **0.8.54–0.8.62, 0.8.184:** enabled remote TCP and pinned TLS beside HTTP, supplied the web certificate and applied/rolled back TLS changes on existing nodes. Missing or unreadable TLS state was refused instead of downgraded.
- **0.8.105–0.8.109:** renamed shared code `service.d` to `agent.d`, runner notes to `agents/`, counters to `records` and record routes to `/record*`, with short compatibility bridges at the time.
- **0.8.154–0.8.186:** shared ENV parsing became linear and non-evaluating. References in unquoted/double-quoted values were refused by line; single quotes preserved literal dollars without exposing bad secret text.
- **0.8.236–0.8.243:** standalone server-info and systemd helpers gained dedicated nologin accounts, explicit removal, ownership-collision checks and retained registrations. Systemd operations and bounded journal reads followed declared unit sets and delivered role tiers; helpers kept only their own credential.

## Review and failure handling

- **0.8.160–0.8.190:** refused multi-line SSH keys before writes and privilege-escalating account mappings, validated User names before provider/storage work, kept audit targets unambiguous and opened database paths as literal files. Registration rejected unwritable fields; debug/audit fragments were bounded.
- **0.8.173–0.8.181:** interrupted checks stopped and waited for their process groups before cleanup. Queue saves no longer held the bus, warnings were rate-limited per text, and fixtures preserved meaningful privilege-loss, parser-bound and concurrent-state assertions.

The source history reused **0.8.126** for Slack secret hardening and the runner's first sandbox. Both distinct changes are retained above; neither is assigned an invented release number.
