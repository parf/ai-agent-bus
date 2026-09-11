# Processes and privileges

The runtime shape, on the **systemd model**: a supervisor that does almost
nothing, and several small children that each do one task with the narrowest
privilege that task needs.

Layers ([modules](10-modules.md)) are compile-time; this is runtime. A module
can move to a different process without changing which layer it belongs to.

## The rule

| | |
|---|---|
| **The supervisor does as little as possible** | it holds no secrets, no queues and no message bodies. It is the process that must not die, so it is the process with the least to go wrong |
| **One task per child** | a child exists because its task needs a privilege, or must be denied one |
| **Privileges are declared, not acquired** | each child has a fixed set, written down like a unit file — never obtained at runtime |
| **Nothing is shared implicitly** | children talk over unix sockets with explicit contracts. No shared memory, no shared store handle, no ambient state |
| **The supervisor owns the listeners** | it opens (and where needed chowns) the sockets and passes the fds down, systemd-style socket activation. That is also what makes zero-downtime reload work |
| **A child dying is normal** | contained, restarted with backoff, counted. Only the supervisor surviving matters |

## The processes

| Process | Task | Privilege it needs | Must never have |
|---|---|---|---|
| **supervisor** (`agent-busd`) | spawn, restart, stop, reload · read config · open and hand over listening sockets · report status | **`CAP_CHOWN`**, for the per-user sockets ([access § local socket](02-access.md#local-socket)) — and nothing else | `master_secret`, queues, bodies, the store |
| **bus** | registry, topics, queues, sessions, delivery — the core | the store; inherited listener fds | `CAP_CHOWN`, the ability to exec, `master_secret` |
| **runner** | spawns, sandboxes and supervises *user* children ([runner role](08-runner-role.md)) | fork/exec, and whatever a sandbox backend needs | the store, queues, bodies |
| **web** | the dashboard ([discovery § dashboard](05-discovery.md#dashboard)) | a read-only view of stats; **cgroup-limited** (CPU / memory / pids) so it can never starve the bus | any write path, bodies |
| **auth** | identities, keys, groups, roles ([AUTH role](06-auth-role.md)) | **alone holds `master_secret`**; its own unix socket | a network listener, the queues |
| **health** | probes generic services ([discovery § health checker](05-discovery.md#health-checker)) | outbound network | the store, queues |

Defaults: supervisor, bus and runner always; web on and may be turned off;
auth `auth: on`; health optional. Billing is deferred
([future/billing.md](future/billing.md)); when it ships it is another child.

## Why the runner is its own process

It is the one component that **executes code it did not write** — arbitrary
user children, under a sandbox it sets up itself. Least privilege says the
process that can `exec` should not also be the process holding everyone's
queues and sessions. So the runner is a child like any other, and the bus
keeps no ability to spawn.

Consequence for [runner role](08-runner-role.md): the runner supervises *user*
children; the supervisor supervises the runner. Two levels, same machinery.

## Why the supervisor holds `CAP_CHOWN`

Per-user sockets have to be chowned to their user, which needs the capability
([access § local socket](02-access.md#local-socket)). Giving it to the
supervisor — which does nothing else — means **no long-running child ever holds
it**: sockets are created and chowned at start and on reload, the fds are
passed down, and the bus that serves traffic has no capability at all.

⚠️ **Today `agent-busd` is one process and chowns its own sockets.** There is
no supervisor yet, so the separation above is a design and not a fact; where
the capability is missing the daemon says so and leaves the socket as its own.
Splitting the processes retires this.

## What is shared

Nothing but file descriptors and unix sockets, both explicit.

| Between | Carried how |
|---|---|
| supervisor → any child | inherited listener fds, config, a control socket |
| bus ↔ auth | unix socket, one question per session start ([identity § resolved at login](01-identity.md#resolved-at-login)) |
| bus ↔ runner | unix socket: register child, report health, start/stop |
| web → bus | read-only stats query |

No child reads another's memory, and no two processes hold the store open for
writing.

❓ **Which process owns the store handle** — the bus writes the registry, but
tokens and AUTH data have different lifetimes and readers. One writer with the
others asking it, or a store adapter per process? *Settled by:* owner.
