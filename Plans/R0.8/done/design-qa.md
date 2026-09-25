# Design QA — compact AgentBus web redesign

📌 **TL;DR:** History. What the 0.5.x web redesign was checked against and what was observed; not a current contract.

## Source of truth

- Selected visual direction: `Plans/R0.8/web-handoff/img/services-option-2.png`
- Selected content direction: Option 1 in `Plans/R0.8/web-handoff/visual-options.md`
- Implemented desktop capture: `tmp/services-option2-implementation-052.png`
- Side-by-side comparison: `tmp/services-option2-comparison-052.png`
- Comparison viewport: 1487 × 1058 CSS pixels at device scale 1
- State: signed-in Owner view against the live read-only AgentBus registry; no registry mutation was made

## Visual passes

1. The first implementation kept the old 76 rem content cap and wrapped the filter toolbar. This made the table visibly narrower and taller than Option 2. The frame now uses a 104 rem operational canvas and keeps the desktop controls on one row.
2. The second capture used a different viewport from the source image. The final capture uses the same 1487 × 1058 dimensions for both sides.
3. The final comparison matches the selected neutral shell, compact header, title and section-link hierarchy, control density, broad table and operational footer. It retains the requested Option 1 facts rather than the mock data.

## Five-surface review

- **Typography:** compact system typography, strong page title, semibold descriptions and monospaced routing names preserve the selected hierarchy. My and Personal use the established blue/orange hierarchy; Personal wins when both apply.
- **Layout:** header, counted view links, search/filter/sort toolbar, full-width records table and compact footer follow Option 2. The service-detail editors are collapsed disclosures. The user editor uses a responsive card and field grid.
- **Colour:** the implementation uses the existing accessible house palette. Ownership and Personal states keep structural signals as well as colour. Numeric fields use tabular figures and right alignment.
- **Images and marks:** the existing bus mark and established page-title/entity glyph vocabulary are reused. No external image or new decorative asset was added.
- **Copy and content:** service descriptions lead, routing names appear once, delivery uses status glyphs, and long explanations move to immediate hover/focus tooltips with structured click popovers. The SSH note names the actual host onboarding command instead of inventing a profile field.

## Interaction and responsive verification

- Changing the Services sort and Activity record selectors submits their GET forms and keeps URL state; a `noscript` Apply fallback remains.
- Hovering a service-detail information button immediately exposes the full explanation; clicking it opens the structured native popover.
- Desktop and 375 px service-detail checks report zero page-level horizontal overflow.
- The narrow view retains the header, navigation, title, facts, disclosures and Danger Zone route without losing content.
- The browser console reported no messages after navigation and interaction.
- Activity visibly states an approximately ten-minute cadence and up-to-24-hour window. The existing pre-deploy daemon history remains honestly timestamped at its measured span.

## Dense administration follow-up

The 0.5.73 pass reviewed the owner-flagged Service registration, User detail,
Groups, Diagnostics and `claude/home-parf@parf.us` detail pages against the
live read-only daemon through a source-built preview. It replaced paragraph
forms with the same card/grid system, moved definition walls into immediate
help, and grouped human-facing counts without changing JSON or form values.

All five pages were inspected at 1440 px and 375 px. Each narrow page measured
`scrollWidth == clientWidth`; the browser console was empty. Hovering the Policy
help exposed its complete tooltip immediately, the click target remained a
native popover, and the service page retained Delivery, Policy, Queue & counters,
Activity, both authorized editors and the Danger Zone. Lighthouse snapshot on
the populated service detail reported 100 Accessibility and 100 Best Practices.

## Intentional differences from the mock

- Counts and rows come from caller-visible live data rather than fabricated examples.
- The table retains Readers, Reached and queue facts required by the accepted service contract.
- The header version/build, footer Owner/Uptime/Calls and current Agent glyph use project vocabulary and live daemon facts.
  *Correction, 2026-09-18 (0.5.82):* the call counts left the footer for the signed-in Overview node strip by owner instruction, so the footer observed here now carries Owner and Uptime only ([what a node says about itself](docs/05-discovery.md#what-a-node-says-about-itself)). The measurement above is what was observed at the time and is not restated.
- Rows without descriptions remain one line instead of receiving invented copy.

final result: passed

## Service and Channel journey follow-up

The 0.5.78 pass began with screenshots of the deployed 0.5.77 Channels list and
Service detail. The live empty Channels page exposed the remaining split defect:
its browser title said Services, its first column said Service, and its generic
empty result did not explain a Channel.

A current-source fixture then rendered a populated queue, a populated pub/sub
channel, owner and visitor detail, and the Channels list at desktop and 375 px.
The corrected pages have their own titles and vocabulary; queue work reads held
messages, pub/sub work reads accepted messages and subscriber count, and the
visitor sees the same readable facts without owner editors. Delivery-mode
filters remain visible at both widths. Every inspected page measured zero
page-level horizontal overflow, and no production record was changed.
