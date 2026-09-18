# Compact web administration pages

📌 **TL;DR:** 0.5.73 makes the dense administration pages share one compact,
responsive form and fact layout while preserving every authority and data boundary.

## Result

Service and Channel registration use labelled field grids, explicit radio or
checkbox choices, full-width line-list ACL input and one primary action. User
detail separates editable profile data from identity, authority, memberships,
lifecycle and owned resources; SSH onboarding remains a host command, not a
fabricated profile field. Groups use responsive cards and full-width membership
textareas.

Record detail presents Delivery, Policy and Queue & counters as compact fact
cards with the existing Activity view. Delivery control lives with Delivery;
settings and Maintainers remain collapsed until selected or refused. Diagnostics
keeps node scope, refusals, backlogs, exchanges, registry, loss and credentials,
while explanatory paragraphs move to immediate hover/focus help and structured
native popovers.

Human HTML groups decimal integer counts with commas. The formatter never
touches JSON, URLs, editable syntax or form values.

## Checks

Focused and repository Go tests pass. Seven targeted mutations fail
independently: removing decimal grouping, the Service fact grid, User split
layout, Group grid, Registry help target, immediate Policy tooltip or Profile
save action. The first action mutant changed the duplicate new-user action and
survived because the named check exercises existing-user detail; it receives no
credit. A corrected mutant removed that page's action and failed the check.
Real Chromium checked all five owner-
flagged pages at 1440 px and 375 px against the live read-only daemon through a
source-built preview. Every narrow page had zero page-level horizontal overflow,
the console was empty, the Policy tooltip appeared on hover, and the populated
service detail scored 100 Accessibility and 100 Best Practices in Lighthouse.

Documentation validation checked 170 Markdown files and 2,897 local links with
zero errors. The corrected candidate passed the frozen slow smoke with 609
checks and zero failures. All 19 candidate files and the smoke runner matched
their frozen hashes after the run; the evidence result was then filled in.

## Scope

This advances F.13.3–F.13.5 presentation and responsive acceptance. It does not
claim the pending reader filter, pagination, complete role journeys, Overview,
installed five-role rerun or a template-engine migration.
