# Unassigned follow-up

## Candidates

| Candidate | What makes it actionable |
|---|---|
| OpenCode push adapter | A runtime transport spike and an owner-selected target release |
| Additional identity providers | A specific provider, enrolment use case and owner-selected scope |
| Second host sandbox backend | A host that cannot use the current backend and a verified confinement requirement |
| Optional snapshot formats and database adapters | Measured persistence need; see [storage](storage.md#storage) |
| Per-call authority cache | A measured need: 0.7's liveness and ownership checks cost lookup and list ~0.2 µs per record ([0.7 compared](../done/R0.8-MVP/0.7-baseline.md#07-compared-k17)); any cache must be invalidated by every commit and never let a revoked or deactivated principal act on a stale answer |
| Lock-free reads by copy-on-write | Measured lock contention between readers and commits: a write builds a new complete view and publishes it by one pointer swap, needing persistent maps to stay cheap at 100k records; not required by the publication rule (Q106) |
| The bus's dotenv grammar in the PHP and Java clients | A PHP or Java agent that needs multi-line or commented secrets; then port `envfile` and run its `common.json` in both suites ([issue K7](../issues/done/2026-10/2026-10-17.md), deferred 2026-10-17) |

| Forced launcher restarts, background servers included | A crash that `--restart` cannot survive because the Codex app-server or `opencode serve` died too; then a `--restart=force` that also rebuilds the owned server, with the t13 ownership check on each rebuild (owner, 2026-10-19: "if needed, later we add force restarts, with bg restarts"). Today `--restart` restarts only a TUI that crashed ([launchers](../../docs/06-launchers.md#smart-launchers)) |
| The container runner's token out of its environment | A container runtime where `podman inspect` or `docker inspect` readers are not trusted: `-e AGENT_BUS_TOKEN` shows the runner's token there; read it from a file or a `podman --secret` instead ([container image](../../src/container/README.md#run)) |
The stable proposals are indexed in [topics](README.md#topics). Assign a release only when the owner does; generic ideas remain here.

## Plugin namespace

If a Claude Code plugin named `ab` is shipped, its commands, skills and agents
use the `ab:` namespace. That namespace is not an MCP tool name. No plugin release
is committed by reserving the name.

## Auth dependencies

Moved from R1.1 by the owner on 2026-11-05; these are undecided future
work, not a delivery commitment. Resolve the linked choices before naming
implementation waves.

| Candidate work | Must precede it |
|---|---|
| ☐ Scoped credentials and encryption | [Settled lifetime policy](../../docs/02-access.md#token-lifetime); remaining key recovery and token grammar decisions |
| ☐ D.1–D.4 encryption carried from MVP | [Encryption acceptance](encryption-wave.md#d--the-bus-stops-reading-payloads); key lifecycle decisions |
| ☐ AUTH authorization freshness | [Q35](QUESTIONS.md#authorization-refresh), independently of credential lifetime |

Crypto acceptance must test what the daemon cannot decrypt.

## Bind a watched directory by file descriptor

The runner re-resolves an agent's `needs.dir` at every launch and then hands bwrap the path, so a swap between the check and the mount (by someone with write access beneath an owner root) is followed. Closing it means opening the directory once with `O_PATH` and no-follow, checking that descriptor, and giving bwrap `--ro-bind-fd` (bubblewrap 0.12), or checking the mount source after the bind. Deferred: it changes the sandbox job from paths to descriptors for all three launch paths. Until then the node owner keeps roots free of untrusted writers ([security](../../docs/runner/security.md#isolation-today)).
