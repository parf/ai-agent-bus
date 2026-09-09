# agent-bus — Handoff for Claude Code

Repo: `github.com/parf/ai-agent-bus` (private). Owner: parf (Go / Linux infra,
Realmo). This document captures **everything discussed so far**, including
ideas that did not make it into `docs/`. Read this first, then `docs/00-04`.

Status: **design phase, general ideas only — no data models, no code yet.**
The owner explicitly deferred data models. Do not invent schemas until asked.

---

## 1. Where things stand

### Existing system (what is being replaced)
- Many agents/services publish info about themselves → service discovery.
- Services communicate via **NATS**.
- Every service keeps its own `user → token` map and its own service-defined
  access level.
- A Claude-session "input channel" already exists on this bus (sessions talk to
  each other and call services). It is the reference **personal** service.

### Decisions made (do not reopen without reason)
| Topic | Decision |
|---|---|
| Broker | **Drop NATS entirely.** Point-to-point connections; per-service in-process bounded queue. |
| Core services | AUTH/Config, Service Discovery (+ health-checker, stats modules). **Discovery (registrations + in-memory queues) is the required minimum, with no AUTH in it**; AUTH and the rest optional. *(revised 2026-09-09, was "all optional")* |
| Access key model | **Option A**: opaque-style key, AUTH is source of truth, **60-min lifetime**, AUTH needed only on first contact. Implemented as *derived* keys (see §3). |
| AUTH replication | **master/slave**, pull-based, **crypto-signed generation-id**. Raft/consensus = overkill. Changes: a few per week. |
| AUTH availability | run **2+ replicas**. |
| Never-expiring keys | **Required** (personal agent-bus with no AUTH at all) → pairwise + static modes. |
| Identity | **Ed25519 everywhere** (users, services, instances). |
| Main user id | **GitHub** (numeric user id); LDAP as second directory. Fetched only at enrollment / explicit refresh. |
| Storage | **SQLite** default; MySQL/PostgreSQL optional. |
| Service kinds | generic, agent, consumer, publisher (publisher/consumer are roles on a principal). |
| Personal/shared | terms are **personal** and **shared**; **shared is default**. |
| Runner | **`agent-busd`** (daemon), **`agent-bus`** (CLI), **`ab_`** MCP tool prefix only. Keyword `agent-bus` everywhere else. |
| Admin access | SSH with admin's own Ed25519 keys, `authorized_keys` forced command → `auth-admin`. |
| Chaining | AUTH and Discovery accept an **upstream**; local first, unresolved forwarded up. |
| Encrypted private config in AUTH/Config | **Yes** — sealed to the instance's own key. |
| Docs | Keep as several small files (`docs/00…04`), main ideas only. |

