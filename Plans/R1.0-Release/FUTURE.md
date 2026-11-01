# R1 future

📌 **TL;DR:** Follow-up the owner has decided is not part of R1. Each entry names the decision that deferred it and what would start it.

## Race-free open of an MCP bigfile path

- **What:** `fencedInput` in `src/mcp/bigfile-fence.ts` checks the real path and then opens it, a window in which a same-account local process could swap a link. Opening with `O_NOFOLLOW` on the real path would close it.
- **Deferred:** outside the threat model the fence answers (a path chosen by a model or an injected message); a process of the same account can already read the file itself (J1 review, 2026-11-01, QA1).
- **To start it:** the owner asks for it, or the fence is widened to a shared directory other accounts write to.

## Key proof bound to the node it was made for

- **What:** the signed message names the name and the daemon's address (for example `agent-bus enrol\n<name>\n<nonce>`), so a node in the middle cannot relay a real daemon's challenge to a user who does not pin it. The daemon would accept both forms for a release.
- **Deferred:** it changes the frozen wire ([Q296](DECISIONS.md#recorded-decisions)); the client now signs only the `agent-bus` namespace and a 32-hex nonce (J1 sweep B1, 2026-11-01). TLS pinning and the local socket cover the relay today.
- **To start it:** the owner asks for it before 1.0, or a node-in-the-middle case is reported.

## Deb and rpm packages

- **What:** `.deb` and `.rpm` packages and the repositories that serve them, so a host installs with its own package manager.
- **Deferred:** not in R1 ([RD171](DECISIONS.md#recorded-decisions), owner, 2026-10-30). The install path is [npm](distribution.md#simple-install).
- **To start it:** the owner asks for it, and names where the package repositories are hosted; nothing is built for it before that.

## Logwatch subscription start boundary

- Deferred from B.29 by lead1 (2026-10-30): enqueue a line, add a subscription, then drain the forward queue; the pre-add line is forwarded. Pin the subscription-start boundary before changing this existing queue behavior.

## Concurrent chat subscription migration

- B.29 runs with one process per Slack/Telegram instance. Two processes migrating the same legacy key can race the journal's add: the second gets 412 and stops serving until reload. If multiple processes are enabled, re-read and finish the winning journal before serving; test simultaneous startup. The existing journal keeps old/new rows from both being served after recovery.

## macOS uninstall command

Deferred by the owner on 2026-10-30; use the [manual macOS uninstall steps](../../docs/07-setup.md#uninstall-the-macos-test-daemon).

## Shell installer

- **Deferred P.4:** the owner chose simple npm install commands on 2026-10-30 ([RD178](DECISIONS.md#recorded-decisions)). `curl … | sh`, `install.sh` distribution and its installer-specific acceptance are outside R1.
- **To start it:** the owner asks for a shell installer. Reuse HTTPS downloads, `SHA256SUMS`, manifests and untouched-host refusal checks; release signing, keys and attestations are not requirements.
- **History:** [earlier installer acceptance](done/installer-acceptance.md#checks). Existing scripts and throwaway-key fixtures remain unchanged until the P.1 migration; they do not require a real owner key or a public signed release.

## Setup waits out the daemon's start limit

- Setup restarts `agent-busd` once per run that changes it, and systemd refuses a start after `StartLimitBurst` starts inside `StartLimitIntervalUSec`: a script that runs setup many times within the window fails on the later runs, loudly and with the wait named ([J59](../issues/2026-10/2026-10-30.md)). **To start it:** setup reads the unit's two limits, counts its recent starts and waits for the window before a restart it must make, as the fresh-install fixture does. Not raising `StartLimit*` and not clearing the failed state are settled.

## M.8 deployment follow-ups

- Monorepo subdirectory packages: deferred by #ab-dir on 2026-10-30; M.8 requires a self-contained package with root `agent.json`.
- A runner-owned deployment root: deferred by #ab-dir on 2026-10-30; it changes the [process boundary](../../docs/08-processes.md#the-rule) and requires the owner's decision. M.8 uses a host-provisioned writable `--agents` directory.

## Asks-only read in the Python client

- **What:** `asks: "only"` on the Python client's consume, as the TypeScript client has it ([RT25](TODO.md#bundled-agents), `#ab-dir`, 2026-10-30).
- **To start it:** a Python agent with a slow start that should answer `status: starting: <what>`; nothing is built before that.

## IMAP folders follow-up

- A hard expunge (permanent delete) for [B.30](TODO.md#bundled-agents), only if the owner asks; off by default, never a bare EXPUNGE.

## M.11 acceptance gaps

- **What:** two checks the M.11 real-host run did not make: a real agent served by an installed system runner, and the same verbs on a Debian host (the run used an Arch container on this x86_64 node); also `enable` before install, whose refusal is in the unit tests only. See [the result](done/m11.md#findings).
- **To start it:** the next system-runner or distribution change that needs the evidence, or the owner asks; M.11 stays ✅ meanwhile ([result](done/m11.md#what-each-verb-left)).

## CLI hint for a plain-text help call

- **What:** when `agent-bus call <name> help` is refused, the CLI suggests `agent-bus help <name>`: the word `help` in a body is data, and the ask is how help is requested.
- **Deferred:** found on `#rag` (2026-11-01, `#ab-dir`); the refusal is the same as every other agent's, and the guide states the difference ([RAG guide](../../docs/agents/rag.md#installing-it)).
- **To start it:** a user is caught by it again, or the owner asks; nothing is built before that.

## Bigfile reopen without the registry lock

- **For R1.1 — A1:** reopening a failed bigfile holder and pruning orphan files currently perform filesystem/index I/O under the global registry lock. Deferred by `#ab-dir`, 2026-11-01; A2's credentials-only preflight is the R1 change.
- **Approach:** claim one attempt, retry timestamp and generation under `b.mu`; reopen outside it into an unpublished holder. Classify orphan IDs briefly under the lock and remove their files outside it: record IDs are never reused, so an ID already absent cannot become another record's live ID. Recheck the remaining IDs before publication.
- **Publication:** install only if the claimed generation still matches; re-read the requested record's current ID and ACL after reopening. A replaced holder or unavailable state invalidates an old attempt. Move the explicit prune's scan/drop I/O out of `b.mu` too.
- **Lifecycle and proof:** start a reopened holder's sweeper only when accepted, or stop it when rejected; never leave an abandoned timer. Channel-gated reopen/prune tests must let an ordinary KV write complete before releasing the blocked I/O, and pin superseded attempts, record removal/transfer and current ACL checks.
- **To start it:** the R1.1 storage work, with the lock and sweeper design reviewed before implementation.

## Database agent transport TLS

- **For R1.1 — L1:** MySQL and PostgreSQL connections do not require TLS, and ClickHouse can send its key over plain HTTP to a non-loopback host. Add explicit TLS configuration and a clear refusal or warning for an unprotected remote connection.
- **Deferred:** the db agent is disabled on this node and no owner use needs it in R1 (`#ab-dir`, 2026-11-01).
- **To start it:** the R1.1 agent security work, or an owner use that needs the database agent sooner.
