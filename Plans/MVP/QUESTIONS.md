# MVP questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q21 | Whether reading an inbox and filtering one become separate options | owner, with the MVP CLI | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| Q70 | Whether the faces should report a filtered read at all, as a fact of its own | owner, on what the faces report | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox). Raised writing F.13.1's backlog fixture and verified: `withLiveness` sets `Reading` only for an unfiltered waiter (core/bus.go), so a read restricted to a topic or tag is not in that bit at all — though `deliver` serves a matching one of those ahead of an unfiltered reader, so it is not a reader that takes nothing. The label now says **no unfiltered reader** rather than *no reader waiting*, and the pages say which readers the column leaves out — so nothing false is being rendered while this is open. What remains open is whether a face should be able to say *how many* filtered readers are attached, which is a new fact on the record and so the owner's to ask for. **For:** attach a reader and find out why the reader is waiting on another topic are different repairs, and today the page cannot tell them apart. **Against:** a second reader count on every row, for a state most nodes never have. No test written either way; the fixture pins today's meaning |
| Q69 | Whether a start may serve when it could not persist a credential it just revoked. Both startup sweeps log a failed `Forget` and carry on; `Tokens.Forget` leaves the authentication maps unchanged when its write fails, so the old bytes still authenticate. The name is then free, and whoever registers it next is answered by their predecessor's credential | owner, on the access contract | [ownerless credentials](../../docs/02-access.md#ownerless-credentials). Raised by codex reviewing H.5.5 and reproduced: keep the bytes, purge the record, re-register the name to somebody else, and the old token answers `200`. Not introduced here — it is the pre-existing policy of the credential sweep, and deleting records only makes names free sooner. **There is no retry, and it is not a window.** Once the freed name is registered again, `identityKind` answers *record*, so `ownerless` is false and every later start **keeps** the old bytes — reproduced. The previous holder authenticates as the new owner's service for as long as that name exists. **For failing the start:** a revocation that did not land is not a revocation, and what follows is a reclaimed identity rather than a refused call. **Against:** a daemon that will not start because one credential could not be dropped is a daemon a full disk takes down entirely. A third shape exists: keep serving but hold the name unregistrable until the revocation lands. No test written either way |
| Q63 | Whether `ConsumeAs` omitting `b.active(name)` is deliberate: a third party may drain an inactive name's inbox although `Send` refuses to deliver to it | owner, on the access contract | [access § what a call carries](../../docs/02-access.md#what-a-call-carries). Raised by home-parf during the web review and verified: `Send` guards `b.active(rec.Name)` (bus.go:529) but the read path guards only the caller's standing and the stored `Disabled` bit. **For:** pausing a person should not orphan their queued work, and a maintainer draining it is recovery. **Against:** an unchecked target state in the read path is the shape [0.5.31](../../CHANGELOG.md) was closing. No test written either way |

## Personal service access

❓ The [Personal ACL rule](../../docs/03-services-and-topics.md#personal-and-shared)
restricts entries to other services. For a non-empty Personal ACL, should
Maintainer or master authority still admit other users without an explicit user
entry? These implicit grants exist today. The empty case is settled by the
[owner-only default](../../docs/02-access.md#acl), not an open question.
The classification and requested web views are settled; the non-empty case's
implicit grants are not.

## Authority model

### Effective Maintainer membership

⁉️ The [owner-control requirement](../../docs/01-identity-and-roles.md#services)
needs protection when an Administrator can edit a group granting maintenance
authority. Nested subgroup edits can also change effective membership. Shared
groups can be assigned by several resource owners.

**Owner answer:** _Pending — separate owner-controlled maintenance membership
from ordinary access groups, or protect every group change affecting it; define
whose approval applies to a group shared by different owners._

### Transfer recipient

❓ May any active registered user receive service/channel ownership, regardless
of whether they currently have a self-owned registry record.

A self-owned record has the same name and owner, for example `alice@host` owned
by `alice@host`. Ordinary user creation and enrolment normally create one;
Administrator membership creates a profile without guaranteeing that record.
Current transfer requires the record. Transferring a service is distinct from
transferring a person's own identity.

**Owner answer:** _Pending. Recommendation: authorize by the recipient's active
user profile, without exposing the self-owned-record requirement to the user._
