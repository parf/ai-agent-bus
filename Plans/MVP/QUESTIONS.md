# MVP questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q21 | Whether reading an inbox and filtering one become separate options | owner, with the MVP CLI | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| Q69 | Whether a start may serve when it could not persist a credential it just revoked. Both startup sweeps log a failed `Forget` and carry on; `Tokens.Forget` leaves the authentication maps unchanged when its write fails, so the old bytes still authenticate. The name is then free, and whoever registers it next is answered by their predecessor's credential | owner, on the access contract | [ownerless credentials](../../docs/02-access.md#ownerless-credentials). Raised by codex reviewing H.5.5 and reproduced: keep the bytes, purge the record, re-register the name to somebody else, and the old token answers `200`. Not introduced here — it is the pre-existing policy of the credential sweep, and deleting records only makes names free sooner. **For failing the start:** a revocation that did not land is not a revocation, and the window ends in a reclaimed identity rather than a refused call. **Against:** a daemon that will not start because one credential could not be dropped is a daemon a full disk takes down entirely, and the current shape retries at the next start. No test written either way |
| Q63 | Whether `ConsumeAs` omitting `b.active(name)` is deliberate: a third party may drain an inactive name's inbox although `Send` refuses to deliver to it | owner, on the access contract | [access § what a call carries](../../docs/02-access.md#what-a-call-carries). Raised by home-parf during the web review and verified: `Send` guards `b.active(rec.Name)` (bus.go:529) but the read path guards only the caller's standing and the stored `Disabled` bit. **For:** pausing a person should not orphan their queued work, and a maintainer draining it is recovery. **Against:** an unchecked target state in the read path is the shape [0.5.31](../../CHANGELOG.md) was closing. No test written either way |
