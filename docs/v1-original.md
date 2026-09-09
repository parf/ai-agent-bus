# V1 — Original Ideas and Existing Implementation

Status: V1 is **implemented** (monorepo `/rd/service/agent-bus`, NATS-based) and
being **replaced by V2** — owner is not satisfied with it. Source: Linear, read 2026-09-09.
Primary: [PRF-36 "Ai agent-bus"](https://linear.app/realmo-product/issue/PRF-36/ai-agent-bus)
(2026-08-04, description + 14 comments, all by owner). Secondary: issues listed in §4; usage in §3 from owner.
This file records what V1 is; `00-04` and `HANDOFF.md` are the V2 design.
V1 pain points (the *why* of V2): to be filled in by owner.

## 1. PRF-36 description — the brainstorm (2026-08-04)

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

## 2. PRF-36 comments — chronological

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

## 3. How V1 is used today (owner, 2026-09-09)

Participants on the bus:

| Participant | Kind | Notes |
|---|---|---|
| **Claude Code sessions** | agent, via **Claude channel** = `claude --channel …` (CLI option; an MCP-based channel through which agent-bus pushes messages into a running session) | talk to each other, to Codex sessions, to agents |
| **Codex sessions** | agent (Codex apps) | same |
| **Simple agents** | agent | Slack in/out, Telegram in/out, SMS out, email out, … |
| **Slack reader** | personal/shared service | receives alerts from Slack channels → forwards to `claude-watch` |
| **`claude-watch`** | CLI Claude session | sorts alerts → forwards to **alerters** or the **fixer** |
| **Fixer** | **single** long-lived Claude session, spawned once | consumes **its own agent queue serially**; the one session keeps history/context of what was done, and serial processing **avoids git conflicts** |

Anyone can see what is registered on the bus; that is how sessions find each other.

Alert pipeline (the reference flow):

```
Slack channels → slack-reader → claude-watch (CLI session)
                                   ├→ alerters  (Slack / Telegram / SMS / email out)
                                   └→ fixer (ONE session, its own queue, serial)
```

V2 must keep serving exactly this: session↔session and session↔agent
messaging, a registry everyone can read, and event forwarding chains — with
its own daemon instead of the broker. Required queue property from the fixer
pattern: **one named queue, one consumer, in-order delivery** (a session
owns its queue; events wait rather than fan out).

## 4. What exists in code (V1 implementation, per Linear)

Not verified from source; taken from issue text.

| Item | Evidence |
|---|---|
| Lives in the Radaris monorepo at `/rd/service/agent-bus`; languages Go, Python, shell (+ PHP, TypeScript below); secrets: `.env`, signing keys, **NATS credentials** | RLM-821 |
| `db-tools` (PHP): `ModelTools::find()`, `ModelQueryGuard`, `QueryGuard` (MySQL vs PostgreSQL quoting), read-only txn, acceptance tests | RLM-608, RLM-609 (fixed `178a7eec38d`) |
| `server-tools` (PHP): `ServerTools::execute` runs SSH commands, output capped at 2 MiB, `stream_select` loop | RLM-613 (fixed `1367a5259b0`) |
| `mcp/` (TypeScript, bun): `main.ts`, `web.ts`, `contracts.ts`; `ServiceDefinition{enabledByDefault, alwaysEnabled}`; catalog in `services.d/*.json`; default **HTTP on 127.0.0.1:3333**, loopback = authenticated; `AGENT_BUS_MCP_TOKEN`; `PATCH /api/services/<id>`; **signed envelope**, `event.user`; service ids like `parf:server-tools`; review doc `fable-review-slice2.md` | PRF-26 |
| Hardening backlog: stdio default, token required for HTTP when catalog has destructive tools, `destructive?: boolean` marker | PRF-26 (Backlog) |
| Nothing deployed as of 2026-07-24 (no listener, no MCP process attached to an agent session) | PRF-26 |
| A Claude-session input channel exists on the bus (reference personal service) | HANDOFF §1 |

Adjacent issues: PRF-15 "Use new NATS queue service" (canceled → Future);
RLM-250 tableflip zero-downtime reload (canceled → Future; 2× RAM caveat);
PRF-49 Plan Management service (In Progress; candidate future bus service, not core).

## 5. V1 → V2 mapping

| V1 idea | V2 status |
|---|---|
| NATS as bus, NATS auth/queues/KV | **Dropped.** Point-to-point + in-process bounded queue. |
| Four auth levels (none / NATS / SSH keys / IdP) | **Replaced** by one optional AUTH + key modes (derived / pairwise / static) + local mapping file. |
| SSH key → JWT + refresh token (comment 3) | **Rejected** as "Option B"; derived HKDF keys instead. Nonce/SSHSIG-namespace ideas **kept** for the signed challenge. |
| Per-service user→token map, no Auth | **Kept** as local mapping file / static mode. |
| Group requirements per method; local admin override | **Kept**: service-defined roles + local file overrides AUTH. |
| GitHub/Google IdP; Auth keeps users & groups | **Kept**: GitHub numeric id as main identity, LDAP second; groups/ACL/roles in AUTH. Google dropped. |
| Providers/consumers without registration | **Kept** as publisher/consumer capabilities on a principal. |
| Personal vs public(group) services | **Kept** as personal vs shared. |
| Instances = service + config, spawned on demand; special user; secrets preserved | **Kept**: instance model, `agent-busd` user, sealed private config. |
| Process manager super service (Apache/php-fpm style, from–to rules) | **Kept** as `agent-busd`; from–to enable rules **not carried over**. |
| Statistics server | **Kept** as stats module of discovery (in-memory). |
| Restricted SSH forced commands | **Kept** for admin access (`auth-admin`). |
| Cloudflare Tunnel/Access, mTLS | **Out of core** (customer exposure only). |
| Wire formats (JSON-RPC, gRPC, GraphQL, msgpack multiquery) | **Undecided** in V2; only "JSON and binary payloads" survives implicitly. |
| Service channel + tag reply, "avoid ephemeral channels" | **Not carried over**; relates to V2 open question #1 (event delivery). |
| MCP auto-generated from services | **Kept**: discovery MCP face, pre-filtered tool lists. |
| MCP gateway for existing services | **Not in V2 docs** (partly covered by generic kind + runner adapters). |
| Auto-doc service, RAG service, KV/DB gateways, writers/updaters | **Not in V2 docs** — non-core; existing `db-tools`/`server-tools` are the V1 realisation. |
