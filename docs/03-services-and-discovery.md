# Services and Discovery

## Service kinds

Two axes: reachable or not, and who owns the record.

| Kind | Reachable | Registered by | Health | Signs events |
|---|---|---|---|---|
| **generic** | yes (host/port, unix socket, HTTP) | a user, on behalf of something existing | health-checker polls per hints | no (no key) |
| **agent** | yes | itself, own key | self-reports health/stats | yes, as service |
| **consumer** | no (pulls) | itself or user | none | no |
| **publisher** | not a service — an identity that signs events (`curl` + user key) | — | none | yes |

Publisher/consumer are **roles on a principal**, not service objects.

## Personal vs shared (default: shared)

- **personal** — runs *as* a specific user with their account/session
  (mail-reader, a Claude-session input channel). Owner = the user; default
  audience = the user; lifecycle follows the user; usually one instance per user.
  May use the user's key (ephemeral sessions) or its own key (recommended for
  anything long-running — narrower access, individually revocable).
- **shared** — runs as `svc:<name>` for many; explicit ACL; long-lived.

## Service vs instance

A service is the *kind* (code, description, roles, health hints, optional MCP
method info). An instance is service + **private config** + a place it runs.
Instances register, heartbeat, vanish; the service definition is signed and
rare-change.

## Service Discovery

- Optional: if you already know where something lives, talk to it directly.
- Registration carries **health hints**: HTTP endpoint + expected status, TCP
  connect, unix-socket ping, command, interval, timeout.
- **Audience** per service — users, services or org groups who may see and
  use it. Discovery is personalized; MCP tool lists come pre-filtered.
- Faces: **web** (humans, curl, dashboards) and **MCP server** (agents ask
  "what can I use, and how").

## Health-checker and Stats (modules of discovery)

- Health from probes (generic) or heartbeats (agents).
- Stats kept **in memory** (ring buffers), shown via discovery's API/dashboard,
  optionally exported (Prometheus `/metrics` first; Grafana reads that).
