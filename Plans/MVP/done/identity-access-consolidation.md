# Identity and access consolidation

📌 **TL;DR:** Three overlapping documents became two concise, expandable references.

## Result

The former identity and owner-model documents now live in
[Identity and roles](../../../docs/01-identity-and-roles.md#scope).
[Access](../../../docs/02-access.md#scope) owns authentication, credentials,
ACL evaluation and the trust boundary. Old files were removed; incoming
references point to the defining sections in the new split.

At review, the three originals totalled 7,596 words and the two replacements
3,225: approximately 58% less text. Details are also collapsed, rather than
being the first thing the reader sees. All 18 current documents begin with a
one-line pin-marked TL;DR; the convention is recorded in CLAUDE.md.

Built/pending distinctions remain visible. Stale statements that owner
suspension and credential cleanup were unbuilt were corrected against source.
Historical incident narratives were removed from current contracts; credential
revocation's unresolved failure policy remains linked to Q69.

## Verification and review

* Markdown check including this note: 126 files, 2,726 relative paths/anchors,
  zero errors.
* All 33 mapped old anchors resolve to their replacement sections.
* Runtime source edits are comment-only reference migrations, not code changes.
* Live mdhouse rendering in isolated Chromium: nine details blocks in Identity
  and roles, seven in Access, all initially closed. The Names table is hidden
  before clicking its summary and visible afterward. Screenshots inspected.
* OpenCode independently checked retained requirements, status wording, summaries,
  references and source-comment scope, and approved the result.

The Markdown checker does not inspect source comments. Those were checked
separately during reference migration. Literal old paths in the earlier
[historical migration table](document-migration.md#section-moves) remain
historical labels. Ignored frozen smoke copies (`*.local.sh`) and scratch logs
also retain their original comments: they are evidence, not current source.
No runtime test rerun or version change was needed for this documentation pass.

## Diagram follow-up

At the owner's request, six Mermaid diagrams now illustrate role scopes,
authorization, enrolment, configuration privacy, topic delivery and receipts
across docs 01–04. Supporting diagrams sit in expandable details. OpenCode
reviewed their semantics; all six rendered in mdhouse and screenshots were
inspected. The link check covered 2,728 local references with no errors.

Access also embeds the existing `getting-tokens.svg` and `user-to-service.svg`.
Both loaded in the browser. Their asset home remains `Plans/MVP` deliberately;
moving that directory must update these image references as well. The originals
were not lost in consolidation and remain linked from the MVP README.
