# MVP questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q21 | Whether reading an inbox and filtering one become separate options | owner, with the MVP CLI | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| Q62 | Who owns the dashboard token file — the one named person every token change goes through | owner | [visual design § risk, recorded](web/visual-design.md#risk-recorded). A standing commitment rather than a technical choice: the recorded density risk is conditional on *nobody owning typography*, and the five checks make the lapse visible without creating the ownership. The control half was settled 2026-09-16 — [neither ships in MVP](web/visual-design.md#theme-and-density-controls) |
| Q63 | Whether `ConsumeAs` omitting `b.active(name)` is deliberate: a third party may drain an inactive name's inbox although `Send` refuses to deliver to it | owner, on the access contract | [access § what a call carries](../../docs/02-access.md#what-a-call-carries). Raised by home-parf during the web review and verified: `Send` guards `b.active(rec.Name)` (bus.go:529) but the read path guards only the caller's standing and the stored `Disabled` bit. **For:** pausing a person should not orphan their queued work, and a maintainer draining it is recovery. **Against:** an unchecked target state in the read path is the shape [0.5.31](../../CHANGELOG.md) was closing. No test written either way |
