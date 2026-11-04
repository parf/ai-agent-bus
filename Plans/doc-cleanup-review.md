# Documentation cleanup review

Status: the recorded user-guide and source-guide cleanup/dedup passes are applied. Final integrated verification passed on 2026-11-04. Push, publication and tagging remain on hold.

This ledger reconciles the earlier full-block Astra reviews with the completed family commits through `0dcc17c6`, plus the closure pass below. A commit alone is not treated as proof of a full semantic review: the evidence combines the recorded reviewer coverage, family handoffs and the actual page changes. Contributor references added late have narrower dispositions where a full dedup pass is not evidenced.

## Review evidence

| Family | Evidence | Coverage |
|---|---|---|
| Entry | `b20a2c9b`, `e7b00d24`, `e647b56b` | Root entry points, short setup, operator and runner references; final runner/macOS reread below |
| Authority | `36ec2d88` | Nine complete identity/access/record/constitution pages |
| Runtime | `1de9f17a`, `807b210a` | Nine complete messaging/KV/discovery/runtime/API/reference pages |
| Agents | `2dc1bad5` | All 26 agent guides read; 25 changed, authoring guidance retained; canonical follow-ups preserve shipped behavior |
| Users | `0dcb9c06` | All nine user guides, including three source-quoted tutorials |
| Source | `cadc83f4`, `377bec36` | All 38 original README/INSTALL/MODULES candidates reviewed; 34 changed in the main source pass |
| Checklist | `4ee18444`, `b74e96a3` | Canonical agent-quality checklist and authoring prompt alignment |
| Closure | This pass | Runner setup, macOS, missing-reference classification, three remaining web specifications and two small contributor wording fixes |

## File dispositions

“Reviewed” below records a completed block and semantic dedup pass for the named family; it is not a final link/gate claim. A retained short reference is a deliberate keep disposition, not an unperformed rewrite. Detailed developer specifications retain implementation tables for their intended audience.