### Open questions (not yet decided)
1. **Event delivery without a broker** — where does a `curl`-published event *land*, and how do consumers pull it (from the target agent's queue? buffered by AUTH/Discovery?). Topic namespace for publish/consume capabilities.
2. **AUTH and Discovery: one daemon with two roles, or two daemons?** Leaning one.
3. **Instance identity** — `host+pid` vs persisted UUID (restart semantics).
4. **MCP method info** — store raw MCP `tools` JSON and pass through, or validate at registration.
5. **Delegation** — when service A calls B for user U: pass U's key/token (per-service `aud`) or A's own identity? Leaning per-service keys → U's key for B.
6. **Role assignment to groups** — should be allowed (owner agreed roles use the same user/group expression pattern), confirm in doc.
7. **Master secret distribution** — out-of-band file (start here) vs sealed per replica in the bundle (later).
8. **Language** — owner's daemons are Go; unconfirmed for this project but assume **Go**.

---

## 2. Identity

- Principal = user | service | instance, each with an Ed25519 key.
  Namespaced ids: `github:<numeric id>`, `ldap:<entryUUID>`, `svc:<name>`,
  possibly `static:<name>`. One human may hold several principals; a grouping
  "person" record was considered and **deferred**.
- Authentication = signed challenge (single-use nonce, ~60 s, bind signature to
  purpose — e.g. `ssh-keygen -Y sign -n agent-bus` namespace — to block replay
  across contexts). Directories supply pubkeys only; they never authenticate.
- **Ed25519 only.** Filter out RSA/ECDSA at fetch time and tell the user which
  keys were skipped (pairwise mode needs Ed25519→X25519).

### GitHub (verified live, 2026-09)
- `GET https://api.github.com/users/<login>` → `id`, `login`, `name`, `avatar_url`, `email` (if public). Use **numeric `id`** as principal id (logins can be renamed).
- `GET https://api.github.com/users/<login>/keys` → per key: `id`, `key`, **`created_at`**, **`last_used`** (the public endpoint does return these, contrary to the docs excerpt). `https://github.com/<login>.keys` is the plain-text fallback.
- Use of the fields: `created_at` → "new key on privileged principal" alert; `last_used` → stale-key pruning (e.g. ignore > 12 months) and liveness hint; `id` → rotation vs addition. Track our own `last_used` separately.
- **Enrollment-time only**: fetched when a person is added or an explicit refresh is requested; keys are pinned in the signed generation. No runtime dependency, no periodic polling, no rate-limit concern. Consequence: a key deleted on GitHub stays valid until refresh — accepted.
- If ever fetched more often: use a token (5 000 req/h) + `If-None-Match` ETag (304s are free), cache last-good keys.
- **Global services**: any developer has a GitHub key → self-service enrollment: claim `github:<login>` on first contact, one-time fetch, prove possession → principal with default role. Per-service policy: **open** (auto-enroll, minimal role) vs **closed** (queue for admin approval).

### LDAP
- OpenSSH-LPK: `sshPublicKey` attribute; also `displayName`, `mail`, `jpegPhoto`. Stable id = `entryUUID`, not `uid`. Same normalized principal record.

---

## 3. Access keys and sessions

### Three key modes, one wire protocol
| Mode | `access_key` | Expiry | AUTH |
|---|---|---|---|
| derived | `HKDF(master_secret, "ak" \| user \| service \| epoch)`, `epoch = floor(now/3600)` | 60 min | once |
| pairwise | `HKDF(X25519(my_priv, their_pub), "pairwise" \| sorted(fp_a, fp_b))` | never | no |
| static | pre-shared key in both configs | never | no |

- Derived keys are **deterministic** → every AUTH replica computes the same key; no shared token store. Accept current **and previous** epoch across the boundary. Shrink epoch (e.g. 15 min) if faster revocation is needed — same design.
- Do **not** put `level`/roles into the key derivation (a role change would break live sessions); deliver roles as metadata.
- `master_secret` lives only on AUTH replicas. Rotate with a `key_version` prefix in the HKDF label, accept both for one epoch.
- Pairwise = personal/standalone mode and break-glass path. Static = fallback for keyless parties (scripts, webhooks). A service may accept several modes; the handshake carries `key_mode` + identifiers (`user_id`/pubkey fp, `service`, `epoch` or none).
- Ed25519→X25519: libsodium `crypto_sign_ed25519_pk_to_curve25519`, Go `filippo.io/edwards25519`.

### Session encryption (Kerberos-like)
- Handshake: client `{user, service, c_nonce}` → server `{s_nonce}` (server does the one-time AUTH lookup in between if user unknown).
- `session_key = HKDF(access_key, "sess" | c_nonce | s_nonce)`.
- AEAD per message (XChaCha20-Poly1305 or AES-256-GCM), per-message counter in associated data for replay protection. Never use `access_key` raw as the cipher key.
- Transport: anything direct — TCP, WebSocket, unix socket. No TLS/PKI needed.

### The one AUTH call a service makes
```
→ who is this user (for me)?          (signed by service key)
← access: ok|denied, roles: [...], key: <access-key>, gen: <n>
```
Cached for the epoch. Roles/access/key all take effect on next epoch.

### Local mapping file (AUTH optional)
- Each service may have `principal → {access, roles[, static key]}`; **local first, then AUTH**. Modes: file only / AUTH only / both (local overrides for owner, break-glass admin, peer services).
- The file can be populated the same way AUTH does it: `github:parf` → fetch once → pin.

---

## 4. AUTH / Config — policy

- **Groups** compose from groups: `&`, `|`, `!` (e.g. `eng & !contractors`).
- **ACL** (who may access) and **roles** (what they may do) are both expressions over users/groups, evaluated by **one engine**. Keep them as **two separate layers**.
- Roles are **service-defined strings** (admin, manager, read-only…). AUTH stores/resolves, never interprets. Roles assigned with the same expression pattern as groups.
- **Resolved at login**; services never see groups or mappings (admin-only).
- **Ownership**: owner = expression. Tiers: `owner` (everything incl. ACL and adding owners) and `maintainer` (definition only). Owners use org groups but cannot create groups or grant beyond their own service. Personal services are owned by their user. Ownership changes are generation data.
- Users create/control their own services without admin; admin's job = identities + org groups.
- **Encrypted private config**: instance encrypts its config (e.g. IMAP creds) to its own key (age-style sealed box), pushes blob to AUTH/Config; AUTH stores opaque bytes + owner + instance id. Boot = key + binary → fetch + decrypt. Versioned, owner-pushed, **not** part of the signed generation. Local file remains default. Multi-instance sharing = encrypt to each key or share a key.

---

## 5. Replication — signed generations

```
bundle { gen, prev_gen, created_at, payload, payload_hash }
signature = Ed25519(offline_signing_key, gen | prev_gen | created_at | payload_hash)
```
- Enforced by replicas **and** services: valid signature; `gen > current` (anti-rollback; identical gen+hash = no-op); `prev_gen == current` (gaps: reject or log — decide).
- **Signing key is offline** (admin machine, `authctl`), never on servers → any replica can be master; failover = pointer flip; compromised replica can only serve stale-but-valid.
- Workflow: edit → `authctl sign --gen N` → push to master. Config-as-code; signed bundles can live in git as audit trail.
- **Master/slave, pull**: slaves `GET /bundle?since=<gen>` every 30–60 s (304 if unchanged); optional push on change. Reads (key issuance, lookups) served by **any** replica (deterministic); writes only via master. Master down → reads continue; promotion is manual flag flip.
- Every response carries `gen`; services refetch config when they see a newer one. Key derivation does **not** include `gen`.
- Payload = principals, pubkeys, services (definitions), groups, ACL/role expressions, ownership, admin keys, identity-source config, optional `next_signing_pubkey` for rotation. **Not** in payload: instance health/stats, encrypted private configs.
- **Consistency window** (write it down): revoked access can be honored for up to poll interval (lagging slave) + one epoch (service cache). Optional `revoked_users` list in bundle checked on every session start for immediate effect on new sessions.

### Admin over SSH
- Core services run as dedicated user (e.g. `agent-bus`/`auth`): nologin shell, no sudo, home 0700.
- `authorized_keys`: `restrict,command="/…/auth-admin <admin-name>" ssh-ed25519 …` (optionally `from=`). Identity bound to key.
- `sshd_config`: `Match User auth` → `ForceCommand`, `PermitTTY no`, `AllowTcpForwarding no`, `AllowAgentForwarding no`, `X11Forwarding no`, `PermitUserEnvironment no`, `PasswordAuthentication no`. `ExposeAuthInfo yes` to log the key fingerprint.
- `auth-admin`: parses `$SSH_ORIGINAL_COMMAND` against a fixed verb grammar (`bundle show|push|history`, `user list`, `service list`, `status`, `replica-sync`); reads signed bundle from **stdin** and **still verifies signature + gen** (SSH gates who may talk; signature gates what config is real); append-only audit log `ts admin fp verb gen result`.
- Admin pubkeys may live in the bundle → regenerate `authorized_keys` on push; never remove the last admin key; keep a **break-glass key offline**.
- Master→slave sync can itself be SSH with a `replica-sync` forced command.
- Test the lockdown: `ssh auth@host bash`, `-L`, `-A`, `-t` must all fail.

---

## 6. Services

### Kinds
| Kind | Reachable | Registered by | Health | Signs events |
|---|---|---|---|---|
| generic | yes (host/port, unix socket, HTTP) | a user, on behalf of existing thing (server/ip/port, description, optional MCP method info) | health-checker polls per **hints** | no (no key) |
| agent | yes | itself, own key | self-reports health + stats | yes, as service |
| consumer | no (pulls) | itself or user | none | no |
| publisher | not a service; an identity that signs events (`curl` + user key) | — | none | yes |
- Publisher/consumer = **roles/capabilities on a principal** (`publish:<topic-glob>`, `consume:<topic-glob>`), not service objects.
- generic vs agent differ only in record owner and health mode → same table, `kind` column (when models are designed).

### Personal vs shared (default shared)
- **personal**: runs *as* a user (mail-reader with configured account, Claude-session input channel). Owner = user; default audience = user; lifecycle follows user; usually one instance per user; typically runs on the user's own machine (why AUTH-optional matters). Key: user's key (ephemeral sessions) or **own key** (recommended for long-running: narrower access, individually revocable). When a personal service calls another service, it is the user calling — no separate delegation.
- **shared**: runs as `svc:<name>` for many; explicit ACL; long-lived; health-checked.
- Alternative names considered and rejected: user-scoped/system, bound/unbound, delegate/proxy, single-/multi-tenant.

### Service vs instance
- Service = kind: code, description, declared roles, health hints, optional MCP method info. Instance = service + **private config** + a place it runs. Private config by default kept by the instance itself (JSON or whatever it likes); optionally sealed in AUTH/Config (§4). Instances register, heartbeat, vanish; service definition is signed/rare-change.

### Service Discovery ("service discovery" is the preferred name over "registration")
- **Optional**: if you know where something lives, talk to it directly.
- Registration carries **health hints**: HTTP endpoint + expected status, TCP connect, unix-socket ping, command, interval, timeout. **Unix-socket services are first-class** (`unix:/path`).
- **Audience** per service: users, services, or org groups (from AUTH) who may see/use it → discovery is personalized; MCP tool lists pre-filtered.
- Two faces: **web service** (humans, curl, dashboards) and **MCP server** (agents ask "what can I use, how").
- Same replicated generation-based core as AUTH; live state separate.

### Health-checker & Stats (optional modules of discovery)
- Health-checker is itself an agent (registered, replicated 2×, holds a `health` role on polled services); generic → probe per hints; agent → heartbeat, K missed → down.
- Stats: agents' heartbeats carry a small metrics blob; probe results (latency, up/down) are the stats for generic. Kept **in memory** (ring buffers, last N hours, fixed resolution). Human face via discovery API (`/stats/<service>`) + dashboard with numbers/sparklines, audience-filtered. Forwarding: **Prometheus `/metrics` first** (Grafana reads it), OTLP/StatsD secondary. Restart = empty window (accepted).

### Chaining (upstream)
- AUTH and Discovery accept an upstream (which may have its own). Resolution: local file → local service → upstream → …; first hit wins. Applies to identities, ACL/roles, service lookups.
- Personal on laptop, team on team node, company/public upstream. Nothing pushed up; definitions never leak upward (queries do).
- Each hop pins upstream's signing key; answers are signed generations (middle hop can fail to forward, not forge). Local shadows upstream by design; warn on shadowing writes. Namespaced ids (`team/ci`, `company/mail`). Cache upstream answers with epoch/gen rules; unreachable upstream → cached or unresolved, never wrong. Derived keys don't span levels (no shared `master_secret`) → cross-level uses pairwise or upstream-issued keys. Discovery MCP face merges levels, tagged by origin.

---

## 7. Runner — `agent-busd`

- pm2 / php-fpm-style supervisor **plus** bus integration **plus** sandboxing (on by default, optional).
- **Supervise**: spawn, restart w/ backoff, stop, reload, log capture, exit codes; many children.
- **Adapt**: MCP servers (stdio) → MCP-capable services; HTTP/REST/any APIs → registered with hints; shell processes (stdin/stdout) → request/response or stream services; ad-hoc spawn/control via runner API.
- **Represent**: registers each child as an instance, heartbeats/stats on its behalf, holds & injects child key + private config (via env/fd, never plaintext on disk inside sandbox). Children need not know the bus exists.
- Runner is itself an **agent**: own key, self-registers, controllable over the bus under owner's ACL.
- Runs as **separate user** (default for shared/server; privileged installer once, no root at runtime; optionally per-tenant users) or **current user** (personal, zero setup).
- **Sandbox profile per child**, default on. Backend by environment: systemd present → `systemd-run` (cgroups + `Protect*`/`Private*`/seccomp/caps, declarative); no systemd or unprivileged → `bubblewrap`; else raw `unshare`; firejail lowest (CVE history); `off` allowed explicitly. Resource limits via cgroups (CPU/mem/pids/io).
- Default profile: private `/tmp`, read-only system, only child's work dir writable, no network unless declared, pids/memory cap, no new privileges. Child registration declares needs (network, paths, sockets); runner grants exactly that.
- Health hints for children are generated by the runner (it knows how it launched them).
- CLI `agent-bus start|stop|ls|logs…`; config `/etc/agent-bus/`, `~/.config/agent-bus/`; unit `agent-busd.service`.

### In-process queue (replaces NATS inside a service)
- Bounded Go channel per named queue: `Push` non-blocking → `ErrFull` (backpressure); `Pop(ctx)` blocking; N goroutines = consumer group. **Non-durable by design.** If one queue ever needs durability, back *that one* with a WAL file — don't reintroduce a broker.

---

## 8. Ideas explored and superseded (keep for reference)

- **Redis Streams design** (before dropping brokers): per-session `session:{id}:inbox` streams with consumer groups, `XREADGROUP … BLOCK` + `XACK`, `XAUTOCLAIM` for stuck messages, dead-letter stream, `MAXLEN ~` trim, AOF `everysec`, registry hash with TTL refreshed by heartbeat, pub/sub only for `discovery:query/response` and `alive`. Writers: Telegram, Slack, post-commit hook, failed-tests hook, inter-agent (Claude/Codex) with `reply_to`. **Superseded** by point-to-point + in-process queues, but the writer list and the discovery/alive ideas carried over.
- **NATS** as bus (accounts, scoped signing keys as roles, KV for discovery, request-reply subjects `auth.svc.config`, `auth.user.key`, `auth.svc.resolve`, `auth.revoke`, `auth.config.changed.<svc>`). **Rejected**: once discovered, a broker is useless; want maximal simplicity.
- **Option B keys** (AUTH-signed JWT verified locally by services). **Rejected** in favor of Option A/derived; revisit only if AUTH-on-first-contact becomes a problem.
- **Claude Code Channels** (`--channels`, MCP-based, Telegram/Discord plugins, research preview 2026): the agent-bus session channel could be wrapped as a custom channel-capable MCP server so Claude Code sessions receive bus events natively. ~~Worth a spike later.~~ *Correction 2026-09-09: already in use in V1 — Claude sessions join the bus via `claude --channel`; see `docs/v1-original.md` §3.*
- **Zero-downtime reload** for Go daemons via `cloudflare/tableflip` (Linear **RLM-250**) — applies to `agent-busd` and AUTH replicas (socket inheritance, load-before-`Ready()`, 2× RAM during overlap; consider mmap/shared memory if datasets are large).
- **Cloudflare Tunnel + Access** and **mTLS** were evaluated for exposing a daemon to *customers*; **restricted SSH tunnels** (`restrict,permitopen`) for technical customers. Not part of agent-bus core, but the SSH lockdown pattern is reused for admin access.
- **Kvrocks / KQIR** (Redis-compatible on RocksDB, search via `FT.CREATE`/`FT.SEARCHSQL`) evaluated separately for Realmo data — not for agent-bus.

---

## 9. Suggested next steps in Claude Code

1. Read `docs/00…04`; fix any drift vs this handoff (this file is more complete).
2. Resolve open questions §1 with the owner **before** modeling — especially #1 (event delivery) and #2 (one daemon or two).
3. Then, only when asked: data model (SQLite-portable: TEXT/INTEGER, JSON as TEXT; one thin `Store` interface, sqlc + pgx-style portability), bundle JSON schema, replica HTTP endpoints, handshake message formats, `auth-admin` verb grammar.
4. Prototype order that matches "simple first": pairwise mode + session encryption + local mapping file (no AUTH at all) → `agent-busd` with one stdio child → AUTH with derived keys → signed generations + replication → discovery web/MCP faces → health/stats → chaining.
5. Keep `agent-bus` as the keyword everywhere; `ab_` only as MCP tool prefix.

## 10. Conventions for the docs
- Small files, main ideas only, tables over prose, no data models until requested.
- Owner writes terse shorthand and corrects directly; update without ceremony.
- Owner reads both English and Russian; docs are in English.
