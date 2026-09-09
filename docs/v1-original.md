# V1 — What Exists, the Brainstorm That Followed, and the V1 → V2 Map

Status: V1 is **implemented and runnable** (Radaris monorepo, `/rd/service/agent-bus`,
NATS JetStream) and being **replaced by V2** — owner is not satisfied with it.

Two sources, two dates — do not conflate them:

| Source | What it is | Where |
|---|---|---|
| **PRF-25** "Universal AI session pipeline" (2026-07-23 →) | the **implemented V1**: normative design, protocol, NATS topology, decisions, TODO | `/rd/vhosts/realty/Plans/PRF-25/` (`README`, `PROTOCOL`, `NATS`, `TOOLS`, `LIBRARIES`, `SECURITY`, `OPERATIONS`, `MIGRATION`, `DECISIONS`, `QUESTIONS`, `TODO`, `SERVICES`) · code `/rd/service/agent-bus/` (`README`, `HOWTO`) · [Linear PRF-25](https://linear.app/realmo-product/issue/PRF-25/dvp-universal-ai-pipeline) |
| **PRF-36** "Ai agent-bus" (2026-08-04) | the owner's **brainstorm for what comes after V1** — the seed of V2 | [Linear PRF-36](https://linear.app/realmo-product/issue/PRF-36/ai-agent-bus), description + 14 comments |

§1 is verified from the source tree (2026-09-09, tree dated 2026-08-11). **PRF-25's plan
docs are stale** (owner, 2026-09-09): use them for what V1 *is*, never for what is or is
not deployed. §2–3 are the brainstorm. §4 is how V1 is used. §5 maps V1 → V2. §6 lists
design weaknesses V1's own docs admit.
❓ V1 pain points *as the owner sees them* are not written down. *Settled by:* owner.

## 1. V1 as built (PRF-25)

**Idea.** Every named participant — a Claude/Codex session, a Telegram reader, an SMS
writer — has a stable address `(user, channel)` and a **durable inbox** on a global
NATS JetStream. Anyone can send while the target is offline; the consumer resumes after
restart and ACKs only after processing. An MCP server hides all of it from agents.

| Aspect | V1 |
|---|---|
| Transport | one standalone **NATS JetStream** (`nats.ny.7w7.us`, dedicated Account `AGENT_BUS`, `replicas=1`, no HA); Core NATS only for lossy discovery/alive |
| Streams / KV | 4 streams (`CHANNEL_EVENTS` work-queue inbox, `CHANNEL_DEAD`, `EVENT_TOPICS` fan-out, `DELIVERY_EVENTS`) · KV: `CHANNEL_REGISTRY`, `CHANNEL_LIVENESS`, `SOURCE_OWNERSHIP`, `EVENT_DEDUP`, `EVENT_OUTPUTS`, `EVENT_INDEX`, `ADAPTER_PENDING` + per-adapter correlation/state/journal buckets |
| Address | `(user, channel_name)`; `user` = **signing principal / service-group** (`parf`, `prod`), not a person. Subjects `channel.<user>.<channel>.inbox`, `event.<user>.<topic>` |
| Envelope | versioned JSON; `user/from_channel → to_user/to_channel`, `source`, `type`, opaque `payload`, `ts`, optional `deadline_ts`, `reply_to`, `reply_channel`; `event_hash` = xxh3-63 of exact wire bytes = message id / idempotency key |
| "Signature" | `sign` = keyed **xxh3 63-bit** over exact bytes (`\HB::hash63`). **Not a MAC**: attribution to a group inside an already-authenticated NATS transport; all holders of a group key are indistinguishable. Authority = NATS credentials + subject ACL |
| Trust | per listener: `AGENT_BUS_SIGN_KEYS {user:{key_id:secret}}`, default-deny `AGENT_BUS_TRUSTED_USERS`, `AGENT_BUS_SEND_RULES` (signer→destination matrix). Local `.env` files only |
| Delivery | at-least-once, explicit ACK, durable dedup (CAS `processing/completed`, 37 d), output journaled **before** ACK, dead-letter with exact original wire + replay, `delivery.*` observations, strict ordering (`MaxAckPending=1`) by default |
| Request / reply | durable both ways through inboxes; reply → signed sender address with `reply_to=<event_hash>`; optional `deadline_ts` (expired request never runs; `FAIL:timeout` published before ACK) |
| Registry | durable registration + separate TTL liveness (heartbeat 10 s, lease 30 s, CAS ownership; incompatible duplicate owner rejected). Sessions are **directory-scoped**: `claude-rd-vhosts-realty` = `claude(/rd/vhosts/realty)` |
| Data classes | `standard` / `sensitive` / `restricted` per channel → retention and metadata-only dead letters |
| Agent faces | **notifier-claude** (Claude Code Channels: `--dangerously-load-development-channels`, `notifications/claude/channel`, `agent_bus_reply` tool) · **notifier-codex** (Codex App Server over private unix socket, `turn/steer|start`, `thread/resume`; modes `notify-only / ask-before-run / auto-run`) · **agent-sync** (Codex ↔ Claude one-shot RPC, fresh read-only process per request) · **MCP direct inbox** (`inbox_receive` / `event_ack` / `event_reply`) |
| MCP control plane | Bun + TS, stdio and Streamable HTTP (`127.0.0.1:3333`), web admin, `/metrics` Prometheus, in-memory 24 h / 14 d stats, enabled-only discovery, **conf.d `services.d/*.json`** descriptors (metadata + static resources, never code); one MCP process = one `user` identity |
| Adapters | Telegram (Python), Slack (Bun), Email (Python), SMS (PHP, Telnyx), Discord (Bun), ChatGPT Workspace Agent (Bun) — each `read | write | read-write` role of one package |
| Local sources | **failed-tests**, **post-commit**, **git-push-bridge** (Redis → JetStream): fsync a local outbox first, a supervised daemon publishes and clears after JetStream ACK |
| Tools | `db-tools` (PHP, ORM/SQL read-only, confirmed process kill) · `server-tools` (PHP, `parf` only, signed request/reply, exact confirmation) |
| Languages | Go reference lib + CLI (`/rd/bin/agent-bus`, multicall `agent-pub/sub/request/reply/channels/sessions/health`), Python, Rust (protocol only), TS/Bun, PHP, sh. Layers everywhere: `protocol` → `ports` → `core` → CLI/MCP/adapters; only `transport/nats` touches NATS |
| Handler contract | one signed envelope on stdin; exit `0` ACK · `75` retry · `65` dead-letter; `--reply-output` publishes correlated reply before ACK; handler never sees NATS creds or keys |

**In use today** (owner, 2026-09-09): `ai-claude-watch` with its **fixer** and
**reviewer** sessions, Claude/Codex session channels, the simple in/out agents (§4).
PRF-25's `TODO.md` / `SERVICES.md` rollout status is stale and is not repeated here.

## 2. PRF-36 description — the brainstorm (2026-08-04)

Participants on the bus:
- **Services** — registered on a bus.
- **Consumers** — use services; receive bus events **without registration**.
- **Providers** — emit bus events **without registration**.

Identity: SSH keys by default, or ask to generate.

Optional services:
- **Statistics server**
- **Auth service** — OAuth2-style: user→role mappings, users' pubkeys, user info
- JSON and binary payloads; encoded payloads

Channels: *avoid ephemeral channels*. A service defines its own channel plus a
tag; replies go to the caller's channel with the tag (marked "discuss").
Personal events · pub/sub events.

Scope: **personal services** · **public (group) services** (group = `prod`,
project name…).

Instances: service + config; may be spawned on demand. Special user to start
services; protect services from each other. Service secrets encoded and preserved.

**Process manager super service** — spawns services like Apache/php-fpm;
from–to enable/disable rules; on demand or predetermined.

MCP: auto-generated from available services on the bus; **MCP gateway** to
connect and publish existing services onto the bus.

RAG: implement as a service.

KV storages / data services: NATS KV · Kvrocks/Redis/memcached · plain
directory (SQL queries à la sql-cacher, Parquet, CSV, JSONL) · **DB gateways**:
safe read-only via granted user + prepared statements, structure caches,
process control (process list, kill, load check), writers/updaters.

## 3. PRF-36 comments — chronological

| # | Time | Idea |
|---|---|---|
| 1 | 05:04 | Wire formats considered: JSON-RPC, gRPC, JSON POST, GraphQL, "Parf's msgpack multiquery". |
| 2 | 05:05 | Special **auto-doc service**. |
| 3 | 05:14 | Pasted design (AI-generated): **SSH key → token**. Challenge nonce (single-use, 60 s) signed with SSHSIG (`ssh-keygen -Y sign -n <ns>`), server verifies against registered key, mints **JWT (~15 min) + refresh token**; services validate JWT statelessly; tables `users / ssh_keys / groups / memberships`; revocation = short expiry + server-side refresh tokens + `revoked_at` on key. |
| 4 | 05:16 | A service may carry its own **user→token mappings; no Auth service needed**. |
| 5 | 05:20 | With **NATS user-owned queues** no Auth is needed at all. |
| 6 | 05:21 | Roles via **NATS authorization** (ADMIN/SERVICE permission templates on subjects) and **scoped signing keys** (key-A = reader template, key-B = writer template). |
| 7 | 05:29 | Service defines **group requirements per method** (admin-only methods). **Local config overrides**: starting locally, "only me is @admin", overriding/extending the global admin group. |
| 8 | 05:39 | Remote access to Auth: **restricted SSH port-forward** (`restrict,port-forwarding,permitopen="localhost:8080",command="…sleep infinity"`), alt. **mTLS API gateway**. |
| 9 | 05:46 | **Cloudflare Tunnel + Access** for customer-facing daemons: no open ports, SSO or service tokens, dashboard revocation/audit. |
| 10 | 05:48 | Cloudflare Access SSO: email OTP first, external IdP (Google/GitHub/Okta…) later. |
| 11 | 05:56 | **Level 0 — No Auth.** Simple setup, local use, NATS auth. **Personal Auth**: one user-token shared by all services. |
| 12 | 05:57 | **Level 1 — Auth with NATS.** |
| 13 | 05:58 | **Level 2 — External Auth: SSH keys.** |
| 14 | 05:58 | **Level 3 — External IdP** (Google, GitHub…); Auth server keeps allowed user lists and groups. |

The ladder in 11–14 is the V1 answer to "Auth is optional": four escalating
levels rather than V2's single optional AUTH with pluggable key modes.

## 4. How V1 is used today (owner, 2026-09-09)

Participants on the bus:

| Participant | Kind | Notes |
|---|---|---|
| **Claude Code sessions** | agent, via **Claude channel** = `claude --channel …` (CLI option; an MCP-based channel through which agent-bus pushes messages into a running session) | talk to each other, to Codex sessions, to agents |
| **Codex sessions** | agent (Codex apps) | same |
| **Simple agents** | agent | Slack in/out, Telegram in/out, SMS out, email out, … |
| **Slack reader** | personal/shared service | receives alerts from Slack channels → forwards to `ai-claude-watch` |
| **`ai-claude-watch`** | CLI Claude session | sorts alerts → forwards to **alerters**, the **fixer** or the **reviewer** |
| **Fixer** | **single** long-lived Claude session, spawned once | consumes **its own agent queue serially**; the one session keeps history/context of what was done, and serial processing **avoids git conflicts** |
| **Reviewer** | long-lived Claude session | reviews what the fixer (or a commit) produced; same own-queue pattern |

Anyone can see what is registered on the bus; that is how sessions find each other.

Alert pipeline (the reference flow):

```
Slack channels → slack-reader → ai-claude-watch (CLI session)
                                   ├→ alerters  (Slack / Telegram / SMS / email out)
                                   ├→ fixer     (ONE session, its own queue, serial)
                                   └→ reviewer  (ONE session, its own queue)
```

This is **current mechanics, one usage among many** — agent-bus is a
universal mechanism, not built around this flow. V2 must still be able to run
it: session↔session and session↔agent messaging, a readable registry, event
forwarding chains, and a queue owned by one consumer with in-order delivery
(the fixer pattern).

## 5. V1 → V2 mapping

| V1 idea | V2 status |
|---|---|
| NATS JetStream as bus; 4 streams + ~15 KV buckets; standalone, no HA | **Dropped.** `agent-busd` is the broker; in-memory bounded queues; registry live in the daemon, snapshotted to git. |
| Four auth levels (none / NATS / SSH keys / IdP) | **Replaced** by one optional AUTH + key modes (derived / pairwise / static) + local mapping file. |
| SSH key → JWT + refresh token (comment 3) | **Rejected** as "Option B"; derived HKDF keys instead. Nonce/SSHSIG-namespace ideas **kept** for the signed challenge. |
| Per-listener `SIGN_KEYS` + `TRUSTED_USERS` + `SEND_RULES`; `user` = service-group | **Kept** as the local mapping file / static mode — but the token names **one principal**, not a group. |
| Group requirements per method; local admin override | **Kept**: service-defined roles + local file overrides AUTH. |
| GitHub/Google IdP; Auth keeps users & groups | **Kept**: GitHub numeric id as main identity, LDAP second; groups/ACL/roles in AUTH. Google dropped. |
| Providers/consumers without registration | **Kept** as publisher/consumer capabilities on a principal. |
| Personal vs public(group) services | **Kept** as personal vs shared. |
| Instances = service + config, spawned on demand; special user; secrets preserved | **Kept**: instance model, `agent-busd` user, sealed private config. |
| Process manager super service (Apache/php-fpm style, from–to rules) | **Kept** as `agent-busd`; from–to enable rules **not carried over**. |
| Statistics server; MCP in-memory 24 h / 14 d buckets + Prometheus | **Kept** as stats module of discovery (in-memory ring buffers, Prometheus first). |
| Restricted SSH forced commands | **Kept** for admin access (`agent-bus auth admin`). |
| Cloudflare Tunnel/Access, mTLS | **Out of core** (customer exposure only). |
| Wire formats (JSON-RPC, gRPC, GraphQL, msgpack multiquery) | **Decided**: JSON + optional msgpack. |
| Service channel + tag reply, "avoid ephemeral channels" | **Kept** as V2 messaging: per-agent queue + topic + tag + reply-to (`03` Messaging). |
| MCP control plane (Bun): enabled-only discovery, conf.d `services.d/*.json` metadata, web admin | **Kept**: `agent-busd` is an MCP server; docs/tools generated per client from registrations (`03`). conf.d descriptors → registration records. |
| MCP gateway for existing services | **Not in V2 docs** (partly covered by generic kind + runner adapters). |
| Auto-doc service | **Kept**, folded into `agent-busd`'s MCP server (generated docs per client). |
| RAG service, KV/DB gateways, writers/updaters | **Deferred, non-core** (2026-09-09) — later as ordinary bus services; existing `db-tools`/`server-tools` are the V1 realisation. |
| `sign` = keyed xxh3-63 (attribution, not a MAC); no transport encryption without NATS TLS | **Replaced**: Ed25519 identities, HKDF access keys, AEAD-encrypted sessions (`02`). |
| Offline delivery: send to a down participant, it reads the backlog on return | **Kept**: a registered agent's queue accepts messages while it is down, bounded by TTL and size; `ring` / `strict` overflow (`03`). |
| Survives a bus restart | **Kept**: in-memory state dumped to Parquet on graceful restart, optional periodic dump for crashes (`00`). |
| Content-derived `event_hash` as message id; dedup by it | **Kept** as `message_id`, unique per channel, assigned by the bus (`03`). |
| Delivery observations `accepted / processing / succeeded / failed` | **Kept, lighter**: optional `ack` (received) and `done` (processed) receipts to the sender (`03`). |
| Dead letter + replay | **Dropped by design**: no dead letter. Any flow that needs it gets its own WAL. |
| `deadline_ts` on requests; `FAIL:timeout` published before ACK | **Kept, lighter**: optional TTL per message; expired messages are dropped and counted, never delivered (`03`). No timeout reply. |
| Data classes `standard / sensitive / restricted` → retention, metadata-only dead letters; payload never in logs/UI | **Superseded by encryption**: bodies are end-to-end encrypted, the bus and its dashboard see envelopes only; consumed = gone. Admin-only debug trace per service (`02`, `03`). Retention is TTL + bound per topic. |
| Directory-scoped sessions `claude(/rd/vhosts/realty)`; heartbeat 10 s / lease 30 s | **Kept** in spirit: instance = `unique-name@host`, heartbeat, K missed → down (`03`). |
| notifier-claude (Channels), notifier-codex (App Server), agent-sync | **Borrowed**: one push adapter per runtime — Claude Code, Codex, ❓ OpenCode (Z.AI); ChatGPT pull-only (`04`). agent-sync becomes ordinary send/receipt between two sessions. |
| Adapter `read / write / read-write` roles of one package | **Partly kept** as publisher/consumer capabilities; the runner (`04`) supervises the processes. |
| Local-outbox writers (failed-tests, post-commit, git-push-bridge) | **Not in V2 docs**; the pattern survives as ordinary publishers with their own WAL. |
| Layered libs `protocol → ports → core → transport`; Go, Python, Rust, TS, PHP, sh | **Kept**: Go first; client libs Go, PHP, Rust, JS, Python (`04`). |
| Handler exit codes `0 / 75 / 65`, envelope on stdin | **Not in V2 docs**; the runner's shell adapter (`04`) is the natural home. |
| Listener `trusted_users`; adapters must not re-sign outside text as a trusted `agent.request` | **Replaced**: the bus delivers verified sender + on-behalf-of and nothing more; the receiver decides (`03`). No message kinds. |
| Destructive tools (`db_process_kill`, `server_service_action`) demand an exact confirmation token | **Kept as a hint only**: the service description marks methods destructive; the MCP face passes it through; confirmation is the caller's business (`03`). |

## 6. Weaknesses V1's own docs admit

Design-level, from PRF-25 `SECURITY.md`, `DECISIONS.md`, `fable-nats-review.md`; **not**
owner-stated pain points (those are still ❓ above). Deployment status is left out — the
plan docs are stale.

- `sign` is not cryptographic; no per-person identity, no third-party proof. A real profile (HMAC/Ed25519) was left for "later".
- Encryption in transit is not part of the protocol; it depends on NATS TLS being configured per credential.
- Authorization is NATS subject ACL, hand-written per role credential; the bus itself has no notion of who may talk to whom beyond `trusted_users`.
- One standalone NATS host by decision: no failover. Bus down = everything down.
- Operational surface: 4 streams, ~15 KV buckets, retention sweeper, daily off-host backups, quarterly restore drill, per-role env files, systemd templates — for a low-volume bus.
- Trust config duplicated in every listener's `.env`; no central identities or groups.
- Docs are large and in two places (plan dir + code dir); PRF-25 README alone is 44 KB.