| File | Audience | Review | Cleanup / dedup disposition |
|---|---|---|---|
| [AI-SETUP.md](../AI-SETUP.md) | Users | Reviewed — Entry | Removed repeated scope questions; reuse the requested install/update/tuning goal. |
| [README.md](../README.md) | Users | Reviewed — Entry | One AI-assisted start; merged installation and KV summaries; retained three images and diagrams. |
| [docs/00-overview.md](../docs/00-overview.md) | Users | Reviewed — Entry | Removed release/provenance and editorial navigation; one architecture explanation and Guides index. |
| [docs/01-identity-and-authority.md](../docs/01-identity-and-authority.md) | Users | Reviewed — Authority | Owners and groups first; normalized authority terms; KV penultimate and locks last; merged repeated introductions. |
| [docs/02-access-remote.md](../docs/02-access-remote.md) | Users | Reviewed — Authority | Merged credential handoff prose; retained SSH/TLS, local-socket and remote-access caveats. |
| [docs/02-access.md](../docs/02-access.md) | Users | Reviewed — Authority | Consolidated authentication and ACL explanations; corrected owner cohort and manager access; retained trust boundary. |
| [docs/03-records-agent.md](../docs/03-records-agent.md) | Users | Reviewed — Authority | Consolidated roles, Configuration and Secret access; removed editorial wrappers and old permission claims. |
| [docs/03-records-channel.md](../docs/03-records-channel.md) | Users | Reviewed — Authority | Separated queue retention from PubSub fan-out; merged repeated channel behavior. |
| [docs/03-records-kv.md](../docs/03-records-kv.md) | Users | Reviewed — Runtime | Merged KV examples and reference; removed exactly-once claims from separate-write gather patterns. |
| [docs/03-records-resource.md](../docs/03-records-resource.md) | Users | Reviewed — Authority | Kept URI/template/MCP reference; removed repeated framing and private-node examples. |
| [docs/03-records-service.md](../docs/03-records-service.md) | Users | Reviewed — Authority | Clarified service versus speaking principal; retained Secret/env syntax and merged repeated access guidance. |
| [docs/03-records.md](../docs/03-records.md) | Users | Reviewed — Authority | Removed stage/future and duplicate kind prose; current actors, tags and record constraints retained. |
| [docs/04-messaging.md](../docs/04-messaging.md) | Users | Reviewed — Runtime | Separated acceptance, dequeue, answer and receipts; consolidated durability, TTL and unknown-outcome caveats. |
| [docs/05-discovery.md](../docs/05-discovery.md) | Users | Reviewed — Runtime | Consolidated discovery/dashboard guidance; corrected Group UI distinctions; preserved public identity disclosure. |
| [docs/06-dedicated-runner.md](../docs/06-dedicated-runner.md) | Users | Reviewed — Runtime | Removed repeated runner framing and stage claims; retained lifecycle, input modes and actual limits. |
| [docs/06-launchers.md](../docs/06-launchers.md) | Users | Reviewed — Runtime | Removed build/status/history tables; merged repeated startup/recovery facts; retained takeover and intentional tag clearing. |
| [docs/07-setup.md](../docs/07-setup.md) | Users | Reviewed — Entry | Short AI-first installation/update path; operator, platform and runner details moved to their guides. |
| [docs/08-processes.md](../docs/08-processes.md) | Users | Reviewed — Runtime | Condensed repeated process descriptions; retained privilege, isolation and restart boundaries. |
| [docs/09-daemon-api.md](../docs/09-daemon-api.md) | Users | Reviewed — Runtime | Removed release policy/history and duplicate prose; retained route reference; aligned strict requests and protocol contract. |
| [docs/10-writing-an-agent.md](../docs/10-writing-an-agent.md) | Agent authors | Reviewed — Checklist | Canonical conditional agent-quality checklist retained; shared requirements replace tutorial-specific guarantees. |
| [docs/administration.md](../docs/administration.md) | Users | Reviewed — Entry | Consolidated installed-node operation, TLS and logs; removed developer-node defaults. |
| [docs/agents/README.md](../docs/agents/README.md) | Users | Reviewed — Agents | Merged agent selection/grants guidance; generated catalogue follows shipped inventory; preserved generated block. |
| [docs/agents/ai.md](../docs/agents/ai.md) | Users | Reviewed — Agents | Removed repeated capability and instance setup prose; retained actual settings and operational limits. |
| [docs/agents/at.md](../docs/agents/at.md) | Users | Reviewed — Agents | Merged scheduling and delivery explanations; kept one-attempt and unknown-outcome conditions. |
| [docs/agents/authoring.md](../docs/agents/authoring.md) | Agent authors | Reviewed — Checklist | Metadata authoring guidance retained; links the canonical behavior checklist instead of a second one. |
| [docs/agents/bus.md](../docs/agents/bus.md) | Users | Reviewed — Agents | Removed repeated built-in scope claims; kept callable operations and event/subscription examples. |
| [docs/agents/create-agent-prompt.md](../docs/agents/create-agent-prompt.md) | Agent authors | Reviewed — Agents | Condensed repeated generation instructions; prompt uses canonical agent checklist. |
| [docs/agents/cron.md](../docs/agents/cron.md) | Users | Reviewed — Agents | Removed repeated transcripts; kept schedule semantics and bounded delivery outcome window. |
| [src/agents/db/guide.md](../src/agents/db/guide.md) | Source-package users | Reviewed — Source | Moved from user catalogue; trimmed repeated setup/history; source installation explicitly scoped. |
| [docs/agents/debug.md](../docs/agents/debug.md) | Users | Reviewed — Agents | Removed duplicated capability lists; installed checks and diagnostic limitations retained. |
| [docs/agents/eventlog.md](../docs/agents/eventlog.md) | Users | Reviewed — Agents | Merged event/forwarding descriptions; retained stored-event and one-attempt delivery distinctions. |
| [docs/agents/fetch.md](../docs/agents/fetch.md) | Users | Reviewed — Agents | Removed repeated examples/provenance; preserved target restrictions and request bounds. |
| [docs/agents/git-autopull.md](../docs/agents/git-autopull.md) | Users | Reviewed — Agents | Consolidated automatic-pull instructions; removed duplicated terminal sessions. |
| [docs/agents/git.md](../docs/agents/git.md) | Users | Reviewed — Agents | Kept canonical repository configuration; removed duplicated commands and developer transcripts. |
| [docs/agents/imap.md](../docs/agents/imap.md) | Users | Reviewed — Agents | Merged long duplicated search/cache descriptions; distinguished free-text and server search; kept settings and authentication limits. |
| [docs/agents/logwatch.md](../docs/agents/logwatch.md) | Users | Reviewed — Agents | Removed stale forwarding-fold claims and repeated sessions; current filters and delivery limits retained. |
| [docs/agents/qdrant.md](../docs/agents/qdrant.md) | Users | Reviewed — Agents | Removed repeated setup prose; retained collection operations and external service prerequisites. |
| [docs/agents/rag.md](../docs/agents/rag.md) | Users | Reviewed — Agents | Removed unsupported external keyword-engine/retry claims; retained revision and partial-failure limits. |
| [docs/agents/server-info.md](../docs/agents/server-info.md) | Users | Reviewed — Agents | Removed repeated inventory and transcripts; kept root versus nonroot method availability. |
| [docs/agents/slack.md](../docs/agents/slack.md) | Users | Reviewed — Agents | Consolidated setup/configuration and removed duplicate transcripts; credentials and subscription limits retained. |
| [docs/agents/sms.md](../docs/agents/sms.md) | Users | Reviewed — Agents | Simplified settings and clarified ACL-dependent access; removed duplicated setup/output. |
| [docs/agents/smtp.md](../docs/agents/smtp.md) | Users | Reviewed — Agents | Merged repeated send/settings guidance; retained one-attempt delivery and provider requirements. |
| [docs/agents/subscriptions.md](../docs/agents/subscriptions.md) | Users | Reviewed — Agents | One shared subscription contract; retained admission, visibility, filtering and uncertain-delivery rules. |
| [docs/agents/systemd.md](../docs/agents/systemd.md) | Users | Reviewed — Agents | Single setting stated directly; retained allowlist shape, instance and privilege limits. |
| [docs/agents/telegram.md](../docs/agents/telegram.md) | Users | Reviewed — Agents | Removed dated/private transcripts and editorial filler; merged overlapping setup/settings instructions. |
| [docs/agents/tmux-sessions.md](../docs/agents/tmux-sessions.md) | Users | Reviewed — Agents | Removed repeated setup claims; retained user-runner context and explicit state location. |
| [docs/agents/voyage.md](../docs/agents/voyage.md) | Users | Reviewed — Agents | One model-default sentence; preserved provider-origin versus sandbox distinction; removed repeated configuration framing. |
| [docs/agents/webhook.md](../docs/agents/webhook.md) | Users | Reviewed — Agents | Merged route/setup explanations; retained authentication, fixed-target and delivery constraints. |
| [docs/constitution.md](../docs/constitution.md) | Users | Reviewed — Authority | Reordered and condensed current obligations; removed stage/evidence narrative while retaining limits and error contracts. |
| [docs/decisions.md](../docs/decisions.md) | Users | Reviewed — Runtime | Current named references retained; dated decision evidence moved to the archive. |
| [docs/dedicated-runner/agent-json.md](../docs/dedicated-runner/agent-json.md) | Users | Reviewed — Entry | Kept compact JSON/flag reference; removed obsolete name inference and repeated lifecycle fields. |
| [docs/glossary.md](../docs/glossary.md) | Users | Reviewed — Runtime | Short definitions link owning contracts instead of duplicating their rules. |
| [docs/platforms/macos.md](../docs/platforms/macos.md) | Users | Reviewed — Closure | Full reread; concise client/daemon split and platform limits retained; missing or mismatched role group refuses setup. |
| [docs/runner/README.md](../docs/runner/README.md) | Users | Reviewed — Entry | Removed duplicated capability/setup lists; links the install and configuration guides. |
| [docs/runner/agents.md](../docs/runner/agents.md) | Users | Reviewed — Entry | Kept inventory/name examples; removed private-node assumptions and duplicate starter prose. |
| [docs/runner/configuration.md](../docs/runner/configuration.md) | Users | Reviewed — Entry | Consolidated package/host/record settings; explicit bundled enablement; obsolete inert keys removed. |
| [docs/runner/getting-started.md](../docs/runner/getting-started.md) | Users | Reviewed — Entry | Portable first use with explicit bundle enablement; retained runnable steps instead of private-node assumptions. |
| [docs/runner/operations.md](../docs/runner/operations.md) | Users | Reviewed — Entry | Merged control/file-call guidance; removed inert history fields and blanket retry assurances. |
| [docs/runner/security.md](../docs/runner/security.md) | Users | Reviewed — Entry | Kept precise sandbox/network/secret limits; removed unsupported guarantees and inert-option commentary. |
| [docs/runner/setup.md](../docs/runner/setup.md) | Users | Reviewed — Closure | Full reread; executable install examples; merged lifecycle instructions and tmux persistence; condensed filesystem mechanics while preserving ownership/secret warnings. |
| [docs/user/README.md](../docs/user/README.md) | Users | Reviewed — Users | Removed private status sample and repeated navigation prose; retained short task index. |
| [docs/user/ai-cli.md](../docs/user/ai-cli.md) | Users | Reviewed — Users | Condensed AI review prompt; ack only for deferred work, findings answer even when empty, no done after answer. |
| [docs/user/backup.md](../docs/user/backup.md) | Users | Reviewed — Users | Separated export exclusions from restart persistence; private archive creation; removed developer cleanup defaults. |
| [docs/user/cli.md](../docs/user/cli.md) | Users | Reviewed — Users | Portable complete JSON; fixed PubSub recipient example and READERS meaning; merged repeated options/examples. |
| [docs/user/daemon.md](../docs/user/daemon.md) | Users | Reviewed — Users | Removed private status/build evidence; consolidated installed checks and operational counters. |
| [docs/user/dedicated-runner.md](../docs/user/dedicated-runner.md) | Users | Reviewed — Users | Removed blind retries and repeated setup; clarified visibility, unknown effects and lifecycle. |
| [docs/user/write-agent-cli.md](../docs/user/write-agent-cli.md) | Users | Reviewed — Users | Source quote retained exactly; clarified simple Secret parser and first-use prerequisites. |
| [docs/user/write-agent-process.md](../docs/user/write-agent-process.md) | Users | Reviewed — Users | Source quote retained exactly; corrected lease/counter guarantees and unbounded-wait guidance. |
| [docs/user/write-agent-runner.md](../docs/user/write-agent-runner.md) | Users | Reviewed — Users | Source quote retained exactly; merged authoring setup and retained actual timeout/Configuration limits. |
| [src/INSTALL.md](../src/INSTALL.md) | Installers/contributors | Reviewed — Source | Separated release-user installation from source packaging; removed publication/evidence noise. |
| [src/MODULES.md](../src/MODULES.md) | Contributors | Reviewed — Source | Current module/dependency boundary reference; removed historical/plan framing and duplicate architecture prose. |
| [src/README.md](../src/README.md) | Contributors | Reviewed — Source | Condensed source index; release users directed to installed packages. |
| [src/agents/ai/README.md](../src/agents/ai/README.md) | Package contributors | Reviewed — Source | Source package mechanics retained; repeated user setup linked to guide. |
| [src/agents/db/README.md](../src/agents/db/README.md) | Source-package contributors | Reviewed — Source | Keep short implementation/test index; source-only package is outside shipped catalogue. |
| [src/agents/debug/README.md](../src/agents/debug/README.md) | Package contributors | Reviewed — Source | Keep short implementation/verification reference; user procedures linked rather than copied. |
| [src/agents/discord/README.md](../src/agents/discord/README.md) | Deferred-package contributors | Reviewed — Source | Scope points to deferred plan; no shipped availability claim. |
| [src/agents/fetch/README.md](../src/agents/fetch/README.md) | Package contributors | Reviewed — Source | Kept concise implementation entry and link to canonical user guide. |
| [src/agents/rag/README.md](../src/agents/rag/README.md) | Package contributors | Reviewed — Source | Corrected supported keyword path; removed duplicate capability claims. |
| [src/agents/slack/README.md](../src/agents/slack/README.md) | Package contributors | Reviewed — Source | Removed repeated deployment/configuration instructions; retained package-specific checks. |
| [src/agents/sms-telnyx/README.md](../src/agents/sms-telnyx/README.md) | Package contributors | Reviewed — Source | Clarified provider/package prerequisites; user setup linked to guide. |
| [src/agents/standalone/README.md](../src/agents/standalone/README.md) | Package contributors | Reviewed — Source | Source starter and privilege choices retained; release-user route separated. |
| [src/agents/standalone/git/README.md](../src/agents/standalone/git/README.md) | Package contributors | Reviewed — Source | Merged duplicated user procedure into guide link; kept packaging details. |
| [src/agents/standalone/logwatch/README.md](../src/agents/standalone/logwatch/README.md) | Package contributors | Reviewed — Source | Corrected active behavior and installation paths; kept implementation-specific limits. |
| [src/agents/standalone/server-info/README.md](../src/agents/standalone/server-info/README.md) | Package contributors | Reviewed — Source | Removed duplicate method/install prose; retained implementation and test entry. |
| [src/agents/standalone/systemd/README.md](../src/agents/standalone/systemd/README.md) | Package contributors | Reviewed — Source | Removed duplicate user setup and source provenance; retained packaging and privilege details. |
| [src/agents/telegram/README.md](../src/agents/telegram/README.md) | Package contributors | Reviewed — Source | User operations linked instead of repeated; implementation details retained. |
| [src/agents/voyage/README.md](../src/agents/voyage/README.md) | Package contributors | Reviewed — Source | Keep compact package and fake-provider test description; no user instruction duplication. |
| [src/agents/webhook/README.md](../src/agents/webhook/README.md) | Package contributors | Reviewed — Source | Keep compact bridge description and link to canonical route/security guide. |
| [src/clients/README.md](../src/clients/README.md) | SDK users/contributors | Reviewed — Source | Consolidated SDK/conformance entry; current contracts replace Plan-as-contract links. |
| [src/clients/examples/README.md](../src/clients/examples/README.md) | SDK example users | Reviewed — Source | Clarified identity prerequisites and side effects; merged setup guidance. |
| [src/clients/examples/codestats/README.md](../src/clients/examples/codestats/README.md) | SDK example users | Reviewed — Source | Preserved example purpose with explicit bus/identity prerequisites. |
| [src/clients/examples/go/README.md](../src/clients/examples/go/README.md) | SDK example users | Reviewed — Source | Added bus/identity setup prerequisite; retained runnable Go example. |
| [src/clients/examples/python/README.md](../src/clients/examples/python/README.md) | SDK example users | Reviewed — Source | Clarified prerequisites and current example entry; removed redundant setup. |
| [src/clients/examples/rust/README.md](../src/clients/examples/rust/README.md) | SDK example users | Reviewed — Source | Corrected example setup and package entry; retained contributor build steps. |
| [src/clients/examples/ts/README.md](../src/clients/examples/ts/README.md) | SDK example users | Reviewed — Source | Clarified environment/identity prerequisites; retained TypeScript example. |
| [src/clients/go/README.md](../src/clients/go/README.md) | SDK users | Reviewed — Source | Full SDK pass: current install/identity prerequisites and protocol behavior; removed repeated setup/history. Examples and language-specific APIs retained. |
| [src/clients/java/README.md](../src/clients/java/README.md) | SDK users | Reviewed — Source | Full SDK pass: current install/identity prerequisites and protocol behavior; removed repeated setup/history. Examples and language-specific APIs retained. |
| [src/clients/php/README.md](../src/clients/php/README.md) | SDK users | Reviewed — Source | Full SDK pass: current install/identity prerequisites and protocol behavior; removed repeated setup/history. Corrected executable PHP snippets. |
| [src/clients/python/README.md](../src/clients/python/README.md) | SDK users | Reviewed — Source | Full SDK pass: current install/identity prerequisites and protocol behavior; removed repeated setup/history. Examples and language-specific APIs retained. |
| [src/clients/rust/README.md](../src/clients/rust/README.md) | SDK users | Reviewed — Source | Full SDK pass: current install/identity prerequisites and protocol behavior; removed repeated setup/history. Examples and language-specific APIs retained. |
| [src/clients/tests/kv/README.md](../src/clients/tests/kv/README.md) | Test contributors | Reviewed — Source | Keep explicit reset/remaining-state and lock caveats; it does not promise no side effects. |
| [src/clients/typescript/README.md](../src/clients/typescript/README.md) | SDK users | Reviewed — Source | Full SDK pass: current install/identity prerequisites and protocol behavior; removed repeated setup/history. Examples and language-specific APIs retained. |
| [src/container/README.md](../src/container/README.md) | Container users/contributors | Reviewed — Source | Release archive path first; separate source image mechanics and actual isolation limits. |
| [src/internal/envfile/testdata/README.md](../src/internal/envfile/testdata/README.md) | Fixture contributors | Reviewed — Source | Retained parser-vector format and real limitations; fixture-only scope. |
| [src/mcp/README.md](../src/mcp/README.md) | MCP users/contributors | Reviewed — Source | Merged repeated setup/tool narrative; correct installed entry and current MCP/protocol contracts. |
| [src/npm/README.md](../src/npm/README.md) | Package users/contributors | Reviewed — Source | Clarified installed binaries versus package build; removed publication/history narration. |
| [src/testdata/client-conformance/README.md](../src/testdata/client-conformance/README.md) | Fixture contributors | Reviewed — Source | Current executable conformance vectors and requirements; removed historical/future compatibility prose. |
| [src/agents/shared/subscriptions.md](../src/agents/shared/subscriptions.md) | Contributors | Full read; wording corrected | Preserve detailed shared-engine/regex contract; TL;DR says one-attempt forwarding, not an exactly-once guarantee. |
| [src/agents/standalone/starter.md](../src/agents/standalone/starter.md) | Contributors | Full read; wording corrected | Preserve source installer/lifecycle contract; generic realm and Configuration link replace private example/stale label. |
| [src/maintenance/OBSERVATION.md](../src/maintenance/OBSERVATION.md) | Maintainers | Full read; keep | Observation-only maintainer procedure, with explicit partial-measurement limits; not an installed-user cleanup policy. |
| [src/mcp/commands/ls.md](../src/mcp/commands/ls.md) | MCP users | Full read; keep | Short prompt maps arguments to listing; no duplicate reference or release narrative. |
| [src/mcp/commands/send.md](../src/mcp/commands/send.md) | MCP users | Full read; keep | Short send/consume prompt preserves unknown-outcome and no-resend guidance. |
| [src/agents/rag/CHANGELOG.md](../src/agents/rag/CHANGELOG.md) | Historical evidence | Full read; keep chronology | Genuine changelog; historical feature descriptions do not override current RAG guide. |
| [src/web/web-face/node.md](../src/web/web-face/node.md) | Web contributors | Full read; current-contract cleanup | Retained UI/state tables; removed obsolete route aliases and rewrite-history section; node diagnostics remain current. |
| [src/web/web-face/site-map.md](../src/web/web-face/site-map.md) | Web contributors | Full read; current-contract cleanup | Canonical tag routes and unsupported aliases; added existing runner settings/control routes; removed history navigation. |
| [src/web/web-face/runners.md](../src/web/web-face/runners.md) | Web contributors | Full read; current-contract cleanup | Current status rows require key/name/state/enabled; removed fixed-version fallback narrative; kept unknown outcomes and optional observations. |
| [src/web/web-face/people.md](../src/web/web-face/people.md) | Web contributors | Reviewed — Source | Full semantic pass: user/group/account contracts retained; repeated Configuration recovery linked to its owning reference. |
| [src/web/web-face/records.md](../src/web/web-face/records.md) | Web contributors | Reviewed — Source | Full semantic pass: routes, filters, authority, forms and Configuration contracts reconciled; Roles duplication removed. |
| [src/web/web-face/shell.md](../src/web/web-face/shell.md) | Web contributors | Reviewed — Source | Full semantic pass: current process/TLS/socket/origin/refusal reference retained; editorial history and duplicate return/count rules removed. |

