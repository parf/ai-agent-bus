# Documentation cleanup review

Status: applying the owner's rules across all offered documentation. Push and publication remain on hold.

Astra read every block in all 68 docs/ Markdown pages plus root README and AI-SETUP. This records analysis coverage, not completion of the final rewrite. Each file still needs its fixes, semantic dedup pass and final link check. Source guides require their own audience review.

| File | Audience | Analysis | Cleanup / dedup |
|---|---|---|---|
| [AI-SETUP.md](../AI-SETUP.md) | Users | All blocks read by Astra | Pending final pass |
| [README.md](../README.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/00-overview.md](../docs/00-overview.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/01-identity-and-authority.md](../docs/01-identity-and-authority.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/02-access-remote.md](../docs/02-access-remote.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/02-access.md](../docs/02-access.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/03-records-agent.md](../docs/03-records-agent.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/03-records-channel.md](../docs/03-records-channel.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/03-records-kv.md](../docs/03-records-kv.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/03-records-resource.md](../docs/03-records-resource.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/03-records-service.md](../docs/03-records-service.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/03-records.md](../docs/03-records.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/04-messaging.md](../docs/04-messaging.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/05-discovery.md](../docs/05-discovery.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/06-dedicated-runner.md](../docs/06-dedicated-runner.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/06-launchers.md](../docs/06-launchers.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/07-setup.md](../docs/07-setup.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/08-processes.md](../docs/08-processes.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/09-daemon-api.md](../docs/09-daemon-api.md) | Users | All blocks read by Astra | Fresh-install start and complete output fixed; full cleanup/dedup pending |
| [docs/10-writing-an-agent.md](../docs/10-writing-an-agent.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/administration.md](../docs/administration.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/README.md](../docs/agents/README.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/ai.md](../docs/agents/ai.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/at.md](../docs/agents/at.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/authoring.md](../docs/agents/authoring.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/bus.md](../docs/agents/bus.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/create-agent-prompt.md](../docs/agents/create-agent-prompt.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/cron.md](../docs/agents/cron.md) | Users | All blocks read by Astra | Pending final pass |
| [src/agents/db/guide.md](../src/agents/db/guide.md) | Source-package users | All blocks read by Astra | Pending final pass |
| [docs/agents/debug.md](../docs/agents/debug.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/discord.md](R1.1/discord-user-guide.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/eventlog.md](../docs/agents/eventlog.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/fetch.md](../docs/agents/fetch.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/git-autopull.md](../docs/agents/git-autopull.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/git.md](../docs/agents/git.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/imap.md](../docs/agents/imap.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/logwatch.md](../docs/agents/logwatch.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/qdrant.md](../docs/agents/qdrant.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/rag.md](../docs/agents/rag.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/server-info.md](../docs/agents/server-info.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/slack.md](../docs/agents/slack.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/sms.md](../docs/agents/sms.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/smtp.md](../docs/agents/smtp.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/subscriptions.md](../docs/agents/subscriptions.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/systemd.md](../docs/agents/systemd.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/telegram.md](../docs/agents/telegram.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/tmux-sessions.md](../docs/agents/tmux-sessions.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/voyage.md](../docs/agents/voyage.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/agents/webhook.md](../docs/agents/webhook.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/constitution.md](../docs/constitution.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/decisions.md](../docs/decisions.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/dedicated-runner/agent-json.md](../docs/dedicated-runner/agent-json.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/glossary.md](../docs/glossary.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/platforms/macos.md](../docs/platforms/macos.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/runner/README.md](../docs/runner/README.md) | Users | All blocks read by Astra | Portable examples and bundle/shared distinction fixed; final dedup pending |
| [docs/runner/agents.md](../docs/runner/agents.md) | Users | All blocks read by Astra | Portable examples and bundle/shared distinction fixed; final dedup pending |
| [docs/runner/configuration.md](../docs/runner/configuration.md) | Users | All blocks read by Astra | Portable examples and bundle/shared distinction fixed; final dedup pending |
| [docs/runner/getting-started.md](../docs/runner/getting-started.md) | Users | All blocks read by Astra | Portable examples and bundle/shared distinction fixed; final dedup pending |
| [docs/runner/operations.md](../docs/runner/operations.md) | Users | All blocks read by Astra | Portable examples and bundle/shared distinction fixed; final dedup pending |
| [docs/runner/security.md](../docs/runner/security.md) | Users | All blocks read by Astra | Portable examples and bundle/shared distinction fixed; final dedup pending |
| [docs/runner/setup.md](../docs/runner/setup.md) | Users | All blocks read by Astra | Portable examples and bundle/shared distinction fixed; final dedup pending |
| [docs/user/README.md](../docs/user/README.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/user/ai-cli.md](../docs/user/ai-cli.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/user/backup.md](../docs/user/backup.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/user/cli.md](../docs/user/cli.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/user/daemon.md](../docs/user/daemon.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/user/dedicated-runner.md](../docs/user/dedicated-runner.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/user/write-agent-cli.md](../docs/user/write-agent-cli.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/user/write-agent-process.md](../docs/user/write-agent-process.md) | Users | All blocks read by Astra | Pending final pass |
| [docs/user/write-agent-runner.md](../docs/user/write-agent-runner.md) | Users | All blocks read by Astra | Pending final pass |
| [src/INSTALL.md](../src/INSTALL.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/MODULES.md](../src/MODULES.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/README.md](../src/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/ai/README.md](../src/agents/ai/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/db/README.md](../src/agents/db/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/debug/README.md](../src/agents/debug/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/discord/README.md](../src/agents/discord/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/fetch/README.md](../src/agents/fetch/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/rag/README.md](../src/agents/rag/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/slack/README.md](../src/agents/slack/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/sms-telnyx/README.md](../src/agents/sms-telnyx/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/standalone/README.md](../src/agents/standalone/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/standalone/git/README.md](../src/agents/standalone/git/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/standalone/logwatch/README.md](../src/agents/standalone/logwatch/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/standalone/server-info/README.md](../src/agents/standalone/server-info/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/standalone/systemd/README.md](../src/agents/standalone/systemd/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/telegram/README.md](../src/agents/telegram/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/voyage/README.md](../src/agents/voyage/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/agents/webhook/README.md](../src/agents/webhook/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/README.md](../src/clients/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/examples/README.md](../src/clients/examples/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/examples/codestats/README.md](../src/clients/examples/codestats/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/examples/go/README.md](../src/clients/examples/go/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/examples/python/README.md](../src/clients/examples/python/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/examples/rust/README.md](../src/clients/examples/rust/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/examples/ts/README.md](../src/clients/examples/ts/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/go/README.md](../src/clients/go/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/java/README.md](../src/clients/java/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/php/README.md](../src/clients/php/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/python/README.md](../src/clients/python/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/rust/README.md](../src/clients/rust/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/tests/kv/README.md](../src/clients/tests/kv/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/clients/typescript/README.md](../src/clients/typescript/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/container/README.md](../src/container/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/internal/envfile/testdata/README.md](../src/internal/envfile/testdata/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/mcp/README.md](../src/mcp/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/npm/README.md](../src/npm/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |
| [src/testdata/client-conformance/README.md](../src/testdata/client-conformance/README.md) | User/developer scope to classify | All blocks read by Astra | Pending final pass |

## Correctness findings to resolve

- Authority: correct @owner cohort and runtime Maintainers terms; realm is the bus namespace, not necessarily host; Configuration reads admit managers; remove obsolete kind counts.
- Messaging: distinguish queued/consumed loss, send attempts and receipt; no done after an answer; reply reservations remain subject to TTL/overflow and are daemon state.
- KV: gather examples have separate-write crash gaps; do not promise exactly-once counters or unconditional safe recovery.
- Agents: DB and Discord are not in the shipped bundle; remove them from the offered release catalogue without claiming they work. Fix the generator, not just the generated block.
- RAG: external keyword engine is not wired; no blanket retry safety or free retries. SMS access depends on ACL. Logwatch no longer folds repeated forwarded lines.
- Runner: shared packages can auto-start but bundled packages need enabled:true; shared directories shadow bundles, including invalid ones. Do not imply trusted agent code can never reveal its received secrets.
- MCP: retain the actual supported protocol revision, not an unsupported latest-only promise.
- Examples: remove development-only names, tags, timestamps, build evidence and jq decoration; preserve real outputs only where useful and make prerequisites explicit.
- Authoring tutorials: source-linked examples have a simplistic Secret parser, unrenewed short lease, non-atomic counters and unbounded waits. Do not silently alter quoted source without coordinating the example.
- Backups: exports deliberately omit some state that survives restart; developer-node retention is not an installed default; create sensitive archive files privately from the outset.
- Discovery: distinguish absent web UI controls from actual Group states; retain public identity disclosure and real account/session/security boundaries.

## Boundaries

- Canonical-only implementation is part of R1: no compatibility for prerelease installations or data is required. Retired formats are refused and no migration or fallback code is shipped; scripts, help and documentation follow the same forms.
- Runtime builds, gates, provider calls and host changes are not needed for prose. Documentation tooling changes receive focused verification.
- Plans retain genuine future scope and dated evidence; user pages describe current behavior without internal history.

Agent guide example pass: removed 17 duplicated private development-node sessions and their dates/build versions; retained the portable use commands and the meaningful cron outcome-window explanation. Instance-dependent commands explicitly require an enabled instance. No provider requests were made.

Catalogue scope: derive generated descriptions from the release packager's actual runtime/standalone inventory, not every source directory. DB source guide and deferred Discord guide leave docs/; source manifest and golden docs URLs follow their real homes. The static regression fails on the old generator because it includes db.

Source-guide analysis is complete: Astra fully read all 38 original source README/INSTALL/MODULES candidates. Remaining corrections include installed-vs-source runtime paths (MCP/container/source index), PHP executable snippets and prerequisites, SDK setup/identity prerequisites, misleading fixtures' no-side-effects claims, source-only history/filler and client Plan-as-contract links. Final cleanup and per-page dedup remain pending until applied.

Root README pass: recommended AI installation leads; remove acceptance claims, roadmap/storage promises, duplicate KV reference/pattern tables and repeated installation navigation. Preserve all three images and all diagrams. Introductory calls use the guaranteed built-in #bus; hello source example explicitly declines unsupported asks. Remaining cross-page semantic review continues.
