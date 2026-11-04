# Documentation cleanup rules

📌 **TL;DR:** Draft for owner review, based on Astra's complete read of eight representative files. User documentation describes the shipped product and helps someone use it; development history, future scope and editorial process stay outside that reading path. This draft changes no product behavior or user guide.

## Review baseline

- Owner direction: public docs cover the shipped R1 scope, without product release/stage narration. Setup is a short guide with the user's AI executing AI-SETUP.md as the recommended first path. The same entry handles requested updates and tuning.
- Deprecated project support is a separate **1.0.1** code task. Removing its historical prose does not remove the implementation.
- Astra reviewed all blocks in the eight files below: **2,743 lines**, plus CLAUDE.md and [.claude/docs/WRITING.md](../.claude/docs/WRITING.md#how-we-write-docs). No edits, tests, builds, installs or provider calls were part of its analysis.
- The sample is the first review batch, not a claim that all documentation has been read. The wider inventory has 65 Markdown files under docs/ and 37 tracked source README/INSTALL candidates, plus root entry points and other linked references.
- Publication, push and the final tag remain on hold.

| Sample file | Main findings |
|---|---|
| [README.md](../README.md) | Competing first actions; roadmap, contributor-policy and evidence links inside the product narrative. |
| [AI-SETUP.md](../AI-SETUP.md) | Mostly useful; repeated scope questions and publication-specific contingencies need tightening. |
| [Overview](../docs/00-overview.md) | Old release-line opening, internal ownership prose, dated examples and future-plan navigation. |
| [Setup](../docs/07-setup.md) | 893 lines mix installation with operator reference, development deployment, migrations and test reports. |
| [Launchers](../docs/06-launchers.md) | Status/MVP tables, dated proof and historical narration mixed with useful runtime limits. |
| [Records](../docs/03-records.md) | Historical compatibility text and roadmap links; actor and PubSub descriptions need verification. |
| [Agent catalogue](../docs/agents/README.md) | Discord availability contradiction, editorial rules and a misleading sandbox/grants explanation. |
| [Telegram](../docs/agents/telegram.md) | Useful operations/settings reference, with dated provenance and editorial narration to remove. |

Line references below refer to main `1ab9031a`, before cleanup.

## Classify each block

| Class | What belongs here | Disposition |
|---|---|---|
| Product instruction | An action the reader can perform with shipped behavior. | Keep; make the action and outcome clear. |
| Necessary limitation | Permissions, platform limits, uncertain effects, data-loss windows, secrets, recovery. | Keep beside the affected action; preserve its exact conditions. |
| Useful reference | Current commands, settings, fields, limits and operational procedures. | Keep in the right audience's reference; link from the short guide where needed. |
| Example | Genuine command/output, diagram or worked scenario. | Keep useful examples; remove test provenance and disclose abbreviations. |
| Development evidence | Build status, test counts, mutant results, run dates or acceptance procedures. | Move to Plans/done or contributor evidence; remove from user pages. |
| History or roadmap | Release/stage changes, superseded designs, owner attribution, future capabilities. | Keep in the appropriate plan/history home; remove from user pages. |
| Editorial filler | Text explaining document ownership, section placement or writing policy. | Delete; put an actual authoring rule in .claude/docs if needed. |
| Suspect claim | Contradictions, inflated guarantees or questionable commands. | Check implementation/shipping evidence; record the result before rewriting. |
| Deprecated support | Old aliases, mirrors, formats, migrations or fallback behavior actually implemented. | Inventory separately for 1.0.1; keep current technical facts truthful until changed. |

## To fix: first batch

### Content and navigation

| Item | Evidence | Required edit |
|---|---|---|
| F01 | README:139–145 | Put the recommended AI execution path first. Follow with the same entry for requested updates/tuning and a manual fallback. Remove the competing “shortest path.” |
| F02 | README:24; setup:27–43,63–71 | State usable install paths. Remove reserved-name/publication/test-process narration. Confirm shipping availability; removing a warning must not turn a placeholder into a working installer. |
| F03 | README:163–164,302–305,317,407–411 | Remove absolute test-history claims, design/evidence links and the “coming soon” backend promise from the product explanation. Separate contributor navigation. |
| F04 | AI-SETUP:21,35,50,74 | Use “read and execute.” Reuse the user's stated goal; ask only for missing choices, access or required approval. Detecting an installed bus must not force another scope conversation. |
| F05 | AI-SETUP:42 | Replace the project-specific npm publication history with a general check that the chosen artifact exists and is installable. |
| F06 | overview:5–6,45,49 | Remove release-line/plan language and ownership scaffolding. Keep the useful navigation map with reader-facing labels. |
| F07 | overview:53,68–69,101–102 | Replace plan/conformance pointers with current references; update setup navigation after splitting it; remove excluded-future process commentary. |
| F08 | overview:32–39; records:30–43; launchers:30–45; Telegram:56–67 | Remove “ran today,” dates and development-node provenance. Keep real output, its useful explanation and any abbreviated-ID disclosure. |
| F09 | records:103–105 | Keep the current ACL rule; remove the undecided future split and its plan link. |
| F10 | agent catalogue:83–85; Telegram:25 | Delete “What a guide leaves out” and “The abilities, last.” These explain editing, not using the product. |
| F11 | catalogue:98,124; docs/agents/discord.md | Match the catalogue and generated index to shipped packages. Discord is outside the selected R1 scope: move its user-guide material to its plan and remove it from the R1 selection paths. Update generation inputs rather than hand-editing the generated block. |
| F12 | launchers:51–58,61,65,76–97,101–165,212,397–404,426–431 | Remove Status/MVP/Scope tracking, Built labels, owner dates, decision IDs and acceptance narratives. Keep current adapter, naming, retry, takeover and recovery behavior. |
| F13 | setup:201,215 | Remove the conflicting “verified”/“still needs acceptance” process claims. Publish the actual supported Mac procedure and limits. |

### Verify meaning before editing

| Item | Evidence | Required investigation |
|---|---|---|
| V01 | records:95–102 | Reconcile the intended actor model (“nobody acts as the other five”) with credential behavior. State exactly who can authenticate and act. |
| V02 | records:177 | Reconcile “PubSub is queued” with the documented no-retention behavior. Distinguish fan-out delivery from durable inbox storage. |
| V03 | catalogue:61–71 | Replace the claim that helper grants alone make an AI sandboxed/unable to touch host files or accounts. Bus permissions and process confinement are different controls. |
| V04 | catalogue:57 | Replace “meant to work” with the actual shared subscription behavior, retaining supported per-helper differences. |
| V05 | launchers:243–250 | Determine how intentional clearing with `--tag ''` differs from refused empty labels; preserve the real distinction. |
| V06 | setup:380 | The user-runner inventory example redirects into an existing file. Provide a fresh-file precondition or a merge procedure that preserves other entries. |
| V07 | setup:389; AI-SETUP:52 | Distinguish shared-package defaults from bundled packages requiring explicit enabled:true. Check the actual loader before rewriting. |

### Separate 1.0.1 compatibility work

- C01 — records:139,208–222: personal field/tag mirror, accepted old input and stored-tag cleanup.
- C02 — launchers:57,103,309,338–340: distinguish external runtime-version support from deprecated AgentBus behavior; inspect prior Claude MCP-definition cleanup and duplicate-reader prevention.
- Extend this inventory across CLI aliases, all six clients, package formats and stored-state paths during the full pass. A search hit containing “legacy” is not itself proof of deprecated project support.
- For each real compatibility path, record its reader, writer, consumer, current tests and removal consequence. Removing support must have a named refusal or deliberate replacement behavior and preserve valid current data.
- Do not silently delete active input/output fields, migration safeguards or security refusals while editing prose. Do not publish a claim that they are already removed.

## Setup page: target shape and destinations

The first screen should give the recommended action and the same entry for later updates or tuning. Use short steps; collapsing the existing manual into details blocks does not solve its scope problem.

1. Start the user's AI on the target computer and ask it to read and execute AI-SETUP.md.
2. Answer only missing choices/account questions. The AI reads main docs and performs the applicable steps.
3. Existing installation: reuse that entry for a requested update, bus configuration or any agent's tuning; preserve existing settings.
4. Manual fallback: concise platform/edition choice and the actual installation instructions.
5. Verify that the selected installation works; link to the relevant troubleshooting/reference page.

| Current setup material | Destination |
|---|---|
| Program inventory (75–89) | Consolidate into the existing CLI guide; no duplicate inventory in installation. |
| Archive installation (91–150) | Keep the manual procedure in INSTALL; setup links to it. |
| Build/release verification, test reports and status (151–195,732–782) | Source/contributor docs and acceptance evidence. |
| Mac selection and active limits (197–230) | Essential choice near install; detailed launchd/socket/storage reference separately. |
| mac2-check teardown (232–251) | Acceptance runbook. A real uninstall guide must describe product state, not test fixtures. |
| Accounts/units (253–292) | Existing process and operator reference. |
| System and user-account runners (294–397) | Runner setup/configuration reference; preserve secret-exposure warnings and fix the inventory questions above. |
| TLS/browser trust (399–463) | Remote-access/security or operator reference; setup links at the point of remote connection. |
| Local users/SSH admin/account maps (465–549) | Current identity/admin reference. |
| Storage (551–567) | Current storage/durability reference. Separate active migrations from historical narration. |
| Backups (568–624) | Dedicated current backup/restore reference, preserving export exclusions and destructive restore consequences. |
| Logs/config locations/sample data (626–668) | Operator/CLI/tutorial references as appropriate. |
| Development-node cleanup/minimal component deployment (670–730) | Maintainer runbook; these are not installed-product defaults. |
| Earlier upgrades (784–893) | Historical runbooks and the separately verified compatibility inventory. Extract any still-current rule first. |

Choose existing homes first. Create a new reference only for a genuinely missing home, then move the original content and update links; do not copy it into another competing manual.

## Before and after: proposed edits

These are editorial proposals, not executed commands or invented output.

| Before | Proposed treatment |
|---|---|
| “this documentation covers current behaviour on the 0.9 line, and anything not built is in its release plan” | Remove the clause. Retain an accurate description of names, permissions and message delivery. |
| “The output is from the development node on 2026-10-30.” | Delete. Explain relevant output fields or replacement names only where useful. |
| “Everything below is the reference. Each section owns its values; other pages link here.” | Delete. Keep actual reference/navigation without an introductory ownership lecture. |
| `MVP / Scope / Built` status table | Delete the tracking table. Retain any unique current capability or limit in its proper functional section. |
| “The launcher no longer detaches …” | State its current wait/start behavior, retaining the actual conditions. |
| “Publication pending … Local package assembly … verified …” | Remove publication/test history. Give the usable install action in the correct place. |

## To avoid during cleanup

- No regex-only rewrite. Read the complete page and classify every paragraph, table row, code fence, caption, diagram and details block.
- No blanket ban on words. Real status commands, scheduled future messages, event history, canonical names and standards terminology can be necessary current behavior.
- No fixed product-release/stage labels in user narrative. Keep useful version diagnostics, protocol/dependency compatibility requirements and legal license identifiers where they serve a concrete action or contract.
- No unsupported affirmative claim produced by deleting “not shipped” or “pending.” Check shipped inventory and behavior first.
- No invented clean output or silently substituted names/counts. Preserve verified examples; label shortened output and make user-specific inputs clear.
- No removal or weakening of current security, privacy, data-loss, platform or recovery caveats to make a page shorter.
- No automatic deletion of technical detail. An API field reference, backup recovery procedure or setting can be useful in the correct guide.
- No silent image/diagram loss. Preserve useful assets; review whether a screenshot still represents the described behavior.
- No duplicate contract/reference after moving content. Preserve useful navigation and repair all inbound links and anchors.
- No operational provider calls, messages, host changes, builds or runtime suites merely to remove prose. Reuse existing evidence; make only necessary, authorized read-only checks when a concrete claim cannot otherwise be verified.

## Apply to all offered documentation

1. Inventory the reading surface: root README/AI-SETUP, docs/, INSTALL, SDK/example guides, container/npm guides and shipped package READMEs. Include linked references and generated blocks; distinguish internal fixture/contributor-only files.
2. Maintain one review row per file: audience, all blocks reviewed, issue classes, keep/delete/move/verify decisions, related links/assets and remaining questions. Every candidate gets a disposition, including “keep as-is.”
3. Start with entry points and setup, then concepts/access/messaging, runner/user guides, agent guides/catalogue, SDKs/examples and remaining references. Do not stop after the first batch or after search hits disappear.
4. Keep history and future scope in Plans. Review public governance/reference pages such as docs/decisions.md and constitution.md too: preserve current rules while removing dated provenance, roadmap and internal stage navigation from the user path.
5. Update governing instructions after owner review: CLAUDE.md still says docs cover MVP with built/pending explicit, conflicting with WRITING.md and the owner's shipped-product rule. Replace that conflict so subsequent writers do not restore the noise.
6. Update generated-content sources and inbound links as part of each move. Inspect generator selection and manifests before excluding an unshipped helper; keep documentation-only scope unless a code defect is separately assigned.
7. Re-read edited pages as a first-time user, then audit navigation across pages. Every retained block must help the reader choose, act, understand a relevant result/contract or recover from a concrete problem.
8. Validate changed paths, anchors, references and assets. Focused documentation-checker changes need their own proof only if the checker must change; no full product gate solely for prose.

## Owner review checklist

- [ ] The block classes and to-avoid exceptions match the intended cleanup.
- [ ] The short setup shape and proposed reference destinations are acceptable.
- [ ] Verification questions V01–V07 stay separate from cosmetic rewriting.
- [ ] Actual deprecated-support removal remains 1.0.1 work.
- [ ] All offered documentation gets a file-by-file disposition after these rules are reviewed.

Status: **draft for review**. The sampled user files have not been cleaned by this change; publication remains on hold.
