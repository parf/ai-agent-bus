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

The set that survives `exec` is the **ambient** one, so the supervisor clears
it on the thread it forks from: its own permitted set is untouched — it can
still chown a socket on reload — and a child starts with no capability at all.
Where the capability is missing altogether the supervisor says so and leaves
the socket as its own.

⚠️ **The split is real for supervisor, bus and web; runner, auth and health
are still design.** That a child's effective set is empty can be read from
`/proc` on a host where the daemon actually holds `CAP_CHOWN` — under the unit
([setup § the service account](09-setup.md#the-service-account)) — and not on
one where nothing had it to begin with.

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

**The bus holds the store**, and it is the only process that does. The
supervisor keeps no handle to anything durable — that is what makes it the
process with the least to go wrong — and a child that wants something durable
asks the process that owns it.

## How a child is started

One binary. The supervisor opens every listener before any child exists, so a
child cannot make one and does not need the capability to.

| | |
|---|---|
| `AGENT_BUS_ROLE=bus` | which role this process is. Absent means supervisor |
| `AGENT_BUS_FDS` | what arrives at fd 3 upwards, in order: `tcp`, `shared`, `user:<principal>` — the whole contract between the two |
| `-web` | the dashboard runs as a child too ([discovery § where it listens](05-discovery.md#where-it-listens)), reaching the bus over its owner's socket, so it holds no token |

A child that dies is restarted with backoff, and the listeners are passed to
the replacement — the socket a client holds is the same file across a restart.
A supervisor that is killed outright takes its children with it
(`PR_SET_PDEATHSIG`), so nothing orphaned keeps a port.