## Moved and historical material

| Former path | Disposition |
|---|---|
| `docs/agents/db.md` | Moved to [source-package guide](../src/agents/db/guide.md); excluded from shipped catalogue |
| `docs/agents/discord.md` | Moved to [deferred guide](R1.1/discord-user-guide.md); excluded from shipped catalogue |
| Dated entries in `docs/decisions.md` | Moved to [decision archive](done/decisions-archive.md); current index links contracts |
| Setup deployment/history blocks | Maintainer procedure in [.claude/docs/deployment.md](../.claude/docs/deployment.md); prior transitions in [setup history](done/setup-upgrade-history.md) |

The catalogue generator selects the release packager's runtime/standalone inventory. Source manifests and generated links follow the moved guides; no unsupported package is presented as shipped.

## Preserved distinctions

- Operational limits remain: uncertain send outcomes, queued versus consumed durability, export exclusions, short-lease and separate-write examples, platform limits and Secret exposure.
- Three source excerpts in the CLI and Python-process tutorials match their source. All three tutorials execute their command blocks, including the runner page, which quotes no source region. Caveats retain the real Secret parsing, lease/counter gaps and bounded waits.
- First use avoids private development identities and unnecessary output filtering. Real version diagnostics, scheduled future events, standards terminology and actual runtime history remain useful.
- Security warnings beside a sensitive action may repeat a short statement from the reference: user-account runner credentials are readable by that user's own processes; backup archives must be private from creation.
- Current-format setup/update/recovery remains supported. Prerelease conversion, retired aliases and old stored formats are separate implementation removals, not prose-only deletions.

## Final verification

- `src/smoke.sh --normal`: **1298 passed, 0 failed**, exit 0, **61.213 seconds**, all 63 selected shards at normal. Runtime tree: `12524143`; subsequent changes are documentation only.
- The one skipped case is `test_real_user_scope_survives_starter_service_restart`, explicitly assigned to slow. No slow-suite or release/tag/publication claim is made.
- Agent-page checks: 8/8; documentation-order checks: 27/27. Standard Markdown/inbound-anchor checker passes.
- Supplemental offline audit of 107 changed public/source documents: 1866 Markdown links, 33 HTML links and 6 local-repository GitHub targets; no missing paths or anchors.
- README retains all three original image assets and four byte-identical Mermaid diagrams.
- The executable guide shard passes 122/0: three source-excerpt comparisons, 18 executable command blocks and five output comparisons. It ran again in the final normal gate.
- Independent reviews, reproduced findings and focused compiling mutation evidence are recorded in [the integration review](issues/2026-11/2026-11-04.md) and the owning commits. The first aborted integration run is retained there as a failed attempt, not a passing gate.

No live deployment, provider request, push, publication or tag was performed. Historical material stays in Plans; user-facing guides describe the current product.
